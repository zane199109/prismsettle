"use client";

// ScoreHistoryChart — Recharts multi-line chart of an agent's reputation
// score over time, colored by source per FR-M09:
//   - Validator   (source=0) → blue   (#3b82f6)
//   - Evaluator   (source=1) → purple (#7c3aed)  [Job path]
//   - Arbitration (source=2) → red    (#ef4444)
//
// DEV-PLAN §Phase 8 任务 8.3. Source coloring added in the UX-fullness pass.

import {
  CartesianGrid,
  Legend,
  Line,
  LineChart,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from "recharts";
import type { ReputationPoint, ReputationSource } from "@/hooks/useReputationHistory";
import { formatBigInt, formatTime } from "@/lib/utils";

interface Props {
  points: ReputationPoint[];
  height?: number;
}

const SOURCE_SERIES: { source: ReputationSource; label: string; color: string }[] = [
  { source: 0, label: "Validator", color: "#3b82f6" },
  { source: 1, label: "Job / Evaluator", color: "#7c3aed" },
  { source: 2, label: "Arbitration", color: "#ef4444" },
];

export function ScoreHistoryChart({ points, height = 240 }: Props) {
  if (points.length === 0) {
    return (
      <div
        className="flex items-center justify-center rounded-lg border border-white/10 bg-prism-surface/30 text-sm text-white/40"
        style={{ height }}
      >
        No reputation history yet.
      </div>
    );
  }

  // Build a unified timeline indexed by timestamp. Each timestamp gets up to
  // three score fields (one per source) so the three Lines share an X axis.
  // Reverse so oldest is leftmost, newest is rightmost.
  const sorted = [...points].sort((a, b) => a.timestamp - b.timestamp);
  const data = sorted.map((p) => {
    const row: Record<string, number | string> = {
      ts: p.timestamp,
      tx: p.tx_hash,
    };
    for (const s of SOURCE_SERIES) {
      row[s.label] = p.source === s.source ? formatBigInt(p.score, 18) : NaN;
    }
    return row;
  });

  return (
    <div style={{ height }}>
      <ResponsiveContainer width="100%" height="100%">
        <LineChart data={data} margin={{ top: 8, right: 16, bottom: 0, left: 0 }}>
          <CartesianGrid stroke="rgba(255,255,255,0.06)" strokeDasharray="3 3" />
          <XAxis
            dataKey="ts"
            tickFormatter={(ts) => formatTime(Number(ts))}
            stroke="rgba(255,255,255,0.4)"
            tick={{ fontSize: 11 }}
          />
          <YAxis
            domain={[0, 1]}
            stroke="rgba(255,255,255,0.4)"
            tick={{ fontSize: 11 }}
            width={36}
          />
          <Tooltip
            contentStyle={{
              background: "rgba(19,19,26,0.95)",
              border: "1px solid rgba(255,255,255,0.1)",
              borderRadius: 8,
              fontSize: 12,
            }}
            labelFormatter={(ts) => formatTime(Number(ts))}
            formatter={(value, name) => [
              Number.isNaN(value) ? "—" : Number(value).toFixed(4),
              name,
            ]}
          />
          <Legend
            wrapperStyle={{ fontSize: 11 }}
            iconType="circle"
          />
          {SOURCE_SERIES.map((s) => (
            <Line
              key={s.source}
              type="monotone"
              dataKey={s.label}
              stroke={s.color}
              strokeWidth={2}
              dot={{ r: 2, fill: s.color }}
              activeDot={{ r: 4 }}
              isAnimationActive
              connectNulls
            />
          ))}
        </LineChart>
      </ResponsiveContainer>
    </div>
  );
}
