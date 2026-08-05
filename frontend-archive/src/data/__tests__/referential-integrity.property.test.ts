/**
 * Property 10: Mock data referential integrity
 *
 * For any ATM ID referenced in reconciliation-exceptions.json, that ID SHALL exist in atms.json.
 * For any vendor name in replenishment-schedules.json, that name SHALL exist in vendors.json.
 * The dashboard KPI atmOnline value SHALL not exceed atmTotal.
 *
 * Feature: cms-dashboard-redesign, Property 10: Mock data referential integrity
 * Validates: Requirements 11.5
 */
import { describe, it, expect } from 'vitest';
import atms from '@/data/atms.json';
import vendors from '@/data/vendors.json';
import reconciliationExceptions from '@/data/reconciliation-exceptions.json';
import replenishmentSchedules from '@/data/replenishment-schedules.json';
import dashboardKpi from '@/data/dashboard-kpi.json';

// Build lookup sets for fast membership testing
const atmIds = new Set(atms.map((a) => a.id));
const vendorNames = new Set(vendors.map((v) => v.name));

describe('Feature: cms-dashboard-redesign, Property 10: Mock data referential integrity', () => {
  it('every atmId in reconciliation-exceptions.json exists in atms.json', () => {
    for (const exception of reconciliationExceptions) {
      expect(atmIds.has(exception.atmId)).toBe(true);
    }
  });

  it('every vendor in replenishment-schedules.json exists in vendors.json', () => {
    for (const schedule of replenishmentSchedules) {
      expect(vendorNames.has(schedule.vendor)).toBe(true);
    }
  });

  it('dashboard-kpi.json atmOnline does not exceed atmTotal', () => {
    expect(dashboardKpi.atmOnline).toBeLessThanOrEqual(dashboardKpi.atmTotal);
  });

  it('every replenishment schedule has completionCount <= machineCount', () => {
    for (const schedule of replenishmentSchedules) {
      expect(schedule.completionCount).toBeLessThanOrEqual(schedule.machineCount);
    }
  });

  it('every reconciliation exception has difference === countedAmount - escrowAmount', () => {
    for (const exception of reconciliationExceptions) {
      expect(exception.difference).toBe(exception.countedAmount - exception.escrowAmount);
    }
  });
});
