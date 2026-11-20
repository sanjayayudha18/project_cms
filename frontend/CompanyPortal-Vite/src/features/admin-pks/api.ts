/**
 * Daftar PKS API (/api/v1/admin/pks) + per-vendor PKS writes
 * (/api/v1/admin/vendors/{vendorID}/pks). Create / terminate / unterminate are
 * maker-checker (202 = staged); the reminder PUT applies immediately.
 */

import { api } from "@/lib/api/client";
import type { ChangeRequestAccepted } from "../master-data/changeRequest";
import type {
  PksListFilter,
  PksListPage,
  PksReasonPayload,
  SubmitPksPayload,
  UpdatePksReminderResponse,
} from "./types";

export async function listPks(filter: PksListFilter): Promise<PksListPage> {
  const q = new URLSearchParams();
  for (const [k, v] of Object.entries(filter)) {
    if (v !== undefined && v !== "") q.set(k, String(v));
  }
  const { data } = await api.get<PksListPage>(`/admin/pks?${q.toString()}`);
  return data;
}

export async function updatePksReminder(
  pksId: number,
  renewalDays: number,
): Promise<UpdatePksReminderResponse> {
  const { data } = await api.put<UpdatePksReminderResponse>(`/admin/pks/${pksId}/reminder`, {
    renewal_days: renewalDays,
  });
  return data;
}

const base = (vendorId: number) => `/admin/vendors/${vendorId}/pks`;

export async function submitVendorPks(
  vendorId: number,
  payload: SubmitPksPayload,
): Promise<ChangeRequestAccepted> {
  const { data } = await api.post<ChangeRequestAccepted>(base(vendorId), payload);
  return data;
}

export async function terminateVendorPks(
  vendorId: number,
  pksId: number,
  payload: PksReasonPayload,
): Promise<ChangeRequestAccepted> {
  const { data } = await api.post<ChangeRequestAccepted>(
    `${base(vendorId)}/${pksId}/terminate`,
    payload,
  );
  return data;
}

export async function unterminateVendorPks(
  vendorId: number,
  pksId: number,
  payload: PksReasonPayload,
): Promise<ChangeRequestAccepted> {
  const { data } = await api.post<ChangeRequestAccepted>(
    `${base(vendorId)}/${pksId}/unterminate`,
    payload,
  );
  return data;
}
