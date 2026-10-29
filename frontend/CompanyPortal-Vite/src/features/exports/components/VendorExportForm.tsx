import { Button } from "@/components/ui/Button";
import { FilterSelect } from "@/components/ui/FilterSelect";
import { useState } from "react";
import type { BranchEntity, ExportRequestParams, StatusFilter, VendorEntity } from "../types";
import { BranchSelect, RequestError, STATUS_OPTIONS, VendorSelect } from "./exportFormParts";

type Level = "vendor" | "cabang";

const VENDOR_ENTITIES: { value: VendorEntity; label: string }[] = [
  { value: "vendor", label: "Profil vendor" },
  { value: "branches", label: "Cabang" },
  { value: "vaults", label: "Vault" },
  { value: "pics", label: "PIC" },
  { value: "dsr-location-maps", label: "Peta lokasi DSR" },
];
const BRANCH_ENTITIES = VENDOR_ENTITIES.filter(
  (e) => e.value !== "vendor" && e.value !== "branches",
);

interface Props {
  onRequest: (p: ExportRequestParams) => void;
  isRequesting: boolean;
  error: unknown;
}

export function VendorExportForm({ onRequest, isRequesting, error }: Props) {
  const [level, setLevel] = useState<Level>("vendor");
  const [vendorId, setVendorId] = useState<number | null>(null);
  const [branchId, setBranchId] = useState<number | null>(null);
  const [entity, setEntity] = useState<VendorEntity>("vendor");
  const [status, setStatus] = useState<StatusFilter>("active");

  const entities = level === "vendor" ? VENDOR_ENTITIES : BRANCH_ENTITIES;
  const invalid = vendorId === null || (level === "cabang" && branchId === null);

  const changeLevel = (l: Level) => {
    setLevel(l);
    if (l === "cabang" && (entity === "vendor" || entity === "branches")) setEntity("vaults");
    if (l === "vendor") setBranchId(null);
  };

  const submit = () => {
    if (vendorId === null) return;
    if (level === "vendor") {
      onRequest({ kind: "vendor", vendorId, entity, status });
    } else if (branchId !== null) {
      onRequest({ kind: "cabang", vendorId, branchId, entity: entity as BranchEntity, status });
    }
  };

  return (
    <form
      className="flex flex-col gap-4"
      onSubmit={(e) => {
        e.preventDefault();
        submit();
      }}
    >
      <fieldset className="flex flex-wrap gap-4">
        <legend className="mb-1 text-xs font-medium text-[var(--n-600)] uppercase tracking-wider">
          Level
        </legend>
        {(["vendor", "cabang"] as const).map((l) => (
          <label
            key={l}
            className="flex min-h-[44px] items-center gap-2 text-sm text-[var(--n-800)]"
          >
            <input
              type="radio"
              name="export-level"
              checked={level === l}
              onChange={() => changeLevel(l)}
            />
            {l === "vendor" ? "Vendor" : "Cabang"}
          </label>
        ))}
      </fieldset>
      <VendorSelect
        value={vendorId}
        onChange={(id) => {
          setVendorId(id);
          setBranchId(null);
        }}
      />
      {level === "cabang" && vendorId !== null && (
        <BranchSelect
          vendorId={vendorId}
          value={branchId}
          onChange={setBranchId}
          placeholder="Pilih cabang"
        />
      )}
      <FilterSelect
        label="Data yang diekspor"
        placeholder="Pilih data"
        options={entities}
        value={entity}
        onChange={(v) => v && setEntity(v as VendorEntity)}
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
        <Button type="submit" disabled={invalid || isRequesting}>
          Minta ekspor
        </Button>
      </div>
    </form>
  );
}
