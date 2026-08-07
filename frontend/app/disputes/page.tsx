"use client";

// Disputes list page (P1-6). Shows all PRISM_DISPUTED events with their
// resolution status. Each row links to /arbitrator?jobId=X for resolution.
//
// Field mapping (offchain/prismsettle/parser/prismsettle_hook_parser.go):
//   - Disputed:        event.to = jobId, event.token_address = reasonHash
//   - DisputeResolved: event.to = jobId, event.value = ruling (1=refund, 2=pay)

import Link from "next/link";
import { Gavel, ArrowRight } from "lucide-react";

import { useEvents } from "@/hooks/useEvents";
import { Card, CardContent } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Skeleton } from "@/components/ui/skeleton";
import { cn, formatTime } from "@/lib/utils";

const RULING_LABEL: Record<string, { label: string; color: string }> = {
  "1": { label: "Refunded", color: "text-amber-400 border-amber-400/40 bg-amber-400/5" },
  "2": { label: "Paid", color: "text-emerald-400 border-emerald-400/40 bg-emerald-400/5" },
};

export default function DisputesPage() {
  // Fetch both Disputed and DisputeResolved events in parallel.
  const { events: disputed, isValidating: loadingDisputed } = useEvents({
    eventType: "PRISM_DISPUTED",
    size: 100,
    intervalMs: 10000,
  });
  const { events: resolved } = useEvents({
    eventType: "PRISM_DISPUTE_RESOLVED",
    size: 100,
    intervalMs: 10000,
  });

  // Build a lookup of resolved jobIds → ruling for O(1) join.
  const resolvedMap = new Map<string, string>();
  for (const r of resolved) {
    resolvedMap.set(r.to, r.value);
  }

  const pendingCount = disputed.filter((d) => !resolvedMap.has(d.to)).length;
  const resolvedCount = disputed.length - pendingCount;

  return (
    <div className="min-h-screen">
      
      <main className="mx-auto max-w-7xl px-6 py-8">
        <div className="mb-6">
          <h1 className="flex items-center gap-2 text-2xl font-bold tracking-tight">
            <Gavel className="h-6 w-6 text-prism-accent" />
            Disputes
          </h1>
          <p className="mt-1 text-sm text-white/60">
            All arbitration cases across the marketplace. Pending cases need Arbitrator resolution.
          </p>
          <div className="mt-3 flex gap-3 text-xs">
            <span className="rounded-md border border-amber-400/30 bg-amber-400/5 px-2 py-1 text-amber-300">
              {pendingCount} pending
            </span>
            <span className="rounded-md border border-emerald-400/30 bg-emerald-400/5 px-2 py-1 text-emerald-300">
              {resolvedCount} resolved
            </span>
            <span className="rounded-md border border-white/10 bg-black/20 px-2 py-1 text-white/60">
              {disputed.length} total
            </span>
          </div>
        </div>

        {loadingDisputed && disputed.length === 0 ? (
          <Skeleton className="h-64 rounded-xl" />
        ) : disputed.length === 0 ? (
          <Card className="border-dashed border-white/20 bg-prism-surface/30 py-16 text-center">
            <CardContent className="py-0">
              <p className="text-sm text-white/60">No disputes filed.</p>
              <p className="mt-1 text-xs text-white/40">
                Disputes appear here when a buyer calls `ArbitrationHook.dispute(jobId, reasonHash)`.
              </p>
            </CardContent>
          </Card>
        ) : (
          <Card className="border-white/10 bg-prism-surface/40">
            <CardContent className="p-0">
              <div className="overflow-x-auto">
                <table className="w-full text-sm">
                  <thead className="border-b border-white/10 bg-black/20 text-xs uppercase tracking-wider text-white/40">
                    <tr>
                      <th className="px-4 py-3 text-left">Job ID</th>
                      <th className="px-4 py-3 text-left">Reason Hash</th>
                      <th className="px-4 py-3 text-left">Filed</th>
                      <th className="px-4 py-3 text-left">Status</th>
                      <th className="px-4 py-3 text-right">Action</th>
                    </tr>
                  </thead>
                  <tbody>
                    {disputed.map((d) => {
                      const ruling = resolvedMap.get(d.to);
                      const isResolved = ruling !== undefined;
                      const rulingInfo = ruling ? RULING_LABEL[ruling] : undefined;
                      return (
                        <tr
                          key={`${d.tx_hash}-${d.log_index ?? 0}`}
                          className="border-b border-white/5 hover:bg-white/5"
                        >
                          <td className="px-4 py-3 font-mono text-xs text-white">
                            #{d.to}
                          </td>
                          <td className="px-4 py-3 font-mono text-xs text-white/60">
                            {d.token_address ? `${d.token_address.slice(0, 10)}…${d.token_address.slice(-6)}` : "—"}
                          </td>
                          <td className="px-4 py-3 text-xs text-white/60">
                            {formatTime(d.block_time)}
                          </td>
                          <td className="px-4 py-3">
                            {isResolved ? (
                              <Badge
                                variant="outline"
                                className={cn("text-[10px] font-bold", rulingInfo?.color)}
                              >
                                {rulingInfo?.label ?? "Resolved"} (ruling={ruling})
                              </Badge>
                            ) : (
                              <Badge
                                variant="outline"
                                className="border-amber-400/40 bg-amber-400/5 text-[10px] font-bold text-amber-400"
                              >
                                Pending
                              </Badge>
                            )}
                          </td>
                          <td className="px-4 py-3 text-right">
                            <Link
                              href={`/arbitrator?jobId=${encodeURIComponent(d.to)}`}
                              className="inline-flex items-center gap-1 rounded-md border border-prism-accent/40 bg-prism-accent/5 px-2 py-1 text-xs text-prism-accent hover:bg-prism-accent/20"
                            >
                              {isResolved ? "View" : "Resolve"}
                              <ArrowRight className="h-3 w-3" />
                            </Link>
                          </td>
                        </tr>
                      );
                    })}
                  </tbody>
                </table>
              </div>
            </CardContent>
          </Card>
        )}
      </main>
    </div>
  );
}
