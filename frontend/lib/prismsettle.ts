"use client";

// Typed API client functions. Each function maps 1:1 to a backend endpoint
// under /api/v1/prismsettle/* (see prismsettle_handler.go RegisterRoutes).
// Components should call these via the SWR hooks in /hooks, not directly —
// that way caching, polling, and revalidation stay in one place.

import { api, getPaginated } from "./api";
import type {
  AgentVO,
  ChainEvent,
  JobTimelineItem,
  JobVO,
  ShardActivity,
  TrustCheckResult,
  TrustThreshold,
  ValidationCount,
} from "./types";

// FR-A06: GET /agents
export function listAgents(params: {
  chainName?: string;
  page?: number;
  size?: number;
}) {
  return getPaginated<AgentVO>("/agents", params);
}

// FR-A07: GET /agents/:agentId
export function getAgent(agentId: string, chainName?: string) {
  return api.get<AgentVO>(`/agents/${encodeURIComponent(agentId)}`, { chainName });
}

// FR-A08: GET /jobs — backend returns the PRISM_JOB_* event stream.
export function listJobs(params: {
  chainName?: string;
  contract?: string;
  page?: number;
  size?: number;
}) {
  return getPaginated<ChainEvent>("/jobs", params);
}

// FR-A09: GET /jobs/:jobId/status
export function getJobStatus(jobId: string, chainName?: string) {
  return api.get<{ status: string; job_id: string }>(
    `/jobs/${encodeURIComponent(jobId)}/status`,
    { chainName },
  );
}

// FR-JM02: GET /jobs/:jobId/timeline — backend returns {job_id, events, count}.
export interface JobTimelineResponse {
  job_id: string;
  events: ChainEvent[];
  count: number;
}
export function getJobTimeline(jobId: string, chainName?: string) {
  return api.get<JobTimelineResponse>(
    `/jobs/${encodeURIComponent(jobId)}/timeline`,
    { chainName },
  );
}

// FR-A11: GET /trust?agent_id=...
export function checkTrust(agentId: string, chainName?: string) {
  return api.get<TrustCheckResult>("/trust", {
    agentId: agentId,
    chainName,
  });
}

// FR-AP11: POST /trust/thresholds
export function setTrustThreshold(body: TrustThreshold) {
  return api.post<TrustThreshold>("/trust/thresholds", body);
}

// FR-A12: GET /reputation/history
// Backend returns {agentId, events: ChainEvent[], count} (NOT a paginated
// envelope). The ChainEvent fields carry the score in `value` and the source
// in `symbol` (0=Validator, 1=Evaluator, 2=Arbitration) — see
// offchain/model/models.go and the listener parser.
export interface ReputationHistoryResponse {
  agent_id: string;
  events: ChainEvent[];
  count: number;
}

export function getReputationHistory(params: {
  agentId: string;
  chainName?: string;
  page?: number;
  size?: number;
}) {
  return api.get<ReputationHistoryResponse>("/reputation/history", params);
}

// --- Demo session (chat-style agent collaboration playback) ---

export interface DemoMessageVO {
  id: number;
  step: number;
  role: string; // buyer/provider/evaluator/system
  content: string;
  report: string; // full deliverable body (markdown), provider submits only
  deliverable_hash: string; // on-chain keccak of the report
  action: string;
  tx_hash: string;
  state: string;
  created_at: number;
}

export interface DemoSessionVO {
  session_id: string;
  job_id: string;
  title: string;
  description: string;
  amount: string;
  token: string;
  provider_agent: string;
  state: string;
  result: string;
  created_at: number;
}

export function createDemoSession(body: {
  title: string;
  description?: string;
  amount: string;
  token?: string;
  provider_agent?: string;
  scenario?: string;
}) {
  return api.post<{ session_id: string; state: string }>("/demo/sessions", body);
}

export function getDemoSession(sessionId: string) {
  return api.get<DemoSessionVO>(`/demo/sessions/${encodeURIComponent(sessionId)}`);
}

/** Most recent demo session that created the given job (history view). */
export function getDemoSessionByJob(jobId: string) {
  return api.get<{ session: DemoSessionVO | null }>("/demo/sessions", { job_id: jobId });
}

export function getDemoMessages(sessionId: string) {
  return api.get<{ session_id: string; count: number; messages: DemoMessageVO[] }>(
    `/demo/sessions/${encodeURIComponent(sessionId)}/messages`,
  );
}

// FR-A04: GET /shards/activity
// Backend returns {chain_name, count, shards: [{shard_id, validations, last_activity}]}.
export interface ShardActivityResponse {
  chain_name?: string;
  count?: number;
  shards?: ShardActivity[];
}
export function getShardActivity(chainName?: string) {
  return api.get<ShardActivityResponse>("/shards/activity", { chainName });
}

// Perf: GET /perf/reorg-feed — backend returns {items, count}.
export interface ReorgFeedResponse {
  items: ChainEvent[];
  count: number;
}
export function getReorgFeed(chainName?: string, limit?: number) {
  return api.get<ReorgFeedResponse>("/perf/reorg-feed", {
    chainName,
    limit,
  });
}

// Existing endpoints (Phase 5/6).
export function getEvents(params: {
  chainName?: string;
  eventType?: string; // backend binds form:"eventType" (camelCase)
  agentId?: string; // matches ChainEvent.to (hex agentId/jobId)
  page?: number;
  size?: number;
}) {
  return getPaginated<ChainEvent>("/events", params);
}

export function getScore(agentId: string, chainName?: string) {
  return api.get<{ agent_id: string; score: string }>("/score", {
    agentId,
    chainName,
  });
}

export function countValidations(chainName?: string) {
  return api.get<ValidationCount>("/validations/count", { chainName });
}

// Grab attempts: agents report grab outcomes (success/failure+reason) so
// operators can see why they lost a job competition.
export interface GrabAttempt {
  id: number;
  chain_name: string;
  agent_id: string;
  job_id: string;
  success: boolean;
  reason: string;
  block_number: number;
  created_at: string;
}

export function listGrabAttempts(params: {
  chainName?: string;
  agentId?: string;
  jobId?: string;
  page?: number;
  size?: number;
}) {
  return api.get<{ list: GrabAttempt[]; total: number; page: number; size: number }>(
    "/grab-attempts",
    params
  );
}

// FR-M11: POST /agent/invoke — proxy to an external agent endpoint.
// Backend binds json tags agent_id/method/params.
export function invokeAgent(body: {
  agent_id: string;
  method: string;
  params?: unknown;
}) {
  return api.post<{ result: unknown }>("/agent/invoke", body);
}
