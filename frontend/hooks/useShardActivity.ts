"use client";

// useShardActivity — polls GET /shards/activity and transforms the sparse
// response (only shards with activity) into a dense 256-element array indexed
// by shard_id. This shape is what ShardHeatmap expects.
//
// SD §4.5: PrismSettle uses 256 shards to mitigate Monad's OCC abort rate.
// The heatmap visualizes write hotspots — when a cell pulses, that shard
// just received a validation. The grid stays mostly dark in healthy state
// (writes evenly distributed); bright clusters would indicate hot shards.
//
// Mock fallback: when the backend is unreachable or returns an empty array,
// the hook falls back to getLiveShardActivity() — a synthetic shard set with
// dynamic timestamps so the heatmap visibly pulses on each poll.

import { useMemo } from "react";
import { usePoll } from "./usePoll";
import { getShardActivity } from "@/lib/prismsettle";
import { getLiveShardActivity } from "@/lib/mock-data";
import type { ShardActivity } from "@/lib/types";

export interface ShardHeatmapData {
  // Dense 256-element array; index = shard_id, value = validation_count.
  counts: number[];
  // Last active timestamp per shard (0 if never active).
  lastActive: number[];
  // Total validations across all shards (sum).
  total: number;
}

const EMPTY: ShardHeatmapData = {
  counts: new Array(256).fill(0),
  lastActive: new Array(256).fill(0),
  total: 0,
};

export function useShardActivity(
  chainName?: string,
  intervalMs = 5000,
) {
  const { data, error, isValidating } = usePoll<ShardActivity[]>(
    chainName !== null ? `shard-activity:${chainName ?? "all"}` : null,
    async () => {
      try {
        const res = await getShardActivity(chainName);
        if (res && res.length > 0) return res;
        return getLiveShardActivity();
      } catch {
        return getLiveShardActivity();
      }
    },
    { intervalMs, pauseWhenHidden: true },
  );

  // Memoize the dense-array transform so consumers don't re-render on every
  // poll unless the underlying data actually changed.
  const transformed = useMemo<ShardHeatmapData>(() => {
    if (!data || data.length === 0) return EMPTY;
    const counts = new Array(256).fill(0);
    const lastActive = new Array(256).fill(0);
    let total = 0;
    for (const row of data) {
      const idx = row.shard_id;
      if (idx < 0 || idx >= 256) continue; // defensive: skip out-of-range
      counts[idx] = row.validation_count;
      lastActive[idx] = row.last_active_at;
      total += row.validation_count;
    }
    return { counts, lastActive, total };
  }, [data]);

  return {
    data: transformed,
    raw: data,
    error,
    isValidating,
  };
}
