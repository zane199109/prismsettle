"use client";

// ScoreHistoryChart — the agent's reputation trend as a line chart.
// ONLY post-aggregation ("after") values are plotted — raw rating inputs
// never appear, so the curve never alternates between the rating and the
// aggregated score. Every point is bound to the job whose rating triggered
// that aggregation; the per-job RATING itself is browsable in the
// Reputation Changes list below.

import {
  CartesianGrid,
  Line,
  LineChart,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from "recharts";
import { formatAgentId, formatTime } from "@/lib/utils";
import type { ReputationPoint } from "@/hooks/useReputationHistory";

interface Props {
  points: ReputationPoint[];
  height?: number;
}

interface Row {
  ts: number;
  score: number;
  job_id: string;
  jobShort: string;
}

export function ScoreHistoryChart({ points, height = 240 }: Props) {
  // Walk events in time order: remember the job of the latest rating, then
  // emit one point per aggregated event carrying that job (the rating that
  // triggered the aggregation).
  const data: Row[] = [];
  let lastJob = "";
  const sorted = [...points].sort((a, b) => a.timestamp - b.timestamp);
  for (const p of sorted) {
    if (p.event_type === "PRISM_VALIDATION_SUBMITTED") {
      if (p.job_id) lastJob = p.job_id;
    } else if (p.event_type === "PRISM_AGGREGATED") {
      data.push({
        ts: p.timestamp,
        score: Number(BigInt(p.score || "0")) / 1e18,
        job_id: lastJob,
        jobShort: lastJob ? formatAgentId(lastJob) : "—",
      });
    }
  }

  if (data.length === 0) {
    return (
      <div
        className="flex items-center justify-center rounded-lg border border-white/10 bg-prism-surface/30 text-sm text-white/40"
        style={{ height }}
      >
        No score history yet — the curve appears after a job is completed and aggregated.
      </div>
    );
  }

  return (
    <div style={{ height }}>
      <ResponsiveContainer width="100%" height="100%">
        <LineChart data={data} margin={{ top: 8, right: 16, bottom: 0, left: 0 }}>
          <CartesianGrid stroke="rgba(255,255,255,0.06)" strokeDasharray="3 3" />
          <XAxis
            dataKey="ts"
            tickFormatter={(ts) => formatTime(Number(ts))}
            stroke="rgba(255,255,255,0.2)"
            fontSize={10}
            minTickGap={40}
          />
          <YAxis
            domain={[0, 1]}
            tickFormatter={(v) => Number(v).toFixed(1)}
            stroke="rgba(255,255,255,0.2)"
            fontSize={10}
            width={32}
          />
          <Tooltip
            cursor={{ stroke: "rgba(34,211,238,0.3)", strokeDasharray: "3 3" }}
            content={({ active, payload }) => {
              if (!active || !payload?.length) return null;
              const r = payload[0].payload as Row;
              return (
                <div className="pointer-events-none rounded-md border border-white/10 bg-black/85 px-3 py-2 text-xs">
                  <div className="text-white/60">{formatTime(r.ts)}</div>
                  <div className="mt-1 font-mono font-bold text-prism-accent">
                    {r.score.toFixed(4)}
                  </div>
                  <div className="mt-0.5 font-mono text-[10px] text-white/40">
                    {r.job_id ? `job ${r.jobShort}` : "周期聚合"}
                  </div>
                </div>
              );
            }}
          />
          <Line
            type="monotone"
            dataKey="score"
            stroke="#22d3ee"
            strokeWidth={2}
            dot={{ r: 3.5, fill: "#22d3ee", strokeWidth: 0 }}
            activeDot={{ r: 5, fill: "#22d3ee", stroke: "#0e7490", strokeWidth: 2 }}
          />
        </LineChart>
      </ResponsiveContainer>
    </div>
  );
}
