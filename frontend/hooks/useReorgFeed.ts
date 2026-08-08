"use client";

// useReorgFeed — polls GET /perf/reorg-feed. DEV-PLAN §Phase 8 任务 8.7.

import { usePoll } from "./usePoll";
import { getReorgFeed } from "@/lib/prismsettle";
import { CHAIN_NAME } from "@/lib/contracts";
import type { ChainEvent } from "@/lib/types";

export function useReorgFeed(
  opts: { chainName?: string; limit?: number; intervalMs?: number } = {},
) {
  const { chainName = CHAIN_NAME, limit = 30, intervalMs = 4000 } = opts;
  const { data, error, isValidating, mutate } = usePoll<ChainEvent[]>(
    `reorg-feed:${chainName ?? "all"}:${limit}`,
    async () => {
      const res = await getReorgFeed(chainName, limit);
      return res?.items ?? [];
    },
    { intervalMs, pauseWhenHidden: true },
  );
  return {
    events: data ?? [],
    error,
    isValidating,
    refresh: mutate,
  };
}
