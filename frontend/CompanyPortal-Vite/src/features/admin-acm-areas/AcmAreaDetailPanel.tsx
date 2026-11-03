import { Button } from "@/components/ui/Button";
import { useToast } from "@/lib/hooks/useToast";
import { useState } from "react";
import {
  type AcmAreaDetail,
  type AcmAreaMutation,
  useAcmArea,
  useAcmAreaMutation,
  useAcmEligibleUsers,
  useAcmRegionOptions,
} from "./api";

const inputClass =
  "min-h-[44px] rounded-[var(--radius-md)] border border-[var(--n-300)] bg-[var(--n-0)] px-3 text-sm text-[var(--n-800)] outline-none focus-visible:border-[var(--red-400)] focus-visible:ring-2 focus-visible:ring-[var(--red-100)]";

function toggle<T>(set: Set<T>, id: T): Set<T> {
  const next = new Set(set);
  if (next.has(id)) next.delete(id);
  else next.add(id);
  return next;
}

/** Manage one area: name, regions (replace-all, cit-vault-select FR1) and members (replace-all). */
export function AcmAreaDetailPanel({ areaId, onClose }: { areaId: number; onClose: () => void }) {
  const detailQuery = useAcmArea(areaId);
  if (detailQuery.isLoading) return <p className="text-sm text-[var(--n-500)]">Memuat…</p>;
  if (!detailQuery.data) {
    return (
      <p role="alert" className="text-sm text-[var(--danger-fg)]">
        Gagal memuat area
      </p>
    );
  }
  return <AreaEditor area={detailQuery.data} onClose={onClose} />;
}

function AreaEditor({ area, onClose }: { area: AcmAreaDetail; onClose: () => void }) {
  const { toast } = useToast();
  const mutation = useAcmAreaMutation();
  const regionOptions = useAcmRegionOptions().data ?? [];
  const users = useAcmEligibleUsers().data ?? [];

  const [name, setName] = useState(area.name);
  const [regionCodes, setRegionCodes] = useState(() => new Set(area.region_codes));
  const [userIds, setUserIds] = useState(() => new Set(area.members.map((m) => m.user_id)));
  const [filter, setFilter] = useState("");

  function run(m: AcmAreaMutation, success: string): void {
    mutation.mutate(m, {
      onSuccess: () => toast({ type: "success", message: success }),
      onError: (err) => toast({ type: "error", message: err.message }),
    });
  }

  // Options only list regions of active branches; keep this area's own codes visible regardless.
  const known = new Set(regionOptions.map((r) => r.region_code));
  const options = [
    ...regionOptions,
    ...area.region_codes
      .filter((c) => !known.has(c))
      .map((c) => ({
        region_code: c,
        branch_count: 0,
        acm_area_id: area.id,
        acm_area_name: area.name,
      })),
  ];
  const q = filter.trim().toUpperCase();
  const visibleRegions = options.filter((r) => !q || r.region_code.includes(q));
  const readOnly = !area.is_active;

  return (
    <section
      aria-label={`Kelola area ${area.name}`}
      className="flex flex-col gap-5 rounded-[var(--radius-lg)] border border-[var(--n-200)] bg-[var(--n-0)] p-5"
    >
      <div className="flex items-center justify-between gap-2">
        <h2 className="text-lg font-semibold text-[var(--n-900)]">Kelola area: {area.name}</h2>
        <Button variant="secondary" onClick={onClose}>
          Tutup
        </Button>
      </div>
      {readOnly && (
        <p className="text-sm text-[var(--n-600)]">
          Area nonaktif — aktifkan dulu untuk mengubah region dan anggota.
        </p>
      )}

      <div className="flex flex-wrap items-end gap-2">
        <label className="flex flex-col gap-1 text-sm font-medium text-[var(--n-700)]">
          Nama area
          <input
            value={name}
            onChange={(e) => setName(e.target.value)}
            maxLength={100}
            className={inputClass}
          />
        </label>
        <Button
          variant="secondary"
          disabled={!name.trim() || name.trim() === area.name || mutation.isPending}
          onClick={() =>
            run({ op: "rename", id: area.id, name: name.trim() }, "Nama area disimpan")
          }
        >
          Simpan Nama
        </Button>
      </div>

      <fieldset className="flex flex-col gap-2" disabled={readOnly}>
        <legend className="text-sm font-semibold text-[var(--n-800)]">
          Region (<span className="tabular-nums">{regionCodes.size}</span> dipilih)
        </legend>
        <p className="text-xs text-[var(--n-600)]">
          Request replenish dengan kode region ini masuk ke area ini. Satu region hanya boleh di
          satu area.
        </p>
        <input
          aria-label="Cari region"
          placeholder="Cari kode region…"
          value={filter}
          onChange={(e) => setFilter(e.target.value)}
          className={inputClass}
        />
        <ul className="max-h-72 overflow-y-auto rounded-[var(--radius-md)] border border-[var(--n-200)]">
          {visibleRegions.map((r) => {
            const heldElsewhere = r.acm_area_id !== null && r.acm_area_id !== area.id;
            return (
              <li
                key={r.region_code}
                className="border-b border-[var(--n-100)] px-3 py-2 text-sm last:border-b-0"
              >
                <label className="flex items-center gap-2">
                  <input
                    type="checkbox"
                    checked={regionCodes.has(r.region_code)}
                    disabled={heldElsewhere}
                    onChange={() => setRegionCodes((s) => toggle(s, r.region_code))}
                  />
                  <span>
                    <span className="font-mono">{r.region_code}</span>
                    <span className="text-[var(--n-500)]">
                      {" "}
                      · <span className="tabular-nums">{r.branch_count}</span> cabang aktif
                    </span>
                    {heldElsewhere && (
                      <span className="text-[var(--n-500)]">
                        {" "}
                        — sudah di area {r.acm_area_name}
                      </span>
                    )}
                  </span>
                </label>
              </li>
            );
          })}
        </ul>
        <div>
          <Button
            disabled={mutation.isPending}
            onClick={() =>
              run(
                { op: "regions", id: area.id, region_codes: [...regionCodes].sort() },
                "Region area disimpan",
              )
            }
          >
            Simpan Region
          </Button>
        </div>
      </fieldset>

      <fieldset className="flex flex-col gap-2" disabled={readOnly}>
        <legend className="text-sm font-semibold text-[var(--n-800)]">
          Anggota (ACM-USER / ACM-SPV) — <span className="tabular-nums">{userIds.size}</span>{" "}
          dipilih
        </legend>
        {users.length === 0 && (
          <p className="text-sm text-[var(--n-500)]">
            Belum ada user aktif dengan role ACM-USER atau ACM-SPV.
          </p>
        )}
        <ul className="flex flex-col gap-1">
          {users.map((u) => (
            <li key={u.id} className="text-sm">
              <label className="flex items-center gap-2">
                <input
                  type="checkbox"
                  checked={userIds.has(u.id)}
                  onChange={() => setUserIds((s) => toggle(s, u.id))}
                />
                {u.full_name} ({u.username}) · {u.role}
              </label>
            </li>
          ))}
        </ul>
        <div>
          <Button
            disabled={mutation.isPending}
            onClick={() =>
              run({ op: "members", id: area.id, user_ids: [...userIds] }, "Anggota area disimpan")
            }
          >
            Simpan Anggota
          </Button>
        </div>
      </fieldset>
    </section>
  );
}
