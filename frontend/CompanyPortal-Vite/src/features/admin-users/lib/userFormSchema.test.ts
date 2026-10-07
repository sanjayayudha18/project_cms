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

  it("edit mode: local without vendor_id still fails", () => {
    const schema = buildUserFormSchema("edit");
    const result = schema.safeParse(baseValues({ auth_source: "local", vendor_id: null }));
    expect(result.success).toBe(false);
  });
});
