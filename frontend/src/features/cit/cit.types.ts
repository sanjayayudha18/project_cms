export interface CitOrder {
  id: string;
  atmId: string;
  vendorId: string;
  orderDate: string;
  scheduledDate: string;
  amount: number;
  status: 'Scheduled' | 'In Transit' | 'Completed' | 'Failed';
  evidenceUrl: string | null;
}

export type CitStatus = CitOrder['status'];

export interface EnrichedCitOrder extends CitOrder {
  vendorName: string;
  atmLocation: string;
}
