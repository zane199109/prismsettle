// ShardHeatmap component test — verifies the 16x16 grid renders all 256
// cells with the expected title attribute. DEV-PLAN §Phase 8 任务 8.9.

import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";

// Mock useShardActivity before importing the component.
vi.mock("@/hooks/useShardActivity", () => ({
  useShardActivity: () => ({
    data: {
      counts: (() => {
        const arr = new Array(256).fill(0);
        arr[0] = 5;
        arr[255] = 3;
        return arr;
      })(),
      lastActive: new Array(256).fill(0),
      total: 8,
    },
    raw: [],
    error: null,
    isValidating: false,
  }),
}));

import { ShardHeatmap } from "@/components/dashboard/ShardHeatmap";

describe("ShardHeatmap", () => {
  beforeEach(() => {
    vi.resetModules();
  });

  it("renders exactly 256 cells in a 16-column grid", () => {
    const { container } = render(<ShardHeatmap />);
    const grid = container.querySelector(".grid");
    expect(grid).toBeTruthy();
    expect(grid?.children.length).toBe(256);
    // 16-column grid template.
    expect(grid?.getAttribute("style")).toContain("repeat(16, 1fr)");
  });

  it("includes shard 0 and shard 255 title attributes", () => {
    render(<ShardHeatmap />);
    // Title format: "Shard <id>: <count> validations"
    const shard0 = screen.getByTitle("Shard 0: 5 validations");
    const shard255 = screen.getByTitle("Shard 255: 3 validations");
    expect(shard0).toBeInTheDocument();
    expect(shard255).toBeInTheDocument();
  });

  it("shows total validation count", () => {
    render(<ShardHeatmap />);
    expect(screen.getByText(/8 validations/)).toBeInTheDocument();
  });
});
