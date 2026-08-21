// Feature: frontend-consolidation, Property 2: AttentionPanel Renders All Required Fields
import { render } from "@testing-library/react";
import * as fc from "fast-check";
import { describe, expect, it, vi } from "vitest";
import type { AttentionCategory, AttentionItem } from "../types";

/**
 * Property 2: AttentionPanel Renders All Required Fields
 * **Validates: Requirements 2.3**
 *
 * For any array of attention items (with category, title, description, and timestamp fields),
 * rendering the AttentionPanel component SHALL produce output that contains every item's title,
 * description, a category-appropriate icon indicator (danger, warning, or info), and a relative
 * timestamp string.
 */

// Mock the static JSON import so we can inject arbitrary data
let mockItems: AttentionItem[] = [];
vi.mock("@/data/attention-items.json", () => {
  return {
    default: new Proxy([], {
      get(_, prop) {
        if (prop === Symbol.iterator) {
          return () => mockItems[Symbol.iterator]();
        }
        if (prop === "length") return mockItems.length;
        if (typeof prop === "string" && !Number.isNaN(Number(prop))) {
          return mockItems[Number(prop)];
        }
        // Array methods
        const val = (mockItems as unknown as Record<string | symbol, unknown>)[prop];
        if (typeof val === "function") return val.bind(mockItems);
        return val;
      },
    }),
  };
});

// Dynamically import after mock setup
const { AttentionPanel } = await import("../AttentionPanel");

describe("Property 2: AttentionPanel Renders All Required Fields", () => {
  const categories: AttentionCategory[] = ["danger", "warning", "info"];
  const validIcons = ["triangle-alert", "alert-circle", "clock", "truck", "file-check"];

  const arbAttentionItem: fc.Arbitrary<AttentionItem> = fc.record({
    id: fc.uuid(),
    category: fc.constantFrom(...categories),
    icon: fc.constantFrom(...validIcons),
    title: fc
      .string({ minLength: 1, maxLength: 80, unit: "grapheme" })
      .filter((s) => s.trim().length > 0),
    description: fc
      .string({ minLength: 1, maxLength: 200, unit: "grapheme" })
      .filter((s) => s.trim().length > 0),
    time: fc.constantFrom("1m", "5m", "15m", "32m", "1h", "2h", "3h", "1d", "2d"),
  });

  const arbAttentionItems = fc.array(arbAttentionItem, {
    minLength: 1,
    maxLength: 8,
  });

  // Semantic CSS class mapping for category verification
  const categoryBgClasses: Record<AttentionCategory, string> = {
    danger: "bg-[var(--danger-bg)]",
    warning: "bg-[var(--warning-bg)]",
    info: "bg-[var(--info-bg)]",
  };

  it("renders every item's title, description, category icon indicator, and timestamp", () => {
    fc.assert(
      fc.property(arbAttentionItems, (items) => {
        // Inject the generated items
        mockItems = items;

        const { container, unmount } = render(<AttentionPanel />);

        const listItems = container.querySelectorAll("li");
        expect(listItems).toHaveLength(items.length);

        for (let i = 0; i < items.length; i++) {
          const li = listItems[i];
          const item = items[i];

          // Title must be present
          const titleEl = li.querySelector("p.text-sm.font-medium");
          expect(titleEl?.textContent).toBe(item.title);

          // Description must be present
          const descEl = li.querySelector("p.text-xs");
          expect(descEl?.textContent).toBe(item.description);

          // Timestamp must be present
          const timeEl = li.querySelector("span.text-xs");
          expect(timeEl?.textContent).toBe(item.time);

          // Category-appropriate icon indicator — check the icon container has correct bg class
          const iconContainer = li.querySelector("div.flex-shrink-0");
          expect(iconContainer?.className).toContain(categoryBgClasses[item.category]);
        }

        unmount();
      }),
      { numRuns: 50 },
    );
  });

  it("renders an empty list when no attention items are provided", () => {
    mockItems = [];
    const { container, unmount } = render(<AttentionPanel />);

    const listItems = container.querySelectorAll("li");
    expect(listItems).toHaveLength(0);

    unmount();
  });
});
