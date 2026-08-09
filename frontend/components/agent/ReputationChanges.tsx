"use client";

// ReputationChanges — every reputation modification for an agent as a list
// of rows bound to the job that triggered it: the buyer's rating score AND
// the evaluation comment the buyer gave (fetched from the demo session that
// rated the job). Each row links to the job's detail page.

import { useEffect, useState } from "react";
import Link from "next/link";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { formatAgentId, formatBigInt, formatTime } from "@/lib/utils";
import { useReputationHistory } from "@/hooks/useReputationHistory";
import { getDemoMessages, getDemoSessionByJob } from "@/lib/prismsettle";

interface Row {
  job_id: string;
  score: string;
  timestamp: number;
  comment: string;
}

export function ReputationChanges({ agentId, limit = 8 }: { agentId: string; limit?: number }) {
  const { points, isValidating } = useReputationHistory(agentId, {
    size: 50,
    intervalMs: 10000,
  });
  const [rows, setRows] = useState<Row[]>([]);

  // One row per RATED job: the buyer's rating event (VALIDATION_SUBMITTED)
  // carries the job_id and the score the buyer gave that deliverable. The
  // evaluation comment comes from the demo session that ran the job (the
  // buyer's complete message).
  const base: { job_id: string; score: string; timestamp: number }[] = [];
  const sorted = [...points].sort((a, b) => a.timestamp - b.timestamp);
  for (const p of sorted) {
    if (p.event_type === "PRISM_VALIDATION_SUBMITTED" && p.job_id) {
      base.push({ job_id: p.job_id, score: p.score, timestamp: p.timestamp });
    }
  }
  const display = base.reverse().slice(0, limit);
  // Stable dependency: `points` is a fresh array on every poll tick (10s),
  // so depending on it would refetch comments forever. Key on the job list
  // content instead — comments are only fetched when the rated-job set
  // actually changes.
  const jobKey = display.map((r) => r.job_id).join("|");

  useEffect(() => {
    let cancelled = false;
    (async () => {
      const enriched = await Promise.all(
        display.map(async (r) => {
          let comment = "";
          try {
            const res = await getDemoSessionByJob(r.job_id);
            const sid = res.session?.session_id;
            if (sid) {
              const msgs = await getDemoMessages(sid);
              const complete = [...msgs.messages]
                .reverse()
                .find((m) => m.action === "complete" && m.content);
              if (complete) comment = complete.content;
            }
          } catch {
            // session lookup is best-effort — the row stays, comment empty
          }
          return { ...r, comment };
        }),
      );
      if (!cancelled) setRows(enriched);
    })();
    return () => {
      cancelled = true;
    };
  }, [jobKey, limit]); // eslint-disable-line react-hooks/exhaustive-deps

  return (
    <Card className="border-white/10 bg-prism-surface/40">
      <CardHeader className="pb-3">
        <CardTitle className="text-base">Reputation Changes</CardTitle>
      </CardHeader>
      <CardContent>
        {isValidating && rows.length === 0 ? (
          <Skeleton className="h-20 rounded-md" />
        ) : rows.length === 0 ? (
          <p className="py-6 text-center text-xs text-white/40">
            No reputation changes yet. Modifications appear here after a job is
            completed and aggregated.
          </p>
        ) : (
          <ul className="space-y-2">
            {rows.map((r, i) => (
              <li
                key={`${r.job_id}-${i}`}
                className="rounded-md border border-white/5 bg-black/20 px-3 py-2"
              >
                <div className="flex items-center gap-3">
                  <Link
                    href={`/jobs/${r.job_id}`}
                    title={r.job_id}
                    className="min-w-0 flex-1 truncate font-mono text-xs text-prism-accent hover:underline"
                  >
                    job {formatAgentId(r.job_id)}
                  </Link>
                  <span className="font-mono text-sm font-bold tabular-nums text-white">
                    {formatBigInt(r.score, 18).toFixed(4)}
                  </span>
                  <span className="w-16 shrink-0 text-right text-[10px] text-white/40">
                    {formatTime(r.timestamp)}
                  </span>
                </div>
                {r.comment && (
                  <p
                    title={r.comment}
                    className="mt-1 line-clamp-2 border-t border-white/5 pt-1.5 text-xs leading-relaxed text-white/50"
                  >
                    {r.comment}
                  </p>
                )}
              </li>
            ))}
          </ul>
        )}
      </CardContent>
    </Card>
  );
}
