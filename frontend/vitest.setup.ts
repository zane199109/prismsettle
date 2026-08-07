// vitest setup — adds jest-dom matchers + localStorage polyfill.
import "@testing-library/jest-dom/vitest";
import { beforeEach } from "vitest";

// jsdom provides localStorage, but ensure it's cleared between tests.
beforeEach(() => {
  if (typeof window !== "undefined" && window.localStorage) {
    window.localStorage.clear();
  }
});

// IntersectionObserver polyfill — required by framer-motion for LivePreview tests.
if (typeof globalThis.IntersectionObserver === "undefined") {
  class MockIntersectionObserver implements IntersectionObserver {
    readonly root: Element | Document | null = null;
    readonly rootMargin: string = "";
    readonly thresholds: ReadonlyArray<number> = [];
    observe = () => {};
    unobserve = () => {};
    disconnect = () => {};
    takeRecords = (): IntersectionObserverEntry[] => [];
  }
  globalThis.IntersectionObserver = MockIntersectionObserver as unknown as {
    new (
      callback: IntersectionObserverCallback,
      options?: IntersectionObserverInit,
    ): IntersectionObserver;
    prototype: IntersectionObserver;
  };
}
