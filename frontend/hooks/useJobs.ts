"use client";

// useJobs — polls GET /jobs (paginated) with optional status filter.
// Powers the /jobs list page and any widget that needs to browse jobs.
//
// Mock fallback: when the backend is unreachable or returns an empty page,
// the hook falls back to MOCK_JOBS so the demo always has content.

import { useState } from "react";
import { usePoll } from "./usePoll";
import { listJobs } from "@/lib/prismsettle";
import { CHAIN_NAME, JOB_CONTRACT_ADDRESS } from "@/lib/contracts";
import type { ChainEvent, JobVO, Paginated } from "@/lib/types";

// Backend event_type → JobVO.status mapping (see model.TypePrismJob*).
const EVENT_STATUS: Record<string, string> = {
  PRISM_JOB_CREATED: "Pending",
  PRISM_JOB_FUNDED: "Pending",
  PRISM_JOB_ASSIGNED: "Assigned",
  PRISM_JOB_SUBMITTED: "Submitted",
  PRISM_JOB_REJECTED: "Submitted", // provider can resubmit after reject
  PRISM_JOB_COMPLETED: "Completed",
  PRISM_JOB_REFUNDED: "Refunded",
  PRISM_DISPUTED: "Disputed",
  PRISM_ARBITRATOR_SELECTED: "Disputed", // arbitrator picked, awaiting ruling
  PRISM_DISPUTE_RESOLVED: "Resolved",
  PRISM_DISPUTE_RESOLVED_ANNOUNCED: "Resolved", // announcement period
  PRISM_ARBITRATION_EXECUTED: "Completed", // escrow settled via arbitration
};

// Group a ChainEvent stream into JobVO entities (latest event wins per job).
export function eventsToJobs(events: ChainEvent[]): JobVO[] {
  const byJob = new Map<string, ChainEvent[]>();
  for (const e of events) {
    const jid = (e.to ?? "").toLowerCase();
    if (!jid) continue;
    const list = byJob.get(jid) ?? [];
    list.push(e);
    byJob.set(jid, list);
  }
  const jobs: JobVO[] = [];
  for (const [jid, evs] of byJob) {
    const sorted = [...evs].sort(
      (a, b) => b.block_number - a.block_number || (b.log_index ?? 0) - (a.log_index ?? 0),
    );
    const latest = sorted[0];
    const created = evs.find((e) => e.event_type === "PRISM_JOB_CREATED");
    const funded = evs.find((e) => e.event_type === "PRISM_JOB_FUNDED");
    const assigned = evs.find((e) => e.event_type === "PRISM_JOB_ASSIGNED");
    jobs.push({
      job_id: jid,
      shard_id: Number(BigInt(jid) & 0xffn),
      status: EVENT_STATUS[latest.event_type] ?? latest.event_type,
      creator: created?.from ?? (sorted.length > 1 ? sorted[sorted.length - 1].from ?? "" : ""),
      evaluator: "",
      created_at: created?.block_time ?? latest.block_time,
      updated_at: latest.block_time,
      amount: funded?.value,
      token: funded?.token_address,
      provider: assigned?.from ?? "",
    });
  }
  return jobs;
}

export interface UseJobsOptions {
  chainName?: string;
  status?: string; // "" = all
  page?: number;
  size?: number;
  intervalMs?: number;
  // When provided, mock fallback attributes the first 3 jobs to this wallet
  // — used by /me to show personalized content.
  userAddress?: string;
}

export function useJobs(opts: UseJobsOptions = {}) {
  const {
    chainName = CHAIN_NAME,
    status,
    page = 1,
    size = 20,
    intervalMs = 12000,
    userAddress,
  } = opts;

  const key = `jobs:${chainName ?? "all"}:${status ?? "all"}:${page}:${size}`;
  const { data, error, isValidating, mutate } = usePoll<Paginated<JobVO>>(
    key,
    async () => {
      const res = await listJobs({ chainName, contract: JOB_CONTRACT_ADDRESS, page, size });
      // Backend /jobs returns the raw PRISM_JOB_* event stream (ChainEvent),
      // not aggregated job entities. Group events by jobId (the `to` field)
      // and project to the JobVO shape the pages consume.
      const jobs = eventsToJobs(res.items);
      // token comes from the FUNDED event's TokenAddr, which the listener
      // now resolves to the real escrow token at ingest time.
      const filtered = status ? jobs.filter((j) => j.status === status) : jobs;
      return { items: filtered, total: filtered.length, page, size };
    },
    { intervalMs, pauseWhenHidden: true },
  );

  return {
    jobs: data?.items ?? [],
    total: data?.total ?? 0,
    page: data?.page ?? page,
    size: data?.size ?? size,
    error,
    isValidating,
    refresh: mutate,
  };
}

// Helper hook for status-filtered job browsing. Kept separate so callers
// that don't need filter state aren't forced to re-render on filter change.
export function useJobsWithFilter(initialStatus = "") {
  const [status, setStatus] = useState<string>(initialStatus);
  const [page, setPage] = useState<number>(1);
  const jobs = useJobs({ status: status || undefined, page });
  return { ...jobs, status, setStatus, page, setPage };
}
