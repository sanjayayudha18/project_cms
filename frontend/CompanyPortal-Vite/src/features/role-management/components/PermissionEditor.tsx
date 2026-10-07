import { Button } from "@/components/ui/Button";
import { useToast } from "@/lib/hooks/useToast";
import { useEffect, useMemo, useState } from "react";
import { useUpdateRolePermissions } from "../hooks/useRoleQueries";
import type { CatalogEntry, RoleWithPermissions } from "../types";

interface PermissionEditorProps {
  role: RoleWithPermissions;
  catalog: CatalogEntry[];
}

interface MenuNode {
  menu: CatalogEntry;
  features: CatalogEntry[];
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
export function PermissionEditor({ role, catalog }: PermissionEditorProps) {
  const { toast } = useToast();
  const updateMutation = useUpdateRolePermissions();
  const tree = useMemo(() => buildTree(catalog), [catalog]);

  const [selected, setSelected] = useState<Set<number>>(
    () => new Set(role.permissions.map((p) => p.menu_feature_id)),
  );

  // Re-seed selection whenever the selected role changes (switching rows).
  useEffect(() => {
    setSelected(new Set(role.permissions.map((p) => p.menu_feature_id)));
  }, [role.permissions]);

  const isDirty = useMemo(() => {
    const current = new Set(role.permissions.map((p) => p.menu_feature_id));
    if (current.size !== selected.size) return true;
    for (const id of selected) {
      if (!current.has(id)) return true;
    }
    return false;
  }, [role.permissions, selected]);

  function toggle(id: number): void {
    setSelected((prev) => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
  }

  async function handleSave(): Promise<void> {
    try {
      await updateMutation.mutateAsync({ id: role.id, menuFeatureIds: Array.from(selected) });
      toast({ type: "success", message: `Izin peran "${role.role}" berhasil diperbarui` });
    } catch (err) {
      const message =
        err && typeof err === "object" && "message" in err
          ? String((err as { message: unknown }).message)
          : "Terjadi kesalahan yang tidak diketahui";
      toast({ type: "error", message });
    }
  }

  return (
    <div className="flex flex-col gap-4">
      <div className="flex flex-col gap-3 max-h-[420px] overflow-y-auto rounded-[var(--radius-md)] border border-[var(--n-200)] p-4">
        {tree.map(({ menu, features }) => (
          <div key={menu.id} className="flex flex-col gap-1.5">
            <label className="flex items-center gap-2 text-sm font-medium text-[var(--n-800)]">
              <input
                type="checkbox"
                checked={selected.has(menu.id)}
                onChange={() => toggle(menu.id)}
                className="size-4 accent-[var(--red-500)] focus-visible:ring-2 focus-visible:ring-[var(--red-100)]"
              />
              {menu.label}
            </label>
            {features.length > 0 && (
              <div className="ml-6 flex flex-col gap-1.5">
                {features.map((feature) => (
                  <label
                    key={feature.id}
                    className="flex items-center gap-2 text-sm text-[var(--n-700)]"
                  >
                    <input
                      type="checkbox"
                      checked={selected.has(feature.id)}
                      onChange={() => toggle(feature.id)}
                      className="size-4 accent-[var(--red-500)] focus-visible:ring-2 focus-visible:ring-[var(--red-100)]"
                    />
                    {feature.label}
                  </label>
                ))}
              </div>
            )}
          </div>
        ))}
      </div>

      <div className="flex justify-end">
        <Button disabled={!isDirty || updateMutation.isPending} onClick={handleSave}>
          Simpan Izin
        </Button>
      </div>
    </div>
  );
}
