import { useEffect, useMemo, useState } from "react";
import {
  Button,
  Checkbox,
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  Empty,
  EmptyDescription,
  EmptyHeader,
  EmptyTitle,
  Label,
  RadioGroup,
  RadioGroupItem,
  Textarea,
  cn,
} from "@kits/shadcn-ui";
import { ArrowRight, Info, ShieldOff, Users } from "lucide-react";
import {
  DOMAINS,
  TIERS,
  VENDORS,
  domainOf,
  eligibility,
  idr,
  levelOf,
  roleLabel,
} from "./data.js";
import { Pill, RolePill } from "./ui.jsx";

const nameOf = (users, id) => users.find((u) => u.id === id)?.name ?? null;

/* ── Tambah anak buah ─────────────────────────────────────────────────────── */
export function AddSubordinateDialog({ open, onOpenChange, spv, users, locked, onAdd }) {
  const [picked, setPicked] = useState([]);
  useEffect(() => {
    if (open) setPicked([]);
  }, [open]);

  const candidates = useMemo(() => {
    if (!spv) return [];
    return users
      .filter((u) => levelOf(u) === "maker" && u.supervisorId !== spv.id)
      .filter((u) => eligibility(u, spv).ok)
      .sort((a, b) => Number(Boolean(a.supervisorId)) - Number(Boolean(b.supervisorId)));
  }, [users, spv]);

  if (!spv) return null;
  const domain = DOMAINS[domainOf(spv)].label;

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-xl gap-0 border-[var(--border-c)] bg-[var(--surface-raised)] p-0">
        <DialogHeader className="border-b border-[var(--border-c)] p-5 text-left">
          <DialogTitle className="text-[1.0625rem] font-semibold tracking-[-0.01em]">
            Tambah anak buah untuk {spv.name}
          </DialogTitle>
          <DialogDescription className="text-[var(--ink-secondary)]">
            Hanya maker di domain {domain}
            {domainOf(spv) === "vendor" ? ` · ${VENDORS[spv.vendor]}` : ""} yang bisa dipilih.
            Checker menyetujui pekerjaan di domainnya sendiri.
          </DialogDescription>
        </DialogHeader>

        <div className="max-h-[330px] overflow-y-auto p-2">
          {candidates.length === 0 ? (
            <Empty className="py-8">
              <EmptyHeader>
                <EmptyTitle className="text-base">Tidak ada kandidat</EmptyTitle>
                <EmptyDescription>
                  Semua maker {domain}
                  {domainOf(spv) === "vendor" ? ` di ${VENDORS[spv.vendor]}` : ""} sudah berada di
                  bawah {spv.name}. Pengguna baru muncul di sini setelah rolenya di-grant.
                </EmptyDescription>
              </EmptyHeader>
            </Empty>
          ) : (
            <ul className="flex flex-col">
              {candidates.map((c) => {
                const isLocked = locked.has(c.id);
                const checked = picked.includes(c.id);
                return (
                  <li key={c.id}>
                    <label
                      className={cn(
                        "flex cursor-pointer items-center gap-3 rounded-[4px] px-3 py-2.5",
                        isLocked ? "cursor-not-allowed opacity-50" : "hover:bg-[var(--surface-sunken)]",
                      )}
                    >
                      <Checkbox
                        checked={checked}
                        disabled={isLocked}
                        onCheckedChange={(v) =>
                          setPicked((p) => (v ? [...p, c.id] : p.filter((x) => x !== c.id)))
                        }
                      />
                      <span className="min-w-0 flex-1">
                        <span className="block truncate text-sm font-medium">{c.name}</span>
                        <span className="mono block truncate text-xs text-[var(--ink-secondary)]">
                          {c.username}
                        </span>
                      </span>
                      {isLocked ? (
                        <Pill tone="info">Menunggu checker</Pill>
                      ) : c.supervisorId ? (
                        <span className="shrink-0 text-xs text-[var(--ink-secondary)]">
                          Kini di bawah {nameOf(users, c.supervisorId)}
                        </span>
                      ) : (
                        <Pill tone="warning" icon={ShieldOff}>
                          Tanpa atasan
                        </Pill>
                      )}
                    </label>
                  </li>
                );
              })}
            </ul>
          )}
        </div>

        <DialogFooter className="items-center gap-3 border-t border-[var(--border-c)] p-4 sm:justify-between">
          <span className="text-sm text-[var(--ink-secondary)]">
            {picked.length} dipilih dari {candidates.length} kandidat
          </span>
          <span className="flex gap-2">
            <Button
              variant="outline"
              className="h-9 rounded-[4px] border-[var(--border-strong)] bg-[var(--surface)]"
              onClick={() => onOpenChange(false)}
            >
              Batal
            </Button>
            <Button
              className="h-9 rounded-[4px]"
              disabled={picked.length === 0}
              onClick={() => {
                onAdd(picked);
                onOpenChange(false);
              }}
            >
              Tambahkan {picked.length > 0 ? picked.length : ""}
            </Button>
          </span>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

/* ── Pindahkan ke supervisor lain ─────────────────────────────────────────── */
export function MoveDialog({ open, onOpenChange, members, users, onMove }) {
  const [target, setTarget] = useState("");
  useEffect(() => {
    if (open) setTarget("");
  }, [open]);

  const first = members[0];
  const options = useMemo(() => {
    if (!first) return [];
    return users
      .filter((u) => levelOf(u) === "checker")
      .map((spv) => ({
        spv,
        blocked: members.filter((m) => !eligibility(m, spv).ok || m.supervisorId === spv.id),
      }))
      .filter((o) => o.blocked.length < members.length)
      .sort((a, b) => a.blocked.length - b.blocked.length || a.spv.name.localeCompare(b.spv.name));
  }, [users, members, first]);

  if (!first) return null;

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-xl gap-0 border-[var(--border-c)] bg-[var(--surface-raised)] p-0">
        <DialogHeader className="border-b border-[var(--border-c)] p-5 text-left">
          <DialogTitle className="text-[1.0625rem] font-semibold tracking-[-0.01em]">
            {members.length === 1
              ? `Pindahkan ${first.name}`
              : `Pindahkan ${members.length} pengguna`}
          </DialogTitle>
          <DialogDescription className="text-[var(--ink-secondary)]">
            {members.length === 1 ? (
              <>
                Atasan saat ini:{" "}
                <strong className="font-semibold text-[var(--ink)]">
                  {nameOf(users, first.supervisorId) ?? "belum ada"}
                </strong>
                . Pilih checker baru.
              </>
            ) : (
              "Pilih satu checker untuk semua pengguna yang dipilih."
            )}
          </DialogDescription>
        </DialogHeader>

        <div className="max-h-[330px] overflow-y-auto p-2">
          <RadioGroup value={target} onValueChange={setTarget} className="gap-0">
            {options.map(({ spv, blocked }) => {
              const id = `spv-${spv.id}`;
              return (
                <Label
                  key={spv.id}
                  htmlFor={id}
                  className="flex cursor-pointer items-center gap-3 rounded-[4px] px-3 py-2.5 font-normal hover:bg-[var(--surface-sunken)]"
                >
                  <RadioGroupItem value={String(spv.id)} id={id} />
                  <span className="min-w-0 flex-1">
                    <span className="block truncate text-sm font-medium">{spv.name}</span>
                    <span className="block truncate text-xs text-[var(--ink-secondary)]">
                      {roleLabel(spv.role)} · {TIERS[spv.tier].label} ·{" "}
                      {users.filter((u) => u.supervisorId === spv.id).length} anak buah
                    </span>
                  </span>
                  {blocked.length > 0 ? (
                    <span className="shrink-0 text-xs text-[var(--warning)]">
                      {blocked.length} tidak memenuhi syarat
                    </span>
                  ) : null}
                </Label>
              );
            })}
          </RadioGroup>
        </div>

        <div className="flex items-start gap-2 border-t border-[var(--border-c)] bg-[var(--surface-sunken)] px-5 py-3 text-xs text-[var(--ink-secondary)]">
          <Info className="mt-px size-3.5 shrink-0" />
          <p className="max-w-none">
            Checker lintas domain tidak ditampilkan: persetujuan dibaca dari{" "}
            <span className="mono">role_level</span> di dalam{" "}
            <span className="mono">domain_scope</span> yang sama.
          </p>
        </div>

        <DialogFooter className="gap-2 border-t border-[var(--border-c)] p-4">
          <Button
            variant="outline"
            className="h-9 rounded-[4px] border-[var(--border-strong)] bg-[var(--surface)]"
            onClick={() => onOpenChange(false)}
          >
            Batal
          </Button>
          <Button
            className="h-9 rounded-[4px]"
            disabled={!target}
            onClick={() => {
              const spvId = Number(target);
              const moved = members.filter(
                (m) =>
                  m.supervisorId !== spvId &&
                  eligibility(m, users.find((u) => u.id === spvId)).ok,
              );
              onMove(moved.map((m) => m.id), spvId);
              onOpenChange(false);
            }}
          >
            Pindahkan
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

/* ── Kirim untuk persetujuan (maker-checker) ──────────────────────────────── */
export function SubmitDialog({ open, onOpenChange, changes, users, onConfirm }) {
  const [note, setNote] = useState("");
  useEffect(() => {
    if (open) setNote("");
  }, [open]);

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-2xl gap-0 border-[var(--border-c)] bg-[var(--surface-raised)] p-0">
        <DialogHeader className="border-b border-[var(--border-c)] p-5 text-left">
          <DialogTitle className="text-[1.0625rem] font-semibold tracking-[-0.01em]">
            Ajukan {changes.length} perubahan hierarki
          </DialogTitle>
          <DialogDescription className="text-[var(--ink-secondary)]">
            Perubahan hierarki adalah entitas <span className="mono">user_roles</span> dan berlaku
            hanya setelah checker lain menyetujui. Anda tidak bisa menyetujui pengajuan sendiri.
          </DialogDescription>
        </DialogHeader>

        <div className="max-h-[300px] overflow-y-auto">
          <table className="crown">
            <thead>
              <tr>
                <th className="label-xs">Pengguna</th>
                <th className="label-xs">Sebelum</th>
                <th className="label-xs">Sesudah</th>
              </tr>
            </thead>
            <tbody>
              {changes.map((c) => (
                <tr key={`${c.userId}-${c.kind}`}>
                  <td>
                    <span className="block font-medium">{nameOf(users, c.userId)}</span>
                    <span className="label-xs text-[var(--ink-muted)]">
                      {c.kind === "supervisor" ? "Atasan" : "Level persetujuan"}
                    </span>
                  </td>
                  <td className="text-[var(--ink-secondary)]">{c.before}</td>
                  <td>
                    <span className="inline-flex items-center gap-2 font-medium text-[var(--ink)]">
                      <ArrowRight className="size-3.5 text-[var(--ink-muted)]" />
                      {c.after}
                    </span>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>

        <div className="flex flex-col gap-1.5 border-t border-[var(--border-c)] p-5">
          <Label htmlFor="alasan" className="label-xs text-[var(--ink-secondary)]">
            Alasan perubahan (masuk ke audit log)
          </Label>
          <Textarea
            id="alasan"
            rows={2}
            value={note}
            onChange={(e) => setNote(e.target.value)}
            placeholder="Rotasi shift Oktober 2026"
            className="rounded-[4px] border-[var(--border-strong)] bg-[var(--surface)] text-sm focus-visible:ring-2 focus-visible:ring-offset-2 focus-visible:ring-offset-[var(--surface-raised)]"
          />
        </div>

        <DialogFooter className="gap-2 border-t border-[var(--border-c)] p-4">
          <Button
            variant="outline"
            className="h-9 rounded-[4px] border-[var(--border-strong)] bg-[var(--surface)]"
            onClick={() => onOpenChange(false)}
          >
            Kembali
          </Button>
          <Button
            className="h-9 rounded-[4px]"
            onClick={() => {
              onConfirm(note);
              onOpenChange(false);
            }}
          >
            <Users /> Kirim untuk persetujuan
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
