"use client";

// Job detail page — status tracking (ERC-8183 4-state + arbitration block),
// deliverable submission (FR-JM03), dispute + ruling display (FR-JM05/JM06),
// and FundFlowChart (FR-JM04).
// DEV-PLAN §Phase 8 任务 8.5a/b/c/d.
//
// Phase 9 wiring: all three write paths (submit / dispute) now go through
// wagmi useWriteContract against the Monad testnet. FundFlowChart reads real
// on-chain Job state via getJobState().

import { use, useEffect, useState } from "react";
import Link from "next/link";
import { useSearchParams } from "next/navigation";
import { ArrowLeft, Loader2, Gavel, FileUp, CheckCircle2, XCircle, Undo2 } from "lucide-react";
import { useAccount, useWriteContract, useWaitForTransactionReceipt, useReadContract } from "wagmi";
import { monadTestnet } from "wagmi/chains";
import { keccak256, toHex, formatUnits } from "viem";

import { JobStatusTracker } from "@/components/job/JobStatusTracker";
import { JobEventTimeline } from "@/components/job/JobEventTimeline";
import { DemoStartPanel } from "@/components/demo/DemoStartPanel";
import { DemoChat } from "@/components/demo/DemoChat";
import { useDemoSession } from "@/hooks/useDemoSession";
import { FundFlowChart } from "@/components/job/FundFlowChart";
import { FundingPathBadge } from "@/components/job/FundingPathBadge";
import { useJobStatusPoll } from "@/hooks/useJobStatusPoll";
import { useJobTimeline } from "@/hooks/useJobTimeline";
import { useEvents } from "@/hooks/useEvents";
import { formatScore } from "@/lib/utils";
import {
  JOB_CONTRACT_ADDRESS,
  JOB_ABI,
  HOOK_CONTRACT_ADDRESS,
  HOOK_ABI,
  PAYMENT_TOKEN_ADDRESS,
  WMON_ADDRESS,
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

  // Escrow amount + currency from the FUNDED event — the real values the
  // task was created with (shown in the header + prefilled in the demo).
  const funded = timeline.find((t) => t.status === "Funded");
  const jobAmountHuman =
    funded?.value !== undefined && funded.value !== ""
      ? formatUnits(BigInt(funded.value), 18)
      : undefined;
  const jobCurrency: "usdc" | "mon" | undefined =
    funded?.symbol === "WMON" ? "mon" : funded?.symbol ? "usdc" : undefined;
  const jobSymbol = funded?.symbol === "WMON" ? "WMON" : "USDC";

  // Read on-chain ruling from resolved dispute events (FR-JM06).
  const { events: disputeEvents } = useEvents({
    eventType: "PRISM_DISPUTE_RESOLVED",
    to: jobId,
    size: 1,
    intervalMs: 15000,
  });
  const resolvedRuling = disputeEvents.length > 0 ? disputeEvents[0].value : null;

  // Buyer's rating for THIS job: VALIDATION_SUBMITTED events carry the job_id
  // they rated (the score the buyer agent gave after reviewing the deliverable).
  const { events: ratingEvents } = useEvents({
    eventType: "PRISM_VALIDATION_SUBMITTED",
    size: 50,
    intervalMs: 15000,
  });
  let jobIdHex = jobId;
  try {
    jobIdHex = "0x" + BigInt(jobId).toString(16).padStart(64, "0");
  } catch {
    // already 0x hex or non-numeric — compare as-is
  }
  const jobRating = ratingEvents.find((e) => e.job_id === jobIdHex)?.value ?? null;

  const canSubmit = status === "Funded" || status === "Assigned" || status === "Submitted"; // resubmission after reject
  const canDispute = status === "Submitted";
  const canReject = status === "Submitted";

  // Shared demo session state: the top Status Tracker follows the live chat
  // flow (liveState overrides the chain state while a demo is running).
  const demo = useDemoSession();

  return (
    <div className="min-h-screen">
      
      <main className="mx-auto max-w-4xl px-6 py-8">
        <Link
          href="/jobs"
          className="mb-4 inline-flex items-center gap-1 text-sm text-white/60 hover:text-white"
        >
          <ArrowLeft className="h-4 w-4" /> 返回任务列表
        </Link>

        <div className="mb-6 flex items-center justify-between">
          <div>
            <h1 className="font-mono text-2xl font-bold text-white">
              Job #<span title={jobId}>{jobId.slice(0, 10)}…{jobId.slice(-4)}</span>
            </h1>
            <p className="mt-1 text-sm text-white/60">
              status:{" "}
              <span className="font-mono text-prism-accent">{status ?? "loading…"}</span>
              {isTerminal && (
                <span className="ml-2 rounded-md border border-emerald-500/40 bg-emerald-500/10 px-1.5 py-0.5 text-[11px] text-emerald-400">
                  terminal
                </span>
              )}
            </p>
            {jobAmountHuman !== undefined && (
              <p className="mt-1 text-sm text-white/60">
                escrow:{" "}
                <span className="font-mono text-white">
                  {jobAmountHuman} {jobSymbol}
                </span>
              </p>
            )}
            {jobRating !== null && (
              <p className="mt-1 text-sm text-white/60">
                buyer rating:{" "}
                <span className="font-mono text-prism-accent">{formatScore(jobRating)}</span>
              </p>
            )}
          </div>
          <FundingPathBadge jobContractAddress={JOB_CONTRACT_ADDRESS} rpcUrl={process.env.NEXT_PUBLIC_RPC_URL} />
        </div>

        {/* Status tracker (8.5a) */}
        <section className="rounded-xl border border-white/10 bg-prism-surface/40 p-5">
          <h2 className="mb-4 text-sm font-medium text-white">Status Tracking</h2>
          <JobStatusTracker
            current={status ?? "Created"}
            timeline={timeline}
            liveState={demo.liveState ?? undefined}
            rejectCount={demo.rejectCount}
          />
        </section>

        {/* Chat-style live collaboration demo (live while it runs, replay
            afterwards) — shown BEFORE the on-chain timeline so the flow is
            read top-down: conversation first, chain events below */}
        <DemoSection demo={demo} jobId={jobId} defaultAmount={jobAmountHuman} defaultToken={jobCurrency} />

        {/* Full lifecycle event stream (submit / reject / dispute / arbitration) —
            appears step-by-step as blocks sync during the demo */}
        <JobEventTimeline timeline={timeline} />

        {/* Deliverable submission (8.5b, FR-JM03) — first submit or resubmit after reject */}
        {canSubmit && <DeliverableSubmit jobId={jobId} />}

        {/* Buyer reject + rework request (reject → resubmit loop) */}
        {canReject && <RejectPanel jobId={jobId} />}

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

// DemoSection — chat-style collaboration demo. On mount it loads the most
// recent demo session for this job (history view); without one it shows the
// start panel. Shares the session state with the top Status Tracker.
function DemoSection({
  demo,
  jobId,
  defaultAmount,
  defaultToken,
}: {
  demo: ReturnType<typeof useDemoSession>;
  jobId: string;
  // Real escrow amount/currency of this job (from the FUNDED event) —
  // prefilled so the demo mirrors the task the user created.
  defaultAmount?: string;
  defaultToken?: "usdc" | "mon";
}) {
  const { sessionId, session, messages, creating, createError, start, loadByJob, reset } = demo;
  const [started, setStarted] = useState(false);

  // Load demo history for this job when the page opens.
  useEffect(() => {
    if (jobId) void loadByJob(jobId);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [jobId]);

  // The demo drives THIS job (the orchestrator resumes it instead of
  // creating a new one), so the status tracker, escrow and the
  // full-lifecycle timeline all stay on the same task.
  const handleStart = async (p: {
    title: string;
    description: string;
    amount: string;
    token?: string;
    provider_agent: string;
    scenario: string;
  }) => {
    const id = await start({ ...p, job_id: jobId });
    if (id) setStarted(true);
  };

  const hasSession = Boolean(sessionId);
  const isFinished = session?.state === "executed" || session?.state === "failed";

  return (
    <div className="space-y-3">
      {!started && !hasSession ? (
        <DemoStartPanel
          onStart={handleStart}
          busy={creating}
          error={createError}
          defaultAmount={defaultAmount}
          defaultToken={defaultToken}
        />
      ) : (
        <>
          <div className="flex items-center justify-between">
            <h3 className="text-sm font-medium text-white">
              {started || !isFinished ? "实时协作演示" : "演示回放"}
            </h3>
            <button
              onClick={() => {
                reset();
                setStarted(false);
              }}
              className="text-xs text-white/40 hover:text-white"
            >
              重新开始
            </button>
          </div>
          {isFinished && (
            <div className="rounded-lg border border-emerald-400/30 bg-emerald-400/10 px-3 py-2 text-xs text-emerald-300">
              ✅ 演示数据已自动保存（链上事件 + 聊天记录）——返回任务列表即可看到当前任务
            </div>
          )}
          <DemoChat messages={messages} session={session} busy={creating && started} />
        </>
      )}
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

  // Per-job payment token (address(0) on-chain resolves to the contract default).
  const { data: paymentToken } = useReadContract({
    address: JOB_CONTRACT_ADDRESS,
    abi: JOB_ABI,
    functionName: "getJobPaymentToken",
    args: [jobIdBig],
    query: { enabled: ready && !Number.isNaN(Number(jobId)) },
  });

  // Human-readable currency label for the job's token.
  function currencyLabel(): string {
    const t = (paymentToken as `0x${string}` | undefined) ?? "";
    if (!t) return "…";
    if (t.toLowerCase() === WMON_ADDRESS.toLowerCase()) return "MON (WMON)";
    if (PAYMENT_TOKEN_ADDRESS && t.toLowerCase() === PAYMENT_TOKEN_ADDRESS.toLowerCase()) return "USDC";
    return t.slice(0, 10) + "…";
  }

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
      <p className="mt-1 text-[11px] text-white/40">
        Escrow currency: <span className="font-mono text-white/60">{currencyLabel()}</span>
        {" "}({Number(amount) > 0 ? "funded" : "unfunded"})
      </p>
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
        Submit Deliverable
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

// ---------- Reject panel: Buyer requests rework (reject → resubmit loop) ----------
function RejectPanel({ jobId }: { jobId: string }) {
  const { address, chain } = useAccount();
  const { writeContractAsync } = useWriteContract();

  const [reason, setReason] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [txHash, setTxHash] = useState<`0x${string}` | null>(null);

  const { isLoading: isConfirming } = useWaitForTransactionReceipt({
    hash: txHash ?? undefined,
  });

  const ready = contractsReady() && JOB_CONTRACT_ADDRESS !== undefined;
  const wrongChain = chain && chain.id !== monadTestnet.id;

  async function handleReject(e: React.FormEvent) {
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
      setError("Job contract address not configured (NEXT_PUBLIC_JOB_CONTRACT_ADDRESS).");
      return;
    }
    if (reason.trim().length < 10) {
      setError("Please describe what needs to change (min 10 chars).");
      return;
    }

    // The opinion is hashed on-chain as the reasonHash (Rejected event).
    const reasonHash = keccak256(toHex(reason)) as `0x${string}`;

    setSubmitting(true);
    try {
      const hash = await writeContractAsync({
        address: JOB_CONTRACT_ADDRESS,
        abi: JOB_ABI,
        functionName: "reject",
        args: [BigInt(jobId), reasonHash],
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
    <section className="mt-6 rounded-xl border border-orange-500/30 bg-orange-500/5 p-5">
      <h2 className="mb-3 flex items-center gap-2 text-sm font-medium text-orange-400">
        <Undo2 className="h-4 w-4" />
        Request Rework (reject)
      </h2>
      <p className="mb-3 text-[11px] text-white/40">
        Buyer-only. Posts a <code className="text-white/60">reject deposit</code> (5% of escrow) which is
        returned when the job settles without arbitration — or forfeited if the buyer loses a later dispute.
        The provider can resubmit an improved deliverable, or open an arbitration if they disagree.
      </p>
      {!done && (
        <form onSubmit={handleReject} className="space-y-3">
          <div>
            <label className="mb-1 block text-xs text-white/60">
              Your opinion — what needs to change? (hashed into reasonHash)
            </label>
            <textarea
              required
              value={reason}
              onChange={(e) => setReason(e.target.value)}
              rows={3}
              placeholder="e.g. the report is missing the on-chain metrics table; please add section 2…"
              className="w-full rounded-md border border-white/10 bg-prism-surface/60 px-3 py-1.5 text-sm text-white placeholder:text-white/30 focus:border-orange-500 focus:outline-none"
            />
          </div>
          {error && <p className="text-xs text-red-400">{error}</p>}
          {txHash && (
            <p className="text-xs text-emerald-400">
              Tx submitted: <code className="font-mono">{txHash.slice(0, 10)}…{txHash.slice(-8)}</code>
              {isConfirming && " (confirming…)"}
            </p>
          )}
          <button
            type="submit"
            disabled={submitting || !ready || !address}
            className="inline-flex items-center gap-2 rounded-md bg-orange-500 px-4 py-2 text-sm font-semibold text-white hover:bg-orange-500/80 disabled:opacity-50"
          >
            {submitting ? (
              <>
                <Loader2 className="h-4 w-4 animate-spin" /> Rejecting…
              </>
            ) : (
              <>
                <XCircle className="h-4 w-4" /> Reject Deliverable
              </>
            )}
          </button>
          <p className="text-[11px] text-white/40">
            Calls <code className="text-white/60">Job.reject(jobId, reasonHash)</code> — requires the buyer to
            have approved the Job contract for USDC (reject deposit).
          </p>
        </form>
      )}
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
        Arbitration
      </h2>
      <p className="mb-3 text-[11px] text-white/40">
        Either party (buyer or provider) may open a dispute within 24h of the latest submit. Both sides post a{" "}
        <code className="text-white/60">dispute deposit</code> (5% of escrow); the losing side&apos;s deposit pays the
        arbitrator, the winner&apos;s is returned, and the escrow goes 100% to the winner.
      </p>

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
