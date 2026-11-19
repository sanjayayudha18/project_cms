/**
 * PKS type master API (mounted at /api/v1/admin/pks-types). Types come from the generated schema.
 * Writes are maker-checker: 202 = staged, not saved. PKS records live in features/admin-pks.
 */

import { api } from "@/lib/api/client";
import type { components } from "@/lib/api/schema";
import type { ChangeRequestAccepted } from "../master-data/changeRequest";

export type PksType = components["schemas"]["PksTypeList"]["types"][number];
export type CreatePksTypePayload = components["schemas"]["CreatePksTypeRequest"];
export type UpdatePksTypePayload = components["schemas"]["UpdatePksTypeRequest"];

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
