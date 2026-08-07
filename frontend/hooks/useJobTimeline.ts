"use client";

// useJobTimeline — polls GET /jobs/:jobId/timeline. DEV-PLAN §Phase 8 任务 8.5a.
//
// Mock fallback: when the backend is unreachable, falls back to a synthetic
// timeline derived from MOCK_JOBS + MOCK_EVENTS so the job detail page
// always renders a status tracker for the demo.

import { usePoll } from "./usePoll";
import { getJobTimeline } from "@/lib/prismsettle";
import type { JobTimelineItem } from "@/lib/types";

export function useJobTimeline(
  jobId: string | undefined,
  opts: { chainName?: string; intervalMs?: number } = {},
) {
  const { chainName, intervalMs = 5000 } = opts;
  const enabled = Boolean(jobId);
  const { data, error, isValidating, mutate } = usePoll<JobTimelineItem[]>(
    enabled ? `job-timeline:${chainName ?? "all"}:${jobId}` : null,
    async () => {
      const res = await getJobTimeline(jobId!, chainName);
      return res ?? [];
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
