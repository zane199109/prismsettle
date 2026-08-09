package config

import (
	"fmt"
	"time"

	"github.com/spf13/viper"
)

// golbal config instance
var Cfg *Config

// Config
type Config struct {
	Env       string          `mapstructure:"env"` // Environment dev/prod
	Server    ServerConfig    `mapstructure:"server"`
	Postgres  PostgresConfig  `mapstructure:"postgres"`
	Redis     RedisConfig     `mapstructure:"redis"`
	Web3      Web3Config      `mapstructure:"web3"`
	Notify    NotifyConfig    `mapstructure:"notify"`     // Alert configuration
	RateLimit RateLimitConfig `mapstructure:"rate_limit"` // Rate limit configuration
	Auth      AuthConfig      `mapstructure:"auth"`       // Authentication configuration
	Evaluator EvaluatorConfig `mapstructure:"evaluator"`  // Evaluator + Keeper bot config
}

// EvaluatorConfig holds the private key and addresses used by the Evaluator
// (Phase 6) and the Deploy script (Phase 3). The private key is shared so
// the Evaluator can sign submitValidation txs with REGISTRY_EVALUATOR_ROLE.
type EvaluatorConfig struct {
	// PrivateKey hex-encoded (no 0x prefix). Required in prod; optional in dev
	// (Deploy script falls back to msg.sender when unset). Used by both
	// Evaluator (submitValidation, complete, resolveDispute) and Keeper
	// (aggregateEpoch) — Keeper has no role requirement so sharing the key
	// is safe.
	PrivateKey string `mapstructure:"private_key"`
	// EvaluatorAddress is the derived address (checksum). Used by Deploy script
	// to grant REGISTRY_EVALUATOR_ROLE. Filled from PrivateKey when empty.
	EvaluatorAddress string `mapstructure:"evaluator_address"`
	// FacilitatorAddress is the x402 facilitator contract address (optional).
	FacilitatorAddress string `mapstructure:"facilitator_address"`
	// ValidatorAddress is the validator address used for staking (optional).
	ValidatorAddress string `mapstructure:"validator_address"`

	// PrismSettle chain binding (Phase 9 P0-A). The Evaluator + Keeper bots
	// run on a single chain (the chain where PrismSettle contracts are
	// deployed). These fields tell the bots which chain + contracts to use.
	ChainID       int64  `mapstructure:"chain_id"`        // e.g. 10143 for Monad testnet
	ChainName     string `mapstructure:"chain_name"`      // must match a Web3.Chains[].ChainName
	RPCURL        string `mapstructure:"rpc_url"`         // RPC endpoint for signing txs
	RegistryAddr  string `mapstructure:"registry_addr"`   // PrismSettleRegistry contract address
	JobAddr       string `mapstructure:"job_addr"`        // PrismSettleJob contract address
	HookAddr      string `mapstructure:"hook_addr"`       // ArbitrationHook contract address
	PaymentToken  string `mapstructure:"payment_token"`   // default payment token (USDC mock)
	// WmonToken is the Wrapped MON address (optional). When set, the demo
	// orchestrator accepts WMON as an alternative per-job token.
	WmonToken string `mapstructure:"wmon_token"`
	AnnouncementWaitSec int `mapstructure:"announcement_wait_sec"` // demo wait before executing arbitration (>= on-chain period)
	PollInterval  int    `mapstructure:"poll_interval"`   // Evaluator poll interval (seconds), default 2
	BatchSize     int    `mapstructure:"batch_size"`      // Evaluator batch size, default 20
	KeeperTickSec int    `mapstructure:"keeper_tick_sec"` // Keeper tick period (seconds), default 30
}

type Web3Config struct {
	Chains []ChainConfig `mapstructure:"chains"`
}

// ServerConfig
type ServerConfig struct {
	Port                      string `mapstructure:"port"`
	Mode                      string `mapstructure:"mode"`
	LogPath                   string `mapstructure:"log_path"`
	ReadTimeout               int    `mapstructure:"read_timeout"` // Unit: seconds
	WriteTimeout              int    `mapstructure:"write_timeout"`
	IdleTimeout               int    `mapstructure:"idle_timeout"`
	ShutdownListenerTimeout   int    `mapstructure:"shutdown_listener_timeout"`
	ShutdownHttpserverTimeout int    `mapstructure:"shutdown_httpserver_timeout"`
}

// PostgresConfig
type PostgresConfig struct {
	Host            string        `mapstructure:"host"`
	Port            int           `mapstructure:"port"`
	User            string        `mapstructure:"user"`
	Password        string        `mapstructure:"password"`
	DbName          string        `mapstructure:"dbname"`
	Timezone        string        `mapstructure:"timezone"` // IANA TZ (e.g. Asia/Shanghai); defaults to UTC if empty
	MaxIdleConns    int           `mapstructure:"max_idle_conns"`
	MaxOpenConns    int           `mapstructure:"max_open_conns"`
	ConnMaxLifetime time.Duration `mapstructure:"conn_max_lifetime"`
}

// RedisConfig
type RedisConfig struct {
	Host            string        `mapstructure:"host"`
	Port            int           `mapstructure:"port"`
	Password        string        `mapstructure:"password"`
	Db              int           `mapstructure:"db"`
	PoolSize        int           `mapstructure:"pool_size"`
	ReadTimeout     time.Duration `mapstructure:"read_timeout"`
	WriteTimeout    time.Duration `mapstructure:"write_timeout"`
	IdleTimeout     time.Duration `mapstructure:"idle_timeout"`
	MaxRetries      int           `mapstructure:"max_retries"`
	MinRetryBackoff time.Duration `mapstructure:"min_retry_backoff"`
	MaxRetryBackoff time.Duration `mapstructure:"max_retry_backoff"`
}

// Web3 config
type ChainConfig struct {
	ChainName      string   `mapstructure:"chain_name"`      // Physical network name, e.g. "monad_testnet"
	ChainType      string   `mapstructure:"chain_type"`      // "evm"
	RPCUrls        []string `mapstructure:"rpc_urls"`        // Multi-RPC nodes
	AbiType        string   `mapstructure:"abi_type"`        // ABI type
	ContractAddr   string   `mapstructure:"contract_addr"`   // Contract address
	ContractParser string   `mapstructure:"contract_parser"` // Contract parser name
	StartBlock     uint64   `mapstructure:"start_block"`     // Start block for this contract
	Workers        int      `mapstructure:"workers"`         // Concurrent goroutine count
	BatchSizeDb    int      `mapstructure:"batch_size_db"`   // Batch insert count
	BatchSizeSync  int      `mapstructure:"batch_size_sync"` // Batch sync block count
	Confirmations  int      `mapstructure:"confirmations"`   // Confirmations
	Timeout        int      `mapstructure:"timeout"`         // Timeout
	Decimals       int      `mapstructure:"decimals"`        // Decimals
	SyncInterval   int      `mapstructure:"sync_interval"`   // Sync interval
	CommitStep     uint64   `mapstructure:"commit_step"`     // Commit step

	BlockCacheSize               int `mapstructure:"block_cache_size"`
	ReorgMinDepth                int `mapstructure:"reorg_min_depth"`
	TaskQueueSize                int `mapstructure:"task_queue_size"`
	TaskBackpressureSeconds      int `mapstructure:"task_backpressure_seconds"`
	TaskSendRetryMS              int `mapstructure:"task_send_retry_ms"`
	BlockNumberRetrySeconds      int `mapstructure:"block_number_retry_seconds"`
	PermanentFailureSleepSeconds int `mapstructure:"permanent_failure_sleep_seconds"`
	LockExpirationSeconds        int `mapstructure:"lock_expiration_seconds"`
	MaxSyncRetries               int `mapstructure:"max_sync_retries"`

	// NativeSymbol / NativeDecimals describe the chain's native currency.
	// Defaults ("ETH", 18) are applied in Validate() when unset, so existing
	// configs keep working and callers no longer need a hard-coded chain→symbol
	// switch in the service layer.
	NativeSymbol   string `mapstructure:"native_symbol"`
	NativeDecimals int    `mapstructure:"native_decimals"`
}

type NotifyConfig struct {
	TelegramToken string `mapstructure:"telegram_token"`
	ChatID        string `mapstructure:"chat_id"`
}

type RateLimitConfig struct {
	PerMinute  int `mapstructure:"per_minute"`
	DailyLimit int `mapstructure:"daily_limit"`
}
type AuthConfig struct {
	Tokens []string `mapstructure:"tokens"` // Support multiple tokens
}

func InitConfig(configPath string) error {
	viper.SetConfigFile(configPath)
	viper.SetConfigType("yaml")

	// read config file
	if err := viper.ReadInConfig(); err != nil {
		return fmt.Errorf("read config failed: %v", err)
	}

	//environment variables
	viper.AutomaticEnv()
	viper.SetEnvPrefix("WEB3") // Environment variable prefix WEB3_
	// viper.BindEnv("postgresql.password", "PG_PASSWORD")
	// viper.BindEnv("web3.rpc_urls", "RPC_URLS")

	// unmarshal config
	if err := viper.Unmarshal(&Cfg); err != nil {
		return fmt.Errorf("unmarshal config failed: %v", err)
	}

	if err := Cfg.Validate(); err != nil {
		return fmt.Errorf("config validation failed: %v", err)
	}

	return nil
}

// Validate checks critical config fields to fail fast at startup instead of
// crashing later at runtime with a zero-value panic (e.g. LRU cache size 0).
func (c *Config) Validate() error {
	if c.Server.Port == "" {
		return fmt.Errorf("server.port is required")
	}
	if c.Postgres.Host == "" || c.Postgres.DbName == "" {
		return fmt.Errorf("postgres.host and postgres.dbname are required")
	}
	if c.Redis.Host == "" {
		return fmt.Errorf("redis.host is required")
	}

	for i := range c.Web3.Chains {
		ch := &c.Web3.Chains[i]
		if ch.ChainName == "" {
			return fmt.Errorf("web3.chains[%d].chain_name is required", i)
		}
		if len(ch.RPCUrls) == 0 {
			return fmt.Errorf("web3.chains[%d] (%s).rpc_urls is required", i, ch.ChainName)
		}
		if ch.ContractAddr == "" {
			return fmt.Errorf("web3.chains[%d] (%s).contract_addr is required", i, ch.ChainName)
		}
		if ch.Workers <= 0 {
			return fmt.Errorf("web3.chains[%d] (%s).workers must be > 0", i, ch.ChainName)
		}
		if ch.BatchSizeDb <= 0 {
			return fmt.Errorf("web3.chains[%d] (%s).batch_size_db must be > 0", i, ch.ChainName)
		}
		if ch.BatchSizeSync <= 0 {
			return fmt.Errorf("web3.chains[%d] (%s).batch_size_sync must be > 0", i, ch.ChainName)
		}
		if ch.BlockCacheSize <= 0 {
			return fmt.Errorf("web3.chains[%d] (%s).block_cache_size must be > 0", i, ch.ChainName)
		}
		if ch.TaskQueueSize <= 0 {
			return fmt.Errorf("web3.chains[%d] (%s).task_queue_size must be > 0", i, ch.ChainName)
		}
		if ch.Confirmations < 0 {
			return fmt.Errorf("web3.chains[%d] (%s).confirmations must be >= 0", i, ch.ChainName)
		}
		// CommitStep is used as a modulo divisor in processEvents; 0 would
		// trigger a divide-by-zero panic at runtime.
		if ch.CommitStep <= 0 {
			return fmt.Errorf("web3.chains[%d] (%s).commit_step must be > 0", i, ch.ChainName)
		}
		if ch.SyncInterval <= 0 {
			return fmt.Errorf("web3.chains[%d] (%s).sync_interval must be > 0", i, ch.ChainName)
		}
		// StartBlock == 0 would replay from genesis, which is almost never
		// intended and can saturate the RPC and DB for hours. Require an
		// explicit value to fail fast at startup.
		if ch.StartBlock == 0 {
			return fmt.Errorf("web3.chains[%d] (%s).start_block must be > 0 (set to the deployment block)",
				i, ch.ChainName)
		}
		// Apply native-currency defaults so the service layer doesn't need a
		// hard-coded chain→symbol switch.
		if ch.NativeSymbol == "" {
			ch.NativeSymbol = "ETH"
		}
		if ch.NativeDecimals <= 0 {
			ch.NativeDecimals = 18
		}
	}

	if len(c.Auth.Tokens) == 0 {
		return fmt.Errorf("auth.tokens is required (at least one API token)")
	}

	// Evaluator config: prod requires private_key for signing submitValidation
	// txs with REGISTRY_EVALUATOR_ROLE. Dev can run without it (Deploy script
	// falls back to msg.sender).
	if c.Env == "prod" && c.Evaluator.PrivateKey == "" {
		return fmt.Errorf("evaluator.private_key is required in prod environment")
	}

	// PrismSettle chain binding: when any of the contract addresses is set,
	// all must be set (Evaluator + Keeper need all three contracts). This
	// is optional in dev (bots stay disabled) but required in prod.
	if c.Evaluator.RegistryAddr != "" || c.Evaluator.JobAddr != "" || c.Evaluator.HookAddr != "" {
		if c.Evaluator.RegistryAddr == "" || c.Evaluator.JobAddr == "" || c.Evaluator.HookAddr == "" {
			return fmt.Errorf("evaluator: registry_addr, job_addr, hook_addr must all be set together")
		}
		if c.Evaluator.ChainID <= 0 {
			return fmt.Errorf("evaluator.chain_id is required when contract addresses are set")
		}
		if c.Evaluator.ChainName == "" {
			return fmt.Errorf("evaluator.chain_name is required when contract addresses are set")
		}
		if c.Evaluator.RPCURL == "" {
			return fmt.Errorf("evaluator.rpc_url is required when contract addresses are set")
		}
	}
	if c.Env == "prod" && c.Evaluator.RegistryAddr == "" {
		return fmt.Errorf("evaluator: PrismSettle contract addresses are required in prod")
	}

	// Apply defaults for optional tuning knobs.
	if c.Evaluator.PollInterval <= 0 {
		c.Evaluator.PollInterval = 2
	}
	if c.Evaluator.BatchSize <= 0 {
		c.Evaluator.BatchSize = 20
	}
	if c.Evaluator.KeeperTickSec <= 0 {
		c.Evaluator.KeeperTickSec = 30
	}

	return nil
}
