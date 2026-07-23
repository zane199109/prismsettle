"use client";

// useTrustCheck — calls GET /trust?agent_id=... and returns the three-state
// decision (allow / deny / review) plus the active thresholds. Used by the
// TrustGate component on the New Job page. DEV-PLAN §Phase 8 任务 8.4.

import { usePoll } from "./usePoll";
import { checkTrust } from "@/lib/prismsettle";
import type { TrustCheckResult } from "@/lib/types";

export function useTrustCheck(
  agentId: string | undefined,
  opts: { chainName?: string; intervalMs?: number; enabled?: boolean } = {},
) {
  const { chainName, intervalMs = 30000, enabled = true } = opts;
  const key = `trust:${chainName ?? "all"}:${agentId ?? "_"}`;
  const { data, error, isValidating, mutate } = usePoll<TrustCheckResult>(
    enabled && agentId ? key : null,
    () => checkTrust(agentId!, chainName),
    { intervalMs, pauseWhenHidden: true },
  );
  return {
    result: data,
    error,
    isValidating,
    refresh: mutate,
  };
}
