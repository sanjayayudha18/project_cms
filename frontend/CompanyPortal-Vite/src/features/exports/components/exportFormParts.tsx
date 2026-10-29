import { FilterSelect } from "@/components/ui/FilterSelect";
import { useVendorBranches, useVendorsList } from "@/features/admin-vendors/hooks";
import { CircleAlert } from "lucide-react";
import { useId, useState } from "react";
import { isApiError, isTooManyExports } from "../api";
import type { StatusFilter } from "../types";

export const STATUS_OPTIONS: { value: StatusFilter; label: string }[] = [
  { value: "active", label: "Aktif" },
  { value: "disabled", label: "Nonaktif" },
  { value: "all", label: "Semua" },
];

const INPUT_CLASS =
  "min-h-[44px] rounded-[var(--radius-md)] border border-[var(--n-300)] bg-[var(--n-0)] px-3 text-sm text-[var(--n-800)] outline-none focus-visible:ring-2 focus-visible:ring-[var(--red-100)] focus-visible:border-[var(--red-400)]";

const LIST_PARAMS = { page: 1, page_size: 100 } as const;

interface PickerProps {
  value: number | null;
  onChange: (id: number | null) => void;
}

const toValue = (id: number | null) => (id === null ? null : String(id));
const fromValue = (v: string | null) => (v === null ? null : Number(v));

/** Vendor picker with a search box (server-side `q`, first 100 matches). */
export function VendorSelect({ value, onChange }: PickerProps) {
  const searchId = useId();
  const [q, setQ] = useState("");
  const { data } = useVendorsList({ ...LIST_PARAMS, q: q || undefined, status: "active" });
  const options = (data?.vendors ?? []).map((v) => ({
    value: String(v.id),
    label: `${v.name} (${v.code})`,
  }));
  return (
    <div className="flex flex-col gap-2">
      <div className="flex flex-col gap-1">
        <label
          htmlFor={searchId}
          className="text-xs font-medium text-[var(--n-600)] uppercase tracking-wider"
        >
          Cari vendor
        </label>
        <input
          id={searchId}
          type="search"
          value={q}
          onChange={(e) => setQ(e.target.value)}
          className={INPUT_CLASS}
        />
      </div>
      <FilterSelect
        label="Vendor"
        placeholder="Pilih vendor"
        options={options}
        value={toValue(value)}
        onChange={(v) => onChange(fromValue(v))}
      />
    </div>
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
