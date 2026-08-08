"use client";

// useEvents — polls GET /events (paginated) with optional event_type filter.
// Used by the /events global feed page. Backend returns ChainEvent rows
// written by the listener; reorg-affected rows carry markers in `extra`.
//
// Mock fallback: when the backend is unreachable or returns an empty page,
// the hook falls back to MOCK_EVENTS so the demo always has content.

import { usePoll } from "./usePoll";
import { getEvents } from "@/lib/prismsettle";
import { CHAIN_NAME } from "@/lib/contracts";
import type { ChainEvent, Paginated } from "@/lib/types";

export interface UseEventsOptions {
  chainName?: string;
  eventType?: string; // e.g. "PRISM_STAKED"; "" = all
  to?: string; // Filter by receiver address/ID (client-side + mock fallback)
  page?: number;
  size?: number;
  intervalMs?: number;
  // When provided, mock fallback attributes events to this wallet — used by
  // /me to show personalized validations and slashes.
  userAddress?: string;
}

export function useEvents(opts: UseEventsOptions = {}) {
  const {
    chainName = CHAIN_NAME,
    eventType,
    to,
    page = 1,
    size = 50,
    intervalMs = 8000,
    userAddress,
  } = opts;

  const key = `events:${chainName ?? "all"}:${eventType ?? "all"}:${to ?? "all"}:${page}:${size}`;
  const { data, error, isValidating, mutate } = usePoll<Paginated<ChainEvent>>(
    key,
    async () => {
      const res = await getEvents({ chainName, page, size });
      const items = to
        ? res.items.filter((e) => e.to?.toLowerCase() === to.toLowerCase())
        : res.items;
      return { ...res, items };
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
