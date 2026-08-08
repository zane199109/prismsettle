"use client";

// useAgentDetail — fetches a single agent + its recent chain events.
// DEV-PLAN §Phase 8 任务 8.3.
//
// Mock fallback: when the backend is unreachable or returns nothing for the
// requested agentId, the hook falls back to MOCK_AGENTS so the detail page
// always renders content for the demo.

import { usePoll } from "./usePoll";
import { getAgent, getEvents } from "@/lib/prismsettle";
import { CHAIN_NAME } from "@/lib/contracts";
import type { AgentVO, ChainEvent, Paginated } from "@/lib/types";

export function useAgentDetail(
  agentId: string | undefined,
  opts: { chainName?: string; intervalMs?: number } = {},
) {
  const { chainName = CHAIN_NAME, intervalMs = 10000 } = opts;
  const enabled = Boolean(agentId);
  const {
    data: agent,
    error: agentErr,
    isValidating: agentLoading,
    mutate,
  } = usePoll<AgentVO>(
    enabled ? `agent:${chainName ?? "all"}:${agentId}` : null,
    async () => {
      const res = await getAgent(agentId as string, chainName);
      if (res) return res;
      throw new Error("agent not found");
    },
    { intervalMs, pauseWhenHidden: true },
  );
  return {
    agent,
    error: agentErr,
    isValidating: agentLoading,
    refresh: mutate,
  };
}

export function useAgentEvents(
  agentId: string | undefined,
  opts: { chainName?: string; size?: number; intervalMs?: number } = {},
) {
  const { chainName = CHAIN_NAME, size = 20, intervalMs = 8000 } = opts;
  const enabled = Boolean(agentId);
  const { data, error, isValidating, mutate } = usePoll<Paginated<ChainEvent>>(
    enabled ? `agent-events:${chainName ?? "all"}:${agentId}:${size}` : null,
    async () => {
      const res = await listAgentEvents(agentId as string, chainName, size);
      const idLower = (agentId ?? "").toLowerCase();
      const filtered = res.items.filter(
        (e) =>
          e.from?.toLowerCase().includes(idLower) ||
          e.to?.toLowerCase().includes(idLower) ||
          e.extra?.toLowerCase().includes(idLower),
      );
      return { items: filtered, total: filtered.length, page: 1, size };
    },
    { intervalMs, pauseWhenHidden: true },
  );
  // Client-side filter: backend lacks server-side agent_id filter so we
  // fetch a page and filter. The fetcher already falls back to mock events
  // targeting this agent when the backend returns nothing.
  const id = agentId ?? ""; // guarded by SWR key (null when disabled)
  const filtered = (data?.items ?? []).filter(
    (e: ChainEvent) =>
      e.from?.toLowerCase().includes(id.toLowerCase()) ||
      e.to?.toLowerCase().includes(id.toLowerCase()) ||
      e.extra?.toLowerCase().includes(id.toLowerCase()),
  );
  return {
    events: filtered,
    total: filtered.length,
    error,
    isValidating,
    refresh: mutate,
  };
}

// Helper: getEvents does not support agent_id filter server-side yet, so we
// fetch a page and filter client-side. Phase 10 will add a server filter.
async function listAgentEvents(
  agentId: string,
  chainName: string | undefined,
  size: number,
): Promise<Paginated<ChainEvent>> {
  const res = await getEvents({ chainName, size });
  return res;
}
