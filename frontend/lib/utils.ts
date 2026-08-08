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

// USDC / WMON token contract addresses (all deployed generations).
// NOTE: v3-v5 MockUSDC are 6-decimals; v6 (0x83cb…) is 18-decimals (deployed
// with the MockERC20 default) even though its symbol is still "USDC".
const USDC_ADDRESSES = new Set([
  "0x252e44550f8b9997901e5540fc0e1da52ab099c6", // v3 MockUSDC (6)
  "0x2bb06a30d464ca8e62563081f024e6380f0eb70b", // v4 MockUSDC (6)
  "0x641419b3347523a26e828a550f4237547df29e0d", // v5 MockUSDC (6)
]);
// USDC-symbol tokens that are NOT 6-decimals (v6 deployed as 18).
const USDC_LABEL_ADDRESSES = new Set([
  "0x83cb612c10a27c09b7a5ab31b906560b880abd9c", // v6 MockUSDC (18)
]);
const WMON_ADDRESS = "0xfb8bf4c1cc7a94c73d209a149ea2abea852bc541";

// tokenDecimals returns the decimals for a payment token contract. USDC-style
// tokens use 6; everything else (WMON, native MON) uses 18.
export function tokenDecimals(addr: string | undefined): number {
  if (!addr) return 18;
  const a = addr.toLowerCase();
  if (USDC_ADDRESSES.has(a)) return 6;
  if (a === WMON_ADDRESS) return 18;
  return 18;
}

// tokenLabel returns the human symbol for a payment token contract.
export function tokenLabel(addr: string | undefined): string {
  if (!addr) return "tokens";
  const a = addr.toLowerCase();
  if (USDC_ADDRESSES.has(a) || USDC_LABEL_ADDRESSES.has(a)) return "USDC";
  if (a === WMON_ADDRESS) return "WMON";
  return "tokens";
}

// Agent display name + description derived from metadata capabilities. The
// registry metadata is a JSON string like
// {"endpointUrl":"...","capabilities":"smart_contract_audit"} — capabilities
// may be comma-separated. Fall back to generic labels for unknown/plain text.
const CAPABILITY_META: Record<string, { name: string; desc: string }> = {
  smart_contract_audit: {
    name: "Smart Contract Auditor",
    desc: "智能合约安全审计：漏洞扫描、gas 优化与修复建议",
  },
  smart_contract_development: {
    name: "Smart Contract Developer",
    desc: "智能合约开发与迭代：实现、测试与部署",
  },
  evaluation: {
    name: "Evaluator",
    desc: "任务质量评估与争议仲裁裁定，维护平台声誉体系",
  },
  defi: { name: "DeFi Analyst", desc: "DeFi 协议分析与策略研究" },
  data_labeling: { name: "Data Labeler", desc: "数据标注与清洗服务" },
  translation: { name: "Translator", desc: "多语言翻译与本地化服务" },
  coding: { name: "Coding Agent", desc: "通用编程与代码审查服务" },
  buyer: {
    name: "Task Buyer",
    desc: "任务发起与验收：创建任务、托管资金、验收交付物、发起争议",
  },
};

function capabilityOf(metadata: string | null | undefined): string {
  const m = (metadata ?? "").toLowerCase();
  // Try JSON capabilities field first.
  try {
    const parsed = JSON.parse(m);
    const caps = parsed?.capabilities;
    if (typeof caps === "string") {
      const first = caps.split(",")[0].trim();
      if (first) return first;
    }
  } catch {
    // plain-text metadata — fall through to keyword match below
  }
  for (const key of Object.keys(CAPABILITY_META)) {
    if (m.includes(key)) return key;
  }
  return "";
}

export function agentDisplayName(metadata: string | null | undefined): string {
  const cap = capabilityOf(metadata);
  return cap ? CAPABILITY_META[cap].name : "Agent";
}

export function agentDescription(metadata: string | null | undefined): string {
  const cap = capabilityOf(metadata);
  return cap ? CAPABILITY_META[cap].desc : "通用 Agent，能力待定";
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
export type AgentCategory = "DeFi" | "Data" | "Translation" | "Eval" | "Audit" | "Other";

export const AGENT_CATEGORIES: AgentCategory[] = ["DeFi", "Data", "Translation", "Eval", "Audit"];

export function agentCategory(metadata: string | null | undefined): AgentCategory {
  const m = (metadata ?? "").toLowerCase();
  if (m.includes("audit") || m.includes("auditor") || m.includes("security")) return "Audit";
  if (m.includes("defi")) return "DeFi";
  if (m.includes("data") || m.includes("label")) return "Data";
  if (m.includes("translat")) return "Translation";
  if (m.includes("eval")) return "Eval";
  return "Other";
}

// parseNaturalLanguage — keyword-based parser for the Create New Job quick
// input: extracts amount/currency/deadline/min-reputation and an agent
// capability tag from a plain-language job description (no LLM dependency).
export function parseNaturalLanguage(text: string): {
  agentTag?: string;
  amount?: string;
  currency?: "usdc" | "mon";
  deadlineIn?: number; // days from now
  minRep?: string;
} {
  const t = text.toLowerCase();
  const out: ReturnType<typeof parseNaturalLanguage> = {};

  // Currency first (USDC / MON / WMON)
  if (/\b(mon|wmon|eth)\b/.test(t)) out.currency = "mon";
  else if (/\b(usdc|u)\b/.test(t)) out.currency = "usdc";

  // Amount: number followed by optional currency word
  const amt = t.match(/(\d+(?:\.\d+)?)\s*(usdc|mon|wmon|u|m|美元|个)?/);
  if (amt && amt[1]) out.amount = amt[1];

  // Deadline: "N天/Nd/Nh/明天/后天/一周" (no \b after CJK chars)
  const days = t.match(/(\d+)\s*(天|日|day|days|d)/);
  if (days && days[1]) {
    out.deadlineIn = Number(days[1]);
  } else if (t.includes("明天")) {
    out.deadlineIn = 1;
  } else if (t.includes("后天")) {
    out.deadlineIn = 2;
  } else if (t.includes("一周") || t.includes("星期")) {
    out.deadlineIn = 7;
  }

  // Agent capability: map keywords to category tags
  const tagMap: Array<[RegExp, string]> = [
    [/(数据|标注|data|label)/, "data"],
    [/(翻译|translat)/, "translation"],
    [/(defi|交易|金融)/, "defi"],
    [/(审计|audit|安全)/, "audit"],
    [/(验证|eval|质检|qa)/, "eval"],
    [/(编码|编程|code|开发)/, "coding"],
  ];
  for (const [re, tag] of tagMap) {
    if (re.test(t)) {
      out.agentTag = tag;
      break;
    }
  }

  // Min reputation: decimal after 信誉/rep/reputation
  const rep = t.match(/(?:信誉|rep|reputation)[^\d]*(\d+(?:\.\d+)?)/);
  if (rep && rep[1]) out.minRep = rep[1];

  return out;
}
