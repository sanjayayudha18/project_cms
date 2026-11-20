import { Button } from "@/components/ui/Button";
import { Dialog } from "@/components/ui/Dialog";
import { useToast } from "@/lib/hooks/useToast";
import { applyServerFieldError } from "@/lib/utils/applyServerFieldError";
import { zodResolver } from "@hookform/resolvers/zod";
import { useState } from "react";
import { useForm } from "react-hook-form";
import { z } from "zod";
import { useUpdatePksReminder } from "../hooks";
import {
  RENEWAL_MAX,
  RENEWAL_MIN,
  errorMessage,
  inputClass,
  labelClass,
  pksDurationDays,
  renewalDaysField,
} from "../lib/pksForm";
import type { PksListItem } from "../types";

interface PksReminderDialogProps {
  item: PksListItem;
  onClose: () => void;
}

/** Ubah reminder (hari) satu PKS: langsung berlaku (bukan maker-checker). Mount only while open. */
export function PksReminderDialog({ item, onClose }: PksReminderDialogProps) {
  const { toast } = useToast();
  const mutation = useUpdatePksReminder();
  const [banner, setBanner] = useState<string | null>(null);
  const maxDays = Math.min(
    RENEWAL_MAX,
    pksDurationDays(item.effective_date, item.expiry_date) ?? RENEWAL_MAX,
  );

  const schema = z.object({
    renewal_days: renewalDaysField.max(maxDays, `Tidak boleh melebihi masa PKS (${maxDays} hari)`),
  });
  type Values = z.infer<typeof schema>;
  const form = useForm<Values>({
    resolver: zodResolver(schema),
    defaultValues: { renewal_days: item.renewal_days },
  });

  async function onSubmit(values: Values): Promise<void> {
    if (item.id === null) return;
    setBanner(null);
    try {
      await mutation.mutateAsync({ pksId: item.id, renewalDays: values.renewal_days });
      toast({ type: "success", message: "Reminder diperbarui" });
      onClose();
    } catch (err) {
      const handled = applyServerFieldError(err, (field, e) =>
        form.setError(field as keyof Values, e),
      );
      if (!handled) setBanner(errorMessage(err));
    }
  }

  const error = form.formState.errors.renewal_days?.message;

  return (
    <Dialog open onClose={onClose} title={`Ubah reminder: ${item.pks_name}`}>
      <form onSubmit={form.handleSubmit(onSubmit)} className="flex flex-col gap-4" noValidate>
        {banner && (
          <p role="alert" className="text-sm font-medium text-[var(--danger-fg)]">
            {banner}
          </p>
        )}
        <div className="flex flex-col gap-1">
          <label htmlFor="pks-reminder" className={labelClass}>
            Reminder (hari sebelum expired)
          </label>
          <input
            id="pks-reminder"
            type="number"
            inputMode="numeric"
            min={RENEWAL_MIN}
            max={maxDays}
            aria-invalid={error ? true : undefined}
            aria-describedby="pks-reminder-hint"
            {...form.register("renewal_days", { valueAsNumber: true })}
            className={`${inputClass} tabular-nums`}
          />
          <span id="pks-reminder-hint" className="text-xs text-[var(--n-600)]">
            {RENEWAL_MIN}–{maxDays} hari. Berlaku langsung, tanpa persetujuan.
          </span>
          {error && (
            <span role="alert" className="text-xs text-[var(--danger-fg)]">
              {error}
            </span>
          )}
        </div>
        <div className="flex justify-end gap-3 pt-2">
          <Button type="button" variant="secondary" disabled={mutation.isPending} onClick={onClose}>
            Batal
          </Button>
          <Button type="submit" disabled={mutation.isPending}>
            Simpan
          </Button>
        </div>
      </form>
    </Dialog>
  );
}
