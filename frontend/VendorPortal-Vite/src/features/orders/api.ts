/**
 * Replenish orders sent to this vendor (backend /api/v1/vendor/replenish-orders,
 * .claude/sdlc/cit-send-vendor FR3/FR4/FR9). One row = one "party": this
 * vendor branch acting as replenish (pelaksana) or vault (penyedia uang) for
 * one Vendor Request. Scope is enforced on the server; there is no vendor
 * parameter. Replaces the data/orders.json mock.
 */

import { api } from "@/lib/api/client";

export type PartyRole = "replenish" | "vault";
export type PartyStatus = "pending" | "accepted" | "rejected" | "withdrawn";

export interface DenomAmount {
  readonly denom: number;
  readonly amount: number; // full IDR
}

export interface PartyBranch {
  readonly id: number;
  readonly code: string;
  readonly name: string;
  readonly vendor_id: number;
  readonly vendor_name: string;
  readonly address?: string;
}

export interface PartyAtm {
  readonly terminal_id: string;
  readonly lokasi: string;
  readonly ticket_number: string;
  readonly denoms: readonly DenomAmount[];
}

/** A vendor vault as shown to vendors; address = vault's, else its branch's. */
export interface PartyVaultRef {
  readonly id: number;
  readonly code: string;
  readonly address?: string;
}

/** One vault of the request with its tiket pemenuhan vault (cit-vault-select FR7). */
export interface PartyVault {
  readonly ticket_number: string;
  readonly vault: PartyVaultRef;
  readonly branch: PartyBranch;
  readonly denoms: readonly DenomAmount[];
}

export interface VendorOrderSummary {
  readonly id: number;
  readonly role: PartyRole;
  readonly status: PartyStatus;
  readonly request_number: string;
  readonly replenish_date: string | null; // RFC 3339 date at 00:00 UTC
  readonly request_status: string;
  readonly is_canceled: boolean;
  readonly branch_id: number;
  readonly branch_code: string;
  readonly branch_name: string;
  readonly atm_count: number;
  readonly vault_count: number;
  readonly totals: readonly DenomAmount[] | null;
  readonly currency: string;
  readonly sent_at: string | null;
  readonly decided_at: string | null;
  readonly can_decide: boolean;
}

/** Vendor realisasi report of one ATM (replenishment-realisasi FR2). */
export interface RealizationView {
  readonly terminal_id: string;
  readonly ticket_number: string;
  readonly result: "success" | "failed" | null;
  readonly realized_date: string | null; // RFC 3339 date at 00:00 UTC
  readonly amounts: readonly DenomAmount[];
  readonly submitted_at: string | null;
  readonly submitted_by_name: string;
}

export interface VendorOrderDetail extends VendorOrderSummary {
  readonly rejection_reason: string | null;
  /** Replenish party only: the report sent so far (empty before the first report). */
  readonly realizations?: readonly RealizationView[];
  /** Replenish party may send (or resend after a rejection) the realisasi report now. */
  readonly can_report?: boolean;
  /** Why CIMB rejected the last report (shown until the vendor resends). */
  readonly completion_rejection_reason?: string | null;
  readonly content: {
    readonly role: PartyRole;
    readonly branch: PartyBranch;
    /** Replenish party: its own ATMs; empty for a vault party. */
    readonly atms: readonly PartyAtm[];
    /** Replenish party: every vault of the request; vault party: its own vaults. */
    readonly vaults: readonly PartyVault[];
    /** Vault party: the request's replenish branches (who collects); empty otherwise. */
    readonly replenish_branches: readonly PartyBranch[];
    readonly totals: readonly DenomAmount[];
    readonly currency: string;
  };
}

export interface VendorOrderList {
  readonly items: readonly VendorOrderSummary[];
  readonly total: number;
  readonly page: number;
  readonly page_size: number;
}

export interface VendorOrderFilter {
  readonly partyStatus: PartyStatus | "";
  readonly from: string | null; // YYYY-MM-DD
  readonly to: string | null;
  readonly page: number;
}

export const ORDER_PAGE_SIZE = 20;
const BASE = "/api/v1/vendor/replenish-orders";

export async function fetchOrders(f: VendorOrderFilter): Promise<VendorOrderList> {
  const search = new URLSearchParams({ page: String(f.page), page_size: String(ORDER_PAGE_SIZE) });
  if (f.partyStatus) search.set("party_status", f.partyStatus);
  if (f.from) search.set("from", f.from);
  if (f.to) search.set("to", f.to);
  const { data } = await api.get<VendorOrderList>(`${BASE}?${search}`);
  return data;
}

export async function fetchOrder(id: number): Promise<VendorOrderDetail> {
  const { data } = await api.get<VendorOrderDetail>(`${BASE}/${id}`);
  return data;
}

export async function acceptOrder(id: number): Promise<VendorOrderDetail> {
  const { data } = await api.post<VendorOrderDetail>(`${BASE}/${id}/accept`);
  return data;
}

export async function rejectOrder(id: number, reason: string): Promise<VendorOrderDetail> {
  const { data } = await api.post<VendorOrderDetail>(`${BASE}/${id}/reject`, { reason });
  return data;
}

/** One ATM in POST /{id}/realization: success needs a date + at least one amount, failed neither. */
export interface RealizationInput {
  readonly terminal_id: string;
  readonly result: "success" | "failed";
  readonly realized_date?: string; // YYYY-MM-DD
  readonly amounts?: Readonly<Record<string, number>>; // denom -> full IDR
}

export async function submitRealization(
  id: number,
  realizations: readonly RealizationInput[],
): Promise<VendorOrderDetail> {
  const { data } = await api.post<VendorOrderDetail>(`${BASE}/${id}/realization`, { realizations });
  return data;
}

// --- Realisasi page (GET /api/v1/vendor/replenish-realizations, FR5.2) ------

export type Classification =
  | "pending"
  | "on_schedule"
  | "early"
  | "late"
  | "not_done"
  | "no_order"
  | "void";

export interface ClassificationAmount {
  readonly denom: number;
  readonly order_amount: number;
  readonly itm_amount: number;
  readonly vendor_amount: number | null;
  readonly diff: number;
}

export interface VendorClassification {
  readonly id: number;
  readonly ticket_number: string | null;
  readonly request_number: string | null;
  readonly terminal_id: string;
  readonly branch_code: string | null;
  readonly scheduled_date: string | null;
  readonly realized_date: string | null;
  readonly workday_offset: number | null;
  readonly classification: Classification;
  readonly has_amount_diff: boolean;
  readonly vendor_mismatch: boolean;
  readonly vendor_report_late: boolean;
  readonly amounts: readonly ClassificationAmount[];
}

export interface VendorClassificationList {
  readonly items: readonly VendorClassification[];
  readonly total: number;
  readonly page: number;
  readonly page_size: number;
}

export interface RealizationFilter {
  readonly from: string; // YYYY-MM-DD, required, max 31 days (K5)
  readonly to: string;
  readonly classification: Classification | "";
  readonly page: number;
}

export async function fetchRealizations(f: RealizationFilter): Promise<VendorClassificationList> {
  const search = new URLSearchParams({
    date_from: f.from,
    date_to: f.to,
    page: String(f.page),
    page_size: String(ORDER_PAGE_SIZE),
  });
  if (f.classification) search.set("classification", f.classification);
  const { data } = await api.get<VendorClassificationList>(
    `/api/v1/vendor/replenish-realizations?${search}`,
  );
  return data;
}
