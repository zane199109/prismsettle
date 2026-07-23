"use client";

// FundingPathBadge — small UI chip showing the active funding path
// (x402 vs ERC-20 fallback). DEV-PLAN §Phase 8 任务 8.5a (FR-AP09).

import { Sparkles, Shield } from "lucide-react";
import { cn } from "@/lib/utils";
import { useFundingPath } from "@/hooks/useFundingPath";

interface Props {
  jobContractAddress: string | undefined;
  rpcUrl?: string;
}

export function FundingPathBadge({ jobContractAddress, rpcUrl }: Props) {
  const { path, facilitatorAvailable, hint, loading } = useFundingPath(
    jobContractAddress,
    { rpcUrl },
  );

  if (loading) {
    return (
      <span className="inline-flex items-center gap-1 rounded-md border border-white/10 bg-prism-surface/40 px-2 py-1 text-[11px] text-white/40">
        probing…
      </span>
    );
  }

  const isX402 = path === "x402" && facilitatorAvailable;
  return (
    <span
      className={cn(
        "inline-flex items-center gap-1 rounded-md border px-2 py-1 text-[11px] font-medium",
        isX402
          ? "border-prism-accent/40 bg-prism-accent/10 text-prism-accent"
          : "border-amber-500/40 bg-amber-500/10 text-amber-400",
      )}
      title={hint}
    >
      {isX402 ? (
        <>
          <Sparkles className="h-3 w-3" /> x402 path
        </>
      ) : (
        <>
          <Shield className="h-3 w-3" /> ERC-20 fallback
        </>
      )}
    </span>
  );
}
