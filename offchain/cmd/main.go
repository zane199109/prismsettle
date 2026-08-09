package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"math/big"
	"net/http"
	_ "net/http/pprof" // import pprof to analyze the performance
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/go-playground/validator/v10"
	"github.com/zane/web3-offchain/internal/api"
	"github.com/zane/web3-offchain/internal/listener"
	"github.com/zane/web3-offchain/internal/middleware"
	"github.com/zane/web3-offchain/internal/repository"
	"github.com/zane/web3-offchain/internal/router"
	"github.com/zane/web3-offchain/internal/service"
	"github.com/zane/web3-offchain/internal/storage"
	"github.com/zane/web3-offchain/model"
	"github.com/zane/web3-offchain/pkg/config"
	"github.com/zane/web3-offchain/pkg/logger"
	prism_api "github.com/zane/web3-offchain/prismsettle/api"
	"github.com/zane/web3-offchain/prismsettle/chainbinding"
	"github.com/zane/web3-offchain/prismsettle/demo"
	"github.com/zane/web3-offchain/prismsettle/evaluator"
	"github.com/zane/web3-offchain/prismsettle/keeper"
	prism_svc "github.com/zane/web3-offchain/prismsettle/service"
)

var configPath = flag.String("config", "../config/prod.yaml", "config file path")

// loadDotEnv reads the project .env (KEY=VALUE lines) into the process
// environment so secrets (BUYER_KEY, AUDITOR_*_KEY, PRISM_EVALUATOR_KEY,
// DEPLOYER_KEY, OPENAI_API_KEY) don't need to be exported manually on every
// start. Existing environment variables win; the file is git-ignored.
func loadDotEnv() {
	for _, p := range []string{"./.env", "../.env"} {
		f, err := os.Open(p)
		if err != nil {
			continue
		}
		sc := bufio.NewScanner(f)
		for sc.Scan() {
			line := strings.TrimSpace(sc.Text())
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			key, val, ok := strings.Cut(line, "=")
			if !ok {
				continue
			}
			key = strings.TrimSpace(key)
			if os.Getenv(key) == "" {
				_ = os.Setenv(key, strings.TrimSpace(val))
			}
		}
		_ = f.Close()
		logger.Info("loaded .env", logger.String("path", p))
		return
	}
	logger.Warn("no .env file found; secrets must come from the environment")
}

func main() {
	loadDotEnv()

	// 1. parse flags
	flag.Parse()
	// 2. initialize config
	if err := config.InitConfig(*configPath); err != nil {
		fmt.Fprintf(os.Stderr, "init config failed: %v\n", err)
		os.Exit(1)
	}

	// 3. initialize logger
	logger.InitLogger()
	logger.Info("start web3-offchain service",
		logger.String("env", config.Cfg.Env),
		logger.String("config_path", *configPath),
	)

	// if  Debug mode,start pprof
	if config.Cfg.Server.Mode == "debug" {
		go func() {
			logger.Info("pprof started on port :6060")
			if err := http.ListenAndServe(":6060", nil); err != nil {
				logger.Errorf("pprof server failed", logger.Error(err))
			}
		}()
	}

	// 4. initialize pgstorage
	pgStorage, err := storage.NewPGStorage()
	if err != nil {
		logger.Fatal("init pg storage failed", logger.Error(err))
	}
	defer func() {
		if err := pgStorage.Close(); err != nil {
			logger.Errorf("pg storage close failed", logger.Error(err))
		}
	}()

	// 5. initialize Redis
	redisStorage, err := storage.NewRedisStorage()
	if err != nil {
		logger.Fatal("init redis storage failed", logger.Error(err))
	}
	defer func() {
		if err := redisStorage.Close(); err != nil {
			logger.Errorf("redis storage close failed", logger.Error(err))
		}
	}()

	// 6. initialize Repository
	chainEventRepo := repository.NewChainEventRepository(pgStorage.DB())
	blockStateRepo := repository.NewBlockStateRepository(pgStorage.DB())
	agentRegistryRepo := repository.NewAgentRegistryRepository(pgStorage.DB())
	trustThresholdRepo := repository.NewTrustThresholdRepository(pgStorage.DB())
	reorgEventRepo := repository.NewReorgEventRepository(pgStorage.DB()) // Phase 9 task 9.4
	perfResultRepo := repository.NewPerfResultRepository(pgStorage.DB()) // Phase 9 task 9.6
	grabAttemptRepo := repository.NewGrabAttemptRepository(pgStorage.DB())

	// 7. create a root context for the entire application lifecycle
	rootCtx, rootCancel := context.WithCancel(context.Background())
	defer rootCancel() // defer cancel the root context when the application is shutting down

	// 8. database tables initialization with auto-migration
	if err := pgStorage.AutoMigrate(rootCtx); err != nil {
		logger.Fatal("create table failed", logger.Error(err))
	}

	// 9.initialize services
	lockService := service.NewLockService(redisStorage.GetRedisClient())
	syncStateService := service.NewSyncStateService(chainEventRepo, blockStateRepo, redisStorage)
	chainEventService := service.NewEventIngestServiceWithAgentRepo(chainEventRepo, agentRegistryRepo)

	// Phase 9: HealthTracker collects runtime status from listener,
	// Evaluator, and Keeper for the /health endpoint (NFR-OBS02).
	healthTracker := prism_svc.NewHealthTracker()
	erc20Service := service.NewERC20Service(chainEventRepo, redisStorage, syncStateService)
	prismService := prism_svc.NewPrismSettleServiceWithPerfRepos(
		chainEventRepo, agentRegistryRepo, trustThresholdRepo,
		reorgEventRepo, perfResultRepo, grabAttemptRepo,
	)

	// Wire the live on-chain score fetcher: seed agents hold their score in
	// the registry contract but emit no aggregation event, so listAgents
	// falls back to an eth_call when the DB has nothing.
	for _, chain := range config.Cfg.Web3.Chains {
		if len(chain.RPCUrls) > 0 && chain.ContractAddr != "" {
			if fetcher, err := prism_svc.NewScoreFetcher(chain.RPCUrls[0], chain.ContractAddr); err == nil {
				prismService.SetScoreFetcher(fetcher)
				defer fetcher.Close()
				logger.Info("score fetcher wired", logger.String("chain", chain.ChainName))
				break
			}
		}
	}

	// Allow PRISM_EVALUATOR_KEY env var to override the yaml config value.
	// This lets us keep the private key out of the version-controlled config file.
	// The docker-compose.yml passes PRISM_EVALUATOR_KEY from .env.
	if envKey := os.Getenv("PRISM_EVALUATOR_KEY"); envKey != "" {
		config.Cfg.Evaluator.PrivateKey = envKey
	}

	// 9b. initialize Evaluator + Keeper (Phase 6 bots, wired in Phase 9 P0-A).
	//
	// The bots are only started when evaluator.registry_addr is configured.
	// In dev without contract addresses, the bots stay disabled and the
	// /health endpoint reports evaluator_state=stopped, keeper_last_run=0.
	var (
		botErrChan = make(chan error, 2) // buffered; Evaluator + Keeper
		botWg      sync.WaitGroup
	)
	if config.Cfg.Evaluator.RegistryAddr != "" {
		evCfg := config.Cfg.Evaluator

		// Connect to the PrismSettle chain RPC for signing txs.
		ethClient, err := ethclient.Dial(evCfg.RPCURL)
		if err != nil {
			logger.Fatal("connect PrismSettle RPC failed",
				logger.String("rpc", evCfg.RPCURL), logger.Error(err))
		}

		// Build the shared transactor (Evaluator + Keeper share the same key).
		auth, err := chainbinding.NewTransactor(chainbinding.TransactorConfig{
			PrivateKeyHex: evCfg.PrivateKey,
			ChainID:       new(big.Int).SetInt64(evCfg.ChainID),
		}, ethClient)
		if err != nil {
			logger.Fatal("build evaluator transactor failed", logger.Error(err))
		}

		// Construct the 4 chainbinding implementations.
		regAddr := common.HexToAddress(evCfg.RegistryAddr)
		hookAddr := common.HexToAddress(evCfg.HookAddr)

		registryAggregator, err := chainbinding.NewRegistryAggregator(
			chainbinding.RegistryAggregatorConfig{
				ChainName:   evCfg.ChainName,
				RegContract: evCfg.RegistryAddr,
			},
			regAddr, ethClient, auth, agentRegistryRepo,
		)
		if err != nil {
			logger.Fatal("build registry aggregator failed", logger.Error(err))
		}

		registryWriter, err := chainbinding.NewRegistryWriter(regAddr, ethClient, auth)
		if err != nil {
			logger.Fatal("build registry writer failed", logger.Error(err))
		}

		hookResolver, err := chainbinding.NewHookResolver(hookAddr, ethClient, auth)
		if err != nil {
			logger.Fatal("build hook resolver failed", logger.Error(err))
		}

		eventSource := chainbinding.NewChainEventSource(
			chainbinding.EventSourceConfig{
				ChainName:    evCfg.ChainName,
				JobContract:  evCfg.JobAddr,
				HookContract: evCfg.HookAddr,
				RegContract:  evCfg.RegistryAddr,
			},
			chainEventRepo, agentRegistryRepo, registryAggregator, // RegistryAggregator implements ScoreFetcher
		)

		// DecisionStore: GORM-backed, shares the main DB. AutoMigrate the
		// decision_logs table if not already created.
		decisionStore := evaluator.NewGormDecisionStore(pgStorage.DB())
		if err := pgStorage.DB().AutoMigrate(&evaluator.DecisionLog{}); err != nil {
			logger.Fatal("automigrate decision_logs failed", logger.Error(err))
		}

		// Evaluator: polls Disputed events and resolves disputes via
		// resolveDispute / setAggregatedScore.
		evaluatorInst, err := evaluator.NewEvaluator(
			evaluator.Config{
				PollInterval:     time.Duration(evCfg.PollInterval) * time.Second,
				BatchSize:        evCfg.BatchSize,
				EvalEndpoint:     "", // TODO: wire from config when eval agent is deployed
				BreakerThreshold: 10,
				BreakerCooldown:  60 * time.Second,
			},
			eventSource,
			registryWriter,
			hookResolver,
			decisionStore,
			eventSource.CurrentScore, // ChainEventSource implements CurrentScore
		)
		if err != nil {
			logger.Fatal("init evaluator failed", logger.Error(err))
		}

		// Keeper: ticks every 30s to aggregate epochs + scan inactive agents.
		keeperInst, err := keeper.NewKeeper(
			keeper.Config{
				TickPeriod:  time.Duration(evCfg.KeeperTickSec) * time.Second,
				ScanPeriod:  time.Hour,
				InactiveAge: 30 * 24 * time.Hour,
				BatchSize:   10,
			},
			registryAggregator,
		)
		if err != nil {
			logger.Fatal("init keeper failed", logger.Error(err))
		}
		keeperInst.SetHealthTracker(healthTracker)

		// Start both bots. They share the root context for graceful shutdown.
		botWg.Add(2)
		go func() {
			defer botWg.Done()
			healthTracker.SetEvaluatorState("running")
			defer healthTracker.SetEvaluatorState("stopped")
			if err := evaluatorInst.Run(rootCtx); err != nil && rootCtx.Err() == nil {
				logger.Errorf("evaluator stopped unexpectedly", logger.Error(err))
				select {
				case botErrChan <- fmt.Errorf("evaluator fatal: %w", err):
				default:
				}
			}
		}()
		go func() {
			defer botWg.Done()
			if err := keeperInst.Run(rootCtx); err != nil && rootCtx.Err() == nil {
				logger.Errorf("keeper stopped unexpectedly", logger.Error(err))
				select {
				case botErrChan <- fmt.Errorf("keeper fatal: %w", err):
				default:
				}
			}
		}()
		logger.Info("evaluator + keeper started",
			logger.String("chain", evCfg.ChainName),
			logger.Int64("chain_id", evCfg.ChainID))
	} else {
		logger.Info("evaluator + keeper disabled (no PrismSettle contract addresses configured)")
	}

	// 10. start listeners for each chain in config
	var wg sync.WaitGroup

	// add buffer to prevent blocking
	listenerErrChan := make(chan error, len(config.Cfg.Web3.Chains))

	for _, chain := range config.Cfg.Web3.Chains {
		if chain.ChainType == "evm" {
			l, err := listener.NewEVMListener(chain, syncStateService, chainEventService, lockService)
			if err != nil {

				// if chain is critical chain should be fatal, but if it's a testnet, maybe just log and skip
				logger.Fatal("create listener failed", logger.String("chain", chain.ChainName), logger.Error(err))
			}

			// Phase 9 P2-K: NewEVMListener returns *EVMListener (not the
			// ChainListener interface), so we can wire Phase 9 hooks directly
			// without a type assertion. *EVMListener still satisfies the
			// ChainListener interface passed to the goroutine below.
			l.SetReorgRepo(reorgEventRepo)
			l.SetHealthTracker(healthTracker)

			wg.Add(1)
			go func(lst listener.ChainListener, name string) {
				defer wg.Done()
				logger.Info("listener started", logger.String("chain", name))

				if err := lst.Start(rootCtx); err != nil {

					//only consider it an abnormal error when the Context is not cancelled
					if rootCtx.Err() == nil {
						logger.Errorf("listener stopped unexpectedly", logger.String("chain", name), logger.Error(err))
						// send error, let the main goroutine decide whether to exit
						select {
						case listenerErrChan <- fmt.Errorf("listener [%s] fatal: %w", name, err):
						default:
						}
					}
				}
			}(l, chain.ChainName)
		}
	}
	validator := validator.New()

	// 11. init handlers
	erc20Handler := api.NewERC20Handler(erc20Service, validator)
	prismHandler := prism_api.NewPrismSettleHandler(prismService, validator)

	// Demo orchestrator: chat-style agent collaboration playback. Wired only
	// when the evaluator chain binding (job + hook) is configured and the
	// signing keys are injected via env (BUYER_KEY / AUDITOR_SENIOR_KEY /
	// PRISM_EVALUATOR_KEY / DEPLOYER_KEY). Missing keys → demo disabled.
	var demoHandler *prism_api.DemoHandler
	if evCfg := config.Cfg.Evaluator; evCfg.JobAddr != "" && evCfg.HookAddr != "" &&
		evCfg.PaymentToken != "" && os.Getenv("BUYER_KEY") != "" {
		if err := pgStorage.DB().AutoMigrate(&model.DemoSession{}, &model.DemoMessage{}); err != nil {
			logger.Fatal("automigrate demo tables failed", logger.Error(err))
		}
		demoRepo := demo.NewSessionRepo(pgStorage.DB())
		demoActions, err := demo.NewActions(demo.ActionsConfig{
			RPCURL:       evCfg.RPCURL,
			ChainID:      evCfg.ChainID,
			JobAddr:      evCfg.JobAddr,
			HookAddr:     evCfg.HookAddr,
			TokenAddr:    evCfg.PaymentToken,
			WmonAddr:     evCfg.WmonToken,
			BuyerKey:     os.Getenv("BUYER_KEY"),
			ProviderKey:  os.Getenv("AUDITOR_SENIOR_KEY"),
			JuniorKey:    os.Getenv("AUDITOR_JUNIOR_KEY"),
			RookieKey:    os.Getenv("AUDITOR_ROOKIE_KEY"),
			EvaluatorKey: os.Getenv("PRISM_EVALUATOR_KEY"),
			DeployerKey:  os.Getenv("DEPLOYER_KEY"),
		})
		if err != nil {
			logger.Fatal("init demo actions", logger.Error(err))
		}
		defer demoActions.Close()
		llmClient := demo.NewLLMClient("", os.Getenv("OPENAI_API_KEY"), "deepseek-v4-flash")
		announcementWait := time.Duration(evCfg.AnnouncementWaitSec) * time.Second
		orch := demo.NewOrchestrator(
			demoRepo, demoActions, llmClient,
			demoActions.BuyerAgentID(), demoActions.ProviderAgentID(), demoActions.EvaluatorAgentID(),
			400*time.Millisecond,
			announcementWait,
		)
		demoHandler = prism_api.NewDemoHandler(orch)
		logger.Info("demo orchestrator wired",
			logger.String("job", evCfg.JobAddr),
			logger.String("hook", evCfg.HookAddr),
			logger.String("token", evCfg.PaymentToken))
	} else {
		logger.Warn("demo orchestrator disabled: job/hook/token or BUYER_KEY missing")
	}

	// 12. init middleware and router
	routables := []router.Routable{
		erc20Handler,
		prismHandler,
	}
	if demoHandler != nil {
		routables = append(routables, demoHandler)
	}

	middleware.Init(*config.Cfg)
	middleware.StartLimiterCleanup()

	r := router.NewRouter(*config.Cfg, routables, healthTracker)

	// 13. start server
	serverAddr := ":" + config.Cfg.Server.Port
	logger.Info("server start on port",
		logger.String("port", config.Cfg.Server.Port),
		logger.String("mode", config.Cfg.Server.Mode))

	server := &http.Server{
		Addr:         serverAddr,
		Handler:      r,
		ReadTimeout:  time.Duration(config.Cfg.Server.ReadTimeout) * time.Second,
		WriteTimeout: time.Duration(config.Cfg.Server.WriteTimeout) * time.Second,
		IdleTimeout:  time.Duration(config.Cfg.Server.IdleTimeout) * time.Second,
	}

	serverErr := make(chan error, 1)
	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			serverErr <- err
		}
		close(serverErr)
	}()

	// 14. handle exit
	quit := make(chan os.Signal, 1)
	// signal.Notify registers the given channel to receive notifications of the specified signals.
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	// 15. wait for exit
	select {
	case <-quit:
		logger.Info("received shutdown signal, starting graceful exit")
	case err := <-serverErr:
		//server error cansel listeners and shutdown server
		logger.Errorf("http server fatal, initiating shutdown", logger.Error(err))
		rootCancel()
	case err := <-listenerErrChan:
		// listener error cansel listeners and shutdown server
		logger.Errorf("listener fatal error, initiating shutdown", logger.Error(err))
		rootCancel()
	case err := <-botErrChan:
		// Evaluator or Keeper fatal error — cancel everything and shutdown.
		logger.Errorf("bot fatal error, initiating shutdown", logger.Error(err))
		rootCancel()
	}

	// ===================== product level shutdown =====================
	//
	//1. rootCancel
	if rootCtx.Err() == nil {
		rootCancel()
	}

	// 2. wait to stop listeners
	logger.Info("waiting for listeners to exit...")
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		logger.Info("all listeners exited")
	case <-time.After(time.Duration(config.Cfg.Server.ShutdownListenerTimeout) * time.Second):
		logger.Warn("listener shutdown timeout, forcing exit")
	}

	// 2b. wait for Evaluator + Keeper to exit (they share rootCtx).
	botDone := make(chan struct{})
	go func() {
		botWg.Wait()
		close(botDone)
	}()
	select {
	case <-botDone:
		logger.Info("all bots exited")
	case <-time.After(time.Duration(config.Cfg.Server.ShutdownListenerTimeout) * time.Second):
		logger.Warn("bot shutdown timeout, forcing exit")
	}

	// 3. stop http server
	logger.Info("stopping http server...")
	ctxHttp, cancelHttp := context.WithTimeout(context.Background(), time.Duration(config.Cfg.Server.ShutdownHttpserverTimeout)*time.Second)
	defer cancelHttp()
	if err := server.Shutdown(ctxHttp); err != nil {
		logger.Errorf("http server forced to close", logger.Error(err))
	}

	logger.Info("service exited successfully")
}
