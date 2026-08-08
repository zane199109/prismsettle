"use client";

// GrabAttemptsList — visualizes the grab competition for a job or an agent:
// every attempt with success/failure and the human-readable reason, so
// operators see exactly why their agent lost.

import { CheckCircle2, XCircle } from "lucide-react";
import { useGrabAttempts } from "@/hooks/useGrabAttempts";
import { formatAgentId, formatTime } from "@/lib/utils";
import { Skeleton } from "@/components/ui/skeleton";

interface Props {
  agentId?: string;
  jobId?: string;
  title?: string;
}

export function GrabAttemptsList({ agentId, jobId, title }: Props) {
  const { attempts, isValidating } = useGrabAttempts({ agentId, jobId, size: 50 });

  if (isValidating && attempts.length === 0) {
    return (
      <div className="space-y-2">
        <Skeleton className="h-5 w-40" />
        <Skeleton className="h-10 w-full" />
        <Skeleton className="h-10 w-full" />
      </div>
    );
  }

  if (attempts.length === 0) {
    return (
      <div className="rounded-lg border border-white/10 bg-prism-surface/40 p-4 text-sm text-white/50">
        {title ?? "抢单记录"}：暂无记录 —— agent 服务尚未尝试抢单。
      </div>
    );
  }

  return (
    <div className="rounded-lg border border-white/10 bg-prism-surface/40 p-4">
      <div className="mb-3 flex items-center justify-between">
        <h3 className="text-sm font-medium text-white">{title ?? "抢单记录"}</h3>
        <span className="text-[11px] text-white/40">{attempts.length} 次尝试</span>
      </div>
      <ul className="space-y-2">
        {attempts.map((a) => (
          <li
            key={a.id}
            className={`flex items-start gap-3 rounded-lg border px-3 py-2 text-xs ${
              a.success
                ? "border-emerald-500/30 bg-emerald-500/5"
                : "border-red-500/25 bg-red-500/5"
            }`}
          >
            {a.success ? (
              <CheckCircle2 className="mt-0.5 h-4 w-4 shrink-0 text-emerald-400" />
            ) : (
              <XCircle className="mt-0.5 h-4 w-4 shrink-0 text-red-400" />
            )}
            <div className="min-w-0 flex-1">
              <div className="flex flex-wrap items-baseline gap-x-2 text-white/80">
                <span className="font-mono">
                  {formatAgentId(a.agent_id)}
                </span>
                <span className="text-white/40">→</span>
                <span className="font-mono">{a.job_id.slice(0, 12)}…</span>
                <span className="ml-auto text-white/35">{formatTime(new Date(a.created_at).getTime() / 1000)}</span>
              </div>
              <div className={`mt-0.5 ${a.success ? "text-emerald-300" : "text-red-300"}`}>
                {a.success ? "抢单成功 ✓" : (a.reason || "抢单失败")}
              </div>
            </div>
          </li>
        ))}
      </ul>
    </div>
  );
}
