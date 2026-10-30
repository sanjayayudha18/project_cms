import { Button } from "@/components/ui/Button";
import { Download, Info } from "lucide-react";
import { exportButtonState } from "../exportButtonState";
import { useExportFlow } from "../hooks";
import type { MasterExportParams } from "../types";
import { ExportProgressDialog } from "./ExportProgressDialog";
import { RequestError } from "./exportFormParts";

const formatCount = new Intl.NumberFormat("id-ID");

interface PreviewExportBarProps {
  /** Applied_Filter params (never the draft); `null` = required filter missing. */
  params: MasterExportParams | null;
  total: number;
  /** Draft filter differs from the applied one. */
  dirty: boolean;
  /** The table is (re)loading. */
  loading: boolean;
}

/** "Ekspor N baris" for exactly the rows the table shows (Req 5). */
export function PreviewExportBar({ params, total, dirty, loading }: PreviewExportBarProps) {
  const flow = useExportFlow();
  const state = exportButtonState({ dirty, loading, total });
  const enabled = state.enabled && params !== null && !flow.isRequesting;

  return (
    <div className="flex flex-col items-start gap-2">
      <Button
        variant="primary"
        disabled={!enabled}
        onClick={() => {
          if (params) void flow.request(params);
        }}
      >
        <Download className="mr-2 h-4 w-4" aria-hidden="true" />
        Ekspor <span className="tabular-nums">&nbsp;{formatCount.format(total)}&nbsp;</span> baris
      </Button>
      {state.message && (
        <output className="flex items-center gap-2 text-sm text-[var(--n-600)]">
          <Info className="h-4 w-4 shrink-0" aria-hidden="true" />
          {state.message}
        </output>
      )}
      <RequestError error={flow.requestError} />
      <p className="text-xs text-[var(--n-500)]">
        File diambil saat ekspor diproses; perubahan data setelah tabel dimuat bisa ikut masuk.
      </p>
      <ExportProgressDialog {...flow.dialogProps} />
    </div>
  );
}
