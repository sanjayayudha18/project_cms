/**
 * Mapping PIC Cash Count (ScDD 5.1, AC-5.1-10..12). CRUD = /api/v1/cash-count/pics (maker-checker, 202);
 * CSV upload = /api/v1/cash-count/import/cash-count-pics/dry-run|confirm (CC-CWM only, replace-all);
 * CSV download = backend-exq `POST /exports/cash-count-pics` (see features/exports).
 */

import type { ApiError } from "@/lib/api/client";
import { api } from "@/lib/api/client";
import type { components } from "@/lib/api/schema";
import { useQuery } from "@tanstack/react-query";
import {
  ImportApiError,
  type ImportConfirmResult,
  type ImportPreview,
  type ImportRowError,
} from "../master-data-io/api";

type S = components["schemas"];
export type CashCountPic = S["CashCountPic"];
export type PicCreate = S["CashCountPicCreate"];
export type PicUpdate = S["CashCountPicUpdate"];
export type PicRole = CashCountPic["pic_role"];

const IMPORT_URL = "/cash-count/import/cash-count-pics";

export const PIC_ROLE_LABEL: Record<PicRole, string> = {
  COORDINATOR: "Coordinator",
  EXECUTOR: "Executor",
};

export const usePics = () =>
  useQuery<CashCountPic[], ApiError>({
    queryKey: ["cash-count", "pics"],
    queryFn: async () =>
      ((await api.get("/cash-count/pics?status=all")).data as S["CashCountPicList"]).pics,
  });

async function upload<T>(path: string, file: File): Promise<T> {
  const form = new FormData();
  form.append("file", file);
  try {
    return (await api.post<T>(path, form)).data;
  } catch (err) {
    const e = err as Partial<ApiError> | null;
    if (typeof e?.status !== "number") throw err;
    const rows = Array.isArray(e.details) ? (e.details as ImportRowError[]) : [];
    throw new ImportApiError(e.message || "Permintaan gagal", e.status, rows);
  }
}

/** The API sends null for empty lists; the shared import components expect arrays. */
export async function dryRunPicImport(file: File): Promise<ImportPreview> {
  const p = await upload<S["CashCountImportPreview"]>(`${IMPORT_URL}/dry-run`, file);
  return {
    ...p,
    errors: p.errors ?? [],
    changes: (p.changes ?? []).map((c) => ({ ...c, fields: c.fields ?? [] })),
  };
}

export function confirmPicImport(file: File): Promise<ImportConfirmResult> {
  return upload<ImportConfirmResult>(`${IMPORT_URL}/confirm`, file);
}
