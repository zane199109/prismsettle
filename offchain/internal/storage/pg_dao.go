package storage

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/zane/web3-offchain/model"
	"github.com/zane/web3-offchain/pkg/config"
	"github.com/zane/web3-offchain/pkg/logger"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// PGStorage implements PostgreSQL storage for blockchain data
// Manages DB connection, migration, indexing, and provides GORM DB access
type PGStorage struct {
	db  *gorm.DB // GORM DB instance for ORM operations
	sql *sql.DB  // Raw SQL DB for connection pool management
}

// NewPGStorage initializes PostgreSQL connection with connection pool
// Configures GORM, connection pool settings, and validates connectivity
func NewPGStorage() (*PGStorage, error) {
	cfg := config.Cfg.Postgres

	// Build PostgreSQL DSN (Data Source Name)
	// Timezone is configurable so deployments in different regions don't
	// silently get Asia/Shanghai semantics. Default to UTC when unset.
	timezone := cfg.Timezone
	if timezone == "" {
		timezone = "UTC"
	}
	dsn := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=disable TimeZone=%s",
		cfg.Host, cfg.Port, cfg.User, cfg.Password, cfg.DbName, timezone,
	)

	// Configure GORM logger: reduce verbosity in production
	var logMode gormlogger.LogLevel
	if config.Cfg.Env == "prod" {
		logMode = gormlogger.Error // Only log errors in production
	} else {
		logMode = gormlogger.Info // Log all in development
	}
	gormLogger := gormlogger.Default.LogMode(logMode)

	// Open database connection
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: gormLogger,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to postgres: %w", err)
	}

	// Get underlying sql.DB for connection pool configuration
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get sql.DB instance: %w", err)
	}

	// Configure connection pool for production stability
	sqlDB.SetMaxOpenConns(cfg.MaxOpenConns)
	sqlDB.SetMaxIdleConns(cfg.MaxIdleConns)
	sqlDB.SetConnMaxLifetime(cfg.ConnMaxLifetime)

	// Verify connection with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := sqlDB.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("postgres ping failed: %w", err)
	}

	logger.Info("postgres connection initialized successfully",
		logger.String("host", cfg.Host),
		logger.String("database", cfg.DbName),
	)

	return &PGStorage{
		db:  db,
		sql: sqlDB,
	}, nil
}

// Close terminates database connection gracefully
func (p *PGStorage) Close() error {
	return p.sql.Close()
}

// AutoMigrate creates database tables and essential indexes
// Critical for query performance in production
func (p *PGStorage) AutoMigrate(ctx context.Context) error {
	// Auto-migrate schema for core models
	if err := p.db.WithContext(ctx).AutoMigrate(&model.ChainEvent{}); err != nil {
		logger.Errorf("failed to migrate chain_events table", logger.Error(err))
		return err
	}

	if err := p.db.WithContext(ctx).AutoMigrate(&model.ChainBlockState{}); err != nil {
		logger.Errorf("failed to migrate chain_block_states table", logger.Error(err))
		return err
	}

	// Phase 7: agent_registry mirrors AgentRegistered events for fast
	// /agents endpoint queries (metadata + endpoint URL).
	if err := p.db.WithContext(ctx).AutoMigrate(&model.AgentRegistryRecord{}); err != nil {
		logger.Errorf("failed to migrate agent_registry table", logger.Error(err))
		return err
	}

	// Grab attempts (competition visibility): agent services report grab
	// outcomes so operators can see why their agent lost a job.
	if err := p.db.WithContext(ctx).AutoMigrate(&model.GrabAttempt{}); err != nil {
		logger.Errorf("failed to migrate grab_attempts table", logger.Error(err))
		return err
	}

	// Phase 7 task 7.2: trust_thresholds holds per-agent or default
	// ALLOW/DENY thresholds used by the /trust endpoint (FR-AP11).
	if err := p.db.WithContext(ctx).AutoMigrate(&model.TrustThreshold{}); err != nil {
		logger.Errorf("failed to migrate trust_thresholds table", logger.Error(err))
		return err
	}

	// Phase 9 task 9.4: reorg_events persists each reorg detected by the
	// listener so /perf/reorg-feed can serve real data instead of the
	// Phase 7 mock. FR-M07 + FR-I03/I04.
	if err := p.db.WithContext(ctx).AutoMigrate(&model.ReorgEvent{}); err != nil {
		logger.Errorf("failed to migrate reorg_events table", logger.Error(err))
		return err
	}

	// Phase 9 task 9.6: perf_results stores V0 vs V1 benchmark runs so
	// /perf/v0-v1-comparison can serve real data instead of the Phase 7 mock.
	// FR-T03/T04/T05.
	if err := p.db.WithContext(ctx).AutoMigrate(&model.PerfResult{}); err != nil {
		logger.Errorf("failed to migrate perf_results table", logger.Error(err))
		return err
	}

	// Create production-critical indexes
	// These indexes prevent full table scans on high-volume data
	indexes := []string{
		// Unique index for idempotent event insertion (tx_hash + log_index)
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_chain_event_tx_hash_idx ON chain_events (tx_hash, log_index)`,
		// Index for fast pagination by chain + block
		`CREATE INDEX IF NOT EXISTS idx_chain_event_chain_block ON chain_events (chain_name, block_number DESC)`,
		// Index for fast FundMe event queries
		`CREATE INDEX IF NOT EXISTS idx_chain_event_contract_type ON chain_events (contract, event_type)`,
		// Expression indexes for case-insensitive address lookups. The query
		// layer uses LOWER("from")/LOWER("to") so a plain B-tree index on the
		// raw column won't be used; without these, GetTransfers/GetTokenBalance
		// would degrade to a full table scan as the events table grows.
		`CREATE INDEX IF NOT EXISTS idx_chain_events_from_lower ON chain_events (LOWER("from"))`,
		`CREATE INDEX IF NOT EXISTS idx_chain_events_to_lower ON chain_events (LOWER("to"))`,
		`CREATE INDEX IF NOT EXISTS idx_chain_events_contract_lower ON chain_events (LOWER(contract))`,
	}

	// Execute index creation (non-blocking)
	for _, idxSQL := range indexes {
		if err := p.db.WithContext(ctx).Exec(idxSQL).Error; err != nil {
			logger.Warn("failed to create index (may already exist)",
				logger.String("sql", idxSQL),
				logger.Error(err),
			)
		}
	}

	logger.Info("postgres auto-migration and index creation completed")
	return nil
}

// DB returns the underlying GORM DB instance
// Used for repository layer operations
func (p *PGStorage) DB() *gorm.DB {
	return p.db
}
