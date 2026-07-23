"use client";

// Job detail page — status tracking (ERC-8183 4-state + arbitration block),
// deliverable submission (FR-JM03), dispute + ruling display (FR-JM05/JM06),
// and FundFlowChart (FR-JM04).
// DEV-PLAN §Phase 8 任务 8.5a/b/c/d.
//
// Phase 9 wiring: all three write paths (submit / dispute) now go through
// wagmi useWriteContract against the Monad testnet. FundFlowChart reads real
// on-chain Job state via getJobState().

import { use, useState } from "react";
import Link from "next/link";
import { useSearchParams } from "next/navigation";
import { ArrowLeft, Loader2, Gavel, FileUp, CheckCircle2, XCircle } from "lucide-react";
import { useAccount, useWriteContract, useWaitForTransactionReceipt, useReadContract } from "wagmi";
import { monadTestnet } from "wagmi/chains";
import { PageHeader } from "@/components/PageHeader";
import { JobStatusTracker } from "@/components/job/JobStatusTracker";
import { FundFlowChart } from "@/components/job/FundFlowChart";
import { FundingPathBadge } from "@/components/job/FundingPathBadge";
import { useJobStatusPoll } from "@/hooks/useJobStatusPoll";
import { useJobTimeline } from "@/hooks/useJobTimeline";
import { useEvents } from "@/hooks/useEvents";
import {
  JOB_CONTRACT_ADDRESS,
  JOB_ABI,
  HOOK_CONTRACT_ADDRESS,
  HOOK_ABI,
  contractsReady,
} from "@/lib/contracts";

interface PageProps {
  params: Promise<{ jobId: string }>;
}

export default function JobDetailPage({ params }: PageProps) {
  const { jobId } = use(params);
  return <JobDetailBody jobId={decodeURIComponent(jobId)} />;
}

function JobDetailBody({ jobId }: { jobId: string }) {
  const searchParams = useSearchParams();
  const agentId = searchParams.get("agent") ?? "";
  const { status, isTerminal } = useJobStatusPoll(jobId);
  const { timeline } = useJobTimeline(jobId);

  // Read on-chain ruling from resolved dispute events (FR-JM06).
  const { events: disputeEvents } = useEvents({
    eventType: "PRISM_DISPUTE_RESOLVED",
    to: jobId,
    size: 1,
    intervalMs: 15000,
  });
  const resolvedRuling = disputeEvents.length > 0 ? disputeEvents[0].value : null;

  const canSubmit = status === "Funded" || status === "Assigned";
  const canDispute = status === "Submitted";

  return (
    <div className="min-h-screen">
      <PageHeader />
      <main className="mx-auto max-w-4xl px-6 py-8">
        <Link
          href="/"
          className="mb-4 inline-flex items-center gap-1 text-sm text-white/60 hover:text-white"
        >
          <ArrowLeft className="h-4 w-4" /> Back to dashboard
        </Link>

        <div className="mb-6 flex items-center justify-between">
          <div>
            <h1 className="font-mono text-2xl font-bold text-white">Job #{jobId}</h1>
            <p className="mt-1 text-sm text-white/60">
              status:{" "}
              <span className="font-mono text-prism-accent">{status ?? "loading…"}</span>
              {isTerminal && (
                <span className="ml-2 rounded-md border border-emerald-500/40 bg-emerald-500/10 px-1.5 py-0.5 text-[11px] text-emerald-400">
                  terminal
                </span>
              )}
            </p>
          </div>
          <FundingPathBadge jobContractAddress={JOB_CONTRACT_ADDRESS} rpcUrl={process.env.NEXT_PUBLIC_RPC_URL} />
        </div>

        {/* Status tracker (8.5a) */}
        <section className="rounded-xl border border-white/10 bg-prism-surface/40 p-5">
          <h2 className="mb-4 text-sm font-medium text-white">Status Tracking</h2>
          <JobStatusTracker current={status ?? "Created"} timeline={timeline} />
        </section>

        {/* Deliverable submission (8.5b, FR-JM03) */}
        {canSubmit && <DeliverableSubmit jobId={jobId} />}

        {/* Dispute (8.5c, FR-JM05/JM06) */}
        {(canDispute || status === "Disputed" || status === "DisputeResolved") && (
          <DisputePanel jobId={jobId} status={status ?? ""} resolvedRuling={resolvedRuling} />
        )}

        {/* Fund flow (8.5d, FR-JM04) — reads real on-chain Job state */}
        <FundFlowFromChain jobId={jobId} />
      </main>
    </div>
  );
}

// ---------- Real on-chain Job state reader for FundFlowChart ----------
// getJobState returns a tuple: (state, buyer, provider, amount,
// deliverableHash, proofHash, deadline, hook).
type JobStateTuple = readonly [
  bigint, // state (enum uint8)
  `0x${string}`, // buyer
  `0x${string}`, // provider
  bigint, // amount
  `0x${string}`, // deliverableHash
  `0x${string}`, // proofHash
  bigint, // deadline
  `0x${string}`, // hook
];

function FundFlowFromChain({ jobId }: { jobId: string }) {
  const jobIdBig = BigInt(jobId);
  const ready = contractsReady() && JOB_CONTRACT_ADDRESS !== undefined;

  const { data, isError, error } = useReadContract({
    address: JOB_CONTRACT_ADDRESS,
    abi: JOB_ABI,
    functionName: "getJobState",
    args: [jobIdBig],
    query: { enabled: ready && !Number.isNaN(Number(jobId)) },
  });

  if (!ready) {
    return (
      <section className="mt-6">
        <FundFlowPlaceholder note="Contract address not configured (NEXT_PUBLIC_JOB_CONTRACT_ADDRESS)." />
      </section>
    );
  }

  if (isError) {
    return (
      <section className="mt-6">
        <FundFlowPlaceholder note={`Failed to read on-chain Job state: ${error?.message ?? "unknown error"}`} />
      </section>
    );
  }

  if (!data) {
    return (
      <section className="mt-6">
        <FundFlowPlaceholder note="Loading on-chain Job state…" />
      </section>
    );
  }

  const job = data as JobStateTuple;
  const buyer = job[1];
  const provider = job[2];
  const amount = job[3].toString();
  const hookAddr = job[7];
  // If hook address is non-zero, an ArbitrationHook is attached; we infer
  // the funding path from the absence of x402 receipt data — for the
  // hackathon demo all fundings use the ERC-20 path (empty receipt).
  // PRD FR-JM04: x402 funding detection. The hackathon demo uses ERC-20
  // funding path exclusively (x402 facilitator not deployed on Monad testnet
  // yet — Phase 9.8). Set to true when the receipt data structure changes
  // to include x402 metadata.
  const usedX402 = false;

  return (
    <section className="mt-6">
      <FundFlowChart
        buyer={buyer}
        jobContract={JOB_CONTRACT_ADDRESS ?? "0x0"}
        provider={provider}
        amount={amount}
        usedX402={usedX402}
      />
      {hookAddr && hookAddr !== "0x0000000000000000000000000000000000000000" && (
        <p className="mt-2 text-[11px] text-white/40">
          Arbitration hook attached: <code className="font-mono text-white/60">{hookAddr}</code>
        </p>
      )}
    </section>
  );
}

function FundFlowPlaceholder({ note }: { note: string }) {
  return (
    <div className="rounded-xl border border-white/10 bg-prism-surface/40 p-5">
      <h3 className="mb-2 text-sm font-medium text-white">Fund Flow</h3>
      <p className="text-xs text-white/40">{note}</p>
    </div>
  );
}

// ---------- 8.5b: Deliverable submission (FR-JM03) — real on-chain call ----------
function DeliverableSubmit({ jobId }: { jobId: string }) {
  const { address, chain } = useAccount();
  const { writeContractAsync } = useWriteContract();

  const [deliverableHash, setDeliverableHash] = useState("");
  const [proofHash, setProofHash] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [txHash, setTxHash] = useState<`0x${string}` | null>(null);

  const { isLoading: isConfirming } = useWaitForTransactionReceipt({
    hash: txHash ?? undefined,
  });

  const ready = contractsReady() && JOB_CONTRACT_ADDRESS !== undefined;
  const wrongChain = chain && chain.id !== monadTestnet.id;

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    setError(null);
    setTxHash(null);

    if (!address) {
      setError("Please connect your wallet first.");
      return;
    }
    if (wrongChain) {
      setError(`Wrong network. Please switch to Monad Testnet (chainId ${monadTestnet.id}).`);
      return;
    }
    if (!JOB_CONTRACT_ADDRESS) {
      setError("Job contract address not configured.");
      return;
    }

    // Validate bytes32 inputs.
    const bytes32Re = /^0x[0-9a-fA-F]{64}$/;
    if (!bytes32Re.test(deliverableHash)) {
      setError("deliverableHash must be a 32-byte hex string (0x + 64 hex chars).");
      return;
    }
    if (!bytes32Re.test(proofHash)) {
      setError("proofHash must be a 32-byte hex string (0x + 64 hex chars).");
      return;
    }

    setSubmitting(true);
    try {
      const hash = await writeContractAsync({
        address: JOB_CONTRACT_ADDRESS,
        abi: JOB_ABI,
        functionName: "submit",
        args: [BigInt(jobId), deliverableHash as `0x${string}`, proofHash as `0x${string}`],
        chainId: monadTestnet.id,
      });
      setTxHash(hash);
    } catch (err) {
      const msg = err instanceof Error ? err.message : String(err);
      // wagmi/viem reject user-cancelled txs with a "UserRejectedRequestError"
      // or similar; surface a friendly message instead of the raw stack.
      setError(msg.includes("UserRejected") ? "Transaction rejected by user." : msg);
    } finally {
      setSubmitting(false);
    }
  }

  const done = Boolean(txHash) && !isConfirming;

  return (
    <section className="mt-6 rounded-xl border border-white/10 bg-prism-surface/40 p-5">
      <h2 className="mb-3 flex items-center gap-2 text-sm font-medium text-white">
        <FileUp className="h-4 w-4 text-prism-accent" />
        Submit Deliverable (FR-JM03)
      </h2>
      <form onSubmit={handleSubmit} className="space-y-3">
        <div>
          <label className="mb-1 block text-xs text-white/60">deliverableHash (bytes32)</label>
          <input
            type="text"
            required
            value={deliverableHash}
            onChange={(e) => setDeliverableHash(e.target.value)}
            placeholder="0x…"
            className="w-full rounded-md border border-white/10 bg-prism-surface/60 px-3 py-1.5 font-mono text-sm text-white placeholder:text-white/30 focus:border-prism-accent focus:outline-none"
          />
        </div>
        <div>
          <label className="mb-1 block text-xs text-white/60">proofHash (bytes32)</label>
          <input
            type="text"
            required
            value={proofHash}
            onChange={(e) => setProofHash(e.target.value)}
            placeholder="0x…"
            className="w-full rounded-md border border-white/10 bg-prism-surface/60 px-3 py-1.5 font-mono text-sm text-white placeholder:text-white/30 focus:border-prism-accent focus:outline-none"
          />
        </div>
        {error && <p className="text-xs text-red-400">{error}</p>}
        {txHash && (
          <p className="text-xs text-emerald-400">
            Tx submitted:{" "}
            <code className="font-mono">{txHash.slice(0, 10)}…{txHash.slice(-8)}</code>
            {isConfirming && " (confirming…)"}
          </p>
        )}
        <button
          type="submit"
          disabled={submitting || !ready || !address}
          className="inline-flex items-center gap-2 rounded-md bg-prism-accent px-4 py-2 text-sm font-semibold text-white hover:bg-prism-accent/80 disabled:opacity-50"
        >
          {submitting ? (
            <>
              <Loader2 className="h-4 w-4 animate-spin" /> Submitting…
            </>
          ) : done ? (
            <>
              <CheckCircle2 className="h-4 w-4" /> Submitted
            </>
          ) : (
            <>
              <FileUp className="h-4 w-4" /> Submit Deliverable
            </>
          )}
        </button>
        <p className="text-[11px] text-white/40">
          Calls <code className="text-white/60">Job.submit(jobId, deliverableHash, proofHash)</code> on Monad Testnet.
        </p>
      </form>
    </section>
  );
}

// ---------- 8.5c: Dispute + ruling (FR-JM05/JM06) — real on-chain call ----------
function DisputePanel({ jobId, status, resolvedRuling }: { jobId: string; status: string; resolvedRuling: string | null }) {
  const { address, chain } = useAccount();
  const { writeContractAsync } = useWriteContract();

  const [reasonHash, setReasonHash] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [txHash, setTxHash] = useState<`0x${string}` | null>(null);

  const { isLoading: isConfirming } = useWaitForTransactionReceipt({
    hash: txHash ?? undefined,
  });

  const isDisputed = status === "Disputed";
  const isResolved = status === "DisputeResolved";
  const ready = contractsReady() && HOOK_CONTRACT_ADDRESS !== undefined;
  const wrongChain = chain && chain.id !== monadTestnet.id;

  async function handleDispute(e: React.FormEvent) {
    e.preventDefault();
    setError(null);
    setTxHash(null);

    if (!address) {
      setError("Please connect your wallet first.");
      return;
    }
    if (wrongChain) {
      setError(`Wrong network. Please switch to Monad Testnet (chainId ${monadTestnet.id}).`);
      return;
    }
    if (!HOOK_CONTRACT_ADDRESS) {
      setError("ArbitrationHook contract address not configured (NEXT_PUBLIC_HOOK_CONTRACT_ADDRESS).");
      return;
    }

    const bytes32Re = /^0x[0-9a-fA-F]{64}$/;
    if (!bytes32Re.test(reasonHash)) {
      setError("reasonHash must be a 32-byte hex string (0x + 64 hex chars).");
      return;
    }

    setSubmitting(true);
    try {
      const hash = await writeContractAsync({
        address: HOOK_CONTRACT_ADDRESS,
        abi: HOOK_ABI,
        functionName: "dispute",
        args: [BigInt(jobId), reasonHash as `0x${string}`],
        chainId: monadTestnet.id,
      });
      setTxHash(hash);
    } catch (err) {
      const msg = err instanceof Error ? err.message : String(err);
      setError(msg.includes("UserRejected") ? "Transaction rejected by user." : msg);
    } finally {
      setSubmitting(false);
    }
  }

  const done = Boolean(txHash) && !isConfirming;

  return (
    <section className="mt-6 rounded-xl border border-amber-500/30 bg-amber-500/5 p-5">
      <h2 className="mb-3 flex items-center gap-2 text-sm font-medium text-amber-400">
        <Gavel className="h-4 w-4" />
        Arbitration (FR-JM05 / FR-JM06)
      </h2>

      {!isDisputed && !isResolved && (
        <form onSubmit={handleDispute} className="space-y-3">
          <div>
            <label className="mb-1 block text-xs text-white/60">
              reasonHash (bytes32, keccak256 of your dispute evidence)
            </label>
            <input
              type="text"
              required
              value={reasonHash}
              onChange={(e) => setReasonHash(e.target.value)}
              placeholder="0x…"
              className="w-full rounded-md border border-white/10 bg-prism-surface/60 px-3 py-1.5 font-mono text-sm text-white placeholder:text-white/30 focus:border-amber-500 focus:outline-none"
            />
          </div>
          {error && <p className="text-xs text-red-400">{error}</p>}
          {txHash && (
            <p className="text-xs text-emerald-400">
              Tx submitted:{" "}
              <code className="font-mono">{txHash.slice(0, 10)}…{txHash.slice(-8)}</code>
              {isConfirming && " (confirming…)"}
            </p>
          )}
          <button
            type="submit"
            disabled={submitting || !ready || !address}
            className="inline-flex items-center gap-2 rounded-md border border-amber-500/40 bg-amber-500/10 px-4 py-2 text-sm font-semibold text-amber-400 hover:bg-amber-500/20 disabled:opacity-50"
          >
            {submitting ? (
              <>
                <Loader2 className="h-4 w-4 animate-spin" /> Raising…
              </>
            ) : done ? (
              <>
                <CheckCircle2 className="h-4 w-4" /> Dispute Raised
              </>
            ) : (
              <>
                <Gavel className="h-4 w-4" /> Raise Dispute
              </>
            )}
          </button>
          <p className="text-[11px] text-white/40">
            Calls <code className="text-white/60">Hook.dispute(jobId, reasonHash)</code> on Monad Testnet.
          </p>
        </form>
      )}

      {isDisputed && (
        <div className="rounded-md border border-amber-500/40 bg-amber-500/10 p-3 text-sm text-amber-300">
          ⚖ Dispute is open. Waiting for resolver ruling…
        </div>
      )}

      {isResolved && (
        <div className="space-y-2">
          <div className="rounded-md border border-emerald-500/40 bg-emerald-500/10 p-3 text-sm text-emerald-300">
            ⚖ Dispute resolved. Ruling:{" "}
            <span className="font-mono">{resolvedRuling === "2" ? "pay provider" : "refund"}</span>
            {" "}(ruling={resolvedRuling ?? "?"})
          </div>
          <p className="text-[11px] text-white/40">
            Funds returned to buyer. Agent&apos;s reputation score was slashed.
          </p>
        </div>
      )}
    </section>
  );
}
