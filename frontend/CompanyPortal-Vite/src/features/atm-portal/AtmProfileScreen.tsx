/**
 * ATM Profile page: breadcrumb, PageHeader, master-data header, tabbed
 * Replenish/Cashpos history. Renders inside AppShell (already applied by
 * _protected.tsx's ProtectedLayout, so this component doesn't wrap itself).
 */

import { PageHeader } from "@/components/ui/PageHeader";
import { Link, useParams } from "@tanstack/react-router";
import { AlertCircle } from "lucide-react";
import { useMemo } from "react";
import { AtmHeader } from "./components/AtmHeader";
import { CashposProfileTable } from "./components/CashposProfileTable";
import { ReplenishTable } from "./components/ReplenishTable";
import { TabNavigation } from "./components/TabNavigation";
import {
  useAtmCashposHistory,
  useAtmMasterData,
  useAtmReplenishHistory,
} from "./useAtmProfileData";
import { useAtmProfileUrlState } from "./useAtmProfileUrlState";

function announcementFor(
  isLoading: boolean,
  isError: boolean,
  resultCount: number,
  label: string,
): string {
  if (isLoading) {
    return `Memuat data ${label}…`;
  }
  if (isError) {
    return `Gagal memuat data ${label}`;
  }
  if (resultCount === 0) {
    return `Tidak ada data ${label} yang sesuai filter`;
  }
  return `Menampilkan ${resultCount} baris ${label}`;
}

export function AtmProfileScreen() {
  const { terminalId } = useParams({ strict: false }) as { terminalId?: string };
  const { tab, historyParams, setTab, setHistoryParams } = useAtmProfileUrlState();

  const masterQuery = useAtmMasterData(terminalId ?? "");
  const replenishQuery = useAtmReplenishHistory(
    terminalId ?? "",
    historyParams,
    tab === "replenish",
  );
  const cashposQuery = useAtmCashposHistory(terminalId ?? "", historyParams, tab === "cashpos");

  const activeQuery = tab === "cashpos" ? cashposQuery : replenishQuery;
  const activeResultCount = activeQuery.data?.data.length ?? 0;
  const liveAnnouncement = useMemo(
    () =>
      announcementFor(
        activeQuery.isLoading,
        activeQuery.isError,
        activeResultCount,
        tab === "cashpos" ? "cash position" : "replenish",
      ),
    [activeQuery.isLoading, activeQuery.isError, activeResultCount, tab],
  );

  if (!terminalId) {
    return <NotFoundState />;
  }

  if (masterQuery.isError) {
    const status = masterQuery.error?.status;
    if (status === 404) {
      return <NotFoundState />;
    }
    return <ErrorState onRetry={() => masterQuery.refetch()} />;
  }

  return (
    <div>
      <nav aria-label="Breadcrumb" className="mb-4">
        <ol className="flex items-center gap-2 text-sm text-[var(--n-600)]">
          <li>
            <Link to="/atm-portal" className="text-[var(--red-600)] hover:underline">
              ATM Portal
            </Link>
          </li>
          <li aria-hidden="true">/</li>
          <li aria-current="page" className="font-mono text-[var(--n-900)]">
            {terminalId}
          </li>
        </ol>
      </nav>

      <PageHeader
        eyebrow="ATM Portal"
        title={terminalId}
        description={masterQuery.data?.location_name || "—"}
      />

      <div className="flex flex-col gap-6">
        <AtmHeader data={masterQuery.data} isLoading={masterQuery.isLoading} />

        <div>
          <TabNavigation activeTab={tab} onTabChange={setTab} />

          <div
            role="tabpanel"
            id={`atm-profile-tabpanel-${tab}`}
            aria-labelledby={`atm-profile-tab-${tab}`}
            className="pt-4"
          >
            {tab === "replenish" ? (
              <ReplenishTable
                data={replenishQuery.data?.data ?? []}
                total={replenishQuery.data?.total ?? 0}
                isLoading={replenishQuery.isLoading}
                isError={replenishQuery.isError}
                onRetry={() => replenishQuery.refetch()}
                params={historyParams}
                onParamsChange={setHistoryParams}
              />
            ) : (
              <CashposProfileTable
                data={cashposQuery.data?.data ?? []}
                total={cashposQuery.data?.total ?? 0}
                isLoading={cashposQuery.isLoading}
                isError={cashposQuery.isError}
                onRetry={() => cashposQuery.refetch()}
                params={historyParams}
                onParamsChange={setHistoryParams}
              />
            )}
          </div>
        </div>
      </div>

      <div aria-live="polite" aria-atomic="true" className="sr-only">
        {liveAnnouncement}
      </div>
    </div>
  );
}

function NotFoundState() {
  return (
    <div className="flex flex-col items-center justify-center gap-4 py-24 text-center">
      <AlertCircle className="h-12 w-12 text-[var(--n-400)]" aria-hidden="true" />
      <p className="text-lg font-medium text-[var(--n-800)]">ATM tidak ditemukan</p>
      <Link to="/atm-portal" className="text-sm text-[var(--red-600)] hover:underline">
        Kembali ke ATM Portal
      </Link>
    </div>
  );
}

function ErrorState({ onRetry }: { onRetry: () => void }) {
  return (
    <div className="flex flex-col items-center justify-center gap-4 py-24 text-center">
      <AlertCircle className="h-12 w-12 text-[var(--danger-fg)]" aria-hidden="true" />
      <p className="text-lg font-medium text-[var(--n-800)]">Gagal memuat data ATM</p>
      <button
        type="button"
        onClick={onRetry}
        className="min-h-[44px] rounded-[var(--radius-md)] border border-[var(--n-300)] bg-[var(--n-0)] px-4 text-sm font-medium text-[var(--n-800)] outline-none focus-visible:ring-2 focus-visible:ring-[var(--red-100)]"
      >
        Coba Lagi
      </button>
    </div>
  );
}
