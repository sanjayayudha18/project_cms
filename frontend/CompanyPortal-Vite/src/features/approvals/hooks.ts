/**
 * TanStack Query hooks for the approval inbox. Approving a master-data change
 * changes the underlying records, so a successful approve/reject also
 * invalidates the admin vendor/ATM lists (they are prefix-matched by key).
 */

import type { ApiError } from "@/lib/api/client";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import {
  type InboxItem,
  type MasterDataDetail,
  approveRequest,
  getInbox,
  getMasterDataDetail,
  rejectRequest,
} from "./api";

export const approvalKeys = {
  all: ["approvals"] as const,
  inbox: () => [...approvalKeys.all, "inbox"] as const,
  masterData: (requestId: number) => [...approvalKeys.all, "master-data", requestId] as const,
};

export function useInbox() {
  return useQuery<InboxItem[], ApiError>({
    queryKey: approvalKeys.inbox(),
    queryFn: getInbox,
  });
}

export function useMasterDataDetail(requestId: number | null) {
  return useQuery<MasterDataDetail, ApiError>({
    queryKey: approvalKeys.masterData(requestId ?? 0),
    queryFn: () => getMasterDataDetail(requestId as number),
    enabled: requestId !== null,
  });
}

function useAfterDecision() {
  const queryClient = useQueryClient();
  return () =>
    Promise.all([
      queryClient.invalidateQueries({ queryKey: approvalKeys.all }),
      queryClient.invalidateQueries({ queryKey: ["admin-vendors"] }),
      queryClient.invalidateQueries({ queryKey: ["admin-atms"] }),
    ]);
}

export function useApprove() {
  const afterDecision = useAfterDecision();
  return useMutation<void, ApiError, number>({
    mutationFn: approveRequest,
    onSuccess: afterDecision,
    // The decision may be recorded even when applying fails (409); refresh either way.
    onError: afterDecision,
  });
}

export function useReject() {
  const afterDecision = useAfterDecision();
  return useMutation<void, ApiError, number>({
    mutationFn: rejectRequest,
    onSuccess: afterDecision,
  });
}
