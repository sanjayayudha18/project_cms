import { MASTER_DATA_IO_SEARCH_SCHEMA } from "@/features/master-data-io/searchSchema";
import { describe, expect, it } from "vitest";

/**
 * Search schema for /settings/admin/master-data-io. The route guard (key
 * `settings.master-data-io`) is covered by lib/auth/permissionCoverage.test.ts.
 */
describe("MASTER_DATA_IO_SEARCH_SCHEMA", () => {
  it("accepts every known param", () => {
    const input = {
      tab: "history",
      batch: 12,
      h_entity: "vendor-pics",
      h_status: "stale",
      h_mine: true,
      h_page: 2,
    };
    expect(MASTER_DATA_IO_SEARCH_SCHEMA.parse(input)).toEqual(input);
  });

  it("drops unknown keys", () => {
    expect(MASTER_DATA_IO_SEARCH_SCHEMA.parse({ tab: "new", evil: "x" })).toEqual({ tab: "new" });
  });

  it.each([
    { tab: "other" },
    { batch: 0 },
    { h_entity: "users" },
    { h_status: "done" },
    { h_page: 0 },
  ])("rejects invalid value %j", (input) => {
    expect(MASTER_DATA_IO_SEARCH_SCHEMA.safeParse(input).success).toBe(false);
  });
});
