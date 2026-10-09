import { Button } from "@/components/ui/Button";
import { useToast } from "@/lib/hooks/useToast";
import { useState } from "react";
import {
  type AcmAreaDetail,
  type AcmAreaMutation,
  useAcmArea,
  useAcmAreaMutation,
  useAcmBranchOptions,
  useAcmEligibleUsers,
} from "./api";

const inputClass =
  "min-h-[44px] rounded-[var(--radius-md)] border border-[var(--n-300)] bg-[var(--n-0)] px-3 text-sm text-[var(--n-800)] outline-none focus-visible:border-[var(--red-400)] focus-visible:ring-2 focus-visible:ring-[var(--red-100)]";

function toggle(set: Set<number>, id: number): Set<number> {
  const next = new Set(set);
  if (next.has(id)) next.delete(id);
  else next.add(id);
  return next;
}

/** Manage one area: name, branches (replace-all) and members (replace-all). */
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
  const branchOptions = useAcmBranchOptions().data ?? [];
  const users = useAcmEligibleUsers().data ?? [];

  const [name, setName] = useState(area.name);
  const [branchIds, setBranchIds] = useState(
    () => new Set(area.branches.map((b) => b.vendor_branch_id)),
  );
  const [userIds, setUserIds] = useState(() => new Set(area.members.map((m) => m.user_id)));
  const [filter, setFilter] = useState("");

  function run(m: AcmAreaMutation, success: string): void {
    mutation.mutate(m, {
      onSuccess: () => toast({ type: "success", message: success }),
      onError: (err) => toast({ type: "error", message: err.message }),
    });
  }

  const q = filter.trim().toLowerCase();
  const visibleBranches = branchOptions.filter(
    (b) =>
      !q ||
      `${b.vendor_name} ${b.branch_code} ${b.branch_name} ${b.region_code ?? ""}`
        .toLowerCase()
        .includes(q),
  );
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
          Area nonaktif — aktifkan dulu untuk mengubah cabang dan anggota.
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
          Cabang vendor (<span className="tabular-nums">{branchIds.size}</span> dipilih)
        </legend>
        <input
          aria-label="Cari cabang"
          placeholder="Cari vendor / kode / nama / region…"
          value={filter}
          onChange={(e) => setFilter(e.target.value)}
          className={inputClass}
        />
        <ul className="max-h-72 overflow-y-auto rounded-[var(--radius-md)] border border-[var(--n-200)]">
          {visibleBranches.map((b) => {
            const heldElsewhere = b.acm_area_id !== null && b.acm_area_id !== area.id;
            return (
              <li
                key={b.vendor_branch_id}
                className="border-b border-[var(--n-100)] px-3 py-2 text-sm last:border-b-0"
              >
                <label className="flex items-center gap-2">
                  <input
                    type="checkbox"
                    checked={branchIds.has(b.vendor_branch_id)}
                    disabled={heldElsewhere}
                    onChange={() => setBranchIds((s) => toggle(s, b.vendor_branch_id))}
                  />
                  <span>
                    {b.vendor_name} · {b.branch_code} {b.branch_name}
                    {b.region_code && (
                      <span className="text-[var(--n-500)]"> · {b.region_code}</span>
                    )}
                    {heldElsewhere && (
                      <span className="text-[var(--n-500)]">
                        {" "}
                        — sudah di area {b.acm_area_name}
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
                { op: "branches", id: area.id, vendor_branch_ids: [...branchIds] },
                "Cabang area disimpan",
              )
            }
          >
            Simpan Cabang
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
