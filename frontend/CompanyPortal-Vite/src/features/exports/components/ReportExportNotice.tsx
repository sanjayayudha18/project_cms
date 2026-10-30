import { Link } from "@tanstack/react-router";
import { AlertCircle } from "lucide-react";
import { isApiError, isTooManyExports } from "../api";

function requestErrorText(e: unknown): string {
  if (isTooManyExports(e))
    return "Terlalu banyak ekspor yang sedang berjalan. Tunggu salah satu selesai lalu coba lagi.";
  return isApiError(e) || e instanceof Error ? e.message : "Permintaan ekspor gagal.";
}

/** Inline request error (429 etc., icon + text, no dialog) and the "Ekspor Saya" link for report screens. */
export function ReportExportNotice({ error }: { error: unknown }) {
  return (
    <div className="flex flex-wrap items-center gap-x-4 gap-y-1 text-sm">
      {error != null && (
        <p role="alert" className="flex items-center gap-2 text-[var(--danger-fg)]">
          <AlertCircle className="h-4 w-4" aria-hidden="true" />
          {requestErrorText(error)}
        </p>
      )}
      <Link to="/exports" className="w-fit font-medium text-[var(--red-600)] underline">
        Lihat Ekspor Saya
      </Link>
    </div>
  );
}
