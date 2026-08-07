// Jobs list tests — verifies mock data shape, amount formatting, and table
// rendering for Amount and Provider columns. Added after P2-2 fix.

import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import { MOCK_JOBS } from "@/lib/mock-data";
import type { JobVO } from "@/lib/types";

// ---------------------------------------------------------------------------
// 1. Mock data shape
// ---------------------------------------------------------------------------

describe("MOCK_JOBS data shape", () => {
  it("every job has an amount field (string or undefined)", () => {
    for (const job of MOCK_JOBS) {
      expect(job).toHaveProperty("amount");
      // amount is optional — either a string or undefined
      if (job.amount !== undefined) {
        expect(typeof job.amount).toBe("string");
      }
    }
  });

  it("every job has a provider field (string or undefined)", () => {
    for (const job of MOCK_JOBS) {
      expect(job).toHaveProperty("provider");
      if (job.provider !== undefined) {
        expect(typeof job.provider).toBe("string");
      }
    }
  });

  it("jobs with a provider have a non-empty string", () => {
    const assignedJobs = MOCK_JOBS.filter((j) => j.provider);
    expect(assignedJobs.length).toBeGreaterThan(0);
    for (const job of assignedJobs) {
      expect(job.provider?.length).toBeGreaterThan(0);
    }
  });

  it("jobs without a provider (Created/Funded) have empty string", () => {
    const unassigned = MOCK_JOBS.filter(
      (j) => j.status === "Created" || j.status === "Funded",
    );
    for (const job of unassigned) {
      expect(job.provider).toBe("");
    }
  });

  it("amount is a valid numeric string when present", () => {
    const withAmount = MOCK_JOBS.filter((j) => j.amount);
    for (const job of withAmount) {
      expect(Number(job.amount)).not.toBeNaN();
      expect(Number(job.amount)).toBeGreaterThan(0);
    }
  });
});

// ---------------------------------------------------------------------------
// 2. Amount formatting
// ---------------------------------------------------------------------------

describe("amount formatting", () => {
  function formatAmount(amount: string | undefined): string {
    if (!amount) return "—";
    return `${(Number(amount) / 1e6).toFixed(2)} USDC`;
  }

  it("formats a valid amount string", () => {
    expect(formatAmount("1000000")).toBe("1.00 USDC");
  });

  it("formats a larger amount", () => {
    expect(formatAmount("5000000")).toBe("5.00 USDC");
  });

  it("formats a small amount", () => {
    expect(formatAmount("750000")).toBe("0.75 USDC");
  });

  it("returns em dash for undefined", () => {
    expect(formatAmount(undefined)).toBe("—");
  });

  it("returns em dash for empty string", () => {
    expect(formatAmount("")).toBe("—");
  });
});

// ---------------------------------------------------------------------------
// 3. Provider display
// ---------------------------------------------------------------------------

describe("provider display", () => {
  // Simulates the inline logic from jobs/page.tsx:
  //   {job.provider ? formatAgentId(job.provider) : "—"}
  function formatAgentId(id: string): string {
    return `${id.slice(0, 6)}…${id.slice(-4)}`;
  }

  function displayProvider(provider: string | undefined): string {
    return provider ? formatAgentId(provider) : "—";
  }

  it("shows truncated address when provider is present", () => {
    const result = displayProvider("0x1234567890abcdef1234567890abcdef12345678");
    expect(result).toMatch(/…/);
    expect(result.length).toBeLessThan(15);
  });

  it("shows em dash when provider is empty", () => {
    expect(displayProvider("")).toBe("—");
  });

  it("shows em dash when provider is undefined", () => {
    expect(displayProvider(undefined)).toBe("—");
  });
});

// ---------------------------------------------------------------------------
// 4. Status filter coverage
// ---------------------------------------------------------------------------

describe("MOCK_JOBS status coverage", () => {
  it("covers all expected UI statuses", () => {
    const statuses = new Set(MOCK_JOBS.map((j) => j.status));
    // The demo contract uses: Created, Funded, Submitted, Completed, Disputed,
    // Resolved, Refunded
    expect(statuses.has("Created")).toBe(true);
    expect(statuses.has("Funded")).toBe(true);
    expect(statuses.has("Submitted")).toBe(true);
    expect(statuses.has("Completed")).toBe(true);
    expect(statuses.has("Disputed")).toBe(true);
    expect(statuses.has("Refunded")).toBe(true);
    expect(statuses.has("Resolved")).toBe(true);
  });
});

// ---------------------------------------------------------------------------
// 5. Page rendering (smoke test with mocked hook)
// ---------------------------------------------------------------------------

vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: vi.fn() }),
  useSearchParams: () => ({ get: vi.fn() }),
}));

vi.mock("@/hooks/useJobs", () => ({
  useJobs: () => ({
    jobs: MOCK_JOBS,
    loading: false,
    error: null,
    stats: { total: MOCK_JOBS.length },
  }),
}));

describe("Jobs list page", () => {
  it("renders Amount and Provider column headers", async () => {
    const { default: JobsPage } = await import("@/app/jobs/page");
    render(<JobsPage />);

    // Both new columns are present
    expect(screen.getByText("Amount")).toBeInTheDocument();
    expect(screen.getByText("Provider")).toBeInTheDocument();

    // Existing columns still present
    expect(screen.getByText("Job ID")).toBeInTheDocument();
    expect(screen.getByText("Status")).toBeInTheDocument();
    expect(screen.getByText("Creator")).toBeInTheDocument();
    // "Created" is both a status and a column; use getAllByText
    expect(screen.getAllByText("Created").length).toBeGreaterThanOrEqual(1);
  });

  it("displays job amount in the table", async () => {
    const { default: JobsPage } = await import("@/app/jobs/page");
    render(<JobsPage />);

    // Two mock jobs have "1.00 USDC" (job 1 and job 5)
    expect(screen.getAllByText("1.00 USDC").length).toBe(2);
    // Second mock job has amount "5000000" → "5.00 USDC"
    expect(screen.getByText("5.00 USDC")).toBeInTheDocument();
  });

  it("displays provider placeholder when unassigned", async () => {
    const { default: JobsPage } = await import("@/app/jobs/page");
    render(<JobsPage />);

    // Jobs 3 and 6 have no provider — show "—"
    const dashes = screen.getAllByText("—");
    expect(dashes.length).toBeGreaterThanOrEqual(2);
  });
});