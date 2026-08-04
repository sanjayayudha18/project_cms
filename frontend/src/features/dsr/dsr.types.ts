export interface DsrRecord {
  id: string;
  atmId: string;
  date: string;
  beginningBalance: number;
  cashIn: number;
  cashOut: number;
  endingBalance: number;
  status: 'Critical' | 'Low' | 'Normal';
}

export interface EnrichedDsrRecord extends DsrRecord {
  location: string;
  vendorName: string;
}
