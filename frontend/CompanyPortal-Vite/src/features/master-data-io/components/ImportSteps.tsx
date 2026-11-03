import { Check } from "lucide-react";

export type ImportStep = 1 | 2 | 3;

const STEP_TITLES = ["Pilih data", "Unggah dan periksa", "Tinjau dan ajukan"] as const;

interface ImportStepsProps {
  current: ImportStep;
  /** Keterangan kecil per langkah (indeks 0-2), mis. nama entitas atau file. */
  details?: readonly (string | undefined)[];
}

/** Indikator tiga langkah. Status = nomor/ikon centang + teks, bukan hanya warna. */
export function ImportSteps({ current, details = [] }: ImportStepsProps) {
  return (
    <ol className="flex flex-col gap-2 sm:flex-row">
      {STEP_TITLES.map((title, i) => {
        const n = (i + 1) as ImportStep;
        const isDone = n < current;
        const isCurrent = n === current;
        const detail = details[i];
        return (
          <li
            key={title}
            aria-current={isCurrent ? "step" : undefined}
            className={`flex min-w-0 flex-1 items-center gap-3 rounded-[var(--radius-md)] border px-3 py-2 text-sm ${
              isCurrent
                ? "border-[var(--red-400)] font-semibold text-[var(--n-900)]"
                : "border-[var(--n-200)] text-[var(--n-600)]"
            }`}
          >
            <span
              className={`grid h-6 w-6 flex-none place-items-center rounded-full text-xs font-bold ${
                isCurrent
                  ? "bg-[var(--red-500)] text-[var(--n-0)]"
                  : "bg-[var(--n-200)] text-[var(--n-700)]"
              }`}
            >
              {isDone ? <Check className="h-3.5 w-3.5" aria-hidden="true" /> : n}
            </span>
            <span className="min-w-0">
              {isDone && <span className="sr-only">Selesai: </span>}
              {n}. {title}
              {detail && (
                <small className="block truncate text-xs font-normal text-[var(--n-500)]">
                  {detail}
                </small>
              )}
            </span>
          </li>
        );
      })}
    </ol>
  );
}
