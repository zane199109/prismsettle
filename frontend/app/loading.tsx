// Route-level loading skeleton — shown by Next.js App Router while the
// page chunk is loading. Mirrors the dashboard layout so the transition
// feels instant instead of flashing white.

import { Skeleton } from "@/components/ui/skeleton";

export default function Loading() {
  return (
    <main className="mx-auto max-w-7xl px-6 py-8">
      {/* Header */}
      <div className="flex items-center justify-between border-b border-white/10 pb-6">
        <div>
          <Skeleton className="h-8 w-40" />
          <Skeleton className="mt-2 h-4 w-64" />
        </div>
        <Skeleton className="h-10 w-32 rounded-lg" />
      </div>

      {/* Row 1: metrics + hologram */}
      <div className="mt-8 grid gap-6 lg:grid-cols-[1fr_auto]">
        <div className="grid gap-4 lg:max-w-xs">
          <Skeleton className="h-20 rounded-xl" />
          <Skeleton className="h-20 rounded-xl" />
          <Skeleton className="h-20 rounded-xl" />
        </div>
        <Skeleton className="h-64 w-64 rounded-xl" />
      </div>

      {/* Row 2: heatmap + feed */}
      <div className="mt-6 grid gap-6 lg:grid-cols-2">
        <Skeleton className="h-48 rounded-xl" />
        <Skeleton className="h-48 rounded-xl" />
      </div>

      {/* Row 3: leaderboard */}
      <Skeleton className="mt-6 h-40 rounded-xl" />
    </main>
  );
}
