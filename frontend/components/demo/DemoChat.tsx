"use client";

// DemoChat — chat-style visualization of the agent collaboration demo.
// Buyer bubbles left, provider right, evaluator center-purple, system
// centered-gray. Each agent message carries its on-chain tx link.

import { useEffect, useRef, useState } from "react";
import { CheckCircle2, ExternalLink, FileText, Loader2, ShieldCheck, XCircle } from "lucide-react";
import { keccak256 } from "viem";
import type { DemoMessageVO, DemoSessionVO } from "@/lib/prismsettle";
import { cn } from "@/lib/utils";

const ROLE_LABEL: Record<string, { name: string; cls: string }> = {
  buyer: { name: "Buyer", cls: "border-sky-400/40 bg-sky-400/10 text-sky-200" },
  provider: { name: "Provider", cls: "border-prism-accent/40 bg-prism-accent/10 text-prism-accent" },
  evaluator: { name: "Evaluator", cls: "border-violet-400/40 bg-violet-400/10 text-violet-200" },
  system: { name: "系统", cls: "border-white/10 bg-white/5 text-white/50" },
};

// Job state shown above the chat while the demo runs.
const STATE_LABEL: Record<string, string> = {
  created: "已创建",
  funded: "已托管",
  assigned: "已接单",
  submitted: "已提交",
  rejected: "已打回",
  disputed: "争议中",
  resolved: "已裁定",
  executed: "已结算",
  failed: "失败",
};

export function DemoChat({
  messages,
  session,
  busy,
}: {
  messages: DemoMessageVO[];
  session: DemoSessionVO | null;
  busy: boolean;
}) {
  const scrollRef = useRef<HTMLDivElement>(null);
  useEffect(() => {
    const el = scrollRef.current;
    if (el) el.scrollTop = el.scrollHeight;
  }, [messages.length]);

  const state = session?.state ?? "created";
  const showTyping = busy && !session?.result && state !== "failed";

  return (
    <div className="overflow-hidden rounded-xl border border-white/10 bg-prism-surface/40">
      {/* Status bar */}
      <div className="flex items-center justify-between border-b border-white/10 px-4 py-2.5">
        <div className="flex items-center gap-2 text-xs text-white/50">
          <span className={cn("h-2 w-2 rounded-full", state === "failed" ? "bg-red-400" : state === "executed" ? "bg-emerald-400" : "animate-pulse bg-prism-accent")} />
          当前状态：
          <span className="font-medium text-white/80">{STATE_LABEL[state] ?? state}</span>
        </div>
        {session?.job_id && (
          <span className="font-mono text-[10px] text-white/30">
            job {session.job_id.slice(0, 10)}…
          </span>
        )}
      </div>

      {/* Messages */}
      <div ref={scrollRef} className="max-h-[420px] space-y-3 overflow-y-auto p-4">
        {messages.length === 0 && (
          <div className="py-10 text-center text-sm text-white/30">
            等待演示开始…
          </div>
        )}
        {messages.map((m) => {
          const meta = ROLE_LABEL[m.role] ?? ROLE_LABEL.system;
          const isSystem = m.role === "system";
          return (
            <div
              key={m.id}
              className={cn(
                "flex",
                m.role === "buyer" && "justify-start",
                m.role === "provider" && "justify-end",
                isSystem && "justify-center",
                m.role === "evaluator" && "justify-center",
              )}
            >
              <div
                className={cn(
                  "max-w-[78%] rounded-lg border px-3 py-2 text-[13px] leading-relaxed",
                  meta.cls,
                  isSystem && "text-center text-[11px] text-white/45",
                  m.role === "evaluator" && "border-dashed",
                )}
              >
                {!isSystem && (
                  <div className="mb-1 flex items-center gap-2">
                    <span className="text-[10px] font-bold uppercase tracking-wide opacity-70">
                      {meta.name}
                    </span>
                    {m.tx_hash && (
                      <a
                        href={`https://testnet.monadexplorer.com/tx/${m.tx_hash}`}
                        target="_blank"
                        rel="noreferrer"
                        className="inline-flex items-center gap-0.5 font-mono text-[10px] opacity-60 hover:opacity-100"
                      >
                        {m.tx_hash.slice(0, 8)}… <ExternalLink className="h-2.5 w-2.5" />
                      </a>
                    )}
                  </div>
                )}
                <span>{m.content}</span>
                {m.report && <DeliverableBody report={m.report} chainHash={m.deliverable_hash} />}
              </div>
            </div>
          );
        })}
        {showTyping && (
          <div className="flex justify-center">
            <div className="flex items-center gap-2 rounded-full border border-white/10 bg-white/5 px-3 py-1.5 text-xs text-white/40">
              <Loader2 className="h-3 w-3 animate-spin" />
              Agents 协作进行中…
            </div>
          </div>
        )}
      </div>
    </div>
  );
}

// DeliverableBody — collapsible full-text view of the submitted deliverable
// (markdown rendered as preformatted text) plus an integrity check: the
// client recomputes keccak256(report) and compares it against the hash the
// provider submitted on-chain.
function DeliverableBody({ report, chainHash }: { report: string; chainHash?: string }) {
  const [open, setOpen] = useState(false);

  const localHash = keccak256(new TextEncoder().encode(report));
  const verified = !!chainHash && localHash.toLowerCase() === chainHash.toLowerCase();
  const shortChainHash = chainHash ? `${chainHash.slice(0, 10)}…` : "—";

  return (
    <div className="mt-2 border-t border-white/10 pt-2">
      <button
        onClick={() => setOpen((v) => !v)}
        className="inline-flex items-center gap-1.5 rounded-md border border-white/10 bg-black/25 px-2 py-1 text-[11px] text-white/60 transition-colors hover:border-prism-accent/40 hover:text-prism-accent"
      >
        <FileText className="h-3 w-3" />
        {open ? "收起交付物" : "查看完整交付物"}
        <span className="text-white/30">({report.length} 字)</span>
      </button>

      {/* Integrity check: local keccak vs the hash stored on-chain */}
      <div
        className={cn(
          "mt-2 flex items-center gap-1.5 rounded-md border px-2 py-1 text-[11px]",
          verified
            ? "border-emerald-500/30 bg-emerald-500/10 text-emerald-300"
            : "border-red-400/30 bg-red-400/10 text-red-300",
        )}
      >
        {verified ? (
          <ShieldCheck className="h-3 w-3" />
        ) : (
          <XCircle className="h-3 w-3" />
        )}
        <span>
          完整性校验：{verified ? "通过 —— 内容哈希与链上锚定一致，未被篡改" : "未通过 —— 内容与链上哈希不一致"}
        </span>
        {chainHash ? (
          <code className="ml-auto font-mono text-[10px] opacity-70">
            {shortChainHash}
          </code>
        ) : (
          <code className="ml-auto font-mono text-[10px] opacity-40">—</code>
        )}
      </div>

      {open && (
        <pre className="mt-2 max-h-72 overflow-auto whitespace-pre-wrap rounded-md border border-white/10 bg-black/30 p-3 text-[11px] leading-relaxed text-white/70">
          {report}
        </pre>
      )}
    </div>
  );
}
