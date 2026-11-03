/**
 * Penetapan vault (cit-acm-plan FR2-FR5, cit-vault-select v3) -- backend/internal/handler/vault_plan_handler.go
 * (/api/v1/vault-plans, ACM-USER/ACM-SPV/ADMIN) and vendor_request_vault_handler.go
 * (/vendor-requests/{id}/vault-*, ATM-SPV review). Money maps are
 * {denom: amount} with whole-IDR decimal strings, never floats on the wire.
 */

import type { ApiError } from "@/lib/api/client";
import { api } from "@/lib/api/client";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

export type VaultPlanStatus = "draft" | "pending_acm_approval" | "acm_approved" | "cancelled";
/** denom -> amount, both decimal strings (whole IDR). */
export type DenomAmounts = Record<string, string>;

export interface VaultPlanSummary {
  id: number;
  vendor_request_id: number;
  request_number: string;
  replenish_date: string | null;
  request_status: string;
  acm_area_id: number;
  acm_area_name: string;
  status: VaultPlanStatus;
  allocation_count: number;
  warning_count: number;
  submitted_at: string | null;
}

export interface VaultBranchRef {
  id: number;
  code: string;
  name: string;
  vendor_id: number;
  vendor_name: string;
  region_code: string | null;
}

export interface VaultPlanAtm {
  terminal_id: string;
  replenish_branch: VaultBranchRef;
  order: DenomAmounts;
}

/** One saved vault row of the plan (cit-vault-select FR4). */
export interface VaultAllocationView {
  id: number;
  vendor_vault_id: number;
  vault_code: string;
  vault_category: string;
  branch: VaultBranchRef;
  tier: number;
  is_urgent: boolean;
  urgent_reason: string | null;
  amounts: DenomAmounts;
  saldo_snapshot: DenomAmounts | null;
  capacity_snapshot: DenomAmounts | null;
  capacity_warning: boolean;
  /** Tiket pemenuhan vault, issued at submit. */
  ticket_number: string | null;
}

export interface VaultTicket {
  ticket_number: string;
  vendor_vault_id: number;
  vault_code: string;
  is_active: boolean;
  created_at: string | null;
  deactivated_at: string | null;
}

export interface VaultPlanDetail {
  id: number;
  vendor_request_id: number;
  request_number: string;
  replenish_date: string | null;
  region_code: string | null;
  request_status: string;
  vault_rejection_reason: string | null;
  /** cit-send-vendor FR4.3: back to draft because a vault vendor rejected. */
  rejected_by_vendor: boolean;
  acm_area_id: number;
  acm_area_name: string;
  status: VaultPlanStatus;
  submitted_by: number | null;
  approved_by: number | null;
  rejection_reason: string | null;
  buffer_reason: string | null;
  atms: VaultPlanAtm[];
  /** Order per denom summed over every ATM of the request. */
  order_totals: DenomAmounts;
  allocations: VaultAllocationView[];
  vault_tickets: VaultTicket[];
}

export interface VaultOption {
  vendor_vault_id: number;
  vault_code: string;
  category: string;
  tier: number;
  saldo_known: boolean;
  saldo: DenomAmounts | null;
  /** DSR saldo minus other live plans on the same date. */
  capacity: DenomAmounts | null;
}

/** Candidate vendor branch with its eligible CASH/ATM_CASH vaults (FR3.4). */
export interface VaultBranchOption {
  vendor_branch_id: number;
  branch_code: string;
  branch_name: string;
  vendor_id: number;
  vendor_name: string;
  region_code: string;
  tier: number;
  vaults: VaultOption[];
}

export interface AllocationInput {
  vendor_vault_id: number;
  amounts: DenomAmounts;
  is_urgent: boolean;
  urgent_reason: string | null;
}

const BASE = "/vault-plans";
const KEY = ["vault-plans"] as const;

/** Empty values = no filter; dates are YYYY-MM-DD on the replenish date. */
export interface VaultPlanFilter {
  status: string;
  from: string;
  to: string;
}

export function useVaultPlans(filter: VaultPlanFilter) {
  return useQuery<VaultPlanSummary[], ApiError>({
    queryKey: [...KEY, "list", filter],
    queryFn: async () => {
      const qs = new URLSearchParams(Object.entries(filter).filter(([, v]) => v !== ""));
      return (await api.get<{ plans: VaultPlanSummary[] }>(`${BASE}?${qs}`)).data.plans;
    },
  });
}

export function useVaultPlan(id: number | null) {
  return useQuery<VaultPlanDetail, ApiError>({
    queryKey: [...KEY, "detail", id],
    queryFn: async () => (await api.get<VaultPlanDetail>(`${BASE}/${id}`)).data,
    enabled: id !== null,
  });
}

/**
 * Always asks with urgent=true: the list then holds every tier and the editor
 * hides tier 3 on rows that are not marked urgent (one request per plan).
 */
export function useVaultCandidates(planId: number, enabled: boolean) {
  return useQuery<VaultBranchOption[], ApiError>({
    queryKey: [...KEY, "candidates", planId],
    queryFn: async () =>
      (await api.get<{ branches: VaultBranchOption[] }>(`${BASE}/${planId}/candidates?urgent=true`))
        .data.branches,
    enabled,
  });
}

export type VaultPlanMutation =
  | { op: "save"; id: number; allocations: AllocationInput[]; bufferReason: string | null }
  | { op: "submit" | "approve"; id: number }
  | { op: "reject"; id: number; reason: string };

function send(m: VaultPlanMutation): Promise<unknown> {
  switch (m.op) {
    case "save":
      return api.put(`${BASE}/${m.id}/allocations`, {
        allocations: m.allocations,
        buffer_reason: m.bufferReason,
      });
    case "reject":
      return api.post(`${BASE}/${m.id}/reject`, { rejection_reason: m.reason });
    default:
      return api.post(`${BASE}/${m.id}/${m.op}`);
  }
}

export function useVaultPlanMutation() {
  const queryClient = useQueryClient();
  return useMutation<unknown, ApiError, VaultPlanMutation>({
    mutationFn: send,
    onSuccess: () => queryClient.invalidateQueries({ queryKey: KEY }),
  });
}

// -- Vendor Request side (ATM-SPV review, FR5) --------------------------------

/** ATM-SPV review panel (FR5.1): the request's live plan, null outside the vault flow. */
export function useRequestVaultReview(requestId: number, enabled: boolean) {
  return useQuery<VaultPlanDetail | null, ApiError>({
    queryKey: [...KEY, "request", requestId],
    queryFn: async () =>
      (
        await api.get<{ plan: VaultPlanDetail | null }>(
          `/vendor-requests/${requestId}/vault-assignments`,
        )
      ).data.plan,
    enabled,
  });
}

export function useRequestVaultDecision() {
  const queryClient = useQueryClient();
  return useMutation<unknown, ApiError, { id: number; approve: boolean; reason?: string }>({
    mutationFn: ({ id, approve, reason }) =>
      approve
        ? api.post(`/vendor-requests/${id}/vault-approve`)
        : api.post(`/vendor-requests/${id}/vault-reject`, { vault_rejection_reason: reason }),
    onSuccess: () =>
      Promise.all([
        queryClient.invalidateQueries({ queryKey: KEY }),
        queryClient.invalidateQueries({ queryKey: ["vendor-requests"] }),
      ]),
  });
}

/** Formats a whole-IDR decimal string; Number is exact below 2^53 (~9 x 10^15). */
export function formatAmount(value: string | undefined): string {
  return value === undefined ? "-" : new Intl.NumberFormat("id-ID").format(Number(value));
}

/** "100K 1.500.000 · 50K 300.000" -- denoms high to low, zero entries left out. */
export function formatDenoms(amounts: DenomAmounts | null): string {
  if (!amounts) return "Tidak diketahui";
  const parts = Object.entries(amounts)
    .filter(([, v]) => v !== "0")
    .sort(([a], [b]) => Number(b) - Number(a))
    .map(([d, v]) => `${Number(d) / 1000}K ${formatAmount(v)}`);
  return parts.length ? parts.join(" · ") : "0";
}

export const PLAN_STATUS_LABELS: Record<VaultPlanStatus, string> = {
  draft: "Draft",
  pending_acm_approval: "Menunggu Approval ACM",
  acm_approved: "Disetujui ACM",
  cancelled: "Dibatalkan",
};

export const TIER_LABELS: Record<number, string> = {
  1: "Tier 1 · vendor & region sama",
  2: "Tier 2 · vendor lain, region sama",
  3: "Tier 3 · region lain (urgent)",
};
