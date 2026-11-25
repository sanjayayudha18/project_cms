import { BackLink } from "@/components/ui/BackLink";
import { Button } from "@/components/ui/Button";
import { Dialog } from "@/components/ui/Dialog";
import { PageHeader } from "@/components/ui/PageHeader";
import { RoleBadge, StatusMessage } from "@/features/rbac-settings";
import { Package, Search, Vault, X } from "lucide-react";
import { useRef, useState } from "react";
import { useCatalogForPortal, useRoles } from "../hooks/useRoleQueries";
import type { PartyScope, Portal, RoleWithPermissions } from "../types";
import { useRoleManagementUrlState } from "../useRoleManagementUrlState";
import { CreateRoleDialog } from "./CreateRoleDialog";
import { PermissionEditor } from "./PermissionEditor";
import { PortalTabs, panelId, tabId } from "./PortalTabs";

const SCOPE_CHIP: Record<PartyScope, { label: string; Icon: typeof Package }> = {
  replenish: { label: "Replenish", Icon: Package },
  vault: { label: "Vault", Icon: Vault },
};

const PORTAL_LABEL: Record<Portal, string> = { main: "Portal Utama", vendor: "Portal Cabang" };

/** Case-insensitive match on name or description (Req 6.2). */
function matchesQuery(role: RoleWithPermissions, q: string): boolean {
  const needle = q.trim().toLowerCase();
  if (!needle) return true;
  return (
    role.role.toLowerCase().includes(needle) ||
    (role.description ?? "").toLowerCase().includes(needle)
  );
}

function ScopeChips({ scope }: { scope: PartyScope[] | null }) {
  if (!scope || scope.length === 0) return null;
  return (
    <>
      {scope.map((s) => {
        const { label, Icon } = SCOPE_CHIP[s];
        return (
          <span
            key={s}
            className="inline-flex items-center gap-1 rounded-full border border-[var(--n-200)] bg-[var(--n-50)] px-2 py-0.5 text-xs text-[var(--n-700)]"
          >
            <Icon className="h-3 w-3" aria-hidden="true" />
            {label}
          </span>
        );
      })}
    </>
  );
}

interface RoleRowProps {
  role: RoleWithPermissions;
  selected: boolean;
  onSelect: (id: number) => void;
}

function RoleRow({ role, selected, onSelect }: RoleRowProps) {
  const description = role.description?.trim() || "Belum ada deskripsi";
  return (
    <li>
      <button
        type="button"
        aria-current={selected ? "true" : undefined}
        onClick={() => onSelect(role.id)}
        className={`flex min-h-[44px] w-full flex-col gap-1.5 border-l-2 px-4 py-3 text-left outline-none transition-colors focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-[var(--red-100)] ${
          selected
            ? "border-[var(--red-500)] bg-[var(--red-50)]"
            : "border-transparent hover:bg-[var(--n-50)]"
        }`}
      >
        <span className="flex items-center justify-between gap-2">
          <RoleBadge role={role.role} />
          <span className="text-xs tabular-nums text-[var(--n-600)]">
            {role.active_user_count} user
          </span>
        </span>
        <span title={description} className="line-clamp-2 text-sm text-[var(--n-700)]">
          {description}
        </span>
        {role.party_scope && role.party_scope.length > 0 && (
          <span className="flex flex-wrap gap-1">
            <ScopeChips scope={role.party_scope} />
          </span>
        )}
      </button>
    </li>
  );
}

/**
 * Manajemen Peran (Req 5, 6): Portal Utama / Portal Cabang tabs, master-detail,
 * client-side search. State lives in the URL (?portal&role&q). Merah Sirih: red
 * only on the "Buat Peran" action, the active tab/row marker and focus halos.
 */
export function RoleManagementPage() {
  const { params, setParams } = useRoleManagementUrlState();
  const rolesQuery = useRoles();
  const catalogQuery = useCatalogForPortal(params.portal);
  const [isCreateOpen, setCreateOpen] = useState(false);
  const detailHeadingRef = useRef<HTMLHeadingElement>(null);
  const [isEditorDirty, setEditorDirty] = useState(false);
  // Navigation held back until the operator confirms discarding unsaved permission changes.
  const [pendingNav, setPendingNav] = useState<(() => void) | null>(null);

  function guardUnsaved(action: () => void) {
    if (isEditorDirty) setPendingNav(() => action);
    else action();
  }

  const allRoles = rolesQuery.data?.roles ?? [];
  const tabRoles = allRoles.filter((r) => r.portal === params.portal);
  const counts: Record<Portal, number> = {
    main: allRoles.filter((r) => r.portal === "main").length,
    vendor: allRoles.filter((r) => r.portal === "vendor").length,
  };
  const visibleRoles = tabRoles.filter((r) => matchesQuery(r, params.q));
  // First role is selected automatically when the URL names none (Req 6.1).
  const selectedRole = tabRoles.find((r) => r.id === params.role) ?? visibleRoles[0] ?? null;
  const catalog = catalogQuery.data ?? [];

  const isLoading = rolesQuery.isLoading || catalogQuery.isLoading;
  const isError = rolesQuery.isError || catalogQuery.isError;
  const hasData = !isLoading && !isError;

  function handleSelect(id: number) {
    if (id === selectedRole?.id) return;
    guardUnsaved(() => selectRole(id));
  }

  function selectRole(id: number) {
    setParams({ role: id });
    // Below lg the detail is stacked under the list: bring it into view and focus its title.
    const isStacked = !window.matchMedia?.("(min-width: 1024px)").matches;
    if (isStacked) {
      detailHeadingRef.current?.focus();
      detailHeadingRef.current?.scrollIntoView?.({ block: "start" });
    }
  }

  function retry() {
    rolesQuery.refetch();
    catalogQuery.refetch();
  }

  return (
    <div className="flex flex-col gap-6 p-6">
      <BackLink to="/settings" label="Kembali ke Pengaturan" />
      <PageHeader
        eyebrow="Pengaturan"
        title="Manajemen Peran"
        description="Kelola peran dan atur menu/fitur yang dapat diakses tiap peran."
        actions={<Button onClick={() => setCreateOpen(true)}>Buat Peran</Button>}
      />

      <PortalTabs
        value={params.portal}
        counts={counts}
        onChange={(portal) => guardUnsaved(() => setParams({ portal, role: null, q: "" }))}
      />

      <div
        role="tabpanel"
        id={panelId(params.portal)}
        aria-labelledby={tabId(params.portal)}
        className="flex flex-col gap-4"
      >
        <StatusMessage
          isLoading={isLoading}
          isError={isError}
          isEmpty={hasData && tabRoles.length === 0}
          loadingLabel="Memuat daftar peran…"
          errorMessage="Gagal memuat daftar peran. Coba lagi."
          emptyMessage={`Belum ada peran di ${PORTAL_LABEL[params.portal]}.`}
        />
        {isError && (
          <div>
            <Button variant="secondary" onClick={retry}>
              Coba lagi
            </Button>
          </div>
        )}

        {hasData && tabRoles.length > 0 && (
          <div className="grid grid-cols-1 gap-6 lg:grid-cols-[minmax(0,360px)_minmax(0,1fr)]">
            <section aria-label="Daftar peran" className="flex flex-col gap-3">
              <div className="relative">
                <Search
                  className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-[var(--n-500)]"
                  aria-hidden="true"
                />
                <input
                  type="search"
                  aria-label="Cari peran"
                  placeholder="Cari nama atau deskripsi"
                  value={params.q}
                  onChange={(e) => setParams({ q: e.target.value })}
                  className="min-h-[44px] w-full rounded-[var(--radius-md)] border border-[var(--n-300)] bg-[var(--n-0)] pl-9 pr-11 text-sm outline-none focus-visible:border-[var(--red-400)] focus-visible:ring-2 focus-visible:ring-[var(--red-100)]"
                />
                {params.q && (
                  <button
                    type="button"
                    aria-label="Bersihkan pencarian"
                    onClick={() => setParams({ q: "" })}
                    className="absolute right-0 top-0 inline-flex h-[44px] w-[44px] items-center justify-center text-[var(--n-600)] outline-none focus-visible:ring-2 focus-visible:ring-[var(--red-100)]"
                  >
                    <X className="h-4 w-4" aria-hidden="true" />
                  </button>
                )}
              </div>

              {visibleRoles.length === 0 ? (
                <div className="flex flex-col items-start gap-3 rounded-[var(--radius-md)] border border-[var(--n-200)] p-4">
                  <output className="text-sm text-[var(--n-700)]">
                    Tidak ada peran yang cocok dengan “{params.q}”.
                  </output>
                  <Button variant="secondary" onClick={() => setParams({ q: "" })}>
                    Hapus pencarian
                  </Button>
                </div>
              ) : (
                <ul className="divide-y divide-[var(--n-200)] overflow-hidden rounded-[var(--radius-md)] border border-[var(--n-200)] bg-[var(--n-0)]">
                  {visibleRoles.map((role) => (
                    <RoleRow
                      key={role.id}
                      role={role}
                      selected={role.id === selectedRole?.id}
                      onSelect={handleSelect}
                    />
                  ))}
                </ul>
              )}
            </section>

            <section aria-label="Detail peran" className="flex min-w-0 flex-col gap-3">
              {selectedRole ? (
                <>
                  <div className="flex flex-col gap-1">
                    <h2
                      ref={detailHeadingRef}
                      tabIndex={-1}
                      className="text-lg font-semibold text-[var(--n-900)] focus-visible:ring-2 focus-visible:ring-[var(--red-100)]"
                    >
                      {selectedRole.role}
                    </h2>
                    <p className="text-sm text-[var(--n-700)]">
                      {selectedRole.description?.trim() || "Belum ada deskripsi"}
                    </p>
                    <div className="flex flex-wrap items-center gap-2 text-xs text-[var(--n-600)]">
                      <span>{PORTAL_LABEL[selectedRole.portal]}</span>
                      <span aria-hidden="true">·</span>
                      <span className="tabular-nums">
                        {selectedRole.active_user_count} user aktif
                      </span>
                      <ScopeChips scope={selectedRole.party_scope} />
                    </div>
                  </div>
                  <PermissionEditor
                    key={selectedRole.id}
                    role={selectedRole}
                    catalog={catalog}
                    onDirtyChange={setEditorDirty}
                  />
                </>
              ) : (
                <p className="text-sm text-[var(--n-600)]">Pilih peran untuk mengatur izin.</p>
              )}
            </section>
          </div>
        )}
      </div>

      <CreateRoleDialog
        open={isCreateOpen}
        onClose={() => setCreateOpen(false)}
        portal={params.portal}
      />

      <Dialog
        open={pendingNav !== null}
        onClose={() => setPendingNav(null)}
        title="Buang perubahan?"
      >
        <div className="flex flex-col gap-4">
          <p className="text-sm text-[var(--n-700)]">
            Perubahan izin pada peran ini belum disimpan. Jika pindah sekarang, perubahan akan
            hilang.
          </p>
          <div className="flex justify-end gap-3">
            <Button variant="secondary" onClick={() => setPendingNav(null)}>
              Tetap di sini
            </Button>
            <Button
              onClick={() => {
                pendingNav?.();
                setPendingNav(null);
              }}
            >
              Buang perubahan
            </Button>
          </div>
        </div>
      </Dialog>
    </div>
  );
}
