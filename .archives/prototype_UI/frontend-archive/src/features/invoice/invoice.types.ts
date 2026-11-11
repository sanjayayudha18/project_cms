export interface InvoiceLineItem {
  id: string;
  description: string;
  invoicedAmount: number;
  matchedOrderRef: string | null;
  expectedAmount: number;
  variance: number;
  matchStatus: 'Matched' | 'Mismatch' | 'Pending Review';
}

export interface Invoice {
  id: string;
  vendorId: string;
  period: string;
  totalAmount: number;
  lineItemsCount: number;
  validationStatus: 'Uploaded' | 'Validated' | 'Approved' | 'Mismatch Detected';
  validatorName: string | null;
  approverName: string | null;
  approvedAt: string | null;
  lineItems: InvoiceLineItem[];
}
