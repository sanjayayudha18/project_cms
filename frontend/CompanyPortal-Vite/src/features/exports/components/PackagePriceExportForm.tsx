import { Button } from "@/components/ui/Button";
import { FilterSelect } from "@/components/ui/FilterSelect";
import { useState } from "react";
import type { ExportRequestParams, PriceLevel, StatusFilter } from "../types";
import { BranchSelect, RequestError, STATUS_OPTIONS, VendorSelect } from "./exportFormParts";

const PRICE_LEVELS: { value: PriceLevel; label: string }[] = [
  { value: "all", label: "Semua" },
  { value: "vendor", label: "Vendor" },
  { value: "branch", label: "Cabang" },
  { value: "atm", label: "ATM" },
];

interface Props {
  onRequest: (p: ExportRequestParams) => void;
  isRequesting: boolean;
  error: unknown;
}

export function PackagePriceExportForm({ onRequest, isRequesting, error }: Props) {
  const [vendorId, setVendorId] = useState<number | null>(null);
  const [branchId, setBranchId] = useState<number | null>(null);
  const [priceLevel, setPriceLevel] = useState<PriceLevel>("all");
  const [status, setStatus] = useState<StatusFilter>("active");

  return (
    <form
      className="flex flex-col gap-4"
      onSubmit={(e) => {
        e.preventDefault();
        if (vendorId === null) return;
        onRequest({
          kind: "harga_paket",
          vendorId,
          branchId: branchId ?? undefined,
          priceLevel,
          status,
        });
      }}
    >
      <VendorSelect
        value={vendorId}
        onChange={(id) => {
          setVendorId(id);
          setBranchId(null);
        }}
      />
      {vendorId !== null && (
        <BranchSelect
          vendorId={vendorId}
          value={branchId}
          onChange={setBranchId}
          placeholder="Semua cabang"
        />
      )}
      <FilterSelect
        label="Tingkat harga"
        placeholder="Pilih tingkat harga"
        options={PRICE_LEVELS}
        value={priceLevel}
        onChange={(v) => v && setPriceLevel(v as PriceLevel)}
      />
      <FilterSelect
        label="Status"
        placeholder="Pilih status"
        options={STATUS_OPTIONS}
        value={status}
        onChange={(v) => v && setStatus(v as StatusFilter)}
      />
      <RequestError error={error} />
      <div>
        <Button type="submit" disabled={vendorId === null || isRequesting}>
          Minta ekspor
        </Button>
      </div>
    </form>
  );
}
