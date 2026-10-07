import { adminUserKeys } from "@/features/admin-users/hooks";
import { adminVendorKeys } from "@/features/admin-vendors/hooks";
import { describe, expect, it } from "vitest";
import { adminATMsKeys } from "../hooks";

describe("admin-atms query keys", () => {
  it("list key is deep-equal for identical params", () => {
    const params = { page: 1, page_size: 25, status: "active" as const };
    expect(adminATMsKeys.list(params)).toEqual(adminATMsKeys.list({ ...params }));
  });

  it("list key differs across different params", () => {
    expect(adminATMsKeys.list({ page: 1, page_size: 25 })).not.toEqual(
      adminATMsKeys.list({ page: 2, page_size: 25 }),
    );
  });

  it("detail key differs across different ids", () => {
    expect(adminATMsKeys.detail(1)).not.toEqual(adminATMsKeys.detail(2));
  });

  it("locations key is stable and distinct from list/detail", () => {
    expect(adminATMsKeys.locations()).toEqual(adminATMsKeys.locations());
    expect(adminATMsKeys.locations()).not.toEqual(adminATMsKeys.list({ page: 1, page_size: 25 }));
    expect(adminATMsKeys.locations()).not.toEqual(adminATMsKeys.detail(1));
  });

  it("admin-atms keys never collide with admin-users or admin-vendors keys", () => {
    const params = { page: 1, page_size: 25 };
    expect(adminATMsKeys.list(params)).not.toEqual(adminUserKeys.list(params));
    expect(adminATMsKeys.list(params)).not.toEqual(adminVendorKeys.list(params));
    expect(adminATMsKeys.detail(1)).not.toEqual(adminUserKeys.detail(1));
    expect(adminATMsKeys.detail(1)).not.toEqual(adminVendorKeys.detail(1));
    expect(adminATMsKeys.all).not.toEqual(adminUserKeys.all);
    expect(adminATMsKeys.all).not.toEqual(adminVendorKeys.all);
  });
});
