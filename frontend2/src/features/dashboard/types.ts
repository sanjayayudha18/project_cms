// ─── Dashboard Domain Types ───────────────────────────────────────────────────

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
