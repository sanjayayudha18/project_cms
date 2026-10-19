import { beforeEach, describe, expect, it, vi } from "vitest";

// fe-coverage C5: approval-hierarchy admin fetchers (the hooks/pages tests mock this module).
const client = vi.hoisted(() => ({
  get: vi.fn(),
  post: vi.fn(),
  put: vi.fn(),
  delete: vi.fn(),
}));
vi.mock("@/lib/api/client", () => ({ api: client }));

import {
  createDelegation,
  createLeave,
  createPolicy,
  getDelegations,
  getLeaves,
  getPolicies,
  getUserHierarchy,
  revokeDelegation,
  setUserHierarchy,
  updatePolicy,
} from "../api";

beforeEach(() => {
  for (const fn of Object.values(client)) {
    fn.mockReset();
    fn.mockResolvedValue({ data: { ok: true } });
  }
});

describe("rbac-settings api", () => {
  it("reads hierarchy, delegations, leaves and policies", async () => {
    await expect(getUserHierarchy()).resolves.toEqual({ ok: true });
    await getDelegations();
    await getLeaves();
    await getPolicies();

    expect(client.get.mock.calls.map((c) => c[0])).toEqual([
      "/admin/approval/users/hierarchy",
      "/admin/approval/delegations",
      "/admin/approval/leaves",
      "/admin/approval/policies",
    ]);
  });

  it("sets a user's supervisor + approval level, sending only those two fields", async () => {
    await setUserHierarchy(7, { supervisor_id: 3, approval_level: 2, extra: "x" } as never);

    expect(client.put).toHaveBeenCalledWith("/admin/approval/users/7/hierarchy", {
      supervisor_id: 3,
      approval_level: 2,
    });
  });

  it("creates and revokes delegations, records leave, and creates/updates policies", async () => {
    const delegation = { delegator_id: 3, delegate_id: 4, start_date: "2026-10-20" };
    const leave = { user_id: 3, start_date: "2026-10-20", end_date: "2026-10-22" };
    const policy = { document_type: "master_data", levels: 2 };

    await createDelegation(delegation as never);
    await revokeDelegation(11);
    await createLeave(leave as never);
    await createPolicy(policy as never);
    await updatePolicy(5, policy as never);

    expect(client.post.mock.calls).toEqual([
      ["/admin/approval/delegations", delegation],
      ["/admin/approval/leaves", leave],
      ["/admin/approval/policies", policy],
    ]);
    expect(client.delete).toHaveBeenCalledWith("/admin/approval/delegations/11");
    expect(client.put).toHaveBeenCalledWith("/admin/approval/policies/5", policy);
  });
});
