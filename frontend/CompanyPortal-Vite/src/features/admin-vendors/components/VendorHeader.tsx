import { Badge } from "@/components/ui/Badge";
import { Button } from "@/components/ui/Button";
import { PageHeader } from "@/components/ui/PageHeader";
import { Link } from "@tanstack/react-router";
import { CheckCircle, XCircle } from "lucide-react";
import type { AdminVendor } from "../types";
import { SummaryStrip } from "./SummaryStrip";

export function VendorHeader({
  vendor,
  vendorId,
}: {
  vendor: AdminVendor | undefined;
  vendorId: number;
}) {
  return (
    <div className="flex flex-col gap-4">
      <Link to="/settings/admin/vendors" search={{}}>
        <Button variant="ghost">← Kembali ke daftar</Button>
      </Link>

      <div className="flex flex-wrap items-start gap-4">
        {vendor && (
          <div className="flex h-12 min-w-12 items-center justify-center rounded-lg bg-[var(--n-900)] px-3 text-sm font-bold tabular-nums text-[var(--red-500)]">
            {vendor.code}
          </div>
        )}
        <div className="flex-1">
          <PageHeader
            eyebrow="Manajemen Vendor"
            title={vendor ? `${vendor.code} · ${vendor.name}` : "Detail Vendor"}
            description={vendor?.legal_name}
          />
        </div>
        {vendor && (
          <div className="pt-1">
            {vendor.is_active ? (
              <Badge variant="success" icon={CheckCircle} label="Aktif" />
            ) : (
              <Badge variant="danger" icon={XCircle} label="Nonaktif" />
            )}
          </div>
        )}
      </div>

      <SummaryStrip vendorId={vendorId} />
    </div>
  );
}
