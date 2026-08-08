"use client";

// JobEventTimeline — full lifecycle event stream for a job: funded → assigned
// → submitted → (rejected loop) → disputed → resolved → executed. Complements
// JobStatusTracker (4-state blocks) with every on-chain action, in order.

import { motion } from "framer-motion";
import { ExternalLink } from "lucide-react";
import type { JobTimelineItem } from "@/lib/types";
import { cn, formatTime } from "@/lib/utils";

// Display label (zh) + accent color per timeline state.
const STATE_META: Record<string, { label: string; dot: string }> = {
  Created: { label: "任务已创建", dot: "bg-white/40" },
  Funded: { label: "资金已托管", dot: "bg-prism-accent" },
  Assigned: { label: "已接单（Provider）", dot: "bg-prism-glow" },
  Submitted: { label: "已提交交付物", dot: "bg-emerald-400" },
  Rejected: { label: "已被买家打回", dot: "bg-amber-400" },
  Completed: { label: "已完成并结算", dot: "bg-emerald-500" },
  Refunded: { label: "已退款", dot: "bg-slate-400" },
  Disputed: { label: "已提起争议", dot: "bg-red-400" },
  ArbitratorSelected: { label: "仲裁方已选定", dot: "bg-red-400" },
  DisputeResolved: { label: "争议已裁定", dot: "bg-violet-400" },
  ArbitrationExecuted: { label: "仲裁结算完成", dot: "bg-violet-500" },
};

export function JobEventTimeline({
  timeline,
  chainName,
}: {
  timeline: JobTimelineItem[];
  chainName?: string;
}) {
  const ordered = [...timeline].sort(
    (a, b) => a.block_number - b.block_number || (a.timestamp || 0) - (b.timestamp || 0),
  );
  if (ordered.length === 0) {
    return (
      <div className="rounded-xl border border-white/10 bg-prism-surface/40 p-5 text-sm text-white/40">
        暂无事件记录
      </div>
    );
  }
  return (
    <div className="rounded-xl border border-white/10 bg-prism-surface/40 p-5">
      <h3 className="mb-4 text-sm font-medium text-white">完整生命周期</h3>
      <ol className="relative space-y-4 pl-5">
        <span className="absolute left-[5px] top-1 h-[calc(100%-8px)] w-px bg-white/10" />
        {ordered.map((item, i) => {
          const meta = STATE_META[item.status] ?? {
            label: item.status,
            dot: "bg-white/40",
          };
          return (
            <motion.li
              key={`${item.block_number}-${i}`}
              initial={{ opacity: 0, x: -6 }}
              animate={{ opacity: 1, x: 0 }}
              transition={{ delay: i * 0.04 }}
              className="relative"
            >
              <span
                className={cn(
                  "absolute -left-5 top-1.5 h-2.5 w-2.5 rounded-full ring-4 ring-black/40",
                  meta.dot,
                )}
              />
              <div className="flex items-center gap-2 text-sm">
                <span className="font-medium text-white">{meta.label}</span>
                <span className="text-xs text-white/30">{formatTime(item.timestamp)}</span>
              </div>
              {item.tx_hash && (
                <a
                  href={`https://testnet.monadexplorer.com/tx/${item.tx_hash}`}
                  target="_blank"
                  rel="noreferrer"
                  className="mt-0.5 inline-flex items-center gap-1 font-mono text-[11px] text-prism-accent/60 hover:text-prism-accent"
                >
                  {item.tx_hash.slice(0, 18)}… <ExternalLink className="h-3 w-3" />
                </a>
              )}
            </motion.li>
          );
        })}
      </ol>
    </div>
  );
}
