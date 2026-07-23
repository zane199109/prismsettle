"use client";

// useValidationRecords — fetches PRISM_VALIDATION_SUBMITTED events for the
// validator console's "recent validations" panel. Each row represents one
// submitValidation call (source=0 validator path; the Evaluator and
// Arbitration paths use PRISM_AGGREGATED events instead).
//
// The `extra` JSON field on ChainEvent may carry { score, source, job_id }
// depending on listener instrumentation; we extract what we can and fall
// back gracefully.

import { useEvents } from "./useEvents";
import type { ChainEvent } from "@/lib/types";

export interface ValidationRecord {
  tx_hash: string;
  block_number: number;
  block_time: number;
  from: string;
  agent_id: string;
  score: string;
  source: number; // 0=Validator, 1=Evaluator, 2=Arbitration
  job_id: string;
}

function parseRecord(e: ChainEvent): ValidationRecord {
  let score = "";
  let source = 0;
  let job_id = "";
  let agent_id = "";
  try {
    if (e.extra) {
      const obj = JSON.parse(e.extra) as {
        score?: string;
        source?: number;
        job_id?: string;
        agent_id?: string;
      };
      score = obj.score ?? "";
      source = obj.source ?? 0;
      job_id = obj.job_id ?? "";
      agent_id = obj.agent_id ?? "";
    }
  } catch {
    // not JSON — keep defaults
  }
  return {
    tx_hash: e.tx_hash,
    block_number: e.block_number,
    block_time: e.block_time,
    from: e.from,
    agent_id: agent_id || e.to,
    score,
    source,
    job_id,
  };
}

export function useValidationRecords(
  opts: { chainName?: string; size?: number; intervalMs?: number } = {},
) {
  const { chainName, size = 30, intervalMs = 10000 } = opts;
  const { events, total, isValidating, error, refresh } = useEvents({
    chainName,
    eventType: "PRISM_VALIDATION_SUBMITTED",
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
