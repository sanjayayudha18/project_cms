/**
 * cit-send-vendor (FR6.1/FR6.6): the vendor side of a Vendor Request on the
 * internal app — every vendor party's status + decision history, and the
 * ATM-SPV "Kembalikan ke pembuat" after a replenish branch rejects.
 */

import { type ApiError, api } from "@/lib/api/client";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { vendorRequestKeys } from "./hooks";
import type { VendorRequestDetail } from "./types";

export type VendorPartyRole = "replenish" | "vault";
export type VendorPartyStatus = "pending" | "accepted" | "rejected" | "withdrawn";

export interface VendorParty {
  id: number;
  role: VendorPartyRole;
  status: VendorPartyStatus;
  branch_id: number;
  branch_code: string;
  branch_name: string;
  vendor_id: number;
  vendor_name: string;
  sent_at: string | null;
  decided_at: string | null;
  decided_by_name: string | null;
  rejection_reason: string | null;
}

export interface VendorPartyEvent {
  id: number;
  party_id: number;
  event: "sent" | "resent" | "accepted" | "rejected" | "withdrawn" | "cancelled";
  reason: string | null;
  actor_name: string;
  created_at: string | null;
}

export interface VendorPartiesResponse {
  parties: VendorParty[];
  events: VendorPartyEvent[];
}

const BASE = "/api/v1/vendor-requests";

export async function fetchVendorParties(id: number): Promise<VendorPartiesResponse> {
  const { data } = await api.get<VendorPartiesResponse>(`${BASE}/${id}/vendor-parties`);
  return data;
}

export async function vendorReturnVendorRequest(
  id: number,
  reason: string,
): Promise<VendorRequestDetail> {
  const { data } = await api.post<VendorRequestDetail>(`${BASE}/${id}/vendor-return`, {
    rejection_reason: reason,
  });
  return data;
}

export function useVendorParties(id: number) {
  return useQuery<VendorPartiesResponse, ApiError>({
    queryKey: [...vendorRequestKeys.all, "vendor-parties", id],
    queryFn: () => fetchVendorParties(id),
  });
}

export function useVendorReturn() {
  const queryClient = useQueryClient();
  return useMutation<VendorRequestDetail, ApiError, { id: number; reason: string }>({
    mutationFn: ({ id, reason }) => vendorReturnVendorRequest(id, reason),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: vendorRequestKeys.all }),
  });
}
