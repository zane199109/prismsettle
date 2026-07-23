"use client";

// useSlashes — fetches PRISM_SLASHED events for the validator console's
// "slash history" panel. Slash events fire when an arbitration ruling
// penalizes an agent (penalty = max(0.2e18, currentScore * 30%)).
//
// The `extra` field may carry { agent_id, penalty, reason }. We surface
// whatever the listener captured and leave remaining fields blank.

import { useEvents } from "./useEvents";
import type { ChainEvent } from "@/lib/types";

export interface SlashRecord {
  tx_hash: string;
  block_number: number;
  block_time: number;
  agent_id: string;
  penalty: string;
  reason: string;
}

function parseRecord(e: ChainEvent): SlashRecord {
  let agent_id = "";
  let penalty = "";
  let reason = "";
  try {
    if (e.extra) {
      const obj = JSON.parse(e.extra) as {
        agent_id?: string;
        penalty?: string;
        reason?: string;
      };
      agent_id = obj.agent_id ?? "";
      penalty = obj.penalty ?? "";
      reason = obj.reason ?? "";
    }
  } catch {
    // keep defaults
  }
  return {
    tx_hash: e.tx_hash,
    block_number: e.block_number,
    block_time: e.block_time,
    agent_id: agent_id || e.to,
    penalty,
    reason,
  };
}

export function useSlashes(
  opts: { chainName?: string; size?: number; intervalMs?: number } = {},
) {
  const { chainName, size = 30, intervalMs = 15000 } = opts;
  const { events, total, isValidating, error, refresh } = useEvents({
    chainName,
    eventType: "PRISM_SLASHED",
    size,
    intervalMs,
  });

  const records = events.map(parseRecord);

  return {
    records,
    total,
    isValidating,
    error,
    refresh,
  };
}
