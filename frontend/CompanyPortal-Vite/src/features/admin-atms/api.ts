/**
 * Admin ATMs API client (backend/internal/handler/admin_atm_handler.go,
 * mounted at /api/v1/admin/atms). Paths here are relative to
 * apiConfig.baseURL ("/api/v1"), same convention as admin-vendors/api.ts.
 */

import { api } from "@/lib/api/client";
import type { ChangeRequestAccepted } from "../master-data/changeRequest";
import type {
  AdminATM,
  AdminATMsListParams,
  AdminATMsListResponse,
  CreateATMPayload,
  LocationOptionsResponse,
  UpdateATMPayload,
} from "./types";

const BASE = "/admin/atms";

function toQueryString(params: AdminATMsListParams): string {
  const q = new URLSearchParams();
  q.set("page", String(params.page));
  q.set("page_size", String(params.page_size));
  if (params.status) q.set("status", params.status);
  if (params.q) q.set("q", params.q);
  if (params.brand) q.set("brand", params.brand);
  if (params.machine_type) q.set("machine_type", params.machine_type);
  if (params.deployment_type) q.set("deployment_type", params.deployment_type);
  if (params.priority_class) q.set("priority_class", params.priority_class);
  if (params.location_id) q.set("location_id", String(params.location_id));
  return q.toString();
}

export async function listATMs(params: AdminATMsListParams): Promise<AdminATMsListResponse> {
  const { data } = await api.get<AdminATMsListResponse>(`${BASE}?${toQueryString(params)}`);
  return data;
}

export async function getATM(id: number): Promise<AdminATM> {
  const { data } = await api.get<AdminATM>(`${BASE}/${id}`);
  return data;
}

// Writes are maker-checker (plan.md D1): they answer 202 with the staged change
// request, not the saved ATM. The list only changes once it is approved.
export async function createATM(payload: CreateATMPayload): Promise<ChangeRequestAccepted> {
  const { data } = await api.post<ChangeRequestAccepted>(BASE, payload);
  return data;
}

export async function updateATM(
  id: number,
  payload: UpdateATMPayload,
): Promise<ChangeRequestAccepted> {
  const { data } = await api.put<ChangeRequestAccepted>(`${BASE}/${id}`, payload);
  return data;
}

export async function disableATM(id: number): Promise<ChangeRequestAccepted> {
  const { data } = await api.post<ChangeRequestAccepted>(`${BASE}/${id}/disable`);
  return data;
}

export async function enableATM(id: number): Promise<ChangeRequestAccepted> {
  const { data } = await api.post<ChangeRequestAccepted>(`${BASE}/${id}/enable`);
  return data;
}

export async function listLocationOptions(): Promise<LocationOptionsResponse> {
  const { data } = await api.get<LocationOptionsResponse>(`${BASE}/locations`);
  return data;
}

// ATM kelolaan (effective-dated vendor package assignments): admin_atm_assignment_handler.go,
// mounted at /api/v1/admin/atms/{id}/assignments. Create is maker-checker (202).
// No priority_class -- vendor_packages lost it in migration 010 (price/class
// now live in vendor_package_prices, keyed off the ATM's own machine_type/
// priority_class instead of the package).
export interface ATMAssignment {
  id: number;
  atm_id: number;
  vendor_package_id: number;
  package_code: string;
  effective_start_date: string;
  effective_end_date: string | null;
  is_active: boolean;
}

export interface CreateATMAssignmentPayload {
  vendor_package_id: number;
  effective_start_date: string;
  effective_end_date: string | null;
}

export async function listATMAssignments(atmId: number): Promise<ATMAssignment[]> {
  const { data } = await api.get<{ assignments: ATMAssignment[] }>(
    `${BASE}/${atmId}/assignments?page=1&page_size=100&status=all`,
  );
  return data.assignments ?? [];
}

export async function createATMAssignment(
  atmId: number,
  payload: CreateATMAssignmentPayload,
): Promise<ChangeRequestAccepted> {
  const { data } = await api.post<ChangeRequestAccepted>(`${BASE}/${atmId}/assignments`, payload);
  return data;
}
