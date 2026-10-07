import { FilterSelect } from "@/components/ui/FilterSelect";
import { useVendorsList } from "@/features/admin-vendors/hooks";
import { ROLE_OPTIONS } from "../lib/userFormSchema";
import type { AdminUsersUrlParams } from "../useAdminUsersUrlState";

const STATUS_OPTIONS = [
  { value: "active", label: "Aktif" },
  { value: "disabled", label: "Nonaktif" },
  { value: "all", label: "Semua" },
];

interface UserFilterBarProps {
  params: AdminUsersUrlParams;
  searchInput: string;
  onSearchInputChange: (value: string) => void;
  onParamsChange: (partial: Partial<AdminUsersUrlParams>) => void;
}

/** Search + Role/Vendor/Status filters for the admin Users list (Req 2, 13.3-13.4). */
export function UserFilterBar({
  params,
  searchInput,
  onSearchInputChange,
  onParamsChange,
}: UserFilterBarProps) {
  const vendorsQuery = useVendorsList({ page: 1, page_size: 100, status: "all" });
  const vendorOptions = (vendorsQuery.data?.vendors ?? []).map((v) => ({
    value: String(v.id),
    label: v.name,
  }));

  return (
    <div className="flex flex-wrap items-end gap-4">
      <div className="flex flex-col gap-1">
        <label
          htmlFor="admin-users-search"
          className="text-xs font-medium uppercase tracking-wider text-[var(--n-600)]"
        >
          Cari
        </label>
        <input
          id="admin-users-search"
          type="text"
          value={searchInput}
          onChange={(e) => onSearchInputChange(e.target.value.slice(0, 100))}
          maxLength={100}
          placeholder="Nama, username, atau email"
          className="min-h-[44px] rounded-[var(--radius-md)] border border-[var(--n-300)] bg-[var(--n-0)] px-3 text-sm text-[var(--n-800)] outline-none focus-visible:border-[var(--red-400)] focus-visible:ring-2 focus-visible:ring-[var(--red-100)]"
        />
      </div>

      <FilterSelect
        label="Role"
        options={ROLE_OPTIONS.map((r) => ({ value: r, label: r }))}
        value={params.role ?? null}
        onChange={(value) => onParamsChange({ role: value ?? undefined, page: 1 })}
      />

      <FilterSelect
        label="Vendor"
        options={vendorOptions}
        value={params.vendor_id ? String(params.vendor_id) : null}
        onChange={(value) =>
          onParamsChange({ vendor_id: value ? Number(value) : undefined, page: 1 })
        }
      />

      <FilterSelect
        label="Status"
        options={STATUS_OPTIONS}
        value={params.status}
        onChange={(value) =>
          onParamsChange({
            status: (value as AdminUsersUrlParams["status"] | null) ?? "active",
            page: 1,
          })
        }
        placeholder="Aktif"
      />
    </div>
  );
}
