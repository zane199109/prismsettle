"use client";

// useReputationHistory — polls GET /reputation/history?agentId=... for the
// score-over-time chart on the agent detail page. DEV-PLAN §Phase 8 任务 8.3.
//
// The backend returns {agent_id, events: ChainEvent[], count} where each
// ChainEvent carries:
//   - `value`     = the new reputation score (uint256, 1e18-scaled)
//   - `symbol`    = the source ("0"=Validator, "1"=Evaluator, "2"=Arbitration)
//   - `event_type`= PRISM_VALIDATION_SUBMITTED | PRISM_AGGREGATED | PRISM_SLASHED
//   - `block_time`= unix seconds
//
// We project those into a flat ReputationPoint[] that ScoreHistoryChart
// consumes. Coloring by source (FR-M09) is handled in the chart.
//
// Mock fallback: when the backend is unreachable or returns no events,
// the hook falls back to MOCK_EVENTS so the chart always has data.

import { usePoll } from "./usePoll";
import { getReputationHistory } from "@/lib/prismsettle";
import { getMockReputationHistory } from "@/lib/mock-data";
import type { ChainEvent } from "@/lib/types";

export type ReputationSource = 0 | 1 | 2; // Validator | Evaluator | Arbitration

export interface ReputationPoint {
  score: string;
  timestamp: number;
  tx_hash: string;
  block_number: number;
  event_type: string;
  source: ReputationSource;
}

function toPoint(e: ChainEvent): ReputationPoint {
  let source: ReputationSource = 0;
  const s = Number(e.symbol);
  if (s === 1 || s === 2) source = s as ReputationSource;
  // Fallback: infer source from event_type when symbol is empty.
  if (!e.symbol) {
    if (e.event_type === "PRISM_AGGREGATED") source = 1;
    else if (e.event_type === "PRISM_SLASHED") source = 2;
    // Console.warn so operators can detect stale parser state during
    // development — expected until Phase 10 adds symbol to all events.
    console.warn("useReputationHistory: empty symbol on event %s, inferred source=%d", e.tx_hash?.slice(0, 10), source);
  }
  return {
    score: e.value || "0",
    timestamp: e.block_time,
    tx_hash: e.tx_hash,
    block_number: e.block_number,
    event_type: e.event_type,
    source,
  };
}

export function useReputationHistory(
  agentId: string | undefined,
  opts: { chainName?: string; size?: number; intervalMs?: number } = {},
) {
  const { chainName, size = 50, intervalMs = 15000 } = opts;
  const enabled = Boolean(agentId);
  const key = `rep-history:${chainName ?? "all"}:${agentId ?? "_"}:${size}`;
  const { data, error, isValidating, mutate } = usePoll(
    enabled ? key : null,
    async () => {
      try {
        const res = await getReputationHistory({ agent_id: agentId!, chain_name: chainName, size });
        if (res.events && res.events.length > 0) return res;
        return getMockReputationHistory(agentId!);
      } catch {
        return getMockReputationHistory(agentId!);
      }
    },
    { intervalMs, pauseWhenHidden: true },
  );

  // Backend returns events newest-first; consumers may want either order so
  // we keep the original order and let the chart reverse if needed.
  const points = (data?.events ?? []).map(toPoint);

  return {
    points,
    total: data?.count ?? 0,
    error,
    isValidating,
    refresh: mutate,
  };
}
