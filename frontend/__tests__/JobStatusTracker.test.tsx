// JobStatusTracker component test — verifies the ERC-8183 4-state display
// + parallel Hook arbitration block. DEV-PLAN §Phase 8 任务 8.9.

import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import { JobStatusTracker } from "@/components/job/JobStatusTracker";
import type { JobTimelineItem } from "@/lib/types";

const NO_TIMELINE: JobTimelineItem[] = [];

describe("JobStatusTracker", () => {
  it("renders all 4 ERC-8183 states (Open/Funded/Submitted/Terminal)", () => {
    render(<JobStatusTracker current="Created" timeline={NO_TIMELINE} />);
    expect(screen.getByText("Open")).toBeInTheDocument();
    expect(screen.getByText("Funded")).toBeInTheDocument();
    expect(screen.getByText("Submitted")).toBeInTheDocument();
    expect(screen.getByText("Terminal")).toBeInTheDocument();
  });

  it("highlights Open as active when current=Created", () => {
    render(<JobStatusTracker current="Created" timeline={NO_TIMELINE} />);
    // getByText matches the inner label div; go up to the card via parentElement.
    const openCard = screen.getByText("Open").parentElement;
    expect(openCard?.className).toContain("bg-prism-accent/10");
  });

  it("marks Funded as active for both Funded and Assigned contract states", () => {
    const { rerender } = render(
      <JobStatusTracker current="Funded" timeline={NO_TIMELINE} />,
    );
    let fundedCard = screen.getByText("Funded").parentElement;
    expect(fundedCard?.className).toContain("bg-prism-accent/10");

    rerender(<JobStatusTracker current="Assigned" timeline={NO_TIMELINE} />);
    fundedCard = screen.getByText("Funded").parentElement;
    expect(fundedCard?.className).toContain("bg-prism-accent/10");
  });

  it("renders Terminal with sub-label for Completed vs Refunded", () => {
    const { rerender } = render(
      <JobStatusTracker current="Completed" timeline={NO_TIMELINE} />,
    );
    expect(screen.getByText("completed")).toBeInTheDocument();

    rerender(<JobStatusTracker current="Refunded" timeline={NO_TIMELINE} />);
    expect(screen.getByText("refunded")).toBeInTheDocument();
  });

  it("always renders the Hook Arbitration parallel block", () => {
    render(<JobStatusTracker current="Funded" timeline={NO_TIMELINE} />);
    expect(screen.getByText(/Hook Arbitration/)).toBeInTheDocument();
  });

  it("flags arbitration when current is Disputed", () => {
    render(<JobStatusTracker current="Disputed" timeline={NO_TIMELINE} />);
    // The "⚖ Hook Arbitration" span is nested inside a flex div inside the
    // card div — go up two levels to reach the card with the border class.
    const arbCard = screen.getByText(/Hook Arbitration/).closest("div.rounded-lg");
    expect(arbCard?.className).toContain("border-amber-500/40");
  });

  it("renders timeline entries in the debug section", () => {
    const timeline: JobTimelineItem[] = [
      {
        status: "Created",
        tx_hash: "0xabc1",
        block_number: 100,
        timestamp: Date.now() / 1000 - 60,
      },
      {
        status: "Funded",
        tx_hash: "0xabc2",
        block_number: 110,
        timestamp: Date.now() / 1000 - 30,
      },
    ];
    render(<JobStatusTracker current="Funded" timeline={timeline} />);
    // "Created" only appears in the timeline (ERC-8183 shows "Open").
    expect(screen.getByText("Created")).toBeInTheDocument();
    // "Funded" appears in both the ERC-8183 grid and the timeline — use AllBy.
    expect(screen.getAllByText("Funded").length).toBeGreaterThanOrEqual(2);
    // Both timeline entries have block numbers shown.
    expect(screen.getByText(/@ block 100/)).toBeInTheDocument();
    expect(screen.getByText(/@ block 110/)).toBeInTheDocument();
  });
});
