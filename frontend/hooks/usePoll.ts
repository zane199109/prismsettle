"use client";

// usePoll: a thin SWR wrapper that polls on a fixed interval and exposes
// pause/resume controls. Used by dashboard widgets that need fresh data
// without user interaction (agents list, shard heatmap, reorg feed).
//
// Phase 7 task 7.5 — kept minimal so Phase 8 can layer in optimistic UI,
// focus pausing (when tab is hidden), and backoff without touching call
// sites.

import useSWR, { SWRConfiguration } from "swr";

export interface UsePollOptions<T> {
  // Polling interval in ms. 0 = no polling (one-shot fetch).
  intervalMs?: number;
  // Pause polling when the document is hidden. Default true to save RPC
  // quota — set to false for endpoints where staleness is unacceptable
  // (e.g. reorg feed).
  pauseWhenHidden?: boolean;
  // Extra SWR config (dedupingInterval, keepPreviousData, etc.).
  swr?: SWRConfiguration<T>;
}

export function usePoll<T>(
  key: string | null,
  fetcher: () => Promise<T>,
  opts: UsePollOptions<T> = {},
) {
  const { intervalMs = 5000, pauseWhenHidden = true, swr } = opts;
  return useSWR<T>(key, fetcher, {
    refreshInterval: intervalMs,
    revalidateOnFocus: false,
    revalidateOnReconnect: true,
    // SWR's isPaused lets us freeze polling when the tab is hidden.
    isPaused: () =>
      pauseWhenHidden &&
      typeof document !== "undefined" &&
      document.visibilityState === "hidden",
    ...swr,
  });
}
