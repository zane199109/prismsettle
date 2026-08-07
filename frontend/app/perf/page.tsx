"use client";

// Performance comparison page — V0V1Comparison + ReorgAwareFeed + ShardHeatmap.
// DEV-PLAN §Phase 8 任务 8.7.


import { V0V1Comparison } from "@/components/perf/V0V1Comparison";
import { ReorgAwareFeed } from "@/components/prism/ReorgAwareFeed";
import { ShardHeatmap } from "@/components/dashboard/ShardHeatmap";

export default function PerfPage() {
  return (
    <div className="min-h-screen">
      
      <main className="mx-auto max-w-7xl px-6 py-8">
        <h1 className="text-2xl font-bold tracking-tight">Performance</h1>
        <p className="mt-1 text-sm text-white/60">
          V0 vs V1 benchmark + reorg-aware live feed + shard heatmap.
        </p>

        {/* V0V1 comparison */}
        <section className="mt-6 rounded-xl border border-white/10 bg-prism-surface/40 p-5">
          <h2 className="mb-4 text-sm font-medium text-white">
            V0 (single-shard) vs V1 (256-shard)
          </h2>
          <V0V1Comparison />
        </section>

        {/* Heatmap + reorg feed */}
        <section className="mt-6 grid gap-6 lg:grid-cols-2">
          <div className="rounded-xl border border-white/10 bg-prism-surface/40 p-5">
            <h2 className="mb-3 text-sm font-medium text-white">Shard Heatmap</h2>
            <ShardHeatmap intervalMs={5000} />
          </div>
          <ReorgAwareFeed limit={30} intervalMs={4000} />
        </section>
      </main>
    </div>
  );
}
