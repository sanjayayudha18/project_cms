import { Button } from "@/components/ui/Button";
import { PageNumbers } from "@/components/ui/PageNumbers";
import { OpBadge } from "@/features/approvals/components/MasterDataReviewDialog";
import { OP_LABELS } from "@/features/approvals/lib/changeDiff";
import { useState } from "react";
import type { ImportChange, ImportChangeOp } from "../api";

export const CHANGE_PAGE_SIZE = 50;

const OP_ORDER: ImportChangeOp[] = ["create", "update", "enable", "disable"];

const NUMERIC = /^-?[\d.,]+$/;

/** Cell values are CSV text: rendered as React text nodes only, never as HTML. */
function Value({ text }: { text: string }) {
  const numeric = NUMERIC.test(text);
  return (
    <span className={numeric ? "inline-block text-right font-mono tabular-nums" : "break-words"}>
      {text === "" ? "—" : text}
    </span>
  );
}

export function ImportChangeTable({ changes }: { changes: ImportChange[] }) {
  const [op, setOp] = useState<ImportChangeOp | "">("");
  const [page, setPage] = useState(1);

  const filtered = op ? changes.filter((c) => c.op === op) : changes;
  const pages = Math.max(1, Math.ceil(filtered.length / CHANGE_PAGE_SIZE));
  const start = (page - 1) * CHANGE_PAGE_SIZE;
  const shown = filtered.slice(start, start + CHANGE_PAGE_SIZE);

  function pickOp(next: ImportChangeOp | ""): void {
    setOp(next);
    setPage(1);
  }

  return (
    <div className="flex flex-col gap-3">
      <div className="flex flex-wrap items-end justify-between gap-3">
        <label className="flex flex-col gap-1 text-sm font-medium text-[var(--n-700)]">
          Operasi
          <select
            className="rounded border border-[var(--n-300)] bg-[var(--n-0)] px-3 py-2 text-sm text-[var(--n-900)]"
            value={op}
            onChange={(e) => pickOp(e.target.value as ImportChangeOp | "")}
          >
            <option value="">Semua operasi ({changes.length})</option>
            {OP_ORDER.map((o) => (
              <option key={o} value={o}>
                {OP_LABELS[o]} ({changes.filter((c) => c.op === o).length})
              </option>
            ))}
          </select>
        </label>
        <p className="text-xs text-[var(--n-500)]">Baris tanpa perubahan tidak ditampilkan</p>
      </div>

      <table className="w-full border-collapse text-sm">
        <thead>
          <tr className="text-left text-[var(--n-500)]">
            <th className="py-2 pr-4 font-medium">Baris</th>
            <th className="py-2 pr-4 font-medium">Kunci</th>
            <th className="py-2 pr-4 font-medium">Operasi</th>
            <th className="py-2 font-medium">Perubahan (sebelum → sesudah)</th>
          </tr>
        </thead>
        <tbody>
          {shown.map((c) => {
            return (
              <tr key={`${c.row}-${c.key}`} className="border-t border-[var(--n-200)] align-top">
                <td className="py-2 pr-4 tabular-nums">{c.row}</td>
                <td className="py-2 pr-4 font-mono">{c.key || "—"}</td>
                <td className="py-2 pr-4">
                  <OpBadge op={c.op} />
                </td>
                <td className="py-2">
                  <ul className="flex flex-col gap-1">
                    {c.fields.map((f) => (
                      <li key={f.field}>
                        <span className="text-[var(--n-600)]">{f.field}: </span>
                        {c.op === "create" ? (
                          <Value text={f.after} />
                        ) : (
                          <>
                            <span className="text-[var(--n-500)] line-through">
                              <Value text={f.before} />
                            </span>
                            {" → "}
                            <Value text={f.after} />
                          </>
                        )}
                      </li>
                    ))}
                  </ul>
                </td>
              </tr>
            );
          })}
        </tbody>
      </table>

      <div className="flex flex-wrap items-center justify-between gap-2 text-xs text-[var(--n-600)]">
        <span className="tabular-nums">
          {filtered.length === 0
            ? "Tidak ada perubahan"
            : `Menampilkan ${start + 1}-${start + shown.length} dari ${filtered.length} perubahan`}
        </span>
        {pages > 1 && (
          <nav aria-label="Halaman perubahan" className="flex items-center gap-1">
            <Button variant="secondary" disabled={page <= 1} onClick={() => setPage(page - 1)}>
              Sebelumnya
            </Button>
            <PageNumbers current={page} total={pages} onChange={setPage} />
            <Button variant="secondary" disabled={page >= pages} onClick={() => setPage(page + 1)}>
              Berikutnya
            </Button>
          </nav>
        )}
      </div>
    </div>
  );
}
