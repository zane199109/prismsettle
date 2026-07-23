package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
	"github.com/zane/web3-offchain/pkg/constant"
	"github.com/zane/web3-offchain/pkg/logger"
)

// Distributed lock error definitions
var (
	ErrLockFailed   = errors.New("failed to acquire lock")   // Lock acquisition failed
	ErrLockReleased = errors.New("failed to release lock")   // Lock release failed
	ErrLockNotOwner = errors.New("lock is not owned by you") // Cannot release a lock owned by others
)

// LockService implements a Redis-based distributed lock service
// Used to prevent duplicate processing in distributed environments (e.g. chain block sync)
type LockService struct {
	rds *redis.Client // Redis client (injected from infrastructure)
}

// NewLockService creates a new LockService instance with dependency injection
func NewLockService(rds *redis.Client) *LockService {
	return &LockService{rds: rds}
}

// AcquireLock attempts to acquire a distributed lock with expiration
// Returns a unique lock value if successful, error otherwise
func (ls *LockService) AcquireLock(
	ctx context.Context,
	lockKey string,
	expire time.Duration,
) (string, error) {
	key := fmt.Sprintf("%s:%s", constant.LockKeyPrefix, lockKey)

	// Use a random UUID as the lock value to identify this lock's owner.
	// Previously this used time.Now().UnixNano(), which could collide across
	// concurrent instances within the same nanosecond and cause a host to
	// wrongly treat another instance's lock as its own.
	lockValue := uuid.NewString()

	// Set lock only if it does not exist (atomic operation)
	ok, err := ls.rds.SetNX(ctx, key, lockValue, expire).Result()
	if err != nil {
		logger.Errorf("Failed to acquire distributed lock",
			logger.String("key", key),
			logger.Error(err),
		)
		return "", ErrLockFailed
	}

	if !ok {
		logger.Info("Failed to acquire distributed lock (already locked)",
			logger.String("key", key),
		)
		return "", ErrLockFailed
	}

	logger.Info("Successfully acquired distributed lock",
		logger.String("key", key),
		logger.Duration("expire", expire),
	)

	return lockValue, nil
}

// ReleaseLock releases the distributed lock safely using Lua script
// Ensures atomicity and prevents deleting locks owned by others
func (ls *LockService) ReleaseLock(
	ctx context.Context,
	lockKey string,
	lockValue string,
) error {
	key := fmt.Sprintf("%s:%s", constant.LockKeyPrefix, lockKey)

	// Lua script guarantees atomicity: delete only if value matches
	// Returns 1 if deleted, 0 otherwise
	script := `
	if redis.call("GET", KEYS[1]) == ARGV[1] then
		return redis.call("DEL", KEYS[1])
	else
		return 0
	end
	`
	// Execute atomic lock release
	result, err := ls.rds.Eval(ctx, script, []string{key}, lockValue).Result()
	if err != nil {
		logger.Errorf("Redis error while releasing lock",
			logger.String("key", key),
			logger.Error(err),
		)
		return fmt.Errorf("%w: %v", ErrLockReleased, err)
	}

	// Handle result
	if resInt, ok := result.(int64); !ok || resInt == 0 {
		logger.Warn("Cannot release lock (not owner or expired)",
			logger.String("key", key),
		)
		return ErrLockNotOwner
	}

	logger.Info("Successfully released distributed lock",
		logger.String("key", key),
	)

	return nil
}
