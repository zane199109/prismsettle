// Utility helpers shared across the dashboard. Pure functions only — no
// React imports — so they can be unit-tested or used in server components.

import { clsx, type ClassValue } from "clsx";
import { twMerge } from "tailwind-merge";

// cn merges Tailwind classes with conditional logic, deduping conflicts.
// Standard shadcn/ui pattern.
export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs));
}

// formatAgentId truncates a 0x-prefixed hex address to "0xabcd…1234" for
// compact display. Non-hex strings pass through unchanged (e.g. agent IDs
// that are uint256 decimals).
export function formatAgentId(id: string): string {
  if (!id) return "";
  if (!id.startsWith("0x")) return id;
  if (id.length <= 12) return id;
  return `${id.slice(0, 8)}…${id.slice(-4)}`;
}

// formatScore converts a uint256-scaled score string (1e18 base) to a
// 4-decimal fixed-point representation. Returns "0.0000" for empty input.
// BigInt parsing avoids Number precision loss on large scores.
export function formatScore(value: string | undefined | null): string {
  if (!value) return "0.0000";
  try {
    const num = Number(BigInt(value)) / 1e18;
    return num.toFixed(4);
  } catch {
    return value;
  }
}

// formatTime renders a unix timestamp (seconds) as a relative "5s ago" /
// "12m ago" / "3h ago" string. Falls back to ISO date for old timestamps.
export function formatTime(ts: number): string {
  if (!ts) return "";
  const diff = Date.now() / 1000 - ts;
  if (diff < 0) return "just now";
  if (diff < 60) return `${Math.floor(diff)}s ago`;
  if (diff < 3600) return `${Math.floor(diff / 60)}m ago`;
  if (diff < 86400) return `${Math.floor(diff / 3600)}h ago`;
  return new Date(ts * 1000).toISOString().slice(0, 10);
}

// gradeFromScore maps a 0..1 score (after 1e18 scaling) to an A/B/C/D grade
// per the SD §4.6 visual identity. Thresholds match the trust_thresholds
// defaults: 0.8 = A, 0.5 = B, 0.3 = C, below = D.
export function gradeFromScore(scoreStr: string): "A" | "B" | "C" | "D" {
  let s = 0;
  try {
    s = Number(BigInt(scoreStr)) / 1e18;
  } catch {
    return "D";
  }
  if (s >= 0.8) return "A";
  if (s >= 0.5) return "B";
  if (s >= 0.3) return "C";
  return "D";
}

// gradeColor returns the Tailwind text-color class for a grade.
export function gradeColor(grade: string): string {
  switch (grade) {
    case "A":
      return "text-emerald-400";
    case "B":
      return "text-blue-400";
    case "C":
      return "text-amber-400";
    case "D":
      return "text-red-400";
    default:
      return "text-zinc-500";
  }
}

// formatBigInt safely converts a decimal string to a Number with the given
// decimals. Used by charts that need numeric values from on-chain BigInts.
export function formatBigInt(value: string, decimals = 18): number {
  if (!value) return 0;
  try {
    return Number(BigInt(value)) / 10 ** decimals;
  } catch {
    return 0;
  }
}

// Agent category tags for the marketplace capability filter (PRD FR-M02 / SD §4.4).
// Derived from metadata keywords since Seed Phase agents use plain-text metadata
// ("DeFi Analysis Agent", "Translation Agent", etc.) rather than a structured schema.
export type AgentCategory = "DeFi" | "Data" | "Translation" | "Eval" | "Other";

export const AGENT_CATEGORIES: AgentCategory[] = ["DeFi", "Data", "Translation", "Eval"];

export function agentCategory(metadata: string): AgentCategory {
  const m = metadata.toLowerCase();
  if (m.includes("defi")) return "DeFi";
  if (m.includes("data") || m.includes("label")) return "Data";
  if (m.includes("translat")) return "Translation";
  if (m.includes("eval")) return "Eval";
  return "Other";
}
