// vitest setup — adds jest-dom matchers + localStorage polyfill.
import "@testing-library/jest-dom/vitest";
import { beforeEach } from "vitest";

// jsdom provides localStorage, but ensure it's cleared between tests.
beforeEach(() => {
  if (typeof window !== "undefined" && window.localStorage) {
    window.localStorage.clear();
  }
});
