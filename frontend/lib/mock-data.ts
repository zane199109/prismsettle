// Mock data for hackathon demo. Used as a fallback by hooks when the backend
// is unreachable or returns empty results — ensures the UI always has content
// to demo. Remove once real on-chain data is flowing on Monad testnet.
//
// Design:
//   - Static arrays exported for direct import
//   - Helper functions accept optional `userAddress` so /me can attribute
//     some mock entries to the connected wallet for a personalized demo
//   - Timestamps are computed at module load to always look "fresh"

import type { AgentVO, ChainEvent, JobTimelineItem, JobVO, ShardActivity } from "./types";

// Four demo wallet addresses — distinct leading bytes for easy visual ID.
export const DEMO_ADDRESSES = {
  alice: "0x7a2f3b4c5d6e7f8090a1b2c3d4e5f60718293a4b",
  bob: "0x3c4d5e6f7081920a3b4c5d6e7f8090a1b2c3d4e5",
  carol: "0x9e8f7a6b5c4d3e2f1a0b9c8d7e6f5a4b3c2d1e0f",
  dave: "0x1a2b3c4d5e6f7081920a3b4c5d6e7f8090a1b2c3",
};

// Convenience helpers
function score(s: number): string {
  return BigInt(Math.floor(s * 1e18)).toString();
}

function minutesAgo(m: number): number {
  return Math.floor(Date.now() / 1000) - m * 60;
}

function hoursAgo(h: number): number {
  return Math.floor(Date.now() / 1000) - h * 3600;
}

// Mock tx hashes — fixed strings for stable React keys.
const TX = (n: number) =>
  `0x${n.toString(16).padStart(64, "0")}` as `0x${string}`;

// ----------------------------------------------------------------------------
// Agents — 8 entries covering all 4 PRD categories with varied grades.
// ----------------------------------------------------------------------------
export const MOCK_AGENTS: AgentVO[] = [
  {
    agent_id: "1",
    owner: DEMO_ADDRESSES.alice,
    metadata: "DeFi Analysis Agent v2.1 — analyzes ETH/USDC arbitrage opportunities on Monad DEX",
    endpoint: "https://agent-defi.prismmock.xyz",
    registered_at: hoursAgo(72),
    block_number: 1024000,
    score: score(0.92),
  },
  {
    agent_id: "2",
    owner: DEMO_ADDRESSES.bob,
    metadata: "Translation Agent Pro — supports 12 languages (zh/en/ja/ko/de/fr/es/ru/ar/pt/it/nl)",
    endpoint: "https://agent-translate.prismmock.xyz",
    registered_at: hoursAgo(60),
    block_number: 1032000,
    score: score(0.85),
  },
  {
    agent_id: "3",
    owner: DEMO_ADDRESSES.carol,
    metadata: "Data Labeling Service — image classification, NLP tagging, sentiment analysis",
    endpoint: "https://agent-data.prismmock.xyz",
    registered_at: hoursAgo(48),
    block_number: 1045000,
    score: score(0.78),
  },
  {
    agent_id: "4",
    owner: DEMO_ADDRESSES.dave,
    metadata: "Evaluation Agent — official evaluator using gpt-4o-mini for semantic scoring",
    endpoint: "https://agent-eval.prismmock.xyz",
    registered_at: hoursAgo(72),
    block_number: 1023000,
    score: score(0.88),
  },
  {
    agent_id: "5",
    owner: DEMO_ADDRESSES.alice,
    metadata: "DeFi Yield Optimizer — auto-compounds LP positions on native DEX",
    endpoint: "https://agent-yield.prismmock.xyz",
    registered_at: hoursAgo(36),
    block_number: 1058000,
    score: score(0.65),
  },
  {
    agent_id: "6",
    owner: DEMO_ADDRESSES.bob,
    metadata: "Multi-language Translator — specialized in technical docs and API references",
    endpoint: "https://agent-multi.prismmock.xyz",
    registered_at: hoursAgo(24),
    block_number: 1072000,
    score: score(0.55),
  },
  {
    agent_id: "7",
    owner: DEMO_ADDRESSES.carol,
    metadata: "Data Cleaning Bot — deduplication, schema validation, outlier detection",
    endpoint: "https://agent-clean.prismmock.xyz",
    registered_at: hoursAgo(18),
    block_number: 1081000,
    score: score(0.32),
  },
  {
    agent_id: "8",
    owner: DEMO_ADDRESSES.dave,
    metadata: "Eval Score Bot — lightweight evaluator for fast scoring, fallback when full eval times out",
    endpoint: "https://agent-evalbot.prismmock.xyz",
    registered_at: hoursAgo(6),
    block_number: 1095000,
    score: score(0.18),
  },
];

// ----------------------------------------------------------------------------
// Jobs — 8 entries in various lifecycle states.
// ----------------------------------------------------------------------------
export const MOCK_JOBS: JobVO[] = [
  {
    job_id: "1",
    shard_id: 1,
    status: "Completed",
    creator: DEMO_ADDRESSES.alice,
    evaluator: DEMO_ADDRESSES.dave,
    created_at: hoursAgo(20),
    updated_at: hoursAgo(19),
    amount: "1000000000000000000",
    token: "0x83cb612C10a27C09b7a5Ab31B906560B880abD9C",
    provider: DEMO_ADDRESSES.bob,
  },
  {
    job_id: "2",
    shard_id: 2,
    status: "Submitted",
    creator: DEMO_ADDRESSES.carol,
    evaluator: "",
    created_at: hoursAgo(8),
    updated_at: hoursAgo(1),
    amount: "5000000000000000000",
    token: "0x83cb612C10a27C09b7a5Ab31B906560B880abD9C",
    provider: DEMO_ADDRESSES.dave,
  },
  {
    job_id: "3",
    shard_id: 3,
    status: "Funded",
    creator: DEMO_ADDRESSES.alice,
    evaluator: "",
    created_at: hoursAgo(4),
    updated_at: hoursAgo(3),
    amount: "2500000000000000000",
    token: "0x83cb612C10a27C09b7a5Ab31B906560B880abD9C",
    provider: "",
  },
  {
    job_id: "4",
    shard_id: 4,
    status: "Disputed",
    creator: DEMO_ADDRESSES.bob,
    evaluator: DEMO_ADDRESSES.dave,
    created_at: hoursAgo(12),
    updated_at: minutesAgo(30),
    amount: "3000000000000000000",
    token: "0x83cb612C10a27C09b7a5Ab31B906560B880abD9C",
    provider: DEMO_ADDRESSES.alice,
  },
  {
    job_id: "5",
    shard_id: 5,
    status: "Refunded",
    creator: DEMO_ADDRESSES.dave,
    evaluator: "",
    created_at: hoursAgo(48),
    updated_at: hoursAgo(40),
    amount: "1000000000000000000",
    token: "0x83cb612C10a27C09b7a5Ab31B906560B880abD9C",
    provider: DEMO_ADDRESSES.carol,
  },
  {
    job_id: "6",
    shard_id: 6,
    status: "Created",
    creator: DEMO_ADDRESSES.alice,
    evaluator: "",
    created_at: minutesAgo(15),
    updated_at: minutesAgo(15),
    amount: "750000000000000000",
    token: "0x83cb612C10a27C09b7a5Ab31B906560B880abD9C",
    provider: "",
  },
  {
    job_id: "7",
    shard_id: 7,
    status: "Completed",
    creator: DEMO_ADDRESSES.bob,
    evaluator: DEMO_ADDRESSES.dave,
    created_at: hoursAgo(30),
    updated_at: hoursAgo(28),
    amount: "2000000000000000000",
    token: "0x83cb612C10a27C09b7a5Ab31B906560B880abD9C",
    provider: DEMO_ADDRESSES.carol,
  },
  {
    job_id: "8",
    shard_id: 8,
    status: "Resolved",
    creator: DEMO_ADDRESSES.carol,
    evaluator: DEMO_ADDRESSES.dave,
    created_at: hoursAgo(60),
    updated_at: hoursAgo(58),
    amount: "4000000000000000000",
    token: "0x83cb612C10a27C09b7a5Ab31B906560B880abD9C",
    provider: DEMO_ADDRESSES.alice,
  },
];

// ----------------------------------------------------------------------------
// Events — mix of all PRISM_* and JOB_* event types for feeds and detail pages.
// ----------------------------------------------------------------------------
// Field mapping (offchain/prismsettle/parser/*.go):
//   ValidationSubmitted: to=agentId, from=validator, value=score, symbol=source, token_address=proofHash
//   Staked:              from=validator, value=amount
//   Slashed:             to=validator, value=slashAmount, from=slasher
//   Disputed:            to=jobId, token_address=reasonHash
//   DisputeResolved:     to=jobId, value=ruling

export const MOCK_EVENTS: ChainEvent[] = [
  // --- PRISM_STAKED (5) ---
  {
    id: 1, chain_name: "monad_testnet", event_type: "PRISM_STAKED",
    from: DEMO_ADDRESSES.alice, to: "", value: score(10), symbol: "",
    token_address: "", tx_hash: TX(1), block_number: 1024001, block_time: hoursAgo(70),
    extra: "",
  },
  {
    id: 2, chain_name: "monad_testnet", event_type: "PRISM_STAKED",
    from: DEMO_ADDRESSES.bob, to: "", value: score(15), symbol: "",
    token_address: "", tx_hash: TX(2), block_number: 1032001, block_time: hoursAgo(58),
    extra: "",
  },
  {
    id: 3, chain_name: "monad_testnet", event_type: "PRISM_STAKED",
    from: DEMO_ADDRESSES.carol, to: "", value: score(8), symbol: "",
    token_address: "", tx_hash: TX(3), block_number: 1045001, block_time: hoursAgo(46),
    extra: "",
  },
  {
    id: 4, chain_name: "monad_testnet", event_type: "PRISM_STAKED",
    from: DEMO_ADDRESSES.dave, to: "", value: score(20), symbol: "",
    token_address: "", tx_hash: TX(4), block_number: 1023001, block_time: hoursAgo(71),
    extra: "",
  },
  {
    id: 5, chain_name: "monad_testnet", event_type: "PRISM_STAKED",
    from: DEMO_ADDRESSES.alice, to: "", value: score(5), symbol: "",
    token_address: "", tx_hash: TX(5), block_number: 1058001, block_time: hoursAgo(34),
    extra: "",
  },

  // --- PRISM_VALIDATION_SUBMITTED (12) — mix of sources 0/1/2 across agents ---
  {
    id: 10, chain_name: "monad_testnet", event_type: "PRISM_VALIDATION_SUBMITTED",
    from: DEMO_ADDRESSES.alice, to: "1", value: score(0.92), symbol: "0",
    token_address: TX(0xaa), tx_hash: TX(10), block_number: 1024010, block_time: hoursAgo(68),
    extra: "jobId=0",
  },
  {
    id: 11, chain_name: "monad_testnet", event_type: "PRISM_VALIDATION_SUBMITTED",
    from: DEMO_ADDRESSES.bob, to: "1", value: score(0.88), symbol: "0",
    token_address: TX(0xbb), tx_hash: TX(11), block_number: 1024011, block_time: hoursAgo(60),
    extra: "jobId=0",
  },
  {
    id: 12, chain_name: "monad_testnet", event_type: "PRISM_VALIDATION_SUBMITTED",
    from: DEMO_ADDRESSES.dave, to: "1", value: score(0.95), symbol: "1",
    token_address: TX(0xcc), tx_hash: TX(12), block_number: 1024012, block_time: hoursAgo(19),
    extra: "jobId=1",
  },
  {
    id: 13, chain_name: "monad_testnet", event_type: "PRISM_VALIDATION_SUBMITTED",
    from: DEMO_ADDRESSES.carol, to: "2", value: score(0.85), symbol: "0",
    token_address: TX(0xdd), tx_hash: TX(13), block_number: 1032010, block_time: hoursAgo(56),
    extra: "jobId=0",
  },
  {
    id: 14, chain_name: "monad_testnet", event_type: "PRISM_VALIDATION_SUBMITTED",
    from: DEMO_ADDRESSES.alice, to: "2", value: score(0.82), symbol: "0",
    token_address: TX(0xee), tx_hash: TX(14), block_number: 1032011, block_time: hoursAgo(48),
    extra: "jobId=0",
  },
  {
    id: 15, chain_name: "monad_testnet", event_type: "PRISM_VALIDATION_SUBMITTED",
    from: DEMO_ADDRESSES.dave, to: "3", value: score(0.78), symbol: "1",
    token_address: TX(0xff), tx_hash: TX(15), block_number: 1045010, block_time: hoursAgo(44),
    extra: "jobId=2",
  },
  {
    id: 16, chain_name: "monad_testnet", event_type: "PRISM_VALIDATION_SUBMITTED",
    from: DEMO_ADDRESSES.bob, to: "3", value: score(0.75), symbol: "0",
    token_address: TX(0x11), tx_hash: TX(16), block_number: 1045011, block_time: hoursAgo(40),
    extra: "jobId=0",
  },
  {
    id: 17, chain_name: "monad_testnet", event_type: "PRISM_VALIDATION_SUBMITTED",
    from: DEMO_ADDRESSES.alice, to: "4", value: score(0.88), symbol: "0",
    token_address: TX(0x22), tx_hash: TX(17), block_number: 1023010, block_time: hoursAgo(66),
    extra: "jobId=0",
  },
  {
    id: 18, chain_name: "monad_testnet", event_type: "PRISM_VALIDATION_SUBMITTED",
    from: DEMO_ADDRESSES.dave, to: "5", value: score(0.65), symbol: "1",
    token_address: TX(0x33), tx_hash: TX(18), block_number: 1058010, block_time: hoursAgo(30),
    extra: "jobId=3",
  },
  {
    id: 19, chain_name: "monad_testnet", event_type: "PRISM_VALIDATION_SUBMITTED",
    from: DEMO_ADDRESSES.carol, to: "5", value: score(0.55), symbol: "2",
    token_address: TX(0x44), tx_hash: TX(19), block_number: 1058011, block_time: hoursAgo(28),
    extra: "jobId=4",
  },
  {
    id: 20, chain_name: "monad_testnet", event_type: "PRISM_VALIDATION_SUBMITTED",
    from: DEMO_ADDRESSES.bob, to: "6", value: score(0.55), symbol: "0",
    token_address: TX(0x55), tx_hash: TX(20), block_number: 1072010, block_time: hoursAgo(20),
    extra: "jobId=0",
  },
  {
    id: 21, chain_name: "monad_testnet", event_type: "PRISM_VALIDATION_SUBMITTED",
    from: DEMO_ADDRESSES.dave, to: "7", value: score(0.32), symbol: "1",
    token_address: TX(0x66), tx_hash: TX(21), block_number: 1081010, block_time: hoursAgo(16),
    extra: "jobId=5",
  },

  // --- PRISM_SLASHED (2) ---
  {
    id: 30, chain_name: "monad_testnet", event_type: "PRISM_SLASHED",
    from: DEMO_ADDRESSES.dave, to: DEMO_ADDRESSES.carol, value: score(2), symbol: "",
    token_address: TX(0x77), tx_hash: TX(30), block_number: 1081011, block_time: hoursAgo(15),
    extra: "evidence=missing_deadline",
  },
  {
    id: 31, chain_name: "monad_testnet", event_type: "PRISM_SLASHED",
    from: DEMO_ADDRESSES.dave, to: DEMO_ADDRESSES.dave, value: score(1.5), symbol: "",
    token_address: TX(0x88), tx_hash: TX(31), block_number: 1095001, block_time: hoursAgo(5),
    extra: "evidence=invalid_score",
  },

  // --- PRISM_DISPUTED (3) ---
  {
    id: 40, chain_name: "monad_testnet", event_type: "PRISM_DISPUTED",
    from: "", to: "4", value: "0", symbol: "",
    token_address: "0xabcd1234abcd1234abcd1234abcd1234abcd1234abcd1234abcd1234abcd1234",
    tx_hash: TX(40), block_number: 1024040, block_time: hoursAgo(11),
    extra: "reason=deliverable_mismatch",
  },
  {
    id: 41, chain_name: "monad_testnet", event_type: "PRISM_DISPUTED",
    from: "", to: "5", value: "0", symbol: "",
    token_address: "0xdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef",
    tx_hash: TX(41), block_number: 1081040, block_time: hoursAgo(14),
    extra: "reason=quality_below_threshold",
  },
  {
    id: 42, chain_name: "monad_testnet", event_type: "PRISM_DISPUTED",
    from: "", to: "8", value: "0", symbol: "",
    token_address: "0xcafebabecafebabecafebabecafebabecafebabecafebabecafebabecafebabe",
    tx_hash: TX(42), block_number: 1095040, block_time: hoursAgo(58),
    extra: "reason=timeout_no_response",
  },

  // --- PRISM_DISPUTE_RESOLVED (2) ---
  {
    id: 50, chain_name: "monad_testnet", event_type: "PRISM_DISPUTE_RESOLVED",
    from: "", to: "5", value: "1", symbol: "",
    token_address: "", tx_hash: TX(50), block_number: 1081050, block_time: hoursAgo(40),
    extra: "ruling=refund_buyer",
  },
  {
    id: 51, chain_name: "monad_testnet", event_type: "PRISM_DISPUTE_RESOLVED",
    from: "", to: "8", value: "2", symbol: "",
    token_address: "", tx_hash: TX(51), block_number: 1095050, block_time: hoursAgo(58),
    extra: "ruling=pay_provider",
  },

  // --- JOB lifecycle events (8) ---
  {
    id: 60, chain_name: "monad_testnet", event_type: "JOB_CREATED",
    from: DEMO_ADDRESSES.alice, to: "1", value: "0", symbol: "",
    token_address: "", tx_hash: TX(60), block_number: 1024060, block_time: hoursAgo(20),
    extra: "agentId=1",
  },
  {
    id: 61, chain_name: "monad_testnet", event_type: "JOB_FUNDED",
    from: DEMO_ADDRESSES.alice, to: "1", value: score(0.5), symbol: "USDC",
    token_address: "", tx_hash: TX(61), block_number: 1024061, block_time: hoursAgo(19),
    extra: "amount=0.5_USDC",
  },
  {
    id: 62, chain_name: "monad_testnet", event_type: "JOB_SUBMITTED",
    from: DEMO_ADDRESSES.bob, to: "1", value: "0", symbol: "",
    token_address: TX(0xa1), tx_hash: TX(62), block_number: 1024062, block_time: hoursAgo(19),
    extra: "deliverableHash=0xa1...",
  },
  {
    id: 63, chain_name: "monad_testnet", event_type: "JOB_COMPLETED",
    from: DEMO_ADDRESSES.dave, to: "1", value: score(0.92), symbol: "",
    token_address: "", tx_hash: TX(63), block_number: 1024063, block_time: hoursAgo(19),
    extra: "evalScore=0.92",
  },
  {
    id: 64, chain_name: "monad_testnet", event_type: "JOB_CREATED",
    from: DEMO_ADDRESSES.carol, to: "2", value: "0", symbol: "",
    token_address: "", tx_hash: TX(64), block_number: 1045060, block_time: hoursAgo(8),
    extra: "agentId=3",
  },
  {
    id: 65, chain_name: "monad_testnet", event_type: "JOB_FUNDED",
    from: DEMO_ADDRESSES.carol, to: "2", value: score(0.3), symbol: "USDC",
    token_address: "", tx_hash: TX(65), block_number: 1045061, block_time: hoursAgo(7),
    extra: "amount=0.3_USDC",
  },
  {
    id: 66, chain_name: "monad_testnet", event_type: "JOB_SUBMITTED",
    from: DEMO_ADDRESSES.dave, to: "2", value: "0", symbol: "",
    token_address: TX(0xb2), tx_hash: TX(66), block_number: 1045062, block_time: hoursAgo(1),
    extra: "deliverableHash=0xb2...",
  },
  {
    id: 67, chain_name: "monad_testnet", event_type: "JOB_CREATED",
    from: DEMO_ADDRESSES.alice, to: "6", value: "0", symbol: "",
    token_address: "", tx_hash: TX(67), block_number: 1095060, block_time: minutesAgo(15),
    extra: "agentId=1",
  },
];

// ----------------------------------------------------------------------------
// Shard activity — spread across ~40 of 256 shards for a realistic heatmap.
// ----------------------------------------------------------------------------
export const MOCK_SHARDS: ShardActivity[] = (() => {
  const out: ShardActivity[] = [];
  // Agents 1-8 map to shards 1, 2, 3, 4, 5, 6, 7, 8 — primary hot spots.
  // Plus some scattered activity on other shards for visual variety.
  const hot = [
    { id: 1, count: 18, ago: minutesAgo(2) },
    { id: 2, count: 12, ago: minutesAgo(5) },
    { id: 3, count: 9, ago: minutesAgo(8) },
    { id: 4, count: 14, ago: minutesAgo(1) },
    { id: 5, count: 6, ago: minutesAgo(15) },
    { id: 6, count: 4, ago: minutesAgo(22) },
    { id: 7, count: 3, ago: minutesAgo(30) },
    { id: 8, count: 2, ago: minutesAgo(45) },
    { id: 42, count: 5, ago: minutesAgo(3) },
    { id: 100, count: 7, ago: minutesAgo(6) },
    { id: 150, count: 3, ago: minutesAgo(12) },
    { id: 200, count: 1, ago: minutesAgo(50) },
    { id: 255, count: 2, ago: minutesAgo(40) },
  ];
  for (const h of hot) {
    out.push({ shard_id: h.id, validation_count: h.count, last_active_at: h.ago });
  }
  return out;
})();

// ----------------------------------------------------------------------------
// Helper functions — used by hooks to shape mock data for queries.
// ----------------------------------------------------------------------------

// When `userAddress` is provided, attribute the first 2 agents/jobs to that
// wallet so /me shows personalized content during demo.
export function getMockAgents(userAddress?: string): AgentVO[] {
  if (!userAddress) return MOCK_AGENTS;
  return MOCK_AGENTS.map((a, i) =>
    i < 2 ? { ...a, owner: userAddress } : a,
  );
}

export function getMockJobs(userAddress?: string): JobVO[] {
  if (!userAddress) return MOCK_JOBS;
  return MOCK_JOBS.map((j, i) =>
    i < 3 ? { ...j, creator: userAddress } : j,
  );
}

export function getMockAgent(agentId: string): AgentVO | undefined {
  return MOCK_AGENTS.find((a) => a.agent_id === agentId);
}

// Filter mock events by event_type; optionally filter by `to` (used by agent
// detail page) or override `from`/`to` to attribute events to a connected
// wallet (used by /me for personalized demo).
export function getMockEvents(opts: {
  eventType?: string;
  // Filter events to those targeting this address/agentId (e.g., agent
  // detail page wants events where to === agentId).
  to?: string;
  // Override `from` on all returned events — used by /me to attribute
  // validations to the connected wallet.
  overrideFrom?: string;
  // Override `to` on all returned events — used by /me to attribute slashes
  // to the connected wallet.
  overrideTo?: string;
  size?: number;
} = {}): { items: ChainEvent[]; total: number } {
  let list = MOCK_EVENTS;
  if (opts.eventType) {
    list = list.filter((e) => e.event_type === opts.eventType);
  }
  if (opts.to) {
    const t = opts.to.toLowerCase();
    list = list.filter((e) => e.to.toLowerCase() === t);
  }
  if (opts.overrideFrom) {
    const from = opts.overrideFrom;
    list = list.map((e) => ({ ...e, from }));
  }
  if (opts.overrideTo) {
    const to = opts.overrideTo;
    list = list.map((e) => ({ ...e, to }));
  }
  const size = opts.size ?? list.length;
  return { items: list.slice(0, size), total: list.length };
}

// Reputation history for a specific agent — returns PRISM_VALIDATION_SUBMITTED
// events targeting that agent, shaped as the backend's /reputation/history
// response.
export function getMockReputationHistory(agentId: string) {
  const events = MOCK_EVENTS.filter(
    (e) => e.event_type === "PRISM_VALIDATION_SUBMITTED" && e.to === agentId,
  );
  return {
    agent_id: agentId,
    events,
    count: events.length,
  };
}

// ----------------------------------------------------------------------------
// Job detail mock — status + timeline for /jobs/[jobId] when backend is down.
// ----------------------------------------------------------------------------

// Returns the mock job status response shape: { status, job_id }.
export function getMockJobStatus(jobId: string): { status: string; job_id: string } | undefined {
  const job = MOCK_JOBS.find((j) => j.job_id === jobId);
  if (!job) return undefined;
  return { status: job.status, job_id: job.job_id };
}

// Build a synthetic timeline for a job by collecting JOB_* events targeting
// that jobId, plus a Created entry derived from the JobVO itself. Returns
// newest-first order to match the backend's /jobs/:id/timeline response.
export function getMockJobTimeline(jobId: string): JobTimelineItem[] {
  const job = MOCK_JOBS.find((j) => j.job_id === jobId);
  if (!job) return [];
  const events = MOCK_EVENTS.filter(
    (e) => e.event_type.startsWith("JOB_") && e.to === jobId,
  );
  const items: JobTimelineItem[] = events.map((e) => ({
    status: e.event_type.replace("JOB_", ""),
    tx_hash: e.tx_hash,
    block_number: e.block_number,
    timestamp: e.block_time,
  }));
  // Always include a Created entry derived from the JobVO itself, in case
  // the JOB_CREATED event is missing for this jobId.
  if (!items.some((i) => i.status === "CREATED")) {
    items.push({
      status: "CREATED",
      tx_hash: TX(900 + Number(jobId)),
      block_number: 1024000,
      timestamp: job.created_at,
    });
  }
  // Newest-first.
  return items.sort((a, b) => b.timestamp - a.timestamp);
}

// ----------------------------------------------------------------------------
// Live feel — dynamic counters and timestamps so the dashboard doesn't look
// like a screenshot. Numbers drift slowly based on Date.now() so each poll
// returns slightly different values, mimicking real on-chain activity.
// ----------------------------------------------------------------------------

// Base validation count = sum of MOCK_SHARDS counts. Add a slow drift that
// increments by 1 every ~7 seconds, capped to a small window so the number
// doesn't grow unbounded during a long demo.
export function getLiveValidationCount(): number {
  const base = MOCK_SHARDS.reduce((s, sh) => s + sh.validation_count, 0);
  const drift = Math.floor(Date.now() / 7000) % 8;
  return base + drift;
}

// Returns a fresh ShardActivity[] where the hottest shard's last_active_at is
// always within the last 30 seconds, so the heatmap visibly pulses on each
// poll. Counts also drift slightly so cells brighten over time and the total
// validation count visibly changes between polls.
//
// Timing: poll interval is 5-6s, so drift cycles every 4s to guarantee each
// poll sees a different value. Tick cycles every 3s so the hot cell moves
// visibly across the heatmap.
export function getLiveShardActivity(): ShardActivity[] {
  const nowSec = Math.floor(Date.now() / 1000);
  const tick = Math.floor(Date.now() / 3000) % MOCK_SHARDS.length;
  // Total drift cycles every 4s, capped to a 10-wide window. Distributed
  // across the first N shards so the sum visibly changes on each poll.
  const totalDrift = Math.floor(Date.now() / 4000) % 10;
  return MOCK_SHARDS.map((sh, i) => {
    // The shard matching the current tick gets a "just now" timestamp.
    const isHot = i === tick;
    const baseAgo = nowSec - sh.last_active_at;
    const last_active_at = isHot ? nowSec - 5 : nowSec - Math.max(baseAgo, 60);
    // First `totalDrift` shards get +1 each so the total visibly drifts.
    const driftShare = i < totalDrift ? 1 : 0;
    const validation_count = sh.validation_count + driftShare + (isHot ? 1 : 0);
    return { shard_id: sh.shard_id, validation_count, last_active_at };
  });
}

// Live agent count — occasionally flickers between 8 and 9 to suggest a new
// registration. Uses a 30-second window so it doesn't feel jittery.
export function getLiveAgentCount(): number {
  const drift = Math.floor(Date.now() / 30000) % 3;
  return MOCK_AGENTS.length + (drift === 2 ? 1 : 0);
}

// ---------------------------------------------------------------------------
// withMockFallback — reduces boilerplate in hooks that try the API first and
// fall back to mock data. Every hook that follows this pattern should use it.
// ---------------------------------------------------------------------------

export interface WithMockFallbackOpts<T> {
  // The actual API call. Return null/undefined to trigger fallback.
  apiCall: () => Promise<T | null | undefined>;
  // Called when apiCall returns null/undefined or throws.
  mockFallback: () => T;
  // Optional: additional validation of the API response.
  isValid?: (res: T) => boolean;
}

export async function withMockFallback<T>(
  opts: WithMockFallbackOpts<T>,
): Promise<T> {
  try {
    const res = await opts.apiCall();
    if (res == null) return opts.mockFallback();
    if (opts.isValid && !opts.isValid(res)) return opts.mockFallback();
    return res;
  } catch {
    return opts.mockFallback();
  }
}
