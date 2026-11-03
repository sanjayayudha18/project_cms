/**
 * Mapping lokasi DSR -> vault (cit-acm-plan FR2). Writes are maker-checker:
 * every mutation stages a change request (202) and the row changes only after
 * approval. Lives in its own module to keep api.ts/hooks.ts from growing.
 */

import type { ApiError } from "@/lib/api/client";
import { api } from "@/lib/api/client";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import type { ChangeRequestAccepted } from "../master-data/changeRequest";

export interface DsrLocationMap {
  id: number;
  vendor_id: number;
  dsr_location: string;
  vendor_vault_id: number;
  vault_code: string;
  vendor_branch_id: number;
  branch_name: string;
  is_active: boolean;
  deleted_at: string | null;
}

export interface UnmappedDsrLocation {
  dsr_location: string;
  last_report_date: string | null;
}

const base = (vendorId: number) => `/admin/vendors/${vendorId}/dsr-location-maps`;
const keys = (vendorId: number) =>
  ["admin-vendors", "children", vendorId, "dsr-location-maps"] as const;

export function useDsrLocationMaps(vendorId: number) {
  return useQuery<DsrLocationMap[], ApiError>({
    queryKey: [...keys(vendorId), "list"],
    queryFn: async () =>
      (await api.get<{ maps: DsrLocationMap[] }>(`${base(vendorId)}?status=all`)).data.maps,
  });
}

export function useUnmappedDsrLocations(vendorId: number) {
  return useQuery<UnmappedDsrLocation[], ApiError>({
    queryKey: [...keys(vendorId), "unmapped"],
    queryFn: async () =>
      (await api.get<{ locations: UnmappedDsrLocation[] }>(`${base(vendorId)}/unmapped`)).data
        .locations,
  });
}

export type MapMutation =
  | { op: "create"; dsr_location: string; vendor_vault_id: number }
  | { op: "update"; id: number; vendor_vault_id: number }
  | { op: "disable" | "enable"; id: number };

function submit(vendorId: number, m: MapMutation): Promise<{ data: ChangeRequestAccepted }> {
  switch (m.op) {
    case "create":
      return api.post(base(vendorId), {
        dsr_location: m.dsr_location,
        vendor_vault_id: m.vendor_vault_id,
      });
    case "update":
      return api.put(`${base(vendorId)}/${m.id}`, { vendor_vault_id: m.vendor_vault_id });
    default:
      return api.post(`${base(vendorId)}/${m.id}/${m.op}`);
  }
}

export function useDsrLocationMapMutation(vendorId: number) {
  const queryClient = useQueryClient();
  return useMutation<ChangeRequestAccepted, ApiError, MapMutation>({
    mutationFn: async (m) => (await submit(vendorId, m)).data,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: keys(vendorId) });
      queryClient.invalidateQueries({ queryKey: ["master-data", "pending"] });
    },
  });
}
