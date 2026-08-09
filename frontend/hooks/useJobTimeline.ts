"use client";

// useJobTimeline — polls GET /jobs/:jobId/timeline. DEV-PLAN §Phase 8 任务 8.5a.
//
// Mock fallback: when the backend is unreachable, falls back to a synthetic
// timeline derived from MOCK_JOBS + MOCK_EVENTS so the job detail page
// always renders a status tracker for the demo.

import { usePoll } from "./usePoll";
import { getJobTimeline } from "@/lib/prismsettle";
import { CHAIN_NAME } from "@/lib/contracts";
import type { JobTimelineItem } from "@/lib/types";

// Normalize a backend event_type to a display state. Job events come as
// PRISM_JOB_* and arbitration events as PRISM_* — both must land on the same
// Title-case space (FUNDED→Funded, DISPUTED→Disputed, ...) so consumers
// (JobStatusTracker, JobEventTimeline) can match on stable values.
const TIMELINE_STATUS: Record<string, string> = {
  CREATED: "Created",
  FUNDED: "Funded",
  ASSIGNED: "Assigned",
  SUBMITTED: "Submitted",
  REJECTED: "Rejected",
  COMPLETED: "Completed",
  REFUNDED: "Refunded",
  DISPUTED: "Disputed",
  ARBITRATOR_SELECTED: "ArbitratorSelected",
  DISPUTE_RESOLVED: "DisputeResolved",
  DISPUTE_RESOLVED_ANNOUNCED: "DisputeResolved",
  ARBITRATION_EXECUTED: "ArbitrationExecuted",
};

export function timelineStatus(eventType: string | undefined): string {
  const raw = (eventType ?? "").replace(/^PRISM_JOB_/, "").replace(/^PRISM_/, "");
  return TIMELINE_STATUS[raw] ?? raw;
}

export function useJobTimeline(
  jobId: string | undefined,
  opts: { chainName?: string; intervalMs?: number } = {},
) {
  const { chainName = CHAIN_NAME, intervalMs = 3000 } = opts;
  const enabled = Boolean(jobId);
  const { data, error, isValidating, mutate, isMounted } = usePoll<JobTimelineItem[]>(
    enabled ? `job-timeline:${chainName ?? "all"}:${jobId}` : null,
    async () => {
      const res = await getJobTimeline(jobId!, chainName);
      const events = res?.events ?? [];
      // Dedupe by tx: one action = one transaction = one row. Some actions
      // emit the same event from two contracts (e.g. resolveDispute emits
      // DisputeResolved from the Hook AND DisputeResolvedAnnounced from the
      // Job contract in the same tx) — showing both would duplicate the row.
      const seen = new Set<string>();
      const items: {
        status: string;
        tx_hash: string;
        block_number: number;
        timestamp: number;
        value?: string;
        token_address?: string;
        symbol?: string;
      }[] = [];
      for (const e of events) {
        if (seen.has(e.tx_hash)) continue;
        seen.add(e.tx_hash);
        items.push({
          status: timelineStatus(e.event_type),
          tx_hash: e.tx_hash,
          block_number: e.block_number,
          timestamp: e.block_time,
          value: e.value,
          token_address: e.token_address,
          symbol: e.symbol,
        });
      }
      return items;
    },
    { intervalMs, pauseWhenHidden: true },
  );
  return {
    timeline: data ?? [],
    error,
    isValidating,
    isMounted,
    refresh: mutate,
  };
}
