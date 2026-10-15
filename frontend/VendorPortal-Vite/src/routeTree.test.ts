import { NAV_ITEMS } from "@/lib/constants";
import { routeTree } from "@/routeTree";
import { createMemoryHistory, createRouter } from "@tanstack/react-router";
import { describe, expect, it } from "vitest";

// T-D1 (FE-05): screens that only render static src/data/*.json are out of the route tree and menu.
const MOCK_PATHS = ["/invoices", "/schedule", "/orders/$id/evidence"];

describe("route tree without mock screens", () => {
  const router = createRouter({ routeTree, history: createMemoryHistory() });
  const paths = Object.keys(router.routesByPath);

  it.each(MOCK_PATHS)("%s is not a registered route", (path) => {
    expect(paths).not.toContain(path);
  });

  it.each(MOCK_PATHS)("%s is not in the menu", (path) => {
    expect(NAV_ITEMS.map((i) => i.path)).not.toContain(path);
  });

  // No match for a removed route; /invoices and /schedule fall to the root (not-found).
  // /orders/abc/evidence fuzzy-matches /orders/$id, whose leftover segment renders not-found.
  it.each([
    ["/invoices", "__root__"],
    ["/schedule", "__root__"],
    ["/orders/abc/evidence", "/_protected/_shell/orders/$id"],
  ])("direct URL %s resolves to %s, not a mock screen", (url, leafId) => {
    expect(router.matchRoutes(url).at(-1)?.routeId).toBe(leafId);
  });
});
