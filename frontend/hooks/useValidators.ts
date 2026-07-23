"use client";

// useValidators — derives a validator leaderboard from PRISM_STAKED events
// since the backend does not yet expose a dedicated /validators endpoint.
//
// Each Staked event signals a validator deposit; we aggregate by `from`
// (the staker address) and sum the value. This is read-only display data —
// the authoritative stake tuple comes from the on-chain Registry.stakeInfo
// (see app/validator/page.tsx for the per-wallet view).
//
// Once the backend lands /validators/list, this hook can switch over with
// no component changes.

import { useMemo } from "react";
import { useEvents } from "./useEvents";
import { formatEther } from "viem";

export interface ValidatorRow {
  address: string;
  totalStaked: string; // decimal ETH string (human-readable)
  totalStakedWei: bigint;
  stakeCount: number;
  lastActiveAt: number;
  txHash: string;
}

export function useValidators(opts: { chainName?: string; size?: number } = {}) {
  const { chainName, size = 100 } = opts;
  // Pull up to `size` staked events — the events API is newest-first.
  const { events, total, isValidating, error } = useEvents({
    chainName,
    eventType: "PRISM_STAKED",
    size,
    intervalMs: 20000,
  });

  const rows = useMemo<ValidatorRow[]>(() => {
    const map = new Map<string, ValidatorRow>();
    for (const e of events) {
      const addr = (e.from || "").toLowerCase();
      if (!addr) continue;
      const prev = map.get(addr);
      let wei = 0n;
      try {
        wei = BigInt(e.value || "0");
      } catch {
        wei = 0n;
      }
      if (!prev) {
        map.set(addr, {
          address: addr,
          totalStakedWei: wei,
          totalStaked: formatEther(wei),
          stakeCount: 1,
          lastActiveAt: e.block_time,
          txHash: e.tx_hash,
        });
      } else {
        const sum = prev.totalStakedWei + wei;
        map.set(addr, {
          ...prev,
          totalStakedWei: sum,
          totalStaked: formatEther(sum),
          stakeCount: prev.stakeCount + 1,
          lastActiveAt: Math.max(prev.lastActiveAt, e.block_time),
          txHash: e.block_time > prev.lastActiveAt ? e.tx_hash : prev.txHash,
        });
      }
    }
    // Sort by total staked (desc).
    return Array.from(map.values()).sort(
      (a, b) => Number(b.totalStakedWei - a.totalStakedWei),
    );
  }, [events]);

  return {
    rows,
    totalStakedEvents: total,
    isValidating,
    error,
  };
}
