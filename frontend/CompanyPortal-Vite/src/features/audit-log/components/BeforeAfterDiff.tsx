import { Badge, type BadgeVariant } from "@/components/ui/Badge";
import { Equal, Minus, Pencil, Plus } from "lucide-react";
import type { LucideIcon } from "lucide-react";
import { type DiffStatus, diffAuditValues } from "../lib/diffAuditValues";

const STATUS: Record<
  DiffStatus,
  { variant: BadgeVariant; icon: LucideIcon; label: string; marker: string }
> = {
  added: { variant: "success", icon: Plus, label: "Ditambah", marker: "+" },
  removed: { variant: "danger", icon: Minus, label: "Dihapus", marker: "−" },
  modified: { variant: "warning", icon: Pencil, label: "Berubah", marker: "~" },
  unchanged: { variant: "neutral", icon: Equal, label: "Tetap", marker: " " },
};

function show(value: unknown): string {
  if (value === undefined) return "";
  return typeof value === "string" ? value : JSON.stringify(value);
}

interface BeforeAfterDiffProps {
  before: Record<string, unknown> | null;
  after: Record<string, unknown> | null;
}

/** before null = a creation ("Dibuat", after only); after null = a removal ("Dihapus", before only). */
export function BeforeAfterDiff({ before, after }: BeforeAfterDiffProps) {
  if (before === null && after === null) {
    return <p className="text-sm text-[var(--n-500)]">Tidak ada data perubahan.</p>;
  }

  const both = before !== null && after !== null;
  const heading = both ? "Perubahan" : before === null ? "Dibuat" : "Dihapus";
  const rows = diffAuditValues(before, after);

  return (
    <section aria-label={heading} className="flex flex-col gap-2">
      <h3 className="text-sm font-semibold text-[var(--n-800)]">{heading}</h3>
      <table className="w-full border-collapse text-sm">
        <thead>
          <tr className="text-left text-[var(--n-500)]">
            <th className="py-2 pr-2 font-medium">Status</th>
            <th className="py-2 pr-4 font-medium">Bidang</th>
            {both && <th className="py-2 pr-4 font-medium">Sebelum</th>}
            <th className="py-2 font-medium">{both ? "Sesudah" : "Nilai"}</th>
          </tr>
        </thead>
        <tbody>
          {rows.map((r) => {
            const s = STATUS[r.status];
            return (
              <tr key={r.key} className="border-t border-[var(--n-200)] align-top">
                <td className="py-2 pr-2">
                  <span aria-hidden="true" className="mr-1 font-mono">
                    {s.marker}
                  </span>
                  <Badge variant={s.variant} icon={s.icon} label={s.label} />
                </td>
                <th scope="row" className="py-2 pr-4 text-left font-medium text-[var(--n-700)]">
                  {r.key}
                </th>
                {both && (
                  <td className="break-words py-2 pr-4 text-[var(--n-600)]">{show(r.before)}</td>
                )}
                <td className="break-words py-2 text-[var(--n-900)]">
                  {show(before === null || both ? r.after : r.before)}
                </td>
              </tr>
            );
          })}
        </tbody>
      </table>
    </section>
  );
}
