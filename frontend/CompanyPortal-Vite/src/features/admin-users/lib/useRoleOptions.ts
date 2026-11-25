import { useRoles } from "@/features/role-management";
import type { Portal, RoleWithPermissions } from "@/features/role-management";

export interface RoleOption {
  value: string;
  label: string;
}

export function toRoleOption(r: RoleWithPermissions): RoleOption {
  return { value: r.role, label: r.description ? `${r.role} — ${r.description}` : r.role };
}

/**
 * Role choices from GET /admin/roles (DQ3), limited to one portal: vendor users
 * get `vendor` roles, internal users `main` roles. `service` roles never show.
 * Pass `null` for every non-service role (list filter). `keep` stays selectable
 * even when absent, so editing a user never blanks an existing role.
 */
export function useRoleOptions(portal: Portal | null, keep?: string) {
  const query = useRoles();
  const roles = (query.data?.roles ?? []).filter(
    (r) => (portal ? r.portal === portal : (r.portal as string) !== "service"), // API may also carry "service" (EXT-CLIENT)
  );
  const options = roles.map(toRoleOption);
  if (keep && !options.some((o) => o.value === keep)) options.push({ value: keep, label: keep });
  return { options, isLoading: query.isLoading, isError: query.isError };
}
