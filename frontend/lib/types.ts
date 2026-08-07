// TypeScript types mirroring the Go backend models under
// offchain/model/models.go and offchain/prismsettle/service. Keep these in
// sync manually until we generate them from OpenAPI (planned Phase 10).

export interface AgentVO {
  agent_id: string;
  owner: string;
  metadata: string;
  endpoint: string;
  registered_at: number;
  block_number: number;
  score: string; // decimal string (uint256 scaled)
}

export interface JobVO {
  job_id: string;
  shard_id: number;
  status: string; // Pending | Submitted | Completed | Disputed | Resolved
  creator: string;
  evaluator: string;
  created_at: number;
  updated_at: number;
  amount?: string; // Escrow amount (token value string)
  provider?: string; // Provider address (assigned via grabJob)
}

// FR-AP06~AP09: x402 / ERC-20 dual funding path.
// When JobForm (Phase 8.5a) is implemented, use this type to mark the
// payment path: "x402" (facilitator settle) or "erc20" (fallback).
// The path is determined by whether the buyer provides an x402 receipt.
export type FundingPath = "x402" | "erc20";

export interface FundingPathInfo {
  path: FundingPath;
  // True when facilitator is address(0) or unreachable → must use ERC-20.
  facilitator_available: boolean;
  // Human-readable hint for the UI badge.
  hint: string;
}

export interface JobTimelineItem {
  status: string;
  tx_hash: string;
  block_number: number;
  timestamp: number;
}

export interface TrustCheckResult {
  agent_id: string;
  score: string;
  allow_threshold: string;
  deny_threshold: string;
  decision: "allow" | "deny" | "review";
}

export interface TrustThreshold {
  agent_id: string; // "" = global default
  allow_threshold: string;
  deny_threshold: string;
}

export interface ShardActivity {
  shard_id: number;
  validation_count: number;
  last_active_at: number;
}

export interface ChainEvent {
  id: number;
  chain_name: string;
  chain_type?: string;
  tx_type?: string;
  event_type: string;
  token_address?: string; // ERC20 token or auxiliary hash (proofHash, hook, reasonHash)
  from: string;
  to: string;
  value: string;
  symbol?: string; // ERC20 symbol or PrismSettle source ("0"/"1"/"2") / buyer address
  decimals?: number;
  tx_hash: string;
  log_index?: number;
  block_number: number;
  block_time: number;
  extra: string;
}

export interface ValidationCount {
  total: number;
  by_status?: Record<string, number>;
}

// Standard API envelope returned by every backend endpoint.
// See offchain/pkg/response/response.go.
export interface ApiEnvelope<T> {
  code: number; // 0 = success, non-0 = business error
  message: string;
  data: T;
}

export interface Paginated<T> {
  items: T[];
  total: number;
  page: number;
  size: number;
}
