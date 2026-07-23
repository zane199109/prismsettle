"use client";

// useAgentDetail — fetches a single agent + its recent chain events.
// DEV-PLAN §Phase 8 任务 8.3.
//
// Mock fallback: when the backend is unreachable or returns nothing for the
// requested agentId, the hook falls back to MOCK_AGENTS so the detail page
// always renders content for the demo.

import { usePoll } from "./usePoll";
import { getAgent, getEvents } from "@/lib/prismsettle";
import { getMockAgent, getMockEvents } from "@/lib/mock-data";
import type { AgentVO, ChainEvent, Paginated } from "@/lib/types";

export function useAgentDetail(
  agentId: string | undefined,
  opts: { chainName?: string; intervalMs?: number } = {},
) {
  const { chainName, intervalMs = 10000 } = opts;
  const enabled = Boolean(agentId);
  const {
    data: agent,
    error: agentErr,
    isValidating: agentLoading,
    mutate,
  } = usePoll<AgentVO>(
    enabled ? `agent:${chainName ?? "all"}:${agentId}` : null,
    async () => {
      try {
        const res = await getAgent(agentId!, chainName);
        if (res) return res;
        const mock = getMockAgent(agentId!);
        if (mock) return mock;
        throw new Error("agent not found");
      } catch {
        const mock = getMockAgent(agentId!);
        if (mock) return mock;
        throw new Error("agent not found");
      }
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
  const { chainName, size = 20, intervalMs = 8000 } = opts;
  const enabled = Boolean(agentId);
  const { data, error, isValidating, mutate } = usePoll<Paginated<ChainEvent>>(
    enabled ? `agent-events:${chainName ?? "all"}:${agentId}:${size}` : null,
    async () => {
      try {
        const res = await listAgentEvents(agentId!, chainName, size);
        const filtered = res.items.filter(
          (e) =>
            e.from?.toLowerCase().includes(agentId!.toLowerCase()) ||
            e.to?.toLowerCase().includes(agentId!.toLowerCase()) ||
            e.extra?.toLowerCase().includes(agentId!.toLowerCase()),
        );
        if (filtered.length > 0) {
          return { items: filtered, total: filtered.length, page: 1, size };
        }
        // Fall back to mock events targeting this agent.
        const mock = getMockEvents({ to: agentId!, size });
        return { items: mock.items, total: mock.total, page: 1, size };
      } catch {
        const mock = getMockEvents({ to: agentId!, size });
        return { items: mock.items, total: mock.total, page: 1, size };
      }
    },
    { intervalMs, pauseWhenHidden: true },
  );
  // Filter to events that mention this agent in `from`/`to`/`extra`.
  const filtered = (data?.items ?? []).filter(
    (e) =>
      e.from?.toLowerCase().includes(agentId!.toLowerCase()) ||
      e.to?.toLowerCase().includes(agentId!.toLowerCase()) ||
      e.extra?.toLowerCase().includes(agentId!.toLowerCase()),
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
  const res = await getEvents({ chain_name: chainName, size });
  return res;
}
