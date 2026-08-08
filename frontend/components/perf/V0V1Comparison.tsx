"use client";

// V0V1Comparison — side-by-side table + grouped bar chart for V0 (single-shard)
// vs V1 (256-shard) performance. DEV-PLAN §Phase 8 任务 8.7.

import {
  Bar,
  BarChart,
  Cell,
  Legend,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from "recharts";
import { Check, AlertCircle } from "lucide-react";
import { cn } from "@/lib/utils";
import { usePerfComparison, type PerfComparison } from "@/hooks/usePerfComparison";

export function V0V1Comparison() {
  const { comparison, isValidating } = usePerfComparison();

  if (isValidating && !comparison) {
    return (
      <div className="h-64 animate-pulse rounded-xl border border-white/10 bg-prism-surface/40" />
    );
  }

  if (!comparison) {
    return (
      <div className="rounded-xl border border-dashed border-white/20 py-12 text-center text-sm text-white/40">
        Performance comparison unavailable.
      </div>
    );
  }

  const data = [
    { metric: "Abort Rate", v0: comparison.v0_abort_rate * 100, v1: comparison.v1_abort_rate * 100, unit: "%" },
    { metric: "Throughput", v0: comparison.v0_throughput, v1: comparison.v1_throughput, unit: "tx/s" },
  ];

  const improvement =
    comparison.v0_abort_rate > 0
      ? ((comparison.v0_abort_rate - comparison.v1_abort_rate) / comparison.v0_abort_rate) * 100
      : 0;

  return (
    <div className="space-y-5">
      {/* Table */}
      <div className="overflow-hidden rounded-lg border border-white/10">
        <table className="w-full text-sm">
          <thead className="bg-prism-surface/60 text-xs uppercase tracking-wider text-white/40">
            <tr>
              <th className="px-4 py-2 text-left">Metric</th>
              <th className="px-4 py-2 text-right">V0 (single-shard)</th>
              <th className="px-4 py-2 text-right">V1 (256-shard)</th>
              <th className="px-4 py-2 text-right">Change</th>
            </tr>
          </thead>
          <tbody className="font-mono">
            <tr className="border-t border-white/5">
              <td className="px-4 py-2 text-white/70">Abort Rate</td>
              <td className="px-4 py-2 text-right text-red-400">
                {(comparison.v0_abort_rate * 100).toFixed(2)}%
              </td>
              <td className="px-4 py-2 text-right text-emerald-400">
                {(comparison.v1_abort_rate * 100).toFixed(2)}%
              </td>
              <td className="px-4 py-2 text-right text-emerald-400">
                -{improvement.toFixed(1)}%
              </td>
            </tr>
            <tr className="border-t border-white/5">
              <td className="px-4 py-2 text-white/70">Throughput</td>
              <td className="px-4 py-2 text-right text-white/60">
                {comparison.v0_throughput.toFixed(1)} tx/s
              </td>
              <td className="px-4 py-2 text-right text-white">
                {comparison.v1_throughput.toFixed(1)} tx/s
              </td>
              <td className="px-4 py-2 text-right text-emerald-400">
                +{comparison.v0_throughput > 0
                  ? ((comparison.v1_throughput / comparison.v0_throughput - 1) * 100).toFixed(1)
                  : "0"}
                %
              </td>
            </tr>
          </tbody>
        </table>
      </div>

      {/* Grouped bar chart */}
      <div className="h-56">
        <ResponsiveContainer width="100%" height="100%">
          <BarChart data={data} margin={{ top: 8, right: 16, bottom: 0, left: 0 }}>
            <XAxis dataKey="metric" stroke="rgba(255,255,255,0.4)" tick={{ fontSize: 11 }} />
            <YAxis stroke="rgba(255,255,255,0.4)" tick={{ fontSize: 11 }} width={48} />
            <Tooltip
              contentStyle={{
                background: "rgba(19,19,26,0.95)",
                border: "1px solid rgba(255,255,255,0.1)",
                borderRadius: 8,
                fontSize: 12,
              }}
              formatter={(value, name) => [Number(value).toFixed(2), name]}
            />
            <Legend wrapperStyle={{ fontSize: 11 }} />
            <Bar dataKey="v0" name="V0 (single-shard)" fill="#ef4444" radius={[4, 4, 0, 0]} />
            <Bar dataKey="v1" name="V1 (256-shard)" fill="#10b981" radius={[4, 4, 0, 0]}>
              {data.map((_, i) => (
                <Cell key={i} />
              ))}
            </Bar>
          </BarChart>
        </ResponsiveContainer>
      </div>

      {/* FR-T06 verdict */}
      <div
        className={cn(
          "flex items-center gap-2 rounded-lg border p-3 text-sm",
          comparison.meets_fr_t06
            ? "border-emerald-500/40 bg-emerald-500/10 text-emerald-300"
            : "border-amber-500/40 bg-amber-500/10 text-amber-300",
        )}
      >
        {comparison.meets_fr_t06 ? (
          <Check className="h-4 w-4" />
        ) : (
          <AlertCircle className="h-4 w-4" />
        )}
        <span>
          Target: V1 abort rate &lt; 5%:{" "}
          <strong>{comparison.meets_fr_t06 ? "PASS" : "PENDING"}</strong>
          <span className="ml-2 text-xs opacity-70">(source: {comparison.source})</span>
        </span>
      </div>

      {/* Conclusion template */}
      <p className="rounded-md border border-white/10 bg-prism-surface/30 p-3 text-sm text-white/60">
        256-shard optimization reduced transaction conflict abort rate from{" "}
        <span className="font-mono text-red-400">
          {(comparison.v0_abort_rate * 100).toFixed(2)}%
        </span>{" "}
        to{" "}
        <span className="font-mono text-emerald-400">
          {(comparison.v1_abort_rate * 100).toFixed(2)}%
        </span>{" "}
        — a {improvement.toFixed(1)}% relative reduction in OCC aborts under
        500-concurrent-writer load.
      </p>
    </div>
  );
}
