/* Version 2: Simplified personnel register. Jakarta Sans + Plex Mono,
 * ivory #FBFAF9, ink #2B2523, action #D31327. One table, one editor.
 * Signature: named supervisor, not a tree. Direct reassignment inspired
 * by Mobike's station-to-station simplicity. No ledger or monetary assumptions.
 */
export { default } from "./SimpleApp";
/*
 * SUBJECT — "Hierarki Pengguna" (Pengaturan) in CROWN, the E2E Cash Management
 * System for CIMB Niaga. Audience: the one or two app admins who wire up who
 * approves whose work. Single job: every maker must have a named checker, and
 * you must be able to move a maker between supervisors without breaking that.
 *
 * THE DEFECT IN THE CURRENT SCREEN — it renders one row per ROLE with the
 * identity column missing, so both editable fields (supervisor, approval level)
 * are a column of em dashes. It shows the shape of the data instead of the
 * relationship the admin came to manage.
 *
 * BORROWED LOGIC — the CIT chain-of-custody handover sheet: every cash bag has
 * a named custodian and a countersignature, and an unsigned line is a visible
 * defect, never a quiet blank. So this screen is supervisor-first (a rail of
 * checkers, each with their team), an unsigned-lines tray pinned at the top of
 * that rail, and a countersign ledger at the bottom: hierarchy edits are
 * themselves maker-checker (approval_requests.entity_type = user_roles), so
 * nothing applies until another checker signs.
 *
 * DESIGN PLAN — tokens, type and colour are not mine to invent: 05-UI-BRAND.md
 * is LAW. Plus Jakarta Sans + IBM Plex Mono, light mode only, CIMB red on <10%
 * of the screen, no cards by default, no side stripes, 24px rhythm.
 * Signature: the pending-changes ledger + the "Tanpa atasan" tray, which turn
 * an empty cell from a shrug into an operational risk you can act on.
 */

import { useMemo, useState } from "react";
import { Button, cn } from "@kits/shadcn-ui";
import { Toaster, toast } from "sonner";
import {
  AlertTriangle,
  Clock3,
  Download,
  Send,
  Undo2,
  X,
} from "lucide-react";
import { DOMAINS, TIERS, USERS, domainOf, levelOf, roleLabel } from "./data.js";
import Rail from "./Rail.jsx";
import TeamPanel from "./TeamPanel.jsx";
import { AddSubordinateDialog, MoveDialog, SubmitDialog } from "./dialogs.jsx";
import { Eyebrow } from "./ui.jsx";

const REQUEST_REF = "AR-2026-0917-004";

function LegacyApp() {
  const [server] = useState(USERS);
  const [draft, setDraft] = useState(USERS);
  const [pending, setPending] = useState(null);
  const [view, setView] = useState({ type: "spv", id: 4 });
  const [query, setQuery] = useState("");
  const [selection, setSelection] = useState([]);
  const [addOpen, setAddOpen] = useState(false);
  const [moveIds, setMoveIds] = useState(null);
  const [submitOpen, setSubmitOpen] = useState(false);

  const locked = useMemo(
    () => new Set(pending ? pending.changes.map((c) => c.userId) : []),
    [pending],
  );

  const nameOf = (id) => (id ? draft.find((u) => u.id === id)?.name ?? "—" : null);
  const serverUser = (id) => server.find((u) => u.id === id);

  const changes = useMemo(() => {
    const out = [];
    for (const u of draft) {
      if (locked.has(u.id)) continue;
      const before = serverUser(u.id);
      if (!before) continue;
      if (before.supervisorId !== u.supervisorId) {
        out.push({
          userId: u.id,
          kind: "supervisor",
          before: before.supervisorId
            ? server.find((s) => s.id === before.supervisorId).name
            : "Tanpa atasan",
          after: u.supervisorId ? nameOf(u.supervisorId) : "Tanpa atasan",
        });
      }
      if (before.tier !== u.tier) {
        out.push({
          userId: u.id,
          kind: "tier",
          before: TIERS[before.tier].label,
          after: TIERS[u.tier].label,
        });
      }
    }
    return out;
  }, [draft, locked, server]);

  const stateOf = (u) => {
    if (locked.has(u.id)) return "pending";
    return changes.some((c) => c.userId === u.id) ? "draft" : "clean";
  };

  const unassigned = draft.filter((u) => levelOf(u) === "maker" && !u.supervisorId);

  const go = (next) => {
    setView(next);
    setSelection([]);
  };

  const assign = (ids, spvId) => {
    setDraft((d) => d.map((u) => (ids.includes(u.id) ? { ...u, supervisorId: spvId } : u)));
    setSelection([]);
    const target = draft.find((u) => u.id === spvId);
    toast.success(
      ids.length === 1
        ? `${draft.find((u) => u.id === ids[0]).name} → ${target.name}`
        : `${ids.length} pengguna → ${target.name}`,
      { description: "Belum berlaku. Kirim untuk persetujuan checker." },
    );
  };

  const detach = (ids) => {
    setDraft((d) => d.map((u) => (ids.includes(u.id) ? { ...u, supervisorId: null } : u)));
    setSelection([]);
    toast(ids.length === 1 ? "1 pengguna dilepas" : `${ids.length} pengguna dilepas`, {
      description: "Tanpa atasan, pengajuan mereka tidak punya checker.",
    });
  };

  const setTier = (id, tier) => {
    setDraft((d) => d.map((u) => (u.id === id ? { ...u, tier } : u)));
    toast.success(`Level persetujuan → ${TIERS[tier].label}`, {
      description: "Belum berlaku. Kirim untuk persetujuan checker.",
    });
  };

  const revert = () => {
    setDraft((d) =>
      d.map((u) => (locked.has(u.id) ? u : { ...u, ...pickEditable(serverUser(u.id)) })),
    );
    setSelection([]);
    toast("Perubahan dibatalkan");
  };

  const submit = (note) => {
    setPending({ ref: REQUEST_REF, changes, note });
    setSelection([]);
    toast.success(`Pengajuan ${REQUEST_REF} terkirim`, {
      description: `${changes.length} perubahan menunggu persetujuan checker lain.`,
    });
  };

  const exportCsv = () => {
    const head = ["id", "nama", "username", "peran", "atasan", "level_persetujuan", "autentikasi"];
    const body = draft.map((u) => [
      u.id,
      u.name,
      u.username,
      roleLabel(u.role),
      u.supervisorId ? nameOf(u.supervisorId) : "",
      u.tier ? TIERS[u.tier].label : "",
      u.auth,
    ]);
    const csv = [head, ...body].map((r) => r.map((c) => `"${c}"`).join(";")).join("\n");
    const url = URL.createObjectURL(new Blob([csv], { type: "text/csv;charset=utf-8" }));
    const a = document.createElement("a");
    a.href = url;
    a.download = "hierarki-pengguna.csv";
    a.click();
    URL.revokeObjectURL(url);
    toast.success("hierarki-pengguna.csv diunduh");
  };

  const spvCount = draft.filter((u) => levelOf(u) === "checker").length;
  const moveMembers = moveIds ? draft.filter((u) => moveIds.includes(u.id)) : [];

  return (
    <div className="flex h-[100dvh] min-h-0 flex-col bg-[var(--bg)] text-[var(--ink)]">
      <link rel="preconnect" href="https://fonts.googleapis.com" />
      <link
        rel="stylesheet"
        href="https://fonts.googleapis.com/css2?family=Plus+Jakarta+Sans:wght@400;500;600;700;800&family=IBM+Plex+Mono:wght@400;500&display=swap"
      />
      <Toaster position="top-right" theme="light" offset={16} />

      {/* Header */}
      <header className="shrink-0 border-b border-[var(--border-c)] bg-[var(--surface)] px-6 pb-5 pt-6">
        <div className="flex flex-wrap items-end justify-between gap-6">
          <div>
            <Eyebrow>Pengaturan</Eyebrow>
            <h1 className="mt-2 text-[1.75rem] font-bold leading-tight tracking-[-0.028em] -ml-[0.035em]">
              Hierarki Pengguna
            </h1>
            <p className="mt-1.5 max-w-[68ch] text-[var(--ink-secondary)]">
              Atur atasan (supervisor) dan level persetujuan setiap pengguna. Setiap maker harus
              punya satu checker di domain yang sama.
            </p>
          </div>
          <div className="flex items-center gap-4">
            <p className="text-sm text-[var(--ink-secondary)]">
              <span className="amount text-[var(--ink)]">{draft.length}</span> pengguna
              <span className="px-1.5 text-[var(--border-strong)]">·</span>
              <span className="amount text-[var(--ink)]">{spvCount}</span> supervisor
              <span className="px-1.5 text-[var(--border-strong)]">·</span>
              <span
                className={cn(
                  "amount",
                  unassigned.length > 0 ? "text-[var(--warning-text)]" : "text-[var(--ink)]",
                )}
              >
                {unassigned.length}
              </span>{" "}
              <span className={unassigned.length > 0 ? "text-[var(--warning-text)]" : undefined}>
                tanpa atasan
              </span>
            </p>
            <Button
              variant="ghost"
              className="h-9 rounded-[4px] text-[var(--ink-secondary)] hover:bg-[var(--surface-sunken)] hover:text-[var(--ink)]"
              onClick={exportCsv}
            >
              <Download /> Unduh CSV
            </Button>
          </div>
        </div>
      </header>

      {/* Risk strip + pending request */}
      {unassigned.length > 0 && view.type !== "unassigned" ? (
        <div className="flex shrink-0 flex-wrap items-center gap-x-3 gap-y-2 border-b border-[var(--border-c)] bg-[var(--warning-tint)] px-6 py-2.5 text-sm">
          <AlertTriangle className="size-4 shrink-0 text-[var(--warning)]" />
          <span className="font-semibold text-[var(--warning-text)]">
            {unassigned.length} maker tanpa atasan
          </span>
          <span className="text-[var(--ink-secondary)]">
            {unassigned
              .slice(0, 3)
              .map((u) => u.name)
              .join(", ")}
            {unassigned.length > 3 ? ` +${unassigned.length - 3} lagi` : ""} — pengajuan mereka
            tidak punya checker dan akan tertahan di antrian.
          </span>
          <Button
            size="sm"
            variant="ghost"
            className="ml-auto h-8 rounded-[4px] text-[var(--warning-text)] hover:bg-[color-mix(in_oklab,var(--warning)_12%,transparent)] hover:text-[var(--warning-text)]"
            onClick={() => go({ type: "unassigned" })}
          >
            Tinjau
          </Button>
        </div>
      ) : null}

      {pending ? (
        <div className="flex shrink-0 flex-wrap items-center gap-x-3 gap-y-2 border-b border-[var(--border-c)] bg-[var(--info-tint)] px-6 py-2.5 text-sm">
          <Clock3 className="size-4 shrink-0 text-[var(--info)]" />
          <span className="font-semibold text-[var(--info)]">
            Pengajuan <span className="mono">{pending.ref}</span> menunggu checker
          </span>
          <span className="text-[var(--ink-secondary)]">
            {pending.changes.length} perubahan terkunci sampai disetujui. Anda tidak bisa
            menyetujui pengajuan sendiri.
          </span>
          <Button
            size="sm"
            variant="ghost"
            className="ml-auto h-8 rounded-[4px] text-[var(--info)] hover:bg-[color-mix(in_oklab,var(--info)_12%,transparent)] hover:text-[var(--info)]"
            onClick={() => {
              setDraft((d) =>
                d.map((u) =>
                  locked.has(u.id) ? { ...u, ...pickEditable(serverUser(u.id)) } : u,
                ),
              );
              setPending(null);
              toast("Pengajuan ditarik kembali");
            }}
          >
            <X /> Tarik pengajuan
          </Button>
        </div>
      ) : null}

      {/* Rail + panel */}
      <div className="flex min-h-0 flex-1 flex-col md:flex-row">
        <Rail
          users={draft}
          selected={view}
          onSelect={go}
          query={query}
          setQuery={setQuery}
          unassignedCount={unassigned.length}
        />
        <main className="flex min-h-0 min-w-0 flex-1 flex-col bg-[var(--surface)]">
          <TeamPanel
            view={view}
            users={draft}
            stateOf={stateOf}
            locked={locked}
            selection={selection}
            setSelection={setSelection}
            onAdd={() => setAddOpen(true)}
            onMove={(ids) => setMoveIds(ids)}
            onDetach={detach}
            onTier={setTier}
          />
        </main>
      </div>

      {/* Countersign ledger */}
      <div
        className={cn(
          "shrink-0 overflow-hidden border-t border-[var(--border-c)] bg-[var(--surface-raised)] transition-[max-height,opacity] duration-300 ease-[cubic-bezier(0.16,1,0.3,1)]",
          changes.length > 0 ? "max-h-40 opacity-100" : "max-h-0 opacity-0",
        )}
        aria-hidden={changes.length === 0}
      >
        <div className="flex flex-wrap items-center gap-x-6 gap-y-3 px-6 py-3.5">
          <div className="min-w-0">
            <p className="text-sm font-semibold">
              {changes.length} perubahan belum diajukan
            </p>
            <p className="mt-0.5 truncate text-xs text-[var(--ink-secondary)]">
              {changes
                .slice(0, 2)
                .map(
                  (c) =>
                    `${nameOf(c.userId)}: ${c.before} → ${c.after}`,
                )
                .join("  ·  ")}
              {changes.length > 2 ? `  ·  +${changes.length - 2} lagi` : ""}
            </p>
          </div>
          <div className="ml-auto flex items-center gap-2">
            <Button
              variant="ghost"
              className="h-10 rounded-[4px] text-[var(--ink-secondary)] hover:bg-[var(--surface-sunken)] hover:text-[var(--ink)]"
              onClick={revert}
            >
              <Undo2 /> Batalkan
            </Button>
            <Button className="h-10 rounded-[4px]" onClick={() => setSubmitOpen(true)}>
              <Send /> Kirim untuk persetujuan
            </Button>
          </div>
        </div>
      </div>

      <AddSubordinateDialog
        open={addOpen}
        onOpenChange={setAddOpen}
        spv={view.type === "spv" ? draft.find((u) => u.id === view.id) : null}
        users={draft}
        locked={locked}
        onAdd={(ids) => assign(ids, view.id)}
      />
      <MoveDialog
        open={moveIds !== null}
        onOpenChange={(v) => !v && setMoveIds(null)}
        members={moveMembers}
        users={draft}
        onMove={(ids, spvId) => (ids.length ? assign(ids, spvId) : null)}
      />
      <SubmitDialog
        open={submitOpen}
        onOpenChange={setSubmitOpen}
        changes={changes}
        users={draft}
        onConfirm={submit}
      />
    </div>
  );
}

const pickEditable = (u) => ({ supervisorId: u.supervisorId, tier: u.tier });
