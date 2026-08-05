/**
 * Dashboard feature local type definitions.
 * Types for KPI data, attention items, and replenishment schedules
 * as consumed by the dashboard components.
 */

// ─── MetricStrip Types ────────────────────────────────────────────────────────

export interface DashboardKpi {
  managedCash: number;
  managedCashDisplay: string;
  managedCashChange: number;
  atmAvailability: number;
  atmOnline: number;
  atmTotal: number;
  todayRoutes: number;
  routesCompleted: number;
  routesActive: number;
  exceptions: number;
  exceptionsHigh: number;
  exceptionsCutoffHour: number;
}

// ─── AttentionPanel Types ─────────────────────────────────────────────────────

export type AttentionCategory = "danger" | "warning" | "info";

export interface AttentionItem {
  id: string;
  category: AttentionCategory;
  icon: string;
  title: string;
  description: string;
  time: string;
}

// ─── ReplenishmentSummary Types ───────────────────────────────────────────────

export type ReplenishmentStatus =
  | "completed"
  | "in-transit"
  | "scheduled"
  | "delayed"
  | "pending-vendor";

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

// ─── Legacy Types (retained for MetricCard property test) ────────────────────

export interface DashboardMetrics {
  activeMachines: number;
  pendingFillInstructions: number;
  openReconciliationItems: number;
  pendingApprovals: number;
}

export interface ActivityEvent {
  id: string;
  type: "upload" | "approval" | "reconciliation" | "generation";
  description: string;
  timestamp: string;
  actor: string;
}

// ─── Shared ───────────────────────────────────────────────────────────────────

export type BadgeVariant = "success" | "warning" | "danger" | "info" | "neutral";

export type ProgressBarStatus = "in-transit" | "completed" | "delayed";
