import { Link } from "@tanstack/react-router";
import { TriangleAlert } from "lucide-react";
import type { DsrLateWarnings } from "./api";

/** FR3.4 / FR6.5: mapping gaps that make the report look cleaner than reality. */
export function WarningsPanel({ data, isAdmin }: { data: DsrLateWarnings; isAdmin: boolean }) {
  const { unmapped_vaults: vaults, unmapped_labels: labels, unresolved_vendors: vendors } = data;
  if (vaults.length + labels.length + vendors.length === 0) return null;

  return (
    <section
      aria-label="Peringatan pemetaan"
      className="rounded-[var(--radius-lg)] border border-[var(--warning-fg)] bg-[var(--warning-bg)] p-4 text-sm text-[var(--n-800)]"
    >
      <h2 className="mb-2 flex items-center gap-2 font-semibold text-[var(--warning-fg)]">
        <TriangleAlert className="h-4 w-4" aria-hidden="true" />
        Laporan belum lengkap: ada pemetaan yang kurang
      </h2>
      {isAdmin && (
        <p className="mb-2">
          Lengkapi peta lokasi DSR di{" "}
          <Link to="/settings/admin/vendors" className="font-medium underline">
            master data vendor
          </Link>
          .
        </p>
      )}
      <ul className="flex flex-col gap-1">
        {vaults.length > 0 && (
          <li>
            <strong>{vaults.length} vault belum ter-map ke label DSR:</strong>
            <ul className="ml-5 list-disc">
              {vaults.map((v) => (
                <li key={v.vendor_vault_id}>
                  {v.vendor_name} · {v.branch_code} – {v.branch_name} · vault {v.vault_code}
                  {v.branch_excluded && <strong> (cabang dikeluarkan dari laporan)</strong>}
                </li>
              ))}
            </ul>
          </li>
        )}
        {labels.length > 0 && (
          <li>
            <strong>{labels.length} label lokasi DSR tidak ter-map:</strong>
            <ul className="ml-5 list-disc">
              {labels.map((l) => (
                <li key={`${l.vendor_id}-${l.dsr_location}`}>
                  {l.vendor_name} · "{l.dsr_location}" ({l.receipt_count} penerimaan)
                </li>
              ))}
            </ul>
          </li>
        )}
        {vendors.length > 0 && (
          <li>
            <strong>{vendors.length} nama vendor di DSR tidak dikenali:</strong>
            <ul className="ml-5 list-disc">
              {vendors.map((v) => (
                <li key={v.vendor_label}>
                  "{v.vendor_label}" ({v.receipt_count} penerimaan)
                </li>
              ))}
            </ul>
          </li>
        )}
      </ul>
    </section>
  );
}
