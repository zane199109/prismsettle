package chainbinding

import (
	"context"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/zane/web3-offchain/internal/repository"
	"github.com/zane/web3-offchain/model"
	"github.com/zane/web3-offchain/pkg/logger"
	"github.com/zane/web3-offchain/prismsettle/evaluator"
)

// EventSourceConfig configures the DB-backed EventSource implementation.
type EventSourceConfig struct {
	ChainName    string            // e.g. "monad_testnet"
	JobContract  string            // PrismSettleJob contract address (lowercase)
	HookContract string            // ArbitrationHook contract address (lowercase)
	RegContract  string            // PrismSettleRegistry contract address (lowercase)
	EthClient    *ethclient.Client // optional: for on-chain queries (e.g. getJobState)
}

// ChainEventSource implements evaluator.EventSource by reading from the
// indexer's chain_events table. It is read-only and does not need an
// ethclient for the main path; CurrentScore falls back to the on-chain
// Registry view if the offchain mirror has no Aggregated event yet.
type ChainEventSource struct {
	cfg          EventSourceConfig
	repo         *repository.ChainEventRepository
	agentDB      *repository.AgentRegistryRepository
	scoreFetcher ScoreFetcher  // optional; nil = offchain-only
	jobContract  boundContract // bound to PrismSettleJob for getJobState queries
}

// ScoreFetcher reads the live aggregated score from the Registry contract.
// Implemented by *RegistryAggregatorBinding.GetScore.
type ScoreFetcher interface {
	GetScore(ctx context.Context, agentID *big.Int) (uint64, error)
}

// NewChainEventSource builds an EventSource backed by the indexer DB.
//
// scoreFetcher is optional — pass nil to use offchain-only score reads
// (latest Aggregated event). When the Evaluator is wired in main.go, pass
// the RegistryAggregatorBinding so CurrentScore reads live on-chain state.
func NewChainEventSource(
	cfg EventSourceConfig,
	repo *repository.ChainEventRepository,
	agentDB *repository.AgentRegistryRepository,
	scoreFetcher ScoreFetcher,
) *ChainEventSource {
	s := &ChainEventSource{
		cfg:          cfg,
		repo:         repo,
		agentDB:      agentDB,
		scoreFetcher: scoreFetcher,
	}
	// Initialize the job contract binding if an ethclient is available.
	// This is used by RecentDisputed to query getJobState for the
	// deliverableHash (P1-3 fix).
	if cfg.EthClient != nil && cfg.JobContract != "" {
		addr := common.HexToAddress(cfg.JobContract)
		bc, err := newBoundContract(addr, jobABI, cfg.EthClient)
		if err == nil {
			s.jobContract = bc
		} else {
			logger.Warn("event_source: failed to bind job contract for getJobState",
				logger.String("address", cfg.JobContract), logger.Error(err))
		}
	}
	return s
}

// Compile-time assertion: ChainEventSource implements evaluator.EventSource.
var _ evaluator.EventSource = (*ChainEventSource)(nil)

// RecentDisputed returns Disputed jobs seen since `since`, newest first.
//
// Storage convention (prismsettle_hook_parser.go):
//   - To        = jobId
//   - TokenAddr = reasonHash
func (s *ChainEventSource) RecentDisputed(ctx context.Context, since time.Time, limit int) ([]evaluator.DisputedJob, error) {
	if limit <= 0 {
		limit = 20
	}
	events, _, err := s.repo.GetPrismEvents(
		ctx, s.cfg.ChainName, s.cfg.HookContract, "", "", 0, 1, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("query disputed events: %w", err)
	}
	out := make([]evaluator.DisputedJob, 0, len(events))
	for _, ev := range events {
		if ev.EventType != model.TypePrismDisputed {
			continue
		}
		if ev.BlockTime > 0 && time.Unix(int64(ev.BlockTime), 0).Before(since) {
			continue
		}
		provider, perr := s.ProviderFor(ctx, ev.To)
		if perr != nil {
			logger.Warn("event_source: provider lookup failed",
				logger.String("job_id", ev.To), logger.Error(perr))
		}
		buyer := s.buyerFor(ctx, ev.To)

		// Query the on-chain job state for the deliverableHash, which is
		// not included in the Disputed event (P1-3 fix).
		deliverableHash := s.getDeliverableHash(ctx, ev.To)

		out = append(out, evaluator.DisputedJob{
			JobID:           ev.To,
			Provider:        provider,
			Buyer:           buyer,
			DeliverableHash: deliverableHash,
			ReasonHash:      ev.TokenAddr,
		})
	}
	return out, nil
}

// ProviderFor returns the provider address for a job by scanning the
// Assigned event (From = provider). Empty string means "unknown / not yet
// assigned".
func (s *ChainEventSource) ProviderFor(ctx context.Context, jobID string) (string, error) {
	events, _, err := s.repo.GetPrismEvents(
		ctx, s.cfg.ChainName, s.cfg.JobContract, jobID, "", 0, 1, 50,
	)
	if err != nil {
		return "", fmt.Errorf("query assigned events: %w", err)
	}
	for _, ev := range events {
		if ev.EventType == model.TypePrismJobAssigned {
			return strings.ToLower(ev.From), nil
		}
	}
	return "", nil
}

// buyerFor returns the buyer address for a job by scanning the JobCreated
// event (From = buyer per prismsettle_job_parser.go).
func (s *ChainEventSource) buyerFor(ctx context.Context, jobID string) string {
	events, _, err := s.repo.GetPrismEvents(
		ctx, s.cfg.ChainName, s.cfg.JobContract, jobID, "", 0, 1, 50,
	)
	if err != nil {
		return ""
	}
	for _, ev := range events {
		if ev.EventType == model.TypePrismJobCreated {
			return strings.ToLower(ev.From)
		}
	}
	return ""
}

// getDeliverableHash queries the PrismSettleJob.getJobState(jobId) view to
// extract the deliverableHash (5th return value, index 4). Returns empty
// string if the job contract binding is unavailable or the call fails.
func (s *ChainEventSource) getDeliverableHash(ctx context.Context, jobID string) string {
	if s.jobContract.bind == nil {
		return ""
	}
	id := new(big.Int)
	if _, ok := id.SetString(jobID, 10); !ok {
		return ""
	}
	var outs []interface{}
	if err := s.jobContract.bind.Call(&bind.CallOpts{Context: ctx}, &outs, "getJobState", id); err != nil {
		logger.Warn("event_source: getJobState call failed",
			logger.String("job_id", jobID), logger.Error(err))
		return ""
	}
	// getJobState returns 10 fields; field index 4 = deliverableHash (bytes32).
	if len(outs) < 5 {
		return ""
	}
	hash, ok := outs[4].([32]byte)
	if !ok {
		return ""
	}
	return common.BytesToHash(hash[:]).Hex()
}

// AgentIDFor returns the Registry agentId for a provider address by scanning
// the agent_registry table (Owner = provider). Returns nil if not found.
func (s *ChainEventSource) AgentIDFor(ctx context.Context, providerAddr string) (*big.Int, error) {
	if providerAddr == "" {
		return nil, fmt.Errorf("event_source: provider address is empty")
	}
	// The agent_registry table stores Owner = From (provider). We need the
	// AgentID (To column). List returns all rows (small agent counts; for
	// large deployments add an Owner index — Phase 10 optimization).
	records, err := s.agentDB.List(ctx, s.cfg.ChainName)
	if err != nil {
		return nil, fmt.Errorf("event_source: list agents: %w", err)
	}
	for _, r := range records {
		if strings.EqualFold(r.Owner, providerAddr) {
			id := new(big.Int)
			// SetString returns (nil, false) on parse failure; without
			// this check a malformed AgentID would silently become 0 and
			// the Evaluator would call submitValidation with agentId=0,
			// which either reverts on-chain or corrupts reputation data.
			if _, ok := id.SetString(strings.TrimPrefix(r.AgentID, "0x"), 16); !ok {
				return nil, fmt.Errorf("event_source: invalid agent id hex %q for provider %s", r.AgentID, providerAddr)
			}
			return id, nil
		}
	}
	return nil, fmt.Errorf("event_source: agent not registered for provider %s", providerAddr)
}

// CurrentScore returns the provider's latest aggregated reputation score.
// Prefers on-chain view (via ScoreFetcher) for freshness; falls back to the
// latest Aggregated ChainEvent if the on-chain call fails.
func (s *ChainEventSource) CurrentScore(ctx context.Context, agentID *big.Int) (uint64, error) {
	if s.scoreFetcher != nil {
		score, err := s.scoreFetcher.GetScore(ctx, agentID)
		if err == nil {
			return score, nil
		}
		logger.Warn("event_source: on-chain score fetch failed, falling back to offchain",
			logger.Error(err))
	}
	// Fallback: read the latest Aggregated event for this agent.
	agentHex := toUint256Hex(agentID)
	events, _, err := s.repo.GetPrismEvents(
		ctx, s.cfg.ChainName, s.cfg.RegContract, agentHex, "", 0, 1, 1,
	)
	if err != nil {
		return 0, fmt.Errorf("event_source: query aggregated events: %w", err)
	}
	for _, ev := range events {
		if ev.EventType == model.TypePrismAggregated {
			score := new(big.Int)
			if _, ok := score.SetString(ev.Value, 10); ok {
				return score.Uint64(), nil
			}
		}
	}
	return 0, nil
}

// toUint256Hex formats a *big.Int as a 0x-prefixed 32-byte hex string,
// matching the parser's storage convention for agentId/jobId in the To column.
func toUint256Hex(id *big.Int) string {
	return common.BytesToHash(id.Bytes()).Hex()
}
