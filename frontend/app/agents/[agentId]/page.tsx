"use client";

// Agent detail page — reputation history chart, recent chain events,
// ShardHeatmap (shards this agent touched), and the AgentFailureCounter
// (FR-M12). DEV-PLAN §Phase 8 任务 8.3.

import { use } from "react";
import { ArrowLeft, ExternalLink } from "lucide-react";
import Link from "next/link";
import { Badge } from "@/components/ui/badge";
import { PageHeader } from "@/components/PageHeader";
import { ShardHeatmap } from "@/components/dashboard/ShardHeatmap";
import { ScoreHistoryChart } from "@/components/agent/ScoreHistoryChart";
import { AgentFailureCounterUI } from "@/components/agent/AgentFailureCounterUI";
import { AgentInvokeBox } from "@/components/agent/AgentInvokeBox";
import { ValidatorReviews } from "@/components/agent/ValidatorReviews";
import { useAgentDetail } from "@/hooks/useAgentDetail";
import { useReputationHistory } from "@/hooks/useReputationHistory";
import {
  cn,
  formatAgentId,
  formatScore,
  formatTime,
  gradeColor,
  gradeFromScore,
} from "@/lib/utils";

interface PageProps {
  params: Promise<{ agentId: string }>;
}

export default function AgentDetailPage({ params }: PageProps) {
  const { agentId } = use(params);
  return <AgentDetailBody agentId={decodeURIComponent(agentId)} />;
}

function AgentDetailBody({ agentId }: { agentId: string }) {
  const { agent, isValidating } = useAgentDetail(agentId);
  const { points } = useReputationHistory(agentId, { size: 50 });

  const grade = gradeFromScore(agent?.score ?? "0");
  const score = formatScore(agent?.score);

  // Compute shard slot (agentId & 0xFF) — 256-shard storage layout (PRD FR-M03 / UC-01).
  // BigInt handles both decimal ("1") and 0x-hex ("0x1a") agent IDs.
  let shardSlot: number | null = null;
  try {
    if (agent?.agent_id) shardSlot = Number(BigInt(agent.agent_id) & 0xFFn);
  } catch {
    shardSlot = null;
  }

  return (
    <div className="min-h-screen">
      <PageHeader />
      <main className="mx-auto max-w-7xl px-6 py-8">
        <Link
          href="/agents"
          className="mb-4 inline-flex items-center gap-1 text-sm text-white/60 hover:text-white"
        >
          <ArrowLeft className="h-4 w-4" /> Back to marketplace
        </Link>

        {isValidating && !agent ? (
          <div className="h-48 animate-pulse rounded-xl border border-white/10 bg-prism-surface/40" />
        ) : !agent ? (
          <div className="rounded-xl border border-dashed border-white/20 py-16 text-center text-sm text-white/60">
            Agent not found: <code className="text-white">{agentId}</code>
          </div>
        ) : (
          <>
            {/* Header card */}
            <section className="rounded-xl border border-white/10 bg-prism-surface/40 p-6">
              <div className="flex flex-wrap items-start justify-between gap-4">
                <div>
                  <div className="flex items-center gap-3">
                    <h1 className="font-mono text-lg font-bold text-white">
                      {formatAgentId(agent.agent_id)}
                    </h1>
                    <span
                      className={cn(
                        "rounded-md border px-2 py-0.5 font-mono text-xs font-bold",
                        gradeColor(grade),
                      )}
                    >
                      {grade}
                    </span>
                  </div>
                  <p className="mt-1 text-xs text-white/40">
                    owner {formatAgentId(agent.owner)} · registered block {agent.block_number} ·{" "}
                    {formatTime(agent.registered_at)}
                  </p>
                </div>
                <div className="text-right">
                  <div className={cn("font-mono text-3xl font-bold", gradeColor(grade))}>
                    {score}
                  </div>
                  <div className="text-xs text-white/40">reputation score</div>
                </div>
              </div>
              {agent.endpoint && (
                <a
                  href={agent.endpoint}
                  target="_blank"
                  rel="noreferrer"
                  className="mt-4 inline-flex items-center gap-1 text-sm text-prism-accent hover:underline"
                >
                  {agent.endpoint}
                  <ExternalLink className="h-3 w-3" />
                </a>
              )}
              {/* Performance labels — 256-shard slot + expected latency (FR-M03 / US-14 / UC-01). */}
              {shardSlot !== null && (
                <div className="mt-4 flex flex-wrap gap-2">
                  <Badge
                    variant="outline"
                    className="border-prism-accent/40 bg-prism-accent/5 font-mono text-[11px] text-prism-accent"
                  >
                    Shard Slot: #{shardSlot}
                  </Badge>
                  <Badge
                    variant="outline"
                    className="border-prism-glow/40 bg-prism-glow/5 font-mono text-[11px] text-prism-glow"
                  >
                    Expected Latency: &lt;1s
                  </Badge>
                </div>
              )}
              {agent.metadata && (
                <pre className="mt-4 max-h-32 overflow-auto rounded-md bg-black/30 p-3 text-xs text-white/60">
                  {agent.metadata}
                </pre>
              )}
            </section>

            {/* Charts row */}
            <section className="mt-6 grid gap-6 lg:grid-cols-2">
              <div className="rounded-xl border border-white/10 bg-prism-surface/40 p-5">
                <h2 className="mb-3 text-sm font-medium text-white">Reputation History</h2>
                <ScoreHistoryChart points={points} />
              </div>
              <div className="rounded-xl border border-white/10 bg-prism-surface/40 p-5">
                <h2 className="mb-3 text-sm font-medium text-white">Shard Activity</h2>
                <ShardHeatmap intervalMs={6000} cellSize={14} />
              </div>
            </section>

            {/* Validator reviews — P1-2 (FR-M03). Full-width below charts. */}
            <section className="mt-6">
              <ValidatorReviews agentId={agentId} limit={8} />
            </section>

            {/* Failure counter + one-click invoke (FR-M04) */}
            <section className="mt-6 grid gap-6 lg:grid-cols-2">
              <AgentFailureCounterUI agentId={agentId} />
              <AgentInvokeBox agentId={agentId} agentScore={agent.score} />
            </section>

            {/* Quick links */}
            <section className="mt-4 flex flex-wrap gap-2">
              <Link
                href={`/jobs/new?agent=${encodeURIComponent(agentId)}`}
                className="rounded-md bg-prism-accent px-3 py-1.5 text-sm font-medium text-white hover:bg-prism-accent/80"
              >
                Create Job with this agent
              </Link>
              <Link
                href="/validator"
                className="rounded-md border border-white/20 px-3 py-1.5 text-sm text-white/80 hover:bg-white/5"
              >
                Stake as validator
              </Link>
            </section>
          </>
        )}
      </main>
    </div>
  );
}
