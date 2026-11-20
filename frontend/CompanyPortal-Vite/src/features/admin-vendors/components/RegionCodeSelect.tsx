import type { UseFormRegisterReturn } from "react-hook-form";
import { useRegionsList } from "../../admin-regions/hooks";

// ponytail: one page of 100 active regions; the master list is ~13 rows. Page through it if it ever grows past 100.
const ACTIVE_REGIONS = { page: 1, page_size: 100, status: "active" } as const;

interface RegionCodeSelectProps {
  registration: UseFormRegisterReturn<"region_code">;
  /** Branch's stored code, kept as an option even if the region is no longer active. */
  currentCode?: string;
  className: string;
}

/**
 * Region select for the vendor branch form (region-standardization Req 3.3):
 * only active standard regions can be picked; blank = no region (NULL).
 */
export function RegionCodeSelect({ registration, currentCode, className }: RegionCodeSelectProps) {
  const query = useRegionsList(ACTIVE_REGIONS);
  const regions = query.data?.regions ?? [];
  const hasCurrent = !!currentCode && !regions.some((r) => r.code === currentCode);

  if (query.isError) {
    return (
      <p role="alert" className="text-sm text-[var(--danger-fg)]">
        Daftar region gagal dimuat
      </p>
    );
  }

  // Mount the registered select only once the options exist, so react-hook-form
  // applies the stored value to a matching <option> instead of falling back to "".
  if (query.isLoading) {
    return (
      <select disabled aria-label="Kode Region" className={className}>
        <option>Memuat region…</option>
      </select>
    );
  }

  return (
    <select {...registration} className={className}>
      <option value="">— Tanpa region —</option>
      {hasCurrent && <option value={currentCode}>{currentCode} (nonaktif)</option>}
      {regions.map((r) => (
        <option key={r.id} value={r.code}>
          {r.region ? `${r.code} — ${r.region}` : r.code}
        </option>
      ))}
    </select>
  );
}
