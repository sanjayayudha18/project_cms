import { Button } from "@/components/ui/Button";
import { Dialog } from "@/components/ui/Dialog";
import type { ApiError } from "@/lib/api/client";
import { useToast } from "@/lib/hooks/useToast";
import { applyServerFieldError } from "@/lib/utils/applyServerFieldError";
import { zodResolver } from "@hookform/resolvers/zod";
import { useEffect, useState } from "react";
import { useForm } from "react-hook-form";
import { z } from "zod";
import type { PksType } from "../types";

const CODE_REGEX = /^[A-Z0-9]{2,10}$/;

const pksTypeSchema = z.object({
  code: z
    .string()
    .trim()
    .min(1, "Kode wajib diisi")
    .regex(
      CODE_REGEX,
      "Kode harus terdiri dari 2 hingga 10 karakter alfanumerik huruf besar (A-Z, 0-9)",
    ),
  name: z.string().trim().min(1, "Nama wajib diisi").max(100, "Nama maksimal 100 karakter"),
});

type FormValues = z.infer<typeof pksTypeSchema>;

interface PksTypeFormDialogProps {
  open: boolean;
  editing: PksType | null;
  isPending: boolean;
  onClose: () => void;
  onSubmit: (values: { code: string; name: string }) => Promise<void>;
}

const inputClass =
  "min-h-[44px] w-full rounded-[var(--radius-md)] border border-[var(--n-300)] bg-[var(--n-0)] px-3 text-sm text-[var(--n-800)] outline-none focus-visible:border-[var(--red-400)] focus-visible:ring-2 focus-visible:ring-[var(--red-100)] disabled:bg-[var(--n-100)] disabled:text-[var(--n-500)]";

export function PksTypeFormDialog({
  open,
  editing,
  isPending,
  onClose,
  onSubmit,
}: PksTypeFormDialogProps) {
  const { toast } = useToast();
  const [formError, setFormError] = useState<string | null>(null);

  const form = useForm<FormValues>({
    resolver: zodResolver(pksTypeSchema),
    defaultValues: {
      code: "",
      name: "",
    },
  });

  useEffect(() => {
    if (open) {
      setFormError(null);
      form.reset({
        code: editing ? editing.code : "",
        name: editing ? editing.name : "",
      });
    }
  }, [open, editing, form]);

  async function handleSubmitForm(values: FormValues) {
    setFormError(null);
    try {
      await onSubmit({
        code: values.code.trim().toUpperCase(),
        name: values.name.trim(),
      });
      onClose();
    } catch (err: unknown) {
      const apiErr = err as Partial<ApiError> | undefined;

      // Handle 409 conflict on code (sentinel error from backend)
      if (
        apiErr?.status === 409 &&
        typeof apiErr.message === "string" &&
        (apiErr.message.toLowerCase().includes("kode") ||
          apiErr.message.toLowerCase().includes("conflict") ||
          apiErr.message.toLowerCase().includes("sudah dipakai"))
      ) {
        form.setError("code", { message: apiErr.message });
        return;
      }

      // Handle 422 or other structured field errors
      const applied = applyServerFieldError(err, (field, error) => {
        form.setError(field as keyof FormValues, error);
      });

      if (!applied) {
        const msg =
          typeof apiErr?.message === "string"
            ? apiErr.message
            : "Terjadi kesalahan saat menyimpan type PKS";
        setFormError(msg);
        toast({ type: "error", message: msg });
      }
    }
  }

  return (
    <Dialog
      open={open}
      onClose={isPending ? () => {} : onClose}
      title={editing ? "Ubah Type PKS" : "Tambah Type PKS"}
    >
      <form
        onSubmit={form.handleSubmit(handleSubmitForm)}
        className="flex flex-col gap-4"
        noValidate
      >
        {formError && (
          <div
            className="rounded-[var(--radius-md)] border border-[var(--danger-border)] bg-[var(--danger-bg)] p-3 text-xs text-[var(--danger-fg)]"
            role="alert"
          >
            {formError}
          </div>
        )}

        <div className="flex flex-col gap-1.5">
          <label htmlFor="pks-type-code" className="text-sm font-medium text-[var(--n-700)]">
            Kode <span className="text-[var(--danger-fg)]">*</span>
          </label>
          <input
            id="pks-type-code"
            type="text"
            disabled={editing !== null || isPending}
            maxLength={10}
            placeholder="Contoh: FLM, CIT"
            className={inputClass}
            aria-describedby={editing !== null ? "pks-type-code-hint" : undefined}
            aria-invalid={form.formState.errors.code ? "true" : "false"}
            {...form.register("code", {
              onChange: (e) => {
                form.setValue("code", e.target.value.toUpperCase(), {
                  shouldValidate: true,
                });
              },
            })}
          />
          {editing !== null && (
            <span id="pks-type-code-hint" className="text-xs text-[var(--n-500)]">
              Kode tidak dapat diubah setelah dibuat.
            </span>
          )}
          {form.formState.errors.code && (
            <span role="alert" className="text-xs text-[var(--danger-fg)] font-medium">
              {form.formState.errors.code.message}
            </span>
          )}
        </div>

        <div className="flex flex-col gap-1.5">
          <label htmlFor="pks-type-name" className="text-sm font-medium text-[var(--n-700)]">
            Nama <span className="text-[var(--danger-fg)]">*</span>
          </label>
          <input
            id="pks-type-name"
            type="text"
            disabled={isPending}
            maxLength={100}
            placeholder="Contoh: First Line Maintenance"
            className={inputClass}
            aria-invalid={form.formState.errors.name ? "true" : "false"}
            {...form.register("name")}
          />
          {form.formState.errors.name && (
            <span role="alert" className="text-xs text-[var(--danger-fg)] font-medium">
              {form.formState.errors.name.message}
            </span>
          )}
        </div>

        <div className="mt-4 flex justify-end gap-3">
          <Button type="button" variant="secondary" disabled={isPending} onClick={onClose}>
            Batal
          </Button>
          <Button type="submit" disabled={isPending}>
            {isPending ? "Menyimpan..." : "Simpan"}
          </Button>
        </div>
      </form>
    </Dialog>
  );
}
