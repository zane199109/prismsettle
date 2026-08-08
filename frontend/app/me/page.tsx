"use client";

// User profile page (P1-5). Aggregates all on-chain activity for the
// connected wallet: registered agents, created jobs, submitted validations,
// and slash history. PRD assumes wallet-as-identity (no separate account);
// this page surfaces everything tied to `address`.
//
// Filters (all client-side since backend doesn't expose `?owner=` / `?creator=`):
//   - Agents:    owner === address
//   - Jobs:      creator === address
//   - Validations: events.from === address && event_type=PRISM_VALIDATION_SUBMITTED
//   - Slashes:    events.to === address && event_type=PRISM_SLASHED
//
// Demo mode: when no wallet is connected, judges can click "View as demo
// user" to preview the page using a fixed demo address (DEMO_ADDRESSES.alice).
// This bypasses wagmi and just feeds the mock fallback directly.

import { useState } from "react";
import Link from "next/link";
import { User, Wallet, ArrowRight, ExternalLink, Sparkles } from "lucide-react";
import { useAccount } from "wagmi";

import { useAgents } from "@/hooks/useAgents";
import { useJobs } from "@/hooks/useJobs";
import { useEvents } from "@/hooks/useEvents";
import { GrabAttemptsList } from "@/components/job/GrabAttemptsList";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { DEMO_ADDRESSES } from "@/lib/mock-data";
import {
  cn,
  formatAgentId,
  formatScore,
  formatTime,
  gradeColor,
  gradeFromScore,
} from "@/lib/utils";

export default function MyProfilePage() {
  const { address } = useAccount();
  const [demoMode, setDemoMode] = useState(false);

  // Effective address: connected wallet takes precedence; otherwise fall back
  // to demo address when demoMode is toggled on.
  const effectiveAddress = address ?? (demoMode ? DEMO_ADDRESSES.alice : undefined);

  return (
    <div className="min-h-screen">
      
      <main className="mx-auto max-w-7xl px-6 py-8">
        <div className="mb-6">
          <h1 className="flex items-center gap-2 text-2xl font-bold tracking-tight">
            <User className="h-6 w-6 text-prism-accent" />
            My Profile
          </h1>
          <p className="mt-1 text-sm text-white/60">
            All on-chain activity tied to your wallet. PRD assumes wallet-as-identity.
          </p>
        </div>

        {!effectiveAddress ? (
          <Card className="border-dashed border-white/20 bg-prism-surface/30 py-16 text-center">
            <CardContent className="py-0">
              <Wallet className="mx-auto h-8 w-8 text-white/40" />
              <p className="mt-3 text-sm text-white/60">Connect your wallet to view your profile.</p>
              <div className="mt-4 flex justify-center">
                <Button
                  variant="outline"
                  size="sm"
                  onClick={() => setDemoMode(true)}
                  className="border-prism-accent/40 text-prism-accent hover:bg-prism-accent/10"
                >
                  <Sparkles className="mr-1.5 h-3.5 w-3.5" />
                  View as demo user (Alice)
                </Button>
              </div>
              <p className="mt-2 text-[10px] text-white/30">
                Demo mode uses a fixed mock wallet to preview the page.
              </p>
            </CardContent>
          </Card>
        ) : (
          <ProfileContent
            address={effectiveAddress}
            isDemo={demoMode && !address}
            onExitDemo={() => setDemoMode(false)}
          />
        )}
      </main>
    </div>
  );
}

function ProfileContent({
  address,
  isDemo,
  onExitDemo,
}: {
  address: string;
  isDemo?: boolean;
  onExitDemo?: () => void;
}) {
  const { agents, isValidating: agentsLoading } = useAgents({ size: 100, intervalMs: 12000, userAddress: address });
  const { jobs, isValidating: jobsLoading } = useJobs({ size: 100, intervalMs: 12000, userAddress: address });
  const { events: validations, isValidating: valLoading } = useEvents({
    eventType: "PRISM_VALIDATION_SUBMITTED",
    size: 100,
    intervalMs: 12000,
    userAddress: address,
  });
  const { events: slashes, isValidating: slashLoading } = useEvents({
    eventType: "PRISM_SLASHED",
    size: 100,
    intervalMs: 12000,
    userAddress: address,
  });

  const lower = address.toLowerCase();
  const myAgents = agents.filter((a) => a.owner.toLowerCase() === lower);
  const myJobs = jobs.filter((j) => j.creator.toLowerCase() === lower);
  const myValidations = validations.filter((e) => e.from.toLowerCase() === lower);
  const mySlashes = slashes.filter((e) => e.to.toLowerCase() === lower);

  return (
    <div className="space-y-6">
      {/* Demo mode banner — shown only when previewing without a connected wallet */}
      {isDemo && (
        <div className="flex items-center justify-between rounded-lg border border-prism-accent/30 bg-prism-accent/5 px-4 py-2.5">
          <div className="flex items-center gap-2 text-xs text-prism-accent">
            <Sparkles className="h-3.5 w-3.5" />
            <span>Demo mode — viewing as Alice. Connect your wallet to see real activity.</span>
          </div>
          <Button variant="ghost" size="sm" onClick={onExitDemo} className="h-7 text-xs text-white/60 hover:text-white">
            Exit demo
          </Button>
        </div>
      )}

      {/* Wallet summary */}
      <Card className="border-white/10 bg-prism-surface/40">
        <CardContent className="flex flex-wrap items-center justify-between gap-4 p-5">
          <div>
            <div className="text-[10px] uppercase tracking-wider text-white/40">Connected Wallet</div>
            <div className="mt-1 font-mono text-sm text-prism-accent">{address}</div>
          </div>
          <div className="flex gap-3 text-center">
            <Metric label="Agents" value={myAgents.length} />
            <Metric label="Jobs" value={myJobs.length} />
            <Metric label="Validations" value={myValidations.length} />
            <Metric label="Slashes" value={mySlashes.length} tone="text-red-400" />
          </div>
        </CardContent>
      </Card>

      {/* My Agents */}
      <Card className="border-white/10 bg-prism-surface/40">
        <CardHeader className="pb-3">
          <CardTitle className="text-base">My Registered Agents</CardTitle>
        </CardHeader>
        <CardContent>
          {agentsLoading && myAgents.length === 0 ? (
            <Skeleton className="h-20 rounded-md" />
          ) : myAgents.length === 0 ? (
            <EmptyHint text="You haven't registered any agents." cta={{ href: "/agents", label: "Register an agent →" }} />
          ) : (
            <ul className="space-y-2">
              {myAgents.map((a) => {
                const grade = gradeFromScore(a.score);
                return (
                  <li key={a.agent_id} className="flex items-center justify-between gap-3 rounded-md border border-white/5 bg-black/20 px-3 py-2">
                    <div className="min-w-0">
                      <div className="flex items-center gap-2">
                        <span className="font-mono text-xs text-white">{formatAgentId(a.agent_id)}</span>
                        <Badge variant="outline" className={cn("text-[9px] font-bold", gradeColor(grade))}>
                          {grade}
                        </Badge>
                      </div>
                      <div className="mt-0.5 truncate text-[10px] text-white/40">
                        {a.metadata || "no metadata"}
                      </div>
                    </div>
                    <div className="text-right">
                      <div className="font-mono text-sm font-bold text-white">{formatScore(a.score)}</div>
                      <Link href={`/agents/${encodeURIComponent(a.agent_id)}`} className="inline-flex items-center gap-0.5 text-[10px] text-prism-accent hover:underline">
                        view <ArrowRight className="h-2.5 w-2.5" />
                      </Link>
                    </div>
                  </li>
                );
              })}
            </ul>
          )}
        </CardContent>
      </Card>

      {/* My Jobs */}
      <Card className="border-white/10 bg-prism-surface/40">
        <CardHeader className="pb-3">
          <CardTitle className="text-base">My Jobs</CardTitle>
        </CardHeader>
        <CardContent>
          {jobsLoading && myJobs.length === 0 ? (
            <Skeleton className="h-20 rounded-md" />
          ) : myJobs.length === 0 ? (
            <EmptyHint text="You haven't created any jobs." cta={{ href: "/jobs", label: "Browse jobs →" }} />
          ) : (
            <ul className="space-y-2">
              {myJobs.map((j) => (
                <li key={j.job_id} className="flex items-center justify-between gap-3 rounded-md border border-white/5 bg-black/20 px-3 py-2">
                  <div className="min-w-0">
                    <div className="font-mono text-xs text-white">Job #{j.job_id}</div>
                    <div className="mt-0.5 flex flex-wrap gap-x-3 gap-y-0.5 text-[10px] text-white/40">
                      <span>created {formatTime(j.created_at)}</span>
                      <span>shard #{j.shard_id}</span>
                      {j.amount && <span>{(Number(j.amount) / 1e18).toFixed(2)} {j.token === undefined ? "" : j.token.startsWith("0x252e") ? "USDC" : j.token.startsWith("0xFb8b") ? "WMON" : ""}</span>}
                      {j.provider && <span>→ provider {formatAgentId(j.provider)}</span>}
                    </div>
                  </div>
                  <div className="flex items-center gap-2">
                    <Badge variant="outline" className="text-[9px] font-bold text-white/70">
                      {j.status}
                    </Badge>
                    <Link href={`/jobs/${encodeURIComponent(j.job_id)}`} className="inline-flex items-center gap-0.5 text-[10px] text-prism-accent hover:underline">
                      view <ArrowRight className="h-2.5 w-2.5" />
                    </Link>
                  </div>
                </li>
              ))}
            </ul>
          )}
        </CardContent>
      </Card>

      {/* My Validations */}
      <div className="grid gap-6 lg:grid-cols-2">
        <Card className="border-white/10 bg-prism-surface/40">
          <CardHeader className="pb-3">
            <CardTitle className="text-base">My Validations</CardTitle>
          </CardHeader>
          <CardContent>
            {valLoading && myValidations.length === 0 ? (
              <Skeleton className="h-20 rounded-md" />
            ) : myValidations.length === 0 ? (
              <EmptyHint text="No validations submitted yet." cta={{ href: "/validator", label: "Submit a validation →" }} />
            ) : (
              <ul className="space-y-2">
                {myValidations.slice(0, 8).map((v) => (
                  <li key={`${v.tx_hash}-${v.log_index ?? 0}`} className="flex items-center justify-between gap-3 rounded-md border border-white/5 bg-black/20 px-3 py-2">
                    <div className="min-w-0">
                      <div className="font-mono text-xs text-white">Agent #{v.to}</div>
                      <div className="mt-0.5 text-[10px] text-white/40">{formatTime(v.block_time)}</div>
                    </div>
                    <div className="text-right">
                      <div className="font-mono text-sm font-bold text-blue-400">{formatScore(v.value)}</div>
                      <div className="text-[9px] text-white/40">source={v.symbol ?? "0"}</div>
                    </div>
                  </li>
                ))}
              </ul>
            )}
          </CardContent>
        </Card>

        {/* My Slashes */}
        <Card className="border-white/10 bg-prism-surface/40">
          <CardHeader className="pb-3">
            <CardTitle className="text-base">My Slashes</CardTitle>
          </CardHeader>
          <CardContent>
            {slashLoading && mySlashes.length === 0 ? (
              <Skeleton className="h-20 rounded-md" />
            ) : mySlashes.length === 0 ? (
              <EmptyHint text="No slash events. Keep up the good work." />
            ) : (
              <ul className="space-y-2">
                {mySlashes.slice(0, 8).map((s) => (
                  <li key={`${s.tx_hash}-${s.log_index ?? 0}`} className="flex items-center justify-between gap-3 rounded-md border border-red-500/20 bg-red-500/5 px-3 py-2">
                    <div className="min-w-0">
                      <div className="font-mono text-xs text-white">
                        slashed by {formatAgentId(s.from)}
                      </div>
                      <div className="mt-0.5 text-[10px] text-white/40">{formatTime(s.block_time)}</div>
                    </div>
                    <div className="text-right">
                      <div className="font-mono text-sm font-bold text-red-400">
                        -{formatScore(s.value)}
                      </div>
                      <a
                        href={`https://testnet.monadexplorer.com/tx/${s.tx_hash}`}
                        target="_blank"
                        rel="noreferrer"
                        className="inline-flex items-center gap-0.5 text-[10px] text-white/40 hover:text-white"
                      >
                        tx <ExternalLink className="h-2.5 w-2.5" />
                      </a>
                    </div>
                  </li>
                ))}
              </ul>
            )}
          </CardContent>
        </Card>

        {/* My grab attempts — why my agent won or lost job competitions */}
        <Card className="border-white/10 bg-prism-surface/40">
          <CardHeader className="pb-3">
            <CardTitle className="text-base">我的抢单记录</CardTitle>
          </CardHeader>
          <CardContent>
            {myAgents.length === 0 ? (
              <EmptyHint text="还没有注册 agent —— 注册后 agent 服务的抢单尝试会显示在这里。" />
            ) : (
              <GrabAttemptsList agentId={myAgents[0].agent_id} title="" />
            )}
          </CardContent>
        </Card>
      </div>
    </div>
  );
}

function Metric({ label, value, tone }: { label: string; value: number; tone?: string }) {
  return (
    <div className="px-3">
      <div className={cn("font-mono text-2xl font-bold", tone ?? "text-white")}>{value}</div>
      <div className="text-[10px] uppercase tracking-wider text-white/40">{label}</div>
    </div>
  );
}

function EmptyHint({ text, cta }: { text: string; cta?: { href: string; label: string } }) {
  return (
    <div className="py-6 text-center">
      <p className="text-xs text-white/40">{text}</p>
      {cta && (
        <Link href={cta.href} className="mt-2 inline-flex items-center gap-1 text-xs text-prism-accent hover:underline">
          {cta.label}
        </Link>
      )}
    </div>
  );
}
