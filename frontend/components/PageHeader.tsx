"use client";

// Minimal navigation bar — inspired by AgentOn's clean design.
// Logo | Agents | Jobs | Dashboard | WalletConnect
// Mobile hamburger menu for md breakpoint and below.

import { useEffect, useState } from "react";
import Link from "next/link";
import { usePathname } from "next/navigation";
import { Menu, X } from "lucide-react";
import { WalletConnect } from "@/components/WalletConnect";
import { cn } from "@/lib/utils";

const NAV = [
  { href: "/agents", label: "Agents" },
  { href: "/jobs", label: "Jobs" },
  { href: "/dashboard", label: "Dashboard" },
  { href: "/validator", label: "Validate" },
  { href: "/arbitrator", label: "Arbitrate" },
  { href: "/events", label: "Events" },
];

export function PageHeader() {
  const pathname = usePathname();
  const [mobileOpen, setMobileOpen] = useState(false);

  useEffect(() => {
    setMobileOpen(false);
  }, [pathname]);

  return (
    <header className="sticky top-0 z-50 border-b border-white/[0.04] bg-[#0a0a0f]/80 backdrop-blur-xl">
      <div className="mx-auto flex max-w-7xl items-center justify-between px-6 py-3">
        {/* Logo */}
        <Link href="/" className="flex items-center gap-2">
          <span className="text-base font-bold tracking-tight">
            <span className="text-prism-accent">Prism</span>
            <span className="text-white/80">Settle</span>
          </span>
        </Link>

        {/* Desktop nav */}
        <nav className="hidden items-center gap-0.5 md:flex">
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
                  "rounded-md px-3 py-1.5 text-sm transition-colors",
                  active
                    ? "bg-white/[0.06] text-white"
                    : "text-white/50 hover:text-white/80",
                )}
              >
                {item.label}
              </Link>
            );
          })}
        </nav>

        {/* Right side */}
        <div className="flex items-center gap-2">
          <WalletConnect />
          <button
            type="button"
            aria-label="Toggle navigation menu"
            aria-expanded={mobileOpen}
            onClick={() => setMobileOpen((v) => !v)}
            className="inline-flex h-8 w-8 items-center justify-center rounded-md text-white/50 hover:bg-white/[0.06] hover:text-white/80 md:hidden"
          >
            {mobileOpen ? <X className="h-4 w-4" /> : <Menu className="h-4 w-4" />}
          </button>
        </div>
      </div>

      {/* Mobile dropdown */}
      {mobileOpen && (
        <nav className="border-t border-white/[0.04] bg-[#0a0a0f]/95 backdrop-blur-xl md:hidden">
          <div className="mx-auto max-w-7xl space-y-0.5 px-6 py-2">
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
                      ? "bg-white/[0.06] text-white"
                      : "text-white/50 hover:text-white/80",
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