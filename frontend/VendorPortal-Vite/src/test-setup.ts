import "@testing-library/jest-dom/vitest";

// jsdom does not implement window.scrollTo; TanStack Router calls it during
// navigation (scroll restoration). Stub it to keep test output clean.
if (typeof window !== "undefined" && typeof window.scrollTo !== "function") {
  window.scrollTo = () => {};
}
