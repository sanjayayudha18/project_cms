import { describe, expect, it } from "vitest";
import { type UserFormValues, buildUserFormSchema } from "./userFormSchema";

function baseValues(overrides: Partial<UserFormValues> = {}): UserFormValues {
  return {
    username: "budi.santoso",
    full_name: "Budi Santoso",
    email: "budi@example.com",
    role: "APPACCESS",
    is_karyawan: true,
    auth_source: "ldap",
    temporary_password: "",
    employee_id: "EMP001",
    vendor_id: null,
    vendor_branch_id: null,
    supervisor_id: null,
    approval_level: null,
    ...overrides,
  };
}

describe("buildUserFormSchema", () => {
  it("create mode: local requires vendor_id and temporary_password", () => {
    const schema = buildUserFormSchema("create");
    const result = schema.safeParse(baseValues({ auth_source: "local" }));
    expect(result.success).toBe(false);
    if (!result.success) {
      const paths = result.error.issues.map((i) => i.path.join("."));
      expect(paths).toContain("vendor_id");
      expect(paths).toContain("temporary_password");
    }
  });

  it("create mode: local with vendor_id + temporary_password passes", () => {
    const schema = buildUserFormSchema("create");
    const result = schema.safeParse(
      baseValues({ auth_source: "local", vendor_id: 1, temporary_password: "Str0ngP@ss!" }),
    );
    expect(result.success).toBe(true);
  });

  it("create mode: ldap forbids vendor_id and temporary_password", () => {
    const schema = buildUserFormSchema("create");
    const result = schema.safeParse(
      baseValues({ auth_source: "ldap", vendor_id: 1, temporary_password: "Str0ngP@ss!" }),
    );
    expect(result.success).toBe(false);
    if (!result.success) {
      const paths = result.error.issues.map((i) => i.path.join("."));
      expect(paths).toContain("vendor_id");
      expect(paths).toContain("temporary_password");
    }
  });

  it("create mode: ldap with both absent passes", () => {
    const schema = buildUserFormSchema("create");
    const result = schema.safeParse(baseValues({ auth_source: "ldap" }));
    expect(result.success).toBe(true);
  });

  it("edit mode: local requires vendor_id but NOT temporary_password", () => {
    const schema = buildUserFormSchema("edit");
    const result = schema.safeParse(
      baseValues({ auth_source: "local", vendor_id: 1, temporary_password: "" }),
    );
    expect(result.success).toBe(true);
  });

  it.each(["VENDOR-USER", "VENDOR-ATM-USER", "VENDOR-VAULT-USER"])(
    "vendor role %s passes with local auth, a vendor and an optional branch",
    (role) => {
      const schema = buildUserFormSchema("edit");
      const base = { role, auth_source: "local" as const, vendor_id: 1 };
      expect(schema.safeParse(baseValues(base)).success).toBe(true);
      expect(schema.safeParse(baseValues({ ...base, vendor_branch_id: 10 })).success).toBe(true);
    },
  );

  it("vendor role with ldap auth_source fails on auth_source", () => {
    const result = buildUserFormSchema("edit").safeParse(
      baseValues({ role: "VENDOR-ATM-USER", auth_source: "ldap" }),
    );
    expect(result.success).toBe(false);
    if (!result.success) {
      expect(result.error.issues.map((i) => i.path.join("."))).toContain("auth_source");
    }
  });

  it("a branch is rejected for a non-vendor role", () => {
    const result = buildUserFormSchema("edit").safeParse(
      baseValues({ role: "ATM-USER", vendor_branch_id: 10 }),
    );
    expect(result.success).toBe(false);
    if (!result.success) {
      expect(result.error.issues.map((i) => i.path.join("."))).toContain("vendor_branch_id");
    }
  });

  it("edit mode: local without vendor_id still fails", () => {
    const schema = buildUserFormSchema("edit");
    const result = schema.safeParse(baseValues({ auth_source: "local", vendor_id: null }));
    expect(result.success).toBe(false);
  });
});
