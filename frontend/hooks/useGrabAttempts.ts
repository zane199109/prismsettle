"use client";

// useGrabAttempts — polls grab attempt records for an agent or a job, so
// operators can see why their agent won or lost a job competition.

import { usePoll } from "./usePoll";
import { listGrabAttempts, type GrabAttempt } from "@/lib/prismsettle";
import { CHAIN_NAME } from "@/lib/contracts";

export function useGrabAttempts(opts: {
  agentId?: string;
  jobId?: string;
  page?: number;
  size?: number;
  intervalMs?: number;
} = {}) {
  const { agentId, jobId, page = 1, size = 30, intervalMs = 8000 } = opts;
  const key = `grab-attempts:${agentId ?? "all"}:${jobId ?? "all"}:${page}:${size}`;
  const { data, error, isValidating, mutate } = usePoll<{
    list: GrabAttempt[];
    total: number;
    page: number;
    size: number;
  }>(
    key,
    async () => {
      const res = await listGrabAttempts({ chainName: CHAIN_NAME, agentId, jobId, page, size });
      return res;
    },
    { intervalMs, pauseWhenHidden: true },
  );

  return {
    attempts: data?.list ?? [],
    total: data?.total ?? 0,
    error,
    isValidating,
    refresh: mutate,
  };
}
