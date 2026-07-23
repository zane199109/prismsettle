"use client";

// ProblemSection — three pain points that PrismSettle solves.
// Targets the "why now" question for hackathon judges and investors.

import { motion } from "framer-motion";
import { Unplug, Bot, GitBranch } from "lucide-react";
import { Card, CardContent } from "@/components/ui/card";

const PROBLEMS = [
  {
    icon: Unplug,
    title: "Trust is fragmented",
    body: "Agent reputation lives in isolated platforms — OpenAI, LangChain, Morpheus each keep their own scores. No portable, on-chain attestation exists.",
    accent: "text-amber-400",
  },
  {
    icon: Bot,
    title: "Sybil attacks are trivial",
    body: "Without staked skin-in-the-game, an attacker can spin up thousands of agent identities and self-validate malicious outputs. Cost of attack = zero.",
    accent: "text-red-400",
  },
  {
    icon: GitBranch,
    title: "OCC writes conflict",
    body: "Monad's optimistic concurrency control aborts ~60% of writes under high concurrency. A naive reputation contract becomes a bottleneck.",
    accent: "text-purple-400",
  },
];

export function ProblemSection() {
  return (
    <section className="border-t border-white/5 py-24">
      <div className="mx-auto max-w-7xl px-6">
        <div className="mb-12 max-w-2xl">
          <div className="mb-2 text-sm font-mono uppercase tracking-wider text-prism-accent">
            The problem
          </div>
          <h2 className="text-3xl font-bold tracking-tight sm:text-4xl">
            Agents are everywhere.
            <br />
            <span className="text-white/50">Trust is nowhere.</span>
          </h2>
        </div>

        <div className="grid gap-6 md:grid-cols-3">
          {PROBLEMS.map((p, i) => (
            <motion.div
              key={p.title}
              initial={{ opacity: 0, y: 16 }}
              whileInView={{ opacity: 1, y: 0 }}
              viewport={{ once: true, margin: "-80px" }}
              transition={{ duration: 0.4, delay: i * 0.1 }}
            >
              <Card className="h-full border-white/10 bg-prism-surface/40 hover:border-white/20 transition-colors">
                <CardContent className="p-6">
                  <p.icon className={`h-8 w-8 ${p.accent}`} />
                  <h3 className="mt-4 text-lg font-semibold text-white">
                    {p.title}
                  </h3>
                  <p className="mt-2 text-sm leading-relaxed text-white/60">
                    {p.body}
                  </p>
                </CardContent>
              </Card>
            </motion.div>
          ))}
        </div>
      </div>
    </section>
  );
}
