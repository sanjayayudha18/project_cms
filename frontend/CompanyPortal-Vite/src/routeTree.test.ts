import { createMemoryHistory, createRouter } from "@tanstack/react-router";
import { describe, expect, it } from "vitest";
import { NAV_CONFIG } from "./lib/config/navigation";
import { routeTree } from "./routeTree";

// T-D1 (FE-05): screens that only render static src/data/*.json are out of the route tree and menu.
const MOCK_PATHS = [
  "/cit",
  "/forecasting/dsr-dashboard",
  "/invoice/list",
  "/invoice/reconciliation",
];

const router = createRouter({ routeTree, history: createMemoryHistory() });
const paths = Object.keys(router.routesByPath);

describe("route tree without mock screens", () => {
  it.each(MOCK_PATHS)("%s is not a registered route", (path) => {
    expect(paths).not.toContain(path);
  });

  it.each(MOCK_PATHS)("%s is not in the menu", (path) => {
    expect(NAV_CONFIG.map((i) => i.href)).not.toContain(path);
  });

  it("keeps the API-backed replenishment screen", () => {
    expect(paths).toContain("/replenishment");
  });
});
