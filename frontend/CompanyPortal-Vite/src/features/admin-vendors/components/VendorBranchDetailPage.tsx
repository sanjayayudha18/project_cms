import { Badge } from "@/components/ui/Badge";
import { Button } from "@/components/ui/Button";
import { PageHeader } from "@/components/ui/PageHeader";
import { Link } from "@tanstack/react-router";
import { CheckCircle, XCircle } from "lucide-react";
import { useState } from "react";
import { useVendorBranch } from "../hooks";
import { ATMsPanel } from "./ATMsPanel";
import { PackagesPanel } from "./PackagesPanel";
import { PicsPanel } from "./PicsPanel";
import { VaultsPanel } from "./VaultsPanel";

const BRANCH_SUB_TABS = [
  { id: "vaults", label: "Vault" },
  { id: "pics", label: "PIC Cabang" },
  { id: "packages", label: "Paket Cabang" },
  { id: "atms", label: "ATM" },
] as const;

type BranchSubTab = (typeof BRANCH_SUB_TABS)[number]["id"];

/**
 * Own page for a vendor branch's Vault/PIC/Paket, reached by clicking a
 * branch row in VendorDetailPage's Cabang tab.
 */
export function VendorBranchDetailPage({
  vendorId,
  branchId,
}: {
  vendorId: number;
  branchId: number;
}) {
  const [subTab, setSubTab] = useState<BranchSubTab>("vaults");
  const branchQuery = useVendorBranch(vendorId, branchId);
  const branch = branchQuery.data;

  return (
    <div className="flex flex-col gap-6 p-6">
      <PageHeader
        eyebrow="Manajemen Vendor"
        title={branch ? `${branch.branch_code} · ${branch.branch_name}` : "Detail Cabang"}
        description="Vault, PIC, dan paket milik cabang ini."
      />
      <Link
        to="/settings/admin/vendors/$vendorId"
        params={{ vendorId: String(vendorId) }}
        search={{}}
      >
        <Button variant="ghost">← Kembali ke daftar cabang</Button>
      </Link>

      {branchQuery.isError && (
        <p role="alert" className="text-sm text-[var(--danger-fg)]">
          Cabang tidak ditemukan
        </p>
      )}

      {branch && (
        <span className="inline-flex w-fit items-center gap-1">
          {branch.is_active ? (
            <Badge variant="success" icon={CheckCircle} label="Aktif" />
          ) : (
            <Badge variant="danger" icon={XCircle} label="Nonaktif" />
          )}
        </span>
      )}

      <div role="tablist" aria-label="Data cabang" className="flex gap-1 border-b border-[var(--n-200)]">
        {BRANCH_SUB_TABS.map((t) => (
          <button
            key={t.id}
            type="button"
            role="tab"
            aria-selected={subTab === t.id}
            onClick={() => setSubTab(t.id)}
            className={`px-4 py-2 text-sm font-medium ${
              subTab === t.id
                ? "border-b-2 border-[var(--red-600)] text-[var(--n-900)]"
                : "text-[var(--n-500)]"
            }`}
          >
            {t.label}
          </button>
        ))}
      </div>

      {subTab === "vaults" && <VaultsPanel vendorId={vendorId} branchId={branchId} />}
      {subTab === "pics" && <PicsPanel vendorId={vendorId} branchId={branchId} />}
      {subTab === "packages" && <PackagesPanel vendorId={vendorId} branchId={branchId} />}
      {subTab === "atms" && <ATMsPanel vendorId={vendorId} branchId={branchId} />}
    </div>
  );
}
