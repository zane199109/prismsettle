"use client";

// SlashHistory — recent PRISM_SLASHED events.
// Fired by ArbitrationHook when ruling=1 (refund). Penalty = max(0.2e18,
// currentScore * 30%) per the SD §3.4 spec.

import { Gavel, ExternalLink } from "lucide-react";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Skeleton } from "@/components/ui/skeleton";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { useSlashes } from "@/hooks/useSlashes";
import { formatAgentId, formatScore, formatTime } from "@/lib/utils";

export function SlashHistory({ limit = 15 }: { limit?: number }) {
  const { records, isValidating } = useSlashes({ size: limit });
  const rows = records.slice(0, limit);

  return (
    <Card className="border-white/10 bg-prism-surface/40">
      <CardHeader className="pb-3">
        <CardTitle className="flex items-center gap-2 text-base">
          <Gavel className="h-4 w-4 text-red-400" />
          Slash History
          <span className="ml-auto text-xs font-normal text-white/40">
            arbitration penalties
          </span>
        </CardTitle>
      </CardHeader>
      <CardContent className="p-0">
        <Table>
          <TableHeader>
            <TableRow className="border-white/10 hover:bg-transparent">
              <TableHead className="pl-6">Agent</TableHead>
              <TableHead>Penalty</TableHead>
              <TableHead>Reason</TableHead>
              <TableHead>Block</TableHead>
              <TableHead className="pr-6">Time</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {isValidating && rows.length === 0 ? (
              Array.from({ length: 4 }).map((_, i) => (
                <TableRow key={i} className="border-white/10">
                  <TableCell className="pl-6"><Skeleton className="h-4 w-20" /></TableCell>
                  <TableCell><Skeleton className="h-4 w-16" /></TableCell>
                  <TableCell><Skeleton className="h-4 w-24" /></TableCell>
                  <TableCell><Skeleton className="h-4 w-12" /></TableCell>
                  <TableCell className="pr-6"><Skeleton className="h-4 w-14" /></TableCell>
                </TableRow>
              ))
            ) : rows.length === 0 ? (
              <TableRow className="border-white/10 hover:bg-transparent">
                <TableCell colSpan={5} className="py-10 text-center text-sm text-white/40">
                  No slashes recorded — agents are behaving.
                </TableCell>
              </TableRow>
            ) : (
              rows.map((r) => (
                <TableRow
                  key={`${r.tx_hash}-${r.block_number}`}
                  className="border-white/10 bg-red-500/5"
                >
                  <TableCell className="pl-6 font-mono text-xs text-white">
                    {formatAgentId(r.agent_id)}
                  </TableCell>
                  <TableCell>
                    <Badge variant="destructive" className="font-mono text-[10px]">
                      -{r.penalty ? formatScore(r.penalty) : "0.0000"}
                    </Badge>
                  </TableCell>
                  <TableCell className="text-xs text-white/60">
                    {r.reason || "arbitration ruling = refund"}
                  </TableCell>
                  <TableCell className="font-mono text-xs text-white/50">
                    #{r.block_number}
                  </TableCell>
                  <TableCell className="pr-6 text-xs text-white/50">
                    <a
                      href={`https://testnet.monadexplorer.com/tx/${r.tx_hash}`}
                      target="_blank"
                      rel="noreferrer"
                      className="inline-flex items-center gap-1 hover:text-white"
                    >
                      {formatTime(r.block_time)}
                      <ExternalLink className="h-3 w-3" />
                    </a>
                  </TableCell>
                </TableRow>
              ))
            )}
          </TableBody>
        </Table>
      </CardContent>
    </Card>
  );
}
