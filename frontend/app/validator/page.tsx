"use client";

// Validator Console — stake / unstake / withdraw UI + submitValidation entry
// (FR-C03 / UC-02) + stake overview.
// DEV-PLAN §Phase 8 任务 8.6, wired in Phase 9. P0-B extended with Validate tab.
//
// Contract calls (PrismSettleRegistry):
//   stake()                       payable — lock MON as stake (MIN_STAKE = 5 ether)
//   unstake(amount)               — begin 7-day unlock period
//   withdrawUnstaked()            — withdraw after lock expires
//   submitValidation(agentId, score, proofHash, jobId, source=0)  — Validator path

import { useEffect, useState } from "react";
import { Lock, Unlock, ArrowDownToLine, Loader2, CheckCircle2 } from "lucide-react";
import { useAccount, useWriteContract, useWaitForTransactionReceipt, useReadContract } from "wagmi";
import { monadTestnet } from "wagmi/chains";
import { formatEther } from "viem";

import { ValidatorLeaderboard } from "@/components/validators/ValidatorLeaderboard";
import { ValidationRecords } from "@/components/validators/ValidationRecords";
import { SlashHistory } from "@/components/validators/SlashHistory";
import { useEvents } from "@/hooks/useEvents";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { REGISTRY_ADDRESS, REGISTRY_ABI, contractsReady } from "@/lib/contracts";

type Action = "stake" | "unstake" | "withdraw" | "validate";

export default function ValidatorConsolePage() {
  const { address, chain } = useAccount();
  const { writeContractAsync, isPending: isWriting } = useWriteContract();

  const [action, setAction] = useState<Action>("stake");
  const [amount, setAmount] = useState("");
  const [validateAgentId, setValidateAgentId] = useState("");
  const [validateScore, setValidateScore] = useState("");
  const [validateProofHash, setValidateProofHash] = useState("");
  const [validateJobId, setValidateJobId] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [success, setSuccess] = useState<string | null>(null);
  const [txHash, setTxHash] = useState<`0x${string}` | null>(null);

  const { isLoading: isConfirming, isSuccess: isConfirmed } = useWaitForTransactionReceipt({
    hash: txHash ?? undefined,
  });

  // Read on-chain stake info for connected wallet.
  const { data: stakeInfo, refetch: refetchStake } = useReadContract({
    address: REGISTRY_ADDRESS,
    abi: REGISTRY_ABI,
    functionName: "validatorStake",
    args: [address ?? "0x0000000000000000000000000000000000000000"],
    query: { enabled: Boolean(address && REGISTRY_ADDRESS) },
  });

  // Refetch on-chain stake info when the transaction confirms, instead of
  // using a blind setTimeout that could fire before or after confirmation.
  // Must be placed after useReadContract to avoid TDZ on refetchStake.
  useEffect(() => {
    if (isConfirmed) {
      refetchStake();
    }
  }, [isConfirmed, refetchStake]);

  // P1-3: Validator earnings tracking (mock). Aggregate this wallet's
  // PRISM_VALIDATION_SUBMITTED events and multiply by a mock unit reward
  // of 0.001 MON. Real reward logic lands in V2 (PRD FR-C11).
  const { events: myValidations } = useEvents({
    eventType: "PRISM_VALIDATION_SUBMITTED",
    size: 200,
    intervalMs: 15000,
  });
  const myValidationCount = address
    ? myValidations.filter((e) => e.from.toLowerCase() === address.toLowerCase()).length
    : 0;
  const mockRewardEth = myValidationCount * 0.001;

  const ready = contractsReady() && REGISTRY_ADDRESS !== undefined;
  const wrongChain = chain && chain.id !== monadTestnet.id;

  // stakeInfo is [amount, pendingUnstake, unstakeAt] tuple from contract.
  const stakeTuple = stakeInfo as readonly [bigint, bigint, bigint] | undefined;
  const stakeAmount = stakeTuple ? stakeTuple[0] : 0n;
  const pendingUnstake = stakeTuple ? stakeTuple[1] : 0n;
  const unstakeAt = stakeTuple ? stakeTuple[2] : 0n;
  const lockExpired = unstakeAt > 0n && BigInt(Math.floor(Date.now() / 1000)) >= unstakeAt + 7n * 86400n;

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
    if (!REGISTRY_ADDRESS) {
      setError("Registry address not configured.");
      return;
    }

    try {
      let hash: `0x${string}`;
      if (action === "stake") {
        const value = parseAmount(amount);
        if (value === null) return;
        hash = await writeContractAsync({
          address: REGISTRY_ADDRESS,
          abi: REGISTRY_ABI,
          functionName: "stake",
          value,
          chainId: monadTestnet.id,
        });
      } else if (action === "unstake") {
        const value = parseAmount(amount);
        if (value === null) return;
        hash = await writeContractAsync({
          address: REGISTRY_ADDRESS,
          abi: REGISTRY_ABI,
          functionName: "unstake",
          args: [value],
          chainId: monadTestnet.id,
        });
      } else if (action === "validate") {
        // submitValidation(agentId, score, proofHash, jobId, source=0) — Validator path (FR-C03 / UC-02).
        const agentIdBig = parseBigInt(validateAgentId, "agentId");
        if (agentIdBig === null) return;
        const scoreBig = parseScore(validateScore);
        if (scoreBig === null) return;
        const proofHash = parseBytes32(validateProofHash);
        if (proofHash === null) return;
        const jobIdBig = validateJobId.trim() === "" ? 0n : parseBigInt(validateJobId, "jobId");
        if (jobIdBig === null) return;
        hash = await writeContractAsync({
          address: REGISTRY_ADDRESS,
          abi: REGISTRY_ABI,
          functionName: "submitValidation",
          args: [agentIdBig, scoreBig, proofHash, jobIdBig, 0],
          chainId: monadTestnet.id,
        });
      } else {
        hash = await writeContractAsync({
          address: REGISTRY_ADDRESS,
          abi: REGISTRY_ABI,
          functionName: "withdrawUnstaked",
          chainId: monadTestnet.id,
        });
      }
      setTxHash(hash);
      setSuccess(`${action} tx sent.`);
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    }
  }

  function parseAmount(input: string): bigint | null {
    // Convert MON decimal string to wei without floating-point precision loss.
    // Handles arbitrary decimals by string-splitting on "." instead of
    // parseFloat, which would lose precision on values like 5.123456789012345678.
    try {
      const trimmed = input.trim();
      if (trimmed === "") {
        setError("Amount is required.");
        return null;
      }
      const parts = trimmed.split(".");
      const intPart = parts[0].replace(/^0+/, "") || "0";
      let decPart = parts[1] ?? "";
      if (decPart.length > 18) decPart = decPart.slice(0, 18);
      decPart = decPart.padEnd(18, "0");
      const wei = BigInt(intPart + decPart);
      if (wei <= 0n) {
        setError("Amount must be positive.");
        return null;
      }
      return wei;
    } catch {
      setError("Invalid amount.");
      return null;
    }
  }

  function parseBigInt(input: string, field: string): bigint | null {
    // Accept decimal or 0x hex string (BigInt handles both).
    try {
      const v = BigInt(input.trim());
      if (v < 0n) {
        setError(`${field} must be non-negative.`);
        return null;
      }
      return v;
    } catch {
      setError(`Invalid ${field}.`);
      return null;
    }
  }

  function parseScore(input: string): bigint | null {
    // Accept decimal in [0, 1] and scale to 1e18 without floating-point
    // precision loss. String-split avoids IEEE 754 rounding from parseFloat.
    const trimmed = input.trim();
    const f = parseFloat(trimmed);
    if (isNaN(f) || f < 0 || f > 1) {
      setError("Score must be in [0, 1].");
      return null;
    }
    // String-split for exact BigInt conversion.
    const parts = trimmed.split(".");
    const intPart = parts[0] || "0";
    let decPart = parts[1] ?? "";
    if (decPart.length > 18) decPart = decPart.slice(0, 18);
    decPart = decPart.padEnd(18, "0");
    const scale = BigInt(intPart) * 10n ** 18n + BigInt(decPart);
    // Sanity: 18-decimal BigInt must fit the same [0, 1e18] range as before.
    if (scale < 0n || scale > 10n ** 18n) {
      setError("Score out of range.");
      return null;
    }
    return scale;
  }

  function parseBytes32(input: string): `0x${string}` | null {
    // Require 0x + 64 hex chars (bytes32).
    const s = input.trim();
    if (!s.startsWith("0x") || s.length !== 66) {
      setError("proofHash must be 0x + 64 hex chars (bytes32).");
      return null;
    }
    try {
      // Validate hex via BigInt parse.
      BigInt(s);
      return s as `0x${string}`;
    } catch {
      setError("Invalid proofHash hex.");
      return null;
    }
  }

  return (
    <div className="min-h-screen">
      
      <main id="main" className="mx-auto max-w-7xl px-6 py-8">
        <div className="mb-6">
          <h1 className="text-2xl font-bold tracking-tight">Validator</h1>
          <p className="mt-1 text-sm text-white/60">
            Stake MON to validate agent outputs. Unstake enters a 7-day lock before withdrawal.
          </p>
        </div>

        {/* Top: leaderboard + records + slashes */}
        <section className="space-y-6">
          <ValidatorLeaderboard limit={10} />
          <div className="grid gap-6 lg:grid-cols-2">
            <ValidationRecords limit={10} />
            <SlashHistory limit={10} />
          </div>
        </section>

        {/* Bottom: personal stake console */}
        <section className="mt-8">
          <Card className="border-white/10 bg-prism-surface/40">
            <CardHeader className="pb-3">
              <CardTitle className="text-base">Your Stake</CardTitle>
            </CardHeader>
            <CardContent className="space-y-5">
              {/* Wallet / stake overview */}
              <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
                <Stat
                  label="Your Stake"
                  value={formatEther(stakeAmount)}
                  unit="MON"
                  tone="text-emerald-400"
                />
                <Stat
                  label="Pending Unstake"
                  value={pendingUnstake > 0n ? formatEther(pendingUnstake) : "0"}
                  unit="MON"
                  tone={pendingUnstake > 0n ? "text-amber-400" : "text-white/60"}
                />
                <Stat
                  label="Wallet"
                  value={address ? `${address.slice(0, 6)}…${address.slice(-4)}` : "Not connected"}
                  tone={address ? "text-prism-accent" : "text-white/60"}
                />
                <div className="relative rounded-lg border border-prism-glow/20 bg-prism-glow/5 p-4">
                  <div className="flex items-center justify-between">
                    <div className="text-[10px] uppercase tracking-wider text-white/40">
                      Total Rewards
                    </div>
                    <Badge
                      variant="outline"
                      className="border-prism-glow/40 bg-prism-glow/10 px-1.5 py-0 text-[9px] font-bold text-prism-glow"
                    >
                      mock
                    </Badge>
                  </div>
                  <div className="mt-1 flex items-baseline gap-1">
                    <span className="font-mono text-xl font-bold text-prism-glow">
                      {mockRewardEth.toFixed(3)}
                    </span>
                    <span className="text-[10px] text-white/40">MON</span>
                  </div>
                  <div className="mt-1 text-[10px] text-white/40">
                    {myValidationCount} validations · 0.001 MON each (mock rate, V2 lands in FR-C11)
                  </div>
                </div>
              </div>

              {/* Action tabs */}
              <div className="rounded-xl border border-white/10 bg-black/20 p-5">
                <div className="mb-4 flex gap-1 rounded-md bg-black/30 p-1">
                  <Tab id="stake" current={action} onChange={setAction} icon={<Lock className="h-3.5 w-3.5" />} label="Stake" />
                  <Tab id="unstake" current={action} onChange={setAction} icon={<Unlock className="h-3.5 w-3.5" />} label="Unstake" />
                  <Tab id="withdraw" current={action} onChange={setAction} icon={<ArrowDownToLine className="h-3.5 w-3.5" />} label="Withdraw" />
                  <Tab id="validate" current={action} onChange={setAction} icon={<CheckCircle2 className="h-3.5 w-3.5" />} label="Validate" />
                </div>

                <form onSubmit={handleSubmit} className="space-y-3">
                  {action === "stake" && (
                    <div>
                      <label className="mb-1 block text-xs text-white/60">amount (MON)</label>
                      <input
                        type="text"
                        required
                        value={amount}
                        onChange={(e) => setAmount(e.target.value)}
                        placeholder="5.0"
                        className="w-full rounded-md border border-white/10 bg-prism-surface/60 px-3 py-1.5 font-mono text-sm text-white placeholder:text-white/30 focus:border-prism-accent focus:outline-none"
                      />
                      <p className="mt-1 text-[11px] text-white/40">MIN_STAKE = 5 MON to submit validations.</p>
                    </div>
                  )}

                  {action === "unstake" && (
                    <div>
                      <label className="mb-1 block text-xs text-white/60">amount (MON)</label>
                      <input
                        type="text"
                        required
                        value={amount}
                        onChange={(e) => setAmount(e.target.value)}
                        placeholder="1.0"
                        className="w-full rounded-md border border-white/10 bg-prism-surface/60 px-3 py-1.5 font-mono text-sm text-white placeholder:text-white/30 focus:border-prism-accent focus:outline-none"
                      />
                      <p className="mt-1 text-[11px] text-white/40">
                        Current stake: {formatEther(stakeAmount)} MON. Enters 7-day lock.
                      </p>
                    </div>
                  )}

                  {action === "withdraw" && (
                    <p className="rounded-md border border-amber-500/30 bg-amber-500/5 p-2 text-[11px] text-amber-300">
                      {pendingUnstake > 0n
                        ? lockExpired
                          ? `Ready to withdraw ${formatEther(pendingUnstake)} MON.`
                          : `Lock active. Unlocks at epoch ${unstakeAt.toString()}.`
                        : "No pending unstake."}
                    </p>
                  )}

                  {action === "validate" && (
                    <div className="space-y-3">
                      <div>
                        <label className="mb-1 block text-xs text-white/60">agentId (uint256)</label>
                        <input
                          type="text"
                          required
                          value={validateAgentId}
                          onChange={(e) => setValidateAgentId(e.target.value)}
                          placeholder="1 or 0x..."
                          className="w-full rounded-md border border-white/10 bg-prism-surface/60 px-3 py-1.5 font-mono text-sm text-white placeholder:text-white/30 focus:border-prism-accent focus:outline-none"
                        />
                      </div>
                      <div>
                        <label className="mb-1 block text-xs text-white/60">score (0..1, e.g. 0.85)</label>
                        <input
                          type="text"
                          required
                          value={validateScore}
                          onChange={(e) => setValidateScore(e.target.value)}
                          placeholder="0.85"
                          className="w-full rounded-md border border-white/10 bg-prism-surface/60 px-3 py-1.5 font-mono text-sm text-white placeholder:text-white/30 focus:border-prism-accent focus:outline-none"
                        />
                        <p className="mt-1 text-[11px] text-white/40">Scaled to 1e18 — 0.85 → 0.85e18.</p>
                      </div>
                      <div>
                        <label className="mb-1 block text-xs text-white/60">proofHash (bytes32, 0x + 64 hex)</label>
                        <input
                          type="text"
                          required
                          value={validateProofHash}
                          onChange={(e) => setValidateProofHash(e.target.value)}
                          placeholder="0x..."
                          className="w-full rounded-md border border-white/10 bg-prism-surface/60 px-3 py-1.5 font-mono text-sm text-white placeholder:text-white/30 focus:border-prism-accent focus:outline-none"
                        />
                      </div>
                      <div>
                        <label className="mb-1 block text-xs text-white/60">jobId (uint256, optional — defaults to 0)</label>
                        <input
                          type="text"
                          value={validateJobId}
                          onChange={(e) => setValidateJobId(e.target.value)}
                          placeholder="0"
                          className="w-full rounded-md border border-white/10 bg-prism-surface/60 px-3 py-1.5 font-mono text-sm text-white placeholder:text-white/30 focus:border-prism-accent focus:outline-none"
                        />
                      </div>
                      <p className="rounded-md border border-prism-accent/30 bg-prism-accent/5 p-2 text-[11px] text-prism-accent/80">
                        source=0 (Validator). Evaluator source=1/2 is auto-triggered off-chain after job completion / arbitration.
                      </p>
                      {stakeAmount === 0n && (
                        <p className="text-[11px] text-amber-400">Stake at least 5 MON first — Validators only (FR-C03).</p>
                      )}
                    </div>
                  )}

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
                    disabled={isWriting || isConfirming || !address || !ready || !!wrongChain}
                    className="inline-flex items-center gap-2 rounded-md bg-prism-accent px-4 py-2 text-sm font-semibold text-white hover:bg-prism-accent/80 disabled:opacity-40"
                  >
                    {isWriting || isConfirming ? (
                      <>
                        <Loader2 className="h-4 w-4 animate-spin" /> Submitting…
                      </>
                    ) : (
                      <>{action.charAt(0).toUpperCase() + action.slice(1)}</>
                    )}
                  </button>

                  {!address && (
                    <p className="text-[11px] text-white/40">Connect wallet (top-right) to interact with the registry.</p>
                  )}
                  {address && wrongChain && (
                    <p className="text-[11px] text-amber-400">Switch to Monad Testnet in your wallet.</p>
                  )}
                </form>
              </div>
            </CardContent>
          </Card>
        </section>
      </main>
    </div>
  );
}

function Stat({
  label,
  value,
  unit,
  tone,
}: {
  label: string;
  value: string;
  unit?: string;
  tone: string;
}) {
  return (
    <div className="rounded-lg border border-white/10 bg-prism-surface/40 p-4">
      <div className="text-[10px] uppercase tracking-wider text-white/40">{label}</div>
      <div className="mt-1 flex items-baseline gap-1">
        <span className={`font-mono text-xl font-bold ${tone}`}>{value}</span>
        {unit && <span className="text-[10px] text-white/40">{unit}</span>}
      </div>
    </div>
  );
}

function Tab({
  id,
  current,
  onChange,
  icon,
  label,
}: {
  id: Action;
  current: Action;
  onChange: (a: Action) => void;
  icon: React.ReactNode;
  label: string;
}) {
  const active = id === current;
  return (
    <button
      type="button"
      onClick={() => onChange(id)}
      className={`flex flex-1 items-center justify-center gap-1.5 rounded-md px-3 py-1.5 text-sm font-medium transition-colors ${
        active ? "bg-prism-accent text-white" : "text-white/60 hover:text-white"
      }`}
    >
      {icon} {label}
    </button>
  );
}
