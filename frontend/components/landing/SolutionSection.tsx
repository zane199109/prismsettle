"use client";

// SolutionSection — three-pillar response to ProblemSection.
// Maps 1:1 with the architecture decisions in AGENTS.md.

import { motion } from "framer-motion";
import { Grid3x3, GitMerge, Shield } from "lucide-react";
import { Card, CardContent } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";

const PILLARS = [
  {
    icon: Grid3x3,
    title: "256-shard storage",
    tag: "OCC mitigation",
    body: "Each agent's reputation lives in its own storage slot (agentId & 0xFF). 500 concurrent writes spread across shards instead of contending for one slot — abort rate drops from 60% to under 5%.",
    metric: "60% → 5%",
  },
  {
    icon: GitMerge,
    title: "Dual-path validation",
    tag: "ERC-8183 core",
    body: "Reputation flows in from two sources: Validators (source=0) stake and submit raw scores; Evaluators (source=1) and Arbitration (source=2) adjust post-job. One score, three signals.",
    metric: "3 sources",
  },
  {
    icon: Shield,
    title: "Stake-weighted scoring",
    tag: "Sybil resistance",
    body: "Validators bond MIN_STAKE = 5 ETH. Submitting a bad score risks slashing up to 30% of accumulated reputation. Cost of attack = real, not theoretical.",
    metric: "5 ETH stake",
  },
];

export function SolutionSection() {
  return (
    <section className="border-t border-white/5 bg-prism-surface/20 py-24">
      <div className="mx-auto max-w-7xl px-6">
        <div className="mb-12 max-w-2xl">
          <div className="mb-2 text-sm font-mono uppercase tracking-wider text-prism-glow">
            The solution
          </div>
          <h2 className="text-3xl font-bold tracking-tight sm:text-4xl">
            Three primitives.
            <br />
            <span className="text-white/50">One trust graph.</span>
          </h2>
        </div>

        <div className="grid gap-6 md:grid-cols-3">
          {PILLARS.map((p, i) => (
            <motion.div
              key={p.title}
              initial={{ opacity: 0, y: 16 }}
              whileInView={{ opacity: 1, y: 0 }}
              viewport={{ once: true, margin: "-80px" }}
              transition={{ duration: 0.4, delay: i * 0.1 }}
            >
              <Card className="h-full border-white/10 bg-prism-surface/60 hover:border-prism-accent/30 transition-colors">
                <CardContent className="p-6">
                  <div className="flex items-center justify-between">
                    <p.icon className="h-8 w-8 text-prism-accent" />
                    <Badge variant="outline" className="font-mono text-[10px]">
                      {p.tag}
                    </Badge>
                  </div>
                  <h3 className="mt-4 text-lg font-semibold text-white">
                    {p.title}
                  </h3>
                  <p className="mt-2 text-sm leading-relaxed text-white/60">
                    {p.body}
                  </p>
                  <div className="mt-4 font-mono text-sm font-bold text-emerald-400">
                    {p.metric}
                  </div>
                </CardContent>
              </Card>
            </motion.div>
          ))}
        </div>
      </div>
    </section>
  );
}
