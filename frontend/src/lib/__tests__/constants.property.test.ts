/**
 * Property-based test: Role-based navigation visibility
 *
 * Validates: Requirements 2.4, 2.5
 */
import { describe, it, expect } from 'vitest';
import * as fc from 'fast-check';
import { filterNavByRole, INTERNAL_ROLES, type Role, type NavItem } from '../constants';

// Arbitrary for Role type
const arbRole = fc.constantFrom<Role>('Admin', 'Operator', 'Manager', 'Vendor');

// Arbitrary for NavItem-like objects with optional internalOnly (legacy field)
const mockIcon = (() => null) as unknown as NavItem['icon'];

const arbNavItem = fc.record({
  path: fc.string({ minLength: 1, maxLength: 20 }).map((s) => `/${s}`),
  label: fc.string({ minLength: 1, maxLength: 30 }),
  icon: fc.constant(mockIcon),
  internalOnly: fc.boolean(),
});

const arbNavItems = fc.array(arbNavItem, { minLength: 0, maxLength: 20 });

describe('Feature: cms-frontend-prototype, Property 1: Role-based navigation visibility', () => {
  it('item is visible iff role is internal OR item.internalOnly is false', () => {
    fc.assert(
      fc.property(arbNavItems, arbRole, (items, role) => {
        const result = filterNavByRole(items, role);
        const isInternal = (INTERNAL_ROLES as readonly string[]).includes(role);

        for (const item of items) {
          const shouldBeVisible = isInternal || !item.internalOnly;
          const isVisible = result.includes(item);
          expect(isVisible).toBe(shouldBeVisible);
        }
      }),
      { numRuns: 100 }
    );
  });

  it('Vendor role sees only items with internalOnly === false', () => {
    fc.assert(
      fc.property(arbNavItems, (items) => {
        const result = filterNavByRole(items, 'Vendor');

        // Every returned item must have internalOnly === false
        for (const item of result) {
          expect(item.internalOnly).toBe(false);
        }

        // Every item with internalOnly === false must be in the result
        const expected = items.filter((item) => !item.internalOnly);
        expect(result).toEqual(expected);
      }),
      { numRuns: 100 }
    );
  });
});
