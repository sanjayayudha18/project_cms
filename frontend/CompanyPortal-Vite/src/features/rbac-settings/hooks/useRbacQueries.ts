/**
 * TanStack Query hooks for the RBAC Settings Menu. Each list endpoint gets a
 * useQuery hook; each write gets a useMutation hook that invalidates only its
 * own entity's query key on success (design.md: "Mutations invalidate the
 * relevant key on success so the table refreshes" -- Requirements 4.4, 5.5, 6.3),
 * unlike vendor-request's hooks.ts which invalidates its whole prefix.
 */

import type { ApiError } from "@/lib/api/client";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import {
  createDelegation,
  createLeave,
  createPolicy,
  getDelegations,
  getLeaves,
  getPolicies,
  getUserHierarchy,
  revokeDelegation,
  setUserHierarchy,
  updatePolicy,
} from "../api";
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
} from "../types";

export const rbacKeys = {
  all: ["rbac-settings"] as const,
  users: () => [...rbacKeys.all, "users"] as const,
  delegations: () => [...rbacKeys.all, "delegations"] as const,
  leaves: () => [...rbacKeys.all, "leaves"] as const,
  policies: () => [...rbacKeys.all, "policies"] as const,
};

// ─── Reads ─────────────────────────────────────────────────────────────────

export function useUserHierarchy() {
  return useQuery<RbacUsersResponse, ApiError>({
    queryKey: rbacKeys.users(),
    queryFn: getUserHierarchy,
  });
}

export function useDelegations() {
  return useQuery<RbacDelegationsResponse, ApiError>({
    queryKey: rbacKeys.delegations(),
    queryFn: getDelegations,
  });
}

export function useLeaves() {
  return useQuery<RbacLeavesResponse, ApiError>({
    queryKey: rbacKeys.leaves(),
    queryFn: getLeaves,
  });
}

export function usePolicies() {
  return useQuery<RbacPoliciesResponse, ApiError>({
    queryKey: rbacKeys.policies(),
    queryFn: getPolicies,
  });
}

// ─── Writes ────────────────────────────────────────────────────────────────

interface SetHierarchyResult {
  id: number;
  supervisor_id: number | null;
  approval_level: number | null;
}

export function useSetHierarchy() {
  const queryClient = useQueryClient();
  return useMutation<SetHierarchyResult, ApiError, { userId: number; values: HierarchyFormValues }>(
    {
      mutationFn: ({ userId, values }) => setUserHierarchy(userId, values),
      onSuccess: () => queryClient.invalidateQueries({ queryKey: rbacKeys.users() }),
    },
  );
}

export function useCreateDelegation() {
  const queryClient = useQueryClient();
  return useMutation<RbacDelegation, ApiError, DelegationFormValues>({
    mutationFn: createDelegation,
    onSuccess: () => queryClient.invalidateQueries({ queryKey: rbacKeys.delegations() }),
  });
}

export function useRevokeDelegation() {
  const queryClient = useQueryClient();
  return useMutation<RbacDelegation, ApiError, number>({
    mutationFn: revokeDelegation,
    onSuccess: () => queryClient.invalidateQueries({ queryKey: rbacKeys.delegations() }),
  });
}

interface CreateLeaveResult {
  id: number;
  user_id: number;
  start_at: string;
  end_at: string;
}

export function useCreateLeave() {
  const queryClient = useQueryClient();
  return useMutation<CreateLeaveResult, ApiError, LeaveFormValues>({
    mutationFn: createLeave,
    onSuccess: () => queryClient.invalidateQueries({ queryKey: rbacKeys.leaves() }),
  });
}

export function useCreatePolicy() {
  const queryClient = useQueryClient();
  return useMutation<RbacPolicy, ApiError, PolicyFormValues>({
    mutationFn: createPolicy,
    onSuccess: () => queryClient.invalidateQueries({ queryKey: rbacKeys.policies() }),
  });
}

export function useUpdatePolicy() {
  const queryClient = useQueryClient();
  return useMutation<RbacPolicy, ApiError, { id: number; values: PolicyFormValues }>({
    mutationFn: ({ id, values }) => updatePolicy(id, values),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: rbacKeys.policies() }),
  });
}
