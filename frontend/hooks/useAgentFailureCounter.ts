"use client";

// AgentFailureCounter — FR-M12 (DEV-PLAN §Phase 8 任务 8.3).
// Aggregates the last 50 invoke-failures for an agent in localStorage so the
// buyer can see "this agent failed 12/50 recent calls → consider avoiding".
// Failures are recorded by the caller via recordFailure().

import { useCallback, useEffect, useState } from "react";

const WINDOW = 50;
const KEY = (agentId: string) => `prismsettle:agent-failures:${agentId}`;

interface FailureRecord {
  ts: number;
  reason: string;
}

function read(agentId: string): FailureRecord[] {
  if (typeof window === "undefined") return [];
  try {
    const raw = window.localStorage.getItem(KEY(agentId));
    if (!raw) return [];
    const arr = JSON.parse(raw) as FailureRecord[];
    return Array.isArray(arr) ? arr.slice(-WINDOW) : [];
  } catch {
    return [];
  }
}

function write(agentId: string, records: FailureRecord[]) {
  if (typeof window === "undefined") return;
  try {
    window.localStorage.setItem(KEY(agentId), JSON.stringify(records.slice(-WINDOW)));
  } catch {
    // Quota exceeded or disabled — silently drop.
  }
}

export interface UseAgentFailureCounter {
  failures: FailureRecord[];
  count: number;
  rate: number; // failures / WINDOW, 0..1
  recordFailure: (reason?: string) => void;
  clear: () => void;
}

export function useAgentFailureCounter(agentId: string | undefined): UseAgentFailureCounter {
  const [failures, setFailures] = useState<FailureRecord[]>([]);

  useEffect(() => {
    if (!agentId) {
      setFailures([]);
      return;
    }
    setFailures(read(agentId));
    const onStorage = (e: StorageEvent) => {
      if (e.key === KEY(agentId)) setFailures(read(agentId));
    };
    window.addEventListener("storage", onStorage);
    return () => window.removeEventListener("storage", onStorage);
  }, [agentId]);

  const recordFailure = useCallback(
    (reason = "invoke_failed") => {
      if (!agentId) return;
      const next = [...read(agentId), { ts: Date.now(), reason }];
      write(agentId, next);
      setFailures(next.slice(-WINDOW));
    },
    [agentId],
  );

  const clear = useCallback(() => {
    if (!agentId) return;
    write(agentId, []);
    setFailures([]);
  }, [agentId]);

  return {
    failures,
    count: failures.length,
    rate: failures.length / WINDOW,
    recordFailure,
    clear,
  };
}
