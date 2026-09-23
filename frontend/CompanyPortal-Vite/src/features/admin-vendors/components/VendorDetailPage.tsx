import { Badge } from "@/components/ui/Badge";
import { Button } from "@/components/ui/Button";
import { PageHeader } from "@/components/ui/PageHeader";
import { Link } from "@tanstack/react-router";
import { CheckCircle, XCircle } from "lucide-react";
import { useState } from "react";
import { useVendor } from "../hooks";
import { BranchesPanel } from "./BranchesPanel";
import { PackagePricesPanel } from "./PackagePricesPanel";
import { PicsPanel } from "./PicsPanel";

const TOP_TABS = [
  { id: "info", label: "Info" },
  { id: "branches", label: "Cabang" },
  { id: "vendorWidePics", label: "PIC Vendor-wide" },
  { id: "packagePrices", label: "Harga Paket" },
] as const;

type TopTab = (typeof TOP_TABS)[number]["id"];

function TabBar<T extends string>({
  tabs,
  active,
  onChange,
  label,
}: {
  tabs: readonly { id: T; label: string }[];
  active: T;
  onChange: (id: T) => void;
  label: string;
}) {
  return (
    <div role="tablist" aria-label={label} className="flex gap-1 border-b border-[var(--n-200)]">
      {tabs.map((t) => (
        <button
          key={t.id}
          type="button"
          role="tab"
          aria-selected={active === t.id}
          onClick={() => onChange(t.id)}
          className={`px-4 py-2 text-sm font-medium ${
            active === t.id
              ? "border-b-2 border-[var(--red-600)] text-[var(--n-900)]"
              : "text-[var(--n-500)]"
          }`}
        >
          {t.label}
        </button>
      ))}
    </div>
  );
}

/**
 * Vendor detail: Info, a clickable Cabang list that navigates to that
 * branch's own Vault/PIC/Paket page (each with full create/edit/disable/
 * enable, all maker-checker via 202-staged change requests), and a separate
 * PIC Vendor-wide tab for PICs with no branch (vendor_branch_id NULL).
 */
export function VendorDetailPage({ vendorId }: { vendorId: number }) {
  const [tab, setTab] = useState<TopTab>("info");
  const vendorQuery = useVendor(vendorId);
  const vendor = vendorQuery.data;

  return (
    <div className="flex flex-col gap-6 p-6">
      <PageHeader
        eyebrow="Manajemen Vendor"
        title={vendor ? `${vendor.code} · ${vendor.name}` : "Detail Vendor"}
        description="Data vendor beserta cabang, vault, PIC, dan paketnya."
      />
      <Link to="/settings/admin/vendors" search={{}}>
        <Button variant="ghost">← Kembali ke daftar</Button>
      </Link>

      <TabBar label="Detail vendor" tabs={TOP_TABS} active={tab} onChange={setTab} />

      {vendorQuery.isError && (
        <p role="alert" className="text-sm text-[var(--danger-fg)]">
          Vendor tidak ditemukan
        </p>
      )}

      {tab === "info" && vendor && (
        <dl className="grid max-w-2xl grid-cols-[12rem_1fr] gap-x-4 gap-y-3 text-sm">
          {(
            [
              ["Kode", vendor.code],
              ["Nama", vendor.name],
              ["Nama badan hukum", vendor.legal_name],
              ["NPWP", vendor.npwp],
              ["Email kontak", vendor.contact_email],
              ["Telepon", vendor.contact_phone],
              ["Alamat kantor pusat", vendor.hq_address],
            ] as const
          ).map(([label, value]) => (
            <div key={label} className="contents">
              <dt className="font-medium text-[var(--n-600)]">{label}</dt>
              <dd className="text-[var(--n-900)]">{value || "—"}</dd>
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
      )}

      {tab === "branches" && <BranchesPanel vendorId={vendorId} />}

      {tab === "vendorWidePics" && <PicsPanel vendorId={vendorId} branchId={null} />}

      {tab === "packagePrices" && <PackagePricesPanel vendorId={vendorId} />}
    </div>
  );
}
