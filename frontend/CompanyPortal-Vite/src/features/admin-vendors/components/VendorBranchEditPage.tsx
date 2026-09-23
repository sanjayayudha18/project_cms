import { Button } from "@/components/ui/Button";
import { PageHeader } from "@/components/ui/PageHeader";
import { useToast } from "@/lib/hooks/useToast";
import { applyServerFieldError } from "@/lib/utils/applyServerFieldError";
import { zodResolver } from "@hookform/resolvers/zod";
import { Link, useNavigate } from "@tanstack/react-router";
import { useEffect } from "react";
import { useForm } from "react-hook-form";
import { pendingApprovalMessage } from "../../master-data/changeRequest";
import { useUpdateVendorBranch, useVendorBranch } from "../hooks";
import { type VendorBranchFormValues, vendorBranchFormSchema } from "../lib/vendorBranchFormSchema";
import { VENDOR_VAULT_CATEGORIES } from "../lib/vendorVaultFormSchema";
import type { AdminVendorBranch, UpdateVendorBranchPayload } from "../types";

function toDefaultValues(branch: AdminVendorBranch): VendorBranchFormValues {
  return {
    branch_code: branch.branch_code,
    branch_name: branch.branch_name,
    location_id: branch.location_id ? String(branch.location_id) : "",
    region: branch.region ?? "",
    category: branch.category,
  };
}

interface VendorBranchEditPageProps {
  vendorId: number;
  branchId: number;
}

/**
 * Standalone page for editing a vendor branch (moved off the vendor detail
 * page per request: create still opens a dialog there, but edit is its own
 * route so it has room to grow and can be linked/refreshed directly).
 */
export function VendorBranchEditPage({ vendorId, branchId }: VendorBranchEditPageProps) {
  const { toast } = useToast();
  const navigate = useNavigate();
  const branchQuery = useVendorBranch(vendorId, branchId);
  const branch = branchQuery.data;
  const updateMutation = useUpdateVendorBranch(vendorId);

  const form = useForm<VendorBranchFormValues>({
    resolver: zodResolver(vendorBranchFormSchema),
    defaultValues: { branch_code: "", branch_name: "", location_id: "", region: "", category: "ATM" },
  });

  // biome-ignore lint/correctness/useExhaustiveDependencies: reset once the branch finishes loading, not on every render
  useEffect(() => {
    if (branch) form.reset(toDefaultValues(branch));
  }, [branch]);

  function backToVendor(): void {
    navigate({ to: "/settings/admin/vendors/$vendorId", params: { vendorId: String(vendorId) } });
  }

  async function onSubmit(values: VendorBranchFormValues): Promise<void> {
    if (!branch) return;
    try {
      const payload: UpdateVendorBranchPayload = {
        branch_name: values.branch_name,
        location_id: values.location_id ? Number(values.location_id) : null,
        region: values.region || null,
        category: values.category,
      };
      const res = await updateMutation.mutateAsync({ branchId: branch.id, payload });
      toast({ type: "success", message: pendingApprovalMessage(res) });
      backToVendor();
    } catch (err) {
      const handled = applyServerFieldError(err, (field, error) =>
        form.setError(field as keyof VendorBranchFormValues, error),
      );
      if (!handled) {
        const message =
          err && typeof err === "object" && "message" in err
            ? String((err as { message: unknown }).message)
            : "Terjadi kesalahan yang tidak diketahui";
        toast({ type: "error", message });
      }
    }
  }

  return (
    <div className="flex flex-col gap-6 p-6">
      <PageHeader
        eyebrow="Manajemen Vendor"
        title={branch ? `Ubah Cabang · ${branch.branch_code}` : "Ubah Cabang"}
        description="Perubahan diajukan untuk persetujuan (maker-checker), belum langsung berlaku."
      />
      <Link to="/settings/admin/vendors/$vendorId" params={{ vendorId: String(vendorId) }}>
        <Button variant="ghost">← Kembali ke detail vendor</Button>
      </Link>

      {branchQuery.isLoading && <p className="text-sm text-[var(--n-500)]">Memuat…</p>}
      {branchQuery.isError && (
        <p role="alert" className="text-sm text-[var(--danger-fg)]">
          Cabang tidak ditemukan
        </p>
      )}

      {branch && (
        <form
          onSubmit={form.handleSubmit(onSubmit)}
          className="flex max-w-lg flex-col gap-4 rounded-lg border border-[var(--n-200)] p-6"
        >
          <Field label="Kode Cabang" error={form.formState.errors.branch_code?.message}>
            <input {...form.register("branch_code")} disabled className={inputClass} />
          </Field>

          <Field label="Nama Cabang" error={form.formState.errors.branch_name?.message}>
            <input {...form.register("branch_name")} className={inputClass} />
          </Field>

          <Field label="Wilayah" error={form.formState.errors.region?.message}>
            <input {...form.register("region")} className={inputClass} />
          </Field>

          <Field label="Tipe" error={form.formState.errors.category?.message}>
            <select {...form.register("category")} className={inputClass}>
              {VENDOR_VAULT_CATEGORIES.map((value) => (
                <option key={value} value={value}>
                  {value}
                </option>
              ))}
            </select>
          </Field>

          <Field label="ID Lokasi (opsional)" error={form.formState.errors.location_id?.message}>
            <input {...form.register("location_id")} inputMode="numeric" className={inputClass} />
          </Field>

          <div className="flex justify-end gap-3 pt-2">
            <Button
              type="button"
              variant="secondary"
              disabled={updateMutation.isPending}
              onClick={backToVendor}
            >
              Batal
            </Button>
            <Button type="submit" disabled={updateMutation.isPending}>
              Simpan
            </Button>
          </div>
        </form>
      )}
    </div>
  );
}

const inputClass =
  "min-h-[44px] rounded-[var(--radius-md)] border border-[var(--n-300)] bg-[var(--n-0)] px-3 text-sm text-[var(--n-800)] outline-none focus-visible:border-[var(--red-400)] focus-visible:ring-2 focus-visible:ring-[var(--red-100)] disabled:bg-[var(--n-100)] disabled:text-[var(--n-500)]";

function Field({
  label,
  error,
  children,
}: {
  label: string;
  error?: string;
  children: React.ReactNode;
}) {
  return (
    <div className="flex flex-col gap-1">
      <span className="text-xs font-medium uppercase tracking-wider text-[var(--n-600)]">
        {label}
      </span>
      {children}
      {error && <span className="text-xs text-[var(--danger-fg)]">{error}</span>}
    </div>
  );
}
