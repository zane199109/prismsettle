"use client";

// AgentLeaderboard — top-N agents by reputation score.
// Polls useAgents(size=N) and renders a compact table with grade chips.
// Clicking a row would navigate to /console/agents/[id] (Phase 9+).

import { useAgents } from "@/hooks/useAgents";
import { formatAgentId, formatScore, gradeColor, gradeFromScore } from "@/lib/utils";

interface AgentLeaderboardProps {
  chainName?: string;
  limit?: number;
}

export function AgentLeaderboard({ chainName, limit = 5 }: AgentLeaderboardProps) {
  const { agents, total, isValidating, error } = useAgents({
    chainName,
    size: limit,
    intervalMs: 10000,
  });

  return (
    <div className="rounded-xl border border-white/10 bg-prism-surface/60 p-5">
      <div className="flex items-center justify-between">
        <div className="text-xs uppercase tracking-wider text-white/40">
          Top Agents
        </div>
        <div className="text-xs text-white/40">
          {total} total
          {isValidating && (
            <span className="ml-2 inline-block h-2 w-2 animate-pulse rounded-full bg-emerald-400 align-middle" />
          )}
        </div>
      </div>

      <div className="mt-3 space-y-2">
        {agents.length === 0 && !error && (
          <div className="py-6 text-center text-xs text-white/30">
            no agents registered yet
          </div>
        )}
        {agents.map((agent, idx) => {
          const grade = gradeFromScore(agent.score);
          return (
            <div
              key={agent.agent_id}
              className="flex items-center gap-3 rounded-lg border border-white/5 bg-black/20 px-3 py-2 transition hover:border-prism-accent/40"
            >
              <span className="w-6 text-center text-sm font-mono text-white/40">
                {idx + 1}
              </span>
              <div className="flex-1 min-w-0">
                <div className="truncate font-mono text-xs text-white/80">
                  {formatAgentId(agent.agent_id)}
                </div>
                <div className="text-[10px] text-white/40">
                  {agent.endpoint || "no endpoint"}
                </div>
              </div>
              <div className="text-right">
                <div className="font-mono text-sm tabular-nums text-white">
                  {formatScore(agent.score)}
                </div>
                <div className={`text-[10px] font-semibold ${gradeColor(grade)}`}>
                  Grade {grade}
                </div>
              </div>
            </div>
          );
        })}
      </div>

      {error && (
        <div className="mt-3 text-xs text-red-400">
          leaderboard error: {error.message}
        </div>
      )}
    </div>
  );
}
