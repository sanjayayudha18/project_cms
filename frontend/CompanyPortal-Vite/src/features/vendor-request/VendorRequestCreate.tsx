/**
 * Vendor Request creation page (Req 12; CIT-2 Req 2, 3, 4). Two entry paths:
 * - DMAA-backed (default): items carried from the Forecast Browser
 *   (selectionStore) — context columns (Brand/FLM Vendor/FLM Vendor Region)
 *   are display-only, vendor_id is already resolved from the browser's
 *   required FLM Vendor filter (Q2).
 * - Manual (CIT-2 Req 3): reachable only when nothing was selected on the
 *   Forecast Browser. The operator additionally picks a vendor, then enters
 *   items directly.
 *
 * replenishment-request-enhancements (Req 1): the Request_Type (category)
 * control is no longer manual-only — every create, DMAA-backed or manual
 * alike, requires picking one before submit (Req 1.1/1.2), and it drives
 * Replenish_Date identically on both paths (Req 1.3-1.5).
 */

import { Button } from "@/components/ui/Button";
import { PageHeader } from "@/components/ui/PageHeader";
import { useToast } from "@/lib/hooks/useToast";
import { formatIDR } from "@/lib/utils/formatCurrency";
import { zodResolver } from "@hookform/resolvers/zod";
import { Link, useNavigate } from "@tanstack/react-router";
import { Plus, Trash2 } from "lucide-react";
import { useEffect, useState } from "react";
import { useFieldArray, useForm } from "react-hook-form";
import { z } from "zod";
import { useCreateVendorRequest, useSubmitVendorRequest, useVendorOptions } from "./hooks";
import { jakartaCalendarDateISO } from "./lib/nextBusinessDay";
import { usePendingVendorRequestSelection } from "./selectionStore";
import type {
  CreateVendorRequestPayload,
  VendorRequestCategory,
  VendorRequestDetail,
} from "./types";

const AMOUNT_MIN = 1;
const AMOUNT_MAX = 999_999_999;
const ERROR_TOAST_MS = 5000;

const CATEGORY_OPTIONS: { value: VendorRequestCategory; label: string }[] = [
  { value: "planned", label: "Planned — replenish H+1" },
  { value: "emergency", label: "Emergency — replenish H+0" },
  { value: "additional", label: "Additional — pilih H+0/H+1/H+2" },
];

/** Plain hyphen, never an em-dash, for an empty/null context column (Req 2.2). */
function displayOrDash(value: string | undefined): string {
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
    .max(AMOUNT_MAX, `Maksimal ${formatIDR(AMOUNT_MAX)}`),
  // Manual_Request-only (CIT-2 Req 3, Q4): editable inputs in manual mode,
  // read-only display in DMAA mode, sent only when is_manual.
  brand: z.string().optional(),
  lokasi_atm: z.string().optional(),
});

const formSchema = z.object({
  notes: z.string().max(500, "Maksimal 500 karakter").optional(),
  items: z.array(itemSchema).min(1, "Minimal 1 item diperlukan"),
});

type FormValues = z.infer<typeof formSchema>;

interface ToPayloadOptions {
  isManual: boolean;
  forecastDate: string;
  replenishDate: string;
  category: VendorRequestCategory;
  vendorId: number;
}

function toPayload(values: FormValues, opts: ToPayloadOptions): CreateVendorRequestPayload {
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

function emptyManualItem() {
  return {
    terminal_id: "",
    denom: 0,
    amount_replenish: 0,
    brand: "",
    lokasi_atm: "",
  };
}

export function VendorRequestCreate() {
  const navigate = useNavigate();
  // Captured once at mount (lazy initializer), NOT a reactive store
  // subscription: the mount effect below clears the store right after
  // reading it, and a reactive `pending` would immediately flip this
  // component to the "nothing selected" empty state on the very next
  // render, wiping out the form it had just seeded.
  const [pending] = useState(() => usePendingVendorRequestSelection.getState().pending);
  const clearPending = usePendingVendorRequestSelection((s) => s.clearPending);
  const createMutation = useCreateVendorRequest();
  const submitMutation = useSubmitVendorRequest();
  const { toast, dismiss } = useToast();
  const vendorOptionsQuery = useVendorOptions();

  // CIT-2 Req 3.1: manual mode is only reachable when there was no
  // Forecast-Browser selection to begin with — an explicit operator choice,
  // not a fallback the page picks silently.
  const [manualRequested, setManualRequested] = useState(false);
  const isManual = !pending;

  // Req 1.2: starts unselected on BOTH paths so the operator must actively
  // pick one — replenishDate stays "" until they do, tripping
  // replenishDateError and blocking submit alongside categoryError.
  const [category, setCategory] = useState<VendorRequestCategory | "">("");
  const [replenishDate, setReplenishDate] = useState("");
  const [manualVendorId, setManualVendorId] = useState<number | null>(null);

  const form = useForm<FormValues>({
    resolver: zodResolver(formSchema),
    mode: "onChange",
    defaultValues: {
      notes: "",
      items: (pending?.items ?? []).map((it) => ({
        terminal_id: it.terminal_id,
        periode_pred: it.periode_pred,
        flm_vendor: it.flm_vendor,
        flm_vendor_region: it.flm_vendor_region,
        denom: it.denom,
        amount_refund: it.amount_refund,
        amount_replenish: it.amount_replenish,
        brand: it.brand,
        lokasi_atm: it.lokasi_atm,
      })),
    },
  });
  const { fields, append, remove } = useFieldArray({ control: form.control, name: "items" });
  const items = form.watch("items");
  const total = items.reduce(
    (sum, it) => sum + (Number.isFinite(it.amount_replenish) ? it.amount_replenish : 0),
    0,
  );

  // Read the handoff exactly once — cleared right after seeding defaultValues above.
  // biome-ignore lint/correctness/useExhaustiveDependencies: clearPending is stable (zustand action), intentionally run once on mount
  useEffect(() => {
    clearPending();
  }, []);

  const vendorId = isManual ? (manualVendorId ?? 0) : (pending?.vendorId ?? 0);
  const categoryError = category === "" ? "Wajib dipilih" : null;
  const replenishDateError = !replenishDate
    ? "Wajib diisi"
    : replenishDate < jakartaCalendarDateISO(0)
      ? "Tidak boleh sebelum hari ini"
      : null;
  const additionalDateOptions = [0, 1, 2].map((offset) => jakartaCalendarDateISO(offset));

  // Req 1.3-1.5: drives Replenish_Date on BOTH paths now, not just manual —
  // Planned -> H+1, Emergency -> H+0, Additional -> a dropdown restricted to
  // exactly {H+0, H+1, H+2} (rendered below), never a freely-typed date.
  function handleCategoryChange(next: VendorRequestCategory | ""): void {
    setCategory(next);
    if (next === "") {
      setReplenishDate("");
      return;
    }
    const additionalDefault = additionalDateOptions[0] ?? jakartaCalendarDateISO(0);
    if (next === "planned") setReplenishDate(jakartaCalendarDateISO(1));
    else if (next === "emergency") setReplenishDate(jakartaCalendarDateISO(0));
    else
      setReplenishDate((prev) => (additionalDateOptions.includes(prev) ? prev : additionalDefault));
  }

  function showErrorToast(message: string): void {
    const id = toast({ type: "error", message });
    setTimeout(() => dismiss(id), ERROR_TOAST_MS);
  }

  function goToDetail(id: number): void {
    navigate({ to: "/replenishment/vendor-requests/$id", params: { id: String(id) } });
  }

  function buildPayload(values: FormValues): CreateVendorRequestPayload {
    if (category === "") throw new Error("request_category is required before submit");
    return toPayload(values, {
      isManual,
      forecastDate: pending?.forecastDate ?? "",
      replenishDate,
      category,
      vendorId,
    });
  }

  async function handleSaveDraft(values: FormValues): Promise<void> {
    if (categoryError || replenishDateError || vendorId <= 0) return;
    try {
      const detail = await createMutation.mutateAsync(buildPayload(values));
      goToDetail(detail.id);
    } catch (err) {
      showErrorToast(errorMessage(err));
    }
  }

  async function handleSubmitForApproval(values: FormValues): Promise<void> {
    if (categoryError || replenishDateError || vendorId <= 0) return;
    let created: VendorRequestDetail;
    try {
      created = await createMutation.mutateAsync(buildPayload(values));
    } catch (err) {
      showErrorToast(errorMessage(err));
      return;
    }
    try {
      await submitMutation.mutateAsync(created.id);
    } catch (err) {
      showErrorToast(
        `Request tersimpan sebagai draft, tapi gagal dikirim untuk approval: ${errorMessage(err)}`,
      );
    }
    goToDetail(created.id);
  }

  const isBusy = createMutation.isPending || submitMutation.isPending;
  const canSubmit =
    form.formState.isValid && !isBusy && !categoryError && !replenishDateError && vendorId > 0;

  if (!pending && !manualRequested) {
    return (
      <div className="flex flex-col gap-6">
        <PageHeader eyebrow="Replenishment" title="Buat Vendor Request" />
        <div className="flex flex-col items-center gap-4 rounded-[var(--radius-lg)] border border-[var(--n-200)] bg-[var(--n-50)] p-8 text-center">
          <p className="text-sm text-[var(--n-700)]">
            Tidak ada item yang dipilih. Pilih item dari Forecast Browser, atau buat request manual
            (Emergency/Additional/Planned tanpa rekomendasi DMAA).
          </p>
          <div className="flex flex-wrap items-center justify-center gap-3">
            <Link
              to="/replenishment/forecast-browser"
              className="text-sm font-medium text-[var(--red-600)] hover:underline"
            >
              Kembali ke Forecast Browser
            </Link>
            <Button type="button" variant="secondary" onClick={() => setManualRequested(true)}>
              Buat Manual
            </Button>
          </div>
        </div>
      </div>
    );
  }

  return (
    <div className="flex flex-col gap-6">
      <PageHeader
        eyebrow="Replenishment"
        title="Buat Vendor Request"
        description={
          isManual
            ? "Buat request manual: pilih kategori, vendor, lalu masukkan item secara langsung"
            : "Tinjau item terpilih, sesuaikan jumlah bila perlu, lalu simpan sebagai draft atau kirim untuk approval"
        }
      />

      {!isManual && (
        <div className="flex flex-col gap-1">
          <span className="text-xs font-medium uppercase tracking-wider text-[var(--n-600)]">
            Tanggal Forecast
          </span>
          <span className="text-sm text-[var(--n-800)]">{pending.forecastDate}</span>
        </div>
      )}

      <div className="flex flex-wrap items-end gap-4">
        <div className="flex flex-col gap-1">
          <label
            htmlFor="request-category"
            className="text-xs font-medium uppercase tracking-wider text-[var(--n-600)]"
          >
            Kategori Request <span className="text-[var(--red-600)]">*</span>
          </label>
          <select
            id="request-category"
            value={category}
            onChange={(e) => handleCategoryChange(e.target.value as VendorRequestCategory | "")}
            className="min-h-[44px] min-w-[44px] rounded-[var(--radius-md)] border border-[var(--n-300)] bg-[var(--n-0)] px-3 text-sm text-[var(--n-800)] outline-none focus-visible:border-[var(--red-400)] focus-visible:ring-2 focus-visible:ring-[var(--red-100)]"
          >
            <option value="" disabled>
              Pilih Kategori
            </option>
            {CATEGORY_OPTIONS.map((opt) => (
              <option key={opt.value} value={opt.value}>
                {opt.label}
              </option>
            ))}
          </select>
          {categoryError && (
            <span className="text-xs text-[var(--danger-fg)]">{categoryError}</span>
          )}
        </div>

        {isManual && (
          <div className="flex flex-col gap-1">
            <label
              htmlFor="manual-vendor"
              className="text-xs font-medium uppercase tracking-wider text-[var(--n-600)]"
            >
              Vendor <span className="text-[var(--red-600)]">*</span>
            </label>
            <select
              id="manual-vendor"
              value={manualVendorId ?? ""}
              onChange={(e) => setManualVendorId(e.target.value ? Number(e.target.value) : null)}
              className="min-h-[44px] min-w-[44px] rounded-[var(--radius-md)] border border-[var(--n-300)] bg-[var(--n-0)] px-3 text-sm text-[var(--n-800)] outline-none focus-visible:border-[var(--red-400)] focus-visible:ring-2 focus-visible:ring-[var(--red-100)]"
            >
              <option value="" disabled>
                Pilih Vendor
              </option>
              {(vendorOptionsQuery.data?.vendors ?? []).map((v) => (
                <option key={v.id} value={v.id}>
                  {v.name}
                </option>
              ))}
            </select>
          </div>
        )}

        <div className="flex flex-col gap-1">
          <label
            htmlFor="replenish-date"
            className="text-xs font-medium uppercase tracking-wider text-[var(--n-600)]"
          >
            Tanggal Replenish <span className="text-[var(--red-600)]">*</span>
          </label>
          {category === "additional" ? (
            <select
              id="replenish-date"
              value={replenishDate}
              onChange={(e) => setReplenishDate(e.target.value)}
              className="min-h-[44px] rounded-[var(--radius-md)] border border-[var(--n-300)] bg-[var(--n-0)] px-3 text-sm text-[var(--n-800)] outline-none focus-visible:border-[var(--red-400)] focus-visible:ring-2 focus-visible:ring-[var(--red-100)]"
            >
              {additionalDateOptions.map((d, i) => (
                <option key={d} value={d}>{`H+${i} (${d})`}</option>
              ))}
            </select>
          ) : (
            // Planned/Emergency (both paths, Req 1.3/1.4) and the
            // not-yet-chosen "" state (Req 1.2) are all auto-set/locked —
            // never freely typed, so this is always a disabled display input.
            <input
              id="replenish-date"
              type="date"
              value={replenishDate}
              disabled
              className="min-h-[44px] rounded-[var(--radius-md)] border border-[var(--n-300)] bg-[var(--n-100)] px-3 text-sm text-[var(--n-600)] outline-none"
            />
          )}
          {replenishDateError && (
            <span className="text-xs text-[var(--danger-fg)]">{replenishDateError}</span>
          )}
        </div>
      </div>

      <div className="flex flex-col gap-1">
        <label
          htmlFor="vendor-request-notes"
          className="text-xs font-medium uppercase tracking-wider text-[var(--n-600)]"
        >
          Catatan (opsional)
        </label>
        <textarea
          id="vendor-request-notes"
          {...form.register("notes")}
          maxLength={500}
          rows={3}
          className="rounded-[var(--radius-md)] border border-[var(--n-300)] bg-[var(--n-0)] px-3 py-2 text-sm text-[var(--n-800)] outline-none focus-visible:border-[var(--red-400)] focus-visible:ring-2 focus-visible:ring-[var(--red-100)]"
        />
        {form.formState.errors.notes && (
          <span className="text-xs text-[var(--danger-fg)]">
            {form.formState.errors.notes.message}
          </span>
        )}
      </div>

      <div className="overflow-x-auto rounded-[var(--radius-lg)] border border-[var(--n-200)]">
        <table className="w-full min-w-[640px] border-collapse text-sm">
          <thead>
            <tr className="border-[var(--n-200)] border-b bg-[var(--n-50)]">
              <th className="px-3 py-2 text-left font-medium text-[var(--n-600)]">ATM ID</th>
              {!isManual && (
                <>
                  <th className="px-3 py-2 text-left font-medium text-[var(--n-600)]">Brand</th>
                  <th className="px-3 py-2 text-left font-medium text-[var(--n-600)]">
                    FLM Vendor
                  </th>
                  <th className="px-3 py-2 text-left font-medium text-[var(--n-600)]">
                    FLM Vendor Region
                  </th>
                </>
              )}
              {isManual && (
                <>
                  <th className="px-3 py-2 text-left font-medium text-[var(--n-600)]">
                    Brand (opsional)
                  </th>
                  <th className="px-3 py-2 text-left font-medium text-[var(--n-600)]">
                    Lokasi ATM (opsional)
                  </th>
                </>
              )}
              <th className="px-3 py-2 text-right font-medium text-[var(--n-600)]">Denominasi</th>
              <th className="px-3 py-2 text-right font-medium text-[var(--n-600)]">
                Amount Replenish
              </th>
              <th className="px-3 py-2" />
            </tr>
          </thead>
          <tbody>
            {fields.map((field, index) => (
              <tr key={field.id} className="border-[var(--n-100)] border-b last:border-0">
                {isManual ? (
                  <td className="px-3 py-2">
                    <input
                      type="text"
                      aria-label={`ATM ID baris ${index + 1}`}
                      {...form.register(`items.${index}.terminal_id`)}
                      className="w-32 rounded-[var(--radius-md)] border border-[var(--n-300)] bg-[var(--n-0)] px-2 py-1 text-sm outline-none focus-visible:border-[var(--red-400)] focus-visible:ring-2 focus-visible:ring-[var(--red-100)]"
                    />
                    {form.formState.errors.items?.[index]?.terminal_id && (
                      <div className="mt-1 text-xs text-[var(--danger-fg)]">
                        {form.formState.errors.items[index]?.terminal_id?.message}
                      </div>
                    )}
                  </td>
                ) : (
                  <td className="px-3 py-2">{field.terminal_id}</td>
                )}

                {!isManual && (
                  <>
                    <td className="px-3 py-2 text-[var(--n-700)]">{displayOrDash(field.brand)}</td>
                    <td className="px-3 py-2 text-[var(--n-700)]">
                      {displayOrDash(field.flm_vendor)}
                    </td>
                    <td className="px-3 py-2 text-[var(--n-700)]">
                      {displayOrDash(field.flm_vendor_region)}
                    </td>
                  </>
                )}
                {isManual && (
                  <>
                    <td className="px-3 py-2">
                      <input
                        type="text"
                        aria-label={`Brand baris ${index + 1}`}
                        {...form.register(`items.${index}.brand`)}
                        className="w-32 rounded-[var(--radius-md)] border border-[var(--n-300)] bg-[var(--n-0)] px-2 py-1 text-sm outline-none focus-visible:border-[var(--red-400)] focus-visible:ring-2 focus-visible:ring-[var(--red-100)]"
                      />
                    </td>
                    <td className="px-3 py-2">
                      <input
                        type="text"
                        aria-label={`Lokasi ATM baris ${index + 1}`}
                        {...form.register(`items.${index}.lokasi_atm`)}
                        className="w-36 rounded-[var(--radius-md)] border border-[var(--n-300)] bg-[var(--n-0)] px-2 py-1 text-sm outline-none focus-visible:border-[var(--red-400)] focus-visible:ring-2 focus-visible:ring-[var(--red-100)]"
                      />
                    </td>
                  </>
                )}

                {isManual ? (
                  <td className="px-3 py-2 text-right">
                    <input
                      type="number"
                      aria-label={`Denominasi baris ${index + 1}`}
                      {...form.register(`items.${index}.denom`, { valueAsNumber: true })}
                      className="w-28 rounded-[var(--radius-md)] border border-[var(--n-300)] bg-[var(--n-0)] px-2 py-1 text-right text-sm tabular-nums outline-none focus-visible:border-[var(--red-400)] focus-visible:ring-2 focus-visible:ring-[var(--red-100)]"
                    />
                    {form.formState.errors.items?.[index]?.denom && (
                      <div className="mt-1 text-xs text-[var(--danger-fg)]">
                        {form.formState.errors.items[index]?.denom?.message}
                      </div>
                    )}
                  </td>
                ) : (
                  <td className="px-3 py-2 text-right tabular-nums">{formatIDR(field.denom)}</td>
                )}

                <td className="px-3 py-2 text-right">
                  <input
                    type="number"
                    aria-label={`Amount replenish untuk ${field.terminal_id || `baris ${index + 1}`}`}
                    {...form.register(`items.${index}.amount_replenish`, { valueAsNumber: true })}
                    className="w-36 rounded-[var(--radius-md)] border border-[var(--n-300)] bg-[var(--n-0)] px-2 py-1 text-right text-sm tabular-nums outline-none focus-visible:border-[var(--red-400)] focus-visible:ring-2 focus-visible:ring-[var(--red-100)]"
                  />
                  {form.formState.errors.items?.[index]?.amount_replenish && (
                    <div className="mt-1 text-xs text-[var(--danger-fg)]">
                      {form.formState.errors.items[index]?.amount_replenish?.message}
                    </div>
                  )}
                </td>
                <td className="px-3 py-2 text-right">
                  <button
                    type="button"
                    onClick={() => remove(index)}
                    aria-label={`Hapus baris ${field.terminal_id || index + 1}`}
                    className="inline-flex min-h-[44px] min-w-[44px] items-center justify-center rounded-[var(--radius-md)] text-[var(--n-500)] outline-none hover:bg-[var(--n-100)] hover:text-[var(--danger-fg)] focus-visible:ring-2 focus-visible:ring-[var(--red-100)]"
                  >
                    <Trash2 className="h-4 w-4" aria-hidden="true" />
                  </button>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
      {form.formState.errors.items?.root && (
        <span className="text-xs text-[var(--danger-fg)]">
          {form.formState.errors.items.root.message}
        </span>
      )}

      {isManual && (
        <Button
          type="button"
          variant="secondary"
          onClick={() => append(emptyManualItem())}
          className="self-start"
        >
          <Plus className="mr-1 h-4 w-4" aria-hidden="true" />
          Tambah Baris
        </Button>
      )}

      <div className="flex flex-wrap items-center justify-between gap-4 rounded-[var(--radius-lg)] border border-[var(--n-200)] bg-[var(--n-50)] px-4 py-3">
        <div className="text-sm text-[var(--n-700)]">
          <span className="font-semibold text-[var(--n-900)]">{fields.length}</span> item — Total:{" "}
          <span className="font-semibold tabular-nums text-[var(--n-900)]">
            Rp {formatIDR(total)}
          </span>
        </div>
        <div className="flex gap-3">
          <Button
            type="button"
            variant="secondary"
            disabled={!canSubmit}
            onClick={form.handleSubmit(handleSaveDraft)}
          >
            {createMutation.isPending ? "Menyimpan..." : "Simpan sebagai Draft"}
          </Button>
          <Button
            type="button"
            disabled={!canSubmit}
            onClick={form.handleSubmit(handleSubmitForApproval)}
          >
            {submitMutation.isPending ? "Mengirim..." : "Kirim untuk Approval"}
          </Button>
        </div>
      </div>
    </div>
  );
}

function errorMessage(error: unknown): string {
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
