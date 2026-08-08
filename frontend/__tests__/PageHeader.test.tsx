// PageHeader tests — verifies all navigation links render correctly.
// Added after P2-1 fix (Validate, Arbitrate, Events links).

import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import { PageHeader } from "@/components/PageHeader";

// Mock next/navigation — PageHeader uses usePathname for active link styling.
vi.mock("next/navigation", () => ({
  usePathname: () => "/",
}));

// Mock WalletConnect — it depends on wagmi context which we don't need here.
vi.mock("@/components/WalletConnect", () => ({
  WalletConnect: () => <div data-testid="wallet-connect" />,
}));

describe("PageHeader", () => {
  it("renders the logo with link to home", () => {
    const { container } = render(<PageHeader />);
    // Logo is split into two spans: "Prism" + "Settle" inside a single <a>.
    const logoLink = container.querySelector('a[href="/"]');
    expect(logoLink).toBeInTheDocument();
    expect(logoLink?.textContent).toBe("PrismSettle");
  });

  it("renders all 3 navigation links", () => {
    render(<PageHeader />);
    expect(screen.getByText("Agents")).toBeInTheDocument();
    expect(screen.getByText("Jobs")).toBeInTheDocument();
    expect(screen.getByText("Events")).toBeInTheDocument();
  });

  it("each nav link has a valid href", () => {
    render(<PageHeader />);
    const links = [
      { label: "Agents", href: "/agents" },
      { label: "Jobs", href: "/jobs" },
      { label: "Events", href: "/events" },
    ];
    for (const { label, href } of links) {
      const link = screen.getByText(label);
      expect(link.closest("a")).toHaveAttribute("href", href);
    }
  });

  it("renders WalletConnect component", () => {
    render(<PageHeader />);
    expect(screen.getByTestId("wallet-connect")).toBeInTheDocument();
  });

  it("has a mobile menu toggle button", () => {
    render(<PageHeader />);
    const toggle = screen.getByLabelText("Toggle navigation menu");
    expect(toggle).toBeInTheDocument();
    expect(toggle).toHaveAttribute("aria-expanded", "false");
  });
});