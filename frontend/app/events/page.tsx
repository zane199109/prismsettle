"use client";

// Global events page — full chain event log with event_type filter and
// pagination. The dashboard feed only shows the latest 20; this page is
// the explorer for diving into historical activity.
//
// Reorg-affected rows render with an amber ↻ marker (parsed from `extra`).

import { useState } from "react";
import { Filter, ChevronLeft, ChevronRight, ExternalLink } from "lucide-react";

import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { Skeleton } from "@/components/ui/skeleton";
import { useEvents } from "@/hooks/useEvents";
import { cn, formatAgentId, formatScore, formatTime, tokenDecimals } from "@/lib/utils";
import { formatUnits } from "viem";
import type { ChainEvent } from "@/lib/types";

const EVENT_TYPES = [
  { value: "all", label: "All events" },
  { value: "PRISM_AGENT_REGISTERED", label: "Agent Registered" },
  { value: "PRISM_VALIDATION_SUBMITTED", label: "Validation Submitted" },
  { value: "PRISM_AGGREGATED", label: "Aggregated" },
  { value: "PRISM_STAKED", label: "Staked" },
  { value: "PRISM_UNSTAKE_STARTED", label: "Unstake Started" },
  { value: "PRISM_UNSTAKE_WITHDRAWN", label: "Unstake Withdrawn" },
  { value: "PRISM_SLASHED", label: "Slashed" },
  { value: "PRISM_JOB_CREATED", label: "Job Created" },
  { value: "PRISM_JOB_FUNDED", label: "Job Funded" },
  { value: "PRISM_JOB_ASSIGNED", label: "Job Assigned" },
  { value: "PRISM_JOB_SUBMITTED", label: "Job Submitted" },
  { value: "PRISM_JOB_REJECTED", label: "Job Rejected" },
  { value: "PRISM_JOB_COMPLETED", label: "Job Completed" },
  { value: "PRISM_JOB_REFUNDED", label: "Job Refunded" },
  { value: "PRISM_DISPUTED", label: "Disputed" },
  { value: "PRISM_ARBITRATOR_SELECTED", label: "Arbitrator Selected" },
  { value: "PRISM_DISPUTE_RESOLVED", label: "Dispute Resolved" },
  { value: "PRISM_DISPUTE_RESOLVED_ANNOUNCED", label: "Resolution Announced" },
  { value: "PRISM_ARBITRATION_EXECUTED", label: "Arbitration Executed" },
  { value: "PRISM_ARBITRATOR_REGISTERED", label: "Arbitrator Registered" },
];

const PAGE_SIZE = 30;

function isReorged(extra: string): boolean {
  if (!extra) return false;
  try {
    const obj = JSON.parse(extra) as { reorged?: boolean };
    return Boolean(obj.reorged);
  } catch {
    return false;
  }
}

// Token contract address → human label.
function tokenLabel(addr: string | undefined): string {
  if (!addr) return "tokens";
  const a = addr.toLowerCase();
  if (a === "0x252e44550f8b9997901e5540fc0e1da52ab099c6") return "USDC";
  if (a === "0x2bb06a30d464ca8e62563081f024e6380f0eb70b") return "USDC";
  if (a === "0xfb8bf4c1cc7a94c73d209a149ea2abea852bc541") return "WMON";
  return "tokens";
}

// Events whose `value` is a 1e18 reputation score, not a token amount.
const SCORE_EVENTS = new Set(["PRISM_AGGREGATED", "PRISM_VALIDATION_SUBMITTED", "PRISM_SLASHED"]);

// Human-readable value: reputation events → "0.75"; amount events → "100 USDC".
function formatEventValue(e: ChainEvent): string {
  if (!e.value || e.value === "0") return "—";
  if (SCORE_EVENTS.has(e.event_type)) {
    return formatScore(e.value);
  }
  try {
    const amt = formatUnits(BigInt(e.value), tokenDecimals(e.token_address));
    return `${amt} ${tokenLabel(e.token_address)}`;
  } catch {
    return e.value;
  }
}

export default function EventsPage() {
  const [eventType, setEventType] = useState<string>("all");
  const [page, setPage] = useState(1);

  const { events, total, isValidating } = useEvents({
    eventType: eventType === "all" ? undefined : eventType,
    page,
    size: PAGE_SIZE,
    intervalMs: 10000,
  });

  const totalPages = Math.max(1, Math.ceil(total / PAGE_SIZE));

  return (
    <div className="min-h-screen">
      
      <main id="main" className="mx-auto max-w-7xl px-6 py-8">
        <div className="mb-6">
          <h1 className="text-2xl font-bold tracking-tight">Chain Events</h1>
          <p className="mt-1 text-sm text-white/60">
            Live event log from the listener. Reorg-affected rows are flagged with ↻.
          </p>
        </div>

        <Card className="border-white/10 bg-prism-surface/40">
          <CardHeader className="pb-3">
            <div className="flex items-center gap-2">
              <Filter className="h-4 w-4 text-white/40" />
              <Select
                value={eventType}
                onValueChange={(v) => { setEventType(v); setPage(1); }}
              >
                <SelectTrigger className="w-[280px] bg-black/30 border-white/10">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {EVENT_TYPES.map((o) => (
                    <SelectItem key={o.value} value={o.value}>
                      {o.label}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
              <span className="ml-auto text-xs text-white/40">
                {total} events
              </span>
            </div>
          </CardHeader>
          <CardContent className="p-0">
            <Table>
              <TableHeader>
                <TableRow className="border-white/10 hover:bg-transparent">
                  <TableHead className="pl-6">Block</TableHead>
                  <TableHead>Event</TableHead>
                  <TableHead>From</TableHead>
                  <TableHead>To</TableHead>
                  <TableHead>Value</TableHead>
                  <TableHead>Tx</TableHead>
                  <TableHead className="pr-6">Time</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {isValidating && events.length === 0 ? (
                  Array.from({ length: 8 }).map((_, i) => (
                    <TableRow key={i} className="border-white/10">
                      <TableCell className="pl-6"><Skeleton className="h-4 w-16" /></TableCell>
                      <TableCell><Skeleton className="h-5 w-32 rounded-full" /></TableCell>
                      <TableCell><Skeleton className="h-4 w-20" /></TableCell>
                      <TableCell><Skeleton className="h-4 w-20" /></TableCell>
                      <TableCell><Skeleton className="h-4 w-16" /></TableCell>
                      <TableCell><Skeleton className="h-4 w-24" /></TableCell>
                      <TableCell className="pr-6"><Skeleton className="h-4 w-16" /></TableCell>
                    </TableRow>
                  ))
                ) : events.length === 0 ? (
                  <TableRow className="border-white/10 hover:bg-transparent">
                    <TableCell colSpan={7} className="py-16 text-center text-sm text-white/40">
                      No events recorded yet.
                    </TableCell>
                  </TableRow>
                ) : (
                  events.map((e) => {
                    const reorged = isReorged(e.extra);
                    return (
                      <TableRow
                        key={`${e.id}`}
                        className={cn(
                          "border-white/10",
                          reorged && "bg-amber-500/5",
                        )}
                      >
                        <TableCell className="pl-6 font-mono text-xs text-white/70">
                          #{e.block_number}
                        </TableCell>
                        <TableCell>
                          <Badge
                            variant="outline"
                            className={cn(
                              "font-mono text-[10px]",
                              reorged
                                ? "border-amber-500/40 text-amber-400"
                                : "border-prism-accent/30 text-prism-accent",
                            )}
                          >
                            {reorged && <span className="mr-1">↻</span>}
                            {e.event_type.replace("PRISM_", "")}
                          </Badge>
                        </TableCell>
                        <TableCell className="font-mono text-xs text-white/70">
                          {formatAgentId(e.from)}
                        </TableCell>
                        <TableCell className="font-mono text-xs text-white/70">
                          {formatAgentId(e.to)}
                        </TableCell>
                        <TableCell className="font-mono text-xs text-white/70">
                          {formatEventValue(e)}
                        </TableCell>
                        <TableCell className="font-mono text-[11px] text-white/50">
                          <a
                            href={`https://testnet.monadexplorer.com/tx/${e.tx_hash}`}
                            target="_blank"
                            rel="noreferrer"
                            className="inline-flex items-center gap-1 hover:text-white"
                          >
                            {e.tx_hash.slice(0, 8)}…
                            <ExternalLink className="h-3 w-3" />
                          </a>
                        </TableCell>
                        <TableCell className="pr-6 text-xs text-white/50">
                          {formatTime(e.block_time)}
                        </TableCell>
                      </TableRow>
                    );
                  })
                )}
              </TableBody>
            </Table>

            {total > PAGE_SIZE && (
              <div className="flex items-center justify-between border-t border-white/10 px-6 py-3 text-xs text-white/60">
                <span>
                  Page {page} of {totalPages}
                </span>
                <div className="flex gap-2">
                  <Button
                    variant="outline"
                    size="sm"
                    disabled={page <= 1}
                    onClick={() => setPage((p) => Math.max(1, p - 1))}
                  >
                    <ChevronLeft className="h-4 w-4" /> Prev
                  </Button>
                  <Button
                    variant="outline"
                    size="sm"
                    disabled={page >= totalPages}
                    onClick={() => setPage((p) => p + 1)}
                  >
                    Next <ChevronRight className="h-4 w-4" />
                  </Button>
                </div>
              </div>
            )}
          </CardContent>
        </Card>
      </main>
    </div>
  );
}
