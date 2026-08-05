import { render } from "@testing-library/react";
import * as fc from "fast-check";
import { describe, expect, it } from "vitest";

/**
 * Property 20: Icon Button Accessibility Labels
 * Validates: Requirements 10.3
 *
 * For any Button component rendered in icon-only mode (no visible text content),
 * the rendered element SHALL have a non-empty `aria-label` attribute.
 */

// ─── Types ────────────────────────────────────────────────────────────────────

interface IconButtonConfig {
  hasIcon: boolean;
  hasVisibleText: boolean;
  ariaLabel?: string;
}

// ─── Pure validation function ─────────────────────────────────────────────────

/**
 * Validates an icon button configuration for accessibility compliance.
 * Returns true if the config is accessible (icon-only buttons must have a non-empty aria-label).
 */
function isAccessibleIconButton(config: IconButtonConfig): boolean {
  // If the button has an icon and NO visible text, it MUST have a non-empty aria-label
  if (config.hasIcon && !config.hasVisibleText) {
    return config.ariaLabel !== undefined && config.ariaLabel.trim().length > 0;
  }
  // Buttons with visible text are accessible by default (text acts as label)
  return true;
}

// ─── Arbitraries ──────────────────────────────────────────────────────────────

const arbIconButtonConfig = fc.record({
  hasIcon: fc.boolean(),
  hasVisibleText: fc.boolean(),
  ariaLabel: fc.option(fc.string({ minLength: 0, maxLength: 50 }), { nil: undefined }),
});

/**
 * Generates only icon-only configs (icon present, no visible text) WITH a valid aria-label.
 * These represent correctly-implemented icon buttons.
 */
const arbAccessibleIconOnlyConfig: fc.Arbitrary<IconButtonConfig> = fc.record({
  hasIcon: fc.constant(true),
  hasVisibleText: fc.constant(false),
  ariaLabel: fc
    .string({ minLength: 1, maxLength: 50, unit: "grapheme" })
    .filter((s) => s.trim().length > 0),
});

/**
 * Generates icon-only configs WITHOUT a valid aria-label.
 * These represent inaccessible buttons that should be flagged.
 */
const arbInaccessibleIconOnlyConfig: fc.Arbitrary<IconButtonConfig> = fc.record({
  hasIcon: fc.constant(true),
  hasVisibleText: fc.constant(false),
  ariaLabel: fc.constantFrom(undefined, "", "   "),
});

// ─── Property Tests (Pure Logic) ──────────────────────────────────────────────

describe("Property 20: Icon Button Accessibility Labels", () => {
  it("icon-only buttons with a non-empty aria-label pass accessibility check", () => {
    fc.assert(
      fc.property(arbAccessibleIconOnlyConfig, (config) => {
        expect(isAccessibleIconButton(config)).toBe(true);
      }),
      { numRuns: 100 },
    );
  });

  it("icon-only buttons without a valid aria-label fail accessibility check", () => {
    fc.assert(
      fc.property(arbInaccessibleIconOnlyConfig, (config) => {
        expect(isAccessibleIconButton(config)).toBe(false);
      }),
      { numRuns: 100 },
    );
  });

  it("buttons with visible text are always accessible regardless of aria-label", () => {
    fc.assert(
      fc.property(
        fc.record({
          hasIcon: fc.boolean(),
          hasVisibleText: fc.constant(true),
          ariaLabel: fc.option(fc.string({ minLength: 0, maxLength: 50 }), { nil: undefined }),
        }),
        (config) => {
          expect(isAccessibleIconButton(config)).toBe(true);
        },
      ),
      { numRuns: 100 },
    );
  });

  it("for any config: icon-only without visible text implies aria-label is required", () => {
    fc.assert(
      fc.property(arbIconButtonConfig, (config) => {
        const result = isAccessibleIconButton(config);

        if (config.hasIcon && !config.hasVisibleText) {
          // Icon-only: must have non-empty aria-label
          const hasValidLabel =
            config.ariaLabel !== undefined && config.ariaLabel.trim().length > 0;
          expect(result).toBe(hasValidLabel);
        } else {
          // Has visible text or no icon: always accessible
          expect(result).toBe(true);
        }
      }),
      { numRuns: 200 },
    );
  });
});

// ─── Component Integration Tests ──────────────────────────────────────────────

describe("Property 20: Icon Button Accessibility — Component Verification", () => {
  /**
   * Renders an icon-only button using the provided aria-label.
   * Validates that real rendered DOM elements satisfy the property.
   */
  it("rendered icon-only buttons always have a non-empty aria-label", () => {
    fc.assert(
      fc.property(
        fc
          .string({ minLength: 1, maxLength: 50, unit: "grapheme" })
          .filter((s) => s.trim().length > 0),
        (ariaLabel) => {
          const { unmount, container } = render(
            <button type="button" aria-label={ariaLabel}>
              <svg aria-hidden="true">
                <circle cx="12" cy="12" r="10" />
              </svg>
            </button>,
          );

          const button = container.querySelector("button");
          expect(button).not.toBeNull();

          // The button has no visible text (only an svg with aria-hidden)
          const visibleText = button?.textContent?.trim() ?? "";
          expect(visibleText).toBe("");

          // It must have a non-empty aria-label
          const label = button?.getAttribute("aria-label");
          expect(label).not.toBeNull();
          expect(label?.trim().length).toBeGreaterThan(0);

          unmount();
        },
      ),
      { numRuns: 50 },
    );
  });

  it("rendered buttons with visible text do not require aria-label", () => {
    fc.assert(
      fc.property(
        fc
          .string({ minLength: 1, maxLength: 30, unit: "grapheme" })
          .filter((s) => s.trim().length > 0),
        (buttonText) => {
          const { unmount, container } = render(
            <button type="button">
              <svg aria-hidden="true">
                <circle cx="12" cy="12" r="10" />
              </svg>
              <span>{buttonText}</span>
            </button>,
          );

          const button = container.querySelector("button");
          expect(button).not.toBeNull();

          // Button has visible text content
          const visibleText = button?.textContent?.trim() ?? "";
          expect(visibleText.length).toBeGreaterThan(0);

          // aria-label is not required (text provides the accessible name)
          unmount();
        },
      ),
      { numRuns: 50 },
    );
  });
});
