"use client";

// TrustGate — three-state decision UI (ALLOW / DENY / REQUIRE_VALIDATION).
// DEV-PLAN §Phase 8 任务 8.4 + FR-AP12/FR-AP13.
//
// Behavior:
//   - ALLOW              → green check, job creation enabled
//   - REQUIRE_VALIDATION → amber, hook will be mounted before funding
//                          (FR-AP12). Job can be created but buyer is warned.
//   - DENY               → red, createJob button is disabled (FR-AP13)
//
// The component reads live thresholds from the trust check result so the
// buyer can see exactly why the decision was made.

import { ShieldCheck, ShieldAlert, ShieldX, RefreshCw } from "lucide-react";
import { useTrustCheck } from "@/hooks/useTrustCheck";
import { cn, formatScore } from "@/lib/utils";
import type { TrustCheckResult } from "@/lib/types";
import { Skeleton } from "@/components/ui/skeleton";

interface Props {
  agentId: string;
  chainName?: string;
  // Optional callback when the decision changes (for parent form to enable /
  // disable the submit button).
  onDecisionChange?: (decision: TrustCheckResult["decision"]) => void;
}

export function TrustGate({ agentId, chainName, onDecisionChange }: Props) {
  const { result, isValidating, refresh } = useTrustCheck(agentId, { chainName });

  // Notify parent of decision changes via effect-safe pattern (parent's
  // setState is idempotent so we just call on each render).
  if (result && onDecisionChange) {
    onDecisionChange(result.decision);
  }

  if (isValidating && !result) {
    return (
      <div className="rounded-lg border border-white/10 bg-prism-surface/40 p-4">
        <div className="flex items-center gap-2">
          <Skeleton className="h-5 w-24" />
          <Skeleton className="ml-auto h-3.5 w-3.5" />
        </div>
        <div className="mt-3 grid grid-cols-3 gap-2">
          <Skeleton className="h-10" />
          <Skeleton className="h-10" />
          <Skeleton className="h-10" />
        </div>
        <Skeleton className="mt-3 h-3 w-full" />
      </div>
    );
  }

  if (!result) {
    return (
      <div className="rounded-lg border border-dashed border-white/20 p-3 text-sm text-white/40">
        Trust check unavailable. Enter a valid agent id.
      </div>
    );
  }

  const tone = TONE[result.decision];
  const Icon = ICON[result.decision];

  return (
    <div className={cn("rounded-lg border p-4", tone.border, tone.bg)}>
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-2">
          <Icon className={cn("h-5 w-5", tone.text)} />
          <span className={cn("text-sm font-semibold", tone.text)}>
            {result.decision.toUpperCase()}
          </span>
        </div>
        <button
          type="button"
          onClick={() => refresh()}
          className="text-white/40 hover:text-white"
          title="Re-check trust"
        >
          <RefreshCw className="h-3.5 w-3.5" />
        </button>
      </div>

      <div className="mt-3 grid grid-cols-3 gap-2 text-xs">
        <Threshold
          label="score"
          value={formatScore(result.score)}
          tone="text-white"
        />
        <Threshold
          label="allow ≥"
          value={formatScore(result.allow_threshold)}
          tone="text-emerald-400"
        />
        <Threshold
          label="deny <"
          value={formatScore(result.deny_threshold)}
          tone="text-red-400"
        />
      </div>

      <p className={cn("mt-3 text-xs", tone.text)}>{tone.hint}</p>
    </div>
  );
}

function Threshold({
  label,
  value,
  tone,
}: {
  label: string;
  value: string;
  tone: string;
}) {
  return (
    <div className="rounded-md bg-black/30 px-2 py-1.5 text-center">
      <div className="text-[10px] uppercase tracking-wider text-white/40">{label}</div>
      <div className={cn("font-mono text-sm font-semibold", tone)}>{value}</div>
    </div>
  );
}

const TONE: Record<
  TrustCheckResult["decision"],
  { border: string; bg: string; text: string; hint: string }
> = {
  allow: {
    border: "border-emerald-500/40",
    bg: "bg-emerald-500/5",
    text: "text-emerald-400",
    hint: "Agent reputation is above the allow threshold. Job creation enabled.",
  },
  review: {
    border: "border-amber-500/40",
    bg: "bg-amber-500/5",
    text: "text-amber-400",
    hint:
      "FR-AP12: score is in the review band. A validation hook will be mounted before funding — proceed with caution.",
  },
  deny: {
    border: "border-red-500/40",
    bg: "bg-red-500/5",
    text: "text-red-400",
    hint:
      "FR-AP13: score is below the deny threshold. createJob is blocked until the agent's reputation recovers.",
  },
};

const ICON = {
  allow: ShieldCheck,
  review: ShieldAlert,
  deny: ShieldX,
} as const;
