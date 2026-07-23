"use client";

// useEvents — polls GET /events (paginated) with optional event_type filter.
// Used by the /events global feed page. Backend returns ChainEvent rows
// written by the listener; reorg-affected rows carry markers in `extra`.
//
// Mock fallback: when the backend is unreachable or returns an empty page,
// the hook falls back to MOCK_EVENTS so the demo always has content.

import { usePoll } from "./usePoll";
import { getEvents } from "@/lib/prismsettle";
import { getMockEvents } from "@/lib/mock-data";
import type { ChainEvent, Paginated } from "@/lib/types";

export interface UseEventsOptions {
  chainName?: string;
  eventType?: string; // e.g. "PRISM_STAKED"; "" = all
  page?: number;
  size?: number;
  intervalMs?: number;
  // When provided, mock fallback attributes events to this wallet — used by
  // /me to show personalized validations and slashes.
  userAddress?: string;
}

export function useEvents(opts: UseEventsOptions = {}) {
  const {
    chainName,
    eventType,
    page = 1,
    size = 50,
    intervalMs = 8000,
    userAddress,
  } = opts;

  const key = `events:${chainName ?? "all"}:${eventType ?? "all"}:${page}:${size}`;
  const { data, error, isValidating, mutate } = usePoll<Paginated<ChainEvent>>(
    key,
    async () => {
      try {
        const res = await getEvents({ chain_name: chainName, event_type: eventType, page, size });
        if (res.items && res.items.length > 0) return res;
        // Empty page — fall back to mock so the UI is never blank.
        const mock = getMockEvents(buildMockOpts(eventType, userAddress, size));
        return { items: mock.items, total: mock.total, page, size };
      } catch {
        const mock = getMockEvents(buildMockOpts(eventType, userAddress, size));
        return { items: mock.items, total: mock.total, page, size };
      }
    },
    { intervalMs, pauseWhenHidden: true },
  );

  return {
    events: data?.items ?? [],
    total: data?.total ?? 0,
    page: data?.page ?? page,
    size: data?.size ?? size,
    error,
    isValidating,
    refresh: mutate,
  };
}

// Build mock fallback options. When userAddress is provided, attribute events
// to the connected wallet so /me shows personalized activity:
//   - PRISM_VALIDATION_SUBMITTED / PRISM_STAKED: override `from` (validator)
//   - PRISM_SLASHED: override `to` (slashed validator)
function buildMockOpts(
  eventType: string | undefined,
  userAddress: string | undefined,
  size: number,
): Parameters<typeof getMockEvents>[0] {
  const opts: Parameters<typeof getMockEvents>[0] = { eventType, size };
  if (userAddress) {
    if (eventType === "PRISM_SLASHED") {
      opts.overrideTo = userAddress;
    } else if (
      eventType === "PRISM_VALIDATION_SUBMITTED" ||
      eventType === "PRISM_STAKED"
    ) {
      opts.overrideFrom = userAddress;
    }
  }
  return opts;
}
