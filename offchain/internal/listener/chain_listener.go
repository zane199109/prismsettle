package listener

import (
	"context"
)

// defined this interface to make sure that all listener implement this interface,
// so that we can use the interface as a parameter in evm_listerner and other listeners in the future,
// and also make sure that all listeners have the same start and stop method,
// which is important for the graceful shutdown of the application.
type ChainListener interface {
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
}
