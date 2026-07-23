"use client";

// Jobs list page — browse all jobs created on-chain with status filter,
// shard ID, creator, and timestamps. Click any row to open the detail page.
// DEV-PLAN §Phase 8 task 8.5 (list view was missing — added here).

import { useMemo, useState } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { Plus, Search, ChevronLeft, ChevronRight, Filter } from "lucide-react";
import { PageHeader } from "@/components/PageHeader";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Input } from "@/components/ui/input";
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
import { useJobs } from "@/hooks/useJobs";
import { cn, formatAgentId, formatTime } from "@/lib/utils";

const STATUS_OPTIONS = [
  { value: "all", label: "All statuses" },
  { value: "Pending", label: "Pending (Funded / Assigned)" },
  { value: "Submitted", label: "Submitted" },
  { value: "Completed", label: "Completed" },
  { value: "Disputed", label: "Disputed" },
  { value: "Resolved", label: "Resolved" },
];

const STATUS_VARIANT: Record<string, "default" | "secondary" | "destructive" | "outline"> = {
  Pending: "secondary",
  Submitted: "default",
  Completed: "default",
  Disputed: "destructive",
  Resolved: "outline",
};

const PAGE_SIZE = 15;

export default function JobsListPage() {
  const router = useRouter();
  const [status, setStatus] = useState<string>("all");
  const [query, setQuery] = useState("");
  const [page, setPage] = useState(1);

  const { jobs, total, isValidating } = useJobs({
    status: status === "all" ? undefined : status,
    page,
    size: PAGE_SIZE,
    intervalMs: 15000,
  });

  const filtered = useMemo(() => {
    const q = query.trim().toLowerCase();
    if (!q) return jobs;
    return jobs.filter(
      (j) =>
        j.job_id.toLowerCase().includes(q) ||
        j.creator.toLowerCase().includes(q) ||
        j.evaluator.toLowerCase().includes(q) ||
        String(j.shard_id).includes(q),
    );
  }, [jobs, query]);

  const totalPages = Math.max(1, Math.ceil(total / PAGE_SIZE));

  return (
    <div className="min-h-screen">
      <PageHeader />
      <main id="main" className="mx-auto max-w-7xl px-6 py-8">
        <div className="mb-6 flex flex-wrap items-end justify-between gap-3">
          <div>
            <h1 className="text-2xl font-bold tracking-tight">Jobs</h1>
            <p className="mt-1 text-sm text-white/60">
              {total} total · ERC-8183 lifecycle · sharded by jobId &amp; 0xFF
            </p>
          </div>
          <Button asChild>
            <Link href="/jobs/new">
              <Plus className="h-4 w-4" /> New Job
            </Link>
          </Button>
        </div>

        <Card className="border-white/10 bg-prism-surface/40">
          <CardHeader className="pb-3">
            <div className="flex flex-wrap items-center gap-3">
              <div className="relative flex-1 min-w-[240px]">
                <Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-white/40" />
                <Input
                  value={query}
                  onChange={(e) => { setQuery(e.target.value); setPage(1); }}
                  placeholder="Filter by jobId / creator / evaluator / shard…"
                  className="pl-9 bg-black/30 border-white/10"
                />
              </div>
              <div className="flex items-center gap-2">
                <Filter className="h-4 w-4 text-white/40" />
                <Select value={status} onValueChange={(v) => { setStatus(v); setPage(1); }}>
                  <SelectTrigger className="w-[220px] bg-black/30 border-white/10">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    {STATUS_OPTIONS.map((o) => (
                      <SelectItem key={o.value} value={o.value}>
                        {o.label}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
            </div>
          </CardHeader>
          <CardContent className="p-0">
            <Table>
              <TableHeader>
                <TableRow className="border-white/10 hover:bg-transparent">
                  <TableHead className="pl-6">Job ID</TableHead>
                  <TableHead>Status</TableHead>
                  <TableHead>Shard</TableHead>
                  <TableHead>Creator</TableHead>
                  <TableHead>Evaluator</TableHead>
                  <TableHead>Created</TableHead>
                  <TableHead className="pr-6 text-right">Action</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {isValidating && filtered.length === 0 ? (
                  Array.from({ length: 6 }).map((_, i) => (
                    <TableRow key={i} className="border-white/10">
                      <TableCell className="pl-6"><Skeleton className="h-4 w-24" /></TableCell>
                      <TableCell><Skeleton className="h-5 w-20 rounded-full" /></TableCell>
                      <TableCell><Skeleton className="h-4 w-8" /></TableCell>
                      <TableCell><Skeleton className="h-4 w-20" /></TableCell>
                      <TableCell><Skeleton className="h-4 w-20" /></TableCell>
                      <TableCell><Skeleton className="h-4 w-16" /></TableCell>
                      <TableCell className="pr-6"><Skeleton className="h-4 w-12 ml-auto" /></TableCell>
                    </TableRow>
                  ))
                ) : filtered.length === 0 ? (
                  <TableRow className="border-white/10 hover:bg-transparent">
                    <TableCell colSpan={7} className="py-16 text-center text-sm text-white/40">
                      No jobs match the current filter.
                      <div className="mt-3">
                        <Button asChild variant="outline" size="sm">
                          <Link href="/jobs/new">Create the first job</Link>
                        </Button>
                      </div>
                    </TableCell>
                  </TableRow>
                ) : (
                  filtered.map((job) => (
                    <TableRow
                      key={job.job_id}
                      className="border-white/10 cursor-pointer"
                      onClick={() => { window.location.href = `/jobs/${encodeURIComponent(job.job_id)}`; }}
                    >
                      <TableCell className="pl-6 font-mono text-xs text-white">
                        #{job.job_id.slice(0, 10)}…
                      </TableCell>
                      <TableCell>
                        <Badge variant={STATUS_VARIANT[job.status] ?? "secondary"}>
                          {job.status}
                        </Badge>
                      </TableCell>
                      <TableCell className="font-mono text-xs text-white/70">
                        #{job.shard_id}
                      </TableCell>
                      <TableCell className="font-mono text-xs text-white/70">
                        {formatAgentId(job.creator)}
                      </TableCell>
                      <TableCell className="font-mono text-xs text-white/70">
                        {job.evaluator ? formatAgentId(job.evaluator) : "—"}
                      </TableCell>
                      <TableCell className="text-xs text-white/50">
                        {formatTime(job.created_at)}
                      </TableCell>
                      <TableCell className="pr-6 text-right">
                        <Button
                          asChild
                          variant="ghost"
                          size="sm"
                          className="h-8"
                          onClick={(e) => e.stopPropagation()}
                        >
                          <Link href={`/jobs/${encodeURIComponent(job.job_id)}`}>View</Link>
                        </Button>
                      </TableCell>
                    </TableRow>
                  ))
                )}
              </TableBody>
            </Table>

            {/* Pagination */}
            {total > PAGE_SIZE && (
              <div className="flex items-center justify-between border-t border-white/10 px-6 py-3 text-xs text-white/60">
                <span>
                  Page {page} of {totalPages} · {total} total
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
