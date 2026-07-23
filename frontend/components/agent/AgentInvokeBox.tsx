"use client";

// AgentInvokeBox — one-click agent invocation UI (FR-M04 / UC-01 / US-01).
//
// Flow:
//   1. Buyer enters a task prompt in the textarea.
//   2. Frontend POSTs to /api/v1/prismsettle/agent/invoke (FR-M11 backend proxy
//      bypasses CORS). The backend resolves the agent's A2A endpoint URL from
//      agent_registry, then forwards a JSON-RPC 2.0 envelope.
//   3. On success: the JSON-RPC `result` is pretty-printed for the buyer.
//   4. Sad path (failure / timeout / low score): the buyer sees [Retry] to
//      re-invoke the same agent, or [Switch Agent] to return to the
//      marketplace and pick a higher-reputation alternative.
//
// The invoke endpoint returns the upstream JSON-RPC response as-is (NOT
// wrapped in the {code,message,data} envelope), so we use a raw fetch
// instead of the shared `api.post` helper.

import { useCallback, useState } from "react";
import { motion, AnimatePresence } from "framer-motion";
import { toast } from "sonner";
import {
  Play,
  RefreshCw,
  Repeat,
  Shuffle,
  CheckCircle2,
  XCircle,
  AlertTriangle,
} from "lucide-react";
import Link from "next/link";
import { cn, formatScore } from "@/lib/utils";
import { CHAIN_NAME } from "@/lib/contracts";

type InvokeStatus = "idle" | "loading" | "success" | "error";

interface InvokeResult {
  // Pretty-printed JSON-RPC result (success) or error message (failure).
  text: string;
  // Whether the upstream returned a JSON-RPC error object.
  isRpcError: boolean;
  // Raw HTTP status from the backend proxy (non-200 = backend error).
  httpStatus: number;
}

interface Props {
  agentId: string;
  agentScore?: string;
  // Optional: override the default A2A method. "execute" is the common
  // default for task-oriented agents; "chat" for conversational ones.
  defaultMethod?: string;
  chainName?: string;
}

const DEFAULT_METHOD = "execute";
const INVOKE_TIMEOUT_MS = 30_000;

export function AgentInvokeBox({
  agentId,
  agentScore,
  defaultMethod = DEFAULT_METHOD,
  chainName = CHAIN_NAME,
}: Props) {
  const [prompt, setPrompt] = useState("");
  const [method, setMethod] = useState(defaultMethod);
  const [status, setStatus] = useState<InvokeStatus>("idle");
  const [result, setResult] = useState<InvokeResult | null>(null);

  const scoreNum = agentScore ? Number(BigInt(agentScore)) / 1e18 : null;
  const lowScore = scoreNum !== null && scoreNum < 0.3;

  const invoke = useCallback(async () => {
    if (!prompt.trim() || !agentId) return;
    setStatus("loading");
    setResult(null);

    const controller = new AbortController();
    const timer = setTimeout(() => controller.abort(), INVOKE_TIMEOUT_MS);

    try {
      const resp = await fetch(
        `/api/v1/prismsettle/agent/invoke?chainName=${encodeURIComponent(chainName)}`,
        {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({
            agent_id: agentId,
            method,
            params: { input: prompt.trim() },
          }),
          signal: controller.signal,
        },
      );

      const raw = await resp.text();
      let parsed: unknown;
      try {
        parsed = JSON.parse(raw);
      } catch {
        // Non-JSON response (e.g. HTML error page from a misconfigured endpoint).
        setResult({
          text: raw.slice(0, 500) || "(empty response body)",
          isRpcError: true,
          httpStatus: resp.status,
        });
        setStatus("error");
        return;
      }

      // Case 1: backend business error envelope {code, message, data}.
      const env = parsed as { code?: number; message?: string; jsonrpc?: string };
      if (typeof env.code === "number" && env.code !== 0 && !env.jsonrpc) {
        setResult({
          text: env.message ?? `backend error (code ${env.code})`,
          isRpcError: true,
          httpStatus: resp.status,
        });
        setStatus("error");
        return;
      }

      // Case 2: JSON-RPC 2.0 envelope {jsonrpc, result, error, id}.
      const rpc = parsed as {
        jsonrpc?: string;
        result?: unknown;
        error?: { code?: number; message?: string; data?: unknown };
      };
      if (rpc.jsonrpc) {
        if (rpc.error) {
          const errText = rpc.error.message
            ? `${rpc.error.message} (code ${rpc.error.code ?? "?"})`
            : JSON.stringify(rpc.error, null, 2);
          setResult({ text: errText, isRpcError: true, httpStatus: resp.status });
          setStatus("error");
        } else {
          setResult({
            text: JSON.stringify(rpc.result ?? null, null, 2),
            isRpcError: false,
            httpStatus: resp.status,
          });
          setStatus("success");
          toast.success("Agent invoked successfully");
        }
        return;
      }

      // Case 3: unknown shape — display raw.
      setResult({
        text: JSON.stringify(parsed, null, 2),
        isRpcError: !resp.ok,
        httpStatus: resp.status,
      });
      setStatus(resp.ok ? "success" : "error");
      if (resp.ok) {
        toast.success("Agent invoked successfully");
      } else {
        toast.error("Agent invocation failed");
      }
    } catch (e) {
      const aborted = (e as Error).name === "AbortError";
      setResult({
        text: aborted
          ? `Request timed out after ${INVOKE_TIMEOUT_MS / 1000}s. The agent may be offline or overloaded.`
          : `network error: ${(e as Error).message}`,
        isRpcError: true,
        httpStatus: 0,
      });
      setStatus("error");
      toast.error(
        aborted ? "Request timed out — agent may be offline" : "Network error",
      );
    } finally {
      clearTimeout(timer);
    }
  }, [prompt, agentId, method, chainName]);

  const canInvoke = prompt.trim().length > 0 && status !== "loading";

  return (
    <div className="rounded-xl border border-white/10 bg-prism-surface/40 p-5">
      {/* Header */}
      <div className="mb-3 flex items-center justify-between">
        <div className="flex items-center gap-2">
          <Play className="h-4 w-4 text-prism-accent" />
          <h2 className="text-sm font-medium text-white">Invoke Agent</h2>
          <span className="rounded bg-prism-accent/15 px-1.5 py-0.5 font-mono text-[10px] text-prism-accent">
            FR-M04
          </span>
        </div>
        {scoreNum !== null && (
          <span
            className={cn(
              "font-mono text-xs",
              lowScore ? "text-amber-400" : "text-emerald-400",
            )}
          >
            score {scoreNum.toFixed(4)}
          </span>
        )}
      </div>

      {/* Low-score warning (trust pre-check hint) */}
      {lowScore && (
        <div className="mb-3 flex items-start gap-2 rounded-lg border border-amber-500/30 bg-amber-500/5 p-2.5 text-xs text-amber-400">
          <AlertTriangle className="mt-0.5 h-3.5 w-3.5 shrink-0" />
          <span>
            This agent&apos;s score is below 0.3. Consider switching to a
            higher-reputation agent if the invocation fails.
          </span>
        </div>
      )}

      {/* Method + prompt input */}
      <div className="flex gap-2">
        <input
          value={method}
          onChange={(e) => setMethod(e.target.value)}
          placeholder="method"
          className="w-28 shrink-0 rounded-md border border-white/10 bg-black/30 px-3 py-2 font-mono text-xs text-white outline-none focus:border-prism-accent/50"
        />
        <textarea
          value={prompt}
          onChange={(e) => setPrompt(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === "Enter" && (e.metaKey || e.ctrlKey)) {
              e.preventDefault();
              invoke();
            }
          }}
          placeholder="Describe the task for this agent…  (⌘/Ctrl+Enter to invoke)"
          rows={3}
          className="min-w-0 flex-1 resize-none rounded-md border border-white/10 bg-black/30 px-3 py-2 text-sm text-white placeholder:text-white/30 outline-none focus:border-prism-accent/50"
        />
      </div>

      {/* Invoke button */}
      <div className="mt-3 flex items-center gap-2">
        <button
          type="button"
          onClick={invoke}
          disabled={!canInvoke}
          className={cn(
            "inline-flex items-center gap-1.5 rounded-md px-4 py-2 text-sm font-medium transition",
            canInvoke
              ? "bg-prism-accent text-white hover:bg-prism-accent/80"
              : "cursor-not-allowed bg-white/5 text-white/30",
          )}
        >
          {status === "loading" ? (
            <RefreshCw className="h-3.5 w-3.5 animate-spin" />
          ) : (
            <Play className="h-3.5 w-3.5" />
          )}
          {status === "loading" ? "Invoking…" : "Invoke"}
        </button>
        {status === "success" && (
          <button
            type="button"
            onClick={() => {
              setStatus("idle");
              setResult(null);
              setPrompt("");
            }}
            className="inline-flex items-center gap-1 rounded-md border border-white/20 px-3 py-2 text-sm text-white/70 hover:bg-white/5"
          >
            <Repeat className="h-3.5 w-3.5" /> New task
          </button>
        )}
      </div>

      {/* Result / Sad path */}
      <AnimatePresence mode="wait">
        {result && (
          <motion.div
            key={status}
            initial={{ opacity: 0, y: 8 }}
            animate={{ opacity: 1, y: 0 }}
            exit={{ opacity: 0, y: -8 }}
            transition={{ duration: 0.2 }}
            className="mt-4"
          >
            {status === "success" ? (
              <ResultPanel result={result} />
            ) : (
              <SadPathPanel result={result} onRetry={invoke} agentId={agentId} />
            )}
          </motion.div>
        )}
      </AnimatePresence>
    </div>
  );
}

// --- Success panel: pretty-printed JSON-RPC result ----------------------------

function ResultPanel({ result }: { result: InvokeResult }) {
  return (
    <div className="rounded-lg border border-emerald-500/30 bg-emerald-500/5 p-3">
      <div className="mb-2 flex items-center gap-1.5 text-sm text-emerald-400">
        <CheckCircle2 className="h-4 w-4" />
        <span className="font-medium">Agent response</span>
        <span className="ml-auto font-mono text-[10px] text-white/40">
          HTTP {result.httpStatus}
        </span>
      </div>
      <pre className="max-h-64 overflow-auto rounded-md bg-black/40 p-3 font-mono text-xs text-emerald-200/90">
        {result.text}
      </pre>
    </div>
  );
}

// --- Sad path: error display + retry / switch actions -------------------------

function SadPathPanel({
  result,
  onRetry,
  agentId,
}: {
  result: InvokeResult;
  onRetry: () => void;
  agentId: string;
}) {
  return (
    <div className="rounded-lg border border-red-500/30 bg-red-500/5 p-3">
      <div className="mb-2 flex items-center gap-1.5 text-sm text-red-400">
        <XCircle className="h-4 w-4" />
        <span className="font-medium">Invocation failed</span>
        {result.httpStatus > 0 && (
          <span className="ml-auto font-mono text-[10px] text-white/40">
            HTTP {result.httpStatus}
          </span>
        )}
      </div>
      <pre className="max-h-32 overflow-auto whitespace-pre-wrap rounded-md bg-black/40 p-3 font-mono text-xs text-red-200/90">
        {result.text}
      </pre>

      {/* Sad-path actions */}
      <div className="mt-3 flex flex-wrap gap-2">
        <button
          type="button"
          onClick={onRetry}
          className="inline-flex items-center gap-1.5 rounded-md border border-white/20 px-3 py-1.5 text-xs text-white/80 hover:bg-white/5"
        >
          <RefreshCw className="h-3 w-3" /> Retry same agent
        </button>
        <Link
          href="/agents"
          className="inline-flex items-center gap-1.5 rounded-md border border-prism-accent/40 px-3 py-1.5 text-xs text-prism-accent hover:bg-prism-accent/10"
        >
          <Shuffle className="h-3 w-3" /> Switch to another agent
        </Link>
      </div>
      <p className="mt-2 text-[10px] text-white/30">
        Tip: if retries keep failing, the agent&apos;s A2A endpoint may be offline.
        Switch to a higher-score agent from the marketplace.
      </p>
    </div>
  );
}
