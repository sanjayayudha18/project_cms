import { Clock3 } from "lucide-react";
import {
  CIT_CUTOFF_WIB,
  cutoffState,
  formatHms,
  formatOpsDate,
  useNow,
} from "../../../lib/shell/time";

/** "15:00" → "15.00" untuk label aksesibel berbahasa Indonesia. */
const CUTOFF_LABEL = CIT_CUTOFF_WIB.replace(":", ".");

/**
 * Kapsul status operasional: tanggal operasional + hitung mundur cut-off CIT (Req 9).
 * `useNow(1000)` hanya di sini agar re-render per detik terisolasi dari topbar.
 */
export function OpsStatusStrip() {
  const now = useNow(1000);
  const date = formatOpsDate(now);
  const { kind, remainingMs } = cutoffState(now);
  const isWarning = kind === "warning";
  const isClosed = kind === "closed";

  return (
    // biome-ignore lint/a11y/useSemanticElements: <fieldset> is for form controls; spec (Req 9.7) mandates role="group"
    <div
      role="group"
      aria-label={`Status operasional: ${date}, cut-off CIT pukul ${CUTOFF_LABEL} WIB`}
      className="hidden h-10 shrink-0 items-center gap-1 rounded-full border border-[var(--shell-input-line)] pr-1 pl-3 text-sm text-[var(--shell-ink)] lg:flex"
    >
      <span className="flex items-center gap-2 pr-2 font-semibold">
        <span aria-hidden="true" className="relative flex size-2">
          <span className="absolute inline-flex size-full rounded-full bg-[var(--success-solid)] opacity-60 motion-safe:animate-ping" />
          <span className="relative inline-flex size-2 rounded-full bg-[var(--success-solid)]" />
        </span>
        {date}
      </span>
      <span
        className={`flex h-8 items-center gap-1.5 rounded-full px-3 ${
          isWarning ? "bg-[var(--shell-accent-tint)]" : ""
        }`}
      >
        <Clock3 aria-hidden="true" className="size-4 shrink-0 text-[var(--shell-muted)]" />
        <span className="text-[var(--shell-muted)]">Cut-off CIT</span>
        {isClosed ? (
          <span className="font-semibold">ditutup</span>
        ) : (
          <>
            <time
              aria-hidden="true"
              className={`font-mono font-semibold tabular-nums ${
                isWarning ? "text-[var(--shell-accent)]" : ""
              }`}
            >
              {formatHms(remainingMs)}
            </time>
            {isWarning && <span className="sr-only">kurang dari 1 jam</span>}
          </>
        )}
      </span>
    </div>
  );
}
