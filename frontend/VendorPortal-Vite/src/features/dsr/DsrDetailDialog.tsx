import type { DsrUploadListItem } from "@/features/dsr/dsrUploadApi";
import { getDsrDailyDetail, getDsrRencanaIsiDetail } from "@/features/dsr/dsrUploadApi";
import { useQuery } from "@tanstack/react-query";
import { Loader2, X } from "lucide-react";

interface DsrDetailDialogProps {
  readonly item: DsrUploadListItem;
  readonly onClose: () => void;
}

function formatIDR(value: number | null): string {
  if (value === null) return "-";
  return value.toLocaleString("id-ID");
}

export function DsrDetailDialog({ item, onClose }: DsrDetailDialogProps) {
  const dailyQuery = useQuery({
    queryKey: ["dsr-daily-detail", item.daily?.file_id],
    queryFn: () => getDsrDailyDetail(item.daily!.file_id),
    enabled: item.daily !== null,
  });

  const rencanaQuery = useQuery({
    queryKey: ["dsr-rencana-detail", item.rencana_isi?.file_id],
    queryFn: () => getDsrRencanaIsiDetail(item.rencana_isi!.file_id),
    enabled: item.rencana_isi !== null,
  });

  const isLoading = dailyQuery.isLoading || rencanaQuery.isLoading;

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/40 p-4"
      role="dialog"
      aria-modal="true"
      aria-labelledby="dsr-detail-title"
      onClick={(e) => {
        if (e.target === e.currentTarget) onClose();
      }}
    >
      <div className="flex max-h-[90vh] w-full max-w-3xl flex-col overflow-hidden rounded-lg bg-white shadow-lg">
        <div className="flex items-center justify-between border-b border-neutral-200 px-5 py-4">
          <h2 id="dsr-detail-title" className="text-lg font-semibold text-surface-text">
            Data DSR — {item.report_date}
          </h2>
          <button
            type="button"
            onClick={onClose}
            aria-label="Tutup"
            className="inline-flex min-h-[44px] min-w-[44px] items-center justify-center rounded-md text-neutral-500 hover:bg-neutral-100"
          >
            <X className="size-5" aria-hidden="true" />
          </button>
        </div>

        <div className="flex-1 overflow-y-auto px-5 py-4">
          {isLoading ? (
            <div className="flex flex-col items-center gap-3 py-12">
              <Loader2 className="size-8 animate-spin text-sidebar-active" aria-hidden="true" />
              <p className="text-sm text-neutral-500">Memuat data...</p>
            </div>
          ) : (
            <div className="flex flex-col gap-6">
              {item.daily && (
                <section>
                  <h3 className="mb-2 text-sm font-semibold text-surface-text">
                    Saldo Harian (Daily)
                  </h3>
                  <div className="max-h-64 overflow-auto rounded-md border border-neutral-200">
                    <table className="w-full text-sm">
                      <thead className="sticky top-0 bg-neutral-50 text-left text-xs text-neutral-500">
                        <tr>
                          <th className="px-3 py-2">#</th>
                          <th className="px-3 py-2">Uraian</th>
                          <th className="px-3 py-2 text-right">Total (IDR)</th>
                        </tr>
                      </thead>
                      <tbody>
                        {dailyQuery.data?.daily_rows?.map((row) => (
                          <tr key={row.row_no} className="border-t border-neutral-100">
                            <td className="px-3 py-1.5 text-neutral-400">{row.row_no}</td>
                            <td className="px-3 py-1.5">{row.line_label}</td>
                            <td className="px-3 py-1.5 text-right tabular-nums">
                              {formatIDR(row.line_total_idr)}
                            </td>
                          </tr>
                        ))}
                      </tbody>
                    </table>
                  </div>
                </section>
              )}

              {item.rencana_isi && (
                <section>
                  <h3 className="mb-2 text-sm font-semibold text-surface-text">Rencana Isi</h3>
                  <div className="max-h-64 overflow-auto rounded-md border border-neutral-200">
                    <table className="w-full text-sm">
                      <thead className="sticky top-0 bg-neutral-50 text-left text-xs text-neutral-500">
                        <tr>
                          <th className="px-3 py-2">ATM ID</th>
                          <th className="px-3 py-2">Lokasi</th>
                          <th className="px-3 py-2 text-right">Isi 100rb</th>
                          <th className="px-3 py-2 text-right">Isi 50rb</th>
                        </tr>
                      </thead>
                      <tbody>
                        {rencanaQuery.data?.rencana_isi_rows?.map((row) => (
                          <tr key={row.row_no} className="border-t border-neutral-100">
                            <td className="px-3 py-1.5">{row.atm_terminal_id}</td>
                            <td className="px-3 py-1.5">{row.atm_location ?? "-"}</td>
                            <td className="px-3 py-1.5 text-right tabular-nums">
                              {formatIDR(row.fill_100k_idr)}
                            </td>
                            <td className="px-3 py-1.5 text-right tabular-nums">
                              {formatIDR(row.fill_50k_idr)}
                            </td>
                          </tr>
                        ))}
                      </tbody>
                    </table>
                  </div>
                  {rencanaQuery.data && rencanaQuery.data.error_count > 0 && (
                    <p className="mt-2 text-xs text-warning-fg">
                      {rencanaQuery.data.error_count} baris dilewati (ATM terminal id tidak
                      dikenal).
                    </p>
                  )}
                </section>
              )}
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
