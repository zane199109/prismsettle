"use client";

// Arbitrator panel (P1-4). Lists pending disputes and provides a resolution
// form to call `Hook.resolveDispute(jobId, ruling)` via wagmi.
//
// Ruling semantics (PRD UC-06):
//   ruling=1 → refund buyer (Agent slashed with max(0.2e18, score×30%))
//   ruling=2 → pay provider (Evaluator score written via submitValidation source=2)
//   ruling=0 → INVALID, contract reverts (SD §3.4)
//
// Field mapping (offchain/prismsettle/parser/prismsettle_hook_parser.go):
//   - Disputed:        event.to = jobId, event.token_address = reasonHash
//   - DisputeResolved: event.to = jobId, event.value = ruling

import { Suspense, useMemo, useState } from "react";
import Link from "next/link";
import { useSearchParams } from "next/navigation";
import { Gavel, ArrowLeft, Loader2, ArrowRight } from "lucide-react";
import { useAccount, useWriteContract, useWaitForTransactionReceipt } from "wagmi";
import { monadTestnet } from "wagmi/chains";
import { PageHeader } from "@/components/PageHeader";
import { useEvents } from "@/hooks/useEvents";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Skeleton } from "@/components/ui/skeleton";
import { HOOK_CONTRACT_ADDRESS, HOOK_ABI, contractsReady } from "@/lib/contracts";
import { cn, formatTime } from "@/lib/utils";

type Ruling = 1 | 2;

export default function ArbitratorPage() {
  return (
    <Suspense fallback={<Skeleton className="h-96" />}>
      <ArbitratorBody />
    </Suspense>
  );
}

function ArbitratorBody() {
  const searchParams = useSearchParams();
  const jobId = searchParams.get("jobId");

  if (jobId) {
    return <ResolvePanel jobId={jobId} />;
  }
  return <PendingDisputesList />;
}

function PendingDisputesList() {
  const { events: disputed, isValidating } = useEvents({
    eventType: "PRISM_DISPUTED",
    size: 100,
    intervalMs: 10000,
  });
  const { events: resolved } = useEvents({
    eventType: "PRISM_DISPUTE_RESOLVED",
    size: 100,
    intervalMs: 10000,
  });

  const resolvedSet = useMemo(
    () => new Set(resolved.map((r) => r.to)),
    [resolved],
  );
  const pending = disputed.filter((d) => !resolvedSet.has(d.to));

  return (
    <div className="min-h-screen">
      <PageHeader />
      <main className="mx-auto max-w-7xl px-6 py-8">
        <div className="mb-6">
          <h1 className="flex items-center gap-2 text-2xl font-bold tracking-tight">
            <Gavel className="h-6 w-6 text-prism-accent" />
            Arbitrator Console
          </h1>
          <p className="mt-1 text-sm text-white/60">
            Pending disputes awaiting resolution. Click <em>Resolve</em> to render a ruling.
          </p>
        </div>

        {isValidating && pending.length === 0 ? (
          <Skeleton className="h-40 rounded-xl" />
        ) : pending.length === 0 ? (
          <Card className="border-dashed border-white/20 bg-prism-surface/30 py-16 text-center">
            <CardContent className="py-0">
              <p className="text-sm text-white/60">No pending disputes.</p>
              <p className="mt-1 text-xs text-white/40">
                All filed disputes have been resolved.
              </p>
            </CardContent>
          </Card>
        ) : (
          <div className="space-y-3">
            {pending.map((d) => (
              <Card key={`${d.tx_hash}-${d.log_index ?? 0}`} className="border-white/10 bg-prism-surface/40">
                <CardContent className="flex flex-wrap items-center justify-between gap-3 p-4">
                  <div className="min-w-0">
                    <div className="flex items-center gap-2">
                      <span className="font-mono text-sm text-white">Job #{d.to}</span>
                      <Badge
                        variant="outline"
                        className="border-amber-400/40 bg-amber-400/5 text-[10px] font-bold text-amber-400"
                      >
                        Pending
                      </Badge>
                    </div>
                    <div className="mt-1 text-xs text-white/40">
                      reason: <span className="font-mono">{d.token_address ? `${d.token_address.slice(0, 12)}…${d.token_address.slice(-6)}` : "—"}</span>
                    </div>
                    <div className="mt-0.5 text-[10px] text-white/40">
                      filed {formatTime(d.block_time)}
                    </div>
                  </div>
                  <Link
                    href={`/arbitrator?jobId=${encodeURIComponent(d.to)}`}
                    className="inline-flex items-center gap-1 rounded-md bg-prism-accent px-3 py-1.5 text-xs font-medium text-white hover:bg-prism-accent/80"
                  >
                    Resolve <ArrowRight className="h-3 w-3" />
                  </Link>
                </CardContent>
              </Card>
            ))}
          </div>
        )}
      </main>
    </div>
  );
}

function ResolvePanel({ jobId }: { jobId: string }) {
  const { address, chain } = useAccount();
  const { writeContractAsync, isPending: isWriting } = useWriteContract();
  const [ruling, setRuling] = useState<Ruling>(2);
  const [error, setError] = useState<string | null>(null);
  const [success, setSuccess] = useState<string | null>(null);
  const [txHash, setTxHash] = useState<`0x${string}` | null>(null);

  const { isLoading: isConfirming } = useWaitForTransactionReceipt({
    hash: txHash ?? undefined,
  });

  // Look up this dispute in PRISM_DISPUTED events to display context.
  const { events: disputed } = useEvents({
    eventType: "PRISM_DISPUTED",
    size: 100,
    intervalMs: 15000,
  });
  const { events: resolved } = useEvents({
    eventType: "PRISM_DISPUTE_RESOLVED",
    size: 100,
    intervalMs: 15000,
  });
  const dispute = disputed.find((d) => d.to === jobId);
  const alreadyResolved = resolved.find((r) => r.to === jobId);

  const ready = contractsReady() && HOOK_CONTRACT_ADDRESS !== undefined;
  const wrongChain = chain && chain.id !== monadTestnet.id;

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    setError(null);
    setSuccess(null);
    setTxHash(null);

    if (!address) {
      setError("Connect wallet first.");
      return;
    }
    if (wrongChain) {
      setError(`Switch to Monad Testnet (chainId ${monadTestnet.id}).`);
      return;
    }
    if (!HOOK_CONTRACT_ADDRESS) {
      setError("Hook contract address not configured.");
      return;
    }
    if (alreadyResolved) {
      setError(`Job #${jobId} is already resolved (ruling=${alreadyResolved.value}).`);
      return;
    }

    try {
      // parseBigInt: jobId is decimal string from event.to (hex agentIdFromTopic).
      // Hook.resolveDispute expects uint256 — pass as BigInt.
      const jobIdBig = parseJobId(jobId);
      if (jobIdBig === null) return;
      const hash = await writeContractAsync({
        address: HOOK_CONTRACT_ADDRESS,
        abi: HOOK_ABI,
        functionName: "resolveDispute",
        args: [jobIdBig, ruling],
        chainId: monadTestnet.id,
      });
      setTxHash(hash);
      setSuccess(`resolveDispute(jobId=${jobId}, ruling=${ruling}) tx sent.`);
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    }
  }

  function parseJobId(input: string): bigint | null {
    try {
      return BigInt(input.trim());
    } catch {
      setError(`Invalid jobId: ${input}`);
      return null;
    }
  }

  return (
    <div className="min-h-screen">
      <PageHeader />
      <main className="mx-auto max-w-3xl px-6 py-8">
        <Link
          href="/arbitrator"
          className="mb-4 inline-flex items-center gap-1 text-sm text-white/60 hover:text-white"
        >
          <ArrowLeft className="h-4 w-4" /> Back to pending disputes
        </Link>

        <div className="mb-6">
          <h1 className="flex items-center gap-2 text-2xl font-bold tracking-tight">
            <Gavel className="h-6 w-6 text-prism-accent" />
            Resolve Job #{jobId}
          </h1>
          <p className="mt-1 text-sm text-white/60">
            Render an arbitration ruling. This action triggers `submitValidation` with source=2.
          </p>
        </div>

        {/* Dispute context */}
        <Card className="border-white/10 bg-prism-surface/40">
          <CardHeader className="pb-3">
            <CardTitle className="text-base">Dispute Context</CardTitle>
          </CardHeader>
          <CardContent className="space-y-2 text-xs">
            <div className="flex justify-between">
              <span className="text-white/40">Job ID</span>
              <span className="font-mono text-white">#{jobId}</span>
            </div>
            <div className="flex justify-between">
              <span className="text-white/40">Reason Hash</span>
              <span className="font-mono text-white/80">
                {dispute?.token_address ? `${dispute.token_address.slice(0, 12)}…${dispute.token_address.slice(-6)}` : "—"}
              </span>
            </div>
            <div className="flex justify-between">
              <span className="text-white/40">Filed</span>
              <span className="text-white/80">
                {dispute ? formatTime(dispute.block_time) : "—"}
              </span>
            </div>
            {alreadyResolved && (
              <div className="mt-3 rounded-md border border-emerald-400/30 bg-emerald-400/5 p-2 text-emerald-300">
                Already resolved: ruling={alreadyResolved.value} ({alreadyResolved.value === "1" ? "Refunded" : "Paid"})
              </div>
            )}
          </CardContent>
        </Card>

        {/* Ruling form */}
        <Card className="mt-6 border-white/10 bg-prism-surface/40">
          <CardHeader className="pb-3">
            <CardTitle className="text-base">Ruling</CardTitle>
          </CardHeader>
          <CardContent>
            <form onSubmit={handleSubmit} className="space-y-4">
              <div className="grid gap-3 sm:grid-cols-2">
                <RulingOption
                  value={1}
                  selected={ruling === 1}
                  onClick={() => setRuling(1)}
                  title="Refund Buyer"
                  description="ruling=1. Buyer gets refund. Agent slashed with max(0.2e18, score×30%)."
                  accent="amber"
                />
                <RulingOption
                  value={2}
                  selected={ruling === 2}
                  onClick={() => setRuling(2)}
                  title="Pay Provider"
                  description="ruling=2. Provider receives locked funds. Evaluator score written via submitValidation source=2."
                  accent="emerald"
                />
              </div>

              <p className="rounded-md border border-red-500/30 bg-red-500/5 p-2 text-[11px] text-red-300">
                Note: ruling=0 (invalid) is rejected by the contract per SD §3.4. Only ruling=1 or ruling=2 is accepted.
              </p>

              {error && <p className="text-xs text-red-400">{error}</p>}
              {success && <p className="text-xs text-emerald-400">{success}</p>}
              {txHash && (
                <p className="font-mono text-[11px] text-white/50">
                  tx: {txHash.slice(0, 18)}…{txHash.slice(-8)}
                  {isConfirming && <span className="ml-2 text-amber-300">confirming…</span>}
                </p>
              )}

              <button
                type="submit"
                disabled={isWriting || isConfirming || !address || !ready || !!wrongChain || !!alreadyResolved}
                className="inline-flex items-center gap-2 rounded-md bg-prism-accent px-4 py-2 text-sm font-semibold text-white hover:bg-prism-accent/80 disabled:opacity-40"
              >
                {isWriting || isConfirming ? (
                  <>
                    <Loader2 className="h-4 w-4 animate-spin" /> Submitting…
                  </>
                ) : (
                  <>Submit Ruling (ruling={ruling})</>
                )}
              </button>

              {!address && (
                <p className="text-[11px] text-white/40">Connect wallet (top-right) to resolve disputes.</p>
              )}
              {address && wrongChain && (
                <p className="text-[11px] text-amber-400">Switch to Monad Testnet in your wallet.</p>
              )}
            </form>
          </CardContent>
        </Card>
      </main>
    </div>
  );
}

function RulingOption({
  value,
  selected,
  onClick,
  title,
  description,
  accent,
}: {
  value: Ruling;
  selected: boolean;
  onClick: () => void;
  title: string;
  description: string;
  accent: "amber" | "emerald";
}) {
  const accentClass =
    accent === "amber"
      ? "border-amber-400/60 bg-amber-400/10"
      : "border-emerald-400/60 bg-emerald-400/10";
  return (
    <button
      type="button"
      onClick={onClick}
      className={cn(
        "rounded-lg border p-4 text-left transition-colors",
        selected ? accentClass : "border-white/10 bg-black/20 hover:border-white/30",
      )}
    >
      <div className="flex items-center justify-between">
        <span className="font-mono text-sm font-bold text-white">{title}</span>
        <span className="font-mono text-[10px] text-white/60">ruling={value}</span>
      </div>
      <p className="mt-2 text-[11px] leading-relaxed text-white/60">{description}</p>
    </button>
  );
}
