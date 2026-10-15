import {
  type RealizationFilter,
  type RealizationInput,
  type VendorOrderFilter,
  acceptOrder,
  fetchOrder,
  fetchOrders,
  fetchRealizations,
  rejectOrder,
  submitRealization,
} from "@/features/orders/api";
import { keepPreviousData, useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

const KEY = ["vendor-orders"] as const;

export function useOrders(filter: VendorOrderFilter) {
  return useQuery({
    queryKey: [...KEY, "list", filter],
    queryFn: () => fetchOrders(filter),
    placeholderData: keepPreviousData,
  });
}

export function useOrder(id: number) {
  return useQuery({
    queryKey: [...KEY, "detail", id],
    queryFn: () => fetchOrder(id),
    enabled: Number.isInteger(id) && id > 0,
  });
}

/** Accept or reject (reason set) one order; refreshes list + detail on success. */
export function useDecideOrder(id: number) {
  const client = useQueryClient();
  return useMutation({
    mutationFn: (reason: string | null) =>
      reason === null ? acceptOrder(id) : rejectOrder(id, reason),
    onSuccess: (detail) => {
      client.setQueryData([...KEY, "detail", id], detail);
      void client.invalidateQueries({ queryKey: [...KEY, "list"] });
    },
  });
}

/** Sends (or resends after a rejection) the realisasi report of a replenish order (FR2). */
export function useSubmitRealization(id: number) {
  const client = useQueryClient();
  return useMutation({
    mutationFn: (rows: readonly RealizationInput[]) => submitRealization(id, rows),
    onSuccess: (detail) => {
      client.setQueryData([...KEY, "detail", id], detail);
      void client.invalidateQueries({ queryKey: [...KEY, "list"] });
    },
  });
}

export function useRealizations(filter: RealizationFilter, enabled: boolean) {
  return useQuery({
    queryKey: ["vendor-realizations", filter],
    queryFn: () => fetchRealizations(filter),
    placeholderData: keepPreviousData,
    enabled,
  });
}
