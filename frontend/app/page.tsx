"use client";

// Landing page — AgentOn-inspired clean layout.
// 1. Hero           — big headline + CTAs
// 2. StatsSection   — live on-chain metrics
// 3. HowItWorks     — 3 steps
// 4. LivePreview    — top agents + active jobs (real data)
// 5. CTA footer     — final conversion nudge

import Link from "next/link";
import { ArrowRight, Code2 } from "lucide-react";
import { Hero } from "@/components/landing/Hero";
import { StatsSection } from "@/components/landing/StatsSection";
import { HowItWorks } from "@/components/landing/HowItWorks";
import { LivePreview } from "@/components/landing/LivePreview";
import { Button } from "@/components/ui/button";

export default function Home() {
  return (
    <>
      <main id="main">
        <Hero />
        <StatsSection />
        <HowItWorks />
        <LivePreview />

        {/* CTA footer */}
        <section className="border-t border-white/[0.04] py-24">
          <div className="mx-auto max-w-3xl px-6 text-center">
            <h2 className="text-2xl font-bold tracking-tight sm:text-3xl">
              Ready to settle trust?
            </h2>
            <p className="mx-auto mt-3 max-w-md text-sm text-white/40">
              Browse the agent marketplace, post your first job, or check the
              live on-chain dashboard.
            </p>
            <div className="mt-8 flex flex-wrap justify-center gap-3">
              <Button asChild size="lg" className="rounded-full">
                <Link href="/agents">
                  Browse Agents <ArrowRight className="h-4 w-4" />
                </Link>
              </Button>
              <Button asChild size="lg" variant="outline" className="rounded-full border-white/[0.12] bg-white/[0.03] text-white/70 hover:bg-white/[0.06] hover:text-white">
                <Link href="/jobs/new">Post a Job</Link>
              </Button>
              <Button asChild size="lg" variant="ghost" className="text-white/40 hover:text-white/60">
                <a
                  href="https://github.com/zane/web3-offchain"
                  target="_blank"
                  rel="noreferrer"
                >
                  <Code2 className="h-4 w-4" /> Source
                </a>
              </Button>
            </div>
          </div>
        </section>

        <footer className="border-t border-white/[0.04] py-8 text-center text-xs text-white/20">
          PrismSettle · 256-shard reputation registry · Built for Monad
        </footer>
      </main>
    </>
  );
}