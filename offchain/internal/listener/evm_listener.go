package listener

import (
	"bytes"
	"context"
	"fmt"
	"math/big"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	lru "github.com/hashicorp/golang-lru/v2"
	_ "github.com/zane/web3-offchain/prismsettle/parser"

	// "github.com/zane/web3-offchain/internal/notify"
	"github.com/zane/web3-offchain/internal/parser"
	"github.com/zane/web3-offchain/internal/service"
	"github.com/zane/web3-offchain/model"
	"github.com/zane/web3-offchain/pkg/config"
	"github.com/zane/web3-offchain/pkg/constant"
	"github.com/zane/web3-offchain/pkg/logger"
	"github.com/zane/web3-offchain/pkg/rpc"
	"github.com/zane/web3-offchain/pkg/utils"
)

// isPrismAmountEvent reports whether a PrismSettleJob event carries an
// escrow amount whose TokenAddr is the Job contract mirror rather than the
// real token (resolved from getJobPaymentToken).
func isPrismAmountEvent(eventType string) bool {
	switch eventType {
	case "PRISM_JOB_FUNDED", "PRISM_JOB_COMPLETED", "PRISM_JOB_REFUNDED", "PRISM_ARBITRATION_EXECUTED":
		return true
	}
	return false
}

// getJobPaymentToken(uint256) selector.
var jobPaymentTokenSig = crypto.Keccak256([]byte("getJobPaymentToken(uint256)"))[:4]

// fetchPaymentToken reads the per-job escrow token from the Job contract via
// eth_call getJobPaymentToken(jobId). Returns "" on any failure (the caller
// then keeps the mirrored TokenAddr).
func (l *EVMListener) fetchPaymentToken(ctx context.Context, client *ethclient.Client, jobID string) (string, error) {
	id, ok := new(big.Int).SetString(strings.TrimPrefix(jobID, "0x"), 16)
	if !ok {
		return "", fmt.Errorf("bad job id: %s", jobID)
	}
	calldata := append(append([]byte{}, jobPaymentTokenSig...), common.LeftPadBytes(id.Bytes(), 32)...)
	jobAddr := common.HexToAddress(l.chainConfig.ContractAddr)
	out, err := client.CallContract(ctx, ethereum.CallMsg{
		To:   &jobAddr,
		Data: calldata,
	}, nil)
	if err != nil {
		return "", err
	}
	if len(out) < 32 {
		return "", fmt.Errorf("short response for getJobPaymentToken")
	}
	return common.BytesToAddress(out[:32]).Hex(), nil
}

// EVMListener listens to EVM blockchain events, processes blocks in order,
// handles reorg, caches block headers, and persists events reliably.
type EVMListener struct {
	chainConfig        config.ChainConfig
	rpcClient          *rpc.RPCClient
	syncStateService   *service.SyncStateService
	eventIngestService *service.EventIngestService
	lockService        *service.LockService
	contractABI        abi.ABI
	contractParser     parser.EventParser

	cancelFunc context.CancelFunc
	stopOnce   sync.Once

	// lastBlock + nextExpected are both guarded by taskMutex. lastBlock is
	// the latest committed block; nextExpected is the next block to fetch.
	// They are updated together in tryCommitAll (under taskMutex.Lock) and
	// in the init path (also under taskMutex.Lock). Readers outside the
	// hot path (e.g. the sync-lag reporter) must take taskMutex.Lock too.
	//
	// P1-D: previously lastBlock had its own lastBlockMutex, which raced
	// with the taskMutex-protected writes — see the audit in the Phase 9
	// design review. Unified to a single lock to fix the data race.
	lastBlock      uint64
	nextExpected   uint64
	completedTasks map[uint64]finishedTask
	taskMutex      sync.Mutex

	blockCache *lru.Cache[uint64, *types.Header]

	// Phase 9: optional hooks for reorg persistence + health tracking.
	// nil = feature disabled (preserves Phase 7 behavior when not wired).
	reorgRepo     ReorgEventWriter
	healthTracker HealthTracker
}

// ReorgEventWriter is the listener's minimal view of the reorg repository.
// Defined here to avoid importing the repository package (which would
// create a cycle in some test graphs). main.go adapts the concrete repo.
type ReorgEventWriter interface {
	Insert(
		ctx context.Context,
		chainName, contractAddr string,
		fromBlock, toBlock uint64,
		oldHash, newHash string,
		rollbackDepth int,
		rolledBackRows int64,
	) error
}

// HealthTracker is the listener's minimal view of the health tracker.
// main.go adapts the concrete *prism_svc.HealthTracker.
type HealthTracker interface {
	IncReorg()
	SetSyncLag(lag int64)
}

// SetReorgRepo wires the reorg event repository. Optional — when nil,
// detected reorgs are still rolled back in chain_events but not persisted
// to the reorg_events table (Phase 7 behavior).
func (l *EVMListener) SetReorgRepo(r ReorgEventWriter) {
	l.reorgRepo = r
}

// SetHealthTracker wires the health tracker for /health endpoint updates.
// Optional — when nil, the listener does not bump reorg_count or update
// sync_lag (Phase 7 behavior).
func (l *EVMListener) SetHealthTracker(h HealthTracker) {
	l.healthTracker = h
}

// standardERC20ABI defines minimal ABI to query token symbol & decimals
var standardERC20ABI = `[{"constant":true,"inputs":[],"name":"symbol","outputs":[{"name":"","type":"string"}],"payable":false,"stateMutability":"view","type":"function"},{"constant":true,"inputs":[],"name":"decimals","outputs":[{"name":"","type":"uint8"}],"payable":false,"stateMutability":"view","type":"function"}]`

// finishedTask represents a finished block range waiting for ordered commit
type finishedTask struct {
	from   uint64
	to     uint64
	events []*model.ChainEvent
}

// BlockRange defines inclusive block range [From, To]
type BlockRange struct {
	From uint64
	To   uint64
}

// NewEVMListener creates a new EVM listener instance with dependency injection
func NewEVMListener(
	chainConfig config.ChainConfig,
	syncStateService *service.SyncStateService,
	eventIngestService *service.EventIngestService,
	lockService *service.LockService,
) (*EVMListener, error) {

	rpcClient, err := rpc.NewRPCClient()
	if err != nil {
		return nil, err
	}

	// Initialize block header cache (size from config)
	blockCache, err := lru.New[uint64, *types.Header](chainConfig.BlockCacheSize)
	if err != nil {
		return nil, fmt.Errorf("init block cache failed: %w", err)
	}

	// Parse base ERC20 ABI
	contractABI, err := abi.JSON(bytes.NewReader([]byte(constant.ABIMap[constant.ABITypeERC20])))
	if err != nil {
		return nil, fmt.Errorf("parse abi failed: %v", err)
	}

	// Get the parser for this single contract
	contractParser := parser.GetParser(chainConfig.ContractParser)
	if contractParser == nil {
		return nil, fmt.Errorf("parser not found for contract %s: %s", chainConfig.ContractAddr, chainConfig.ContractParser)
	}

	return &EVMListener{
		chainConfig:        chainConfig,
		rpcClient:          rpcClient,
		syncStateService:   syncStateService,
		eventIngestService: eventIngestService,
		lockService:        lockService,
		contractABI:        contractABI,
		contractParser:     contractParser,
		completedTasks:     make(map[uint64]finishedTask),
		blockCache:         blockCache,
	}, nil
}

// Start starts the listener: resume from last checkpoint, handle reorg, start producer & workers
func (l *EVMListener) Start(ctx context.Context) error {
	defer utils.LogPanic()
	defer l.rpcClient.Close()

	// Load last block for this single contract
	lastBlock, lastBlockHash, err := l.syncStateService.GetLastBlock(ctx, l.chainConfig.ChainName, l.chainConfig.ContractAddr)
	if err != nil {
		logger.Errorf("get last block failed for contract %s", logger.String("contract", l.chainConfig.ContractAddr), logger.Error(err))
		return err
	}
	if lastBlock == 0 {
		lastBlock = l.chainConfig.StartBlock
		lastBlockHash = ""
	}

	// Iterative reorg detection: roll back until hash matches (supports unlimited depth)
	if lastBlock > 0 && lastBlockHash != "" {
		for {
			reorgDepth, err := l.detectAndHandleReorg(ctx, lastBlock, lastBlockHash)
			if err != nil {
				logger.Errorf("handle reorg failed", logger.Error(err))
				return err
			}
			if reorgDepth == 0 {
				break
			}

			// Update to stable block after rollback
			lastBlock = lastBlock - uint64(reorgDepth)
			logger.Warn("reorg handled, rolled back to block", logger.Uint64("new_start_block", lastBlock))

			// Retry header fetch with backoff. Without this, a transient RPC
			// failure at startup would abort the entire listener; subsequent
			// restarts would loop forever on the same reorg.
			var h *types.Header
			headerRetry := l.chainConfig.MaxSyncRetries
			for attempt := 0; attempt < headerRetry; attempt++ {
				client, err := l.rpcClient.GetClient(l.chainConfig.ChainName)
				if err == nil {
					h, err = client.HeaderByNumber(ctx, big.NewInt(int64(lastBlock)))
					if err == nil {
						break
					}
				}
				if ctx.Err() != nil {
					return ctx.Err()
				}
				logger.Warn("get block header failed during reorg recovery, retrying",
					logger.Int("attempt", attempt+1),
					logger.Int("max", headerRetry),
					logger.Error(err))
				select {
				case <-time.After(time.Duration(attempt+1) * time.Second):
				case <-ctx.Done():
					return ctx.Err()
				}
			}
			if h == nil {
				return fmt.Errorf("get block header after reorg: exhausted retries at block %d", lastBlock)
			}

			_ = l.syncStateService.SetLastBlock(ctx, l.chainConfig.ChainName, l.chainConfig.ContractAddr, lastBlock, h.Hash().Hex())
			lastBlockHash = h.Hash().Hex()
		}
	}

	// Initialize sync state
	l.taskMutex.Lock()
	l.lastBlock = lastBlock
	l.nextExpected = lastBlock + 1
	l.taskMutex.Unlock()

	logger.Info("start listening",
		logger.Uint64("start_block", lastBlock),
		logger.Uint64("next_expected", l.nextExpected),
		logger.Int("workers", l.chainConfig.Workers),
	)

	// Create cancellable context
	ctx, cancel := context.WithCancel(ctx)
	l.cancelFunc = cancel

	// Task queue (buffer size from config)
	jobs := make(chan BlockRange, l.chainConfig.TaskQueueSize)
	var wg sync.WaitGroup

	// Start worker pool
	for i := 0; i < l.chainConfig.Workers; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			l.worker(ctx, id, jobs)
		}(i)
	}

	// Start block producer with backpressure control
	go func() {
		defer close(jobs)
		lastSendTime := time.Now()
		backpressureTimeout := time.Duration(l.chainConfig.TaskBackpressureSeconds) * time.Second
		sendRetryInterval := time.Duration(l.chainConfig.TaskSendRetryMS) * time.Millisecond
		blockNumberRetryDelay := time.Duration(l.chainConfig.BlockNumberRetrySeconds) * time.Second

		for {
			select {
			case <-ctx.Done():
				return
			default:
			}

			// Get latest chain block number
			currentChainBlock, err := l.GetBlockNumber(ctx, l.chainConfig.ChainName)
			if err != nil {
				logger.Warn("get block number failed, retrying...", logger.Error(err))
				time.Sleep(blockNumberRetryDelay)
				continue
			}

			// Phase 9 P0-C: report sync lag to /health endpoint.
			// Lag = current tip - last committed block. P1-D unified the
			// lastBlock lock to taskMutex, so we take it here for the read.
			l.taskMutex.Lock()
			lastCommitted := l.lastBlock
			l.taskMutex.Unlock()
			if l.healthTracker != nil && currentChainBlock >= lastCommitted {
				l.healthTracker.SetSyncLag(int64(currentChainBlock - lastCommitted))
			}

			// Only process confirmed blocks. Guard against unsigned underflow
			// when the chain height is below the confirmation threshold (e.g.
			// chain just started, RPC returned a low tip, or testnet reset).
			// Without this check, `currentChainBlock - Confirmations` wraps to
			// ~math.MaxUint64 and the listener would request a giant block
			// range from the RPC, causing errors or OOM.
			if currentChainBlock <= uint64(l.chainConfig.Confirmations) {
				time.Sleep(time.Duration(l.chainConfig.SyncInterval) * time.Millisecond)
				continue
			}
			processTo := currentChainBlock - uint64(l.chainConfig.Confirmations)

			l.taskMutex.Lock()
			currentNextExpected := l.nextExpected
			l.taskMutex.Unlock()

			// Up to date, wait
			if currentNextExpected > processTo {
				time.Sleep(time.Duration(l.chainConfig.SyncInterval) * time.Millisecond)
				continue
			}

			// Build task
			batch := uint64(l.chainConfig.BatchSizeSync)
			taskTo := currentNextExpected + batch - 1
			if taskTo > processTo {
				taskTo = processTo
			}

			task := BlockRange{From: currentNextExpected, To: taskTo}

			// Send with backpressure timeout
			select {
			case jobs <- task:
				lastSendTime = time.Now()
			case <-time.After(sendRetryInterval):
				if time.Since(lastSendTime) > backpressureTimeout {
					// Use Errorf + Stop instead of Fatal: Fatal calls os.Exit
					// and bypasses the graceful shutdown in main(), which can
					// leave in-flight commits half-written. Stopping the
					// listener surfaces the error to main via listenerErrChan.
					logger.Errorf("task queue congested for too long, stopping listener for manual intervention",
						logger.String("chain", l.chainConfig.ChainName))
					_ = l.Stop(ctx)
					return
				}
			}
		}
	}()

	wg.Wait()
	logger.Info("listener stopped normally")
	return nil
}

// worker processes block range tasks from queue
func (l *EVMListener) worker(ctx context.Context, workerID int, jobs <-chan BlockRange) {
	permanentFailureSleep := time.Duration(l.chainConfig.PermanentFailureSleepSeconds) * time.Second

	for {
		select {
		case <-ctx.Done():
			return
		case task, ok := <-jobs:
			if !ok {
				return
			}

			// Sync with retry (max times from config)
			events, err := l.syncBlockRangeWithRetry(ctx, task.From, task.To)
			if err != nil {
				logger.Errorf("sync block range permanently failed",
					logger.Uint64("from", task.From),
					logger.Uint64("to", task.To),
					logger.Error(err))
				// _ = notify.SendTelegramMsg(fmt.Sprintf("CRITICAL: sync stuck at %d-%d on %s", task.From, task.To, l.chainConfig.ChainName))
				time.Sleep(permanentFailureSleep)
				continue
			}

			// Mark task as completed
			l.taskMutex.Lock()
			l.completedTasks[task.From] = finishedTask{
				from:   task.From,
				to:     task.To,
				events: events,
			}
			l.taskMutex.Unlock()

			// Try sequential commit
			l.tryCommitAll(ctx)
		}
	}
}

// syncBlockRangeWithRetry retries sync up to configured max retries
func (l *EVMListener) syncBlockRangeWithRetry(ctx context.Context, from, to uint64) ([]*model.ChainEvent, error) {
	maxRetries := l.chainConfig.MaxSyncRetries

	var events []*model.ChainEvent
	var err error

	for i := 0; i < maxRetries; i++ {
		events, err = l.syncBlockRange(ctx, from, to)
		if err == nil {
			return events, nil
		}

		logger.Warn("sync range retry",
			logger.Uint64("from", from),
			logger.Uint64("to", to),
			logger.Int("retry", i+1),
			logger.Error(err))

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(time.Duration(i+1) * time.Second):
		}
	}
	return nil, err
}

// nextReadyTask is the pure ordering primitive behind tryCommitAll.
// It returns the task positioned at nextExpected (if present) along with the
// next expected value after committing it (task.to + 1). Extracted as a pure
// function so the sequential-commit invariant is unit-testable without RPC/DB.
func nextReadyTask(completed map[uint64]finishedTask, nextExpected uint64) (finishedTask, uint64, bool) {
	t, ok := completed[nextExpected]
	if !ok {
		return finishedTask{}, nextExpected, false
	}
	return t, t.to + 1, true
}

// tryCommitAll commits tasks in strict sequential order to ensure data consistency
func (l *EVMListener) tryCommitAll(ctx context.Context) {
	l.taskMutex.Lock()
	defer l.taskMutex.Unlock()

	for {
		task, advanced, ok := nextReadyTask(l.completedTasks, l.nextExpected)
		if !ok {
			break
		}

		// Persist events in order
		err := l.processEvents(ctx, task.from, task.to, task.events)
		if err != nil {
			logger.Errorf("process events failed", logger.Error(err))
			break
		}

		// Clean up and advance pointer
		delete(l.completedTasks, l.nextExpected)
		task.events = nil
		l.nextExpected = advanced
		l.lastBlock = task.to

		// Flush checkpoint (non-blocking on failure)
		if err := l.flushLastBlock(ctx); err != nil {
			logger.Errorf("flush last block failed", logger.Error(err))
			break
		}

		logger.Debug("order commit success", logger.Uint64("to", task.to))
	}
}

// processEvents batches events by block and saves to DB.
//
// Iterates only the block numbers that actually have events (sorted), and
// flushes once every commitStep blocks or at the end of the range. This
// avoids the O(range) walk the previous implementation did when most block
// numbers in [from, to] had no events.
func (l *EVMListener) processEvents(ctx context.Context, from, to uint64, events []*model.ChainEvent) error {
	if len(events) == 0 {
		return nil
	}

	// Group events by block number.
	group := make(map[uint64][]*model.ChainEvent, len(events))
	for _, e := range events {
		group[e.BlockNumber] = append(group[e.BlockNumber], e)
	}

	// Collect & sort the block numbers that have events.
	nums := make([]uint64, 0, len(group))
	for n := range group {
		nums = append(nums, n)
	}
	sort.Slice(nums, func(i, j int) bool { return nums[i] < nums[j] })

	commitStep := l.chainConfig.CommitStep
	var buf []*model.ChainEvent
	// Current flush boundary: smallest multiple of commitStep that is >=
	// the first event's block number. Subsequent boundaries advance by
	// commitStep each time. This keeps the modulo-equivalent semantics
	// (flush every commitStep blocks) without walking empty blocks.
	nextBoundary := ((nums[0] + commitStep - 1) / commitStep) * commitStep

	for _, num := range nums {
		buf = append(buf, group[num]...)

		for num >= nextBoundary {
			if len(buf) > 0 {
				if err := l.BatchSaveChainEvents(ctx, buf); err != nil {
					return err
				}
				buf = buf[:0]
			}
			nextBoundary += commitStep
		}
	}

	// Flush remainder.
	if len(buf) > 0 {
		if err := l.BatchSaveChainEvents(ctx, buf); err != nil {
			return err
		}
	}
	return nil
}

// syncBlockRange fetches logs and parses events from a block range
func (l *EVMListener) syncBlockRange(ctx context.Context, startBlock uint64, endBlock uint64) ([]*model.ChainEvent, error) {
	client, err := l.rpcClient.GetClient(l.chainConfig.ChainName)
	if err != nil {
		return nil, err
	}

	query := ethereum.FilterQuery{
		FromBlock: big.NewInt(int64(startBlock)),
		ToBlock:   big.NewInt(int64(endBlock)),
		Addresses: []common.Address{common.HexToAddress(l.chainConfig.ContractAddr)},
	}

	logs, err := client.FilterLogs(ctx, query)
	if err != nil {
		return nil, err
	}

	if len(logs) == 0 {
		return nil, nil
	}

	var chainEvents []*model.ChainEvent
	for _, logEntry := range logs {
		// Use the single contract parser for this listener
		if !l.contractParser.Match(logEntry) {
			continue
		}

		data, err := l.contractParser.Parse(logEntry)
		if err != nil || data == nil {
			// Don't fail the whole batch for one malformed log, but make the
			// dropped event traceable so operators can investigate data gaps.
			logger.Warn("drop log: parse failed",
				logger.String("tx_hash", logEntry.TxHash.Hex()),
				logger.Uint64("block", logEntry.BlockNumber),
				logger.Uint64("log_index", uint64(logEntry.Index)),
				logger.Error(err))
			continue
		}

		ce, ok := data.(*model.ChainEvent)
		if !ok {
			logger.Warn("drop log: parser returned unexpected type",
				logger.String("tx_hash", logEntry.TxHash.Hex()),
				logger.Uint64("block", logEntry.BlockNumber))
			continue
		}

		// Escrow token resolution: PrismSettleJob amount events (Funded /
		// Completed / Refunded / ArbitrationExecuted) carry the Job contract
		// in TokenAddr. The real escrow token is the per-job paymentToken in
		// the contract — resolve it via eth_call getJobPaymentToken(jobId) so
		// the getTokenInfo call below stores the correct symbol/decimals.
		if isPrismAmountEvent(string(ce.EventType)) {
			if tok, terr := l.fetchPaymentToken(ctx, client, ce.To); terr == nil && tok != "" {
				ce.TokenAddr = tok
			}
		}

		// Get block time from cache or RPC
		h, err := l.GetBlockHeader(ctx, client, logEntry.BlockNumber)
		if err != nil {
			logger.Warn("drop log: get block header failed",
				logger.String("tx_hash", logEntry.TxHash.Hex()),
				logger.Uint64("block", logEntry.BlockNumber),
				logger.Error(err))
			continue
		}

		// Get token info
		token, err := l.getTokenInfo(ctx, client, common.HexToAddress(ce.TokenAddr))
		if err != nil {
			logger.Warn("drop log: get token info failed",
				logger.String("tx_hash", logEntry.TxHash.Hex()),
				logger.Uint64("block", logEntry.BlockNumber),
				logger.String("token", ce.TokenAddr),
				logger.Error(err))
			continue
		}

		// Fill metadata
		ce.ChainName = l.chainConfig.ChainName
		ce.ChainType = l.chainConfig.ChainType
		ce.TxType = constant.TxTypeToken
		ce.Symbol = token.Symbol
		ce.Decimals = token.Decimals
		ce.BlockTime = h.Time
		ce.Status = 1

		chainEvents = append(chainEvents, ce)
	}

	return chainEvents, nil
}

// GetBlockHeader returns header from cache first, then RPC.
//
// P1-E: client may be nil (flushLastBlock passes nil to rely on cache).
// Previously a cache miss would nil-deref here and panic. Now we fall back
// to l.rpcClient.GetClient(chainName) when client is nil, returning an
// explicit error if the chain is unknown rather than panicking.
func (l *EVMListener) GetBlockHeader(ctx context.Context, client *ethclient.Client, num uint64) (*types.Header, error) {
	if h, ok := l.blockCache.Get(num); ok {
		return h, nil
	}

	// Cache miss: ensure we have a client. flushLastBlock passes nil because
	// the cache should normally be warm (syncBlockRange populated it). If the
	// cache was evicted (LRU pressure), fetch a client here.
	if client == nil {
		c, err := l.rpcClient.GetClient(l.chainConfig.ChainName)
		if err != nil {
			return nil, fmt.Errorf("get block header: no client and rpc get client failed: %w", err)
		}
		client = c
	}

	newH, err := client.HeaderByNumber(ctx, big.NewInt(int64(num)))
	if err != nil {
		return nil, err
	}

	l.blockCache.Add(num, newH)
	return newH, nil
}

// getTokenInfo returns token info from cache first, then chain
func (l *EVMListener) getTokenInfo(ctx context.Context, client *ethclient.Client, tokenAddr common.Address) (*model.TokenInfo, error) {
	addrHex := tokenAddr.Hex()
	chainName := l.chainConfig.ChainName

	// Try cache/DB first
	info, err := l.syncStateService.GetTokenInfo(ctx, chainName, addrHex)
	if err == nil {
		return info, nil
	}

	// Native coin
	if tokenAddr == common.HexToAddress("0xEeeeeEeeeEeEeeEeEeEeeEEEeeeeEeeeeeeeEEeE") {
		info := &model.TokenInfo{Symbol: "ETH", Decimals: 18}
		_ = l.syncStateService.SetTokenInfo(ctx, chainName, addrHex, info)
		return info, nil
	}

	// Fetch from chain
	newInfo, err := l.fetchTokenInfoFromChain(ctx, client, tokenAddr)
	if err != nil {
		return nil, err
	}
	_ = l.syncStateService.SetTokenInfo(ctx, chainName, addrHex, newInfo)
	return newInfo, nil
}

// fetchTokenInfoFromChain queries token metadata from chain
func (l *EVMListener) fetchTokenInfoFromChain(ctx context.Context, client *ethclient.Client, tokenAddr common.Address) (*model.TokenInfo, error) {
	abiToUse := l.contractABI

	// Fallback to standard ERC20 ABI if needed
	needFallback := false
	if abiToUse.Methods == nil {
		needFallback = true
	} else if _, hasSymbol := abiToUse.Methods["symbol"]; !hasSymbol {
		needFallback = true
	}

	if needFallback {
		fallbackABI, err := abi.JSON(strings.NewReader(standardERC20ABI))
		if err == nil {
			abiToUse = fallbackABI
		}
	}

	var symbol string
	decimals := uint8(18)

	// Get symbol
	symbolData, err := client.CallContract(ctx, ethereum.CallMsg{
		To:   &tokenAddr,
		Data: abiToUse.Methods["symbol"].ID,
	}, nil)

	if err == nil && len(symbolData) > 0 {
		if len(symbolData) == 32 {
			symbol = string(bytes.Trim(symbolData, "\x00"))
		} else {
			_ = abiToUse.UnpackIntoInterface(&symbol, "symbol", symbolData)
		}
	}
	if symbol == "" {
		symbol = "UNKNOWN"
	}

	// Get decimals
	decimalData, err := client.CallContract(ctx, ethereum.CallMsg{
		To:   &tokenAddr,
		Data: abiToUse.Methods["decimals"].ID,
	}, nil)

	if err == nil && len(decimalData) >= 32 {
		decimals = uint8(new(big.Int).SetBytes(decimalData).Uint64())
	}
	if decimals < 1 || decimals > 18 {
		decimals = 18
	}

	return &model.TokenInfo{
		Symbol:   symbol,
		Decimals: decimals,
	}, nil
}

// BatchSaveChainEvents saves events with distributed lock.
//
// Lock acquisition is retried with bounded backoff because a transient lock
// failure (e.g. another instance mid-flush) must NOT break the ordered-commit
// chain in tryCommitAll — a returned error would leave nextExpected stalled
// until a future reorg rollback unblocks it, causing a silent data gap.
func (l *EVMListener) BatchSaveChainEvents(ctx context.Context, chainEvents []*model.ChainEvent) error {
	start := time.Now()
	defer utils.LogPanic()

	key := fmt.Sprintf("%s:%s:%s", constant.KeyBatchSaveLock, l.chainConfig.ChainName, "multi")
	expire := time.Duration(l.chainConfig.LockExpirationSeconds) * time.Second

	const maxLockRetries = 3
	baseBackoff := 200 * time.Millisecond

	var value string
	var err error
	for attempt := 0; attempt < maxLockRetries; attempt++ {
		value, err = l.lockService.AcquireLock(ctx, key, expire)
		if err == nil {
			break
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		logger.Warn("acquire lock failed, retrying",
			logger.Int("attempt", attempt+1),
			logger.Int("max", maxLockRetries),
			logger.Error(err))
		// Linear backoff (with jitter cap). Bounded retries keep the commit
		// chain moving without holding callers indefinitely.
		select {
		case <-time.After(time.Duration(attempt+1) * baseBackoff):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	if err != nil {
		logger.Errorf("acquire lock exhausted retries", logger.Error(err))
		return fmt.Errorf("acquire batch-save lock after %d retries: %w", maxLockRetries, err)
	}
	defer l.lockService.ReleaseLock(ctx, key, value)

	if err := l.eventIngestService.BatchSaveChainEvents(ctx, chainEvents); err != nil {
		logger.Errorf("batch save failed", logger.Error(err))
		return err
	}

	logger.Debug("batch save success", logger.Int("count", len(chainEvents)), logger.Cost(start))
	return nil
}

// GetBlockNumber gets latest block number with retry
func (l *EVMListener) GetBlockNumber(ctx context.Context, chainName string) (uint64, error) {
	var bn uint64
	err := utils.Retry(ctx, l.chainConfig.MaxSyncRetries, 5*time.Second, func() error {
		client, err := l.rpcClient.GetClient(chainName)
		if err != nil {
			return err
		}
		n, err := client.BlockNumber(ctx)
		if err != nil {
			return err
		}
		bn = n
		return nil
	})
	return bn, err
}

// detectAndHandleReorg checks reorg by block hash, rolls back data, purges cache
func (l *EVMListener) detectAndHandleReorg(ctx context.Context, dbLastBlock uint64, dbLastBlockHash string) (int, error) {
	client, err := l.rpcClient.GetClient(l.chainConfig.ChainName)
	if err != nil {
		return 0, err
	}

	// Get real-time header (NO CACHE)
	chainHeader, err := client.HeaderByNumber(ctx, big.NewInt(int64(dbLastBlock)))
	if err != nil {
		// Surface RPC errors instead of swallowing them as "no reorg".
		// Treating RPC failure as "no reorg" would let the listener continue
		// syncing from a potentially stale state, silently corrupting data.
		logger.Errorf("get header for reorg check failed",
			logger.Uint64("block", dbLastBlock), logger.Error(err))
		return 0, fmt.Errorf("get header for reorg check: %w", err)
	}

	chainHash := chainHeader.Hash().Hex()

	// No reorg
	if strings.EqualFold(strings.ToLower(chainHash), strings.ToLower(dbLastBlockHash)) {
		return 0, nil
	}

	logger.Warn("REORG DETECTED",
		logger.Uint64("block", dbLastBlock),
		logger.String("db_hash", dbLastBlockHash),
		logger.String("chain_hash", chainHash))

	// Purge entire cache to eliminate stale data
	l.blockCache.Purge()
	logger.Info("block cache purged due to reorg")

	// Calculate rollback depth
	rollbackDepth := l.chainConfig.Confirmations
	minReorgDepth := l.chainConfig.ReorgMinDepth

	if rollbackDepth < minReorgDepth {
		rollbackDepth = minReorgDepth
	}
	if int(dbLastBlock) < rollbackDepth {
		rollbackDepth = int(dbLastBlock)
	}

	startRollbackBlock := dbLastBlock - uint64(rollbackDepth) + 1

	logger.Info("rolling back events",
		logger.Uint64("from", startRollbackBlock),
		logger.Uint64("to", dbLastBlock))

	// Roll back invalid data for this single contract
	rows, err := l.eventIngestService.RollbackEvents(ctx, l.chainConfig.ChainName, l.chainConfig.ContractAddr, startRollbackBlock, dbLastBlock)
	if err != nil {
		logger.Errorf("rollback failed for contract %s", logger.String("contract", l.chainConfig.ContractAddr), logger.Error(err))
		return 0, fmt.Errorf("rollback failed for contract %s: %w", l.chainConfig.ContractAddr, err)
	}

	// Phase 9: persist the reorg for the /perf/reorg-feed endpoint (best-effort).
	// P1-F: rolledBackRows is now the real count returned by RollbackEvents,
	// not a hard-coded 0. Failures are logged but do not propagate — the
	// reorg has already been handled, only the audit row is missing.
	if l.reorgRepo != nil {
		if err := l.reorgRepo.Insert(
			ctx,
			l.chainConfig.ChainName,
			l.chainConfig.ContractAddr,
			startRollbackBlock,
			dbLastBlock,
			dbLastBlockHash,
			chainHash,
			rollbackDepth,
			rows,
		); err != nil {
			logger.Errorf("persist reorg_event failed (non-fatal)", logger.Error(err))
		}
	}

	// Bump in-memory reorg counter for the /health endpoint.
	if l.healthTracker != nil {
		l.healthTracker.IncReorg()
	}

	return rollbackDepth, nil
}

// flushLastBlock persists latest synced block and its hash.
//
// The header for lastBlock was fetched & cached during syncBlockRange via
// GetBlockHeader, so we reuse the cache here instead of issuing another RPC.
// Only on cache miss do we fall back to a fresh HeaderByNumber. This removes
// one RPC call per committed block range from the hot path.
//
// P1-D: flushLastBlock is only called from tryCommitAll, which already holds
// taskMutex. So lastBlock is stable for the duration of this call — no
// extra locking needed here.
func (l *EVMListener) flushLastBlock(ctx context.Context) error {
	current := l.lastBlock

	if current == 0 || ctx.Err() != nil {
		return ctx.Err()
	}

	// Cache hit is the common path: syncBlockRange already populated it.
	header, err := l.GetBlockHeader(ctx, nil, current)
	if err != nil {
		// Cache miss: fall back to a fresh RPC.
		client, cerr := l.rpcClient.GetClient(l.chainConfig.ChainName)
		if cerr != nil {
			logger.Errorf("flush get rpc client failed", logger.Uint64("block", current), logger.Error(cerr))
			return cerr
		}
		header, err = client.HeaderByNumber(ctx, big.NewInt(int64(current)))
		if err != nil {
			logger.Errorf("flush get header failed", logger.Uint64("block", current), logger.Error(err))
			return err
		}
	}

	return l.syncStateService.SetLastBlock(
		ctx,
		l.chainConfig.ChainName,
		l.chainConfig.ContractAddr,
		current,
		header.Hash().Hex(),
	)
}

// Stop gracefully shuts down the listener. It is safe to call multiple times.
func (l *EVMListener) Stop(ctx context.Context) error {
	l.stopOnce.Do(func() {
		if l.cancelFunc != nil {
			l.cancelFunc()
		}
	})
	return nil
}
