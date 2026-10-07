import { Button } from "@/components/ui/Button";
import { Dialog } from "@/components/ui/Dialog";
import type { ApiError } from "@/lib/api/client";
import { useToast } from "@/lib/hooks/useToast";
import { applyServerFieldError } from "@/lib/utils/applyServerFieldError";
import { zodResolver } from "@hookform/resolvers/zod";
import { useEffect } from "react";
import { useForm } from "react-hook-form";
import { useCreateRegion, useUpdateRegion } from "../hooks";
import { type CreateRegionFormValues, createRegionSchema } from "../types";
import type { AdminRegion } from "../types";

interface RegionFormDialogProps {
  open: boolean;
  onClose: () => void;
  region: AdminRegion | null;
}

/**
 * Maps a 409/400 RegionAdminService error to its field, for the cases the
 * shared applyServerFieldError (regex on a "field: message" backend message)
 * cannot: ErrRegionCodeConflict (409, create) and ErrRegionCodeImmutable
 * (400, update) are plain sentinel messages with no field prefix -- same
 * situation as ATMFormDialog's applyATMConflictOrReferenceError. Both cases
 * are about `code`, so status alone is enough to route the error.
 */
function applyRegionConflictOrImmutableError(
  error: unknown,
  setError: (field: "code", error: { message: string }) => void,
): boolean {
  const err = error as Partial<ApiError> | undefined;
  if (!err || typeof err.status !== "number" || typeof err.message !== "string") return false;
  if (err.status === 409 || err.status === 400) {
    setError("code", { message: err.message });
    return true;
  }
  return false;
}

type FormValues = CreateRegionFormValues;

/**
 * Create/edit form for admin regions. Code is immutable once created (Req
 * 3.2): in edit mode the field is disabled and never sent in the update
 * payload, so validating it against createRegionSchema is harmless (an
 * existing region's code is always already valid) -- this keeps one schema
 * and one FormValues type for both modes instead of a resolver union that
 * react-hook-form's generics can't express (TFieldValues must be a single
 * concrete type).
 */
export function RegionFormDialog({ open, onClose, region }: RegionFormDialogProps) {
  const mode = region ? "edit" : "create";
  const { toast } = useToast();
  const createMutation = useCreateRegion();
  const updateMutation = useUpdateRegion();

  const form = useForm<FormValues>({
    resolver: zodResolver(createRegionSchema),
    defaultValues: { code: region?.code ?? "", region: region?.region ?? "" },
  });

  // biome-ignore lint/correctness/useExhaustiveDependencies: reset only when the dialog (re)opens for a given region, not on every render
  useEffect(() => {
    if (!open) return;
    form.reset({ code: region?.code ?? "", region: region?.region ?? "" });
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open, region]);

  const isPending = createMutation.isPending || updateMutation.isPending;

  async function onSubmit(values: FormValues): Promise<void> {
    try {
      if (mode === "create") {
        const created = await createMutation.mutateAsync({
          code: values.code,
          region: values.region,
        });
        toast({ type: "success", message: `Region "${created.code}" berhasil dibuat` });
      } else {
        if (!region) return;
        const updated = await updateMutation.mutateAsync({
          id: region.id,
          payload: { region: values.region },
        });
        toast({ type: "success", message: `Region "${updated.code}" berhasil diubah` });
      }
      onClose();
    } catch (err) {
      const setError = (field: "code" | "region", error: { message: string }) =>
        form.setError(field, error);
      const handled =
        applyServerFieldError(err, (field, error) => setError(field as "code" | "region", error)) ||
        applyRegionConflictOrImmutableError(err, setError);
      if (!handled) {
        const message =
          err && typeof err === "object" && "message" in err
            ? String((err as { message: unknown }).message)
            : "Terjadi kesalahan yang tidak diketahui";
        toast({ type: "error", message });
      }
      // Dialog stays open either way so the operator can see/fix the error.
    }
  }

  return (
    <Dialog
      open={open}
      onClose={onClose}
      title={mode === "create" ? "Tambah Region" : "Ubah Region"}
    >
      <form onSubmit={form.handleSubmit(onSubmit)} className="flex flex-col gap-4">
        <Field id="code" label="Code" error={form.formState.errors.code?.message}>
          <input
            id="code"
            {...form.register("code")}
            disabled={mode === "edit"}
            maxLength={20}
            className={inputClass}
          />
        </Field>

        <Field id="region" label="Nama" error={form.formState.errors.region?.message}>
          <input id="region" {...form.register("region")} maxLength={100} className={inputClass} />
        </Field>

        <div className="flex justify-end gap-3 pt-2">
          <Button type="button" variant="secondary" disabled={isPending} onClick={onClose}>
            Batal
          </Button>
          <Button type="submit" disabled={isPending}>
            Simpan
          </Button>
        </div>
      </form>
    </Dialog>
  );
}

const inputClass =
  "min-h-[44px] w-full rounded-[var(--radius-md)] border border-[var(--n-300)] bg-[var(--n-0)] px-3 text-sm text-[var(--n-800)] outline-none focus-visible:border-[var(--red-400)] focus-visible:ring-2 focus-visible:ring-[var(--red-100)] disabled:bg-[var(--n-100)] disabled:text-[var(--n-500)]";

function Field({
  id,
  label,
  error,
  children,
}: {
  id: string;
  label: string;
  error?: string;
  children: React.ReactNode;
}) {
  return (
    <div className="flex flex-col gap-1">
      <label
        htmlFor={id}
        className="text-xs font-medium uppercase tracking-wider text-[var(--n-600)]"
      >
        {label}
      </label>
      {children}
      {error && <span className="text-xs text-[var(--danger-fg)]">{error}</span>}
    </div>
  );
}
