import { Badge } from "@/components/ui/Badge";
import { Button } from "@/components/ui/Button";
import { FilterSelect } from "@/components/ui/FilterSelect";
import { PageHeader } from "@/components/ui/PageHeader";
import { formatAtmDate } from "@/features/atm-portal/lib/formatters";
import { useToast } from "@/lib/hooks/useToast";
import { Upload } from "lucide-react";
import { useState } from "react";
import {
  type Holiday,
  type HolidayStatus,
  useCreateHoliday,
  useHolidays,
  useImportHolidays,
  useRenameHoliday,
  useSetHolidayActive,
} from "./api";

const inputClass =
  "min-h-[44px] rounded-[var(--radius-md)] border border-[var(--n-300)] bg-[var(--n-0)] px-3 text-sm text-[var(--n-800)] outline-none focus-visible:border-[var(--red-400)] focus-visible:ring-2 focus-visible:ring-[var(--red-100)]";
const labelClass = "text-xs font-medium uppercase tracking-wider text-[var(--n-600)]";
const NAME_MAX = 100;

const STATUS_OPTIONS = [
  { value: "active", label: "Aktif" },
  { value: "inactive", label: "Nonaktif" },
];

function errorText(e: unknown): string {
  return e instanceof Error || (typeof e === "object" && e !== null && "message" in e)
    ? String((e as { message: string }).message)
    : "Terjadi kesalahan";
}

/**
 * Hari libur (replenishment-realisasi FR1): ADMIN maintains the holiday calendar the realisasi job
 * uses to count workdays. Changes apply immediately and are audited; the date of a row is fixed
 * (wrong date = nonaktifkan + tambah baru).
 */
export function AdminHolidaysPage() {
  const { toast } = useToast();
  const thisYear = Number(
    new Date().toLocaleDateString("sv-SE", { timeZone: "Asia/Jakarta" }).slice(0, 4),
  );
  const [year, setYear] = useState(thisYear);
  const [status, setStatus] = useState<string | null>(null);
  const [newDate, setNewDate] = useState("");
  const [newName, setNewName] = useState("");
  const [file, setFile] = useState<File | null>(null);
  const [editing, setEditing] = useState<{ id: number; name: string } | null>(null);

  const list = useHolidays(year, (status ?? "all") as HolidayStatus);
  const create = useCreateHoliday();
  const rename = useRenameHoliday();
  const setActive = useSetHolidayActive();
  const upload = useImportHolidays();
  const nameOk = (n: string) => n.trim().length >= 1 && n.trim().length <= NAME_MAX;

  function done(message: string) {
    return {
      onSuccess: () => toast({ type: "success", message }),
      onError: (e: unknown) => toast({ type: "error", message: errorText(e) }),
    };
  }

  return (
    <div className="flex flex-col gap-6">
      <PageHeader
        eyebrow="Data master"
        title="Hari Libur"
        description="Kalender libur untuk menghitung hari kerja pada klasifikasi realisasi pengisian. Perubahan langsung berlaku dan tercatat di audit log."
      />

      <section
        aria-labelledby="add-holiday"
        className="flex flex-wrap items-end gap-4 rounded-[var(--radius-lg)] border border-[var(--n-200)] p-4"
      >
        <h2 id="add-holiday" className="sr-only">
          Tambah hari libur
        </h2>
        <div className="flex flex-col gap-1">
          <label htmlFor="hl-date" className={labelClass}>
            Tanggal
          </label>
          <input
            id="hl-date"
            type="date"
            value={newDate}
            onChange={(e) => setNewDate(e.target.value)}
            className={inputClass}
          />
        </div>
        <div className="flex flex-col gap-1">
          <label htmlFor="hl-name" className={labelClass}>
            Nama
          </label>
          <input
            id="hl-name"
            value={newName}
            maxLength={NAME_MAX}
            onChange={(e) => setNewName(e.target.value)}
            placeholder="Hari Raya Natal"
            className={`${inputClass} w-64`}
          />
        </div>
        <Button
          disabled={!newDate || !nameOk(newName) || create.isPending}
          onClick={() =>
            create.mutate(
              { holiday_date: newDate, name: newName.trim() },
              {
                ...done("Hari libur ditambahkan"),
                onSuccess: () => {
                  toast({ type: "success", message: "Hari libur ditambahkan" });
                  setNewDate("");
                  setNewName("");
                },
              },
            )
          }
        >
          Tambah
        </Button>
        <div className="ml-auto flex flex-col gap-1">
          <label htmlFor="hl-file" className={labelClass}>
            Impor CSV (holiday_date,name)
          </label>
          <div className="flex gap-2">
            <input
              id="hl-file"
              type="file"
              accept=".csv,text/csv"
              onChange={(e) => setFile(e.target.files?.[0] ?? null)}
              className="text-sm"
            />
            <Button
              variant="secondary"
              disabled={!file || upload.isPending}
              onClick={() =>
                file &&
                upload.mutate(file, {
                  onSuccess: (r) =>
                    toast({
                      type: "success",
                      message: `${r.created} hari libur diimpor${r.skipped.length ? `, ${r.skipped.length} dilewati (sudah ada): ${r.skipped.join(", ")}` : ""}`,
                    }),
                  onError: (e) => toast({ type: "error", message: errorText(e) }),
                })
              }
            >
              <Upload className="h-4 w-4" aria-hidden="true" />
              Unggah
            </Button>
          </div>
        </div>
      </section>

      <div className="flex flex-wrap items-end gap-4">
        <div className="flex flex-col gap-1">
          <label htmlFor="hl-year" className={labelClass}>
            Tahun
          </label>
          <input
            id="hl-year"
            type="number"
            min={2000}
            max={2100}
            value={year}
            onChange={(e) => setYear(Number(e.target.value) || thisYear)}
            className={`${inputClass} w-28 tabular-nums`}
          />
        </div>
        <FilterSelect
          label="Status"
          options={STATUS_OPTIONS}
          value={status}
          onChange={setStatus}
          placeholder="Semua status"
        />
        <span className="pb-3 text-sm text-[var(--n-500)]">{list.data?.total ?? 0} hari libur</span>
      </div>

      <div className="overflow-x-auto rounded-[var(--radius-lg)] border border-[var(--n-200)]">
        <table className="w-full min-w-[640px] border-collapse text-sm">
          <thead>
            <tr className="border-b border-[var(--n-200)] bg-[var(--n-50)] text-[var(--n-600)]">
              <th className="px-3 py-2 text-left font-medium">Tanggal</th>
              <th className="px-3 py-2 text-left font-medium">Nama</th>
              <th className="px-3 py-2 text-left font-medium">Status</th>
              <th className="px-3 py-2 text-right font-medium">Aksi</th>
            </tr>
          </thead>
          <tbody>
            {list.isLoading && (
              <tr>
                <td colSpan={4} className="px-3 py-8 text-center text-[var(--n-500)]">
                  Memuat...
                </td>
              </tr>
            )}
            {list.isError && (
              <tr>
                <td colSpan={4} className="px-3 py-8 text-center text-[var(--danger-fg)]">
                  Gagal memuat hari libur.
                </td>
              </tr>
            )}
            {list.data?.items.length === 0 && (
              <tr>
                <td colSpan={4} className="px-3 py-8 text-center text-[var(--n-600)]">
                  Belum ada hari libur untuk tahun {year}. Tanpa data, hari kerja = Senin–Jumat.
                </td>
              </tr>
            )}
            {list.data?.items.map((h: Holiday) => (
              <tr key={h.id} className="border-b border-[var(--n-100)] last:border-0">
                <td className="px-3 py-2 tabular-nums">
                  {formatAtmDate(new Date(h.holiday_date))}
                </td>
                <td className="px-3 py-2">
                  {editing?.id === h.id ? (
                    <input
                      aria-label={`Nama baru ${h.name}`}
                      value={editing.name}
                      maxLength={NAME_MAX}
                      onChange={(e) => setEditing({ id: h.id, name: e.target.value })}
                      className={`${inputClass} w-64`}
                    />
                  ) : (
                    h.name
                  )}
                </td>
                <td className="px-3 py-2">
                  <Badge
                    variant={h.is_active ? "success" : "neutral"}
                    label={h.is_active ? "Aktif" : "Nonaktif"}
                  />
                </td>
                <td className="px-3 py-2">
                  <div className="flex justify-end gap-2">
                    {editing?.id === h.id ? (
                      <>
                        <Button
                          variant="secondary"
                          disabled={!nameOk(editing.name) || rename.isPending}
                          onClick={() =>
                            rename.mutate(
                              { id: h.id, name: editing.name.trim() },
                              {
                                ...done("Nama hari libur diubah"),
                                onSuccess: () => {
                                  toast({ type: "success", message: "Nama hari libur diubah" });
                                  setEditing(null);
                                },
                              },
                            )
                          }
                        >
                          Simpan
                        </Button>
                        <Button variant="secondary" onClick={() => setEditing(null)}>
                          Batal
                        </Button>
                      </>
                    ) : (
                      <>
                        <Button
                          variant="secondary"
                          onClick={() => setEditing({ id: h.id, name: h.name })}
                        >
                          Ubah nama
                        </Button>
                        <Button
                          variant={h.is_active ? "danger" : "secondary"}
                          disabled={setActive.isPending}
                          onClick={() =>
                            setActive.mutate(
                              { id: h.id, active: !h.is_active },
                              done(
                                h.is_active ? "Hari libur dinonaktifkan" : "Hari libur diaktifkan",
                              ),
                            )
                          }
                        >
                          {h.is_active ? "Nonaktifkan" : "Aktifkan"}
                        </Button>
                      </>
                    )}
                  </div>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  );
}
