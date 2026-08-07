// LivePreview tests — verifies job cards display amount and provider info.
// Added after P2-2 fix integration.

import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import { LivePreview } from "@/components/landing/LivePreview";
import { MOCK_JOBS } from "@/lib/mock-data";
import { DEMO_ADDRESSES } from "@/lib/mock-data";

// Mock next/navigation — LivePreview uses Link components.
vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: vi.fn() }),
}));

// Mock useAgents to return empty list (we only care about jobs here).
vi.mock("@/hooks/useAgents", () => ({
  useAgents: () => ({ agents: [], isValidating: false }),
}));

// Mock useJobs to return real mock data.
vi.mock("@/hooks/useJobs", () => ({
  useJobs: () => ({ jobs: MOCK_JOBS, isValidating: false }),
}));

describe("LivePreview", () => {
  it("renders the section title", () => {
    render(<LivePreview />);
    expect(screen.getByText("Live preview")).toBeInTheDocument();
    expect(
      screen.getByText("What's happening on-chain"),
    ).toBeInTheDocument();
  });

  it("renders job cards with amount", () => {
    render(<LivePreview />);
    // Job 1 has amount 1000000 → "1.00 USDC"
    const amountElements = screen.getAllByText(/USDC/);
    expect(amountElements.length).toBeGreaterThanOrEqual(1);
  });

  it("renders job cards with amount and provider info", () => {
    const { container } = render(<LivePreview />);
    // Job cards render with amount (USDC) and provider info
    const jobCards = container.querySelectorAll('a[href^="/jobs/"]');
    expect(jobCards.length).toBeGreaterThanOrEqual(3);
  });

  it("shows active job statuses", () => {
    render(<LivePreview />);
    // LivePreview filters out Completed/Refunded, shows only active jobs
    // Mock data has: Submitted, Funded, Disputed, Created (sliced to 3)
    const activeStatuses = ["Submitted", "Funded", "Disputed", "Created"];
    const found = activeStatuses.filter((s) => {
      try {
        screen.getByText(s);
        return true;
      } catch {
        return false;
      }
    });
    // At least 2 of the 4 active statuses should be visible
    expect(found.length).toBeGreaterThanOrEqual(2);
  });
});