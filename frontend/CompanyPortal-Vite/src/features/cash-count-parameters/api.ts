/**
 * Parameter Cash Count (ScDD 5.1) -- /api/v1/cash-count/*. Every write is
 * maker-checker: the API answers 202 and the row changes only after approval.
 * Reads: category-params/areas = CC-CWM|ADMIN, anomaly-categories/checklist-items = CC-CCR|ADMIN.
 */

import type { ApiError } from "@/lib/api/client";
import { api } from "@/lib/api/client";
import type { components } from "@/lib/api/schema";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import type { ChangeRequestAccepted } from "../master-data/changeRequest";

type S = components["schemas"];
export type CategoryParamsSet = S["CashCountCategoryParamsSet"];
export type CategoryParamsList = S["CashCountCategoryParamsList"];
export type CategoryParamsRequest = S["CashCountCategoryParamsRequest"];
export type AnomalyCategory = S["CashCountAnomalyCategory"];
export type AnomalyCreate = S["CashCountAnomalyCategoryCreate"];
export type AnomalyUpdate = S["CashCountAnomalyCategoryUpdate"];
export type ChecklistItem = S["CashCountChecklistItem"];
export type ChecklistCreate = S["CashCountChecklistItemCreate"];
export type ChecklistUpdate = S["CashCountChecklistItemUpdate"];
export type CashCountArea = S["CashCountArea"];
export type AreaCreate = S["CashCountAreaCreate"];
export type AreaUpdate = S["CashCountAreaUpdate"];

export type ToggleOp = "disable" | "enable";

const BASE = "/cash-count";
const KEY = ["cash-count"] as const;

function useList<T>(path: string, pick: (body: never) => T, query = "") {
  return useQuery<T, ApiError>({
    queryKey: [...KEY, path],
    queryFn: async () => pick((await api.get(`${BASE}/${path}${query}`)).data as never),
  });
}

export const useCategoryParams = () =>
  useList<CategoryParamsList>("category-params", (b) => b as CategoryParamsList);
export const useAnomalyCategories = () =>
  useList<AnomalyCategory[]>(
    "anomaly-categories",
    (b) => (b as S["CashCountAnomalyCategoryList"]).categories,
    "?status=all",
  );
export const useChecklistItems = () =>
  useList<ChecklistItem[]>(
    "checklist-items",
    (b) => (b as S["CashCountChecklistItemList"]).items,
    "?status=all",
  );
export const useCashCountAreas = () =>
  useList<CashCountArea[]>("areas", (b) => (b as S["CashCountAreaList"]).areas, "?status=all");

/** One mutation shape per resource: create / update / disable / enable, all answer 202. */
export type Mutation<C, U> =
  | { op: "create"; body: C }
  | { op: "update"; id: number; body: U }
  | { op: ToggleOp; id: number };

export function useCashCountMutation<C, U>(path: string) {
  const queryClient = useQueryClient();
  return useMutation<ChangeRequestAccepted, ApiError, Mutation<C, U>>({
    mutationFn: async (m) => {
      const url = `${BASE}/${path}`;
      if (m.op === "create") return (await api.post<ChangeRequestAccepted>(url, m.body)).data;
      if (m.op === "update")
        return (await api.put<ChangeRequestAccepted>(`${url}/${m.id}`, m.body)).data;
      return (await api.post<ChangeRequestAccepted>(`${url}/${m.id}/${m.op}`)).data;
    },
    onSuccess: () => queryClient.invalidateQueries({ queryKey: [...KEY, path] }),
  });
}

/**
 * Exact decimal text -> "5.000.000.000,00" (id-ID grouping) without ever
 * going through a float (CLAUDE.md Sec 6).
 */
export function formatIdrAmount(value: string | null): string {
  if (value === null) return "-";
  const [int = "0", frac = ""] = value.split(".");
  const neg = int.startsWith("-");
  const grouped = int.replace("-", "").replace(/\B(?=(\d{3})+(?!\d))/g, ".");
  return `${neg ? "-" : ""}${grouped},${frac.padEnd(2, "0").slice(0, 2)}`;
}
