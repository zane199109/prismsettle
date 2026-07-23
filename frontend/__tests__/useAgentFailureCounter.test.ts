// useAgentFailureCounter hook test — verifies FR-M12 localStorage aggregation
// keeps only the last 50 failures. DEV-PLAN §Phase 8 任务 8.9.

import { describe, it, expect } from "vitest";
import { renderHook, act } from "@testing-library/react";
import { useAgentFailureCounter } from "@/hooks/useAgentFailureCounter";

const WINDOW = 50;

describe("useAgentFailureCounter", () => {
  it("starts empty for a new agent", () => {
    const { result } = renderHook(() => useAgentFailureCounter("agent-1"));
    expect(result.current.count).toBe(0);
    expect(result.current.rate).toBe(0);
  });

  it("records a failure and persists to localStorage", () => {
    const { result } = renderHook(() => useAgentFailureCounter("agent-1"));
    act(() => {
      result.current.recordFailure("timeout");
    });
    expect(result.current.count).toBe(1);
    expect(result.current.rate).toBe(1 / WINDOW);
    // Persisted to localStorage.
    const raw = window.localStorage.getItem("prismsettle:agent-failures:agent-1");
    expect(raw).toBeTruthy();
    const arr = JSON.parse(raw!);
    expect(arr).toHaveLength(1);
    expect(arr[0].reason).toBe("timeout");
  });

  it("caps at the last 50 failures", () => {
    const { result } = renderHook(() => useAgentFailureCounter("agent-cap"));
    act(() => {
      for (let i = 0; i < 60; i++) {
        result.current.recordFailure(`err-${i}`);
      }
    });
    expect(result.current.count).toBe(WINDOW);
    expect(result.current.rate).toBe(1);
    // The earliest failures (err-0..err-9) should have been dropped; only
    // err-10..err-59 remain.
    const raw = window.localStorage.getItem("prismsettle:agent-failures:agent-cap");
    const arr = JSON.parse(raw!);
    expect(arr).toHaveLength(WINDOW);
    expect(arr[0].reason).toBe("err-10");
    expect(arr[WINDOW - 1].reason).toBe("err-59");
  });

  it("clear() resets the failure log", () => {
    const { result } = renderHook(() => useAgentFailureCounter("agent-clear"));
    act(() => {
      result.current.recordFailure("a");
      result.current.recordFailure("b");
    });
    expect(result.current.count).toBe(2);
    act(() => {
      result.current.clear();
    });
    expect(result.current.count).toBe(0);
    expect(result.current.rate).toBe(0);
    expect(window.localStorage.getItem("prismsettle:agent-failures:agent-clear")).toBe("[]");
  });

  it("isolates failures per agent id", () => {
    const { result: r1 } = renderHook(() => useAgentFailureCounter("agent-A"));
    const { result: r2 } = renderHook(() => useAgentFailureCounter("agent-B"));
    act(() => {
      r1.current.recordFailure("a");
      r1.current.recordFailure("a");
      r2.current.recordFailure("b");
    });
    expect(r1.current.count).toBe(2);
    expect(r2.current.count).toBe(1);
  });
});
