/**
 * Admin ATMs API client (backend/internal/handler/admin_atm_handler.go,
 * mounted at /api/v1/admin/atms). Paths here are relative to
 * apiConfig.baseURL ("/api/v1"), same convention as admin-vendors/api.ts.
 */

import { api } from "@/lib/api/client";
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

export async function createATM(payload: CreateATMPayload): Promise<AdminATM> {
  const { data } = await api.post<AdminATM>(BASE, payload);
  return data;
}

export async function updateATM(id: number, payload: UpdateATMPayload): Promise<AdminATM> {
  const { data } = await api.put<AdminATM>(`${BASE}/${id}`, payload);
  return data;
}

export async function disableATM(id: number): Promise<{ message: string }> {
  const { data } = await api.post<{ message: string }>(`${BASE}/${id}/disable`);
  return data;
}

export async function enableATM(id: number): Promise<{ message: string }> {
  const { data } = await api.post<{ message: string }>(`${BASE}/${id}/enable`);
  return data;
}

export async function listLocationOptions(): Promise<LocationOptionsResponse> {
  const { data } = await api.get<LocationOptionsResponse>(`${BASE}/locations`);
  return data;
}
