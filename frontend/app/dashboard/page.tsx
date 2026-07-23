"use client";

// Dashboard page — the PrismSettle control center.
// Migrated from `/` to `/dashboard` so the root path can host the Landing
// page (see app/page.tsx). Layout (top-to-bottom, left-to-right):
//   ┌──────────────────────────────────────────────────┐
//   │ Header: brand + nav + wallet connect              │
//   ├──────────┬───────────────────────────────────────┤
//   │ Metrics  │ PrismHologram (rotating, agg score)   │
//   │ (3 cards)│                                       │
//   ├──────────┴───────────────────────────────────────┤
//   │ ShardHeatmap (16x16)        │ ValidationFeed     │
//   │                             │ (live ticker)      │
//   ├─────────────────────────────┴────────────────────┤
//   │ AgentLeaderboard (top 5)                          │
//   └──────────────────────────────────────────────────┘
//
// All components self-poll via their own SWR hooks, so the page itself is
// purely declarative. This keeps the layout file readable and lets each
// widget manage its own refresh cadence independently.

import { Activity, Users, Zap } from "lucide-react";
import { Providers } from "../providers";
import { PageHeader } from "@/components/PageHeader";
import { PrismHologram } from "@/components/dashboard/PrismHologram";
import { ShardHeatmap } from "@/components/dashboard/ShardHeatmap";
import { ValidationFeed } from "@/components/dashboard/ValidationFeed";
import { MetricCard } from "@/components/dashboard/MetricCard";
import { AgentLeaderboard } from "@/components/dashboard/AgentLeaderboard";
import { useShardActivity } from "@/hooks/useShardActivity";
import { useAgents } from "@/hooks/useAgents";

function DashboardBody() {
  const { data: shardData } = useShardActivity(undefined, 6000);
  const { total: agentCount } = useAgents({ size: 1, intervalMs: 15000 });

  return (
    <>
      <PageHeader />
      <main id="main" className="mx-auto max-w-7xl px-6 py-8">
        <div className="mb-6 flex flex-wrap items-center gap-3">
          <div>
            <h1 className="flex items-center gap-2 text-2xl font-bold tracking-tight">
              Dashboard
              <span className="inline-flex items-center gap-1.5 rounded-full border border-emerald-500/40 bg-emerald-500/10 px-2 py-0.5 text-[10px] font-mono uppercase tracking-wider text-emerald-400">
                <span className="relative flex h-1.5 w-1.5">
                  <span className="absolute inline-flex h-full w-full animate-ping rounded-full bg-emerald-400 opacity-75" />
                  <span className="relative inline-flex h-1.5 w-1.5 rounded-full bg-emerald-400" />
                </span>
                Live
              </span>
            </h1>
            <p className="mt-1 text-sm text-white/60">
              256-shard reputation registry for AI agents on Monad.
            </p>
          </div>
        </div>

        <section className="grid gap-6 lg:grid-cols-[1fr_auto]">
          <div className="grid gap-4 sm:grid-cols-3 lg:grid-cols-1 lg:max-w-xs">
            <MetricCard
              label="Total Validations"
              value={shardData.total}
              animateNumber
              icon={<Activity className="h-3 w-3" />}
              delta={shardData.total > 0 ? `+${shardData.total} all-time` : "—"}
            />
            <MetricCard
              label="Registered Agents"
              value={agentCount}
              animateNumber
              icon={<Users className="h-3 w-3" />}
            />
            <MetricCard
              label="Active Shards"
              value={shardData.counts.filter((c) => c > 0).length}
              animateNumber
              icon={<Zap className="h-3 w-3" />}
              delta="out of 256"
            />
          </div>

          <div className="card-glow flex items-center justify-center rounded-xl p-8">
            <PrismHologram
              score={shardData.total > 0 ? "800000000000000000" : "0"}
              size={256}
              label="Aggregate Score"
            />
          </div>
        </section>

        <section className="mt-6 grid gap-6 lg:grid-cols-2">
          <div className="card-glow rounded-xl p-5">
            <ShardHeatmap intervalMs={5000} />
          </div>
          <div className="card-glow rounded-xl p-5">
            <ValidationFeed limit={20} intervalMs={4000} />
          </div>
        </section>

        <section className="mt-6">
          <AgentLeaderboard limit={5} />
        </section>

        <footer className="mt-16 border-t border-white/10 pt-6 text-center text-xs text-white/30">
          PrismSettle · live data via{" "}
          <code className="text-white/50">/api/v1/prismsettle</code>
        </footer>
      </main>
    </>
  );
}

export default function DashboardPage() {
  return (
    <Providers>
      <DashboardBody />
    </Providers>
  );
}
