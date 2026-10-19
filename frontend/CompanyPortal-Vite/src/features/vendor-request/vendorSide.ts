/**
 * cit-send-vendor (FR6.1/FR6.6): the vendor side of a Vendor Request on the
 * internal app — every vendor party's status + decision history, and the
 * ATM-SPV "Kembalikan ke pembuat" after a replenish branch rejects.
 */

import { type ApiError, api } from "@/lib/api/client";
import type { components } from "@/lib/api/schema";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { vendorRequestKeys } from "./hooks";
import type { VendorRequestDetail } from "./types";

type S = components["schemas"];

export type VendorParty = S["RequestVendorParty"];
export type VendorPartyRole = VendorParty["role"];
export type VendorPartyStatus = VendorParty["status"];
export type VendorPartyEvent = S["RequestVendorPartyEvent"];
export type VendorPartiesResponse = S["RequestVendorParties"];

// Relative to apiConfig.baseURL ("/api/v1"), same as api.ts — the client adds the prefix.
const BASE = "/vendor-requests";

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
