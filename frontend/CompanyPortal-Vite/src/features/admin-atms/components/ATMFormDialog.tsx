import { Button } from "@/components/ui/Button";
import { Dialog } from "@/components/ui/Dialog";
import type { ApiError } from "@/lib/api/client";
import { useToast } from "@/lib/hooks/useToast";
import { applyServerFieldError } from "@/lib/utils/applyServerFieldError";
import { zodResolver } from "@hookform/resolvers/zod";
import { useEffect } from "react";
import { useForm } from "react-hook-form";
import { pendingApprovalMessage } from "../../master-data/changeRequest";
import { useCreateATM, useLocationOptions, useUpdateATM } from "../hooks";
import { type ATMFormValues, PRIORITY_CLASS_VALUES, atmFormSchema } from "../lib/atmFormSchema";
import type { AdminATM, CreateATMPayload, UpdateATMPayload } from "../types";

interface ATMFormDialogProps {
  open: boolean;
  onClose: () => void;
  atm: AdminATM | null;
}

function toDefaultValues(atm: AdminATM | null): ATMFormValues {
  if (!atm) {
    return {
      terminal_id: "",
      location_id: 0,
      machine_type: "",
      brand: "",
      model: "",
      operation_hours: "",
      deployment_type: "",
      capacity_amount: "",
      low_threshold_amount: "",
      critical_threshold_amount: "",
      blacklisted: false,
      escrow_account: "",
      priority_class: "",
    };
  }
  return {
    terminal_id: atm.terminal_id,
    location_id: atm.location_id,
    machine_type: atm.machine_type,
    brand: atm.brand,
    model: atm.model,
    operation_hours: atm.operation_hours,
    deployment_type: atm.deployment_type,
    capacity_amount: atm.capacity_amount ?? "",
    low_threshold_amount: atm.low_threshold_amount ?? "",
    critical_threshold_amount: atm.critical_threshold_amount ?? "",
    blacklisted: atm.blacklisted,
    escrow_account: atm.escrow_account ?? "",
    priority_class: atm.priority_class ?? "",
  };
}

/**
 * Maps a 409/400 ATMAdminService error to its field, for the cases the
 * shared applyServerFieldError (regex on a "field: message" backend
 * message) cannot: ErrATMTerminalIDConflict/ErrATMInvalidReference
 * (atm_admin.go) are plain sentinel messages with no field prefix. Since
 * terminal_id is never sent on update (disabled + omitted from the
 * payload, see onSubmit), the only case this form can ever produce a 400
 * for is invalid_reference (location_id) -- the immutable-terminal_id 400
 * cannot occur here.
 */
function applyATMConflictOrReferenceError(
  error: unknown,
  setError: (field: keyof ATMFormValues, error: { message: string }) => void,
): boolean {
  const err = error as Partial<ApiError> | undefined;
  if (!err || typeof err.status !== "number" || typeof err.message !== "string") return false;
  if (err.status === 409) {
    setError("terminal_id", { message: err.message });
    return true;
  }
  if (err.status === 400) {
    setError("location_id", { message: err.message });
    return true;
  }
  return false;
}

/** Create/edit form for admin ATMs (Req 3-4). Terminal ID is immutable once created (Req 4.2). */
export function ATMFormDialog({ open, onClose, atm }: ATMFormDialogProps) {
  const mode = atm ? "edit" : "create";
  const { toast } = useToast();
  const createMutation = useCreateATM();
  const updateMutation = useUpdateATM();
  const locationOptionsQuery = useLocationOptions();

  const form = useForm<ATMFormValues>({
    resolver: zodResolver(atmFormSchema),
    defaultValues: toDefaultValues(atm),
  });

  // biome-ignore lint/correctness/useExhaustiveDependencies: reset only when the dialog (re)opens for a given atm, not on every render
  useEffect(() => {
    if (open) form.reset(toDefaultValues(atm));
  }, [open, atm]);

  const isPending = createMutation.isPending || updateMutation.isPending;

  async function onSubmit(values: ATMFormValues): Promise<void> {
    try {
      if (mode === "create") {
        const payload: CreateATMPayload = {
          terminal_id: values.terminal_id,
          location_id: values.location_id,
          machine_type: values.machine_type,
          brand: values.brand,
          model: values.model,
          operation_hours: values.operation_hours,
          deployment_type: values.deployment_type,
          capacity_amount: values.capacity_amount || null,
          low_threshold_amount: values.low_threshold_amount || null,
          critical_threshold_amount: values.critical_threshold_amount || null,
          blacklisted: values.blacklisted,
          escrow_account: values.escrow_account || null,
          priority_class: values.priority_class || null,
        };
        const res = await createMutation.mutateAsync(payload);
        toast({ type: "success", message: pendingApprovalMessage(res) });
      } else {
        if (!atm) return;
        const payload: UpdateATMPayload = {
          location_id: values.location_id,
          machine_type: values.machine_type,
          brand: values.brand,
          model: values.model,
          operation_hours: values.operation_hours,
          deployment_type: values.deployment_type,
          capacity_amount: values.capacity_amount || null,
          low_threshold_amount: values.low_threshold_amount || null,
          critical_threshold_amount: values.critical_threshold_amount || null,
          blacklisted: values.blacklisted,
          escrow_account: values.escrow_account || null,
          priority_class: values.priority_class || null,
        };
        const res = await updateMutation.mutateAsync({ id: atm.id, payload });
        toast({ type: "success", message: pendingApprovalMessage(res) });
      }
      onClose();
    } catch (err) {
      const setError = (field: keyof ATMFormValues, error: { message: string }) =>
        form.setError(field, error);
      const handled =
        applyServerFieldError(err, (field, error) =>
          setError(field as keyof ATMFormValues, error),
        ) || applyATMConflictOrReferenceError(err, setError);
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

  const locations = locationOptionsQuery.data?.locations ?? [];

  return (
    <Dialog open={open} onClose={onClose} title={mode === "create" ? "Tambah ATM" : "Ubah ATM"}>
      <form onSubmit={form.handleSubmit(onSubmit)} className="flex flex-col gap-4">
        <Field
          id="terminal_id"
          label="Terminal ID"
          error={form.formState.errors.terminal_id?.message}
        >
          <input
            id="terminal_id"
            {...form.register("terminal_id")}
            disabled={mode === "edit"}
            className={inputClass}
          />
        </Field>

        <Field id="location_id" label="Lokasi" error={form.formState.errors.location_id?.message}>
          <select
            id="location_id"
            {...form.register("location_id", { valueAsNumber: true })}
            className={inputClass}
          >
            <option value={0}>Pilih lokasi</option>
            {locations.map((loc) => (
              <option key={loc.id} value={loc.id}>
                {loc.name}
              </option>
            ))}
          </select>
        </Field>

        <Field
          id="machine_type"
          label="Tipe Mesin"
          error={form.formState.errors.machine_type?.message}
        >
          <input id="machine_type" {...form.register("machine_type")} className={inputClass} />
        </Field>

        <Field id="brand" label="Brand" error={form.formState.errors.brand?.message}>
          <input id="brand" {...form.register("brand")} className={inputClass} />
        </Field>

        <Field id="model" label="Model" error={form.formState.errors.model?.message}>
          <input id="model" {...form.register("model")} className={inputClass} />
        </Field>

        <Field
          id="operation_hours"
          label="Jam Operasional"
          error={form.formState.errors.operation_hours?.message}
        >
          <input
            id="operation_hours"
            {...form.register("operation_hours")}
            className={inputClass}
          />
        </Field>

        <Field
          id="deployment_type"
          label="Deployment"
          error={form.formState.errors.deployment_type?.message}
        >
          <input
            id="deployment_type"
            {...form.register("deployment_type")}
            className={inputClass}
          />
        </Field>

        <Field
          id="priority_class"
          label="Prioritas"
          error={form.formState.errors.priority_class?.message}
        >
          <select id="priority_class" {...form.register("priority_class")} className={inputClass}>
            <option value="">Tidak ditentukan</option>
            {PRIORITY_CLASS_VALUES.map((value) => (
              <option key={value} value={value}>
                {value}
              </option>
            ))}
          </select>
        </Field>

        <Field
          id="capacity_amount"
          label="Kapasitas (IDR)"
          error={form.formState.errors.capacity_amount?.message}
        >
          <input
            id="capacity_amount"
            {...form.register("capacity_amount")}
            inputMode="decimal"
            placeholder="0.00"
            className={`${inputClass} text-right tabular-nums`}
          />
        </Field>

        <Field
          id="low_threshold_amount"
          label="Batas Rendah (IDR)"
          error={form.formState.errors.low_threshold_amount?.message}
        >
          <input
            id="low_threshold_amount"
            {...form.register("low_threshold_amount")}
            inputMode="decimal"
            placeholder="0.00"
            className={`${inputClass} text-right tabular-nums`}
          />
        </Field>

        <Field
          id="critical_threshold_amount"
          label="Batas Kritis (IDR)"
          error={form.formState.errors.critical_threshold_amount?.message}
        >
          <input
            id="critical_threshold_amount"
            {...form.register("critical_threshold_amount")}
            inputMode="decimal"
            placeholder="0.00"
            className={`${inputClass} text-right tabular-nums`}
          />
        </Field>

        <Field
          id="escrow_account"
          label="Nomor Rekening Escrow"
          error={form.formState.errors.escrow_account?.message}
        >
          <input id="escrow_account" {...form.register("escrow_account")} className={inputClass} />
        </Field>

        <label className="flex items-center gap-2 text-sm text-[var(--n-700)]">
          <input type="checkbox" {...form.register("blacklisted")} />
          Blacklist
        </label>

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
