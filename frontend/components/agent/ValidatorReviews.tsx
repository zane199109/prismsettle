"use client";

// ValidatorReviews — lists recent PRISM_VALIDATION_SUBMITTED events for a
// given agent (PRD FR-M03 / P1-2). Renders validator address, score, source
// label (Validator / Job / Arbitration), and timestamp. Source coloring
// matches ScoreHistoryChart: blue=Validator, purple=Job, red=Arbitration.

import { useEvents } from "@/hooks/useEvents";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { Badge } from "@/components/ui/badge";
import { cn, formatAgentId, formatScore, formatTime } from "@/lib/utils";

const SOURCE_LABEL: Record<string, { label: string; color: string }> = {
  "0": { label: "Validator", color: "text-blue-400 border-blue-400/40 bg-blue-400/5" },
  "1": { label: "Job", color: "text-purple-400 border-purple-400/40 bg-purple-400/5" },
  "2": { label: "Arbitration", color: "text-red-400 border-red-400/40 bg-red-400/5" },
};

export function ValidatorReviews({ agentId, limit = 8 }: { agentId: string; limit?: number }) {
  const { events, isValidating } = useEvents({
    eventType: "PRISM_VALIDATION_SUBMITTED",
    size: 50,
    intervalMs: 10000,
  });

  // Filter events targeted at this agent. `to` carries the agentId per
  // offchain/prismsettle/parser/prismsettle_parser.go.
  const reviews = events
    .filter((e) => e.to === agentId)
    .slice(0, limit);

  return (
    <Card className="border-white/10 bg-prism-surface/40">
      <CardHeader className="pb-3">
        <CardTitle className="text-base">Validator Reviews</CardTitle>
      </CardHeader>
      <CardContent>
        {isValidating && reviews.length === 0 ? (
          <Skeleton className="h-20 rounded-md" />
        ) : reviews.length === 0 ? (
          <p className="py-6 text-center text-xs text-white/40">
            No validator reviews yet. Reviews appear here after `submitValidation` is called.
          </p>
        ) : (
          <ul className="space-y-2">
            {reviews.map((e) => {
              const src = SOURCE_LABEL[e.symbol ?? "0"] ?? SOURCE_LABEL["0"];
              return (
                <li
                  key={`${e.tx_hash}-${e.log_index ?? 0}`}
                  className="flex items-center justify-between gap-3 rounded-md border border-white/5 bg-black/20 px-3 py-2"
                >
                  <div className="min-w-0">
                    <div className="flex items-center gap-2">
                      <span className="truncate font-mono text-xs text-white/80">
                        {formatAgentId(e.from)}
                      </span>
                      <Badge
                        variant="outline"
                        className={cn("px-1.5 py-0 text-[9px] font-bold", src.color)}
                      >
                        {src.label}
                      </Badge>
                    </div>
                    <div className="mt-0.5 text-[10px] text-white/40">
                      {formatTime(e.block_time)} · job #{e.extra ? e.extra.slice(0, 10) : "—"}
                    </div>
                  </div>
                  <div className="text-right">
                    <div className="font-mono text-sm font-bold text-white">
                      {formatScore(e.value)}
                    </div>
                    <div className="text-[9px] text-white/40">score</div>
                  </div>
                </li>
              );
            })}
          </ul>
        )}
      </CardContent>
    </Card>
  );
}
