import { Badge } from "@/components/ui/Badge";
import { CheckCircle, XCircle } from "lucide-react";
import type { AdminVendor } from "../types";

const PROFILE_FIELDS = [
  ["Kode", "code"],
  ["Nama", "name"],
  ["Nama badan hukum", "legal_name"],
  ["NPWP", "npwp"],
  ["Email kontak", "contact_email"],
  ["Telepon", "contact_phone"],
] as const satisfies readonly [string, keyof AdminVendor][];

/**
 * Presentational only -- no edit control here by design (Req 4.5); the edit
 * path stays on the vendor list.
 */
export function InfoTab({ vendor }: { vendor: AdminVendor }) {
  return (
    <div className="grid gap-4 md:grid-cols-2">
      <div className="rounded-lg border border-[var(--n-200)] bg-[var(--n-0)] p-5">
        <h3 className="mb-4 text-sm font-semibold text-[var(--n-900)]">Profil Vendor</h3>
        <dl className="grid grid-cols-[10rem_1fr] gap-x-4 gap-y-3 text-sm">
          {PROFILE_FIELDS.map(([label, field]) => (
            <div key={field} className="contents">
              <dt className="font-medium text-[var(--n-600)]">{label}</dt>
              <dd className="tabular-nums text-[var(--n-900)]">{vendor[field] || "—"}</dd>
            </div>
          ))}
          <dt className="font-medium text-[var(--n-600)]">Status</dt>
          <dd>
            {vendor.is_active ? (
              <Badge variant="success" icon={CheckCircle} label="Aktif" />
            ) : (
              <Badge variant="danger" icon={XCircle} label="Nonaktif" />
            )}
          </dd>
        </dl>
      </div>

      <div className="rounded-lg border border-[var(--n-200)] bg-[var(--n-0)] p-5">
        <h3 className="mb-4 text-sm font-semibold text-[var(--n-900)]">Alamat Kantor Pusat</h3>
        <p className="text-sm text-[var(--n-900)]">{vendor.hq_address || "—"}</p>
        <p className="mt-4 text-xs text-[var(--n-500)]">
          Perubahan nama badan hukum, NPWP, atau alamat dilakukan melalui halaman Manajemen Vendor.
        </p>
      </div>
    </div>
  );
}
