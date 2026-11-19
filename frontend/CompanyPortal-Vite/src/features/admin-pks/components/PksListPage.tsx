import { BackLink } from "@/components/ui/BackLink";
import { Button } from "@/components/ui/Button";
import { EmptyState } from "@/components/ui/EmptyState";
import { PageHeader } from "@/components/ui/PageHeader";
import { useAuthStore } from "@/lib/auth/store";
import { AlertCircle, Download, Plus } from "lucide-react";
import { useCallback, useMemo, useState } from "react";
import { usePksTypesList, useVendorsList } from "../../admin-vendors/hooks";
import {
  ExportProgressDialog,
  ReportExportNotice,
  type ReportParams,
  useExportFlow,
} from "../../exports";
import { usePksList } from "../hooks";
import type { PksListItem } from "../types";
import { useAdminPksUrlState } from "../useAdminPksUrlState";
import { PksFilterBar } from "./PksFilterBar";
import { PksFormDialog } from "./PksFormDialog";
import { PksReasonDialog } from "./PksReasonDialog";
import { PksReminderDialog } from "./PksReminderDialog";
import { type PksRowAction, PksRowActions } from "./PksRowActions";
import { PksTable } from "./PksTable";

const SKELETON_ROW_KEYS = ["row-1", "row-2", "row-3", "row-4", "row-5"];
const VENDOR_OPTIONS_LIMIT = 100;
const WRITE_ROLES: ReadonlySet<string> = new Set(["ADMIN", "ADMIN_PARAM"]);

type DialogState =
  | { kind: "create" }
  | { kind: "replace"; item: PksListItem }
  | { kind: "reason"; mode: "terminate" | "unterminate"; item: PksListItem }
  | { kind: "reminder"; item: PksListItem }
  | null;

/** Daftar PKS (Req 5): filterable, paginated list of every PKS record + CSV export of the same filter. */
export function PksListPage() {
  const { params, searchInput, setSearchInput, setParams } = useAdminPksUrlState();
  const exportFlow = useExportFlow();
  // Write controls are not rendered for read-only roles (ATM-SPV).
  const canWrite = useAuthStore((st) => WRITE_ROLES.has(st.user?.role ?? ""));
  const [dialog, setDialog] = useState<DialogState>(null);
  const closeDialog = useCallback(() => setDialog(null), []);
  const rowActions = useCallback(
    (item: PksListItem) => (
      <PksRowActions
        item={item}
        onAction={(action: PksRowAction, it) =>
          setDialog(
            action === "replace"
              ? { kind: "replace", item: it }
              : action === "reminder"
                ? { kind: "reminder", item: it }
                : { kind: "reason", mode: action, item: it },
          )
        }
      />
    ),
    [],
  );

  const listQuery = usePksList({
    status: params.status,
    pks_type: params.pks_type,
    vendor: params.vendor,
    q: params.q,
    page: params.page,
    page_size: params.page_size,
  });
  const typesQuery = usePksTypesList();
  const vendorsQuery = useVendorsList({
    page: 1,
    page_size: VENDOR_OPTIONS_LIMIT,
    status: "active",
    kind: "FLM_VENDOR",
  });

  const typeOptions = useMemo(
    () =>
      (typesQuery.data ?? [])
        .filter((t) => t.is_active)
        .sort((a, b) => a.code.localeCompare(b.code))
        .map((t) => ({ value: t.code, label: t.name ? `${t.code} - ${t.name}` : t.code })),
    [typesQuery.data],
  );
  const vendorOptions = useMemo(
    () =>
      (vendorsQuery.data?.vendors ?? []).map((v) => ({
        value: v.code,
        label: `${v.code} - ${v.name}`,
      })),
    [vendorsQuery.data],
  );

  const items = listQuery.data?.items ?? [];
  const total = listQuery.data?.total ?? 0;
  const totalPages = Math.max(1, Math.ceil(total / params.page_size));

  // Export carries the same filter as the screen (Req 10.1); the page is not part of it.
  function requestExport(): void {
    // ReportParams does not list `vendor` and types `status` for master data; the PKS report takes
    // the list filter {status, pks_type, vendor, q}, serialized as-is into the query string.
    const reportParams: Record<string, string> = { status: params.status };
    if (params.pks_type) reportParams.pks_type = params.pks_type;
    if (params.vendor) reportParams.vendor = params.vendor;
    if (params.q) reportParams.q = params.q;
    void exportFlow.request({
      kind: "report",
      report: "vendor-pks",
      params: reportParams as unknown as ReportParams,
    });
  }

  return (
    <div className="flex flex-col gap-6 p-6">
      <div className="self-start">
        <BackLink to="/settings" label="Kembali ke Pengaturan" />
      </div>
      <PageHeader
        eyebrow="Pengaturan"
        title="Daftar PKS"
        actions={
          <>
            <Button variant="secondary" disabled={exportFlow.isRequesting} onClick={requestExport}>
              <Download className="h-4 w-4" aria-hidden="true" />
              Ekspor CSV
            </Button>
            {canWrite && (
              <Button onClick={() => setDialog({ kind: "create" })}>
                <Plus className="h-4 w-4" aria-hidden="true" />
                Catat PKS
              </Button>
            )}
          </>
        }
      />
      <ReportExportNotice error={exportFlow.requestError} showMyExportsLink={false} />
      <ExportProgressDialog {...exportFlow.dialogProps} />

      <PksFilterBar
        params={params}
        searchInput={searchInput}
        onSearchInputChange={setSearchInput}
        onParamsChange={setParams}
        typeOptions={typeOptions}
        vendorOptions={vendorOptions}
      />

      {listQuery.isLoading && (
        <div className="flex flex-col gap-2">
          {SKELETON_ROW_KEYS.map((key) => (
            <div key={key} className="h-10 w-full animate-pulse rounded bg-[var(--n-200)]" />
          ))}
        </div>
      )}

      {!listQuery.isLoading && listQuery.isError && (
        <div className="flex flex-col items-center gap-3 py-12">
          <AlertCircle className="h-10 w-10 text-[var(--danger-fg)]" aria-hidden="true" />
          <p className="text-sm text-[var(--n-700)]">Gagal memuat daftar PKS</p>
          <Button variant="secondary" onClick={() => listQuery.refetch()}>
            Coba Lagi
          </Button>
        </div>
      )}

      {!listQuery.isLoading && !listQuery.isError && items.length === 0 && (
        <EmptyState message="Tidak ada PKS yang sesuai dengan filter saat ini" />
      )}

      {!listQuery.isLoading && !listQuery.isError && items.length > 0 && (
        <PksTable items={items} actions={canWrite ? rowActions : false} />
      )}

      <div className="flex flex-wrap items-center justify-between gap-4 text-sm text-[var(--n-600)]">
        <span>
          Halaman {params.page} dari {totalPages} ({total} PKS)
        </span>
        <div className="flex items-center gap-2">
          <Button
            variant="secondary"
            disabled={params.page <= 1}
            onClick={() => setParams({ page: params.page - 1 })}
          >
            Sebelumnya
          </Button>
          <Button
            variant="secondary"
            disabled={params.page >= totalPages}
            onClick={() => setParams({ page: params.page + 1 })}
          >
            Berikutnya
          </Button>
        </div>
      </div>

      {dialog?.kind === "create" && <PksFormDialog onClose={closeDialog} />}
      {dialog?.kind === "replace" && <PksFormDialog replace={dialog.item} onClose={closeDialog} />}
      {dialog?.kind === "reason" && (
        <PksReasonDialog item={dialog.item} mode={dialog.mode} onClose={closeDialog} />
      )}
      {dialog?.kind === "reminder" && (
        <PksReminderDialog item={dialog.item} onClose={closeDialog} />
      )}
    </div>
  );
}
