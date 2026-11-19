/**
 * Vendor PKS API (backend/internal/handler, mounted at
 * /api/v1/admin/vendors/{vendorID}/pks). Types come from the generated schema.
 * Submit / terminate / unterminate are maker-checker: 202 = staged, not saved.
 */

import { api } from "@/lib/api/client";
import type { components } from "@/lib/api/schema";
import type { ChangeRequestAccepted } from "../master-data/changeRequest";

export type VendorPksDetail = components["schemas"]["VendorPksDetail"];
export type SubmitVendorPksPayload = components["schemas"]["SubmitVendorPksRequest"];
export type VendorPksReasonPayload = components["schemas"]["VendorPksReasonRequest"];

export type PksType = components["schemas"]["PksTypeList"]["types"][number];
export type CreatePksTypePayload = components["schemas"]["CreatePksTypeRequest"];
export type UpdatePksTypePayload = components["schemas"]["UpdatePksTypeRequest"];

const base = (vendorId: number) => `/admin/vendors/${vendorId}/pks`;

/** PKS type master (maker-checker; 202 = staged). Mounted at /api/v1/admin/pks-types. */
export async function listPksTypes(): Promise<PksType[]> {
  const { data } = await api.get<components["schemas"]["PksTypeList"]>("/admin/pks-types");
  return data.types;
}

export async function submitPksType(payload: CreatePksTypePayload): Promise<ChangeRequestAccepted> {
  const { data } = await api.post<ChangeRequestAccepted>("/admin/pks-types", payload);
  return data;
}

export async function updatePksType(
  id: number,
  payload: UpdatePksTypePayload,
): Promise<ChangeRequestAccepted> {
  const { data } = await api.put<ChangeRequestAccepted>(`/admin/pks-types/${id}`, payload);
  return data;
}

export async function disablePksType(id: number): Promise<ChangeRequestAccepted> {
  const { data } = await api.post<ChangeRequestAccepted>(`/admin/pks-types/${id}/disable`);
  return data;
}

export async function enablePksType(id: number): Promise<ChangeRequestAccepted> {
  const { data } = await api.post<ChangeRequestAccepted>(`/admin/pks-types/${id}/enable`);
  return data;
}

export async function getVendorPks(vendorId: number): Promise<VendorPksDetail> {
  const { data } = await api.get<VendorPksDetail>(base(vendorId));
  return data;
}

export async function submitVendorPks(
  vendorId: number,
  payload: SubmitVendorPksPayload,
): Promise<ChangeRequestAccepted> {
  const { data } = await api.post<ChangeRequestAccepted>(base(vendorId), payload);
  return data;
}

export async function terminateVendorPks(
  vendorId: number,
  pksId: number,
  payload: VendorPksReasonPayload,
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
  payload: VendorPksReasonPayload,
): Promise<ChangeRequestAccepted> {
  const { data } = await api.post<ChangeRequestAccepted>(
    `${base(vendorId)}/${pksId}/unterminate`,
    payload,
  );
  return data;
}
