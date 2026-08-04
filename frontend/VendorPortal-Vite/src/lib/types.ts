// Shared TypeScript interfaces and types for the Vendor Portal

export interface VendorUser {
  readonly id: string;
  readonly username: string;
  readonly displayName: string;
  readonly vendorId: string;
  readonly vendorName: string;
  readonly role: 'Vendor';
}

export interface AuthState {
  readonly token: string | null;
  readonly user: VendorUser | null;
  readonly isAuthenticated: boolean;
}

export interface JwtPayload {
  readonly sub: string;
  readonly auth_source: 'local';
  readonly role: 'Vendor';
  readonly vendor_id: string;
  readonly vendor_name: string;
  readonly display_name: string;
  readonly exp: number;
  readonly iat: number;
}

export interface CITOrder {
  readonly id: string;
  readonly atmId: string;
  readonly location: string;
  readonly orderType: 'Pickup' | 'Delivery';
  readonly scheduledDate: string; // ISO date
  readonly amount: number; // IDR integer
  readonly status: 'Scheduled' | 'In Transit' | 'Completed' | 'Failed';
  readonly vendorId: string;
  readonly hasEvidence: boolean;
}

export interface HandoverEvidence {
  readonly orderId: string;
  readonly files: readonly EvidenceFile[];
  readonly handoverTimestamp: string; // ISO datetime
  readonly recipientName: string;
  readonly notes?: string;
  readonly uploadedAt: string; // ISO datetime
}

export interface EvidenceFile {
  readonly name: string;
  readonly url: string;
  readonly type: 'image/jpeg' | 'image/png' | 'application/pdf';
  readonly size: number; // bytes
}

export interface Invoice {
  readonly id: string;
  readonly invoiceNumber: string;
  readonly period: string;
  readonly totalAmount: number; // IDR integer
  readonly lineItemsCount: number;
  readonly validationStatus: 'Uploaded' | 'Validated' | 'Mismatch Detected' | 'Approved';
  readonly vendorId: string;
  readonly lineItems: readonly InvoiceLineItem[];
}

export interface InvoiceLineItem {
  readonly description: string;
  readonly invoicedAmount: number;
  readonly matchedOrderRef: string;
  readonly expectedAmount: number;
  readonly variance: number;
  readonly matchStatus: 'Match' | 'Mismatch' | 'Pending';
}

export interface ReplenishmentSchedule {
  readonly id: string;
  readonly atmId: string;
  readonly location: string;
  readonly scheduledDate: string; // ISO date
  readonly recommendedAmount: number; // IDR integer
  readonly priority: 'High' | 'Medium' | 'Low';
  readonly status: 'Pending' | 'Confirmed' | 'Executed' | 'Cancelled';
  readonly vendorId: string;
}

export interface DsrRecord {
  readonly atmId: string;
  readonly location: string;
  readonly date: string; // ISO date
  readonly beginningBalance: number; // IDR integer
  readonly cashIn: number;
  readonly cashOut: number;
  readonly endingBalance: number;
  readonly vendorId: string;
}

export type BalanceStatus = 'Critical' | 'Low' | 'Normal';

export interface Notification {
  readonly id: string;
  readonly timestamp: string; // ISO datetime
  readonly type: 'New Assignment' | 'Order Status Changed' | 'Invoice Status Updated' | 'Schedule Updated';
  readonly message: string;
  readonly isRead: boolean;
  readonly vendorId: string;
  readonly relatedRoute: string;
}
