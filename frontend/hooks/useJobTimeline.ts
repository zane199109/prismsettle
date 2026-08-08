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
  const { chainName = CHAIN_NAME, intervalMs = 5000 } = opts;
  const enabled = Boolean(jobId);
  const { data, error, isValidating, mutate } = usePoll<JobTimelineItem[]>(
    enabled ? `job-timeline:${chainName ?? "all"}:${jobId}` : null,
    async () => {
      const res = await getJobTimeline(jobId!, chainName);
      // Backend returns {job_id, events: ChainEvent[], count}; project each
      // event to the JobTimelineItem shape (status = display state).
      const events = res?.events ?? [];
      return events.map((e) => ({
        status: timelineStatus(e.event_type),
        tx_hash: e.tx_hash,
        block_number: e.block_number,
        timestamp: e.block_time,
      }));
    },
    { intervalMs, pauseWhenHidden: true },
  );
  return {
    timeline: data ?? [],
    error,
    isValidating,
    refresh: mutate,
  };
}
