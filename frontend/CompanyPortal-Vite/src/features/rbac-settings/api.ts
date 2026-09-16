/**
 * RBAC Settings Menu API client (backend/internal/handler/rbac_list_handler.go
 * + the existing admin_approval_handler.go writes). All paths are relative to
 * apiConfig.baseURL ("/api/v1"), so "/admin/approval/..." here maps to
 * "/api/v1/admin/approval/..." on the wire.
 */

import { api } from "@/lib/api/client";
import type {
  DelegationFormValues,
  HierarchyFormValues,
  LeaveFormValues,
  PolicyFormValues,
  RbacDelegation,
  RbacDelegationsResponse,
  RbacLeavesResponse,
  RbacPoliciesResponse,
  RbacPolicy,
  RbacUsersResponse,
} from "./types";

const BASE = "/admin/approval";

// ─── Reads ─────────────────────────────────────────────────────────────────

export async function getUserHierarchy(): Promise<RbacUsersResponse> {
  const { data } = await api.get<RbacUsersResponse>(`${BASE}/users/hierarchy`);
  return data;
}

export async function getDelegations(): Promise<RbacDelegationsResponse> {
  const { data } = await api.get<RbacDelegationsResponse>(`${BASE}/delegations`);
  return data;
}

export async function getLeaves(): Promise<RbacLeavesResponse> {
  const { data } = await api.get<RbacLeavesResponse>(`${BASE}/leaves`);
  return data;
}

export async function getPolicies(): Promise<RbacPoliciesResponse> {
  const { data } = await api.get<RbacPoliciesResponse>(`${BASE}/policies`);
  return data;
}

// ─── Writes ────────────────────────────────────────────────────────────────

interface SetHierarchyResponse {
  id: number;
  supervisor_id: number | null;
  approval_level: number | null;
}

export async function setUserHierarchy(
  userId: number,
  values: HierarchyFormValues,
): Promise<SetHierarchyResponse> {
  const { data } = await api.put<SetHierarchyResponse>(`${BASE}/users/${userId}/hierarchy`, {
    supervisor_id: values.supervisor_id,
    approval_level: values.approval_level,
  });
  return data;
}

export async function createDelegation(values: DelegationFormValues): Promise<RbacDelegation> {
  const { data } = await api.post<RbacDelegation>(`${BASE}/delegations`, values);
  return data;
}

export async function revokeDelegation(id: number): Promise<RbacDelegation> {
  const { data } = await api.delete<RbacDelegation>(`${BASE}/delegations/${id}`);
  return data;
}

interface CreateLeaveResponse {
  id: number;
  user_id: number;
  start_at: string;
  end_at: string;
}

export async function createLeave(values: LeaveFormValues): Promise<CreateLeaveResponse> {
  const { data } = await api.post<CreateLeaveResponse>(`${BASE}/leaves`, values);
  return data;
}

export async function createPolicy(values: PolicyFormValues): Promise<RbacPolicy> {
  const { data } = await api.post<RbacPolicy>(`${BASE}/policies`, values);
  return data;
}

export async function updatePolicy(id: number, values: PolicyFormValues): Promise<RbacPolicy> {
  const { data } = await api.put<RbacPolicy>(`${BASE}/policies/${id}`, values);
  return data;
}
