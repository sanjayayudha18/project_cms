import { Badge } from "@/components/ui/Badge";
import { EmptyState } from "@/components/ui/EmptyState";
import { PageNumbers } from "@/components/ui/PageNumbers";
import { formatWibDateTime } from "@/features/eod-monitoring/utils";
import { useNavigate, useSearch } from "@tanstack/react-router";
import { AlertTriangle, Loader2 } from "lucide-react";
import { IO_ENTITIES, type ImportBatchFilter } from "../api";
import { BATCH_STATUS, BATCH_STATUS_CODES } from "../batchStatus";
import { useImportBatches } from "../hooks";
import type { MasterDataIoSearchParams } from "../searchSchema";

export const HISTORY_PAGE_SIZE = 20;

const SELECT_CLASS =
  "min-h-[44px] rounded border border-[var(--n-300)] bg-[var(--n-0)] px-3 py-2 text-sm text-[var(--n-900)]";
const TH = "px-3 py-2 text-left text-xs font-semibold text-[var(--n-600)]";
const TD = "px-3 py-2 align-top text-sm text-[var(--n-800)]";

const entityLabel = (id: string) => IO_ENTITIES.find((e) => e.id === id)?.label ?? id;

/** URL search params (`h_*`) -> API filter. Page defaults to 1. */
export function toBatchFilter(s: MasterDataIoSearchParams): ImportBatchFilter {
  return {
    entity: s.h_entity,
    status: s.h_status,
    mine: s.h_mine || undefined,
    page: s.h_page ?? 1,
    page_size: HISTORY_PAGE_SIZE,
  };
}

/**
 * Tab "Riwayat impor": all import batches, filters + page in the URL
 * (master-data-import-revamp Req 5). Changing a filter returns to page 1.
 * The detail dialog (`?batch=ID`) is task 12.2.
 */
export function ImportHistoryTab() {
  const search = useSearch({ strict: false }) as MasterDataIoSearchParams;
  const navigate = useNavigate();
  const filter = toBatchFilter(search);
  const { data, isLoading, isError, refetch } = useImportBatches(filter);

  const filtered = Boolean(filter.entity || filter.status || filter.mine);
  const page = filter.page ?? 1;
  const totalPages = Math.max(1, Math.ceil((data?.total ?? 0) / HISTORY_PAGE_SIZE));

  /** Merge a patch into the URL; undefined/false values are dropped. */
  function setParams(patch: Partial<MasterDataIoSearchParams>): void {
    const next = { ...search, ...patch };
    const clean = Object.fromEntries(
      Object.entries(next).filter(([, v]) => v !== undefined && v !== false),
    );
    navigate({ to: ".", search: clean });
  }
  const setFilter = (patch: Partial<MasterDataIoSearchParams>) =>
    setParams({ ...patch, h_page: undefined });
  const openBatch = (id: number) => setParams({ batch: id });

  return (
    <div className="flex flex-col gap-4">
      <div className="flex flex-wrap items-end gap-4">
        <label className="flex flex-col gap-1 text-sm font-medium text-[var(--n-700)]">
          Entitas
          <select
            className={SELECT_CLASS}
            value={filter.entity ?? ""}
            onChange={(e) =>
              setFilter({ h_entity: (e.target.value || undefined) as typeof filter.entity })
            }
          >
            <option value="">Semua entitas</option>
            {IO_ENTITIES.map((e) => (
              <option key={e.id} value={e.id}>
                {e.label}
              </option>
            ))}
          </select>
        </label>
        <label className="flex flex-col gap-1 text-sm font-medium text-[var(--n-700)]">
          Status
          <select
            className={SELECT_CLASS}
            value={filter.status ?? ""}
            onChange={(e) =>
              setFilter({ h_status: (e.target.value || undefined) as typeof filter.status })
            }
          >
            <option value="">Semua status</option>
            {BATCH_STATUS_CODES.map((c) => (
              <option key={c} value={c}>
                {BATCH_STATUS[c].label}
              </option>
            ))}
          </select>
        </label>
        <label className="flex min-h-[44px] items-center gap-2 text-sm text-[var(--n-800)]">
          <input
            type="checkbox"
            className="h-4 w-4"
            checked={Boolean(filter.mine)}
            onChange={(e) => setFilter({ h_mine: e.target.checked || undefined })}
          />
          Hanya milik saya
        </label>
      </div>

      {isLoading ? (
        <output className="flex items-center gap-2 text-sm text-[var(--n-600)]">
          <Loader2 className="h-4 w-4 animate-spin" aria-hidden="true" />
          Memuat riwayat impor...
        </output>
      ) : isError ? (
        <div role="alert" className="flex items-center gap-3 text-sm text-[var(--danger-fg)]">
          <AlertTriangle className="h-4 w-4 shrink-0" aria-hidden="true" />
          Riwayat impor gagal dimuat.
          <button
            type="button"
            onClick={() => refetch()}
            className="min-h-[44px] rounded border border-[var(--n-300)] px-3 text-[var(--n-800)]"
          >
            Coba lagi
          </button>
        </div>
      ) : !data || data.items.length === 0 ? (
        <EmptyState
          message={
            filtered
              ? "Tidak ada impor yang cocok dengan filter. Ubah filter, atau ajukan file lewat tab Impor baru."
              : "Belum ada impor. Ajukan file lewat tab Impor baru."
          }
        />
      ) : (
        <>
          <div className="overflow-x-auto rounded-lg border border-[var(--n-200)]">
            <table className="w-full border-collapse">
              <thead className="border-b border-[var(--n-200)] bg-[var(--n-50)]">
                <tr>
                  <th className={TH}>Batch</th>
                  <th className={TH}>Entitas</th>
                  <th className={TH}>Nama file</th>
                  <th className={`${TH} text-right`}>Baris</th>
                  <th className={TH}>Diajukan oleh</th>
                  <th className={TH}>Diajukan (WIB)</th>
                  <th className={TH}>Status</th>
                  <th className={TH}>Diputuskan (WIB)</th>
                </tr>
              </thead>
              <tbody>
                {data.items.map((b) => {
                  const st = BATCH_STATUS[b.status];
                  return (
                    <tr
                      key={b.id}
                      className="border-b border-[var(--n-100)] hover:bg-[var(--red-50)]"
                    >
                      <td className={`${TD} tabular-nums`}>
                        <button
                          type="button"
                          aria-label={`Buka batch ${b.id}`}
                          onClick={() => openBatch(b.id)}
                          className="min-h-[44px] min-w-[44px] text-left text-[var(--red-600)] underline-offset-2 outline-none hover:underline focus-visible:ring-2 focus-visible:ring-[var(--red-100)]"
                        >
                          #{b.id}
                        </button>
                      </td>
                      <td className={TD}>{entityLabel(b.entity)}</td>
                      <td className={TD}>{b.file_name ?? "—"}</td>
                      <td className={`${TD} text-right tabular-nums`}>{b.row_count}</td>
                      <td className={TD}>{b.maker_name}</td>
                      <td className={`${TD} tabular-nums`}>{formatWibDateTime(b.created_at)}</td>
                      <td className={TD}>
                        <Badge variant={st.variant} icon={st.icon} label={st.label} />
                      </td>
                      <td className={`${TD} tabular-nums`}>
                        {b.decided_at ? (
                          <>
                            {formatWibDateTime(b.decided_at)}
                            {b.decided_by_name ? (
                              <div className="text-xs text-[var(--n-500)]">{b.decided_by_name}</div>
                            ) : null}
                          </>
                        ) : (
                          "—"
                        )}
                      </td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
          <nav
            aria-label="Halaman riwayat impor"
            className="flex flex-wrap items-center justify-between gap-2 text-sm text-[var(--n-600)]"
          >
            <span className="tabular-nums">
              Menampilkan {(page - 1) * HISTORY_PAGE_SIZE + 1}-
              {(page - 1) * HISTORY_PAGE_SIZE + data.items.length} dari {data.total} batch
            </span>
            <div className="flex items-center gap-1">
              <PageNumbers
                current={page}
                total={totalPages}
                onChange={(p) => setParams({ h_page: p === 1 ? undefined : p })}
              />
            </div>
          </nav>
        </>
      )}
    </div>
  );
}
