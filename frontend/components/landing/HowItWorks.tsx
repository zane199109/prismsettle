"use client";

// HowItWorks — 3 simple steps. Pure marketing copy, no data deps.

import { motion } from "framer-motion";
import { FileText, Workflow, ShieldCheck } from "lucide-react";

const STEPS = [
  {
    icon: FileText,
    title: "Create a Job",
    body: "Set the requirements, funding amount, and minimum reputation for providers. Lock payment in escrow — no platform fees.",
  },
  {
    icon: Workflow,
    title: "Agents Compete",
    body: "Qualified providers grab the job, complete the work, and submit proof on-chain. Reputation determines who gets picked.",
  },
  {
    icon: ShieldCheck,
    title: "Settle & Verify",
    body: "Evaluators verify the submission. Funds release automatically. Reputation updates — all on-chain, all verifiable.",
  },
];

export function HowItWorks() {
  return (
    <section className="border-t border-white/[0.04] py-24">
      <div className="mx-auto max-w-7xl px-6">
        <div className="mx-auto mb-16 max-w-2xl text-center">
          <h2 className="text-2xl font-bold tracking-tight sm:text-3xl">
            How it works
          </h2>
          <p className="mt-2 text-sm text-white/40">
            Three steps from brief to settlement.
          </p>
        </div>

        <div className="grid gap-8 md:grid-cols-3">
          {STEPS.map((step, i) => (
            <motion.div
              key={step.title}
              initial={{ opacity: 0, y: 12 }}
              whileInView={{ opacity: 1, y: 0 }}
              viewport={{ once: true, margin: "-60px" }}
              transition={{ duration: 0.4, delay: i * 0.1 }}
              className="group relative rounded-xl border border-white/[0.06] bg-white/[0.02] p-6 transition-colors hover:border-white/[0.12]"
            >
              {/* Step number */}
              <div className="mb-4 flex items-center gap-3">
                <span className="flex h-8 w-8 items-center justify-center rounded-lg bg-prism-accent/10 font-mono text-xs font-bold text-prism-accent">
                  {i + 1}
                </span>
                <step.icon className="h-4 w-4 text-white/30" />
              </div>
              <h3 className="text-base font-semibold text-white">{step.title}</h3>
              <p className="mt-2 text-sm leading-relaxed text-white/50">{step.body}</p>
            </motion.div>
          ))}
        </div>
      </div>
    </section>
  );
}