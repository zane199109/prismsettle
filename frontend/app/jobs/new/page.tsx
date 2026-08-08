"use client";

// New Job page — create a job with TrustGate integration + ERC-20 funding.
// DEV-PLAN §Phase 8 任务 8.5a, wired in Phase 9.
//
// Flow (two on-chain steps):
//   1. createJob(agentId, 0, deadline, hook) → parse JobCreated event for jobId
//   2. ERC20 approve(jobContract, amount) if allowance insufficient
//   3. fundViaToken(jobId, amount, "0x")  // empty receipt = ERC-20 path
// Then route to /jobs/[jobId].

import { Suspense, useEffect, useMemo, useState } from "react";
import Link from "next/link";
import { useRouter, useSearchParams } from "next/navigation";
import { ArrowRight, Loader2 } from "lucide-react";
import { useAccount, useWriteContract, useWaitForTransactionReceipt, useReadContract } from "wagmi";
import { monadTestnet } from "wagmi/chains";
import { decodeEventLog, parseAbiItem, parseUnits } from "viem";

import { TrustGate } from "@/components/job/TrustGate";
import { FundingPathBadge } from "@/components/job/FundingPathBadge";
import { useAgents } from "@/hooks/useAgents";
import { agentCategory, parseNaturalLanguage } from "@/lib/utils";
import type { TrustCheckResult } from "@/lib/types";
import {
  JOB_CONTRACT_ADDRESS,
  JOB_ABI,
  HOOK_CONTRACT_ADDRESS,
  ERC20_ABI,
  PAYMENT_TOKEN_ADDRESS,
  WMON_ADDRESS,
  WMON_ABI,
  paymentTokenFor,
  contractsReady,
} from "@/lib/contracts";

const RPC_URL = process.env.NEXT_PUBLIC_RPC_URL;

// JobCreated event signature for log parsing. Must match the full on-chain
// event including the minProviderReputation and paymentToken fields.
const JOB_CREATED_EVENT = parseAbiItem(
  "event JobCreated(uint256 indexed agentId, uint256 indexed jobId, address buyer, uint64 deadline, address hook, uint96 minProviderReputation, address paymentToken)"
);


function NewJobBody() {
  const router = useRouter();
  const searchParams = useSearchParams();
  const initialAgent = searchParams.get("agent") ?? "";

  const { address, chain } = useAccount();
  const { writeContractAsync, isPending: isWriting } = useWriteContract();

  const { agents } = useAgents({ size: 100 });
  // Agents owned by the connected wallet (agentId binds to wallet owner).
  const myAgents = agents.filter(
    (a) => a.owner.toLowerCase() === (address ?? "").toLowerCase()
  );

  const [agentId, setAgentId] = useState(initialAgent);
  // deadline stored as a datetime-local string; converted to epoch seconds on submit.
  // Prefilled to now+24h so the form is usable without typing.
  const [deadline, setDeadline] = useState(() => defaultDeadline());
  const [amount, setAmount] = useState("5");
  const [minProviderReputation, setMinProviderReputation] = useState("0.5");
  const [currency, setCurrency] = useState<"usdc" | "mon">("usdc");
  const [wrapAmount, setWrapAmount] = useState("");
  const [decision, setDecision] = useState<TrustCheckResult["decision"] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [stepLabel, setStepLabel] = useState<string | null>(null);
  const [txHash, setTxHash] = useState<`0x${string}` | null>(null);
  const [nlInput, setNlInput] = useState(
    "帮我创建 5 USDC 的智能合约安全审计任务给审计 agent，1 天内完成，最低信誉 0.5",
  );
  const [nlApplied, setNlApplied] = useState(false);

  const { isLoading: isConfirming } = useWaitForTransactionReceipt({
    hash: txHash ?? undefined,
  });

  // The job's payment token for the selected currency:
  // createJob arg — 0 = contract default (USDC), or WMON for MON mode.
  const paymentTokenArg = paymentTokenFor(currency);
  // The real ERC-20 contract to approve/query for the selected currency.
  const erc20Address = currency === "mon" ? WMON_ADDRESS : PAYMENT_TOKEN_ADDRESS;

  // Read current ERC-20 allowance to decide if approve is needed.
  const { data: allowance } = useReadContract({
    address: erc20Address,
    abi: ERC20_ABI,
    functionName: "allowance",
    args: [address ?? "0x0", JOB_CONTRACT_ADDRESS ?? "0x0"],
    query: { enabled: Boolean(address && erc20Address && JOB_CONTRACT_ADDRESS) },
  });

  // WMON balance (MON mode) to decide whether wrapping is needed.
  const { data: wmonBalance } = useReadContract({
    address: WMON_ADDRESS,
    abi: ERC20_ABI,
    functionName: "balanceOf",
    args: [address ?? "0x0"],
    query: { enabled: Boolean(address) },
  });

  const ready = contractsReady() && JOB_CONTRACT_ADDRESS !== undefined;
  const wrongChain = chain && chain.id !== monadTestnet.id;
  const blocked = decision === "deny";

  // Auto-pick the first wallet-owned agent once the wallet connects.
  useEffect(() => {
    if (address && myAgents.length > 0 && !agentId) {
      setAgentId(myAgents[0].agent_id);
    }
  }, [address, myAgents]); // eslint-disable-line react-hooks/exhaustive-deps

  // Parse the natural-language request and map it onto the form fields.
  function applyNaturalLanguage() {
    const parsed = parseNaturalLanguage(nlInput);
    if (parsed.amount) setAmount(parsed.amount);
    if (parsed.currency) setCurrency(parsed.currency);
    if (parsed.minRep) setMinProviderReputation(parsed.minRep);
    if (parsed.deadlineIn) {
      const d = new Date(Date.now() + parsed.deadlineIn * 86400_000);
      const local = new Date(d.getTime() - d.getTimezoneOffset() * 60000).toISOString().slice(0, 16);
      setDeadline(local);
    }
    if (parsed.agentTag) {
      const tag = parsed.agentTag;
      const matched =
        myAgents.find((a) => agentCategory(a.metadata).toLowerCase() === tag) ??
        myAgents[0];
      if (matched) setAgentId(matched.agent_id);
    }
    setNlApplied(true);
  }

  // Human-friendly amounts: parseUnits converts "100" / "0.5" to wei (18 decimals).
  // Returns -1n for invalid input so callers can surface an error.
  const parseAmount = (v: string): bigint => {
    if (!v) return 0n;
    try {
      return parseUnits(v, 18);
    } catch {
      return -1n;
    }
  };
  const amountBig = parseAmount(amount);
  const allowanceBig = (allowance as bigint | undefined) ?? 0n;
  const needsApprove = allowanceBig < amountBig;
  // MON mode: wrapping only needed when the wallet's WMON balance is short.
  const wmonBalanceBig = (wmonBalance as bigint | undefined) ?? 0n;
  const needsWrap = currency === "mon" && wmonBalanceBig < amountBig;
  const wrapAmountBig = parseAmount(wrapAmount);

  async function handleWrap(e: React.FormEvent) {
    e.preventDefault();
    if (!address || wrongChain || wrapAmountBig <= 0n) return;
    setError(null);
    setTxHash(null);
    try {
      setStepLabel("Wrapping MON → WMON…");
      const wrapHash = await writeContractAsync({
        address: WMON_ADDRESS,
        abi: WMON_ABI,
        functionName: "deposit",
        value: wrapAmountBig,
        chainId: monadTestnet.id,
      });
      setTxHash(wrapHash);
      await waitForReceipt(wrapHash);
      setStepLabel(null);
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
      setStepLabel(null);
    }
  }

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    if (blocked || !address || !JOB_CONTRACT_ADDRESS || (currency === "usdc" && !PAYMENT_TOKEN_ADDRESS)) return;
    setError(null);
    setTxHash(null);

    if (amountBig < 0n) {
      setError("Invalid amount (use e.g. 100).");
      return;
    }
    if (amountBig <= 0n) {
      setError("Amount must be greater than 0.");
      return;
    }

    if (wrongChain) {
      setError(`Switch to Monad Testnet (chainId ${monadTestnet.id}).`);
      return;
    }

    let agentIdBig: bigint;
    try {
      agentIdBig = BigInt(agentId.startsWith("0x") ? agentId : agentId);
    } catch {
      setError("Invalid Agent ID.");
      return;
    }
    const deadlineBig = deadline
      ? BigInt(Math.floor(new Date(deadline).getTime() / 1000))
      : 0n;
    if (deadlineBig <= BigInt(Math.floor(Date.now() / 1000))) {
      setError("Deadline must be in the future.");
      return;
    }
    const minRepBig = minProviderReputation ? parseAmount(minProviderReputation) : 0n;
    if (minRepBig < 0n) {
      setError("Invalid min reputation (use e.g. 0.5).");
      return;
    }

    try {
      // Step 1: createJob — per-job payment token (0 = default USDC, or WMON)
      setStepLabel("Step 1/3: Creating job on-chain…");
      const createHash = await writeContractAsync({
        address: JOB_CONTRACT_ADDRESS,
        abi: JOB_ABI,
        functionName: "createJob",
        args: [agentIdBig, 0n, deadlineBig, HOOK_CONTRACT_ADDRESS ?? "0x0000000000000000000000000000000000000000", minRepBig, paymentTokenArg],
        chainId: monadTestnet.id,
      });
      setTxHash(createHash);
      // Wait for receipt to parse JobCreated event.
      const receipt = await waitForReceipt(createHash);
      let jobId: bigint | null = null;
      for (const log of receipt.logs) {
        try {
          const decoded = decodeEventLog({
            abi: [JOB_CREATED_EVENT],
            data: log.data,
            topics: log.topics,
          });
          if (decoded.eventName === "JobCreated") {
            jobId = (decoded.args as { jobId: bigint }).jobId;
            break;
          }
        } catch {
          // skip non-matching logs
        }
      }
      if (jobId === null) {
        setError("JobCreated event not found in receipt. Check tx on explorer.");
        return;
      }

      // Step 2: approve if needed
      if (needsApprove) {
        setStepLabel("Step 2/3: Approving ERC-20 transfer…");
        setTxHash(null);
        const approveHash = await writeContractAsync({
          address: erc20Address!,
          abi: ERC20_ABI,
          functionName: "approve",
          args: [JOB_CONTRACT_ADDRESS, amountBig],
          chainId: monadTestnet.id,
        });
        setTxHash(approveHash);
        await waitForReceipt(approveHash);
      } else {
        setStepLabel("Step 2/3: Allowance sufficient, skipping approve…");
      }

      // Step 3: fundViaToken (empty receipt = ERC-20 path)
      setStepLabel("Step 3/3: Funding job…");
      setTxHash(null);
      const fundHash = await writeContractAsync({
        address: JOB_CONTRACT_ADDRESS,
        abi: JOB_ABI,
        functionName: "fundViaToken",
        args: [jobId, amountBig, "0x" as `0x${string}`],
        chainId: monadTestnet.id,
      });
      setTxHash(fundHash);
      await waitForReceipt(fundHash);

      setStepLabel(null);
      router.push(`/jobs/${jobId.toString()}?agent=${encodeURIComponent(agentId)}`);
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
      setStepLabel(null);
    }
  }

  // Helper: poll for receipt via wagmi hook is not possible inside async,
  // so we use viem client directly.
  async function waitForReceipt(hash: `0x${string}`) {
    const { createPublicClient, http } = await import("viem");
    const client = createPublicClient({
      chain: monadTestnet,
      transport: http(RPC_URL ?? "https://testnet-rpc.monad.xyz"),
    });
    return client.waitForTransactionReceipt({ hash });
  }

  return (
    <div className="min-h-screen">
      
      <main className="mx-auto max-w-3xl px-6 py-8">
        <h1 className="text-2xl font-bold tracking-tight">Create New Job</h1>
        <p className="mt-1 text-sm text-white/60">
          Trust gate checks agent reputation before creation. Funding uses ERC-20 transferFrom.
        </p>

        {/* Natural-language quick input */}
        <div className="mt-6 rounded-lg border border-prism-accent/30 bg-prism-accent/5 p-4">
          <label className="mb-1.5 block text-sm font-medium text-white/80">
            Describe the job in plain words
            <span className="ml-2 text-xs font-normal text-white/40">— auto-fills the form below</span>
          </label>
          <div className="flex gap-2">
            <input
              type="text"
              value={nlInput}
              onChange={(e) => setNlInput(e.target.value)}
              onKeyDown={(e) => e.key === "Enter" && (e.preventDefault(), applyNaturalLanguage())}
              placeholder='e.g. 帮我创建 5 USDC 的任务给做数据标注的 agent，3 天内完成，最低信誉 0.5'
              className="flex-1 rounded-lg border border-white/10 bg-black/30 px-3 py-2 text-sm text-white placeholder:text-white/30 focus:border-prism-accent focus:outline-none"
            />
            <button
              type="button"
              onClick={applyNaturalLanguage}
              className="shrink-0 rounded-lg bg-prism-accent px-4 py-2 text-sm font-medium text-black hover:bg-prism-accent/80"
            >
              Apply
            </button>
          </div>
          {nlApplied && (
            <p className="mt-2 text-[11px] text-emerald-400">Applied — review the fields below before submitting.</p>
          )}
        </div>

        <form onSubmit={handleSubmit} className="mt-8 space-y-6">
          {/* Agent selector — bound to the connected wallet */}
          <div>
            <label className="mb-1.5 block text-sm font-medium text-white/80">
              Agent <span className="text-white/40">(your registered agents)</span>
            </label>
            {address ? (
              myAgents.length > 0 ? (
                <select
                  required
                  value={agentId}
                  onChange={(e) => setAgentId(e.target.value)}
                  className="w-full rounded-lg border border-white/10 bg-prism-surface/60 px-3 py-2 font-mono text-sm text-white focus:border-prism-accent focus:outline-none"
                >
                  {myAgents.map((a) => (
                    <option key={a.agent_id} value={a.agent_id} className="bg-zinc-900">
                      {a.agent_id.length > 20 ? `${a.agent_id.slice(0, 14)}…${a.agent_id.slice(-6)}` : a.agent_id}
                      {a.score && a.score !== "0" ? ` · ${(Number(BigInt(a.score)) / 1e18).toFixed(2)} rep` : " · new"}
                      {a.metadata ? ` · ${agentCategory(a.metadata)}` : ""}
                    </option>
                  ))}
                </select>
              ) : (
                <div className="rounded-lg border border-amber-500/30 bg-amber-500/5 p-3 text-sm text-amber-300">
                  This wallet has no registered agent yet. Register one on the{" "}
                  <Link href="/agents" className="underline">Agents page</Link> to create jobs.
                </div>
              )
            ) : (
              <p className="rounded-lg border border-white/10 bg-prism-surface/60 p-3 text-sm text-white/50">
                Connect your wallet to see the agents bound to it.
              </p>
            )}
            {agentId && (
              <div className="mt-3">
                <TrustGate agentId={agentId} onDecisionChange={setDecision} />
              </div>
            )}
          </div>

          {/* Currency selector (per-job payment token) */}
          <div>
            <label className="mb-1.5 block text-sm font-medium text-white/80">Currency</label>
            <div className="grid grid-cols-2 gap-2">
              <button
                type="button"
                onClick={() => setCurrency("usdc")}
                className={`rounded-lg border px-3 py-2 text-sm font-medium transition ${
                  currency === "usdc"
                    ? "border-prism-accent bg-prism-accent/10 text-prism-accent"
                    : "border-white/10 bg-prism-surface/60 text-white/60 hover:border-white/20"
                }`}
              >
                USDC
                <span className="block text-[10px] font-normal text-white/40">default token</span>
              </button>
              <button
                type="button"
                onClick={() => setCurrency("mon")}
                className={`rounded-lg border px-3 py-2 text-sm font-medium transition ${
                  currency === "mon"
                    ? "border-prism-accent bg-prism-accent/10 text-prism-accent"
                    : "border-white/10 bg-prism-surface/60 text-white/60 hover:border-white/20"
                }`}
              >
                MON (WMON)
                <span className="block text-[10px] font-normal text-white/40">wrapped MON escrow</span>
              </button>
            </div>
            {currency === "mon" && (
              <p className="mt-1 text-[11px] text-white/40">
                Escrow is held in WMON ({WMON_ADDRESS}). Wrap MON below if your WMON balance is short.
              </p>
            )}
          </div>

          {/* Wrap MON → WMON (MON mode, when WMON balance is short) */}
          {currency === "mon" && needsWrap && (
            <form onSubmit={handleWrap} className="rounded-lg border border-white/10 bg-prism-surface/40 p-3">
              <label className="mb-1.5 block text-sm font-medium text-white/80">
                Wrap MON → WMON <span className="text-white/40">(balance {(wmonBalanceBig / 10n ** 18n).toString()} WMON)</span>
              </label>
              <div className="flex gap-2">
                <input
                  type="text"
                  required
                  value={wrapAmount}
                  onChange={(e) => setWrapAmount(e.target.value)}
                  placeholder="e.g. 100 (MON to wrap)"
                  className="flex-1 rounded-lg border border-white/10 bg-prism-surface/60 px-3 py-2 font-mono text-sm text-white placeholder:text-white/30 focus:border-prism-accent focus:outline-none"
                />
                <button
                  type="submit"
                  disabled={isWriting || wrapAmountBig <= 0n}
                  className="rounded-lg bg-prism-accent px-4 py-2 text-sm font-medium text-black disabled:opacity-50"
                >
                  Wrap
                </button>
              </div>
            </form>
          )}

          {/* Funding path detection */}
          <div>
            <div className="mb-1.5 flex items-center justify-between">
              <label className="block text-sm font-medium text-white/80">Funding Path</label>
              <FundingPathBadge jobContractAddress={JOB_CONTRACT_ADDRESS} rpcUrl={RPC_URL} />
            </div>
            <input
              type="text"
              required
              value={amount}
              onChange={(e) => setAmount(e.target.value)}
              placeholder="e.g. 100 (USDC or MON)"
              className="w-full rounded-lg border border-white/10 bg-prism-surface/60 px-3 py-2 font-mono text-sm text-white placeholder:text-white/30 focus:border-prism-accent focus:outline-none disabled:opacity-50"
            />
            <p className="mt-1 text-[11px] text-white/40">
              If facilitator is address(0), funds go via ERC-20 transferFrom.
              Otherwise x402 receipt is forwarded to the facilitator.
            </p>
            {needsApprove && amount && (
              <p className="mt-1 text-[11px] text-amber-400">ERC-20 allowance insufficient — will approve first.</p>
            )}
          </div>

          {/* Deadline — calendar picker */}
          <div>
            <label className="mb-1.5 block text-sm font-medium text-white/80">
              Deadline <span className="text-white/40">(pick date &amp; time)</span>
            </label>
            <input
              type="datetime-local"
              required
              value={deadline}
              onChange={(e) => setDeadline(e.target.value)}
              className="w-full rounded-lg border border-white/10 bg-prism-surface/60 px-3 py-2 text-sm text-white focus:border-prism-accent focus:outline-none"
            />
            <p className="mt-1 text-[11px] text-white/40">Converted to epoch seconds on-chain (e.g. tomorrow 14:00).</p>
          </div>
          {/* Min Provider Reputation */}
          <div>
            <label className="mb-1.5 block text-sm font-medium text-white/80">
              Min Provider Reputation <span className="text-white/40">(18 decimals, 0 = any)</span>
            </label>
            <input
              type="text"
              value={minProviderReputation}
              onChange={(e) => setMinProviderReputation(e.target.value)}
              placeholder="e.g. 0.5 (reputation threshold)"
              className="w-full rounded-lg border border-white/10 bg-prism-surface/60 px-3 py-2 font-mono text-sm text-white placeholder:text-white/30 focus:border-prism-accent focus:outline-none"
            />
            <p className="mt-1 text-[11px] text-white/40">
              Only providers with score ≥ this threshold can grab the job. Leave empty for no minimum.
            </p>
          </div>

          {error && (
            <div className="rounded-lg border border-red-500/40 bg-red-500/10 p-3 text-sm text-red-300">{error}</div>
          )}
          {stepLabel && (
            <div className="rounded-lg border border-amber-500/40 bg-amber-500/10 p-3 text-sm text-amber-300">
              {stepLabel}
            </div>
          )}
          {txHash && (
            <div className="rounded-lg border border-white/10 bg-prism-surface/60 p-3 text-xs font-mono text-white/70">
              tx: {txHash.slice(0, 18)}…{txHash.slice(-8)}
              {isConfirming && <span className="ml-2 text-amber-300">confirming…</span>}
            </div>
          )}

          {/* Submit */}
          <div className="flex items-center gap-3">
            <button
              type="submit"
              disabled={blocked || isWriting || isConfirming || !address || !agentId || !amount || !ready || !!wrongChain}
              className="inline-flex items-center gap-2 rounded-lg bg-prism-accent px-5 py-2.5 text-sm font-semibold text-white hover:bg-prism-accent/80 disabled:cursor-not-allowed disabled:opacity-40"
            >
              {isWriting || isConfirming ? (
                <>
                  <Loader2 className="h-4 w-4 animate-spin" /> Submitting…
                </>
              ) : (
                <>
                  Create & Fund Job <ArrowRight className="h-4 w-4" />
                </>
              )}
            </button>
            {blocked && (
              <span className="text-xs text-red-400">
                Blocked by TrustGate: agent score below the deny threshold.
              </span>
            )}
          </div>
        </form>
      </main>
    </div>
  );
}

export default function NewJobPage() {
  return (
    <Suspense fallback={<div className="p-8 text-white/60">Loading…</div>}>
      <NewJobBody />
    </Suspense>
  );
}

// defaultDeadline returns now+24h as a datetime-local value (YYYY-MM-DDTHH:mm)
// so the create-job form ships prefilled and is immediately usable.
function defaultDeadline(): string {
  const d = new Date(Date.now() + 24 * 3600 * 1000);
  const pad = (n: number) => String(n).padStart(2, "0");
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`;
}
