/**
 * Area ACM (cit-acm-plan FR7) -- backend/internal/handler/admin_acm_area_handler.go,
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
  branch_count: number;
  member_count: number;
}

export interface AcmAreaBranch {
  vendor_branch_id: number;
  branch_code: string;
  branch_name: string;
  region_code: string | null;
  branch_is_active: boolean;
  vendor_name: string;
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
  branches: AcmAreaBranch[];
  members: AcmAreaMember[];
}

export interface AcmBranchOption {
  vendor_branch_id: number;
  branch_code: string;
  branch_name: string;
  region_code: string | null;
  vendor_name: string;
  acm_area_id: number | null;
  acm_area_name: string | null;
}

export interface AcmEligibleUser {
  id: number;
  username: string;
  full_name: string;
  role: string;
}

export interface BranchWithoutArea {
  vendor_branch_id: number;
  branch_code: string;
  branch_name: string;
  vendor_name: string;
  request_count: number;
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

export function useAcmBranchOptions() {
  return useQuery<AcmBranchOption[], ApiError>({
    queryKey: [...KEY, "branch-options"],
    queryFn: async () =>
      (await api.get<{ branches: AcmBranchOption[] }>(`${BASE}/branch-options`)).data.branches,
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
  return useQuery<BranchWithoutArea[], ApiError>({
    queryKey: [...KEY, "warnings"],
    queryFn: async () =>
      (await api.get<{ branches_without_area: BranchWithoutArea[] }>(`${BASE}/warnings`)).data
        .branches_without_area,
  });
}

export type AcmAreaMutation =
  | { op: "create"; name: string }
  | { op: "rename"; id: number; name: string }
  | { op: "disable" | "enable"; id: number }
  | { op: "branches"; id: number; vendor_branch_ids: number[] }
  | { op: "members"; id: number; user_ids: number[] };

function send(m: AcmAreaMutation): Promise<unknown> {
  switch (m.op) {
    case "create":
      return api.post(BASE, { name: m.name });
    case "rename":
      return api.put(`${BASE}/${m.id}`, { name: m.name });
    case "branches":
      return api.put(`${BASE}/${m.id}/branches`, { vendor_branch_ids: m.vendor_branch_ids });
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
