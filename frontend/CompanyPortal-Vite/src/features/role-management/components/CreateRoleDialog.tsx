import { Button } from "@/components/ui/Button";
import { Dialog } from "@/components/ui/Dialog";
import { useToast } from "@/lib/hooks/useToast";
import { applyServerFieldError } from "@/lib/utils/applyServerFieldError";
import { zodResolver } from "@hookform/resolvers/zod";
import { useEffect } from "react";
import { useForm } from "react-hook-form";
import { useCreateRole } from "../hooks/useRoleQueries";
import { type CreateRoleFormValues, createRoleSchema } from "../types";

interface CreateRoleDialogProps {
  open: boolean;
  onClose: () => void;
}

const DEFAULT_VALUES: CreateRoleFormValues = { role: "", description: "" };

/**
 * Create-role form (Req 2.1, 7.2, 7.3): RHF + zodResolver(createRoleSchema)
 * validates client-side before the backend call. New roles are born with
 * zero access — permission granting happens afterward in PermissionEditor,
 * not here.
 */
export function CreateRoleDialog({ open, onClose }: CreateRoleDialogProps) {
  const { toast } = useToast();
  const createMutation = useCreateRole();

  const form = useForm<CreateRoleFormValues>({
    resolver: zodResolver(createRoleSchema),
    defaultValues: DEFAULT_VALUES,
  });

  // biome-ignore lint/correctness/useExhaustiveDependencies: reset only when the dialog (re)opens, not on every render
  useEffect(() => {
    if (open) form.reset(DEFAULT_VALUES);
  }, [open]);

  async function onSubmit(values: CreateRoleFormValues): Promise<void> {
    try {
      await createMutation.mutateAsync(values);
      toast({ type: "success", message: "Peran berhasil dibuat" });
      onClose();
    } catch (err) {
      const handled = applyServerFieldError(err, (field, error) =>
        form.setError(field as keyof CreateRoleFormValues, error),
      );
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
    <Dialog open={open} onClose={onClose} title="Buat Peran Baru">
      <form onSubmit={form.handleSubmit(onSubmit)} className="flex flex-col gap-4">
        <Field id="role" label="Nama Peran" error={form.formState.errors.role?.message}>
          <input
            id="role"
            {...form.register("role")}
            placeholder="AUDITOR-VIEW"
            className={inputClass}
          />
        </Field>

        <Field
          id="description"
          label="Deskripsi"
          error={form.formState.errors.description?.message}
        >
          <input id="description" {...form.register("description")} className={inputClass} />
        </Field>

        <p className="text-xs text-[var(--n-500)]">
          Peran baru dibuat tanpa akses menu/fitur apa pun. Atur izinnya setelah dibuat.
        </p>

        <div className="flex justify-end gap-3 pt-2">
          <Button
            type="button"
            variant="secondary"
            disabled={createMutation.isPending}
            onClick={onClose}
          >
            Batal
          </Button>
          <Button type="submit" disabled={createMutation.isPending}>
            Buat Peran
          </Button>
        </div>
      </form>
    </Dialog>
  );
}

const inputClass =
  "min-h-[44px] rounded-[var(--radius-md)] border border-[var(--n-300)] bg-[var(--n-0)] px-3 text-sm text-[var(--n-800)] outline-none focus-visible:border-[var(--red-400)] focus-visible:ring-2 focus-visible:ring-[var(--red-100)] disabled:bg-[var(--n-100)] disabled:text-[var(--n-500)]";

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
