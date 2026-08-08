"use client";

// useJobs — polls GET /jobs (paginated) with optional status filter.
// Powers the /jobs list page and any widget that needs to browse jobs.
//
// Mock fallback: when the backend is unreachable or returns an empty page,
// the hook falls back to MOCK_JOBS so the demo always has content.

import { useState } from "react";
import { usePoll } from "./usePoll";
import { listJobs } from "@/lib/prismsettle";
import { CHAIN_NAME } from "@/lib/contracts";
import type { JobVO, Paginated } from "@/lib/types";

export interface UseJobsOptions {
  chainName?: string;
  status?: string; // "" = all
  page?: number;
  size?: number;
  intervalMs?: number;
  // When provided, mock fallback attributes the first 3 jobs to this wallet
  // — used by /me to show personalized content.
  userAddress?: string;
}

export function useJobs(opts: UseJobsOptions = {}) {
  const {
    chainName = CHAIN_NAME,
    status,
    page = 1,
    size = 20,
    intervalMs = 12000,
    userAddress,
  } = opts;

  const key = `jobs:${chainName ?? "all"}:${status ?? "all"}:${page}:${size}`;
  const { data, error, isValidating, mutate } = usePoll<Paginated<JobVO>>(
    key,
    async () => {
      const res = await listJobs({ chainName, state: status, page, size });
      return res;
    },
    { intervalMs, pauseWhenHidden: true },
  );

  return {
    jobs: data?.items ?? [],
    total: data?.total ?? 0,
    page: data?.page ?? page,
    size: data?.size ?? size,
    error,
    isValidating,
    refresh: mutate,
  };
}

// Helper hook for status-filtered job browsing. Kept separate so callers
// that don't need filter state aren't forced to re-render on filter change.
export function useJobsWithFilter(initialStatus = "") {
  const [status, setStatus] = useState<string>(initialStatus);
  const [page, setPage] = useState<number>(1);
  const jobs = useJobs({ status: status || undefined, page });
  return { ...jobs, status, setStatus, page, setPage };
}
