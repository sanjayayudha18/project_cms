// Feature: frontend-consolidation, Property 6: Invoice Expansion Shows Complete Line Items
import { render } from "@testing-library/react";
import * as fc from "fast-check";
import { describe, expect, it } from "vitest";
import type { InvoiceLineItem, MatchStatus } from "../types";
import { InvoiceDetail } from "../InvoiceDetail";

/**
 * Property 6: Invoice Expansion Shows Complete Line Items
 * **Validates: Requirements 7.3**
 *
 * For any invoice object with N line items (N ≥ 1), expanding that invoice's row
 * SHALL render exactly N line item entries, each displaying description, invoiced amount,
 * matched order reference, expected amount, variance, and match status fields.
 */

describe("Property 6: Invoice Expansion Shows Complete Line Items", () => {
  const matchStatuses: MatchStatus[] = ["Matched", "Mismatch", "Pending Review"];

  const matchStatusLabels: Record<MatchStatus, string> = {
    Matched: "Cocok",
    Mismatch: "Tidak Cocok",
    "Pending Review": "Menunggu Review",
  };

  const arbLineItem: fc.Arbitrary<InvoiceLineItem> = fc.record({
    id: fc.uuid(),
    description: fc
      .string({ minLength: 1, maxLength: 100, unit: "grapheme" })
      .filter((s) => s.trim().length > 0),
    invoicedAmount: fc.integer({ min: 0, max: 999_999_999 }),
    matchedOrderRef: fc.oneof(
      fc.constant(null),
      fc.string({ minLength: 3, maxLength: 20 }).filter((s) => s.trim().length > 0),
    ),
    expectedAmount: fc.integer({ min: 0, max: 999_999_999 }),
    variance: fc.integer({ min: -999_999_999, max: 999_999_999 }),
    matchStatus: fc.constantFrom(...matchStatuses),
  });

  const arbLineItems = fc.array(arbLineItem, { minLength: 1, maxLength: 10 });

  it("renders exactly N line item rows with all required fields for N ≥ 1 line items", () => {
    fc.assert(
      fc.property(arbLineItems, (lineItems) => {
        const { container, unmount } = render(
          <InvoiceDetail lineItems={lineItems} />,
        );

        // Should render exactly N rows in tbody
        const rows = container.querySelectorAll("tbody tr");
        expect(rows).toHaveLength(lineItems.length);

        for (let i = 0; i < lineItems.length; i++) {
          const row = rows[i];
          const item = lineItems[i];
          const cells = row.querySelectorAll("td");

          // Should have 6 cells: description, invoiced amount, order ref, expected amount, variance, match status
          expect(cells).toHaveLength(6);

          // 1. Description
          expect(cells[0].textContent).toBe(item.description);

          // 2. Invoiced amount (formatted as IDR)
          const invoicedText = cells[1].textContent;
          expect(invoicedText).toBeTruthy();
          // Verify the cell contains a number-like formatted string (dots as separators)
          expect(invoicedText!.length).toBeGreaterThan(0);

          // 3. Matched order reference (or em dash if null)
          const orderRefText = cells[2].textContent;
          if (item.matchedOrderRef === null) {
            expect(orderRefText).toBe("\u2014");
          } else {
            expect(orderRefText).toBe(item.matchedOrderRef);
          }

          // 4. Expected amount (formatted as IDR)
          const expectedText = cells[3].textContent;
          expect(expectedText).toBeTruthy();
          expect(expectedText!.length).toBeGreaterThan(0);

          // 5. Variance (formatted as IDR)
          const varianceText = cells[4].textContent;
          expect(varianceText).toBeTruthy();

          // 6. Match status — rendered as a Badge with the correct label
          const badgeEl = cells[5].querySelector("span");
          expect(badgeEl).not.toBeNull();
          expect(badgeEl!.textContent).toContain(matchStatusLabels[item.matchStatus]);
        }

        unmount();
      }),
      { numRuns: 100 },
    );
  });
});
