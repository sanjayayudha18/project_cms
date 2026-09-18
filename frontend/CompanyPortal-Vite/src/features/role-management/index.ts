export { CreateRoleDialog } from "./components/CreateRoleDialog";
export { PermissionEditor } from "./components/PermissionEditor";
export { RoleManagementPage } from "./components/RoleManagementPage";
export {
  roleKeys,
  useCatalog,
  useCreateRole,
  useRoles,
  useUpdateRolePermissions,
} from "./hooks/useRoleQueries";
export { createRoleSchema, permissionsSchema } from "./types";
export type {
  CatalogEntry,
  CatalogResponse,
  CreatedRole,
  CreateRoleFormValues,
  GrantedPermission,
  PermissionsFormValues,
  RolesResponse,
  RoleWithPermissions,
  UpdateRolePermissionsResult,
} from "./types";
