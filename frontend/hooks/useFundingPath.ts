"use client";

// useFundingPath — probes the PrismSettleJob contract to detect whether the
// x402 facilitator is configured. When facilitator == address(0), the buyer
// must use the ERC-20 fallback path. DEV-PLAN §Phase 8 任务 8.5a (FR-AP09).
//
// In anvil/local builds the facilitator is address(0) → fallback path.
// On Monad testnet the facilitator is set at deploy time → x402 path.

import { useEffect, useState } from "react";

export interface FundingPathInfo {
  path: "x402" | "erc20";
  facilitatorAddress: string;
  facilitatorAvailable: boolean;
  // Human-readable hint for the UI badge.
  hint: string;
  loading: boolean;
  error: string | null;
}

const ZERO_ADDR = "0x0000000000000000000000000000000000000000";

export function useFundingPath(
  jobContractAddress: string | undefined,
  opts: { rpcUrl?: string } = {},
): FundingPathInfo {
  const { rpcUrl } = opts;
  const [state, setState] = useState<FundingPathInfo>({
    path: "erc20",
    facilitatorAddress: ZERO_ADDR,
    facilitatorAvailable: false,
    hint: "Probing facilitator…",
    loading: true,
    error: null,
  });

  useEffect(() => {
    if (!jobContractAddress) {
      setState({
        path: "erc20",
        facilitatorAddress: ZERO_ADDR,
        facilitatorAvailable: false,
        hint: "Job contract address not configured — using ERC-20 fallback.",
        loading: false,
        error: null,
      });
      return;
    }

    let cancelled = false;
    (async () => {
      try {
        // eth_call to facilitator() selector: 0x6f5e3a3a
        // (keccak256("facilitator()") first 4 bytes).
        const data = "0x6f5e3a3a";
        const endpoint = rpcUrl ?? "/api/rpc";
        const res = await fetch(endpoint, {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({
            jsonrpc: "2.0",
            id: 1,
            method: "eth_call",
            params: [{ to: jobContractAddress, data }, "latest"],
          }),
        });
        const json = await res.json();
        if (cancelled) return;
        if (json.error) throw new Error(json.error.message ?? "eth_call failed");
        const result: string = json.result ?? "0x";
        // Result is 32 bytes; the address is the last 20 bytes.
        const addr = result.length >= 66
          ? ("0x" + result.slice(26)).toLowerCase()
          : ZERO_ADDR;
        const available = addr !== ZERO_ADDR;
        setState({
          path: available ? "x402" : "erc20",
          facilitatorAddress: addr,
          facilitatorAvailable: available,
          hint: available
            ? "x402 facilitator active — payment routed via Monad official facilitator."
            : "facilitator is address(0) — using ERC-20 fallback path (FR-AP09).",
          loading: false,
          error: null,
        });
      } catch (err) {
        if (cancelled) return;
        setState({
          path: "erc20",
          facilitatorAddress: ZERO_ADDR,
          facilitatorAvailable: false,
          hint: "Facilitator probe failed — defaulting to ERC-20 fallback.",
          loading: false,
          error: err instanceof Error ? err.message : String(err),
        });
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [jobContractAddress, rpcUrl]);

  return state;
}
