"use client";

// JobStatusTracker — ERC-8183 four-state display + Hook arbitration block.
// DEV-PLAN §Phase 8 任务 8.5a.
//
// Mapping (per SD §613):
//   ERC-8183 Open       ← contract Created (just created, not funded)
//   ERC-8183 Funded     ← contract Funded (Assigned merges into Funded for display)
//   ERC-8183 Submitted  ← contract Submitted
//   ERC-8183 Terminal   ← contract Completed / Refunded (sub-label distinguishes)
//
// Hook arbitration states (Disputed / DisputeResolved) are NOT in ERC-8183's
// 4-state enum — they get a parallel block, never merged into Terminal.

import { motion } from "framer-motion";
import type { JobTimelineItem } from "@/lib/types";
import { cn, formatTime } from "@/lib/utils";

type ContractState =
  | "Created"
  | "Funded"
  | "Assigned"
  | "Submitted"
  | "Completed"
  | "Refunded"
  | "Disputed"
  | "DisputeResolved";

interface Props {
  current: ContractState | string;
  timeline: JobTimelineItem[];
  /** Live state from a running demo session (lowercase) — overrides `current` */
  liveState?: string;
  /** Number of rejects seen in the live demo (shown next to Submitted) */
  rejectCount?: number;
}

// demo session states → contract-style states.
function fromDemoState(s: string): ContractState | string {
  switch (s) {
    case "created":
      return "Created";
    case "funded":
      return "Funded";
    case "assigned":
      return "Assigned";
    case "submitted":
    case "rejected":
      return "Submitted";
    case "disputed":
      return "Disputed";
    case "resolved":
      return "DisputeResolved";
    case "executed":
      return "Completed";
    default:
      return s;
  }
}

const ERC8183_STATES = ["Open", "Funded", "Submitted", "Terminal"] as const;
type Erc8183State = (typeof ERC8183_STATES)[number];

function toErc8183(state: string): Erc8183State {
  switch (state) {
    case "Created":
      return "Open";
    case "Funded":
    case "Assigned":
      return "Funded";
    case "Submitted":
      return "Submitted";
    case "Completed":
    case "Refunded":
      return "Terminal";
    default:
      // Disputed / DisputeResolved — handled separately, not in 4-state enum.
      return "Terminal";
  }
}

function isArbitration(state: string): boolean {
  return state === "Disputed" || state === "DisputeResolved";
}

export function JobStatusTracker({ current, timeline, liveState, rejectCount }: Props) {
  const effective = liveState ? fromDemoState(liveState) : current;
  const currentErc = toErc8183(effective);
  const currentIndex = ERC8183_STATES.indexOf(currentErc);
  // Find timestamp when we entered current state from timeline.
  const enteredAt = timeline.find((t) => t.status === effective)?.timestamp;

  // Arbitration state (parallel block, not part of 4-state enum).
  const arbTimeline = timeline.filter((t) => isArbitration(t.status));
  const inArbitration = isArbitration(effective);

  return (
    <div className="space-y-4">
      {/* ERC-8183 4-state blocks */}
      <div className="grid grid-cols-4 gap-2">
        {ERC8183_STATES.map((state, i) => {
          const reached = i <= currentIndex;
          const active = i === currentIndex;
          return (
            <div
              key={state}
              className={cn(
                "rounded-lg border p-3 text-center transition-colors",
                active
                  ? "border-prism-accent bg-prism-accent/10"
                  : reached
                    ? "border-emerald-500/30 bg-emerald-500/5"
                    : "border-white/10 bg-prism-surface/30",
              )}
            >
              <div
                className={cn(
                  "text-xs font-semibold",
                  active
                    ? "text-prism-accent"
                    : reached
                      ? "text-emerald-400"
                      : "text-white/40",
                )}
              >
                {state}
              </div>
              {active && enteredAt && (
                <div className="mt-1 text-[10px] text-white/40">
                  {formatTime(enteredAt)}
                </div>
              )}
              {active && effective === "Completed" && (
                <div className="mt-0.5 text-[10px] text-emerald-400">completed</div>
              )}
              {active && effective === "Refunded" && (
                <div className="mt-0.5 text-[10px] text-amber-400">refunded</div>
              )}
              {/* Reject marker: live demo saw N rejects while in Submitted */}
              {state === "Submitted" && rejectCount !== undefined && rejectCount > 0 && (
                <div className="mt-0.5 text-[10px] font-semibold text-amber-400">
                  ↺ 已打回 ×{rejectCount}
                </div>
              )}
            </div>
          );
        })}
      </div>

      {/* Hook arbitration parallel block (Disputed / DisputeResolved) */}
      <div
        className={cn(
          "rounded-lg border p-3",
          inArbitration
            ? "border-amber-500/40 bg-amber-500/5"
            : "border-white/10 bg-prism-surface/30",
        )}
      >
        <div className="flex items-center justify-between">
          <span
            className={cn(
              "text-xs font-semibold",
              inArbitration ? "text-amber-400" : "text-white/40",
            )}
          >
            ⚖ Hook Arbitration
          </span>
          {inArbitration && (
            <span className="text-[10px] text-amber-400">{effective}</span>
          )}
        </div>
        {arbTimeline.length === 0 ? (
          <p className="mt-1 text-[10px] text-white/30">
            No arbitration raised. ERC-8183 4-state enum does not include
            Disputed — this block is parallel.
          </p>
        ) : (
          <div className="mt-2 space-y-1">
            {arbTimeline.map((t, i) => (
              <div
                key={`${t.status}-${t.tx_hash ?? ""}-${i}`}
                className="flex items-center justify-between text-[10px]"
              >
                <span className="text-amber-300">{t.status}</span>
                <span className="text-white/40">{formatTime(t.timestamp)}</span>
              </div>
            ))}
          </div>
        )}
      </div>

      {/* Internal 6-state timeline (debug view) */}
      <details className="rounded-lg border border-white/10 bg-prism-surface/30 p-3">
        <summary className="cursor-pointer text-xs text-white/60">
          Internal 6-state timeline (debug)
        </summary>
        <div className="mt-2 space-y-1">
          {timeline.length === 0 ? (
            <p className="text-[10px] text-white/30">No state transitions yet.</p>
          ) : (
            timeline.map((t, i) => (
              <motion.div
                key={`${t.tx_hash}-${i}`}
                initial={{ opacity: 0, x: -10 }}
                animate={{ opacity: 1, x: 0 }}
                className="flex items-center gap-2 text-[11px]"
              >
                <span className="font-mono text-prism-accent">{t.status}</span>
                <span className="text-white/30">@ block {t.block_number}</span>
                <span className="ml-auto text-white/40">{formatTime(t.timestamp)}</span>
              </motion.div>
            ))
          )}
        </div>
      </details>
    </div>
  );
}
