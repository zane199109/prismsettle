"use client";

// LivePreview — pulls 3 top-scoring agents and 3 active jobs so visitors
// can see real activity without leaving the landing page. Clicking any
// card routes to the detail page.
//
// If the backend is offline (e.g. local dev without offchain service),
// the components show empty state — no error UI.

import Link from "next/link";
import { motion } from "framer-motion";
import { ArrowUpRight, Trophy, Briefcase } from "lucide-react";
import { Card, CardContent } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Skeleton } from "@/components/ui/skeleton";
import { useAgents } from "@/hooks/useAgents";
import { useJobs } from "@/hooks/useJobs";
import {
  cn,
  formatAgentId,
  formatScore,
  gradeColor,
  gradeFromScore,
} from "@/lib/utils";

export function LivePreview() {
  const { agents, isValidating: agentsLoading, isMounted } = useAgents({
    size: 3,
    intervalMs: 30000,
  });
  // Active = Pending (funded/assigned) or Submitted — anything not terminal.
  const { jobs, isValidating: jobsLoading, isMounted: jobsMounted } = useJobs({
    size: 3,
    intervalMs: 30000,
  });

  const topAgents = agents.slice(0, 3);
  const activeJobs = jobs
    .filter((j) => j.status !== "Completed" && j.status !== "Refunded")
    .slice(0, 3);

  return (
    <section className="border-t border-white/5 py-24">
      <div className="mx-auto max-w-7xl px-6">
        <div className="mb-12 max-w-2xl">
          <div className="mb-2 text-sm font-mono uppercase tracking-wider text-prism-accent">
            Live preview
          </div>
          <h2 className="text-3xl font-bold tracking-tight sm:text-4xl">
            What&apos;s happening on-chain
          </h2>
          <p className="mt-2 text-white/60">
            Real agents with real stakes. Real jobs with real funding.
          </p>
        </div>

        <div className="grid gap-8 lg:grid-cols-2">
          {/* Top Agents */}
          <div>
            <div className="mb-4 flex items-center gap-2">
              <Trophy className="h-4 w-4 text-amber-400" />
              <h3 className="text-sm font-semibold uppercase tracking-wider text-white/70">
                Top Agents
              </h3>
              <Link
                href="/agents"
                className="ml-auto text-xs text-prism-accent hover:underline"
              >
                View all →
              </Link>
            </div>
            <div className="space-y-3">
              {isMounted && agentsLoading && topAgents.length === 0 ? (
                Array.from({ length: 3 }).map((_, i) => (
                  <Skeleton key={i} className="h-20 rounded-xl" />
                ))
              ) : topAgents.length === 0 ? (
                <Card className="border-dashed border-white/20 bg-prism-surface/30">
                  <CardContent className="py-8 text-center text-sm text-white/40">
                    No agents registered yet.
                  </CardContent>
                </Card>
              ) : (
                topAgents.map((agent, i) => {
                  const grade = gradeFromScore(agent.score);
                  return (
                    <motion.div
                      key={agent.agent_id}
                      initial={{ opacity: 0, x: -10 }}
                      whileInView={{ opacity: 1, x: 0 }}
                      viewport={{ once: true }}
                      transition={{ duration: 0.3, delay: i * 0.05 }}
                    >
                      <Link href={`/agents/${encodeURIComponent(agent.agent_id)}`}>
                        <Card className="border-white/10 bg-prism-surface/40 hover:border-prism-accent/40 hover:bg-prism-surface/70 transition-colors">
                          <CardContent className="flex items-center gap-4 p-4">
                            <div className="flex h-10 w-10 shrink-0 items-center justify-center rounded-full bg-prism-accent/15 font-mono text-sm font-bold text-prism-accent">
                              #{i + 1}
                            </div>
                            <div className="min-w-0 flex-1">
                              <div className="truncate font-mono text-sm text-white">
                                {formatAgentId(agent.agent_id)}
                              </div>
                              <div className="mt-0.5 text-xs text-white/40">
                                score {formatScore(agent.score)}
                              </div>
                            </div>
                            <Badge
                              variant="outline"
                              className={cn("font-mono text-xs", gradeColor(grade))}
                            >
                              {grade}
                            </Badge>
                            <ArrowUpRight className="h-4 w-4 text-white/40" />
                          </CardContent>
                        </Card>
                      </Link>
                    </motion.div>
                  );
                })
              )}
            </div>
          </div>

          {/* Active Jobs */}
          <div>
            <div className="mb-4 flex items-center gap-2">
              <Briefcase className="h-4 w-4 text-prism-glow" />
              <h3 className="text-sm font-semibold uppercase tracking-wider text-white/70">
                Active Jobs
              </h3>
              <Link
                href="/jobs"
                className="ml-auto text-xs text-prism-accent hover:underline"
              >
                View all →
              </Link>
            </div>
            <div className="space-y-3">
              {jobsMounted && jobsLoading && activeJobs.length === 0 ? (
                Array.from({ length: 3 }).map((_, i) => (
                  <Skeleton key={i} className="h-20 rounded-xl" />
                ))
              ) : activeJobs.length === 0 ? (
                <Card className="border-dashed border-white/20 bg-prism-surface/30">
                  <CardContent className="py-8 text-center text-sm text-white/40">
                    No active jobs. Be the first to post one.
                  </CardContent>
                </Card>
              ) : (
                activeJobs.map((job, i) => (
                  <motion.div
                    key={job.job_id}
                    initial={{ opacity: 0, x: 10 }}
                    whileInView={{ opacity: 1, x: 0 }}
                    viewport={{ once: true }}
                    transition={{ duration: 0.3, delay: i * 0.05 }}
                  >
                    <Link href={`/jobs/${encodeURIComponent(job.job_id)}`}>
                      <Card className="border-white/10 bg-prism-surface/40 hover:border-prism-glow/40 hover:bg-prism-surface/70 transition-colors">
                        <CardContent className="flex items-center gap-4 p-4">
                          <div className="flex h-10 w-10 shrink-0 items-center justify-center rounded-full bg-prism-glow/15 font-mono text-xs font-bold text-prism-glow">
                            #{job.shard_id}
                          </div>
                          <div className="min-w-0 flex-1">
                            <div className="truncate font-mono text-sm text-white">
                              {formatAgentId(job.job_id)}
                            </div>
                            <div className="mt-0.5 flex flex-wrap gap-x-3 gap-y-0.5 text-xs text-white/40">
                              <span>creator {formatAgentId(job.creator)}</span>
                              {job.amount && (
                                <span>{(Number(job.amount) / 1e6).toFixed(2)} USDC</span>
                              )}
                              {job.provider && (
                                <span>→ {formatAgentId(job.provider)}</span>
                              )}
                            </div>
                          </div>
                          <Badge variant="secondary" className="font-mono text-xs">
                            {job.status}
                          </Badge>
                          <ArrowUpRight className="h-4 w-4 text-white/40" />
                        </CardContent>
                      </Card>
                    </Link>
                  </motion.div>
                ))
              )}
            </div>
          </div>
        </div>
      </div>
    </section>
  );
}
