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

const base = (vendorId: number) => `/admin/vendors/${vendorId}/pks`;

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
