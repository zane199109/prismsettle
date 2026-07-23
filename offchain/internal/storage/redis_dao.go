package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/zane/web3-offchain/pkg/config"
	"github.com/zane/web3-offchain/pkg/logger"
)

// RedisStorage provides a unified Redis client wrapper with production-ready utilities
// Includes caching, locking, JSON serialization, and connection pool monitoring
type RedisStorage struct {
	client *redis.Client
}

// NewRedisStorage initializes a Redis client with configuration from config
// Validates connection and returns a ready-to-use storage instance
func NewRedisStorage() (*RedisStorage, error) {
	cfg := config.Cfg.Redis

	client := redis.NewClient(&redis.Options{
		Addr:            fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
		Password:        cfg.Password,
		DB:              cfg.Db,
		PoolSize:        cfg.PoolSize,
		ReadTimeout:     cfg.ReadTimeout,
		WriteTimeout:    cfg.WriteTimeout,
		IdleTimeout:     cfg.IdleTimeout,
		MaxRetries:      cfg.MaxRetries,
		MinRetryBackoff: cfg.MinRetryBackoff,
		MaxRetryBackoff: cfg.MaxRetryBackoff,
	})

	// Test connection with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, err := client.Ping(ctx).Result(); err != nil {
		logger.Errorf("Redis connection failed", logger.Error(err))
		return nil, fmt.Errorf("redis ping failed: %w", err)
	}

	logger.Info("Redis client initialized successfully")
	return &RedisStorage{client: client}, nil
}

// StartPoolMonitoring starts a background goroutine to log Redis pool stats every minute
// Useful for production performance monitoring and connection leak detection
func (r *RedisStorage) StartPoolMonitoring(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(1 * time.Minute)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				logger.Info("Redis pool monitoring stopped")
				return
			case <-ticker.C:
				stats := r.client.PoolStats()
				logger.Debug("Redis pool stats",
					logger.Int("hits", int(stats.Hits)),
					logger.Int("misses", int(stats.Misses)),
					logger.Int("timeouts", int(stats.Timeouts)),
					logger.Int("total_conns", int(stats.TotalConns)),
					logger.Int("idle_conns", int(stats.IdleConns)),
					logger.Int("stale_conns", int(stats.StaleConns)),
				)
			}
		}
	}()
}

// HealthCheck performs a Redis ping for liveness/readiness probes
// Returns nil if healthy, error otherwise
func (r *RedisStorage) HealthCheck(ctx context.Context) error {
	if _, err := r.client.Ping(ctx).Result(); err != nil {
		return fmt.Errorf("redis health check failed: %w", err)
	}
	return nil
}

// GetRedisClient returns the underlying go-redis Client
// For advanced operations not covered by wrapper methods
func (r *RedisStorage) GetRedisClient() *redis.Client {
	return r.client
}

// Set stores a key-value pair with expiration time
func (r *RedisStorage) Set(ctx context.Context, key string, value interface{}, expire time.Duration) error {
	return r.client.Set(ctx, key, value, expire).Err()
}

// Get retrieves a value by key
// Returns empty string and nil error if key does not exist (normal case)
func (r *RedisStorage) Get(ctx context.Context, key string) (string, error) {
	val, err := r.client.Get(ctx, key).Result()
	if err == redis.Nil {
		return "", nil // Key not found: return empty without error
	}
	return val, err
}

// Del deletes a key
func (r *RedisStorage) Del(ctx context.Context, key string) error {
	return r.client.Del(ctx, key).Err()
}

// SetNX implements atomic "SET if not exists" for distributed locks
// Enforces minimum 5s expiration to prevent deadlocks
func (r *RedisStorage) SetNX(ctx context.Context, key string, value string, expire time.Duration) (bool, error) {
	if expire < 1*time.Second {
		expire = 5 * time.Second
	}

	ok, err := r.client.SetNX(ctx, key, value, expire).Result()
	if err != nil {
		logger.Errorf("SetNX failed", logger.String("key", key), logger.Error(err))
		return false, fmt.Errorf("SetNX failed: %w", err)
	}
	return ok, nil
}

// SetJSON serializes a struct to JSON and stores it in Redis
func (r *RedisStorage) SetJSON(ctx context.Context, key string, value interface{}, expire time.Duration) error {
	data, err := json.Marshal(value)
	if err != nil {
		logger.Errorf("JSON marshal failed", logger.String("key", key), logger.Error(err))
		return fmt.Errorf("json marshal failed: %w", err)
	}
	return r.Set(ctx, key, string(data), expire)
}

// GetJSON retrieves and unmarshals JSON into a destination struct
// Returns redis.Nil if key not found (expected behavior)
func (r *RedisStorage) GetJSON(ctx context.Context, key string, dest interface{}) error {
	val, err := r.Get(ctx, key)
	if err != nil {
		return fmt.Errorf("get key failed: %w", err)
	}
	if val == "" {
		return redis.Nil
	}
	return json.Unmarshal([]byte(val), dest)
}

// Exists checks if a key exists
func (r *RedisStorage) Exists(ctx context.Context, key string) (bool, error) {
	count, err := r.client.Exists(ctx, key).Result()
	return count > 0, err
}

// Expire updates the expiration time of a key
func (r *RedisStorage) Expire(ctx context.Context, key string, expire time.Duration) error {
	return r.client.Expire(ctx, key, expire).Err()
}

// TTL returns the remaining time to live of a key
func (r *RedisStorage) TTL(ctx context.Context, key string) (time.Duration, error) {
	return r.client.TTL(ctx, key).Result()
}

// Incr atomically increments an integer key
func (r *RedisStorage) Incr(ctx context.Context, key string) (int64, error) {
	return r.client.Incr(ctx, key).Result()
}

// Decr atomically decrements an integer key
func (r *RedisStorage) Decr(ctx context.Context, key string) (int64, error) {
	return r.client.Decr(ctx, key).Result()
}

// GetPoolStats returns connection pool statistics for monitoring
func (r *RedisStorage) GetPoolStats() *redis.PoolStats {
	return r.client.PoolStats()
}

// Close closes the Redis client and releases connections
func (r *RedisStorage) Close() error {
	return r.client.Close()
}
