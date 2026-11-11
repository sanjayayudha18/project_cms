import {
  Button,
  Checkbox,
  Empty,
  EmptyDescription,
  EmptyHeader,
  EmptyMedia,
  EmptyTitle,
  ToggleGroup,
  ToggleGroupItem,
  cn,
} from "@kits/shadcn-ui";
import {
  ArrowRightLeft,
  CheckCircle2,
  Clock3,
  KeyRound,
  ShieldOff,
  Sparkle,
  UserMinus,
  UserPlus,
} from "lucide-react";
import { DOMAINS, TIERS, VENDORS, domainOf, idr, levelOf, roleLabel } from "./data.js";
import { AuthTag, Pill, RolePill } from "./ui.jsx";

function Th({ children, className }) {
  return <th className={cn("label-xs whitespace-nowrap", className)}>{children}</th>;
}

function TierCell({ user }) {
  if (levelOf(user) === "maker")
    return (
      <span className="whitespace-nowrap text-sm text-[var(--ink-secondary)]">
        Tidak berlaku
      </span>
    );
  if (levelOf(user) === "admin")
    return (
      <span className="whitespace-nowrap text-sm text-[var(--ink-secondary)]">
        Tidak berlaku
      </span>
    );
  return (
    <span className="whitespace-nowrap text-sm">
      {TIERS[user.tier].label}
      <span
        className={cn(
          "ml-2 text-xs text-[var(--ink-secondary)]",
          TIERS[user.tier].ceiling === null ? "" : "amount",
        )}
      >
        {TIERS[user.tier].ceiling === null ? "tanpa batas" : idr(TIERS[user.tier].ceiling)}
      </span>
    </span>
  );
}

function NameCell({ user, state }) {
  return (
    <span className="flex items-center gap-2 whitespace-nowrap">
      <span className="min-w-0 truncate font-medium text-[var(--ink)]" title={user.name}>
        {user.name}
      </span>
      {state === "draft" ? (
        <Pill tone="brand" icon={Sparkle}>
          Belum diajukan
        </Pill>
      ) : null}
    </span>
  );
}

function PendingCell() {
  return (
    <Pill tone="info" icon={Clock3}>
      Menunggu checker
    </Pill>
  );
}

function SectionHead({ eyebrow, title, children, right }) {
  return (
    <div className="flex min-w-0 flex-wrap items-start justify-between gap-6 border-b border-[var(--border-c)] px-6 pb-5 pt-6">
      <div className="min-w-0">
        <p className="label-xs text-[var(--ink-secondary)]">{eyebrow}</p>
        <h2 className="mt-1.5 text-[1.375rem] font-semibold tracking-[-0.02em]">{title}</h2>
        <div className="mt-2 flex flex-wrap items-center gap-x-3 gap-y-1.5 text-sm text-[var(--ink-secondary)]">
          {children}
        </div>
      </div>
      {right}
    </div>
  );
}

function Toolbar({ left, right }) {
  return (
    <div className="flex min-h-[52px] flex-wrap items-center justify-between gap-3 px-6 py-2.5">
      <div className="flex items-center gap-2">{left}</div>
      <div className="flex items-center gap-2">{right}</div>
    </div>
  );
}

export default function TeamPanel({
  view,
  users,
  stateOf,
  locked,
  selection,
  setSelection,
  onAdd,
  onMove,
  onDetach,
  onTier,
}) {
  const toggleAll = (rows, checked) =>
    setSelection(checked ? rows.filter((r) => !locked.has(r.id)).map((r) => r.id) : []);

  /* ── Tanpa atasan ───────────────────────────────────────────────────────── */
  if (view.type === "unassigned") {
    const rows = users
      .filter((u) => levelOf(u) === "maker" && !u.supervisorId)
      .sort((a, b) => a.name.localeCompare(b.name));
    const selectable = rows.filter((r) => !locked.has(r.id));

    return (
      <section className="flex min-h-0 min-w-0 flex-1 flex-col">
        <SectionHead
          eyebrow="Perlu tindakan"
          title="Tanpa atasan"
          right={
            <div className="max-w-[420px] rounded-[6px] bg-[var(--warning-tint)] px-4 py-3 text-sm text-[var(--warning-text)]">
              <span className="flex items-center gap-2 font-semibold">
                <ShieldOff className="size-4" /> Pengajuan tertahan
              </span>
              <p className="mt-1 leading-snug text-[var(--ink-secondary)]">
                Maker tanpa checker bisa membuat order, tapi tidak ada yang berwenang
                menyetujuinya. Pengajuan akan menumpuk di antrian sampai atasan ditetapkan.
              </p>
            </div>
          }
        >
          <span>
            <span className="amount">{rows.length}</span> maker belum punya checker
          </span>
        </SectionHead>

        {rows.length === 0 ? (
          <Empty className="flex-1 py-20">
            <EmptyHeader>
              <EmptyMedia variant="icon" className="bg-[var(--success-tint)] text-[var(--success)]">
                <CheckCircle2 />
              </EmptyMedia>
              <EmptyTitle>Semua maker punya atasan</EmptyTitle>
              <EmptyDescription>
                Setiap maker aktif sudah terhubung ke satu checker di domainnya. Pengguna baru akan
                muncul di sini begitu rolenya di-grant dan sebelum atasannya ditetapkan.
              </EmptyDescription>
            </EmptyHeader>
          </Empty>
        ) : (
          <>
            <Toolbar
              left={
                <span className="text-sm text-[var(--ink-secondary)]">
                  {selection.length > 0 ? (
                    <strong className="font-semibold text-[var(--ink)]">
                      {selection.length} dipilih
                    </strong>
                  ) : (
                    `${rows.length} pengguna`
                  )}
                </span>
              }
              right={
                <Button
                  className="h-9 rounded-[4px]"
                  disabled={selection.length === 0}
                  onClick={() => onMove(selection)}
                >
                  <ArrowRightLeft /> Tetapkan atasan
                </Button>
              }
            />
            <div className="min-h-0 flex-1 overflow-auto border-t border-[var(--border-c)]">
              <table className="crown">
                <thead>
                  <tr>
                    <Th className="w-10">
                      <Checkbox
                        aria-label="Pilih semua"
                        checked={
                          selectable.length > 0 && selection.length === selectable.length
                            ? true
                            : selection.length > 0
                              ? "indeterminate"
                              : false
                        }
                        onCheckedChange={(v) => toggleAll(rows, v === true)}
                      />
                    </Th>
                    <Th>Nama</Th>
                    <Th>Username</Th>
                    <Th>Peran</Th>
                    <Th>Domain</Th>
                    <Th>Autentikasi</Th>
                    <Th className="text-right">Aksi</Th>
                  </tr>
                </thead>
                <tbody>
                  {rows.map((u) => {
                    const isLocked = locked.has(u.id);
                    const checked = selection.includes(u.id);
                    return (
                      <tr key={u.id} data-selected={checked}>
                        <td>
                          <Checkbox
                            aria-label={`Pilih ${u.name}`}
                            checked={checked}
                            disabled={isLocked}
                            onCheckedChange={(v) =>
                              setSelection((s) => (v ? [...s, u.id] : s.filter((x) => x !== u.id)))
                            }
                          />
                        </td>
                        <td>
                          <NameCell user={u} state={stateOf(u)} />
                        </td>
                        <td className="mono max-w-[220px] truncate text-[var(--ink-secondary)]" title={u.username}>
                          {u.username}
                        </td>
                        <td>
                          <RolePill role={u.role} />
                        </td>
                        <td className="text-[var(--ink-secondary)]">
                          {DOMAINS[domainOf(u)].label}
                          {u.vendor ? ` · ${VENDORS[u.vendor]}` : ""}
                        </td>
                        <td>
                          <AuthTag auth={u.auth} />
                        </td>
                        <td className="text-right">
                          {isLocked ? (
                            <PendingCell />
                          ) : (
                            <Button
                              size="sm"
                              variant="ghost"
                              className="h-8 rounded-[4px] text-[var(--brand-text)] hover:bg-[var(--brand-tint)] hover:text-[var(--brand-text)]"
                              onClick={() => onMove([u.id])}
                            >
                              Tetapkan atasan
                            </Button>
                          )}
                        </td>
                      </tr>
                    );
                  })}
                </tbody>
              </table>
            </div>
          </>
        )}
      </section>
    );
  }

  /* ── Admin & support ────────────────────────────────────────────────────── */
  if (view.type === "admin") {
    const rows = users.filter((u) => levelOf(u) === "admin");
    return (
      <section className="flex min-h-0 min-w-0 flex-1 flex-col">
        <SectionHead
          eyebrow="Di luar rantai"
          title="Admin & App Support"
          right={
            <div className="max-w-[420px] rounded-[6px] bg-[var(--surface-sunken)] px-4 py-3 text-sm">
              <span className="flex items-center gap-2 font-semibold text-[var(--ink)]">
                <KeyRound className="size-4 text-[var(--ink-secondary)]" /> Kenapa kolomnya kosong
              </span>
              <p className="mt-1 leading-snug text-[var(--ink-secondary)]">
                Role <span className="mono">admin</span> bukan maker dan bukan checker, jadi tidak
                punya atasan maupun plafon persetujuan. Kosong di sini adalah fakta, bukan data yang
                belum diisi.
              </p>
            </div>
          }
        >
          <span>
            <span className="amount">{rows.length}</span> pengguna
          </span>
        </SectionHead>
        <div className="min-h-0 flex-1 overflow-auto border-t border-[var(--border-c)]">
          <table className="crown">
            <thead>
              <tr>
                <Th>Nama</Th>
                <Th>Username</Th>
                <Th>Peran</Th>
                <Th>Domain</Th>
                <Th>Autentikasi</Th>
              </tr>
            </thead>
            <tbody>
              {rows.map((u) => (
                <tr key={u.id}>
                  <td>
                    <NameCell user={u} state={stateOf(u)} />
                  </td>
                  <td className="mono max-w-[220px] truncate text-[var(--ink-secondary)]" title={u.username}>
                          {u.username}
                        </td>
                  <td>
                    <RolePill role={u.role} tone="neutral" />
                  </td>
                  <td className="text-[var(--ink-secondary)]">{DOMAINS[domainOf(u)].label}</td>
                  <td>
                    <AuthTag auth={u.auth} />
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </section>
    );
  }

  /* ── Satu supervisor ────────────────────────────────────────────────────── */
  const spv = users.find((u) => u.id === view.id);
  if (!spv) return null;
  const rows = users
    .filter((u) => u.supervisorId === spv.id)
    .sort((a, b) => a.name.localeCompare(b.name));
  const selectable = rows.filter((r) => !locked.has(r.id));
  const spvLocked = locked.has(spv.id);

  return (
    <section className="flex min-h-0 min-w-0 flex-1 flex-col">
      <SectionHead
        eyebrow={`${DOMAINS[domainOf(spv)].label} · Checker`}
        title={spv.name}
        right={
          <div className="flex w-fit shrink-0 flex-col gap-1.5">
            <span className="label-xs text-[var(--ink-secondary)]">Level persetujuan</span>
            <ToggleGroup
              type="single"
              value={String(spv.tier)}
              onValueChange={(v) => v && !spvLocked && onTier(spv.id, Number(v))}
              className="justify-start gap-1 rounded-[4px] bg-[var(--surface-sunken)] p-1"
            >
              {Object.entries(TIERS).map(([tier, t]) => (
                <ToggleGroupItem
                  key={tier}
                  value={tier}
                  disabled={spvLocked}
                  aria-label={`${t.label} — ${idr(t.ceiling)}`}
                  className={cn(
                    "h-8 rounded-[4px] px-3 text-sm data-[state=on]:bg-[var(--surface)] data-[state=on]:text-[var(--brand-text)] data-[state=on]:shadow-[0_1px_2px_rgba(43,37,35,0.06)]",
                  )}
                >
                  {t.label}
                </ToggleGroupItem>
              ))}
            </ToggleGroup>
            <p className="text-xs text-[var(--ink-secondary)]">
              Plafon per persetujuan{" "}
              {TIERS[spv.tier].ceiling === null ? (
                <span className="font-semibold text-[var(--ink)]">tanpa batas</span>
              ) : (
                <span className="amount text-[var(--ink)]">{idr(TIERS[spv.tier].ceiling)}</span>
              )}
              {" · "}
              {TIERS[spv.tier].note}
            </p>
          </div>
        }
      >
        <span className="mono text-[13px]">{spv.username}</span>
        <RolePill role={spv.role} />
        {spv.vendor ? <Pill tone="neutral">{VENDORS[spv.vendor]}</Pill> : null}
        <span>
          <span className="amount">{rows.length}</span> anak buah
        </span>
        <AuthTag auth={spv.auth} />
      </SectionHead>

      <Toolbar
        left={
          <>
            <Button className="h-9 rounded-[4px]" onClick={onAdd}>
              <UserPlus /> Tambah anak buah
            </Button>
            <span className="pl-1 text-sm text-[var(--ink-secondary)]">
              {selection.length > 0 ? (
                <strong className="font-semibold text-[var(--ink)]">
                  {selection.length} dipilih
                </strong>
              ) : (
                `${rows.length} pengguna`
              )}
            </span>
          </>
        }
        right={
          <>
            <Button
              variant="outline"
              className="h-9 rounded-[4px] border-[var(--border-strong)] bg-[var(--surface)]"
              disabled={selection.length === 0}
              onClick={() => onMove(selection)}
            >
              <ArrowRightLeft /> Pindahkan
            </Button>
            <Button
              variant="outline"
              className="h-9 rounded-[4px] border-[var(--danger)] bg-transparent text-[var(--danger)] hover:bg-[var(--danger-tint)] hover:text-[var(--danger)]"
              disabled={selection.length === 0}
              onClick={() => onDetach(selection)}
            >
              <UserMinus /> Lepas dari {spv.name.split(" ")[0]}
            </Button>
          </>
        }
      />

      {rows.length === 0 ? (
        <Empty className="flex-1 py-20">
          <EmptyHeader>
            <EmptyMedia variant="icon" className="bg-[var(--surface-sunken)]">
              <UserPlus className="text-[var(--ink-secondary)]" />
            </EmptyMedia>
            <EmptyTitle>Belum ada anak buah</EmptyTitle>
            <EmptyDescription>
              {spv.name} belum menjadi atasan siapa pun, jadi tidak ada pengajuan yang masuk ke
              antriannya. Tambahkan maker {DOMAINS[domainOf(spv)].label}
              {spv.vendor ? ` dari ${VENDORS[spv.vendor]}` : ""} lewat “Tambah anak buah”.
            </EmptyDescription>
          </EmptyHeader>
        </Empty>
      ) : (
        <div className="min-h-0 flex-1 overflow-auto border-t border-[var(--border-c)]">
          <table className="crown">
            <thead>
              <tr>
                <Th className="w-10">
                  <Checkbox
                    aria-label="Pilih semua"
                    checked={
                      selectable.length > 0 && selection.length === selectable.length
                        ? true
                        : selection.length > 0
                          ? "indeterminate"
                          : false
                    }
                    onCheckedChange={(v) => toggleAll(rows, v === true)}
                  />
                </Th>
                <Th>Nama</Th>
                <Th>Username</Th>
                <Th>Peran</Th>
                <Th>Level persetujuan</Th>
                <Th>Autentikasi</Th>
                <Th className="text-right">Aksi</Th>
              </tr>
            </thead>
            <tbody>
              {rows.map((u) => {
                const isLocked = locked.has(u.id);
                const checked = selection.includes(u.id);
                return (
                  <tr key={u.id} data-selected={checked}>
                    <td>
                      <Checkbox
                        aria-label={`Pilih ${u.name}`}
                        checked={checked}
                        disabled={isLocked}
                        onCheckedChange={(v) =>
                          setSelection((s) => (v ? [...s, u.id] : s.filter((x) => x !== u.id)))
                        }
                      />
                    </td>
                    <td>
                      <NameCell user={u} state={stateOf(u)} />
                    </td>
                    <td className="mono max-w-[220px] truncate text-[var(--ink-secondary)]" title={u.username}>
                          {u.username}
                        </td>
                    <td>
                      <RolePill role={u.role} />
                    </td>
                    <td>
                      <TierCell user={u} />
                    </td>
                    <td>
                      <AuthTag auth={u.auth} />
                    </td>
                    <td className="whitespace-nowrap text-right">
                      {isLocked ? (
                        <PendingCell />
                      ) : (
                        <span className="inline-flex items-center gap-1">
                          <Button
                            size="sm"
                            variant="ghost"
                            className="h-8 rounded-[4px] text-[var(--ink-secondary)] hover:bg-[var(--surface-sunken)] hover:text-[var(--ink)]"
                            onClick={() => onMove([u.id])}
                          >
                            Pindahkan
                          </Button>
                          <Button
                            size="sm"
                            variant="ghost"
                            className="h-8 rounded-[4px] text-[var(--ink-secondary)] hover:bg-[var(--danger-tint)] hover:text-[var(--danger)]"
                            onClick={() => onDetach([u.id])}
                          >
                            Lepas
                          </Button>
                        </span>
                      )}
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        </div>
      )}
    </section>
  );
}
