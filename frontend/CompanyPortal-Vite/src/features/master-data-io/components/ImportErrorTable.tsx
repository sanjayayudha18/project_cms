import { AlertCircle } from "lucide-react";
import type { ImportRowError } from "../api";

interface ImportErrorTableProps {
  errors: ImportRowError[];
  truncated: boolean;
}

export function ImportErrorTable({ errors, truncated }: ImportErrorTableProps) {
  return (
    <div role="alert" className="flex flex-col gap-2">
      <p className="flex items-center gap-2 text-sm font-medium text-[var(--danger-fg)]">
        <AlertCircle className="h-4 w-4" aria-hidden="true" />
        {errors.length} kesalahan, perbaiki file lalu periksa ulang
      </p>
      <table className="w-full border-collapse text-sm">
        <thead>
          <tr className="text-left text-[var(--n-500)]">
            <th className="py-2 pr-4 font-medium">Baris</th>
            <th className="py-2 pr-4 font-medium">Kolom</th>
            <th className="py-2 font-medium">Kesalahan</th>
          </tr>
        </thead>
        <tbody>
          {errors.map((e) => (
            <tr key={`${e.row}-${e.field}-${e.message}`} className="border-t border-[var(--n-200)]">
              <td className="py-2 pr-4 tabular-nums">{e.row || "—"}</td>
              <td className="py-2 pr-4">{e.field || "—"}</td>
              <td className="py-2">{e.message}</td>
            </tr>
          ))}
        </tbody>
      </table>
      {truncated && (
        <p className="text-xs text-[var(--n-500)]">
          Hanya sebagian kesalahan yang ditampilkan; perbaiki lalu periksa ulang.
        </p>
      )}
    </div>
  );
}
