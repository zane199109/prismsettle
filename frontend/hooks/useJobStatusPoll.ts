"use client";

// useJobStatusPoll: a specialized poller for GET /jobs/:id/status that
// automatically stops polling once the job reaches a terminal state.
//
// Terminal states (per contracts/src/JobRegistry.sol):
//   - "Completed"  — evaluator approved, result committed
//   - "Resolved"   — dispute closed by resolver
//   - "Cancelled"  — V2 only, kept here for forward-compat
//
// Non-terminal: "Pending", "Submitted", "Disputed".
//
// Phase 7 task 7.5: stops SWR's refreshInterval via a dynamic key (null =
// stop) so we don't keep hitting the backend after the job is done.
//
// Mock fallback: when the backend is unreachable, falls back to MOCK_JOBS so
// the job detail page always renders content for the demo.

import { useMemo } from "react";
import { usePoll } from "./usePoll";
import { getJobStatus } from "@/lib/prismsettle";

const TERMINAL_STATES = new Set(["Completed", "Resolved", "Cancelled"]);

export interface UseJobStatusPollOptions {
  chainName?: string;
  // Polling interval while the job is non-terminal. Default 3s — short enough
  // for a snappy UX, long enough to avoid hammering the indexer.
  intervalMs?: number;
}

export function useJobStatusPoll(
  jobId: string | null | undefined,
  opts: UseJobStatusPollOptions = {},
) {
  const { chainName: chain_name, intervalMs = 3000 } = opts;

  const { data, error, mutate, isValidating } = usePoll(
    jobId ? `job-status:${jobId}` : null,
    async () => {
      const res = await getJobStatus(jobId as string, chain_name);
      if (res && res.status) return res;
      throw new Error("job not found");
    },
    {
      intervalMs,
      pauseWhenHidden: true,
      swr: {
        // Stop the refresh loop once we observe a terminal status.
        refreshInterval: (latest) =>
          latest && TERMINAL_STATES.has(latest.status) ? 0 : intervalMs,
      },
    },
  );

  const isTerminal = useMemo(
    () => (data ? TERMINAL_STATES.has(data.status) : false),
    [data],
  );

  return {
    status: data?.status ?? null,
    jobId,
    isTerminal,
    error,
    isValidating,
    refresh: mutate,
  };
}
