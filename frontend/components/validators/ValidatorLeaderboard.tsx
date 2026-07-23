"use client";

// ValidatorLeaderboard — top validators by aggregated stake.
// Derives from PRISM_STAKED events (no dedicated /validators endpoint yet).

import { Trophy, ExternalLink } from "lucide-react";
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
import { useValidators } from "@/hooks/useValidators";
import { formatAgentId, formatTime } from "@/lib/utils";

export function ValidatorLeaderboard({ limit = 10 }: { limit?: number }) {
  const { rows, isValidating } = useValidators({ size: 100 });

  const top = rows.slice(0, limit);

  return (
    <Card className="border-white/10 bg-prism-surface/40">
      <CardHeader className="pb-3">
        <CardTitle className="flex items-center gap-2 text-base">
          <Trophy className="h-4 w-4 text-amber-400" />
          Validator Leaderboard
          <span className="ml-auto text-xs font-normal text-white/40">
            by total staked
          </span>
        </CardTitle>
      </CardHeader>
      <CardContent className="p-0">
        <Table>
          <TableHeader>
            <TableRow className="border-white/10 hover:bg-transparent">
              <TableHead className="pl-6 w-12">#</TableHead>
              <TableHead>Address</TableHead>
              <TableHead>Stake</TableHead>
              <TableHead>Stake Count</TableHead>
              <TableHead className="pr-6">Last Active</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {isValidating && top.length === 0 ? (
              Array.from({ length: 5 }).map((_, i) => (
                <TableRow key={i} className="border-white/10">
                  <TableCell className="pl-6"><Skeleton className="h-4 w-6" /></TableCell>
                  <TableCell><Skeleton className="h-4 w-24" /></TableCell>
                  <TableCell><Skeleton className="h-4 w-16" /></TableCell>
                  <TableCell><Skeleton className="h-4 w-8" /></TableCell>
                  <TableCell className="pr-6"><Skeleton className="h-4 w-16" /></TableCell>
                </TableRow>
              ))
            ) : top.length === 0 ? (
              <TableRow className="border-white/10 hover:bg-transparent">
                <TableCell colSpan={5} className="py-10 text-center text-sm text-white/40">
                  No stakers yet. Be the first validator.
                </TableCell>
              </TableRow>
            ) : (
              top.map((v, i) => (
                <TableRow key={v.address} className="border-white/10">
                  <TableCell className="pl-6">
                    <Badge
                      variant={i < 3 ? "default" : "secondary"}
                      className="w-6 justify-center px-0"
                    >
                      {i + 1}
                    </Badge>
                  </TableCell>
                  <TableCell className="font-mono text-xs text-white">
                    <a
                      href={`https://testnet.monadexplorer.com/address/${v.address}`}
                      target="_blank"
                      rel="noreferrer"
                      className="inline-flex items-center gap-1 hover:text-prism-accent"
                    >
                      {formatAgentId(v.address)}
                      <ExternalLink className="h-3 w-3" />
                    </a>
                  </TableCell>
                  <TableCell className="font-mono text-sm font-semibold text-emerald-400">
                    {v.totalStaked} ETH
                  </TableCell>
                  <TableCell className="font-mono text-xs text-white/70">
                    {v.stakeCount}
                  </TableCell>
                  <TableCell className="pr-6 text-xs text-white/50">
                    {formatTime(v.lastActiveAt)}
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
