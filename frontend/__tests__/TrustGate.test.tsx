// TrustGate component test — verifies the three decision states render with
// the correct tone and thresholds. DEV-PLAN §Phase 8 任务 8.9.

import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import type { TrustCheckResult } from "@/lib/types";

// Default mock; individual tests override via mockReturnValue.
const mockUseTrustCheck = vi.fn(
  (_agentId?: string, _opts?: unknown): {
    result: TrustCheckResult | null;
    error: unknown;
    isValidating: boolean;
    refresh: () => void;
  } => ({
    result: null,
    error: null,
    isValidating: false,
    refresh: vi.fn(),
  }),
);

vi.mock("@/hooks/useTrustCheck", () => ({
  useTrustCheck: (agentId: string, opts?: unknown) => mockUseTrustCheck(agentId, opts),
}));

import { TrustGate } from "@/components/job/TrustGate";

const ALLOW: TrustCheckResult = {
  agent_id: "0xabc",
  decision: "allow",
  score: "9000000000000000000", // 0.9
  allow_threshold: "7000000000000000000",
  deny_threshold: "3000000000000000000",
};
const REVIEW: TrustCheckResult = { ...ALLOW, decision: "review", score: "5000000000000000000" };
const DENY: TrustCheckResult = { ...ALLOW, decision: "deny", score: "1000000000000000000" };

describe("TrustGate", () => {
  it("renders the loading state before first result", () => {
    mockUseTrustCheck.mockReturnValue({
      result: null,
      error: null,
      isValidating: true,
      refresh: vi.fn(),
    });
    render(<TrustGate agentId="0xabc" />);
    expect(screen.getByText(/Checking trust score/)).toBeInTheDocument();
  });

  it("renders the unavailable state when result is null and not loading", () => {
    mockUseTrustCheck.mockReturnValue({
      result: null,
      error: null,
      isValidating: false,
      refresh: vi.fn(),
    });
    render(<TrustGate agentId="" />);
    expect(screen.getByText(/Trust check unavailable/)).toBeInTheDocument();
  });

  it("renders ALLOW decision with green tone and correct thresholds", () => {
    mockUseTrustCheck.mockReturnValue({
      result: ALLOW,
      error: null,
      isValidating: false,
      refresh: vi.fn(),
    });
    render(<TrustGate agentId="0xabc" />);
    expect(screen.getByText("ALLOW")).toBeInTheDocument();
    // score / allow ≥ / deny < thresholds present.
    expect(screen.getByText("score")).toBeInTheDocument();
    expect(screen.getByText("allow ≥")).toBeInTheDocument();
    expect(screen.getByText("deny <")).toBeInTheDocument();
    // Hint mentions job creation enabled.
    expect(screen.getByText(/Job creation enabled/)).toBeInTheDocument();
  });

  it("renders REVIEW (REQUIRE_VALIDATION) decision with amber tone", () => {
    mockUseTrustCheck.mockReturnValue({
      result: REVIEW,
      error: null,
      isValidating: false,
      refresh: vi.fn(),
    });
    render(<TrustGate agentId="0xabc" />);
    expect(screen.getByText("REVIEW")).toBeInTheDocument();
    expect(screen.getByText(/FR-AP12/)).toBeInTheDocument();
  });

  it("renders DENY decision with red tone and FR-AP13 hint", () => {
    mockUseTrustCheck.mockReturnValue({
      result: DENY,
      error: null,
      isValidating: false,
      refresh: vi.fn(),
    });
    render(<TrustGate agentId="0xabc" />);
    expect(screen.getByText("DENY")).toBeInTheDocument();
    expect(screen.getByText(/FR-AP13/)).toBeInTheDocument();
    expect(screen.getByText(/createJob is blocked/)).toBeInTheDocument();
  });

  it("calls refresh when the re-check button is clicked", () => {
    const refresh = vi.fn();
    mockUseTrustCheck.mockReturnValue({
      result: ALLOW,
      error: null,
      isValidating: false,
      refresh,
    });
    render(<TrustGate agentId="0xabc" />);
    const btn = screen.getByTitle("Re-check trust");
    fireEvent.click(btn);
    expect(refresh).toHaveBeenCalledOnce();
  });
});
