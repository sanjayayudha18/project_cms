import { Dialog } from "@/components/ui/Dialog";
import { useState } from "react";
import { useCashCountAreas } from "../cash-count-parameters/api";
import { Field, FormActions, inputClass } from "../cash-count-parameters/shared";
import {
  type CashCountPic,
  PIC_ROLE_LABEL,
  type PicCreate,
  type PicRole,
  type PicUpdate,
} from "./api";

export type PicSubmit =
  | { op: "create"; body: PicCreate }
  | { op: "update"; id: number; body: PicUpdate };

/**
 * Add / edit one mapping (AC-5.1-09/10). Coordinator = user per region, Executor = user per area.
 * The role of an existing mapping is fixed; the API checks the user is active, LDAP and holds the role.
 */
export function PicDialog({
  editing,
  isPending,
  onClose,
  onSubmit,
}: {
  editing: CashCountPic | null;
  isPending: boolean;
  onClose: () => void;
  onSubmit: (m: PicSubmit) => void;
}) {
  const areas = useCashCountAreas();
  const [role, setRole] = useState<PicRole>(editing?.pic_role ?? "COORDINATOR");
  const [userId, setUserId] = useState(editing ? String(editing.user_id) : "");
  const [region, setRegion] = useState(editing?.region_code ?? "");
  const [areaId, setAreaId] = useState(editing?.area_id ? String(editing.area_id) : "");

  const uid = Number(userId);
  const regionCode = region.trim().toUpperCase();
  const area = Number(areaId);
  const targetOk = role === "COORDINATOR" ? regionCode !== "" : Number.isInteger(area) && area > 0;
  const canSubmit = Number.isInteger(uid) && uid > 0 && targetOk && !isPending;

  function submit(e: React.FormEvent): void {
    e.preventDefault();
    if (!canSubmit) return;
    const target = role === "COORDINATOR" ? { region_code: regionCode } : { area_id: area };
    onSubmit(
      editing
        ? { op: "update", id: editing.id, body: { user_id: uid, ...target } }
        : { op: "create", body: { pic_role: role, user_id: uid, ...target } },
    );
  }

  return (
    <Dialog open onClose={onClose} title={editing ? "Ubah Mapping PIC" : "Tambah Mapping PIC"}>
      <form onSubmit={submit} className="flex flex-col gap-4">
        <Field label="Peran PIC">
          <select
            value={role}
            disabled={editing !== null}
            onChange={(e) => setRole(e.target.value as PicRole)}
            className={inputClass}
          >
            {(Object.keys(PIC_ROLE_LABEL) as PicRole[]).map((r) => (
              <option key={r} value={r}>
                {PIC_ROLE_LABEL[r]}
              </option>
            ))}
          </select>
        </Field>
        <Field label="ID user">
          <input
            value={userId}
            inputMode="numeric"
            onChange={(e) => setUserId(e.target.value)}
            className={`${inputClass} tabular-nums`}
          />
        </Field>
        <p className="-mt-2 text-xs text-[var(--n-500)]">
          User wajib aktif, login LDAP, dan berperan{" "}
          {role === "COORDINATOR" ? "CC-COORDINATOR" : "CC-EXECUTOR"}.
        </p>
        {role === "COORDINATOR" ? (
          <Field label="Region">
            <input
              value={region}
              onChange={(e) => setRegion(e.target.value)}
              className={inputClass}
              placeholder="JKT1"
            />
          </Field>
        ) : (
          <Field label="Area cash count">
            <select
              value={areaId}
              onChange={(e) => setAreaId(e.target.value)}
              className={inputClass}
            >
              <option value="">Pilih area…</option>
              {(areas.data ?? [])
                .filter((a) => a.is_active)
                .map((a) => (
                  <option key={a.id} value={a.id}>
                    {a.code} — {a.name}
                  </option>
                ))}
            </select>
          </Field>
        )}
        <FormActions onClose={onClose} canSubmit={canSubmit} />
      </form>
    </Dialog>
  );
}
