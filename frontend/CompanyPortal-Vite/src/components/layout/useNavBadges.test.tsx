// Feature: app-shell-revamp, Property 9: Badge hanya untuk item terlihat dan nilai positif
import { useInbox } from "@/features/approvals/hooks";
import { useEodSummary } from "@/features/eod-monitoring/hooks/useEodQueries";
import { useVendorRequests } from "@/features/vendor-request/hooks";
import type { DbRole } from "@/lib/auth/store";
import { getVisibleNav } from "@/lib/config/navigation";
import { formatBadgeCount } from "@/lib/utils/formatters";
import * as fc from "fast-check";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { badgeRequestStatuses, useNavBadges } from "./useNavBadges";

vi.mock("@/features/approvals/hooks", () => ({ useInbox: vi.fn() }));
vi.mock("@/features/eod-monitoring/hooks/useEodQueries", () => ({ useEodSummary: vi.fn() }));
vi.mock("@/features/vendor-request/hooks", () => ({ useVendorRequests: vi.fn() }));

const ROLES: (DbRole | null)[] = [
  null,
  "ADMIN",
  "ADMIN_PARAM",
  "APPACCESS",
  "ATM-USER",
  "ATM-SPV",
  "BRANCH-USER",
  "BRANCH-SPV",
  "BRANCH-ATM-USER",
  "BRANCH-ATM-SPV",
  "VENDOR-USER",
  "VENDOR-ATM-USER",
  "VENDOR-VAULT-USER",
  "ACM-USER",
  "ACM-SPV",
];

type Counts = { failed: number; max_retries_exhausted: number; late: number };
type QueryState = { isError: boolean; data: unknown };

const mockInbox = vi.mocked(useInbox);
const mockCit = vi.mocked(useVendorRequests);
const mockEod = vi.mocked(useEodSummary);

const inboxState = (n: number | undefined, isError = false): QueryState => ({
  isError,
  data: n === undefined ? undefined : Array.from({ length: n }),
});
const citState = (n: number | undefined, isError = false): QueryState => ({
  isError,
  data: n === undefined ? undefined : { pagination: { total_count: n } },
});
const eodState = (counts: Counts | undefined, isError = false): QueryState => ({
  isError,
  data: counts ? { counts } : undefined,
});

function setup(inbox: QueryState, cit: QueryState, eod: QueryState) {
  mockInbox.mockReturnValue(inbox as never);
  mockCit.mockReturnValue(cit as never);
  mockEod.mockReturnValue(eod as never);
}

function run(role: DbRole | null) {
  return useNavBadges(getVisibleNav(role), role);
}

const lastEnabled = (m: { mock: { calls: unknown[][] } }, optIdx: number) =>
  (m.mock.calls.at(-1)?.[optIdx] as { enabled?: boolean }).enabled;

beforeEach(() => {
  vi.clearAllMocks();
  setup(inboxState(undefined), citState(undefined), eodState(undefined));
});

const arbCount = fc.integer({ min: 0, max: 500 });
const arbCounts: fc.Arbitrary<Counts> = fc.record({
  failed: fc.integer({ min: 0, max: 5 }),
  max_retries_exhausted: fc.integer({ min: 0, max: 5 }),
  late: fc.integer({ min: 0, max: 5 }),
});

describe("useNavBadges — Property 9", () => {
  it("enabled only for sources of visible items; EOD only for admin; values follow data", () => {
    fc.assert(
      fc.property(
        fc.constantFrom(...ROLES),
        arbCount,
        arbCount,
        arbCounts,
        (role, inboxN, citN, counts) => {
          setup(inboxState(inboxN), citState(citN), eodState(counts));
          const visible = getVisibleNav(role);
          const sources = new Set(visible.all.map((i) => i.badge));
          const statuses = role ? badgeRequestStatuses(role) : null;
          const isAdmin = role === "ADMIN" || role === "ADMIN_PARAM";

          const badges = useNavBadges(visible, role);

          expect(lastEnabled(mockInbox, 0)).toBe(sources.has("approvals-inbox"));
          expect(lastEnabled(mockCit, 1)).toBe(sources.has("cit-requests") && statuses !== null);
          expect(lastEnabled(mockEod, 2)).toBe(sources.has("eod-health") && isAdmin);
          if (!isAdmin) expect(lastEnabled(mockEod, 2)).toBe(false);

          const inboxVal = formatBadgeCount(inboxN);
          expect(badges["approvals-inbox"]).toEqual(
            inboxVal ? { kind: "count", value: inboxVal, raw: inboxN } : undefined,
          );
          const citVal = formatBadgeCount(citN);
          expect(badges["cit-requests"]).toEqual(
            citVal ? { kind: "count", value: citVal, raw: citN } : undefined,
          );
          const attention = counts.failed + counts.max_retries_exhausted + counts.late;
          expect(badges["eod-health"]).toEqual(attention > 0 ? { kind: "dot" } : undefined);
        },
      ),
      { numRuns: 200 },
    );
  });

  it("never enables any query for a null role", () => {
    run(null);
    expect(lastEnabled(mockInbox, 0)).toBe(false);
    expect(lastEnabled(mockCit, 1)).toBe(false);
    expect(lastEnabled(mockEod, 2)).toBe(false);
  });
});

describe("badgeRequestStatuses (D8)", () => {
  const table: [DbRole, string[] | null][] = [
    ["ATM-SPV", ["pending_approval", "completion_pending", "vault_review"]],
    ["BRANCH-ATM-SPV", ["pending_approval", "completion_pending", "vault_review"]],
    ["ATM-USER", ["draft", "rejected"]],
    ["BRANCH-ATM-USER", ["draft", "rejected"]],
    ["ADMIN", null],
    ["ADMIN_PARAM", null],
    ["APPACCESS", null],
    ["BRANCH-USER", null],
    ["BRANCH-SPV", null],
    ["VENDOR-USER", null],
    ["ACM-USER", null],
    ["ACM-SPV", null],
  ];

  it.each(table)("%s → %j", (role, expected) => {
    expect(badgeRequestStatuses(role)).toEqual(expected);
  });

  it.each(table.filter(([, s]) => s !== null))(
    "passes %s statuses to useVendorRequests",
    (role, expected) => {
      run(role);
      expect(mockCit.mock.calls.at(-1)?.[0]).toEqual({ status: expected, pageSize: 1 });
      expect(lastEnabled(mockCit, 1)).toBe(true);
    },
  );
});

describe("useNavBadges — hidden states", () => {
  it("hides count badges at 0", () => {
    setup(inboxState(0), citState(0), eodState({ failed: 0, max_retries_exhausted: 0, late: 0 }));
    expect(run("ATM-SPV")).toEqual({
      "approvals-inbox": undefined,
      "cit-requests": undefined,
      "eod-health": undefined,
    });
  });

  it("hides badges while loading", () => {
    expect(run("ATM-SPV")["approvals-inbox"]).toBeUndefined();
    expect(run("ATM-SPV")["cit-requests"]).toBeUndefined();
    expect(run("ADMIN")["eod-health"]).toBeUndefined();
  });

  it("hides badges on error even when stale data is present", () => {
    setup(
      inboxState(5, true),
      citState(5, true),
      eodState({ failed: 1, max_retries_exhausted: 0, late: 0 }, true),
    );
    expect(run("ATM-SPV")["approvals-inbox"]).toBeUndefined();
    expect(run("ATM-SPV")["cit-requests"]).toBeUndefined();
    expect(run("ADMIN")["eod-health"]).toBeUndefined();
  });

  it("caps counts at 99+", () => {
    setup(inboxState(120), citState(1), eodState(undefined));
    expect(run("ATM-SPV")["approvals-inbox"]).toEqual({ kind: "count", value: "99+", raw: 120 });
  });

  it.each([
    [{ failed: 1, max_retries_exhausted: 0, late: 0 }],
    [{ failed: 0, max_retries_exhausted: 1, late: 0 }],
    [{ failed: 0, max_retries_exhausted: 0, late: 1 }],
  ])("shows EOD dot for %j", (counts) => {
    setup(inboxState(0), citState(0), eodState(counts));
    expect(run("ADMIN")["eod-health"]).toEqual({ kind: "dot" });
  });
});
