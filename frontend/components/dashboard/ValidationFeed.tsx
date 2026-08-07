"use client";

// ValidationFeed — live event ticker styled like a trading terminal.
// New events slide in with a green flash that fades; reorg-affected events
// get an amber ↻ marker; slash events get a red ⛔.
//
// Reorg-awareness (Phase 8 task 8.5): the backend marks events whose block
// was orphaned by setting `extra` to a JSON object containing
// `{ "reorged": true }`. When the feed sees such an event, it renders the
// amber marker and a strikethrough on the value — making chain reorgs
// visible in real time. This is rare in dashboards and proves infra
// credibility (SD §4.7).
//
// Uses AnimatePresence so removed items (when the list rolls over) exit
// gracefully instead of popping.

import { AnimatePresence, motion } from "framer-motion";
import { usePoll } from "@/hooks/usePoll";
import { getEvents } from "@/lib/prismsettle";
import type { ChainEvent } from "@/lib/types";
import { formatAgentId, formatScore, formatTime } from "@/lib/utils";

interface ValidationFeedProps {
  chainName?: string;
  // Number of events to display. Default 20.
  limit?: number;
  // Polling interval in ms. Default 4s — slightly faster than heatmap
  // because the feed is the "live pulse" of the dashboard.
  intervalMs?: number;
}

// isReorged inspects the event's extra JSON payload for a reorg marker.
// Defensive parsing — extra may be empty, malformed, or a non-JSON string
// from older indexer versions.
function isReorged(ev: ChainEvent): boolean {
  if (!ev.extra) return false;
  try {
    const obj = JSON.parse(ev.extra);
    return obj?.reorged === true || obj?.rolled_back === true;
  } catch {
    return false;
  }
}

function isSlashEvent(eventType: string): boolean {
  return eventType.toUpperCase().includes("SLASH");
}

function isReorgEvent(eventType: string): boolean {
  return eventType.toUpperCase().includes("REORG") ||
    eventType.toUpperCase().includes("ROLLBACK");
}

export function ValidationFeed({
  chainName,
  limit = 20,
  intervalMs = 4000,
}: ValidationFeedProps) {
  // Fetch more than we display so when new events arrive the list doesn't
  // look empty during the roll-over.
  const { data, error, isValidating } = usePoll(
    chainName !== null ? `events:${chainName ?? "all"}` : null,
    () => getEvents({ chainName, size: limit }),
    { intervalMs, pauseWhenHidden: true },
  );

  const events = data?.items ?? [];

  return (
    <div className="flex flex-col gap-2">
      <div className="flex items-center justify-between">
        <div className="text-xs uppercase tracking-wider text-white/40">
          Live Validation Feed
        </div>
        {isValidating && (
          <span className="h-2 w-2 animate-pulse rounded-full bg-emerald-400" />
        )}
      </div>

      <div className="space-y-1 font-mono text-xs">
        <AnimatePresence initial={false}>
          {events.length === 0 && !error && (
            <motion.div
              initial={{ opacity: 0 }}
              animate={{ opacity: 1 }}
              className="px-2 py-3 text-center text-white/30"
            >
              waiting for events…
            </motion.div>
          )}
          {events.map((e) => {
            const reorged = isReorged(e);
            const slashed = isSlashEvent(e.event_type);
            const isReorgEvt = isReorgEvent(e.event_type);
            return (
              <motion.div
                key={`${e.tx_hash}-${e.block_number}-${e.id}`}
                initial={{ opacity: 0, x: -16, backgroundColor: "rgba(16, 185, 129, 0.25)" }}
                animate={{
                  opacity: 1,
                  x: 0,
                  backgroundColor: "rgba(0, 0, 0, 0)",
                }}
                exit={{ opacity: 0, x: 16 }}
                transition={{ duration: 0.6 }}
                className={`flex items-center gap-3 rounded px-2 py-1 ${
                  reorged ? "line-through opacity-60" : ""
                }`}
              >
                {/* Status icon */}
                {slashed ? (
                  <span className="text-red-400" title="Slash event">⛔</span>
                ) : isReorgEvt || reorged ? (
                  <span className="text-amber-400" title="Reorg-affected">↻</span>
                ) : (
                  <span className="text-emerald-400">•</span>
                )}

                <span className="text-zinc-400">
                  {formatAgentId(e.to || e.from)}
                </span>
                <span className="text-violet-400">
                  {e.event_type.replace(/^PRISM_/, "").replace(/_/g, " ")}
                </span>

                <span className="ml-auto text-zinc-500">
                  {formatTime(e.block_time)}
                </span>

                {e.value && e.value !== "0" && (
                  <span
                    className={`tabular-nums ${
                      slashed ? "text-red-400" : "text-emerald-400"
                    }`}
                  >
                    {formatScore(e.value)}
                  </span>
                )}
              </motion.div>
            );
          })}
        </AnimatePresence>
      </div>

      {error && (
        <div className="px-2 py-1 text-xs text-red-400">
          feed error: {error.message}
        </div>
      )}
    </div>
  );
}
