/**
 * Area ACM (cit-acm-plan FR7, cit-vault-select FR1: area per region) --
 * backend/internal/handler/admin_acm_area_handler.go,
 * mounted at /api/v1/admin/acm-areas (ADMIN only). Writes apply immediately
 * (documented GR#3 deviation, audited in the same tx) -- no maker-checker.
 */

import type { ApiError } from "@/lib/api/client";
import { api } from "@/lib/api/client";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

export interface AcmArea {
  id: number;
  name: string;
  is_active: boolean;
  deleted_at: string | null;
  region_count: number;
  member_count: number;
}

export interface AcmAreaMember {
  user_id: number;
  username: string;
  full_name: string;
  role: string;
  user_is_active: boolean;
}

export interface AcmAreaDetail {
  id: number;
  name: string;
  is_active: boolean;
  region_codes: string[];
  members: AcmAreaMember[];
}

/** A region code used by an active vendor branch, with the area holding it (null = free). */
export interface AcmRegionOption {
  region_code: string;
  branch_count: number;
  acm_area_id: number | null;
  acm_area_name: string | null;
}

export interface AcmEligibleUser {
  id: number;
  username: string;
  full_name: string;
  role: string;
}

/** FR1.4: a request waiting in vault_assignment with no plan (region without area / no region code). */
export interface RequestWithoutArea {
  vendor_request_id: number;
  request_number: string;
  region_code: string | null;
  replenish_date: string | null;
}

const BASE = "/admin/acm-areas";
const KEY = ["admin-acm-areas"] as const;

export function useAcmAreas() {
  return useQuery<AcmArea[], ApiError>({
    queryKey: [...KEY, "list"],
    queryFn: async () => (await api.get<{ areas: AcmArea[] }>(`${BASE}?status=all`)).data.areas,
  });
}

export function useAcmArea(id: number | null) {
  return useQuery<AcmAreaDetail, ApiError>({
    queryKey: [...KEY, "detail", id],
    queryFn: async () => (await api.get<AcmAreaDetail>(`${BASE}/${id}`)).data,
    enabled: id !== null,
  });
}

export function useAcmRegionOptions() {
  return useQuery<AcmRegionOption[], ApiError>({
    queryKey: [...KEY, "region-options"],
    queryFn: async () =>
      (await api.get<{ regions: AcmRegionOption[] }>(`${BASE}/region-options`)).data.regions,
  });
}

export function useAcmEligibleUsers() {
  return useQuery<AcmEligibleUser[], ApiError>({
    queryKey: [...KEY, "eligible-users"],
    queryFn: async () =>
      (await api.get<{ users: AcmEligibleUser[] }>(`${BASE}/eligible-users`)).data.users,
  });
}

export function useAcmAreaWarnings() {
  return useQuery<RequestWithoutArea[], ApiError>({
    queryKey: [...KEY, "warnings"],
    queryFn: async () =>
      (await api.get<{ requests_without_area: RequestWithoutArea[] }>(`${BASE}/warnings`)).data
        .requests_without_area,
  });
}

export type AcmAreaMutation =
  | { op: "create"; name: string }
  | { op: "rename"; id: number; name: string }
  | { op: "disable" | "enable"; id: number }
  | { op: "regions"; id: number; region_codes: string[] }
  | { op: "members"; id: number; user_ids: number[] };

function send(m: AcmAreaMutation): Promise<unknown> {
  switch (m.op) {
    case "create":
      return api.post(BASE, { name: m.name });
    case "rename":
      return api.put(`${BASE}/${m.id}`, { name: m.name });
    case "regions":
      return api.put(`${BASE}/${m.id}/regions`, { region_codes: m.region_codes });
    case "members":
      return api.put(`${BASE}/${m.id}/members`, { user_ids: m.user_ids });
    default:
      return api.post(`${BASE}/${m.id}/${m.op}`);
  }
}

export function useAcmAreaMutation() {
  const queryClient = useQueryClient();
  return useMutation<unknown, ApiError, AcmAreaMutation>({
    mutationFn: send,
    onSuccess: () => queryClient.invalidateQueries({ queryKey: KEY }),
  });
}
