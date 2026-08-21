export type DsrStatus = "Critical" | "Low" | "Normal";

export interface DsrRecord {
  id: string;
  atmId: string;
  date: string;
  beginningBalance: number;
  cashIn: number;
  cashOut: number;
  endingBalance: number;
  status: DsrStatus;
}

export interface EnrichedDsrRecord extends DsrRecord {
  location: string;
  vendorName: string;
}

export interface DsrSummaryTotals {
  beginningBalance: number;
  cashIn: number;
  cashOut: number;
  endingBalance: number;
}
