/** Shared GET /admin/roles fixture for tests that render the user form / filter bar. */
const role = (id: number, name: string, portal: "main" | "vendor" | "service") => ({
  id,
  role: name,
  description: null,
  portal,
  party_scope: portal === "vendor" ? ["replenish"] : null,
  active_user_count: 0,
  permissions: [],
});

export const rolesFixture = {
  roles: [
    role(1, "ADMIN", "main"),
    role(2, "ATM-USER", "main"),
    role(3, "VENDOR-USER", "vendor"),
    role(4, "VENDOR-ATM-USER", "vendor"),
    role(5, "VENDOR-VAULT-USER", "vendor"),
    role(6, "VENDOR-NEW-USER", "vendor"),
    role(7, "EXT-CLIENT", "service"),
  ],
};

export const roleManagementMock = () => ({
  useRoles: () => ({ data: rolesFixture, isLoading: false, isError: false }),
});
