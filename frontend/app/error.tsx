"use client";

// Root error boundary — catches unhandled runtime errors across the app.
// Follows Next.js App Router error handling convention (ui-ux-pro-max guideline).
// Must be a Client Component with a reset callback.

import { useEffect } from "react";
import { AlertTriangle, RotateCcw, Home } from "lucide-react";
import Link from "next/link";

export default function Error({
  error,
  reset,
}: {
  error: Error & { digest?: string };
  reset: () => void;
}) {
  useEffect(() => {
    // Log to console for dev; in prod this would go to Sentry/Telegraf.
    console.error("Root error boundary caught:", error);
  }, [error]);

  return (
    <div className="prism-bg flex min-h-screen flex-col items-center justify-center px-6">
      <div className="w-full max-w-md rounded-xl border border-red-500/30 bg-prism-surface/60 p-8 text-center">
        <div className="mx-auto mb-4 flex h-12 w-12 items-center justify-center rounded-full bg-red-500/10">
          <AlertTriangle className="h-6 w-6 text-red-400" />
        </div>
        <h2 className="mb-2 text-lg font-semibold text-white">
          Something went wrong
        </h2>
        <p className="mb-1 text-sm text-white/60">
          An unexpected error occurred while rendering this page.
        </p>
        {error.digest && (
          <p className="mb-4 font-mono text-xs text-white/40">
            digest: {error.digest}
          </p>
        )}
        {!error.digest && <div className="mb-4" />}
        <div className="flex items-center justify-center gap-3">
          <button
            onClick={reset}
            className="inline-flex cursor-pointer items-center gap-2 rounded-md bg-prism-accent px-4 py-2 text-sm font-semibold text-white transition-colors hover:bg-prism-accent/80"
          >
            <RotateCcw className="h-4 w-4" />
            Try again
          </button>
          <Link
            href="/"
            className="inline-flex cursor-pointer items-center gap-2 rounded-md border border-white/20 px-4 py-2 text-sm font-semibold text-white/80 transition-colors hover:bg-white/5"
          >
            <Home className="h-4 w-4" />
            Home
          </Link>
        </div>
      </div>
    </div>
  );
}
