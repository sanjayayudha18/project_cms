import { Button } from "@/components/ui/Button";
import { FilterSelect } from "@/components/ui/FilterSelect";
import { Info } from "lucide-react";
import { useEffect, useState } from "react";
import { type MasterDataset, type PreviewFilter, toSearch } from "../datasets";
import type { PriceLevel } from "../types";
import { BranchSelect, STATUS_OPTIONS, VendorSelect } from "./exportFormParts";

const PRICE_LEVEL_OPTIONS: { value: PriceLevel; label: string }[] = [
  { value: "vendor", label: "Vendor" },
  { value: "branch", label: "Cabang" },
  { value: "atm", label: "ATM" },
];

const STATUS_FILTER_OPTIONS = STATUS_OPTIONS.filter((o) => o.value !== "all");

/** True when both filters serialise to the same URL search. */
export const isSameFilter = (a: PreviewFilter, b: PreviewFilter): boolean =>
  JSON.stringify(toSearch(a)) === JSON.stringify(toSearch(b));

interface PreviewFilterBarProps {
  dataset: MasterDataset;
  /** The Applied_Filter (from the URL). The draft restarts from it whenever it changes. */
  applied: PreviewFilter;
  /** "Terapkan": the page writes the draft to the URL search. */
  onApply: (draft: PreviewFilter) => void;
  /** Fires on every draft edit so the page can compute `dirty`. */
  onDraftChange?: (draft: PreviewFilter) => void;
}

/** Draft filter inputs for one dataset; nothing reaches the table until "Terapkan". */
export function PreviewFilterBar({
  dataset,
  applied,
  onApply,
  onDraftChange,
}: PreviewFilterBarProps) {
  const [draft, setDraft] = useState<PreviewFilter>(applied);
  const appliedKey = JSON.stringify(toSearch(applied));

  // biome-ignore lint/correctness/useExhaustiveDependencies: restart only when the applied filter's content changes, not its identity.
  useEffect(() => {
    setDraft(applied);
    onDraftChange?.(applied);
  }, [appliedKey]);

  const { filters } = dataset;
  const update = (next: PreviewFilter) => {
    setDraft(next);
    onDraftChange?.(next);
  };
  const setVendor = (id: number | null) => {
    const { vendorId: _v, branchId: _b, ...rest } = draft;
    update(id === null ? rest : { ...rest, vendorId: id });
  };
  const setBranch = (id: number | null) => {
    const { branchId: _b, ...rest } = draft;
    update(id === null ? rest : { ...rest, branchId: id });
  };

  const missingVendor = filters.vendor === "required" && draft.vendorId === undefined;

  return (
    <form
      aria-label={`Filter ${dataset.label}`}
      className="flex flex-col gap-3"
      onSubmit={(e) => {
        e.preventDefault();
        if (!missingVendor) onApply(draft);
      }}
    >
      <div className="flex flex-wrap items-end gap-4">
        {filters.vendor !== "none" && (
          <VendorSelect value={draft.vendorId ?? null} onChange={setVendor} />
        )}
        {filters.branch && draft.vendorId !== undefined && (
          <BranchSelect
            vendorId={draft.vendorId}
            value={draft.branchId ?? null}
            onChange={setBranch}
            placeholder="Semua cabang"
          />
        )}
        <FilterSelect
          label="Status"
          placeholder="Semua"
          options={STATUS_FILTER_OPTIONS}
          value={draft.status === "all" ? null : draft.status}
          onChange={(v) =>
            update({ ...draft, status: v === "active" || v === "disabled" ? v : "all" })
          }
        />
        {filters.priceLevel && (
          <FilterSelect
            label="Tingkat harga"
            placeholder="Semua tingkat"
            options={PRICE_LEVEL_OPTIONS}
            value={draft.priceLevel && draft.priceLevel !== "all" ? draft.priceLevel : null}
            onChange={(v) => update({ ...draft, priceLevel: (v as PriceLevel | null) ?? "all" })}
          />
        )}
        <Button type="submit" variant="secondary" disabled={missingVendor}>
          Terapkan
        </Button>
      </div>
      {missingVendor && (
        <output className="flex items-center gap-2 text-sm text-[var(--n-600)]">
          <Info className="h-4 w-4 shrink-0" aria-hidden="true" />
          Pilih vendor dulu untuk melihat dan mengekspor {dataset.label.toLowerCase()}.
        </output>
      )}
    </form>
  );
}
