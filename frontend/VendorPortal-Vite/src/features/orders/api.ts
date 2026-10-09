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
  readonly counterpart: PartyBranch;
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
  readonly totals: readonly DenomAmount[] | null;
  readonly currency: string;
  readonly sent_at: string | null;
  readonly decided_at: string | null;
  readonly can_decide: boolean;
}

export interface VendorOrderDetail extends VendorOrderSummary {
  readonly rejection_reason: string | null;
  readonly content: {
    readonly role: PartyRole;
    readonly branch: PartyBranch;
    readonly atms: readonly PartyAtm[];
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
