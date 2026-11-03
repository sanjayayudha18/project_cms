import type { ImportPreview } from "../api";

/** Seven counters of a dry-run. "Ubah" = data updates only; enable/disable are their own counters. */
export function ImportSummary({ preview }: { preview: ImportPreview }) {
  const stats: { label: string; value: number; danger?: boolean }[] = [
    { label: "Total baris", value: preview.total_rows },
    { label: "Baru", value: preview.creates },
    { label: "Ubah", value: preview.data_updates },
    { label: "Aktifkan", value: preview.enables },
    { label: "Nonaktifkan", value: preview.disables },
    { label: "Tanpa perubahan", value: preview.unchanged },
    { label: "Kesalahan", value: preview.errors.length, danger: preview.errors.length > 0 },
  ];
  return (
    <dl className="grid grid-cols-2 gap-3 sm:grid-cols-4 lg:grid-cols-7">
      {stats.map((s) => (
        <div
          key={s.label}
          className={`rounded border px-3 py-2 ${
            s.danger ? "border-[var(--danger-fg)]" : "border-[var(--n-200)]"
          }`}
        >
          <dt className="text-xs text-[var(--n-600)]">{s.label}</dt>
          <dd
            className={`text-xl font-semibold tabular-nums ${
              s.danger ? "text-[var(--danger-fg)]" : "text-[var(--n-900)]"
            }`}
          >
            {s.value.toLocaleString("id-ID")}
          </dd>
        </div>
      ))}
    </dl>
  );
}
