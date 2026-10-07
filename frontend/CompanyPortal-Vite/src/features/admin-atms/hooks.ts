/**
 * TanStack Query hooks for the admin ATMs screen. Same conventions as
 * admin-vendors/hooks.ts: keepPreviousData on the list, mutations invalidate
 * only this feature's query keys on success. useLocationOptions uses a long
 * staleTime (master data, same convention as vendor-request's
 * useVendorOptions) since the Location select rarely changes mid-session.
 */

import type { ApiError } from "@/lib/api/client";
import { keepPreviousData, useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import type { ChangeRequestAccepted } from "../master-data/changeRequest";
import {
  createATM,
  disableATM,
  enableATM,
  getATM,
  listATMs,
  listLocationOptions,
  updateATM,
} from "./api";
import type {
  AdminATM,
  AdminATMsListParams,
  AdminATMsListResponse,
  CreateATMPayload,
  LocationOptionsResponse,
  UpdateATMPayload,
} from "./types";

const LOCATION_OPTIONS_STALE_TIME = 5 * 60_000;

export const adminATMsKeys = {
  all: ["admin-atms"] as const,
  list: (params: AdminATMsListParams) => [...adminATMsKeys.all, "list", params] as const,
  detail: (id: number) => [...adminATMsKeys.all, "detail", id] as const,
  locations: () => [...adminATMsKeys.all, "locations"] as const,
};

export function useATMsList(params: AdminATMsListParams) {
  return useQuery<AdminATMsListResponse, ApiError>({
    queryKey: adminATMsKeys.list(params),
    queryFn: () => listATMs(params),
    placeholderData: keepPreviousData,
  });
}

export function useATM(id: number | null) {
  return useQuery<AdminATM, ApiError>({
    queryKey: adminATMsKeys.detail(id ?? 0),
    queryFn: () => getATM(id as number),
    enabled: id !== null,
  });
}

export function useLocationOptions() {
  return useQuery<LocationOptionsResponse, ApiError>({
    queryKey: adminATMsKeys.locations(),
    queryFn: listLocationOptions,
    staleTime: LOCATION_OPTIONS_STALE_TIME,
  });
}

function useInvalidateList() {
  const queryClient = useQueryClient();
  return () => {
    queryClient.invalidateQueries({ queryKey: adminATMsKeys.all });
    // a submitted change is pending: refresh the row badges too
    queryClient.invalidateQueries({ queryKey: ["master-data", "pending"] });
  };
}

export function useCreateATM() {
  const invalidate = useInvalidateList();
  return useMutation<ChangeRequestAccepted, ApiError, CreateATMPayload>({
    mutationFn: createATM,
    onSuccess: invalidate,
  });
}

export function useUpdateATM() {
  const invalidate = useInvalidateList();
  return useMutation<ChangeRequestAccepted, ApiError, { id: number; payload: UpdateATMPayload }>({
    mutationFn: ({ id, payload }) => updateATM(id, payload),
    onSuccess: invalidate,
  });
}

export function useDisableATM() {
  const invalidate = useInvalidateList();
  return useMutation<ChangeRequestAccepted, ApiError, number>({
    mutationFn: disableATM,
    onSuccess: invalidate,
  });
}

export function useEnableATM() {
  const invalidate = useInvalidateList();
  return useMutation<ChangeRequestAccepted, ApiError, number>({
    mutationFn: enableATM,
    onSuccess: invalidate,
  });
}
