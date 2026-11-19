import type { ChangeRequestAccepted } from "@/features/master-data/changeRequest";
import type { ApiError } from "@/lib/api/client";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { disablePksType, enablePksType, listPksTypes, submitPksType, updatePksType } from "./api";
import type { CreatePksTypePayload, PksType, UpdatePksTypePayload } from "./types";

export const adminPksTypesKeys = {
  all: ["admin-pks-types"] as const,
  list: () => [...adminPksTypesKeys.all, "list"] as const,
};

export function usePksTypesList() {
  return useQuery<PksType[], ApiError>({
    queryKey: adminPksTypesKeys.list(),
    queryFn: listPksTypes,
  });
}

function useInvalidatePksTypes() {
  const queryClient = useQueryClient();
  return () => {
    queryClient.invalidateQueries({ queryKey: adminPksTypesKeys.all });
    queryClient.invalidateQueries({ queryKey: ["master-data", "pending"] });
  };
}

export function useSubmitPksType() {
  const invalidate = useInvalidatePksTypes();
  return useMutation<ChangeRequestAccepted, ApiError, CreatePksTypePayload>({
    mutationFn: (payload) => submitPksType(payload),
    onSuccess: invalidate,
  });
}

export function useUpdatePksType() {
  const invalidate = useInvalidatePksTypes();
  return useMutation<
    ChangeRequestAccepted,
    ApiError,
    { id: number; payload: UpdatePksTypePayload }
  >({
    mutationFn: ({ id, payload }) => updatePksType(id, payload),
    onSuccess: invalidate,
  });
}

export function useDisablePksType() {
  const invalidate = useInvalidatePksTypes();
  return useMutation<ChangeRequestAccepted, ApiError, number>({
    mutationFn: (id) => disablePksType(id),
    onSuccess: invalidate,
  });
}

export function useEnablePksType() {
  const invalidate = useInvalidatePksTypes();
  return useMutation<ChangeRequestAccepted, ApiError, number>({
    mutationFn: (id) => enablePksType(id),
    onSuccess: invalidate,
  });
}
