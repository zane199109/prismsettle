"use client";

// ShardHeatmap — 16x16 grid visualizing the 256 shards' validation activity.
// Each cell pulses when its shard receives a new validation; brightness
// scales with the validation count.
//
// The grid is the literal embodiment of PrismSettle's 256-shard design
// (SD §4.5). In the pitch, this animates as live events land — instantly
// communicating "we solved the OCC abort problem visually".
//
// Tailwind v3 doesn't ship grid-cols-16 by default, so we use inline style
// with gridTemplateColumns: repeat(16, 1fr).

import { motion } from "framer-motion";
import { useShardActivity } from "@/hooks/useShardActivity";

interface ShardHeatmapProps {
  chainName?: string;
  // Polling interval in ms. Default 5s.
  intervalMs?: number;
  // Cell size in pixels. Default 16 (256px total grid).
  cellSize?: number;
}

export function ShardHeatmap({
  chainName,
  intervalMs = 5000,
  cellSize = 16,
}: ShardHeatmapProps) {
  const { data, error, isValidating } = useShardActivity(chainName, intervalMs);

  // Max count across all shards — used to normalize intensity 0..1.
  // Fallback to 1 to avoid div-by-zero when all shards are cold.
  const maxCount = Math.max(1, ...data.counts);

  return (
    <div className="flex flex-col gap-3">
      <div className="flex items-center justify-between">
        <div className="text-xs uppercase tracking-wider text-white/40">
          Shard Activity (256)
        </div>
        <div className="flex items-center gap-2 text-xs text-white/50">
          {isValidating && (
            <span className="h-2 w-2 animate-pulse rounded-full bg-emerald-400" />
          )}
          <span>{data.total.toLocaleString()} validations</span>
        </div>
      </div>

      <div
        className="grid gap-1"
        style={{
          gridTemplateColumns: "repeat(16, 1fr)",
          width: 16 * (cellSize + 4) - 4,
        }}
      >
        {Array.from({ length: 256 }, (_, shardId) => {
          const count = data.counts[shardId];
          const intensity = Math.min(1, count / maxCount);
          const isActive = count > 0;
          return (
            <motion.div
              key={shardId}
              initial={{ opacity: 0.15 }}
              animate={{
                opacity: 0.15 + intensity * 0.85,
                scale: isActive ? [1, 1.4, 1] : 1,
              }}
              transition={{ duration: 0.4 }}
              className="aspect-square rounded-sm"
              style={{
                width: cellSize,
                height: cellSize,
                backgroundColor: `rgba(124, 58, 237, ${0.15 + intensity * 0.85})`,
                boxShadow: isActive
                  ? `0 0 ${Math.min(12, count * 2)}px rgba(124, 58, 237, 0.8)`
                  : "none",
              }}
              title={`Shard ${shardId}: ${count} validations`}
            />
          );
        })}
      </div>

      {/* Legend: cold → hot gradient bar */}
      <div className="flex items-center gap-2 text-[10px] text-white/40">
        <span>Cold</span>
        <div
          className="h-1.5 w-24 rounded-full"
          style={{
            background:
              "linear-gradient(90deg, rgba(124,58,237,0.15) 0%, rgba(124,58,237,1) 100%)",
          }}
        />
        <span>Hot</span>
        {error && (
          <span className="ml-auto text-red-400">
            feed error: {error.message}
          </span>
        )}
      </div>
    </div>
  );
}
