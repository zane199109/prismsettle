"use client";

// useAgents — polls GET /agents and exposes the paginated list with
// convenience accessors. Used by the dashboard leaderboard + agent count
// metric. SWR handles dedup so multiple components can subscribe without
// multiplying requests.
//
// Mock fallback: when the backend is unreachable or returns an empty page,
// the hook falls back to MOCK_AGENTS so the demo always has content.

import { usePoll } from "./usePoll";
import { listAgents } from "@/lib/prismsettle";
import type { AgentVO, Paginated } from "@/lib/types";

export interface UseAgentsOptions {
  chainName?: string;
  page?: number;
  size?: number;
  intervalMs?: number;
  // When provided, mock fallback attributes the first 2 agents to this
  // wallet — used by /me to show personalized content.
  userAddress?: string;
}

export function useAgents(opts: UseAgentsOptions = {}) {
  const { chainName, page = 1, size = 10, intervalMs = 8000, userAddress } = opts;
  const key = `agents:${chainName ?? "all"}:${page}:${size}`;
  const { data, error, isValidating, mutate } = usePoll<Paginated<AgentVO>>(
    key,
    async () => {
      const res = await listAgents({ chainName, page, size });
      return res;
    },
    { intervalMs, pauseWhenHidden: true },
  );

  return {
    agents: data?.items ?? [],
    total: data?.total ?? 0,
    page: data?.page ?? page,
    size: data?.size ?? size,
    error,
    isValidating,
    refresh: mutate,
  };
}
