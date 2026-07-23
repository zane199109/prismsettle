"use client";

// TechStackSection — proof of engineering depth. Lists the stack and key
// benchmarks that justify the architecture decisions. The 60%→5% number
// is the headline metric for the OCC mitigation story.

import { motion } from "framer-motion";
import { Cpu, Boxes, Zap } from "lucide-react";
import { Card, CardContent } from "@/components/ui/card";

const STACK = [
  { name: "Solidity 0.8.24", role: "Contracts · Foundry" },
  { name: "Go 1.25", role: "Offchain listener + service" },
  { name: "Monad", role: "L1 EVM chain · OCC" },
  { name: "Next.js 15", role: "Frontend · App Router" },
  { name: "wagmi v2 + viem", role: "Wallet + contract reads" },
  { name: "shadcn/ui", role: "Component primitives" },
];

const BENCHMARKS = [
  {
    icon: Zap,
    label: "Concurrent submitValidation",
    value: "500",
    unit: "txns",
    note: "sustained load in V0 vs V1 benchmark",
  },
  {
    icon: Boxes,
    label: "Write conflict abort rate",
    value: "60% → 5%",
    unit: "",
    note: "single-slot vs 256-shard on Monad OCC",
  },
  {
    icon: Cpu,
    label: "End-to-end validation latency",
    value: "<5",
    unit: "s",
    note: "rule check + LLM semantic scoring",
  },
];

export function TechStackSection() {
  return (
    <section className="border-t border-white/5 bg-prism-surface/20 py-24">
      <div className="mx-auto max-w-7xl px-6">
        <div className="mb-12 max-w-2xl">
          <div className="mb-2 text-sm font-mono uppercase tracking-wider text-prism-glow">
            Engineering
          </div>
          <h2 className="text-3xl font-bold tracking-tight sm:text-4xl">
            Built for scale, audited for trust
          </h2>
        </div>

        {/* Benchmarks */}
        <div className="mb-12 grid gap-6 md:grid-cols-3">
          {BENCHMARKS.map((b, i) => (
            <motion.div
              key={b.label}
              initial={{ opacity: 0, y: 16 }}
              whileInView={{ opacity: 1, y: 0 }}
              viewport={{ once: true, margin: "-80px" }}
              transition={{ duration: 0.4, delay: i * 0.1 }}
            >
              <Card className="border-white/10 bg-prism-surface/60">
                <CardContent className="p-6">
                  <b.icon className="h-6 w-6 text-prism-glow" />
                  <div className="mt-4 flex items-baseline gap-2">
                    <span className="font-mono text-3xl font-bold text-white">
                      {b.value}
                    </span>
                    {b.unit && (
                      <span className="text-sm text-white/50">{b.unit}</span>
                    )}
                  </div>
                  <div className="mt-1 text-sm text-white/70">{b.label}</div>
                  <div className="mt-1 text-xs text-white/40">{b.note}</div>
                </CardContent>
              </Card>
            </motion.div>
          ))}
        </div>

        {/* Stack list */}
        <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
          {STACK.map((s, i) => (
            <motion.div
              key={s.name}
              initial={{ opacity: 0 }}
              whileInView={{ opacity: 1 }}
              viewport={{ once: true }}
              transition={{ duration: 0.3, delay: i * 0.04 }}
              className="flex items-center justify-between rounded-lg border border-white/10 bg-prism-surface/40 px-4 py-3"
            >
              <span className="font-mono text-sm text-white">{s.name}</span>
              <span className="text-xs text-white/50">{s.role}</span>
            </motion.div>
          ))}
        </div>
      </div>
    </section>
  );
}
