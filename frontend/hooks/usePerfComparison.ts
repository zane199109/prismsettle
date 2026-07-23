"use client";

// usePerfComparison — polls GET /perf/v0-v1-comparison.
// DEV-PLAN §Phase 8 任务 8.7.

import { usePoll } from "./usePoll";
import { api } from "@/lib/api";

export interface PerfComparison {
  v0_abort_rate: number;
  v1_abort_rate: number;
  v0_throughput: number;
  v1_throughput: number;
  meets_fr_t06: boolean;
  source: string; // "mock" or "benchmark"
}

export function usePerfComparison(intervalMs = 30000) {
  const { data, error, isValidating, mutate } = usePoll<PerfComparison>(
    "perf:v0-v1-comparison",
    () => api.get<PerfComparison>("/perf/v0-v1-comparison"),
    { intervalMs, pauseWhenHidden: true },
  );
  return {
    comparison: data,
    error,
    isValidating,
    refresh: mutate,
  };
}
