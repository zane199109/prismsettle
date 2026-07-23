package utils

import (
	"context"
	"fmt"
	"time"

	"github.com/zane/web3-offchain/pkg/logger"
)

// LogPanic captures panic and logs
func LogPanic() {
	if r := recover(); r != nil {
		logger.Errorf("panic recovered",
			logger.Any("panic", r),
			logger.Stack("stacktrace"),
		)
	}
}

// Retry retries fn up to maxRetries times with the given interval between
// attempts. The retry loop returns early when ctx is cancelled (e.g.
// shutdown or request timeout) so callers don't block indefinitely on a
// failing operation that nobody is waiting for anymore.
func Retry(ctx context.Context, maxRetries int, interval time.Duration, fn func() error) error {
	var err error
	for i := 0; i < maxRetries; i++ {
		if err = fn(); err == nil {
			return nil
		}
		if ctxErr := ctx.Err(); ctxErr != nil {
			return fmt.Errorf("retry cancelled: %w (last error: %v)", ctxErr, err)
		}
		logger.Warn("retry failed",
			logger.Int("retry_count", i+1),
			logger.Int("max", maxRetries),
			logger.Error(err),
		)
		select {
		case <-time.After(interval):
		case <-ctx.Done():
			return fmt.Errorf("retry cancelled: %w (last error: %v)", ctx.Err(), err)
		}
	}
	return fmt.Errorf("max retries (%d) exceeded: %w", maxRetries, err)
}
