// Package chainbinding provides production implementations of the on-chain
// interfaces consumed by the Evaluator and Keeper (Phase 6 bots).
//
// All bindings are hand-written minimal ABIs (same pattern as
// cmd/agent/registry_binding.go) rather than abigen output, because each
// interface only needs 1-3 functions and pulling in full abigen bindings
// would bloat the binary. If more functions are needed later, regenerate
// via `abigen --abi <contract>.json --type <Name> --out <name>.go`.
package chainbinding

// registryABI exposes the subset of PrismSettleRegistry functions needed by
// the Evaluator (submitValidation) and Keeper (aggregateEpoch, getScore,
// getValidationCount, agents).
const registryABI = `[
  {"name":"submitValidation","type":"function","stateMutability":"nonpayable",
   "inputs":[{"name":"agentId","type":"uint256"},{"name":"score","type":"uint96"},{"name":"proofHash","type":"bytes32"},{"name":"jobId","type":"uint256"},{"name":"source","type":"uint8"}],
   "outputs":[]},
  {"name":"aggregateEpoch","type":"function","stateMutability":"nonpayable",
   "inputs":[{"name":"agentId","type":"uint256"}],
   "outputs":[]},
  {"name":"getScore","type":"function","stateMutability":"view",
   "inputs":[{"name":"agentId","type":"uint256"}],
   "outputs":[{"name":"","type":"uint256"}]},
  {"name":"getValidationCount","type":"function","stateMutability":"view",
   "inputs":[{"name":"agentId","type":"uint256"}],
   "outputs":[{"name":"","type":"uint256"}]},
  {"name":"agents","type":"function","stateMutability":"view",
   "inputs":[{"name":"","type":"uint256"}],
   "outputs":[{"name":"registered","type":"bool"},{"name":"operator","type":"address"},{"name":"metadata","type":"string"},{"name":"endpoint","type":"string"},{"name":"lastActivity","type":"uint64"},{"name":"aggregatedScore","type":"uint96"},{"name":"lastAggregate","type":"uint64"},{"name":"unstakeInitiated","type":"uint64"},{"name":"unstakeAvailableAt","type":"uint64"}]}
]`

// jobABI exposes PrismSettleJob.complete (Evaluator main path).
const jobABI = `[
  {"name":"complete","type":"function","stateMutability":"nonpayable",
   "inputs":[{"name":"jobId","type":"uint256"}],
   "outputs":[]},
  {"name":"getJobState","type":"function","stateMutability":"view",
   "inputs":[{"name":"jobId","type":"uint256"}],
   "outputs":[{"name":"state","type":"uint8"},{"name":"buyer","type":"address"},{"name":"provider","type":"address"},{"name":"amount","type":"uint256"},{"name":"deliverableHash","type":"bytes32"},{"name":"proofHash","type":"bytes32"},{"name":"deadline","type":"uint64"},{"name":"hook","type":"address"}]}
]`

// hookABI exposes ArbitrationHook.resolveDispute (Evaluator arbitration path).
const hookABI = `[
  {"name":"resolveDispute","type":"function","stateMutability":"nonpayable",
   "inputs":[{"name":"jobId","type":"uint256"},{"name":"ruling","type":"uint8"}],
   "outputs":[]}
]`
