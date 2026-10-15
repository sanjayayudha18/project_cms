/**
 * Hari libur (replenishment-realisasi FR1) -- backend/internal/handler/admin_holiday_handler.go
 * (/api/v1/admin/holidays, ADMIN only). Changes apply immediately (audited server-side); the realisasi
 * job counts workdays as Monday-Friday minus active holidays.
 */

import type { ApiError } from "@/lib/api/client";
import { api } from "@/lib/api/client";
import { apiConfig } from "@/lib/api/config";
import { useAuthStore } from "@/lib/auth/store";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

export interface Holiday {
  id: number;
  holiday_date: string; // RFC 3339 date at 00:00 UTC
  name: string;
  is_active: boolean;
}

export interface HolidayList {
  items: Holiday[];
  total: number;
}

export interface HolidayImportResult {
  created: number;
  skipped: string[];
}

export type HolidayStatus = "all" | "active" | "inactive";

const BASE = "/admin/holidays";
const KEY = ["admin-holidays"] as const;

export function useHolidays(year: number, status: HolidayStatus) {
  return useQuery<HolidayList, ApiError>({
    queryKey: [...KEY, year, status],
    queryFn: async () =>
      (await api.get<HolidayList>(`${BASE}?year=${year}&status=${status}&page_size=100`)).data,
  });
}

function useHolidayMutation<T>(fn: (input: T) => Promise<unknown>) {
  const client = useQueryClient();
  return useMutation<unknown, ApiError, T>({
    mutationFn: fn,
    onSuccess: () => client.invalidateQueries({ queryKey: KEY }),
  });
}

export function useCreateHoliday() {
  return useHolidayMutation((input: { holiday_date: string; name: string }) =>
    api.post(BASE, input),
  );
}

export function useRenameHoliday() {
  return useHolidayMutation((input: { id: number; name: string }) =>
    api.put(`${BASE}/${input.id}`, { name: input.name }),
  );
}

export function useSetHolidayActive() {
  return useHolidayMutation((input: { id: number; active: boolean }) =>
    api.post(`${BASE}/${input.id}/${input.active ? "enable" : "disable"}`),
  );
}

/** Multipart upload (field "file"); api.post sends JSON, so this one uses fetch directly. */
export async function importHolidays(file: File): Promise<HolidayImportResult> {
  const form = new FormData();
  form.append("file", file);
  const token = useAuthStore.getState().accessToken;
  const res = await fetch(`${apiConfig.baseURL}${BASE}/import`, {
    method: "POST",
    body: form,
    headers: token ? { Authorization: `Bearer ${token}` } : {},
    credentials: "include",
  });
  const body = (await res.json().catch(() => ({}))) as HolidayImportResult & { message?: string };
  if (!res.ok) throw new Error(body.message ?? "Impor gagal");
  return body;
}

export function useImportHolidays() {
  const client = useQueryClient();
  return useMutation<HolidayImportResult, Error, File>({
    mutationFn: importHolidays,
    onSuccess: () => client.invalidateQueries({ queryKey: KEY }),
  });
}
