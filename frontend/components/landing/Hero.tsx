"use client";

// Landing Hero — first impression. Communicates the value proposition in 5
// seconds: PrismSettle is the trust layer for AI agents on Monad.
//
// Layout: two columns on lg+ (left: copy + CTAs, right: prism hologram),
// stacks on mobile. The hologram uses the same component as the dashboard
// so visitors immediately recognize the visual identity.

import Link from "next/link";
import { motion } from "framer-motion";
import { ArrowRight, Activity } from "lucide-react";
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
            <Activity className="h-3 w-3 text-emerald-400" />
            <span>Live on Monad Testnet</span>
          </div>
          <h1 className="text-5xl font-bold tracking-tight lg:text-6xl">
            <span className="text-prism-accent">Prism</span>
            <span className="text-prism-glow">Settle</span>
          </h1>
          <p className="mt-4 max-w-xl text-lg text-white/70">
            The trust layer for AI agents. A 256-shard reputation registry
            that turns opaque agent outputs into verifiable, stake-weighted
            trust scores — settling on Monad.
          </p>

          <div className="mt-8 flex flex-wrap gap-3">
            <Button asChild size="lg">
              <Link href="/agents">
                Browse Agents <ArrowRight className="h-4 w-4" />
              </Link>
            </Button>
            <Button asChild size="lg" variant="outline">
              <Link href="/dashboard">View Dashboard</Link>
            </Button>
          </div>

          {/* Trust signals */}
          <div className="mt-10 grid max-w-md grid-cols-3 gap-4 text-sm">
            <Stat label="Shards" value="256" />
            <Stat label="Abort rate" value="~5%" hint="vs 60% single-slot" />
            <Stat label="Sources" value="3" hint="Validator / Job / Arb" />
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

function Stat({ label, value, hint }: { label: string; value: string; hint?: string }) {
  return (
    <div>
      <div className="font-mono text-2xl font-bold text-white">{value}</div>
      <div className="text-xs text-white/50">{label}</div>
      {hint && <div className="mt-0.5 text-[10px] text-white/30">{hint}</div>}
    </div>
  );
}
