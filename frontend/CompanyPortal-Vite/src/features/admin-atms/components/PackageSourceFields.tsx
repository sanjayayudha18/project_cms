import { useQuery } from "@tanstack/react-query";
import { useState } from "react";
import { listVendorPackages } from "../../admin-vendors/api";
import { formatIDR } from "../../admin-vendors/lib/packagePrice";
import { type ATMAssignmentSource, type ATMPackageOption, listATMPackageOptions } from "../api";

const inputClass =
  "min-h-[44px] rounded-[var(--radius-md)] border border-[var(--n-300)] bg-[var(--n-0)] px-3 text-sm text-[var(--n-800)]";

const SOURCE_OPTIONS: { value: ATMAssignmentSource; label: string; hint: string }[] = [
  { value: "branch", label: "Paket khusus cabang", hint: "Harga khusus milik cabang terpilih" },
  {
    value: "vendor",
    label: "Paket seluruh vendor",
    hint: "Paket kontrak yang berlaku untuk vendor",
  },
];

interface Props {
  atmId: number;
  vendorId: string;
  branchId: string;
  source: ATMAssignmentSource;
  /** Selected vendor_packages_branch id ("branch") or vendor-wide label ("vendor"). */
  value: string;
  onSourceChange: (source: ATMAssignmentSource) => void;
  onValueChange: (value: string) => void;
}

/**
 * "Jenis paket" choice + the Paket dropdown it drives. The cabang stays chosen
 * outside (it is the managing branch in both modes); only where the package list
 * comes from differs: the branch's own packages, or the vendor-wide tariff rows
 * matching this ATM. Vendor-wide shows one option per tariff (code · tier ·
 * price) but reports only its label upward -- the assignment stores the label.
 */
function vendorOptionLabel(o: ATMPackageOption): string {
  const tier = o.tier_max === null ? `${o.tier_min}+` : `${o.tier_min}-${o.tier_max}`;
  const price = o.base_price === "" ? "—" : formatIDR(o.base_price);
  return `${o.package_code} · ${o.package} · tingkat ${tier} · ${price}`;
}

export function PackageSourceFields({
  atmId,
  vendorId,
  branchId,
  source,
  value,
  onSourceChange,
  onValueChange,
}: Props) {
  // Only currently-effective branch packages (vendor_packages_branch has no
  // is_active since migration 016 -- "active" = effective_end_date open/future).
  const branchPackages = useQuery({
    queryKey: ["admin-vendors", "packages-options", Number(vendorId), branchId],
    queryFn: () =>
      listVendorPackages(Number(vendorId), {
        page: 1,
        page_size: 100,
        status: "active",
        branch_id: Number(branchId),
      }),
    enabled: source === "branch" && vendorId !== "" && branchId !== "",
  });
  const vendorPackages = useQuery({
    queryKey: ["admin-atms", "package-options", atmId, Number(vendorId)],
    queryFn: () => listATMPackageOptions(atmId, Number(vendorId)),
    enabled: source === "vendor" && vendorId !== "",
  });

  // Several tariff rows share one label, so the select tracks the chosen code;
  // the parent clearing `value` clears the selection.
  const [vendorCode, setVendorCode] = useState("");
  const selectValue = source === "vendor" ? (value === "" ? "" : vendorCode) : value;

  const needsBranch = source === "branch" && branchId === "";
  const loadError = source === "branch" ? branchPackages.isError : vendorPackages.isError;
  const emptyVendorList =
    source === "vendor" && vendorPackages.data !== undefined && vendorPackages.data.length === 0;

  return (
    <>
      <fieldset className="flex flex-col gap-2">
        <legend className="text-xs font-medium text-[var(--n-600)]">Jenis paket</legend>
        <div className="flex flex-wrap gap-2">
          {SOURCE_OPTIONS.map((o) => (
            <label
              key={o.value}
              className={`flex min-h-[44px] flex-1 cursor-pointer flex-col justify-center rounded-[var(--radius-md)] border px-3 py-1 text-sm ${
                source === o.value
                  ? "border-[var(--red-600)] bg-[var(--red-50)] text-[var(--n-800)]"
                  : "border-[var(--n-300)] bg-[var(--n-0)] text-[var(--n-700)]"
              }`}
            >
              <span className="flex items-center gap-2 font-medium">
                <input
                  type="radio"
                  name="package-source"
                  value={o.value}
                  checked={source === o.value}
                  onChange={() => onSourceChange(o.value)}
                />
                {o.label}
              </span>
              <span className="pl-6 text-xs text-[var(--n-500)]">{o.hint}</span>
            </label>
          ))}
        </div>
      </fieldset>
      <label className="flex flex-col gap-1 text-xs font-medium text-[var(--n-600)]">
        Paket
        <select
          className={inputClass}
          value={selectValue}
          disabled={vendorId === "" || needsBranch}
          onChange={(e) => {
            if (source === "branch") return onValueChange(e.target.value);
            const code = e.target.value;
            setVendorCode(code);
            onValueChange(vendorPackages.data?.find((o) => o.package_code === code)?.package ?? "");
          }}
        >
          <option value="">Pilih paket</option>
          {source === "branch"
            ? (branchPackages.data?.packages ?? []).map((p) => (
                <option key={p.id} value={String(p.id)}>
                  {p.package_code} · {p.machine_group} · {p.price_class}
                  {p.atm_id !== null ? ` · ATM #${p.atm_id}` : ""}
                </option>
              ))
            : (vendorPackages.data ?? []).map((o) => (
                <option key={o.package_code} value={o.package_code}>
                  {vendorOptionLabel(o)}
                </option>
              ))}
        </select>
      </label>
      {needsBranch && (
        <p className="text-xs text-[var(--n-500)]">Pilih cabang untuk melihat paket khususnya.</p>
      )}
      {emptyVendorList && (
        <p className="text-xs text-[var(--n-500)]">
          Vendor ini belum punya tarif aktif untuk jenis mesin ATM ini.
        </p>
      )}
      {loadError && (
        <p role="alert" className="text-sm text-[var(--danger-fg)]">
          Gagal memuat daftar paket
        </p>
      )}
    </>
  );
}
