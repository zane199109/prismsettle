"use client";

// useReputationHistory — polls GET /reputation/history?agentId=... for the
// score-over-time data on the agent detail page.
//
// The backend returns {agent_id, events: ChainEvent[], count} where each
// event is either:
//   - PRISM_VALIDATION_SUBMITTED (buyer/evaluator rating; carries job_id)
//   - PRISM_AGGREGATED          (post-aggregation score — the "after" value)
//
// We project those into ReputationPoint[] so the summary list can pair each
// rated job with the score AFTER aggregation (the "after" value only — the
// raw rating input is never shown).

import { usePoll } from "./usePoll";
import { getReputationHistory } from "@/lib/prismsettle";
import { CHAIN_NAME } from "@/lib/contracts";
import type { ChainEvent } from "@/lib/types";

export interface ReputationPoint {
  score: string;
  timestamp: number;
  tx_hash: string;
  block_number: number;
  event_type: string;
  job_id: string;
}

function toPoint(e: ChainEvent): ReputationPoint {
  return {
    score: e.value || "0",
    timestamp: e.block_time,
    tx_hash: e.tx_hash,
    block_number: e.block_number,
    event_type: e.event_type,
    job_id: e.job_id ?? "",
  };
}

export function useReputationHistory(
  agentId: string | undefined,
  opts: { chainName?: string; size?: number; intervalMs?: number } = {},
) {
  const { chainName = CHAIN_NAME, size = 50, intervalMs = 15000 } = opts;
  const enabled = Boolean(agentId);
  const key = `rep-history:${chainName ?? "all"}:${agentId ?? "_"}:${size}`;
  const { data, error, isValidating, mutate } = usePoll(
    enabled ? key : null,
    async () => {
      const res = await getReputationHistory({ agentId: agentId!, chainName, size });
      return res;
    },
    { intervalMs, pauseWhenHidden: true },
  );

  const points = (data?.events ?? []).map(toPoint);

  return {
    points,
    total: data?.count ?? 0,
    error,
    isValidating,
    refresh: mutate,
  };
}
