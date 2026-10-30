import { Button } from "@/components/ui/Button";
import { Dialog } from "@/components/ui/Dialog";
import { CircleAlert, CircleCheck, Clock, Download, Loader, TriangleAlert } from "lucide-react";
import type { ComponentType } from "react";
import type { ExportDialogProps, ExportPhase } from "../hooks";
import { isReportKind } from "../status";

type Icon = ComponentType<{ className?: string }>;

const NUMBER = new Intl.NumberFormat("id-ID");
const KEEP_RUNNING_NOTE =
  "Jika ditutup, ekspor tetap berjalan dan file bisa diunduh dari Ekspor Saya selama 10 menit setelah siap.";
const MY_EXPORTS_PATH = "/settings/admin/exports";
const REPORT_MY_EXPORTS_PATH = "/exports";

function describe(
  phase: ExportPhase,
  job: ExportDialogProps["job"],
): { icon: Icon; spin: boolean; label: string } {
  switch (phase) {
    case "waiting":
      return job?.status === "running"
        ? { icon: Loader, spin: true, label: "Membuat file" }
        : { icon: Clock, spin: false, label: "Antri" };
    case "downloading":
      return { icon: Download, spin: true, label: "Mengunduh" };
    case "done":
      return { icon: CircleCheck, spin: false, label: "Selesai" };
    case "failed":
      return { icon: CircleAlert, spin: false, label: "Ekspor gagal" };
    case "download_failed":
      return { icon: TriangleAlert, spin: false, label: "Unduhan gagal" };
    default:
      return { icon: Clock, spin: false, label: "" };
  }
}

/**
 * Progress pop-up for one export (Req 12.8-12.15). Status is always icon + text, never colour
 * alone. Spinning is transform-only and disabled under prefers-reduced-motion.
 */
export function ExportProgressDialog({
  open,
  phase,
  job,
  message,
  onClose,
  onRetry,
}: ExportDialogProps) {
  const { icon: Icon, spin, label } = describe(phase, job);
  const isBusy = phase === "waiting" || phase === "downloading";
  const isError = phase === "failed" || phase === "download_failed";
  const isReport = job ? isReportKind(job.kind) : false;
  const rows = job?.row_count ?? 0;
  const showRows =
    rows > 0 && (phase === "done" || (phase === "waiting" && job?.status === "running"));

  return (
    <Dialog open={open} onClose={onClose} title="Ekspor data">
      <div className="space-y-4">
        <div className="flex items-start gap-3">
          <Icon
            className={`mt-0.5 size-5 shrink-0 text-[var(--n-700)] ${spin ? "animate-spin motion-reduce:animate-none" : ""}`}
          />
          <div className="min-w-0 space-y-1">
            {/* One announcement per phase change; failures use role="alert". */}
            <p
              className="font-medium text-[var(--n-900)]"
              {...(isError ? { role: "alert" } : { "aria-live": "polite" as const })}
            >
              {label}
            </p>
            {isError && message && <p className="text-sm text-[var(--n-700)]">{message}</p>}
            {phase === "done" && job?.file_name && (
              <p className="break-all text-sm text-[var(--n-700)]">{job.file_name}</p>
            )}
            {showRows && (
              <p className="text-sm text-[var(--n-700)] tabular-nums">
                {NUMBER.format(rows)} baris
              </p>
            )}
            {phase === "download_failed" && (
              <p className="text-sm text-[var(--n-700)]">
                Cek menu Ekspor Saya ({isReport ? REPORT_MY_EXPORTS_PATH : MY_EXPORTS_PATH}) untuk
                mengunduh ulang.
              </p>
            )}
            {isReport && (phase === "done" || isError) && (
              <p className="text-sm">
                {/* Plain anchor: the unload guard is off in these phases, so a full navigation is safe. */}
                <a
                  href={REPORT_MY_EXPORTS_PATH}
                  className="font-medium text-[var(--red-600)] underline"
                >
                  Lihat Ekspor Saya
                </a>
              </p>
            )}
          </div>
        </div>
        {isBusy && <p className="text-sm text-[var(--n-600)]">{KEEP_RUNNING_NOTE}</p>}
        <div className="flex justify-end gap-2">
          {phase === "failed" && <Button onClick={onRetry}>Coba lagi</Button>}
          <Button variant="secondary" onClick={onClose}>
            Tutup
          </Button>
        </div>
      </div>
    </Dialog>
  );
}
