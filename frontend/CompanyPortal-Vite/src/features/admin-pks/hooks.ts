import type { ApiError } from "@/lib/api/client";
import { keepPreviousData, useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import type { ChangeRequestAccepted } from "../master-data/changeRequest";
import {
  listPks,
  submitVendorPks,
  terminateVendorPks,
  unterminateVendorPks,
  updatePksReminder,
} from "./api";
import type {
  PksListFilter,
  PksListPage,
  PksReasonPayload,
  SubmitPksPayload,
  UpdatePksReminderResponse,
} from "./types";

export const pksKeys = {
  all: ["admin-pks"] as const,
  list: (filter: PksListFilter) => [...pksKeys.all, "list", filter] as const,
};

/** Refresh Daftar PKS, the vendor list (pks_count) and the pending badges. */
function useInvalidatePks() {
  const queryClient = useQueryClient();
  return () => {
    queryClient.invalidateQueries({ queryKey: pksKeys.all });
    queryClient.invalidateQueries({ queryKey: ["admin-vendors"] });
    queryClient.invalidateQueries({ queryKey: ["master-data", "pending"] });
  };
}

export function usePksList(filter: PksListFilter) {
  return useQuery<PksListPage, ApiError>({
    queryKey: pksKeys.list(filter),
    queryFn: () => listPks(filter),
    placeholderData: keepPreviousData,
  });
}

export function useUpdatePksReminder() {
  const invalidate = useInvalidatePks();
  return useMutation<UpdatePksReminderResponse, ApiError, { pksId: number; renewalDays: number }>({
    mutationFn: ({ pksId, renewalDays }) => updatePksReminder(pksId, renewalDays),
    onSuccess: invalidate,
  });
}

export function useSubmitVendorPks(vendorId: number) {
  const invalidate = useInvalidatePks();
  return useMutation<ChangeRequestAccepted, ApiError, SubmitPksPayload>({
    mutationFn: (payload) => submitVendorPks(vendorId, payload),
    onSuccess: invalidate,
  });
}

type ReasonVars = { pksId: number; payload: PksReasonPayload };

export function useTerminateVendorPks(vendorId: number) {
  const invalidate = useInvalidatePks();
  return useMutation<ChangeRequestAccepted, ApiError, ReasonVars>({
    mutationFn: ({ pksId, payload }) => terminateVendorPks(vendorId, pksId, payload),
    onSuccess: invalidate,
  });
}

export function useUnterminateVendorPks(vendorId: number) {
  const invalidate = useInvalidatePks();
  return useMutation<ChangeRequestAccepted, ApiError, ReasonVars>({
    mutationFn: ({ pksId, payload }) => unterminateVendorPks(vendorId, pksId, payload),
    onSuccess: invalidate,
  });
}
