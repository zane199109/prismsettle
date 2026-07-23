"use client";

// ReorgAwareFeed — scrolling live event stream that visually flags reorgs.
// DEV-PLAN §Phase 8 任务 8.7.
//
// Reorgs are marked with an amber ↻ icon and the rolled-back event is
// rendered with a strikethrough red style, then the replacement event
// slides in green. This is the infra-credibility centerpiece visual.

import { AnimatePresence, motion } from "framer-motion";
import { useReorgFeed } from "@/hooks/useReorgFeed";
import { cn, formatAgentId, formatTime } from "@/lib/utils";

interface Props {
  chainName?: string;
  limit?: number;
  intervalMs?: number;
}

interface FeedItem {
  key: string;
  tx_hash: string;
  block_number: number;
  block_time: number;
  event_type: string;
  from: string;
  to: string;
  value: string;
  reorged: boolean;
  rolled_back?: boolean;
}

function toFeedItem(raw: {
  tx_hash: string;
  block_number: number;
  block_time: number;
  event_type: string;
  from: string;
  to: string;
  value: string;
  extra: string;
}): FeedItem {
  // The backend signals reorg via extra JSON: {"reorged": true}
  // or {"rolled_back": true} (see offchain/internal/repository).
  let reorged = false;
  let rolled_back = false;
  try {
    if (raw.extra) {
      const obj = JSON.parse(raw.extra) as { reorged?: boolean; rolled_back?: boolean };
      reorged = Boolean(obj.reorged);
      rolled_back = Boolean(obj.rolled_back);
    }
  } catch {
    // not JSON — ignore
  }
  // Event type may also include a REORG marker.
  if (raw.event_type.includes("REORG")) reorged = true;
  return {
    key: `${raw.tx_hash}-${raw.block_number}`,
    tx_hash: raw.tx_hash,
    block_number: raw.block_number,
    block_time: raw.block_time,
    event_type: raw.event_type,
    from: raw.from,
    to: raw.to,
    value: raw.value,
    reorged,
    rolled_back,
  };
}

export function ReorgAwareFeed({ chainName, limit = 30, intervalMs = 4000 }: Props) {
  const { events } = useReorgFeed({ chainName, limit, intervalMs });
  const items = events.map(toFeedItem).slice(0, limit);

  return (
    <div className="rounded-xl border border-white/10 bg-prism-surface/40 p-4">
      <div className="mb-3 flex items-center justify-between">
        <h3 className="text-sm font-medium text-white">Reorg-Aware Event Feed</h3>
        <div className="flex items-center gap-3 text-[10px] text-white/40">
          <span className="flex items-center gap-1">
            <span className="inline-block h-2 w-2 rounded-full bg-amber-400" /> reorg
          </span>
          <span className="flex items-center gap-1">
            <span className="inline-block h-2 w-2 rounded-full bg-red-400" /> rolled back
          </span>
          <span className="flex items-center gap-1">
            <span className="inline-block h-2 w-2 rounded-full bg-emerald-400" /> live
          </span>
        </div>
      </div>

      <div className="max-h-80 space-y-1 overflow-y-auto font-mono text-xs">
        <AnimatePresence initial={false}>
          {items.length === 0 ? (
            <motion.div
              key="empty"
              initial={{ opacity: 0 }}
              animate={{ opacity: 1 }}
              className="py-8 text-center text-white/30"
            >
              No events yet. Waiting for chain activity…
            </motion.div>
          ) : (
            items.map((e) => (
              <motion.div
                key={e.key}
                layout
                initial={{ opacity: 0, x: -16, backgroundColor: "rgba(16,185,129,0.25)" }}
                animate={{
                  opacity: 1,
                  x: 0,
                  backgroundColor: "rgba(0,0,0,0)",
                }}
                exit={{ opacity: 0, x: 16 }}
                transition={{ duration: 0.5 }}
                className={cn(
                  "flex items-center gap-2 rounded px-2 py-1",
                  e.rolled_back && "line-through opacity-50",
                )}
              >
                {e.rolled_back ? (
                  <span className="text-red-400">⛔</span>
                ) : e.reorged ? (
                  <span className="text-amber-400">↻</span>
                ) : (
                  <span className="text-emerald-400">•</span>
                )}
                <span className="text-white/40">#{e.block_number}</span>
                <span className="text-prism-accent">
                  {e.event_type.replace("PRISM_", "")}
                </span>
                <span className="truncate text-white/60">{formatAgentId(e.from || e.to)}</span>
                <span className="ml-auto text-white/30">{formatTime(e.block_time)}</span>
              </motion.div>
            ))
          )}
        </AnimatePresence>
      </div>
    </div>
  );
}
