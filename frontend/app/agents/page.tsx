"use client";

// Agent Marketplace — browse all registered agents, sorted by reputation
// score with A/B/C/D grade chips and free-text tag filter.
// DEV-PLAN §Phase 8 任务 8.2.

import { useMemo, useState } from "react";
import Link from "next/link";
import { Search, ArrowDownWideNarrow, UserPlus } from "lucide-react";
import { PageHeader } from "@/components/PageHeader";
import { AgentRegisterForm } from "@/components/agent/AgentRegisterForm";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { useAgents } from "@/hooks/useAgents";
import {
  cn,
  formatAgentId,
  formatScore,
  gradeColor,
  gradeFromScore,
  agentCategory,
  AGENT_CATEGORIES,
} from "@/lib/utils";
import type { AgentVO } from "@/lib/types";

type SortKey = "score" | "recent";
type CategoryFilter = "All" | (typeof AGENT_CATEGORIES)[number];

export default function AgentMarketplacePage() {
  const [query, setQuery] = useState("");
  const [sort, setSort] = useState<SortKey>("score");
  const [category, setCategory] = useState<CategoryFilter>("All");
  const { agents, total, isValidating } = useAgents({ size: 50, intervalMs: 10000 });

  const filtered = useMemo(() => {
    const q = query.trim().toLowerCase();
    const list = agents.filter((a) => {
      if (category !== "All" && agentCategory(a.metadata) !== category) return false;
      if (!q) return true;
      return (
        a.agent_id.toLowerCase().includes(q) ||
        a.metadata.toLowerCase().includes(q) ||
        a.endpoint.toLowerCase().includes(q)
      );
    });
    const sorted = [...list].sort((a, b) => {
      if (sort === "score") {
        // Higher score first; non-numeric falls back to 0.
        const sa = Number(BigInt(a.score || "0")) / 1e18;
        const sb = Number(BigInt(b.score || "0")) / 1e18;
        return sb - sa;
      }
      // Recent: higher block_number first.
      return (b.block_number ?? 0) - (a.block_number ?? 0);
    });
    return sorted;
  }, [agents, query, sort, category]);

  return (
    <div className="min-h-screen">
      <PageHeader />
      <main id="main" className="mx-auto max-w-7xl px-6 py-8">
        <div className="mb-6 flex flex-wrap items-end justify-between gap-3">
          <div>
            <h1 className="text-2xl font-bold tracking-tight">Agent Marketplace</h1>
            <p className="mt-1 text-sm text-white/60">
              {total} registered agents · stake-weighted reputation
            </p>
          </div>
          <div className="flex items-center gap-2">
            <ArrowDownWideNarrow className="h-4 w-4 text-white/40" />
            <Select value={sort} onValueChange={(v) => setSort(v as SortKey)}>
              <SelectTrigger className="w-[180px] bg-black/30 border-white/10">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="score">Sort: Score</SelectItem>
                <SelectItem value="recent">Sort: Recent</SelectItem>
              </SelectContent>
            </Select>
          </div>
        </div>

        <div className="relative mb-4">
          <Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-white/40" />
          <Input
            type="text"
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            placeholder="Filter by agent id, metadata, or endpoint…"
            className="pl-10 bg-black/30 border-white/10"
          />
        </div>

        {/* Capability filter — PRD FR-M02 (DeFi / Data / Translation / Eval). */}
        <div className="mb-4 flex flex-wrap items-center gap-2">
          <span className="text-xs text-white/40">capability:</span>
          <CategoryChip
            label="All"
            active={category === "All"}
            onClick={() => setCategory("All")}
          />
          {AGENT_CATEGORIES.map((cat) => (
            <CategoryChip
              key={cat}
              label={cat}
              active={category === cat}
              onClick={() => setCategory(cat)}
            />
          ))}
        </div>

        {isValidating && filtered.length === 0 ? (
          <SkeletonGrid />
        ) : filtered.length === 0 ? (
          <EmptyState />
        ) : (
          <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
            {filtered.map((agent) => (
              <AgentCard key={agent.agent_id} agent={agent} />
            ))}
          </div>
        )}

        {/* 8.8: Agent register form (FR-M06) */}
        <Card className="mt-8 border-white/10 bg-prism-surface/40">
          <details id="register-agent-form">
            <summary className="flex cursor-pointer items-center gap-2 p-5 text-sm font-medium text-white">
              <UserPlus className="h-4 w-4 text-prism-accent" />
              Register a new agent (FR-M06)
            </summary>
            <div className="border-t border-white/10 p-5">
              <AgentRegisterForm />
            </div>
          </details>
        </Card>
      </main>
    </div>
  );
}

function AgentCard({ agent }: { agent: AgentVO }) {
  const grade = gradeFromScore(agent.score);
  const score = formatScore(agent.score);
  return (
    <Link
      href={`/agents/${encodeURIComponent(agent.agent_id)}`}
      className="group block"
    >
      <Card className="border-white/10 bg-prism-surface/40 transition-colors hover:border-prism-accent/50 hover:bg-prism-surface/70">
        <CardContent className="p-5">
          <div className="flex items-start justify-between">
            <div className="min-w-0">
              <div className="truncate font-mono text-sm text-white">
                {formatAgentId(agent.agent_id)}
              </div>
              <div className="mt-1 truncate text-xs text-white/40">
                owner {formatAgentId(agent.owner)}
              </div>
            </div>
            <Badge
              variant="outline"
              className={cn("font-mono text-xs font-bold", gradeColor(grade))}
            >
              {grade}
            </Badge>
          </div>
          <div className="mt-4 flex items-baseline gap-2">
            <span className="font-mono text-2xl font-bold text-white">{score}</span>
            <span className="text-xs text-white/40">reputation score</span>
          </div>
          {agent.endpoint && (
            <div className="mt-3 truncate text-xs text-prism-accent/80">
              ↗ {agent.endpoint}
            </div>
          )}
          <div className="mt-3 text-[11px] text-white/30">
            registered at block {agent.block_number}
          </div>
        </CardContent>
      </Card>
    </Link>
  );
}

function SkeletonGrid() {
  return (
    <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
      {Array.from({ length: 6 }).map((_, i) => (
        <Skeleton key={i} className="h-40 rounded-xl" />
      ))}
    </div>
  );
}

function EmptyState() {
  return (
    <Card className="border-dashed border-white/20 bg-prism-surface/30 py-16 text-center">
      <CardContent className="py-0">
        <p className="text-sm text-white/60">No agents registered yet.</p>
        <Button
          className="mt-4"
          onClick={() => {
            const el = document.getElementById("register-agent-form");
            if (el) {
              (el as HTMLDetailsElement).open = true;
              el.scrollIntoView({ behavior: "smooth" });
            }
          }}
        >
          Register the first agent →
        </Button>
      </CardContent>
    </Card>
  );
}

function CategoryChip({
  label,
  active,
  onClick,
}: {
  label: string;
  active: boolean;
  onClick: () => void;
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      className={cn(
        "rounded-full border px-3 py-1 text-xs font-medium transition-colors",
        active
          ? "border-prism-accent bg-prism-accent/20 text-prism-accent"
          : "border-white/10 bg-black/20 text-white/60 hover:text-white",
      )}
    >
      {label}
    </button>
  );
}
