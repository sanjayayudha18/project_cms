/**
 * Approval inbox API client (backend/internal/handler/approval_handler.go,
 * mounted at /api/v1/approvals; the master-data detail is
 * approval_master_data_detail.go). Paths are relative to apiConfig.baseURL.
 */

import { api } from "@/lib/api/client";

export interface InboxItem {
  step_id: number;
  request_id: number;
  step_level: number;
  document_type: string;
  document_id: number;
  amount: string;
  maker_id: number;
}

export type MasterDataOp = "create" | "update" | "disable" | "enable";

export interface MasterDataChange {
  id: number;
  entity_type: string;
  entity_id: number | null;
  op: MasterDataOp;
  /** Proposed state (create/update) or the flag flip (disable/enable). */
  payload: Record<string, unknown> | null;
  /** The entity as it was when the change was submitted; null for create. */
  before: Record<string, unknown> | null;
  status: string;
  maker_id: number;
}

export interface MasterDataDetail {
  request_id: number;
  status: string;
  /** Set when the request is a CSV import batch (one approval for many rows). */
  batch_id: number | null;
  total: number;
  counts: Partial<Record<MasterDataOp, number>>;
  /** True when `changes` holds only the first rows of a bigger batch. */
  truncated: boolean;
  changes: MasterDataChange[];
}

const BASE = "/approvals";

export async function getInbox(): Promise<InboxItem[]> {
  const { data } = await api.get<InboxItem[]>(`${BASE}/inbox`);
  return data;
}

export async function getMasterDataDetail(requestId: number): Promise<MasterDataDetail> {
  const { data } = await api.get<MasterDataDetail>(`${BASE}/${requestId}/master-data`);
  return data;
}

export async function approveRequest(requestId: number): Promise<void> {
  await api.post(`${BASE}/${requestId}/approve`);
}

export async function rejectRequest(requestId: number): Promise<void> {
  await api.post(`${BASE}/${requestId}/reject`);
}
