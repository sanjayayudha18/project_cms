import { beforeEach, describe, expect, it, vi } from "vitest";

const getMock = vi.fn();
const postMock = vi.fn();
const putMock = vi.fn();

vi.mock("@/lib/api/client", () => ({
  api: {
    get: (...args: unknown[]) => getMock(...args),
    post: (...args: unknown[]) => postMock(...args),
    put: (...args: unknown[]) => putMock(...args),
  },
}));

import {
  createATM,
  disableATM,
  enableATM,
  getATM,
  listATMs,
  listLocationOptions,
  updateATM,
} from "./api";

describe("admin-atms api client", () => {
  beforeEach(() => {
    getMock.mockReset();
    postMock.mockReset();
    putMock.mockReset();
  });

  it("listATMs builds a query string with only the provided filters", async () => {
    getMock.mockResolvedValue({ data: { atms: [], page: 1, page_size: 25, total: 0 } });

    await listATMs({ page: 2, page_size: 10, status: "disabled", q: "TATM", brand: "NCR" });

    const [path] = getMock.mock.calls[0];
    expect(path).toBe("/admin/atms?page=2&page_size=10&status=disabled&q=TATM&brand=NCR");
  });

  it("listATMs omits absent optional filters entirely", async () => {
    getMock.mockResolvedValue({ data: { atms: [], page: 1, page_size: 25, total: 0 } });

    await listATMs({ page: 1, page_size: 25 });

    const [path] = getMock.mock.calls[0];
    expect(path).toBe("/admin/atms?page=1&page_size=25");
  });

  it("getATM fetches by id", async () => {
    getMock.mockResolvedValue({ data: { id: 1 } });
    const result = await getATM(1);
    expect(getMock).toHaveBeenCalledWith("/admin/atms/1");
    expect(result).toEqual({ id: 1 });
  });

  it("createATM posts the payload to the base path", async () => {
    const payload = { terminal_id: "TATM001" };
    postMock.mockResolvedValue({ data: { id: 1 } });
    await createATM(payload as never);
    expect(postMock).toHaveBeenCalledWith("/admin/atms", payload);
  });

  it("updateATM puts the payload to the id path", async () => {
    const payload = { location_id: 10 };
    putMock.mockResolvedValue({ data: { id: 1 } });
    await updateATM(1, payload as never);
    expect(putMock).toHaveBeenCalledWith("/admin/atms/1", payload);
  });

  it("disableATM posts to the disable sub-path", async () => {
    postMock.mockResolvedValue({ data: { message: "ok" } });
    const result = await disableATM(1);
    expect(postMock).toHaveBeenCalledWith("/admin/atms/1/disable");
    expect(result).toEqual({ message: "ok" });
  });

  it("enableATM posts to the enable sub-path", async () => {
    postMock.mockResolvedValue({ data: { message: "ok" } });
    const result = await enableATM(1);
    expect(postMock).toHaveBeenCalledWith("/admin/atms/1/enable");
    expect(result).toEqual({ message: "ok" });
  });

  it("listLocationOptions fetches the locations sub-path", async () => {
    getMock.mockResolvedValue({ data: { locations: [] } });
    const result = await listLocationOptions();
    expect(getMock).toHaveBeenCalledWith("/admin/atms/locations");
    expect(result).toEqual({ locations: [] });
  });
});
