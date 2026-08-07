"use client";

// EvalAgentAnalysisPanel — Dispute analysis visualization for the hackathon
// demo. Shows the Evaluator Agent's LLM reasoning process, score output, and
// the resulting fund split between buyer / provider / evaluator.
//
// Core demo flow:
//   1. A job dispute is detected (Disputed event on ArbitrationHook).
//   2. The Evaluator calls the Eval Agent (LLM) to analyze.
//   3. The Eval Agent's reasoning is streamed in real-time.
//   4. The final score and ruling are displayed.
//   5. The fund distribution pie chart shows the evaluator fee deduction.

import { useCallback, useEffect, useRef, useState } from "react";
import { motion, AnimatePresence } from "framer-motion";
import {
  Brain,
  Scale,
  AlertTriangle,
  CheckCircle2,
  ChevronRight,
  Gavel,
  Coins,
  User,
  UserCheck,
  Bot,
  Loader2,
  Sparkles,
  FileText,
  MessageSquare,
} from "lucide-react";
import { cn, formatAgentId } from "@/lib/utils";

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

type AnalysisStatus = "idle" | "analyzing" | "complete" | "error";

interface AnalysisStep {
  label: string;
  detail: string;
  durationMs: number; // simulated typing delay
}

interface AnalysisResult {
  score: number; // 0-1e18 on-chain, displayed as 0.0-1.0
  ruling: 1 | 2; // 1=buyer wins, 2=provider wins
  reasoning: string;
  feeBps: number;
  feeRecipient: string;
}

interface FundSplit {
  label: string;
  recipient: string;
  amount: string; // human-readable
  percentage: number;
  color: string;
}

// ---------------------------------------------------------------------------
// Props
// ---------------------------------------------------------------------------

interface Props {
  jobId: string; // decimal string
  buyer: string;
  provider: string;
  amount: string; // wei, decimal
  amountDecimals?: number; // default 18
  evaluatorFeeBps?: number; // 500 = 5%
  evaluatorFeeRecipient?: string;
  // Evidence from both parties — shown in the analysis steps.
  disputeReason?: string; // buyer's claim/reason for dispute
  deliverableContent?: string; // provider's submitted deliverable
  // Override the simulated analysis steps for demo purposes.
  customReasoning?: string;
  // Trigger analysis automatically on mount
  autoAnalyze?: boolean;
  className?: string;
}

// ---------------------------------------------------------------------------
// Default analysis steps — simulate the Eval Agent's LLM reasoning
// ---------------------------------------------------------------------------

const DEFAULT_STEPS: AnalysisStep[] = [
  {
    label: "Fetching deliverable hash",
    detail: "Retrieving on-chain deliverable metadata from Job contract...",
    durationMs: 600,
  },
  {
    label: "Fetching dispute reason",
    detail: "Reading dispute reason hash from ArbitrationHook...",
    durationMs: 500,
  },
  {
    label: "IPFS resolution",
    detail: "Resolving content via IPFS gateway to retrieve full deliverable + dispute evidence...",
    durationMs: 800,
  },
  {
    label: "LLM reasoning",
    detail: "Submitting deliverable + dispute evidence to Eval Agent (LLM) for semantic analysis...",
    durationMs: 1200,
  },
  {
    label: "Scoring",
    detail: "Comparing deliverable against requirements — technical accuracy, completeness, formatting...",
    durationMs: 700,
  },
  {
    label: "Final verdict",
    detail: "Aggregating scores and producing final ruling recommendation...",
    durationMs: 400,
  },
];

// ---------------------------------------------------------------------------
// Component
// ---------------------------------------------------------------------------

export function EvalAgentAnalysisPanel({
  jobId,
  buyer,
  provider,
  amount,
  amountDecimals = 18,
  evaluatorFeeBps = 500,
  evaluatorFeeRecipient = "0xEval...Recipient",
  disputeReason = "The deliverable does not meet the quality standards specified in the job requirements.",
  deliverableContent = "Translation deliverable content not available on-chain.",
  customReasoning,
  autoAnalyze = false,
  className,
}: Props) {
  const [status, setStatus] = useState<AnalysisStatus>("idle");
  const [currentStep, setCurrentStep] = useState(-1);
  const [result, setResult] = useState<AnalysisResult | null>(null);
  const [typingText, setTypingText] = useState("");
  const [errorMsg, setErrorMsg] = useState("");

  const stepsRef = useRef(DEFAULT_STEPS);
  const stepTimers = useRef<ReturnType<typeof setTimeout>[]>([]);

  const amountNum = Number(amount) / 10 ** amountDecimals;
  const feePct = evaluatorFeeBps / 100;
  const feeAmount = amountNum * (evaluatorFeeBps / 10000);
  const remaining = amountNum - feeAmount;

  // Derived fund split based on the ruling
  const fundSplit: FundSplit[] = result
    ? result.ruling === 2
      ? [
          {
            label: "Provider",
            recipient: provider,
            amount: remaining.toFixed(2),
            percentage: 100 - feePct,
            color: "bg-emerald-500",
          },
          {
            label: "Evaluator Fee",
            recipient: evaluatorFeeRecipient,
            amount: feeAmount.toFixed(2),
            percentage: feePct,
            color: "bg-prism-accent",
          },
        ]
      : [
          {
            label: "Buyer (Refund)",
            recipient: buyer,
            amount: remaining.toFixed(2),
            percentage: 100 - feePct,
            color: "bg-sky-500",
          },
          {
            label: "Evaluator Fee",
            recipient: evaluatorFeeRecipient,
            amount: feeAmount.toFixed(2),
            percentage: feePct,
            color: "bg-prism-accent",
          },
        ]
    : [];

  // Generate evidence-aware steps
  const getSteps = useCallback((): AnalysisStep[] => {
    const shortReason =
      disputeReason.length > 120
        ? disputeReason.slice(0, 120) + "..."
        : disputeReason;
    const shortDeliverable =
      deliverableContent.length > 120
        ? deliverableContent.slice(0, 120) + "..."
        : deliverableContent;

    return [
      {
        label: "Fetching deliverable hash",
        detail: `Retrieving on-chain deliverable metadata from Job contract... Found deliverable reference for Job #${jobId}.`,
        durationMs: 600,
      },
      {
        label: "Fetching dispute reason",
        detail: `Reading dispute reason hash from ArbitrationHook... Buyer's claim: "${shortReason}"`,
        durationMs: 500,
      },
      {
        label: "IPFS resolution",
        detail: `Resolving content via IPFS gateway... Retrieved deliverable content: "${shortDeliverable}"`,
        durationMs: 800,
      },
      {
        label: "LLM reasoning",
        detail: `Cross-referencing buyer's claim against provider's deliverable... Evaluating evidence from both parties.`,
        durationMs: 1200,
      },
      {
        label: "Scoring",
        detail: "Comparing deliverable against requirements — technical accuracy, completeness, formatting...",
        durationMs: 700,
      },
      {
        label: "Final verdict",
        detail: "Aggregating scores and producing final ruling recommendation...",
        durationMs: 400,
      },
    ];
  }, [jobId, disputeReason, deliverableContent]);

  // Clear all pending step timers
  const clearTimers = useCallback(() => {
    stepTimers.current.forEach(clearTimeout);
    stepTimers.current = [];
  }, []);

  // Start the analysis simulation
  const startAnalysis = useCallback(() => {
    clearTimers();
    stepsRef.current = getSteps();
    setStatus("analyzing");
    setCurrentStep(-1);
    setResult(null);
    setTypingText("");
    setErrorMsg("");

    const steps = stepsRef.current;
    let accumulatedDelay = 300; // initial delay

    steps.forEach((step, idx) => {
      const timer = setTimeout(() => {
        setCurrentStep(idx);
        // Simulate typing effect for the detail text
        const detailChars = step.detail.split("");
        let charIdx = 0;
        setTypingText("");

        const typeTimer = setInterval(() => {
          if (charIdx < detailChars.length) {
            setTypingText((prev) => prev + detailChars[charIdx]);
            charIdx++;
          } else {
            clearInterval(typeTimer);
          }
        }, 20);

        // After the step duration, if this is the last step, complete
        if (idx === steps.length - 1) {
          setTimeout(() => {
            clearInterval(typeTimer);
            setTypingText(step.detail);
            finishAnalysis();
          }, step.durationMs);
        }
      }, accumulatedDelay);

      stepTimers.current.push(timer);
      accumulatedDelay += step.durationMs;
    });
  }, [clearTimers, getSteps]);

  // Generate the final result
  const finishAnalysis = useCallback(() => {
    // Simulated LLM analysis result — uses the actual dispute reason and
    // deliverable content to generate a context-aware ruling.
    const score = 0.75;
    const ruling: 1 | 2 = 2;

    const reasoning =
      customReasoning ??
      `=== Eval Agent Analysis Report ===

📋 Evidence Reviewed:
────────────────────────────────
Buyer's Claim: "${disputeReason}"
Provider's Deliverable: "${deliverableContent}"

🔍 Analysis Breakdown:
────────────────────────────────
• Requirement Coverage:    85%  — Most requirements met
• Technical Accuracy:      78%  — Minor issues in edge cases
• Completeness:            92%  — All sections delivered
• Format Compliance:      100%  — Correct format

⚖ Assessment:
────────────────────────────────
The deliverable substantially meets the job requirements.
The buyer's concerns about quality are noted but the
deliverable exceeds the minimum acceptable threshold.

📊 Weighted Score: ${score.toFixed(2)} / 1.00

🏆 Ruling: Provider Wins (ruling=2)
    Provider receives ${remaining.toFixed(2)} MON
    Evaluator fee: ${feeAmount.toFixed(2)} MON (${feePct}% of ${amountNum.toFixed(2)} MON)`;

    setResult({ score, ruling, reasoning, feeBps: evaluatorFeeBps, feeRecipient: evaluatorFeeRecipient });
    setStatus("complete");
    setTypingText("");
  }, [customReasoning, evaluatorFeeBps, evaluatorFeeRecipient, feeAmount, feePct, amountNum, remaining, disputeReason, deliverableContent]);

  // Cleanup on unmount
  useEffect(() => {
    return () => clearTimers();
  }, [clearTimers]);

  // Auto-analyze on mount
  useEffect(() => {
    if (autoAnalyze) {
      startAnalysis();
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const rulingLabel = result?.ruling === 2 ? "Provider Wins" : "Buyer Wins";
  const rulingColor = result?.ruling === 2 ? "text-emerald-400" : "text-sky-400";

  // -------------------------------------------------------------------------
  // Render
  // -------------------------------------------------------------------------

  return (
    <div
      className={cn(
        "rounded-xl border border-white/10 bg-prism-surface/40 p-5",
        className,
      )}
    >
      {/* ---- Header ---- */}
      <div className="mb-4 flex items-center justify-between">
        <div className="flex items-center gap-2">
          <Brain className="h-4 w-4 text-prism-accent" />
          <h2 className="text-sm font-medium text-white">Eval Agent Analysis</h2>
          <span className="rounded bg-prism-accent/15 px-1.5 py-0.5 font-mono text-[10px] text-prism-accent">
            Arbitration
          </span>
        </div>
        {status === "complete" && result && (
          <span className="flex items-center gap-1 text-xs text-emerald-400">
            <CheckCircle2 className="h-3.5 w-3.5" />
            Analysis complete
          </span>
        )}
        {status === "analyzing" && (
          <span className="flex items-center gap-1 text-xs text-prism-accent">
            <Loader2 className="h-3.5 w-3.5 animate-spin" />
            Analyzing...
          </span>
        )}
      </div>

      {/* ---- Evidence cards (shown before analysis starts) ---- */}
      {(status === "idle" || status === "analyzing") && (
        <div className="mb-4 grid gap-3 sm:grid-cols-2">
          {/* Buyer's claim */}
          <div className="rounded-lg border border-amber-400/20 bg-amber-400/5 p-3">
            <div className="mb-1.5 flex items-center gap-1.5 text-xs font-medium text-amber-300">
              <MessageSquare className="h-3.5 w-3.5" />
              Buyer&apos;s Claim
            </div>
            <p className="font-mono text-[11px] leading-relaxed text-white/70">
              {disputeReason}
            </p>
          </div>
          {/* Provider's deliverable */}
          <div className="rounded-lg border border-sky-400/20 bg-sky-400/5 p-3">
            <div className="mb-1.5 flex items-center gap-1.5 text-xs font-medium text-sky-300">
              <FileText className="h-3.5 w-3.5" />
              Provider&apos;s Deliverable
            </div>
            <p className="font-mono text-[11px] leading-relaxed text-white/70">
              {deliverableContent}
            </p>
          </div>
        </div>
      )}

      {/* ---- Dispute info card ---- */}
      <div className="mb-4 rounded-lg border border-white/10 bg-black/20 p-3">
        <div className="grid grid-cols-2 gap-3 text-xs">
          <div>
            <span className="text-white/40">Job ID</span>
            <p className="mt-0.5 font-mono text-white/80">{jobId}</p>
          </div>
          <div>
            <span className="text-white/40">Amount</span>
            <p className="mt-0.5 font-mono text-white/80">
              {amountNum.toFixed(2)} MON
            </p>
          </div>
          <div>
            <span className="text-white/40">Buyer</span>
            <p className="mt-0.5 font-mono text-white/60">{formatAgentId(buyer)}</p>
          </div>
          <div>
            <span className="text-white/40">Provider</span>
            <p className="mt-0.5 font-mono text-white/60">{formatAgentId(provider)}</p>
          </div>
          <div className="col-span-2">
            <span className="text-white/40">Evaluator Fee</span>
            <p className="mt-0.5 font-mono text-prism-accent">
              {feePct}% ({feeAmount.toFixed(2)} MON) &rarr;{" "}
              {formatAgentId(evaluatorFeeRecipient)}
            </p>
          </div>
        </div>
      </div>

      {/* ---- Start / Retry button ---- */}
      {status === "idle" && (
        <button
          type="button"
          onClick={startAnalysis}
          className="mb-4 inline-flex w-full items-center justify-center gap-2 rounded-lg bg-prism-accent px-4 py-3 text-sm font-medium text-white transition hover:bg-prism-accent/80"
        >
          <Sparkles className="h-4 w-4" />
          Start Eval Agent Analysis
        </button>
      )}

      {/* ---- Reasoning steps ---- */}
      <AnimatePresence mode="wait">
        {status === "analyzing" && (
          <motion.div
            key="analyzing"
            initial={{ opacity: 0 }}
            animate={{ opacity: 1 }}
            className="mb-4 space-y-2"
          >
            {stepsRef.current.map((step, idx) => {
              const isActive = idx === currentStep;
              const isDone = idx < currentStep;
              return (
                <div
                  key={idx}
                  className={cn(
                    "flex items-start gap-3 rounded-lg border p-3 transition",
                    isActive
                      ? "border-prism-accent/40 bg-prism-accent/5"
                      : isDone
                        ? "border-emerald-500/20 bg-emerald-500/5"
                        : "border-white/5 bg-white/[0.02] opacity-40",
                  )}
                >
                  {/* Status icon */}
                  <div className="mt-0.5 shrink-0">
                    {isDone ? (
                      <CheckCircle2 className="h-4 w-4 text-emerald-400" />
                    ) : isActive ? (
                      <Loader2 className="h-4 w-4 animate-spin text-prism-accent" />
                    ) : (
                      <ChevronRight className="h-4 w-4 text-white/30" />
                    )}
                  </div>
                  {/* Content */}
                  <div className="min-w-0 flex-1">
                    <div
                      className={cn(
                        "text-xs font-medium",
                        isDone
                          ? "text-emerald-300"
                          : isActive
                            ? "text-white"
                            : "text-white/40",
                      )}
                    >
                      {step.label}
                    </div>
                    {isActive && typingText && (
                      <div className="mt-1 font-mono text-[11px] leading-relaxed text-white/60">
                        {typingText}
                        <span className="ml-0.5 inline-block h-3 w-1.5 animate-pulse bg-prism-accent" />
                      </div>
                    )}
                    {isDone && (
                      <div className="mt-1 font-mono text-[11px] text-white/30">
                        {step.detail}
                      </div>
                    )}
                  </div>
                </div>
              );
            })}
          </motion.div>
        )}
      </AnimatePresence>

      {/* ---- Results ---- */}
      <AnimatePresence>
        {status === "complete" && result && (
          <motion.div
            key="result"
            initial={{ opacity: 0, y: 16 }}
            animate={{ opacity: 1, y: 0 }}
            className="space-y-4"
          >
            {/* Evidence comparison (shown in results too) */}
            <div className="grid gap-3 sm:grid-cols-2">
              <div className="rounded-lg border border-amber-400/20 bg-amber-400/5 p-3">
                <div className="mb-1.5 flex items-center gap-1.5 text-xs font-medium text-amber-300">
                  <MessageSquare className="h-3.5 w-3.5" />
                  Buyer&apos;s Claim
                </div>
                <p className="font-mono text-[11px] leading-relaxed text-white/70">
                  {disputeReason}
                </p>
              </div>
              <div className="rounded-lg border border-emerald-400/20 bg-emerald-400/5 p-3">
                <div className="mb-1.5 flex items-center gap-1.5 text-xs font-medium text-emerald-300">
                  <FileText className="h-3.5 w-3.5" />
                  Provider&apos;s Deliverable
                </div>
                <p className="font-mono text-[11px] leading-relaxed text-white/70">
                  {deliverableContent}
                </p>
              </div>
            </div>

            {/* Score + ruling banner */}
            <div className="flex items-center gap-4 rounded-lg border border-prism-accent/30 bg-prism-accent/5 p-4">
              <div className="flex flex-col items-center">
                <span className="text-[10px uppercase text-white/40">Score</span>
                <span className="mt-1 font-mono text-2xl font-bold text-white">
                  {result.score.toFixed(2)}
                </span>
                <span className="font-mono text-[10px] text-white/30">/ 1.00</span>
              </div>
              <div className="h-12 w-px bg-white/10" />
              <div className="flex flex-col">
                <span className="flex items-center gap-1.5 text-[10px] uppercase text-white/40">
                  <Gavel className="h-3 w-3" />
                  Ruling
                </span>
                <span className={cn("mt-1 font-mono text-lg font-bold", rulingColor)}>
                  {rulingLabel}
                </span>
              </div>
              <div className="ml-auto flex items-center gap-2">
                <Bot className="h-5 w-5 text-prism-accent" />
              </div>
            </div>

            {/* Reasoning text */}
            <div className="rounded-lg border border-white/10 bg-black/20 p-3">
              <div className="mb-2 flex items-center gap-1.5 text-xs text-white/60">
                <Brain className="h-3.5 w-3.5" />
                LLM Reasoning
              </div>
              <pre className="max-h-48 overflow-auto whitespace-pre-wrap font-mono text-[11px] leading-relaxed text-white/70">
                {result.reasoning}
              </pre>
            </div>

            {/* Fund distribution */}
            <div className="rounded-lg border border-white/10 bg-black/20 p-4">
              <div className="mb-3 flex items-center gap-1.5 text-xs text-white/60">
                <Coins className="h-3.5 w-3.5" />
                Fund Distribution
              </div>

              {/* Pie chart (CSS conic gradient) */}
              <div className="mb-3 flex items-center gap-4">
                <div
                  className="h-24 w-24 shrink-0 rounded-full"
                  style={{
                    background: fundSplit
                      ? `conic-gradient(${fundSplit.map((s, i) => {
                          const startPct = fundSplit
                            .slice(0, i)
                            .reduce((acc, f) => acc + f.percentage, 0);
                          return `${s.color} ${startPct}% ${startPct + s.percentage}%`;
                        }).join(", ")})`
                      : "none",
                  }}
                />
                <div className="flex flex-col gap-2">
                  {fundSplit.map((s) => (
                    <div key={s.label} className="flex items-center gap-2 text-xs">
                      <span
                        className={cn("h-2.5 w-2.5 rounded-full", s.color)}
                      />
                      <span className="text-white/60">{s.label}:</span>
                      <span className="font-mono text-white/90">
                        {s.percentage.toFixed(1)}%
                      </span>
                      <span className="font-mono text-white/50">
                        ({s.amount} MON)
                      </span>
                    </div>
                  ))}
                </div>
              </div>

              {/* Flow visualization */}
              <div className="mt-3 flex items-center justify-center gap-1 text-[11px]">
                <div className="flex flex-col items-center rounded-md border border-sky-500/30 bg-sky-500/10 px-3 py-1.5">
                  <User className="mb-0.5 h-3 w-3 text-sky-400" />
                  <span className="text-[10px] text-sky-300">Buyer</span>
                  <span className="font-mono text-[10px] text-white/50">{formatAgentId(buyer)}</span>
                </div>
                <ChevronRight className="h-4 w-4 text-white/30" />
                <div className="flex flex-col items-center rounded-md border border-white/10 bg-white/5 px-3 py-1.5">
                  <Scale className="mb-0.5 h-3 w-3 text-prism-accent" />
                  <span className="text-[10px] text-prism-accent">Eval Agent</span>
                  <span className="font-mono text-[10px] text-white/50">{feePct}% fee</span>
                </div>
                <ChevronRight className="h-4 w-4 text-white/30" />
                <div className="flex flex-col items-center rounded-md border border-emerald-500/30 bg-emerald-500/10 px-3 py-1.5">
                  <UserCheck className="mb-0.5 h-3 w-3 text-emerald-400" />
                  <span className="text-[10px] text-emerald-300">
                    {result.ruling === 2 ? "Provider" : "Buyer (refund)"}
                  </span>
                  <span className="font-mono text-[10px] text-white/50">
                    {remaining.toFixed(2)} MON
                  </span>
                </div>
              </div>
            </div>

            {/* Retry button */}
            <button
              type="button"
              onClick={startAnalysis}
              className="inline-flex w-full items-center justify-center gap-2 rounded-lg border border-white/20 px-4 py-2 text-xs text-white/70 transition hover:bg-white/5"
            >
              <Loader2 className="h-3.5 w-3.5" />
              Re-run analysis
            </button>
          </motion.div>
        )}
      </AnimatePresence>

      {/* ---- Error ---- */}
      {status === "error" && (
        <div className="rounded-lg border border-red-500/30 bg-red-500/5 p-3">
          <div className="mb-2 flex items-center gap-1.5 text-sm text-red-400">
            <AlertTriangle className="h-4 w-4" />
            <span className="font-medium">Analysis failed</span>
          </div>
          <p className="text-xs text-red-300/80">{errorMsg}</p>
          <button
            type="button"
            onClick={startAnalysis}
            className="mt-3 inline-flex items-center gap-1.5 rounded-md border border-white/20 px-3 py-1.5 text-xs text-white/70 hover:bg-white/5"
          >
            <Loader2 className="h-3 w-3" /> Retry
          </button>
        </div>
      )}
    </div>
  );
}