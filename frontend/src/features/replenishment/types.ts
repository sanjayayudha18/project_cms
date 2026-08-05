/**
 * Replenishment feature local type definitions.
 * Types for replenishment schedules as consumed by ReplenishmentScreen.
 */

export type ReplenishmentStatus =
  | 'completed'
  | 'in-transit'
  | 'scheduled'
  | 'delayed'
  | 'pending-vendor';

export interface ReplenishmentSchedule {
  id: string;
  routeCode: string;
  region: string;
  vendor: string;
  windowStart: string;
  windowEnd: string;
  machineCount: number;
  completionCount: number;
  status: ReplenishmentStatus;
  cashValue: number;
}
