import { useQuery } from "@tanstack/react-query";
import { listVendorPackages } from "../../admin-vendors/api";
import { type ATMAssignmentSource, listATMPackageOptions } from "../api";

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
 * comes from differs: the branch's own packages, or the vendor-wide package
 * labels that have a tariff matching this ATM.
 */
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
          value={value}
          disabled={vendorId === "" || needsBranch}
          onChange={(e) => onValueChange(e.target.value)}
        >
          <option value="">Pilih paket</option>
          {source === "branch"
            ? (branchPackages.data?.packages ?? []).map((p) => (
                <option key={p.id} value={String(p.id)}>
                  {p.package_code} · {p.machine_group} · {p.price_class}
                  {p.atm_id !== null ? ` · ATM #${p.atm_id}` : ""}
                </option>
              ))
            : (vendorPackages.data ?? []).map((label) => (
                <option key={label} value={label}>
                  {label}
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
