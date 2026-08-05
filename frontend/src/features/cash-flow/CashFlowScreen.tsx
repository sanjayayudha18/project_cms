import { AlertTriangle, Database, Calendar } from "lucide-react";

import { PageHeader } from "@/components/ui/PageHeader";
import { Badge } from "@/components/ui/Badge";
import { StatsCardGrid } from "./StatsCardGrid";
import { VendorBarChart } from "./VendorBarChart";
import { AtmLevelTable } from "./AtmLevelTable";
import { useCashFlowData } from "./useCashFlowData";

export function CashFlowScreen() {
  const { data, isLoading, isError, refetch } = useCashFlowData();

  if (isLoading) return <CashFlowSkeleton />;
  if (isError) return <ErrorState onRetry={refetch} />;

  return (
    <div className="py-6 max-[759px]:py-4">
      <PageHeader
        eyebrow="monitoring"
        title="Cash Flow Monitoring"
        description="Baca dari read replica · basis summary EOD per 00:00 · periode Jul 2026"
      />
      <Badge variant="info" icon={Database} label="Sumber: EOD H-1" />

      <div className="mt-4">
        <StatsCardGrid stats={data!.stats} />
      </div>

      {/* Layout terpisah: chart + tabel */}
      <div className="grid grid-cols-1 min-[1024px]:grid-cols-[1.5fr_1fr] gap-6 mt-8">
        {/* Panel VendorBarChart */}
        <div className="rounded-lg bg-[var(--n-0)] p-5 shadow-sm">
          <div className="flex items-center justify-between mb-4">
            <h2 className="text-lg font-semibold text-[var(--n-900)]">
              Cash Flow Harian per Vendor
            </h2>
            <Badge variant="neutral" icon={Calendar} label="7 hari" />
          </div>
          <VendorBarChart data={data!.vendorChart.data} vendors={data!.vendorChart.vendors} />
        </div>

        {/* Panel AtmLevelTable */}
        <div className="rounded-lg bg-[var(--n-0)] p-5 shadow-sm">
          <h2 className="text-lg font-semibold text-[var(--n-900)] mb-4">Level Kas per ATM</h2>
          <AtmLevelTable levels={data!.atmLevels} />
        </div>
      </div>
    </div>
  );
}

function CashFlowSkeleton() {
  return (
    <div className="py-6 animate-pulse space-y-6">
      <div className="h-8 w-64 bg-[var(--n-200)] rounded" />
      <div className="grid grid-cols-1 min-[768px]:grid-cols-4 gap-4">
        {Array.from({ length: 4 }).map((_, i) => (
          <div key={i} className="h-24 bg-[var(--n-100)] rounded-lg" />
        ))}
      </div>
      <div className="grid grid-cols-1 min-[1024px]:grid-cols-[1.5fr_1fr] gap-6">
        <div className="h-72 bg-[var(--n-100)] rounded-lg" />
        <div className="h-72 bg-[var(--n-100)] rounded-lg" />
      </div>
    </div>
  );
}

function ErrorState({ onRetry }: { onRetry: () => void }) {
  return (
    <div className="py-6">
      <div className="bg-[var(--danger-bg)] text-[var(--danger-fg)] rounded-lg p-4 text-sm flex items-center gap-3">
        <AlertTriangle size={18} aria-hidden="true" className="shrink-0" />
        <span className="flex-1">Gagal memuat data cash flow. Silakan coba lagi.</span>
        <button
          type="button"
          onClick={onRetry}
          className="text-sm font-medium underline hover:no-underline"
        >
          Coba Lagi
        </button>
      </div>
    </div>
  );
}
