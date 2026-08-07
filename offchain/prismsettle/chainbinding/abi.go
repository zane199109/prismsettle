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
// getValidationCount, agents, setAggregatedScore).
const registryABI = `[
  {"name":"submitValidation","type":"function","stateMutability":"nonpayable",
   "inputs":[{"name":"agentId","type":"uint256"},{"name":"score","type":"uint96"},{"name":"proofHash","type":"bytes32"},{"name":"jobId","type":"uint256"},{"name":"source","type":"uint8"}],
   "outputs":[]},
  {"name":"aggregateEpoch","type":"function","stateMutability":"nonpayable",
   "inputs":[{"name":"agentId","type":"uint256"}],
   "outputs":[]},
  {"name":"setAggregatedScore","type":"function","stateMutability":"nonpayable",
   "inputs":[{"name":"agentId","type":"uint256"},{"name":"newScore","type":"uint256"}],
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

// jobABI exposes PrismSettleJob functions needed by the Evaluator and
// arbitrators (complete, getJobState, grabJob, executeArbitrationResult, etc.).
const jobABI = `[
  {"name":"complete","type":"function","stateMutability":"nonpayable",
   "inputs":[{"name":"jobId","type":"uint256"},{"name":"score","type":"uint96"}],
   "outputs":[]},
  {"name":"reject","type":"function","stateMutability":"nonpayable",
   "inputs":[{"name":"jobId","type":"uint256"},{"name":"reasonHash","type":"bytes32"}],
   "outputs":[]},
  {"name":"submit","type":"function","stateMutability":"nonpayable",
   "inputs":[{"name":"jobId","type":"uint256"},{"name":"deliverableHash","type":"bytes32"},{"name":"proofHash","type":"bytes32"}],
   "outputs":[]},
  {"name":"getJobState","type":"function","stateMutability":"view",
   "inputs":[{"name":"jobId","type":"uint256"}],
   "outputs":[{"name":"state","type":"uint8"},{"name":"buyer","type":"address"},{"name":"provider","type":"address"},{"name":"amount","type":"uint256"},{"name":"deliverableHash","type":"bytes32"},{"name":"proofHash","type":"bytes32"},{"name":"deadline","type":"uint64"},{"name":"hook","type":"address"},{"name":"minProviderReputation","type":"uint96"},{"name":"disputeResolvedAt","type":"uint256"}]},
  {"name":"grabJob","type":"function","stateMutability":"nonpayable",
   "inputs":[{"name":"jobId","type":"uint256"},{"name":"providerAgentId","type":"uint256"}],
   "outputs":[]},
  {"name":"executeArbitrationResult","type":"function","stateMutability":"nonpayable",
   "inputs":[{"name":"jobId","type":"uint256"}],
   "outputs":[]},
  {"name":"setRegistry","type":"function","stateMutability":"nonpayable",
   "inputs":[{"name":"registryAddr","type":"address"}],
   "outputs":[]},
  {"name":"notifyDisputeResolved","type":"function","stateMutability":"nonpayable",
   "inputs":[{"name":"jobId","type":"uint256"},{"name":"ruling","type":"uint8"}],
   "outputs":[]}
]`

// hookABI exposes ArbitrationHook functions needed by the Evaluator and
// resolvers (resolveDispute, dispute, getHookState, getArbitratorFeeConfig,
// registerArbitrator, etc.).
const hookABI = `[
  {"name":"resolveDispute","type":"function","stateMutability":"nonpayable",
   "inputs":[{"name":"jobId","type":"uint256"},{"name":"ruling","type":"uint8"}],
   "outputs":[]},
  {"name":"dispute","type":"function","stateMutability":"nonpayable",
   "inputs":[{"name":"jobId","type":"uint256"},{"name":"reasonHash","type":"bytes32"}],
   "outputs":[]},
  {"name":"getHookState","type":"function","stateMutability":"view",
   "inputs":[{"name":"jobId","type":"uint256"}],
   "outputs":[{"name":"state","type":"uint8"},{"name":"reasonHash","type":"bytes32"},{"name":"ruling","type":"uint8"}]},
  {"name":"getArbitratorFeeConfig","type":"function","stateMutability":"view",
   "inputs":[{"name":"jobId","type":"uint256"}],
   "outputs":[{"name":"feeBps","type":"uint256"},{"name":"recipient","type":"address"}]},
  {"name":"registerArbitrator","type":"function","stateMutability":"nonpayable",
   "inputs":[{"name":"agentId","type":"uint256"},{"name":"feeBps","type":"uint256"},{"name":"feeRecipient","type":"address"}],
   "outputs":[]},
  {"name":"activeArbitratorCount","type":"function","stateMutability":"view",
   "inputs":[],
   "outputs":[{"name":"","type":"uint256"}]},
  {"name":"arbitratorList","type":"function","stateMutability":"view",
   "inputs":[{"name":"","type":"uint256"}],
   "outputs":[{"name":"","type":"address"}]},
  {"name":"arbitratorConfigs","type":"function","stateMutability":"view",
   "inputs":[{"name":"","type":"address"}],
   "outputs":[{"name":"agentId","type":"uint256"},{"name":"feeBps","type":"uint256"},{"name":"feeRecipient","type":"address"},{"name":"registered","type":"bool"}]},
  {"name":"disputeArbitrator","type":"function","stateMutability":"view",
   "inputs":[{"name":"","type":"uint256"}],
   "outputs":[{"name":"","type":"address"}]}
]`