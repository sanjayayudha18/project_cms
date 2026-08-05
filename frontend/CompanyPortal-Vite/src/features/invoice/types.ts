export type MatchStatus = 'Matched' | 'Mismatch' | 'Pending Review';

export type ValidationStatus = 'Uploaded' | 'Validated' | 'Approved' | 'Mismatch Detected';

export interface InvoiceLineItem {
  id: string;
  description: string;
  invoicedAmount: number;
  matchedOrderRef: string | null;
  expectedAmount: number;
  variance: number;
  matchStatus: MatchStatus;
}

export interface Invoice {
  id: string;
  vendorId: string;
  period: string;
  totalAmount: number;
  lineItemsCount: number;
  validationStatus: ValidationStatus;
  validatorName: string | null;
  approverName: string | null;
  approvedAt: string | null;
  lineItems: InvoiceLineItem[];
}

export interface InvoiceWithVendor extends Invoice {
  vendorName: string;
}
