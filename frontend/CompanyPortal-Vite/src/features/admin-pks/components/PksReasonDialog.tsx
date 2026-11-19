import { Button } from "@/components/ui/Button";
import { Dialog } from "@/components/ui/Dialog";
import { useToast } from "@/lib/hooks/useToast";
import { applyServerFieldError } from "@/lib/utils/applyServerFieldError";
import { zodResolver } from "@hookform/resolvers/zod";
import { useState } from "react";
import { useForm } from "react-hook-form";
import { pendingApprovalMessage } from "../../master-data/changeRequest";
import { useTerminateVendorPks, useUnterminateVendorPks } from "../hooks";
import { type PksReasonValues, errorMessage, labelClass, pksReasonSchema } from "../lib/pksForm";
import type { PksListItem } from "../types";

export type PksReasonMode = "terminate" | "unterminate";

const COPY = {
  terminate: {
    title: "Terminasi PKS",
    hint: "PKS menjadi Terminated setelah disetujui. Tanggal berlaku tidak berubah.",
  },
  unterminate: {
    title: "Batalkan terminasi",
    hint: "PKS kembali ke status sesuai tanggalnya setelah disetujui.",
  },
} as const;

interface PksReasonDialogProps {
  item: PksListItem;
  mode: PksReasonMode;
  onClose: () => void;
}

/** Alasan wajib (maks 500) untuk terminasi / batal terminasi. 202 = diajukan (maker-checker). Mount only while open. */
export function PksReasonDialog({ item, mode, onClose }: PksReasonDialogProps) {
  const { toast } = useToast();
  const terminate = useTerminateVendorPks(item.vendor_id);
  const unterminate = useUnterminateVendorPks(item.vendor_id);
  const mutation = mode === "terminate" ? terminate : unterminate;
  const [banner, setBanner] = useState<string | null>(null);
  const form = useForm<PksReasonValues>({
    resolver: zodResolver(pksReasonSchema),
    defaultValues: { reason: "" },
  });

  async function onSubmit(values: PksReasonValues): Promise<void> {
    if (item.id === null) return;
    setBanner(null);
    try {
      const res = await mutation.mutateAsync({
        pksId: item.id,
        payload: { reason: values.reason },
      });
      toast({ type: "success", message: pendingApprovalMessage(res) });
      onClose();
    } catch (err) {
      const handled = applyServerFieldError(err, (field, e) =>
        form.setError(field as keyof PksReasonValues, e),
      );
      if (!handled) setBanner(errorMessage(err));
    }
  }

  const reasonError = form.formState.errors.reason?.message;

  return (
    <Dialog open onClose={onClose} title={`${COPY[mode].title}: ${item.pks_name}`}>
      <form onSubmit={form.handleSubmit(onSubmit)} className="flex flex-col gap-4" noValidate>
        {banner && (
          <p role="alert" className="text-sm font-medium text-[var(--danger-fg)]">
            {banner}
          </p>
        )}
        <p className="text-sm text-[var(--n-700)]">{COPY[mode].hint}</p>
        <div className="flex flex-col gap-1">
          <label htmlFor="pks-reason" className={labelClass}>
            Alasan
          </label>
          <textarea
            id="pks-reason"
            rows={4}
            maxLength={500}
            aria-invalid={reasonError ? true : undefined}
            aria-describedby={reasonError ? "pks-reason-error" : undefined}
            {...form.register("reason")}
            className="rounded-[var(--radius-md)] border border-[var(--n-300)] bg-[var(--n-0)] px-3 py-2 text-sm text-[var(--n-800)] outline-none focus-visible:border-[var(--red-400)] focus-visible:ring-2 focus-visible:ring-[var(--red-100)]"
          />
          {reasonError && (
            <span id="pks-reason-error" className="text-xs text-[var(--danger-fg)]">
              {reasonError}
            </span>
          )}
        </div>
        <div className="flex justify-end gap-3 pt-2">
          <Button type="button" variant="secondary" disabled={mutation.isPending} onClick={onClose}>
            Batal
          </Button>
          <Button type="submit" disabled={mutation.isPending}>
            Ajukan
          </Button>
        </div>
      </form>
    </Dialog>
  );
}
