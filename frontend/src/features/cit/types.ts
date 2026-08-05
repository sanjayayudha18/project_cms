export type CitStatus = 'Scheduled' | 'In Transit' | 'Completed' | 'Failed';

export interface CitOrder {
  id: string;
  atmId: string;
  vendorId: string;
  orderDate: string;
  scheduledDate: string;
  amount: number;
  status: CitStatus;
  evidenceUrl: string | null;
}

export interface EnrichedCitOrder extends CitOrder {
  vendorName: string;
  atmLocation: string;
}
