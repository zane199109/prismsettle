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

// FR-A08: GET /jobs
export function listJobs(params: {
  chainName?: string;
  state?: string;
  page?: number;
  size?: number;
}) {
  return getPaginated<JobVO>("/jobs", params);
}

// FR-A09: GET /jobs/:jobId/status
export function getJobStatus(jobId: string, chainName?: string) {
  return api.get<{ status: string; job_id: string }>(
    `/jobs/${encodeURIComponent(jobId)}/status`,
    { chainName },
  );
}

// FR-JM02: GET /jobs/:jobId/timeline
export function getJobTimeline(jobId: string, chainName?: string) {
  return api.get<JobTimelineItem[]>(
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
  agentId: string;
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

// FR-A04: GET /shards/activity
export function getShardActivity(chainName?: string) {
  return api.get<ShardActivity[]>("/shards/activity", { chainName });
}

// Perf: GET /perf/reorg-feed — Phase 9 wires this to the reorg-aware feed.
export function getReorgFeed(chainName?: string, limit?: number) {
  return api.get<ChainEvent[]>("/perf/reorg-feed", {
    chainName,
    limit,
  });
}

// Existing endpoints (Phase 5/6).
export function getEvents(params: {
  chainName?: string;
  event_type?: string;
  page?: number;
  size?: number;
}) {
  return getPaginated<ChainEvent>("/events", params);
}

export function getScore(agentId: string, chainName?: string) {
  return api.get<{ agentId: string; score: string }>("/score", {
    agentId,
    chainName,
  });
}

export function countValidations(chainName?: string) {
  return api.get<ValidationCount>("/validations/count", { chainName });
}

// FR-M11: POST /agent/invoke — proxy to an external agent endpoint.
export function invokeAgent(body: {
  agentId: string;
  method: string;
  params?: unknown;
}) {
  return api.post<{ result: unknown; decision_id: string }>(
    "/agent/invoke",
    body,
  );
}
