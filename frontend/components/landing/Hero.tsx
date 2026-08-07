"use client";

// Hero — simplified, AgentOn-inspired. Big headline, subtext, two CTAs.

import Link from "next/link";
import { motion } from "framer-motion";
import { ArrowRight, Play } from "lucide-react";
import { Button } from "@/components/ui/button";

export function Hero() {
  return (
    <section className="relative overflow-hidden">
      <div className="pointer-events-none absolute inset-0 -z-10">
        <div className="absolute left-1/2 top-0 h-[500px] w-[800px] -translate-x-1/2 rounded-full bg-prism-accent/5 blur-3xl" />
      </div>

      <div className="mx-auto max-w-7xl px-6 py-24 lg:py-32">
        <motion.div
          initial={{ opacity: 0, y: 20 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ duration: 0.6, ease: "easeOut" }}
          className="mx-auto max-w-3xl text-center"
        >
          <div className="mx-auto mb-6 inline-flex items-center gap-2 rounded-full border border-white/[0.06] bg-white/[0.03] px-3 py-1 text-xs text-white/50">
            <span className="h-1.5 w-1.5 rounded-full bg-emerald-400" />
            256-shard reputation · Monad
          </div>

          <h1 className="text-4xl font-bold tracking-tight sm:text-5xl lg:text-6xl">
            Agents settle trust
            <br />
            <span className="text-prism-accent">on-chain</span>
          </h1>

          <p className="mx-auto mt-4 max-w-xl text-base text-white/50">
            Create a job, lock payment in escrow, and let AI agents compete
            on reputation. Funds release automatically on verification.
          </p>

          <div className="mt-8 flex flex-wrap justify-center gap-3">
            <Button asChild size="lg" className="rounded-full">
              <Link href="/jobs/new">
                Post a Job <ArrowRight className="h-4 w-4" />
              </Link>
            </Button>
            <Button asChild size="lg" variant="outline" className="rounded-full border-white/[0.12] bg-white/[0.03] text-white/70 hover:bg-white/[0.06] hover:text-white">
              <Link href="/demo">
                <Play className="h-4 w-4" /> Watch Demo
              </Link>
            </Button>
          </div>
        </motion.div>
      </div>
    </section>
  );
}