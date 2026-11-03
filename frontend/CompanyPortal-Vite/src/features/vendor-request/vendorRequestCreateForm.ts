/** Schema, payload mapping and small helpers for the Vendor Request create page. */

import { formatThousands } from "@/lib/utils/formatCurrency";
import { z } from "zod";
import type { CreateVendorRequestPayload, VendorRequestCategory } from "./types";

const AMOUNT_MIN = 1;
const AMOUNT_MAX = 999_999_999;

export const CATEGORY_OPTIONS: { value: VendorRequestCategory; label: string }[] = [
  { value: "planned", label: "Planned — replenish H+1" },
  { value: "emergency", label: "Emergency — replenish H+0" },
  { value: "additional", label: "Additional — pilih tanggal sendiri" },
];

/** Plain hyphen, never an em-dash, for an empty/null context column (Req 2.2). */
export function displayOrDash(value: string | undefined): string {
  return value ? value : "-";
}

const itemSchema = z.object({
  terminal_id: z.string().min(1, "Wajib diisi").max(64, "Maksimal 64 karakter"),
  // DMAA-only fields: present (read-only) for a Forecast-Browser-sourced
  // row, absent for a manually added one. Never rendered as inputs.
  periode_pred: z.string().optional(),
  flm_vendor: z.string().optional(),
  flm_vendor_region: z.string().optional(),
  denom: z
    .number({ invalid_type_error: "Wajib berupa angka" })
    .int("Harus bilangan bulat")
    .positive("Harus lebih besar dari 0"),
  amount_refund: z.number().optional(),
  amount_replenish: z
    .number({ invalid_type_error: "Wajib berupa angka" })
    .int("Harus bilangan bulat")
    .min(AMOUNT_MIN, `Minimal ${AMOUNT_MIN}`)
    .max(AMOUNT_MAX, `Maksimal ${formatThousands(AMOUNT_MAX)}`),
  // Manual_Request-only (CIT-2 Req 3, Q4): editable inputs in manual mode,
  // read-only display in DMAA mode, sent only when is_manual.
  brand: z.string().optional(),
  lokasi_atm: z.string().optional(),
});

export const formSchema = z.object({
  notes: z.string().max(500, "Maksimal 500 karakter").optional(),
  items: z.array(itemSchema).min(1, "Minimal 1 item diperlukan"),
});

export type FormValues = z.infer<typeof formSchema>;

export interface ToPayloadOptions {
  isManual: boolean;
  forecastDate: string;
  replenishDate: string;
  category: VendorRequestCategory;
  vendorId: number;
}

export function toPayload(values: FormValues, opts: ToPayloadOptions): CreateVendorRequestPayload {
  return {
    forecast_date: opts.isManual ? undefined : opts.forecastDate,
    replenish_date: opts.replenishDate,
    // Req 1.9: required for EVERY create now, not just manual — callers only
    // reach here once canSubmit has confirmed opts.category is non-empty.
    request_category: opts.category,
    is_manual: opts.isManual,
    vendor_id: opts.vendorId,
    notes: values.notes,
    items: values.items.map((it) => ({
      terminal_id: it.terminal_id,
      periode_pred: opts.isManual ? undefined : it.periode_pred,
      denom: it.denom,
      amount_replenish: it.amount_replenish,
      brand: opts.isManual && it.brand ? it.brand : undefined,
      lokasi_atm: opts.isManual && it.lokasi_atm ? it.lokasi_atm : undefined,
    })),
  };
}

export function emptyManualItem() {
  return {
    terminal_id: "",
    denom: 0,
    amount_replenish: 0,
    brand: "",
    lokasi_atm: "",
  };
}

export function errorMessage(error: unknown): string {
  if (error instanceof Error) return error.message;
  if (
    typeof error === "object" &&
    error !== null &&
    "message" in error &&
    typeof (error as { message: unknown }).message === "string"
  ) {
    return (error as { message: string }).message;
  }
  return "Terjadi kesalahan yang tidak diketahui";
}
