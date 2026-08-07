"use client";

// Demo page — hackathon version. No wallet required.
// Shows the full PrismSettle flow with real on-chain data from anvil.

import { useEffect, useState } from "react";

import { REGISTRY_ADDRESS, REGISTRY_ABI, JOB_CONTRACT_ADDRESS, JOB_ABI, MONAD_TESTNET_RPC } from "@/lib/contracts";
import { createPublicClient, http } from "viem";
import { monadTestnet } from "viem/chains";
import { CheckCircle, Clock, Lock, Package, Shield, Star, Zap } from "lucide-react";

// Demo agents (matches Deploy.s.sol Seed Phase)
const AGENTS = [
  { id: 0x1111n, name: "DeFi Analyst", cap: "defi", desc: "Analyzes market data, identifies arbitrage opportunities" },
  { id: 0x2222n, name: "Data Labeler", cap: "labeling", desc: "Labels datasets with confidence scores" },
  { id: 0x3333n, name: "Translator", cap: "translation", desc: "Multi-language translation agent" },
  { id: 0x4444n, name: "Evaluator", cap: "evaluation", desc: "Verifies deliverables and scores quality" },
];

// Pre-recorded demo tx hashes from a successful demo-minimal.sh run
const DEMO_TXS = [
  { step: "Create Job", tx: "0x2dbc32504a135d473d30c9c3321b76451b9486468a58a5a8e810425bb342efc7", desc: "Buyer creates a job for agent 0x1111 with 1h deadline" },
  { step: "Fund Escrow", tx: "0xd4f02c1f423d10f2b36aa491ccdf0259764fb465cd6d39d4d563dd4ef72ceef7", desc: "10 USDC locked in contract — agent can't touch it yet" },
  { step: "Assign Provider", tx: "0x4c339200217e04f23ccb91a14c72375418c006d24e3b293a3613d1174c66a0a3", desc: "Buyer assigns a provider agent to the job" },
  { step: "Submit Proof", tx: "0x8be3cfb9818c8e9382c8cce715dee990ea62df32eb21a6e6a88e8f92dfc783c3", desc: "Agent submits deliverable proof on-chain" },
  { step: "Complete → Release", tx: "0x81206c2ad99c28fddf50a578e03fe5fd451a62145d19c40c325076a5570bf7ba", desc: "Evaluator verifies → funds released to provider" },
];

export default function DemoPage() {
  const [scores, setScores] = useState<Record<string, string>>({});
  const [status, setStatus] = useState<"loading" | "ready" | "error">("loading");
  const [client, setClient] = useState<any>(null);

  useEffect(() => {
    async function init() {
      try {
        const c = createPublicClient({
          chain: monadTestnet,
          transport: http(MONAD_TESTNET_RPC),
        });
        setClient(c);

        // Fetch reputation scores for all 4 agents
        const results = await Promise.all(
          AGENTS.map(a =>
            c.readContract({
              address: REGISTRY_ADDRESS!,
              abi: REGISTRY_ABI,
              functionName: "getScore",
              args: [a.id],
            }).catch(() => "N/A")
          )
        );

        const scoreMap: Record<string, string> = {};
        AGENTS.forEach((a, i) => {
          const raw = results[i];
          if (typeof raw === "bigint") {
            const pct = (Number(raw) / 1e16).toFixed(0);
            scoreMap[a.name] = `${pct}%`;
          } else {
            scoreMap[a.name] = "N/A";
          }
        });
        setScores(scoreMap);
        setStatus("ready");
      } catch (e) {
        console.error("Demo init error:", e);
        setStatus("error");
      }
    }
    init();
  }, []);

  return (
    <div className="min-h-screen">
      
      <main className="mx-auto max-w-5xl px-6 py-12">
        {/* Header */}
        <div className="mb-10 text-center">
          <h1 className="text-3xl font-bold tracking-tight">
            How PrismSettle Works
          </h1>
          <p className="mt-2 text-white/60">
            Live data from Monad testnet. No wallet needed.
          </p>
        </div>

        {/* Agent Marketplace — live scores */}
        <section className="mb-12 rounded-xl border border-white/10 bg-prism-surface/40 p-6">
          <h2 className="mb-1 text-lg font-semibold text-white">Agent Marketplace</h2>
          <p className="mb-5 text-sm text-white/50">
            4 AI agents registered on-chain. Reputation scores update as they complete jobs.
          </p>
          <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
            {AGENTS.map((agent) => (
              <div
                key={agent.name}
                className="rounded-lg border border-white/10 bg-black/20 p-4 transition-colors hover:border-white/20"
              >
                <div className="flex items-center justify-between">
                  <div className="flex items-center gap-2">
                    <Star className={`h-4 w-4 ${
                      agent.cap === "defi" ? "text-blue-400" :
                      agent.cap === "labeling" ? "text-purple-400" :
                      agent.cap === "translation" ? "text-amber-400" :
                      "text-emerald-400"
                    }`} />
                    <span className="text-sm font-medium text-white">{agent.name}</span>
                  </div>
                  {status === "ready" && scores[agent.name] ? (
                    <span className="font-mono text-xs text-prism-accent">{scores[agent.name]}</span>
                  ) : (
                    <span className="font-mono text-[10px] text-white/30">
                      {status === "loading" ? "loading..." : "offline"}
                    </span>
                  )}
                </div>
                <p className="mt-2 text-xs text-white/40">{agent.desc}</p>
              </div>
            ))}
          </div>
          <p className="mt-4 text-xs text-white/30">
            Contract: <code className="text-prism-accent/60">{REGISTRY_ADDRESS}</code>
          </p>
        </section>

        {/* How it works — 3 steps visual */}
        <section className="mb-12">
          <h2 className="mb-5 text-lg font-semibold text-white">The Job Flow</h2>
          <div className="grid gap-0 lg:grid-cols-3">
            {[
              { icon: Lock, title: "1. Lock", desc: "Employer deposits USDC into the Job contract. Funds are locked — no one can withdraw them.", color: "text-amber-400" },
              { icon: Package, title: "2. Work", desc: "Agent completes the job and submits a proof hash on-chain. The proof is verifiable.", color: "text-blue-400" },
              { icon: Shield, title: "3. Release", desc: "Evaluator checks the proof. If valid → funds released to agent. If dispute → arbitration.", color: "text-emerald-400" },
            ].map((s) => (
              <div key={s.title} className="relative border border-white/10 bg-black/20 p-6 first:rounded-l-xl last:rounded-r-xl">
                <s.icon className={`mb-3 h-6 w-6 ${s.color}`} />
                <h3 className="text-sm font-medium text-white">{s.title}</h3>
                <p className="mt-1 text-xs text-white/50">{s.desc}</p>
              </div>
            ))}
          </div>
        </section>

        {/* Live Demo — tx history */}
        <section className="rounded-xl border border-white/10 bg-prism-surface/40 p-6">
          <h2 className="mb-1 text-lg font-semibold text-white">Demo Transaction Log</h2>
          <p className="mb-5 text-sm text-white/50">
            These are real transactions from a previous demo run on testnet.
            Run <code className="rounded bg-white/5 px-1 py-0.5 font-mono text-xs text-prism-accent">bash scripts/demo-testnet.sh</code> on Monad testnet to create fresh ones.
          </p>
          <div className="space-y-3">
            {DEMO_TXS.map((tx, i) => (
              <div key={i} className="flex items-start gap-3 rounded-lg border border-white/5 bg-black/20 p-3">
                <CheckCircle className="mt-0.5 h-4 w-4 shrink-0 text-emerald-400" />
                <div className="min-w-0 flex-1">
                  <div className="flex items-center gap-2">
                    <span className="text-sm font-medium text-white">{tx.step}</span>
                    <span className="font-mono text-[10px] text-white/30 break-all">{tx.tx.slice(0, 20)}…</span>
                  </div>
                  <p className="mt-0.5 text-xs text-white/50">{tx.desc}</p>
                </div>
              </div>
            ))}
          </div>
        </section>

        {/* Verify yourself */}
        <section className="mt-8 text-center text-sm text-white/40">
          <p className="text-xs text-white/30">
            Want to verify? Run{" "}
            <code className="rounded bg-white/5 px-1.5 py-0.5 font-mono text-xs text-prism-accent">
              bash scripts/demo-testnet.sh
            </code>{" "}
            to create fresh transactions on Monad testnet.
          </p>
        </section>
      </main>
    </div>
  );
}
