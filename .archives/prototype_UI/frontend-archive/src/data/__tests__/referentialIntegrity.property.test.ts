/**
 * Property-based test: Mock data referential integrity
 *
 * Validates: Requirements 7.5
 */
import { describe, it, expect } from 'vitest';
import * as fc from 'fast-check';
import atms from '@/data/atms.json';
import vendors from '@/data/vendors.json';
import citOrders from '@/data/cit-orders.json';
import invoices from '@/data/invoices.json';
import forecast from '@/data/forecast.json';
import dsr from '@/data/dsr.json';

// Build lookup sets for fast membership testing
const atmIds = new Set(atms.map((a) => a.id));
const vendorIds = new Set(vendors.map((v) => v.id));
const citOrderIds = new Set(citOrders.map((c) => c.id));

describe('Feature: cms-frontend-prototype, Property 6: Mock data referential integrity', () => {
  it('every CIT order atmId exists in atms.json', () => {
    fc.assert(
      fc.property(
        fc.constantFrom(...citOrders),
        (order) => {
          expect(atmIds.has(order.atmId)).toBe(true);
        }
      ),
      { numRuns: 100 }
    );
  });

  it('every CIT order vendorId exists in vendors.json', () => {
    fc.assert(
      fc.property(
        fc.constantFrom(...citOrders),
        (order) => {
          expect(vendorIds.has(order.vendorId)).toBe(true);
        }
      ),
      { numRuns: 100 }
    );
  });

  it('every invoice vendorId exists in vendors.json', () => {
    fc.assert(
      fc.property(
        fc.constantFrom(...invoices),
        (invoice) => {
          expect(vendorIds.has(invoice.vendorId)).toBe(true);
        }
      ),
      { numRuns: 100 }
    );
  });

  it('every invoice line item matchedOrderRef exists in cit-orders.json (when non-null)', () => {
    const lineItemsWithRef = invoices.flatMap((inv) =>
      inv.lineItems.filter((li) => li.matchedOrderRef !== null)
    );

    fc.assert(
      fc.property(
        fc.constantFrom(...lineItemsWithRef),
        (lineItem) => {
          expect(citOrderIds.has(lineItem.matchedOrderRef!)).toBe(true);
        }
      ),
      { numRuns: 100 }
    );
  });

  it('every forecast record atmId exists in atms.json', () => {
    fc.assert(
      fc.property(
        fc.constantFrom(...forecast),
        (record) => {
          expect(atmIds.has(record.atmId)).toBe(true);
        }
      ),
      { numRuns: 100 }
    );
  });

  it('every DSR record atmId exists in atms.json', () => {
    fc.assert(
      fc.property(
        fc.constantFrom(...dsr),
        (record) => {
          expect(atmIds.has(record.atmId)).toBe(true);
        }
      ),
      { numRuns: 100 }
    );
  });
});
