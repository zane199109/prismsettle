package main

import (
	"context"
	"flag"
	"fmt"
	"math/big"
	"net/http"
	_ "net/http/pprof" // import pprof to analyze the performance
	"os"
	"os/signal"
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
	"github.com/zane/web3-offchain/pkg/config"
	"github.com/zane/web3-offchain/pkg/logger"
	prism_api "github.com/zane/web3-offchain/prismsettle/api"
	"github.com/zane/web3-offchain/prismsettle/chainbinding"
	"github.com/zane/web3-offchain/prismsettle/evaluator"
	"github.com/zane/web3-offchain/prismsettle/keeper"
	prism_svc "github.com/zane/web3-offchain/prismsettle/service"
)

var configPath = flag.String("config", "../config/prod.yaml", "config file path")

func main() {
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
		reorgEventRepo, perfResultRepo,
	)

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

		// Construct the 5 chainbinding implementations.
		regAddr := common.HexToAddress(evCfg.RegistryAddr)
		jobAddr := common.HexToAddress(evCfg.JobAddr)
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

		jobCompleter, err := chainbinding.NewJobCompleter(jobAddr, ethClient, auth)
		if err != nil {
			logger.Fatal("build job completer failed", logger.Error(err))
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

		// Evaluator: polls Submitted + Disputed events, calls complete /
		// submitValidation / resolveDispute.
		evaluatorInst, err := evaluator.NewEvaluator(
			evaluator.Config{
				PollInterval:     time.Duration(evCfg.PollInterval) * time.Second,
				BatchSize:        evCfg.BatchSize,
				IPFSGateway:      "", // TODO: wire from config when IPFS support is added
				EvalEndpoint:     "", // TODO: wire from config when eval agent is deployed
				BreakerThreshold: 10,
				BreakerCooldown:  60 * time.Second,
			},
			eventSource,
			jobCompleter,
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

	// 12. init middleware and router
	routables := []router.Routable{
		erc20Handler,
		prismHandler,
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
