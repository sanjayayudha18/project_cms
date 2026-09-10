/**
 * Vendor Request creation page (Req 12): review items carried from the
 * Forecast Browser (via selectionStore), edit amounts/notes, then save as
 * draft or submit for approval.
 */

import { Button } from "@/components/ui/Button";
import { PageHeader } from "@/components/ui/PageHeader";
import { useToast } from "@/lib/hooks/useToast";
import { formatIDR } from "@/lib/utils/formatCurrency";
import { zodResolver } from "@hookform/resolvers/zod";
import { Link, useNavigate } from "@tanstack/react-router";
import { Trash2 } from "lucide-react";
import { useEffect, useState } from "react";
import { useFieldArray, useForm } from "react-hook-form";
import { z } from "zod";
import { useCreateVendorRequest, useSubmitVendorRequest } from "./hooks";
import { usePendingVendorRequestSelection } from "./selectionStore";
import type { CreateVendorRequestPayload, VendorRequestDetail } from "./types";

const AMOUNT_MIN = 1;
const AMOUNT_MAX = 999_999_999;
const ERROR_TOAST_MS = 5000;

const itemSchema = z.object({
  terminal_id: z.string(),
  periode_pred: z.string(),
  denom: z.number(),
  amount_refund: z.number(),
  amount_replenish: z
    .number({ invalid_type_error: "Wajib berupa angka" })
    .int("Harus bilangan bulat")
    .min(AMOUNT_MIN, `Minimal ${AMOUNT_MIN}`)
    .max(AMOUNT_MAX, `Maksimal ${formatIDR(AMOUNT_MAX)}`),
});

const formSchema = z.object({
  notes: z.string().max(500, "Maksimal 500 karakter").optional(),
  items: z.array(itemSchema).min(1, "Minimal 1 item diperlukan"),
});

type FormValues = z.infer<typeof formSchema>;

function toPayload(values: FormValues, forecastDate: string): CreateVendorRequestPayload {
  return {
    forecast_date: forecastDate,
    notes: values.notes,
    items: values.items.map((it) => ({
      terminal_id: it.terminal_id,
      periode_pred: it.periode_pred,
      denom: it.denom,
      amount_replenish: it.amount_replenish,
    })),
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

  const form = useForm<FormValues>({
    resolver: zodResolver(formSchema),
    mode: "onChange",
    defaultValues: {
      notes: "",
      items: (pending?.items ?? []).map((it) => ({
        terminal_id: it.terminal_id,
        periode_pred: it.periode_pred,
        denom: it.denom,
        amount_refund: it.amount_refund,
        amount_replenish: it.amount_replenish,
      })),
    },
  });
  const { fields, remove } = useFieldArray({ control: form.control, name: "items" });
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

  function showErrorToast(message: string): void {
    const id = toast({ type: "error", message });
    setTimeout(() => dismiss(id), ERROR_TOAST_MS);
  }

  function goToDetail(id: number): void {
    navigate({ to: "/replenishment/vendor-requests/$id", params: { id: String(id) } });
  }

  async function handleSaveDraft(values: FormValues): Promise<void> {
    if (!pending) return;
    try {
      const detail = await createMutation.mutateAsync(toPayload(values, pending.forecastDate));
      goToDetail(detail.id);
    } catch (err) {
      showErrorToast(errorMessage(err));
    }
  }

  async function handleSubmitForApproval(values: FormValues): Promise<void> {
    if (!pending) return;
    let created: VendorRequestDetail;
    try {
      created = await createMutation.mutateAsync(toPayload(values, pending.forecastDate));
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
  const canSubmit = form.formState.isValid && !isBusy;

  if (!pending) {
    return (
      <div className="flex flex-col gap-6">
        <PageHeader eyebrow="Replenishment" title="Buat Vendor Request" />
        <div className="rounded-[var(--radius-lg)] border border-[var(--n-200)] bg-[var(--n-50)] p-8 text-center">
          <p className="text-sm text-[var(--n-700)]">
            Tidak ada item yang dipilih. Silakan pilih item dari Forecast Browser terlebih dahulu.
          </p>
          <Link
            to="/replenishment/forecast-browser"
            className="mt-4 inline-block text-sm font-medium text-[var(--red-600)] hover:underline"
          >
            Kembali ke Forecast Browser
          </Link>
        </div>
      </div>
    );
  }

  return (
    <div className="flex flex-col gap-6">
      <PageHeader
        eyebrow="Replenishment"
        title="Buat Vendor Request"
        description="Tinjau item terpilih, sesuaikan jumlah bila perlu, lalu simpan sebagai draft atau kirim untuk approval"
      />

      <div className="flex flex-col gap-1">
        <span className="text-xs font-medium uppercase tracking-wider text-[var(--n-600)]">
          Tanggal Forecast
        </span>
        <span className="text-sm text-[var(--n-800)]">{pending.forecastDate}</span>
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
                <td className="px-3 py-2">{field.terminal_id}</td>
                <td className="px-3 py-2 text-right tabular-nums">{formatIDR(field.denom)}</td>
                <td className="px-3 py-2 text-right">
                  <input
                    type="number"
                    aria-label={`Amount replenish untuk ${field.terminal_id}`}
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
                    aria-label={`Hapus baris ${field.terminal_id}`}
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
