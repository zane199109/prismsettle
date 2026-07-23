"use client";

// Shared page header with brand, primary nav, wallet connect, and a mobile
// hamburger dropdown (FR-M10). Used across all console routes
// (agents/jobs/validator/perf/events/dashboard) to keep the chrome consistent
// and avoid duplicating layout code in each page.

import { useEffect, useState } from "react";
import Link from "next/link";
import { usePathname } from "next/navigation";
import { Menu, X } from "lucide-react";
import { WalletConnect } from "@/components/WalletConnect";
import { cn } from "@/lib/utils";

const NAV = [
  { href: "/", label: "Home" },
  { href: "/dashboard", label: "Dashboard" },
  { href: "/agents", label: "Agents" },
  { href: "/jobs", label: "Jobs" },
  { href: "/validator", label: "Validator" },
  { href: "/events", label: "Events" },
  { href: "/disputes", label: "Disputes" },
  { href: "/arbitrator", label: "Arbitrator" },
  { href: "/me", label: "Profile" },
  { href: "/perf", label: "Performance" },
];

export function PageHeader() {
  const pathname = usePathname();
  const [mobileOpen, setMobileOpen] = useState(false);

  // Close the mobile menu whenever the route changes.
  useEffect(() => {
    setMobileOpen(false);
  }, [pathname]);

  return (
    <header className="border-b border-white/10 bg-prism-surface/30 backdrop-blur">
      <div className="mx-auto flex max-w-7xl items-center justify-between px-6 py-4">
        <Link href="/" className="group flex items-center gap-2">
          <span className="text-xl font-bold tracking-tight">
            <span className="text-prism-accent">Prism</span>
            <span className="text-prism-glow">Settle</span>
          </span>
        </Link>
        <nav className="hidden items-center gap-0.5 lg:flex">
          {NAV.map((item) => {
            const active =
              item.href === "/"
                ? pathname === "/"
                : pathname.startsWith(item.href);
            return (
              <Link
                key={item.href}
                href={item.href}
                className={cn(
                  "rounded-md px-2.5 py-1.5 text-xs transition-colors",
                  active
                    ? "bg-prism-accent/20 text-prism-accent"
                    : "text-white/60 hover:bg-white/5 hover:text-white",
                )}
              >
                {item.label}
              </Link>
            );
          })}
        </nav>
        <div className="flex items-center gap-2">
          <WalletConnect />
          <button
            type="button"
            aria-label="Toggle navigation menu"
            aria-expanded={mobileOpen}
            onClick={() => setMobileOpen((v) => !v)}
            className="inline-flex h-9 w-9 items-center justify-center rounded-md border border-white/10 text-white/70 hover:bg-white/5 lg:hidden"
          >
            {mobileOpen ? <X className="h-4 w-4" /> : <Menu className="h-4 w-4" />}
          </button>
        </div>
      </div>

      {/* Mobile dropdown */}
      {mobileOpen && (
        <nav className="border-t border-white/10 bg-prism-surface/95 backdrop-blur lg:hidden">
          <div className="mx-auto max-w-7xl space-y-1 px-6 py-3">
            {NAV.map((item) => {
              const active =
                item.href === "/"
                  ? pathname === "/"
                  : pathname.startsWith(item.href);
              return (
                <Link
                  key={item.href}
                  href={item.href}
                  className={cn(
                    "block rounded-md px-3 py-2 text-sm transition-colors",
                    active
                      ? "bg-prism-accent/20 text-prism-accent"
                      : "text-white/70 hover:bg-white/5 hover:text-white",
                  )}
                >
                  {item.label}
                </Link>
              );
            })}
          </div>
        </nav>
      )}
    </header>
  );
}
