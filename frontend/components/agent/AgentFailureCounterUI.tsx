"use client";

// AgentFailureCounterUI — visual badge for FR-M12. Shows the recent failure
// rate (last 50 calls) with a colored bar: green <10%, amber <30%, red ≥30%.

import { AlertTriangle, Trash2 } from "lucide-react";
import { useAgentFailureCounter } from "@/hooks/useAgentFailureCounter";
import { cn } from "@/lib/utils";

const WINDOW = 50;

export function AgentFailureCounterUI({ agentId }: { agentId: string }) {
  const { count, rate, clear } = useAgentFailureCounter(agentId);
  const pct = Math.round(rate * 100);
  const tone =
    rate >= 0.3 ? "text-red-400" : rate >= 0.1 ? "text-amber-400" : "text-emerald-400";
  const bar =
    rate >= 0.3
      ? "bg-red-400"
      : rate >= 0.1
        ? "bg-amber-400"
        : "bg-emerald-400";

  return (
    <div className="rounded-lg border border-white/10 bg-prism-surface/40 p-4">
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-2">
          <AlertTriangle className={cn("h-4 w-4", tone)} />
          <span className="text-sm font-medium text-white">Recent Failures</span>
        </div>
        <button
          type="button"
          onClick={clear}
          className="text-white/40 hover:text-white"
          title="Clear failure log"
        >
          <Trash2 className="h-3.5 w-3.5" />
        </button>
      </div>
      <div className="mt-3 flex items-baseline gap-2">
        <span className={cn("font-mono text-2xl font-bold", tone)}>{count}</span>
        <span className="text-xs text-white/40">/ {WINDOW} recent calls</span>
        <span className={cn("ml-auto text-sm font-medium", tone)}>{pct}%</span>
      </div>
      <div className="mt-2 h-1.5 overflow-hidden rounded-full bg-white/5">
        <div
          className={cn("h-full transition-all", bar)}
          style={{ width: `${pct}%` }}
        />
      </div>
      <p className="mt-2 text-[11px] text-white/40">
        Locally aggregated from your invoke attempts. Use to avoid flaky agents.
      </p>
    </div>
  );
}
