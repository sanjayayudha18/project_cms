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
 *
 * Split (FE-12): schema/payload helpers live in vendorRequestCreateForm.ts,
 * header inputs in VendorRequestCreateFields.tsx, the item table in
 * VendorRequestCreateItemsTable.tsx; this file owns state and submit flow.
 */

import { Button } from "@/components/ui/Button";
import { PageHeader } from "@/components/ui/PageHeader";
import { useToast } from "@/lib/hooks/useToast";
import { formatThousands } from "@/lib/utils/formatCurrency";
import { zodResolver } from "@hookform/resolvers/zod";
import { Link, useNavigate } from "@tanstack/react-router";
import { useEffect, useState } from "react";
import { useFieldArray, useForm } from "react-hook-form";
import { VendorRequestCreateFields } from "./VendorRequestCreateFields";
import { VendorRequestCreateItemsTable } from "./VendorRequestCreateItemsTable";
import { useCreateVendorRequest, useSubmitVendorRequest, useVendorOptions } from "./hooks";
import { jakartaCalendarDateISO } from "./lib/nextBusinessDay";
import { usePendingVendorRequestSelection } from "./selectionStore";
import type {
  CreateVendorRequestPayload,
  VendorRequestCategory,
  VendorRequestDetail,
} from "./types";
import { type FormValues, errorMessage, formSchema, toPayload } from "./vendorRequestCreateForm";

const ERROR_TOAST_MS = 5000;

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
  const todayISO = jakartaCalendarDateISO(0);

  // Req 1.3-1.5: drives Replenish_Date on BOTH paths now, not just manual —
  // Planned -> H+1, Emergency -> H+0, Additional -> operator-picked date
  // (today or later; replenishDateError blocks past dates).
  function handleCategoryChange(next: VendorRequestCategory | ""): void {
    setCategory(next);
    if (next === "") {
      setReplenishDate("");
      return;
    }
    if (next === "planned") setReplenishDate(jakartaCalendarDateISO(1));
    else if (next === "emergency") setReplenishDate(todayISO);
    else setReplenishDate((prev) => (prev >= todayISO ? prev : todayISO));
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

      <VendorRequestCreateFields
        isManual={isManual}
        forecastDate={pending?.forecastDate ?? ""}
        category={category}
        categoryError={categoryError}
        onCategoryChange={handleCategoryChange}
        manualVendorId={manualVendorId}
        onManualVendorChange={setManualVendorId}
        vendors={vendorOptionsQuery.data?.vendors ?? []}
        replenishDate={replenishDate}
        replenishDateError={replenishDateError}
        todayISO={todayISO}
        onReplenishDateChange={setReplenishDate}
        form={form}
      />

      <VendorRequestCreateItemsTable
        isManual={isManual}
        form={form}
        fields={fields}
        onRemove={remove}
        onAppend={append}
      />

      <div className="flex flex-wrap items-center justify-between gap-4 rounded-[var(--radius-lg)] border border-[var(--n-200)] bg-[var(--n-50)] px-4 py-3">
        <div className="text-sm text-[var(--n-700)]">
          <span className="font-semibold text-[var(--n-900)]">{fields.length}</span> item — Total:{" "}
          <span className="font-semibold tabular-nums text-[var(--n-900)]">
            Rp {formatThousands(total)}
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
