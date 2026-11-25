import { Button } from "@/components/ui/Button";
import { ALWAYS_ON_KEYS, LOCKED_KEYS, useAuthStore, usePermissionStore } from "@/lib/auth";
import { useToast } from "@/lib/hooks/useToast";
import { useEffect, useMemo, useState } from "react";
import { useUpdateRolePermissions } from "../hooks/useRoleQueries";
import type { CatalogEntry, RoleWithPermissions } from "../types";

interface PermissionEditorProps {
  role: RoleWithPermissions;
  /** Only the active portal tab's catalog — the page filters before passing it. */
  catalog: CatalogEntry[];
  /** Reports unsaved changes so the page can confirm before switching roles/tabs. */
  onDirtyChange?: (isDirty: boolean) => void;
}

interface MenuNode {
  menu: CatalogEntry;
  features: CatalogEntry[];
}

const ALWAYS_ON = new Set<string>(ALWAYS_ON_KEYS);
const LOCKED = new Set<string>(LOCKED_KEYS);

type Fixed = "always-on" | "locked" | null;

function fixedKind(entry: CatalogEntry): Fixed {
  if (ALWAYS_ON.has(entry.key)) return "always-on";
  if (LOCKED.has(entry.key)) return "locked";
  return null;
}

function errorMessage(err: unknown): string {
  if (err && typeof err === "object") {
    const e = err as { code?: string; message?: unknown };
    if (e.code === "role_not_assignable") {
      return `Izin tidak dapat diberikan ke peran ini. ${String(e.message ?? "")}`.trim();
    }
    if ("message" in e) return String(e.message);
  }
  return "Terjadi kesalahan yang tidak diketahui";
}

function buildTree(catalog: CatalogEntry[]): MenuNode[] {
  const menus = catalog
    .filter((entry) => entry.kind === "menu")
    .sort((a, b) => a.sort_order - b.sort_order);
  return menus.map((menu) => ({
    menu,
    features: catalog
      .filter((entry) => entry.kind === "feature" && entry.parent_id === menu.id)
      .sort((a, b) => a.sort_order - b.sort_order),
  }));
}

/**
 * Menu→feature toggle tree for a selected role (Req 3.1, 3.5, 7.2). Submits
 * the full selected set on "Simpan" — a full-set replace, not incremental
 * grants/revokes (design property 5). No maker-checker UI: the change
 * applies immediately once saved (documented Golden Rule #3 deviation).
 */
export function PermissionEditor({ role, catalog, onDirtyChange }: PermissionEditorProps) {
  const [saveError, setSaveError] = useState<string | null>(null);
  const { toast } = useToast();
  const updateMutation = useUpdateRolePermissions();
  const tree = useMemo(() => buildTree(catalog), [catalog]);
  const ownRole = useAuthStore((s) => s.user?.role);
  // Always_On/Locked keys are never sent (backend keeps them as stored) — the dirty check ignores them too.
  const fixedIds = useMemo(
    () => new Set(catalog.filter((c) => fixedKind(c) !== null).map((c) => c.id)),
    [catalog],
  );

  const [selected, setSelected] = useState<Set<number>>(
    () => new Set(role.permissions.map((p) => p.menu_feature_id)),
  );

  // Re-seed selection whenever the selected role changes (switching rows).
  useEffect(() => {
    setSelected(new Set(role.permissions.map((p) => p.menu_feature_id)));
  }, [role.permissions]);

  const isDirty = useMemo(() => {
    const current = new Set(
      role.permissions.map((p) => p.menu_feature_id).filter((id) => !fixedIds.has(id)),
    );
    const next = Array.from(selected).filter((id) => !fixedIds.has(id));
    if (current.size !== next.length) return true;
    return next.some((id) => !current.has(id));
  }, [role.permissions, selected, fixedIds]);

  useEffect(() => {
    onDirtyChange?.(isDirty);
  }, [isDirty, onDirtyChange]);

  function handleCancel(): void {
    setSelected(new Set(role.permissions.map((p) => p.menu_feature_id)));
    setSaveError(null);
  }

  function toggle(id: number): void {
    setSelected((prev) => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
  }

  async function handleSave(): Promise<void> {
    setSaveError(null);
    try {
      const menuFeatureIds = Array.from(selected).filter((id) => !fixedIds.has(id));
      await updateMutation.mutateAsync({ id: role.id, menuFeatureIds });
      toast({ type: "success", message: `Izin peran "${role.role}" berhasil diperbarui` });
      if (role.role === ownRole) await usePermissionStore.getState().load();
    } catch (err) {
      const message = errorMessage(err);
      setSaveError(message);
      toast({ type: "error", message });
    }
  }

  return (
    <div className="flex flex-col gap-4">
      <div className="flex flex-col gap-3 max-h-[420px] overflow-y-auto rounded-[var(--radius-md)] border border-[var(--n-200)] p-4">
        {tree.map(({ menu, features }) => (
          <div key={menu.id} className="flex flex-col gap-1.5">
            <EntryRow
              entry={menu}
              role={role.role}
              checked={selected.has(menu.id)}
              onToggle={toggle}
              strong
            />
            {features.length > 0 && (
              <div className="ml-6 flex flex-col gap-1.5">
                {features.map((feature) => (
                  <EntryRow
                    key={feature.id}
                    entry={feature}
                    role={role.role}
                    checked={selected.has(feature.id)}
                    onToggle={toggle}
                  />
                ))}
              </div>
            )}
          </div>
        ))}
      </div>

      {saveError && (
        <p role="alert" className="text-sm text-[var(--danger-fg)]">
          {saveError}
        </p>
      )}

      <div className="flex justify-end gap-3">
        <Button
          variant="secondary"
          disabled={!isDirty || updateMutation.isPending}
          onClick={handleCancel}
        >
          Batalkan
        </Button>
        <Button disabled={!isDirty || updateMutation.isPending} onClick={handleSave}>
          Simpan Izin
        </Button>
      </div>
    </div>
  );
}

interface EntryRowProps {
  entry: CatalogEntry;
  role: string;
  checked: boolean;
  onToggle: (id: number) => void;
  strong?: boolean;
}

/** One checkbox row; fixed/restricted entries are disabled and say why in text (not colour alone). */
function EntryRow({ entry, role, checked, onToggle, strong }: EntryRowProps) {
  const fixed = fixedKind(entry);
  const allowed = entry.assignable_roles ?? null;
  const outsideList = fixed === null && allowed !== null && !allowed.includes(role);
  // A legacy grant outside the list stays revocable; only new grants are blocked.
  const disabled = fixed !== null || (outsideList && !checked);
  const note =
    fixed === "always-on"
      ? "Selalu aktif"
      : fixed === "locked"
        ? "Hanya APPACCESS/ADMIN"
        : outsideList
          ? `Hanya: ${(allowed ?? []).join(", ")}`
          : null;

  return (
    <label
      title={outsideList ? `Hanya dapat diberikan ke: ${(allowed ?? []).join(", ")}` : undefined}
      className={`flex items-center gap-2 text-sm ${
        strong ? "font-medium text-[var(--n-800)]" : "text-[var(--n-700)]"
      } ${disabled ? "cursor-not-allowed opacity-80" : ""}`}
    >
      <input
        type="checkbox"
        checked={fixed === "always-on" ? true : checked}
        disabled={disabled}
        onChange={() => onToggle(entry.id)}
        className="size-4 accent-[var(--red-500)] focus-visible:ring-2 focus-visible:ring-[var(--red-100)]"
      />
      {entry.label}
      {note && (
        <span className="rounded-[var(--radius-sm)] border border-[var(--n-200)] bg-[var(--n-50)] px-1.5 py-0.5 text-xs font-normal text-[var(--n-600)]">
          {note}
        </span>
      )}
    </label>
  );
}
