package errno

import "fmt"

// Errno  custom error type with code and message
type Errno struct {
	Code int    `json:"code"` // Error code (for frontend/superior judgment)
	Msg  string `json:"msg"`  // Error message (user-readable)
	Err  error  `json:"-"`    // Original error (for debugging, not returned to frontend)
}

// Error implement error interface
func (e *Errno) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("code:%d, msg:%s, err:%v", e.Code, e.Msg, e.Err)
	}
	return fmt.Sprintf("code:%d, msg:%s", e.Code, e.Msg)
}

// Unwrap implement errors.Unwrapper to support errors.Is/errors.As
func (e *Errno) Unwrap() error {
	return e.Err
}

var (
	// === Global Standard Errors ===
	Success            = New(0, "success", nil)
	ErrBadRequest      = New(400, "bad request", nil)
	ErrInvalidParam    = New(400, "invalid parameters", nil)
	ErrNotFound        = New(404, "resource not found", nil)
	ErrUnauthorized    = New(401, "unauthorized", nil)
	ErrForbidden       = New(403, "forbidden", nil)
	ErrTooManyRequests = New(429, "too many requests", nil)
	ErrInternal        = New(500, "internal server error", nil)

	// === Blockchain Infrastructure Errors (100xx) ===
	// used for listener sync, RPC interaction and other underlying infrastructure
	ErrRPCConnectionFailed = New(10001, "failed to connect to RPC node", nil)
	ErrRPCTimeout          = New(10002, "RPC request timed out", nil)
	ErrBlockSyncFailed     = New(10003, "block synchronization failed", nil)
	ErrChainReorgDetected  = New(10004, "chain reorganization detected", nil)
	ErrInvalidBlockNumber  = New(10005, "invalid block number", nil)
	ErrEventParseFailed    = New(10006, "failed to parse blockchain event logs", nil)

	// === Contract & ABI Errors (101xx) ===
	// used for contract interaction, ABI parsing, token metadata retrieval
	ErrInvalidContractAddress = New(10101, "invalid contract address", nil)
	ErrABINotFound            = New(10102, "contract ABI not found or invalid", nil)
	ErrContractCallFailed     = New(10103, "contract static call failed", nil)
	ErrTokenMetaNotFound      = New(10104, "token metadata (symbol/decimals) not found", nil)
	ErrUnsupportedTokenType   = New(10105, "unsupported token standard (e.g., non-ERC20)", nil)

	// === Business Data Errors (102xx) ===
	// used for API queries, balance calculations, business logic validation
	ErrAddressNotIndexed    = New(10201, "address has no indexed transaction history", nil)
	ErrBalanceCalcFailed    = New(10202, "failed to calculate balance", nil)
	ErrTransactionNotFound  = New(10203, "transaction hash not found in database", nil)
	ErrInvalidAddressFormat = New(10204, "invalid ethereum address format", nil)
	ErrDataConsistencyError = New(10205, "data consistency check failed", nil)

	// === PrismSettle Business Errors (103xx) ===
	// SD §8.3: PrismSettle-specific error codes for agent/job/trust endpoints.
	ErrAgentNotFound        = New(10301, "agent not found in registry", nil)
	ErrJobNotFound          = New(10302, "job not found", nil)
	ErrJobStateInvalid      = New(10303, "invalid job state filter", nil)
	ErrScoreNotAggregated   = New(10304, "no aggregated score yet for agent", nil)
	ErrThresholdInvalid     = New(10305, "invalid trust threshold value", nil)
	ErrAgentEndpointMissing = New(10306, "agent endpoint not configured", nil)
	ErrAgentInvokeFailed    = New(10307, "agent invoke upstream failed", nil)
	ErrShardNotFound        = New(10308, "shard has no validation activity", nil)
)

// New  create a new custom error
func New(code int, msg string, err error) *Errno {
	return &Errno{
		Code: code,
		Msg:  msg,
		Err:  err,
	}
}

// WithError  chainable with error wrapper to add original error context without losing the custom error code/message
func (e *Errno) WithError(err error) *Errno {
	return &Errno{
		Code: e.Code,
		Msg:  e.Msg,
		Err:  err,
	}
}
