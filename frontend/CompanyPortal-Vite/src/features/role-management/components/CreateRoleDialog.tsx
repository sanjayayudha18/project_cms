import { Button } from "@/components/ui/Button";
import { Dialog } from "@/components/ui/Dialog";
import { useToast } from "@/lib/hooks/useToast";
import { applyServerFieldError } from "@/lib/utils/applyServerFieldError";
import { zodResolver } from "@hookform/resolvers/zod";
import { useEffect } from "react";
import { type Resolver, useForm } from "react-hook-form";
import { useCreateRole } from "../hooks/useRoleQueries";
import {
  type CreateRoleFormValues,
  type PartyScope,
  type Portal,
  createRoleSchema,
} from "../types";

interface CreateRoleDialogProps {
  open: boolean;
  onClose: () => void;
  /** Tab the role is created in; vendor roles get a locked VENDOR- prefix and a required scope. */
  portal?: Portal;
}

type DialogFormValues = CreateRoleFormValues & { party_scope: PartyScope | "" };

const VENDOR_PREFIX = "VENDOR-";
const DEFAULT_VALUES: DialogFormValues = { role: "", description: "", party_scope: "" };

const SCOPE_OPTIONS: { value: PartyScope; label: string; hint: string }[] = [
  { value: "replenish", label: "Replenish", hint: "Pihak replenish saja" },
  { value: "vault", label: "Vault", hint: "Pihak vault saja" },
];

/**
 * Create-role form (Req 2.1, 2.2, 7.2, 7.3): RHF + zodResolver(createRoleSchema)
 * validates client-side before the backend call. New roles are born with
 * zero access — permission granting happens afterward in PermissionEditor.
 * Vendor portal: the "VENDOR-" prefix is locked (only the suffix is typed) and
 * exactly one party scope is required.
 */
export function CreateRoleDialog({ open, onClose, portal = "main" }: CreateRoleDialogProps) {
  const isVendor = portal === "vendor";
  const { toast } = useToast();
  const createMutation = useCreateRole();

  const form = useForm<DialogFormValues>({
    resolver: zodResolver(createRoleSchema) as unknown as Resolver<DialogFormValues>, // party_scope is validated in onSubmit
    defaultValues: DEFAULT_VALUES,
  });
  const { errors } = form.formState;

  // biome-ignore lint/correctness/useExhaustiveDependencies: reset only when the dialog (re)opens, not on every render
  useEffect(() => {
    if (open) form.reset(DEFAULT_VALUES);
  }, [open]);

  async function onSubmit(values: DialogFormValues): Promise<void> {
    const scope = form.getValues("party_scope");
    if (isVendor && !scope) {
      form.setError("party_scope", { message: "Pilih cakupan pihak" });
      return;
    }
    try {
      await createMutation.mutateAsync({
        description: values.description,
        role: isVendor ? `${VENDOR_PREFIX}${values.role}` : values.role,
        portal,
        ...(isVendor && scope ? { party_scope: [scope] } : {}),
      });
      toast({ type: "success", message: "Peran berhasil dibuat" });
      onClose();
    } catch (err) {
      const handled = applyServerFieldError(err, (field, error) =>
        form.setError(field as keyof DialogFormValues, error),
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
    <Dialog
      open={open}
      onClose={onClose}
      title={isVendor ? "Buat Peran Portal Cabang" : "Buat Peran Baru"}
    >
      <form onSubmit={form.handleSubmit(onSubmit)} className="flex flex-col gap-4">
        <Field id="role" label="Nama Peran" error={errors.role?.message}>
          <div className="flex">
            {isVendor && (
              <span
                id="role-prefix"
                className="inline-flex min-h-[44px] items-center rounded-l-[var(--radius-md)] border border-r-0 border-[var(--n-300)] bg-[var(--n-100)] px-3 text-sm text-[var(--n-700)]"
              >
                {VENDOR_PREFIX}
              </span>
            )}
            <input
              id="role"
              {...form.register("role")}
              placeholder={isVendor ? "REPLENISH-CHECKER" : "AUDITOR-VIEW"}
              aria-describedby={[isVendor ? "role-prefix" : "", errors.role ? "role-error" : ""]
                .filter(Boolean)
                .join(" ")}
              aria-invalid={!!errors.role}
              className={`${inputClass} min-w-0 flex-1 ${isVendor ? "rounded-l-none" : ""}`}
            />
          </div>
        </Field>

        <Field id="description" label="Deskripsi" error={errors.description?.message}>
          <input id="description" {...form.register("description")} className={inputClass} />
        </Field>

        {isVendor && (
          <fieldset
            className="flex flex-col gap-2"
            aria-describedby={errors.party_scope ? "party_scope-error" : undefined}
          >
            <legend className="text-xs font-medium uppercase tracking-wider text-[var(--n-600)]">
              Cakupan Pihak <span aria-hidden="true">*</span>
            </legend>
            {SCOPE_OPTIONS.map((opt) => (
              <label
                key={opt.value}
                className="flex min-h-[44px] items-center gap-2 text-sm text-[var(--n-800)]"
              >
                <input
                  type="radio"
                  value={opt.value}
                  {...form.register("party_scope")}
                  className="size-4 accent-[var(--red-500)] focus-visible:ring-2 focus-visible:ring-[var(--red-100)]"
                />
                <span className="font-medium">{opt.label}</span>
                <span className="text-[var(--n-600)]">— {opt.hint}</span>
              </label>
            ))}
            {errors.party_scope && (
              <span id="party_scope-error" role="alert" className="text-xs text-[var(--danger-fg)]">
                {errors.party_scope.message}
              </span>
            )}
          </fieldset>
        )}

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
      {error && (
        <span id={`${id}-error`} role="alert" className="text-xs text-[var(--danger-fg)]">
          {error}
        </span>
      )}
    </div>
  );
}
