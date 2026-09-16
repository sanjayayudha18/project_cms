import { adminVendorKeys } from "@/features/admin-vendors/hooks";
import { describe, expect, it } from "vitest";
import { adminUserKeys } from "../hooks";

describe("admin-users/admin-vendors query keys", () => {
  it("list key is deep-equal for identical params", () => {
    const params = { page: 1, page_size: 25 };
    expect(adminUserKeys.list(params)).toEqual(adminUserKeys.list({ ...params }));
  });

  it("list key differs across different params", () => {
    expect(adminUserKeys.list({ page: 1, page_size: 25 })).not.toEqual(
      adminUserKeys.list({ page: 2, page_size: 25 }),
    );
  });

  it("detail key differs across different ids", () => {
    expect(adminUserKeys.detail(1)).not.toEqual(adminUserKeys.detail(2));
  });

  it("admin-users and admin-vendors keys never collide", () => {
    const params = { page: 1, page_size: 25 };
    expect(adminUserKeys.list(params)).not.toEqual(adminVendorKeys.list(params));
    expect(adminUserKeys.detail(1)).not.toEqual(adminVendorKeys.detail(1));
    expect(adminUserKeys.all).not.toEqual(adminVendorKeys.all);
  });
});
