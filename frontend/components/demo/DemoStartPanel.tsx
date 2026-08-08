"use client";

// DemoStartPanel — the user-facing kickoff for the chat-style collaboration
// demo. Fills in the job title/description/amount/agent, then the orchestrator
// takes over (fund → grab → submit → reject loop → arbitration → settle).

import { useState } from "react";
import { Play, Sparkles, Loader2 } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { PAYMENT_TOKEN_ADDRESS } from "@/lib/contracts";
import { cn } from "@/lib/utils";

const AGENT_OPTIONS = [
  { value: "senior", label: "Senior Auditor (0.90)" },
  { value: "junior", label: "Junior Auditor (0.80)" },
  { value: "rookie", label: "Rookie Auditor (0.70)" },
];

export function DemoStartPanel({
  onStart,
  busy,
  error,
}: {
  onStart: (params: { title: string; description: string; amount: string; provider_agent: string; scenario: string }) => Promise<void>;
  busy: boolean;
  error: string | null;
}) {
  const [title, setTitle] = useState("智能合约安全审计");
  const [description, setDescription] = useState("检查重入漏洞、权限控制与 gas 优化，输出审计报告");
  const [amount, setAmount] = useState("5");
  const [agent, setAgent] = useState("senior");
  const [scenario, setScenario] = useState<"arbitration" | "direct">("arbitration");

  const canStart = title.trim() && Number(amount) > 0 && !busy && Boolean(PAYMENT_TOKEN_ADDRESS);

  return (
    <div className="rounded-xl border border-white/10 bg-prism-surface/40 p-5">
      <div className="mb-4 flex items-center gap-2">
        <Sparkles className="h-4 w-4 text-prism-accent" />
        <h3 className="text-sm font-medium text-white">实时协作演示</h3>
        <span className="text-[11px] text-white/30">
          Buyer 创建任务后，Provider / Buyer / Evaluator 三个 Agent 自动对话协作直至结算
        </span>
      </div>

      <div className="grid gap-3 sm:grid-cols-2">
        <label className="block">
          <span className="mb-1 block text-xs text-white/50">任务标题</span>
          <Input
            value={title}
            onChange={(e) => setTitle(e.target.value)}
            className="bg-black/30 border-white/10"
            placeholder="智能合约安全审计"
          />
        </label>
        <label className="block">
          <span className="mb-1 block text-xs text-white/50">金额 (USDC)</span>
          <Input
            value={amount}
            onChange={(e) => setAmount(e.target.value)}
            type="number"
            min="1"
            className="bg-black/30 border-white/10"
          />
        </label>
        <label className="block sm:col-span-2">
          <span className="mb-1 block text-xs text-white/50">任务描述</span>
          <Input
            value={description}
            onChange={(e) => setDescription(e.target.value)}
            className="bg-black/30 border-white/10"
          />
        </label>
        <label className="block sm:col-span-2">
          <span className="mb-1 block text-xs text-white/50">接单 Agent</span>
          <div className="flex flex-wrap gap-2">
            {AGENT_OPTIONS.map((o) => (
              <button
                key={o.value}
                type="button"
                onClick={() => setAgent(o.value)}
                className={cn(
                  "rounded-lg border px-3 py-1.5 text-xs transition-colors",
                  agent === o.value
                    ? "border-prism-accent/60 bg-prism-accent/15 text-prism-accent"
                    : "border-white/10 bg-black/20 text-white/50 hover:border-white/25",
                )}
              >
                {o.label}
              </button>
            ))}
          </div>
        </label>
        <label className="block sm:col-span-2">
          <span className="mb-1 block text-xs text-white/50">演示场景</span>
          <div className="flex flex-wrap gap-2">
            <button
              type="button"
              onClick={() => setScenario("arbitration")}
              className={cn(
                "rounded-lg border px-3 py-1.5 text-xs transition-colors",
                scenario === "arbitration"
                  ? "border-prism-accent/60 bg-prism-accent/15 text-prism-accent"
                  : "border-white/10 bg-black/20 text-white/50 hover:border-white/25",
              )}
            >
              ⚖ 争议仲裁（打回 ×2 → 仲裁 → 结算）
            </button>
            <button
              type="button"
              onClick={() => setScenario("direct")}
              className={cn(
                "rounded-lg border px-3 py-1.5 text-xs transition-colors",
                scenario === "direct"
                  ? "border-prism-accent/60 bg-prism-accent/15 text-prism-accent"
                  : "border-white/10 bg-black/20 text-white/50 hover:border-white/25",
              )}
            >
              ✅ 直接完成（交付 → 验收 → 结算）
            </button>
          </div>
        </label>
      </div>

      {error && (
        <p className="mt-3 rounded-md border border-red-400/30 bg-red-400/10 px-3 py-2 text-xs text-red-300">
          {error}
        </p>
      )}

      <div className="mt-4 flex items-center gap-3">
        <Button
          disabled={!canStart}
          onClick={() =>
            onStart({
              title: title.trim(),
              description: description.trim(),
              amount: String(Math.round(Number(amount) * 1e18)),
              provider_agent: agent,
              scenario,
            })
          }
          className="bg-prism-accent text-white hover:bg-prism-accent/90"
        >
          {busy ? <Loader2 className="mr-2 h-4 w-4 animate-spin" /> : <Play className="mr-2 h-4 w-4" />}
          开始演示
        </Button>
        <span className="text-[11px] text-white/30">
          {scenario === "arbitration"
            ? "将创建真实链上任务并自动走完 打回×2 → 仲裁 → 结算"
            : "将创建真实链上任务并自动走完 交付 → 验收 → 结算（无争议）"}
        </span>
      </div>
    </div>
  );
}
