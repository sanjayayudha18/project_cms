import { FilterSelect } from "@/components/ui/FilterSelect";
import { useVendorBranches, useVendorsList } from "@/features/admin-vendors/hooks";
import { CircleAlert } from "lucide-react";
import { isApiError, isTooManyExports } from "../api";
import type { StatusFilter } from "../types";

export const STATUS_OPTIONS: { value: StatusFilter; label: string }[] = [
  { value: "active", label: "Aktif" },
  { value: "disabled", label: "Nonaktif" },
  { value: "all", label: "Semua" },
];

const LIST_PARAMS = { page: 1, page_size: 100 } as const;

interface PickerProps {
  value: number | null;
  onChange: (id: number | null) => void;
}

const toValue = (id: number | null) => (id === null ? null : String(id));
const fromValue = (v: string | null) => (v === null ? null : Number(v));

/** Vendor picker: one dropdown of active vendors (first 100, endpoint cap). */
export function VendorSelect({ value, onChange }: PickerProps) {
  const { data } = useVendorsList({ ...LIST_PARAMS, status: "active" });
  const options = (data?.vendors ?? []).map((v) => ({
    value: String(v.id),
    label: `${v.name} (${v.code})`,
  }));
  return (
    <FilterSelect
      label="Vendor"
      placeholder="Pilih vendor"
      options={options}
      value={toValue(value)}
      onChange={(v) => onChange(fromValue(v))}
    />
  );
}

/** Only that vendor's cabang. Mount it only once a vendor is chosen. */
export function BranchSelect({
  vendorId,
  value,
  onChange,
  placeholder,
}: PickerProps & { vendorId: number; placeholder: string }) {
  const { data } = useVendorBranches(vendorId, { ...LIST_PARAMS, status: "active" });
  const options = (data?.branches ?? []).map((b) => ({
    value: String(b.id),
    label: `${b.branch_name} (${b.branch_code})`,
  }));
  return (
    <FilterSelect
      label="Cabang"
      placeholder={placeholder}
      options={options}
      value={toValue(value)}
      onChange={(v) => onChange(fromValue(v))}
    />
  );
}

/** Inline request error (icon + text, never colour alone); 429 gets its own wording. */
export function RequestError({ error }: { error: unknown }) {
  if (!error) return null;
  const text = isTooManyExports(error)
    ? "Terlalu banyak ekspor yang sedang berjalan. Tunggu sampai salah satu selesai, lalu coba lagi."
    : isApiError(error) || error instanceof Error
      ? error.message
      : "Permintaan ekspor gagal.";
  return (
    <p role="alert" className="flex items-center gap-2 text-sm text-[var(--danger-fg)]">
      <CircleAlert className="h-4 w-4 shrink-0" aria-hidden="true" />
      {text}
    </p>
  );
}
