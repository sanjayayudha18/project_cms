import { Button } from "@/components/ui/Button";
import { Dialog } from "@/components/ui/Dialog";
import { useToast } from "@/lib/hooks/useToast";
import { applyServerFieldError } from "@/lib/utils/applyServerFieldError";
import { zodResolver } from "@hookform/resolvers/zod";
import { useEffect, useState } from "react";
import { useForm } from "react-hook-form";
import { z } from "zod";
import { pendingApprovalMessage } from "../../master-data/changeRequest";
import { useTerminateVendorPks, useUnterminateVendorPks } from "../hooks";

export const pksReasonSchema = z.object({
  reason: z.string().trim().min(1, "Alasan wajib diisi").max(500, "Maksimal 500 karakter"),
});

type ReasonValues = z.infer<typeof pksReasonSchema>;

interface PksReasonDialogProps {
  open: boolean;
  onClose: () => void;
  vendorId: number;
  pksId: number;
  pksName: string;
  mode: "terminate" | "unterminate";
}

const COPY = {
  terminate: {
    title: "Terminate PKS",
    hint: "PKS menjadi Terminated setelah disetujui. Tanggal berlaku tidak berubah.",
  },
  unterminate: {
    title: "Batalkan terminate",
    hint: "PKS kembali ke status sesuai tanggalnya setelah disetujui.",
  },
} as const;

function errorMessage(err: unknown): string {
  return err && typeof err === "object" && "message" in err
    ? String((err as { message: unknown }).message)
    : "Terjadi kesalahan yang tidak diketahui";
}

/** Alasan wajib (maks 500) untuk terminate / batalkan terminate. 202 = diajukan, bukan tersimpan. */
export function PksReasonDialog({
  open,
  onClose,
  vendorId,
  pksId,
  pksName,
  mode,
}: PksReasonDialogProps) {
  const { toast } = useToast();
  const terminate = useTerminateVendorPks(vendorId);
  const unterminate = useUnterminateVendorPks(vendorId);
  const mutation = mode === "terminate" ? terminate : unterminate;
  const [banner, setBanner] = useState<string | null>(null);

  const form = useForm<ReasonValues>({
    resolver: zodResolver(pksReasonSchema),
    defaultValues: { reason: "" },
  });

  // biome-ignore lint/correctness/useExhaustiveDependencies: reset only when the dialog (re)opens
  useEffect(() => {
    if (open) {
      form.reset({ reason: "" });
      setBanner(null);
    }
  }, [open]);

  async function onSubmit(values: ReasonValues): Promise<void> {
    setBanner(null);
    try {
      const res = await mutation.mutateAsync({ pksId, payload: { reason: values.reason } });
      toast({ type: "success", message: pendingApprovalMessage(res) });
      onClose();
    } catch (err) {
      const handled = applyServerFieldError(err, (field, e) =>
        form.setError(field as keyof ReasonValues, e),
      );
      if (!handled) setBanner(errorMessage(err));
    }
  }

  const reasonError = form.formState.errors.reason?.message;

  return (
    <Dialog open={open} onClose={onClose} title={`${COPY[mode].title}: ${pksName}`}>
      <form onSubmit={form.handleSubmit(onSubmit)} className="flex flex-col gap-4" noValidate>
        {banner && (
          <p role="alert" className="text-sm font-medium text-[var(--danger-fg)]">
            {banner}
          </p>
        )}
        <p className="text-sm text-[var(--n-700)]">{COPY[mode].hint}</p>

        <div className="flex flex-col gap-1">
          <label
            htmlFor="pks-reason"
            className="text-xs font-medium uppercase tracking-wider text-[var(--n-600)]"
          >
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
