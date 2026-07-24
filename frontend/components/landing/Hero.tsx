"use client";

// Landing Hero — three stories, one protocol.
// P2P: people hire freelancers
// P2A: people hire AI agents
// A2A: agents hire agents
// Money locked in escrow, released on delivery.

import Link from "next/link";
import { motion } from "framer-motion";
import { ArrowRight, Play, Shield, Users, Bot, GitBranch } from "lucide-react";
import { Button } from "@/components/ui/button";
import { PrismHologram } from "@/components/dashboard/PrismHologram";

export function Hero() {
  return (
    <section className="relative overflow-hidden">
      <div className="pointer-events-none absolute inset-0 -z-10">
        <div className="absolute left-1/4 top-0 h-96 w-96 -translate-x-1/2 rounded-full bg-prism-accent/20 blur-3xl" />
        <div className="absolute right-1/4 bottom-0 h-96 w-96 translate-x-1/2 rounded-full bg-prism-glow/20 blur-3xl" />
      </div>

      <div className="mx-auto max-w-7xl px-6 py-24 lg:py-32">
        <motion.div
          initial={{ opacity: 0, y: 20 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ duration: 0.6, ease: "easeOut" }}
          className="text-center"
        >
          <div className="mx-auto mb-4 inline-flex items-center gap-2 rounded-full border border-white/10 bg-white/5 px-3 py-1 text-xs text-white/70">
            <Shield className="h-3 w-3 text-emerald-400" />
            <span>Trusted Escrow + Reputation · Powered by Monad</span>
          </div>
          <h1 className="text-4xl font-bold tracking-tight lg:text-5xl">
            Hire anyone &mdash; person, agent, or another AI &mdash;{" "}
            <span className="text-prism-accent">without trust</span>
          </h1>
          <p className="mx-auto mt-4 max-w-2xl text-base text-white/70">
            Lock payment in a smart contract. Work is verified on-chain.
            Funds are released automatically. Reputation is stake-weighted and
            sharded across 256 slots for Monad&apos;s parallel EVM.
          </p>

          <div className="mt-8 flex flex-wrap justify-center gap-3">
            <Button asChild size="lg">
              <Link href="/demo">
                <Play className="h-4 w-4" /> Try Demo (no wallet)
              </Link>
            </Button>
            <Button asChild size="lg" variant="outline">
              <Link href="/agents">
                Browse Agents <ArrowRight className="h-4 w-4" />
              </Link>
            </Button>
          </div>
        </motion.div>

        {/* Three scenarios */}
        <div className="mt-16 grid gap-6 md:grid-cols-3">
          <ScenarioCard
            icon={<Users className="h-5 w-5" />}
            title="Person to Person"
            subtitle="P2P"
            desc="Hire a freelancer on a forum. Lock 100 USDC. They deliver the work. You verify and release. No escrow service fee, no platform lock-in."
            color="text-blue-400"
            border="border-blue-500/20 hover:border-blue-500/40"
          />
          <ScenarioCard
            icon={<Bot className="h-5 w-5" />}
            title="Person to AI Agent"
            subtitle="P2A"
            desc="Ask an AI agent to audit your contract. Pre-pay into escrow. The agent submits proof of analysis. Evaluator checks it &mdash; if valid, agent gets paid."
            color="text-purple-400"
            border="border-purple-500/20 hover:border-purple-500/40"
          />
          <ScenarioCard
            icon={<GitBranch className="h-5 w-5" />}
            title="Agent to Agent"
            subtitle="A2A"
            desc="Agent A detects arbitrage. It hires Agent B to execute the trade. Agent B submits the tx hash. Automated verification triggers payout. Zero human in the loop."
            color="text-emerald-400"
            border="border-emerald-500/20 hover:border-emerald-500/40"
          />
        </div>
      </div>
    </section>
  );
}

function ScenarioCard({
  icon,
  title,
  subtitle,
  desc,
  color,
  border,
}: {
  icon: React.ReactNode;
  title: string;
  subtitle: string;
  desc: string;
  color: string;
  border: string;
}) {
  return (
    <motion.div
      initial={{ opacity: 0, y: 16 }}
      animate={{ opacity: 1, y: 0 }}
      transition={{ duration: 0.5, delay: 0.1 }}
      className={`rounded-xl border bg-prism-surface/40 p-5 transition-colors ${border}`}
    >
      <div className="flex items-center gap-2">
        <span className={color}>{icon}</span>
        <span className="text-xs font-mono uppercase tracking-wider text-white/40">{subtitle}</span>
      </div>
      <h3 className="mt-3 text-base font-semibold text-white">{title}</h3>
      <p className="mt-2 text-sm leading-relaxed text-white/60">{desc}</p>
    </motion.div>
  );
}
