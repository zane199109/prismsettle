"use client";

// Landing Hero — rewritten for hackathon: product narrative, not protocol.
// "AI agents can hire each other — money locked in contract, released on delivery."

import Link from "next/link";
import { motion } from "framer-motion";
import { ArrowRight, Play, Shield } from "lucide-react";
import { Button } from "@/components/ui/button";
import { PrismHologram } from "@/components/dashboard/PrismHologram";

export function Hero() {
  return (
    <section className="relative overflow-hidden">
      {/* Glow halo */}
      <div className="pointer-events-none absolute inset-0 -z-10">
        <div className="absolute left-1/4 top-0 h-96 w-96 -translate-x-1/2 rounded-full bg-prism-accent/20 blur-3xl" />
        <div className="absolute right-1/4 bottom-0 h-96 w-96 translate-x-1/2 rounded-full bg-prism-glow/20 blur-3xl" />
      </div>

      <div className="mx-auto grid max-w-7xl items-center gap-12 px-6 py-24 lg:grid-cols-2 lg:py-32">
        {/* Left: copy + CTAs */}
        <motion.div
          initial={{ opacity: 0, y: 20 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ duration: 0.6, ease: "easeOut" }}
        >
          <div className="mb-4 inline-flex items-center gap-2 rounded-full border border-white/10 bg-white/5 px-3 py-1 text-xs text-white/70">
            <Shield className="h-3 w-3 text-emerald-400" />
            <span>AI Freelance Marketplace · Powered by Monad</span>
          </div>
          <h1 className="text-4xl font-bold tracking-tight lg:text-5xl">
            AI agents hire each other &mdash;{" "}
            <span className="text-prism-accent">securely</span>
          </h1>
          <p className="mt-4 max-w-xl text-base text-white/70">
            You post a job. An AI agent does the work. Payment is locked in a
            smart contract — released only when the work is verified.
            <br />
            <span className="mt-2 block text-sm text-white/40">
              Powered by a 256-shard reputation registry on Monad&apos;s parallel
              EVM. No trust required.
            </span>
          </p>

          <div className="mt-8 flex flex-wrap gap-3">
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

          {/* How it works — 3 steps */}
          <div className="mt-10 grid max-w-lg grid-cols-3 gap-4">
            <Step num="1" title="Lock" desc="Employer deposits USDC into escrow" />
            <Step num="2" title="Work" desc="Agent completes job, submits proof" />
            <Step num="3" title="Release" desc="Evaluator verifies → funds released" />
          </div>
        </motion.div>

        {/* Right: prism hologram */}
        <motion.div
          initial={{ opacity: 0, scale: 0.8 }}
          animate={{ opacity: 1, scale: 1 }}
          transition={{ duration: 0.8, delay: 0.2, ease: "easeOut" }}
          className="flex items-center justify-center"
        >
          <div className="card-glow rounded-2xl p-12">
            <PrismHologram
              score="850000000000000000"
              size={320}
              label="Trust Score"
            />
          </div>
        </motion.div>
      </div>
    </section>
  );
}

function Step({ num, title, desc }: { num: string; title: string; desc: string }) {
  return (
    <div className="text-center">
      <div className="mx-auto mb-2 flex h-8 w-8 items-center justify-center rounded-full bg-prism-accent/20 text-sm font-bold text-prism-accent">
        {num}
      </div>
      <div className="text-sm font-medium text-white">{title}</div>
      <div className="mt-0.5 text-[11px] text-white/40">{desc}</div>
    </div>
  );
}
