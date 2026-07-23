"use client";

// ValidationRecords — recent PRISM_VALIDATION_SUBMITTED events.
// Each row shows one submitValidation call: agent, score, source, tx.

import { CheckCircle2, ExternalLink } from "lucide-react";
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
import { useValidationRecords } from "@/hooks/useValidationRecords";
import { formatAgentId, formatScore, formatTime } from "@/lib/utils";

const SOURCE_LABEL: Record<number, { label: string; color: string }> = {
  0: { label: "Validator", color: "text-blue-400 border-blue-400/30" },
  1: { label: "Evaluator", color: "text-purple-400 border-purple-400/30" },
  2: { label: "Arbitration", color: "text-red-400 border-red-400/30" },
};

export function ValidationRecords({ limit = 15 }: { limit?: number }) {
  const { records, isValidating } = useValidationRecords({ size: limit });
  const rows = records.slice(0, limit);

  return (
    <Card className="border-white/10 bg-prism-surface/40">
      <CardHeader className="pb-3">
        <CardTitle className="flex items-center gap-2 text-base">
          <CheckCircle2 className="h-4 w-4 text-emerald-400" />
          Recent Validations
          <span className="ml-auto text-xs font-normal text-white/40">
            submitValidation calls
          </span>
        </CardTitle>
      </CardHeader>
      <CardContent className="p-0">
        <Table>
          <TableHeader>
            <TableRow className="border-white/10 hover:bg-transparent">
              <TableHead className="pl-6">Agent</TableHead>
              <TableHead>Score</TableHead>
              <TableHead>Source</TableHead>
              <TableHead>Job</TableHead>
              <TableHead>Block</TableHead>
              <TableHead className="pr-6">Time</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {isValidating && rows.length === 0 ? (
              Array.from({ length: 5 }).map((_, i) => (
                <TableRow key={i} className="border-white/10">
                  <TableCell className="pl-6"><Skeleton className="h-4 w-20" /></TableCell>
                  <TableCell><Skeleton className="h-4 w-16" /></TableCell>
                  <TableCell><Skeleton className="h-5 w-20 rounded-full" /></TableCell>
                  <TableCell><Skeleton className="h-4 w-16" /></TableCell>
                  <TableCell><Skeleton className="h-4 w-12" /></TableCell>
                  <TableCell className="pr-6"><Skeleton className="h-4 w-14" /></TableCell>
                </TableRow>
              ))
            ) : rows.length === 0 ? (
              <TableRow className="border-white/10 hover:bg-transparent">
                <TableCell colSpan={6} className="py-10 text-center text-sm text-white/40">
                  No validations submitted yet.
                </TableCell>
              </TableRow>
            ) : (
              rows.map((r) => {
                const src = SOURCE_LABEL[r.source] ?? SOURCE_LABEL[0];
                return (
                  <TableRow key={`${r.tx_hash}-${r.block_number}`} className="border-white/10">
                    <TableCell className="pl-6 font-mono text-xs text-white">
                      {formatAgentId(r.agent_id)}
                    </TableCell>
                    <TableCell className="font-mono text-sm font-semibold text-prism-accent">
                      {r.score ? formatScore(r.score) : "—"}
                    </TableCell>
                    <TableCell>
                      <Badge variant="outline" className={`font-mono text-[10px] ${src.color}`}>
                        {src.label}
                      </Badge>
                    </TableCell>
                    <TableCell className="font-mono text-xs text-white/70">
                      {r.job_id ? `#${r.job_id.slice(0, 8)}` : "—"}
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
                );
              })
            )}
          </TableBody>
        </Table>
      </CardContent>
    </Card>
  );
}
