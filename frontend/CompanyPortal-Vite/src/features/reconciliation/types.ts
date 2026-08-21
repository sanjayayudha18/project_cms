export interface ReconciliationException {
  id: string;
  atmId: string;
  lastCountTime: string;
  location: string;
  countedAmount: number;
  escrowAmount: number;
  difference: number;
  severity: "high" | "medium";
  owner: string | null;
}
