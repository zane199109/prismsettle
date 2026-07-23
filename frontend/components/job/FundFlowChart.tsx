"use client";

// FundFlowChart — Buyer → Job contract → Provider flow visualization with
// x402 / ERC-20 dual path labels. DEV-PLAN §Phase 8 任务 8.5d (FR-JM04).
//
// ASCII-art style horizontal flow (kept lightweight, no SVG dependency):
//
//   [Buyer] ──amount──▶ [Job Contract] ──amount──▶ [Provider]
//      │                                          ▲
//      │ x402 receipt OR ERC-20 approve           │
//      └──── facilitator (if x402) ──────────────┘

import { ArrowRight, Sparkles, Shield } from "lucide-react";
import { motion } from "framer-motion";
import { cn, formatAgentId } from "@/lib/utils";

interface Props {
  buyer: string;
  jobContract: string;
  provider: string;
  amount: string;
  // Whether the x402 facilitator was used. When false, ERC-20 transferFrom
  // was used instead.
  usedX402: boolean;
}

export function FundFlowChart({
  buyer,
  jobContract,
  provider,
  amount,
  usedX402,
}: Props) {
  return (
    <div className="rounded-xl border border-white/10 bg-prism-surface/40 p-5">
      <div className="mb-4 flex items-center justify-between">
        <h3 className="text-sm font-medium text-white">Fund Flow</h3>
        <span
          className={cn(
            "inline-flex items-center gap-1 rounded-md border px-2 py-0.5 text-[11px] font-medium",
            usedX402
              ? "border-prism-accent/40 bg-prism-accent/10 text-prism-accent"
              : "border-amber-500/40 bg-amber-500/10 text-amber-400",
          )}
        >
          {usedX402 ? (
            <>
              <Sparkles className="h-3 w-3" /> x402 path
            </>
          ) : (
            <>
              <Shield className="h-3 w-3" /> ERC-20 path
            </>
          )}
        </span>
      </div>

      <div className="flex items-center gap-2 overflow-x-auto">
        <Node label="Buyer" address={buyer} tone="violet" />
        <Edge label={`${amount} wei`} />
        <Node label="Job Contract" address={jobContract} tone="pink" />
        <Edge label={`${amount} wei`} />
        <Node label="Provider" address={provider} tone="emerald" />
      </div>

      <p className="mt-4 text-[11px] text-white/40">
        {usedX402
          ? "Funds settled via Monad x402 facilitator (receipt forwarded to facilitator.settleWithReceipt)."
          : "Funds pulled via ERC-20 transferFrom (facilitator was address(0) — fallback path)."}
      </p>

      {/* Facilitator bypass line (only shown for x402) */}
      {usedX402 && (
        <motion.div
          initial={{ opacity: 0 }}
          animate={{ opacity: 1 }}
          className="mt-3 border-t border-dashed border-prism-accent/30 pt-3"
        >
          <div className="flex items-center gap-2 text-[11px] text-prism-accent/80">
            <Sparkles className="h-3 w-3" />
            Facilitator bypassed the Job contract for receipt verification;
            net flow is still Buyer → Job → Provider.
          </div>
        </motion.div>
      )}
    </div>
  );
}

function Node({
  label,
  address,
  tone,
}: {
  label: string;
  address: string;
  tone: "violet" | "pink" | "emerald";
}) {
  const toneClass = {
    violet: "border-prism-accent/40 bg-prism-accent/5 text-prism-accent",
    pink: "border-pink-500/40 bg-pink-500/5 text-pink-400",
    emerald: "border-emerald-500/40 bg-emerald-500/5 text-emerald-400",
  }[tone];
  return (
    <div
      className={cn(
        "min-w-[140px] rounded-lg border px-3 py-2 text-center",
        toneClass,
      )}
    >
      <div className="text-[10px] uppercase tracking-wider opacity-70">{label}</div>
      <div className="mt-0.5 truncate font-mono text-xs">{formatAgentId(address)}</div>
    </div>
  );
}

function Edge({ label }: { label: string }) {
  return (
    <div className="flex flex-col items-center px-1">
      <span className="text-[10px] text-white/40">{label}</span>
      <ArrowRight className="h-4 w-4 text-white/60" />
    </div>
  );
}
