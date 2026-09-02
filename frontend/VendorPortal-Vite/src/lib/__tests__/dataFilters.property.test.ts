import dsrData from "@/data/dsr.json";
import invoicesData from "@/data/invoices.json";
import notificationsData from "@/data/notifications.json";
import ordersData from "@/data/orders.json";
import schedulesData from "@/data/schedules.json";
import * as fc from "fast-check";

// --- Pure filter utility functions ---

/** Filter any collection by vendorId */
function filterByVendor<T extends { vendorId: string }>(data: readonly T[], vendorId: string): T[] {
  return data.filter((record) => record.vendorId === vendorId);
}

/** Filter records by date range (inclusive on both bounds) */
function filterByDateRange<T extends { scheduledDate: string }>(
  data: readonly T[],
  startDate?: string,
  endDate?: string,
): T[] {
  return data.filter((record) => {
    if (startDate && record.scheduledDate < startDate) return false;
    if (endDate && record.scheduledDate > endDate) return false;
    return true;
  });
}

/** Filter records by status */
function filterByStatus<T extends { status: string }>(data: readonly T[], status: string): T[] {
  return data.filter((record) => record.status === status);
}

/** Filter records by both status and date range (AND composition) */
function filterByStatusAndDateRange<T extends { status: string; scheduledDate: string }>(
  data: readonly T[],
  status: string,
  startDate?: string,
  endDate?: string,
): T[] {
  return data.filter((record) => {
    if (record.status !== status) return false;
    if (startDate && record.scheduledDate < startDate) return false;
    if (endDate && record.scheduledDate > endDate) return false;
    return true;
  });
}

// --- Generators ---

const vendorIdArb = fc.constantFrom("vendor-gardanet", "vendor-ssi", "vendor-g4s");

const orderStatusArb = fc.constantFrom("Scheduled", "In Transit", "Completed", "Failed");

// Extract the date range from actual orders data for realistic testing
const allDates = ordersData.map((o) => o.scheduledDate).sort();
const minDate = allDates[0];
const maxDate = allDates[allDates.length - 1];

/** Generate a date string within the orders date range */
function dateInRange(): fc.Arbitrary<string> {
  const minDay = new Date(minDate).getTime();
  const maxDay = new Date(maxDate).getTime();
  const dayMs = 24 * 60 * 60 * 1000;
  const totalDays = Math.floor((maxDay - minDay) / dayMs);

  return fc.integer({ min: 0, max: totalDays }).map((offset) => {
    const date = new Date(minDay + offset * dayMs);
    return date.toISOString().split("T")[0];
  });
}

/** Generate a valid date range (start <= end) within orders data range */
function dateRangeArb(): fc.Arbitrary<{
  startDate: string | undefined;
  endDate: string | undefined;
}> {
  return fc
    .tuple(
      fc.option(dateInRange(), { nil: undefined }),
      fc.option(dateInRange(), { nil: undefined }),
    )
    .map(([d1, d2]) => {
      if (d1 && d2) {
        const [startDate, endDate] = d1 <= d2 ? [d1, d2] : [d2, d1];
        return { startDate, endDate };
      }
      return { startDate: d1, endDate: d2 };
    });
}

// =========================================================================
// Property 1: Vendor Data Isolation
// Validates: Requirements 3.2, 5.2, 6.2, 7.2, 9.3, 9.9, 12.1, 12.2
// =========================================================================

describe("Property 1: Vendor Data Isolation", () => {
  it("filtering orders by vendorId returns only matching records", () => {
    fc.assert(
      fc.property(vendorIdArb, (vendorId) => {
        const result = filterByVendor(ordersData, vendorId);
        expect(result.every((r) => r.vendorId === vendorId)).toBe(true);
        expect(result.some((r) => r.vendorId !== vendorId)).toBe(false);
      }),
      { numRuns: 100 },
    );
  });

  it("filtering invoices by vendorId returns only matching records", () => {
    fc.assert(
      fc.property(vendorIdArb, (vendorId) => {
        const result = filterByVendor(invoicesData, vendorId);
        expect(result.every((r) => r.vendorId === vendorId)).toBe(true);
        expect(result.some((r) => r.vendorId !== vendorId)).toBe(false);
      }),
      { numRuns: 100 },
    );
  });

  it("filtering schedules by vendorId returns only matching records", () => {
    fc.assert(
      fc.property(vendorIdArb, (vendorId) => {
        const result = filterByVendor(schedulesData, vendorId);
        expect(result.every((r) => r.vendorId === vendorId)).toBe(true);
        expect(result.some((r) => r.vendorId !== vendorId)).toBe(false);
      }),
      { numRuns: 100 },
    );
  });

  it("filtering DSR records by vendorId returns only matching records", () => {
    fc.assert(
      fc.property(vendorIdArb, (vendorId) => {
        const result = filterByVendor(dsrData, vendorId);
        expect(result.every((r) => r.vendorId === vendorId)).toBe(true);
        expect(result.some((r) => r.vendorId !== vendorId)).toBe(false);
      }),
      { numRuns: 100 },
    );
  });

  it("filtering notifications by vendorId returns only matching records", () => {
    fc.assert(
      fc.property(vendorIdArb, (vendorId) => {
        const result = filterByVendor(notificationsData, vendorId);
        expect(result.every((r) => r.vendorId === vendorId)).toBe(true);
        expect(result.some((r) => r.vendorId !== vendorId)).toBe(false);
      }),
      { numRuns: 100 },
    );
  });

  it("filtering with non-existent vendorId returns empty array", () => {
    const nonExistentIds = ["vendor-unknown", "vendor-xyz", "vendor-abc"];
    for (const vendorId of nonExistentIds) {
      expect(filterByVendor(ordersData, vendorId)).toHaveLength(0);
      expect(filterByVendor(invoicesData, vendorId)).toHaveLength(0);
      expect(filterByVendor(schedulesData, vendorId)).toHaveLength(0);
      expect(filterByVendor(dsrData, vendorId)).toHaveLength(0);
      expect(filterByVendor(notificationsData, vendorId)).toHaveLength(0);
    }
  });

  it("union of all vendor-filtered results equals the full dataset", () => {
    const allVendors = ["vendor-gardanet", "vendor-ssi", "vendor-g4s"] as const;

    const allOrdersFiltered = allVendors.flatMap((v) => filterByVendor(ordersData, v));
    expect(allOrdersFiltered.length).toBe(ordersData.length);

    const allInvoicesFiltered = allVendors.flatMap((v) => filterByVendor(invoicesData, v));
    expect(allInvoicesFiltered.length).toBe(invoicesData.length);

    const allSchedulesFiltered = allVendors.flatMap((v) => filterByVendor(schedulesData, v));
    expect(allSchedulesFiltered.length).toBe(schedulesData.length);

    const allDsrFiltered = allVendors.flatMap((v) => filterByVendor(dsrData, v));
    expect(allDsrFiltered.length).toBe(dsrData.length);

    const allNotificationsFiltered = allVendors.flatMap((v) =>
      filterByVendor(notificationsData, v),
    );
    expect(allNotificationsFiltered.length).toBe(notificationsData.length);
  });
});

// =========================================================================
// Property 6: Date Range Filtering
// Validates: Requirements 3.5, 6.8, 6.9
// =========================================================================

describe("Property 6: Date Range Filtering", () => {
  it("filtered results contain only records within the date range (inclusive)", () => {
    fc.assert(
      fc.property(dateRangeArb(), ({ startDate, endDate }) => {
        const result = filterByDateRange(ordersData, startDate, endDate);

        for (const record of result) {
          if (startDate) {
            expect(record.scheduledDate >= startDate).toBe(true);
          }
          if (endDate) {
            expect(record.scheduledDate <= endDate).toBe(true);
          }
        }
      }),
      { numRuns: 100 },
    );
  });

  it("result contains exactly the records within bounds (no missing records)", () => {
    fc.assert(
      fc.property(dateRangeArb(), ({ startDate, endDate }) => {
        const result = filterByDateRange(ordersData, startDate, endDate);
        const resultIds = new Set(result.map((r) => r.id));

        for (const order of ordersData) {
          const inRange =
            (!startDate || order.scheduledDate >= startDate) &&
            (!endDate || order.scheduledDate <= endDate);

          if (inRange) {
            expect(resultIds.has(order.id)).toBe(true);
          } else {
            expect(resultIds.has(order.id)).toBe(false);
          }
        }
      }),
      { numRuns: 100 },
    );
  });

  it("if only startDate provided, includes all records from that date onward", () => {
    fc.assert(
      fc.property(dateInRange(), (startDate) => {
        const result = filterByDateRange(ordersData, startDate, undefined);

        const expected = ordersData.filter((o) => o.scheduledDate >= startDate);
        expect(result.length).toBe(expected.length);
        expect(result.every((r) => r.scheduledDate >= startDate)).toBe(true);
      }),
      { numRuns: 100 },
    );
  });

  it("if only endDate provided, includes all records up to and including that date", () => {
    fc.assert(
      fc.property(dateInRange(), (endDate) => {
        const result = filterByDateRange(ordersData, undefined, endDate);

        const expected = ordersData.filter((o) => o.scheduledDate <= endDate);
        expect(result.length).toBe(expected.length);
        expect(result.every((r) => r.scheduledDate <= endDate)).toBe(true);
      }),
      { numRuns: 100 },
    );
  });

  it("result is always a subset of the input", () => {
    fc.assert(
      fc.property(dateRangeArb(), ({ startDate, endDate }) => {
        const result = filterByDateRange(ordersData, startDate, endDate);
        expect(result.length).toBeLessThanOrEqual(ordersData.length);
        const allIds = new Set(ordersData.map((r) => r.id));
        expect(result.every((r) => allIds.has(r.id))).toBe(true);
      }),
      { numRuns: 100 },
    );
  });
});

// =========================================================================
// Property 7: Status Filter with AND Composition
// Validates: Requirements 3.4
// =========================================================================

describe("Property 7: Status Filter with AND Composition", () => {
  it("combined filter equals intersection of independent filters", () => {
    fc.assert(
      fc.property(orderStatusArb, dateRangeArb(), (status, { startDate, endDate }) => {
        const combined = filterByStatusAndDateRange(ordersData, status, startDate, endDate);

        const byStatus = filterByStatus(ordersData, status);
        const byDateRange = filterByDateRange(ordersData, startDate, endDate);

        // Intersection: records in both sets
        const byStatusIds = new Set(byStatus.map((r) => r.id));
        const intersection = byDateRange.filter((r) => byStatusIds.has(r.id));

        const combinedIds = new Set(combined.map((r) => r.id));
        const intersectionIds = new Set(intersection.map((r) => r.id));

        expect(combinedIds.size).toBe(intersectionIds.size);
        for (const id of combinedIds) {
          expect(intersectionIds.has(id)).toBe(true);
        }
      }),
      { numRuns: 100 },
    );
  });

  it("every record in combined result satisfies both status AND date range", () => {
    fc.assert(
      fc.property(orderStatusArb, dateRangeArb(), (status, { startDate, endDate }) => {
        const combined = filterByStatusAndDateRange(ordersData, status, startDate, endDate);

        for (const record of combined) {
          expect(record.status).toBe(status);
          if (startDate) {
            expect(record.scheduledDate >= startDate).toBe(true);
          }
          if (endDate) {
            expect(record.scheduledDate <= endDate).toBe(true);
          }
        }
      }),
      { numRuns: 100 },
    );
  });

  it("no record satisfying both conditions is missing from combined result", () => {
    fc.assert(
      fc.property(orderStatusArb, dateRangeArb(), (status, { startDate, endDate }) => {
        const combined = filterByStatusAndDateRange(ordersData, status, startDate, endDate);
        const combinedIds = new Set(combined.map((r) => r.id));

        for (const order of ordersData) {
          const matchesStatus = order.status === status;
          const matchesDateRange =
            (!startDate || order.scheduledDate >= startDate) &&
            (!endDate || order.scheduledDate <= endDate);

          if (matchesStatus && matchesDateRange) {
            expect(combinedIds.has(order.id)).toBe(true);
          }
        }
      }),
      { numRuns: 100 },
    );
  });
});
