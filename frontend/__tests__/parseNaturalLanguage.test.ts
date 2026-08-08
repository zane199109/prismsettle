import { describe, it, expect } from "vitest";
import { parseNaturalLanguage } from "@/lib/utils";

describe("parseNaturalLanguage", () => {
  it("extracts amount, currency, deadline and agent tag", () => {
    const r = parseNaturalLanguage("帮我创建 5 USDC 的任务给做数据标注的 agent，3 天内完成，最低信誉 0.5");
    expect(r.amount).toBe("5");
    expect(r.currency).toBe("usdc");
    expect(r.deadlineIn).toBe(3);
    expect(r.agentTag).toBe("data");
    expect(r.minRep).toBe("0.5");
  });

  it("detects MON currency", () => {
    expect(parseNaturalLanguage("1 MON 任务").currency).toBe("mon");
  });

  it("detects translation agent and tomorrow deadline", () => {
    const r = parseNaturalLanguage("翻译任务，明天截止");
    expect(r.agentTag).toBe("translation");
    expect(r.deadlineIn).toBe(1);
  });

  it("returns empty fields for gibberish", () => {
    expect(parseNaturalLanguage("随便什么")).toEqual({});
  });
});
