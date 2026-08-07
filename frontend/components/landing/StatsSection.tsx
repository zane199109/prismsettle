"use client";

// StatsSection — 4 live metrics pulled from the API.
// Falls back to "--" when data isn't ready yet.

import { motion } from "framer-motion";
import { useAgents } from "@/hooks/useAgents";
import { useJobs } from "@/hooks/useJobs";

const STATS = [
  { key: "agents", label: "Agents", suffix: "" },
  { key: "jobs", label: "Active Jobs", suffix: "" },
  { key: "volume", label: "Total Volume", prefix: "$", suffix: "" },
  { key: "completed", label: "Completed", suffix: "" },
];

export function StatsSection() {
  const { agents, isValidating: agentsLoading } = useAgents({ size: 999 });
  const { jobs, isValidating: jobsLoading } = useJobs({ size: 999 });

  const activeJobs = jobs.filter(
    (j) => j.status !== "Completed" && j.status !== "Refunded",
  );
  const completedJobs = jobs.filter(
    (j) => j.status === "Completed",
  );

  const data: Record<string, string> = {
    agents: agents.length > 0 ? String(agents.length) : "--",
    jobs: activeJobs.length > 0 ? String(activeJobs.length) : "--",
    volume: completedJobs.length > 0 ? String(completedJobs.length) : "--",
    completed: completedJobs.length > 0 ? String(completedJobs.length) : "--",
  };

  return (
    <section className="border-t border-white/[0.04]">
      <div className="mx-auto max-w-5xl px-6 py-16">
        <div className="grid grid-cols-2 gap-px overflow-hidden rounded-xl border border-white/[0.06] bg-white/[0.04] md:grid-cols-4">
          {STATS.map((stat, i) => (
            <motion.div
              key={stat.key}
              initial={{ opacity: 0, y: 8 }}
              whileInView={{ opacity: 1, y: 0 }}
              viewport={{ once: true }}
              transition={{ duration: 0.3, delay: i * 0.05 }}
              className="bg-[#0a0a0f] px-6 py-8 text-center"
            >
              <div className="text-2xl font-bold tracking-tight text-white">
                {stat.prefix ?? ""}
                {data[stat.key]}
                {stat.suffix ?? ""}
              </div>
              <div className="mt-1 text-xs text-white/40">{stat.label}</div>
            </motion.div>
          ))}
        </div>
      </div>
    </section>
  );
}