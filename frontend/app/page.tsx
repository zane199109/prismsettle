"use client";

// Landing page — the front door of PrismSettle.
//
// Structure (top to bottom):
//   1. Hero             — value prop + CTAs + prism hologram
//   2. ProblemSection   — 3 pain points (trust fragmentation / Sybil / OCC)
//   3. SolutionSection  — 3 pillars (256-shard / dual-path / stake-weighted)
//   4. LivePreview      — Top 3 Agents + Active 3 Jobs (real on-chain data)
//   5. TechStackSection — benchmarks + stack
//   6. CTA footer       — final conversion nudge
//
// The dashboard moves to /dashboard so this page can focus on the narrative.

import Link from "next/link";
import { ArrowRight, Code2 } from "lucide-react";
import { Providers } from "./providers";
import { PageHeader } from "@/components/PageHeader";
import { Hero } from "@/components/landing/Hero";
import { ProblemSection } from "@/components/landing/ProblemSection";
import { SolutionSection } from "@/components/landing/SolutionSection";
import { LivePreview } from "@/components/landing/LivePreview";
import { TechStackSection } from "@/components/landing/TechStackSection";
import { Button } from "@/components/ui/button";

function LandingBody() {
  return (
    <>
      <PageHeader />
      <main id="main">
        <Hero />
        <ProblemSection />
        <SolutionSection />
        <LivePreview />
        <TechStackSection />

        {/* CTA footer */}
        <section className="border-t border-white/5 py-24">
          <div className="mx-auto max-w-4xl px-6 text-center">
            <h2 className="text-3xl font-bold tracking-tight sm:text-4xl">
              Ready to settle trust?
            </h2>
            <p className="mx-auto mt-4 max-w-xl text-white/60">
              Browse the agent marketplace, post your first job, or stake ETH
              to become a validator. Everything is on-chain, everything is
              verifiable.
            </p>
            <div className="mt-8 flex flex-wrap justify-center gap-3">
              <Button asChild size="lg">
                <Link href="/agents">
                  Browse Agents <ArrowRight className="h-4 w-4" />
                </Link>
              </Button>
              <Button asChild size="lg" variant="outline">
                <Link href="/jobs/new">Post a Job</Link>
              </Button>
              <Button asChild size="lg" variant="ghost">
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

        <footer className="border-t border-white/10 py-8 text-center text-xs text-white/30">
          PrismSettle · Built for Monad · 256-shard reputation registry
        </footer>
      </main>
    </>
  );
}

export default function Home() {
  return (
    <Providers>
      <LandingBody />
    </Providers>
  );
}
