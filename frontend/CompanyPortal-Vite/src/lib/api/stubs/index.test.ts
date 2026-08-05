import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { handleStubRequest } from "./index";

describe("handleStubRequest", () => {
  beforeEach(() => {
    vi.useFakeTimers();
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it("returns null for unmatched routes", async () => {
    const promise = handleStubRequest("/api/v1/unknown-endpoint");
    vi.advanceTimersByTime(1000);
    const result = await promise;
    expect(result).toBeNull();
  });

  it("returns dashboard metrics for GET /dashboard/metrics", async () => {
    const promise = handleStubRequest("/api/v1/dashboard/metrics");
    vi.advanceTimersByTime(1000);
    const response = await promise;

    expect(response).not.toBeNull();
    expect(response?.status).toBe(200);

    const data = await response?.json();
    expect(data).toHaveProperty("activeMachines");
    expect(data).toHaveProperty("pendingFillInstructions");
    expect(data).toHaveProperty("openReconciliationItems");
    expect(data).toHaveProperty("pendingApprovals");
    expect(data.activeMachines).toBe(2847);
  });

  it("returns dashboard activity for GET /dashboard/activity", async () => {
    const promise = handleStubRequest("/api/v1/dashboard/activity?limit=10");
    vi.advanceTimersByTime(1000);
    const response = await promise;

    expect(response).not.toBeNull();
    expect(response?.status).toBe(200);

    const data = await response?.json();
    expect(Array.isArray(data)).toBe(true);
    expect(data).toHaveLength(10);
    expect(data[0]).toHaveProperty("id");
    expect(data[0]).toHaveProperty("type");
    expect(data[0]).toHaveProperty("description");
    expect(data[0]).toHaveProperty("timestamp");
    expect(data[0]).toHaveProperty("actor");
  });

  it("returns DSR upload history for GET /forecasting/dsr/uploads", async () => {
    const promise = handleStubRequest("/api/v1/forecasting/dsr/uploads");
    vi.advanceTimersByTime(1000);
    const response = await promise;

    expect(response).not.toBeNull();
    expect(response?.status).toBe(200);

    const data = await response?.json();
    expect(Array.isArray(data)).toBe(true);
    expect(data).toHaveLength(30);
    expect(data[0]).toHaveProperty("id");
    expect(data[0]).toHaveProperty("date");
    expect(data[0]).toHaveProperty("filename");
    expect(data[0]).toHaveProperty("rowCount");
    expect(data[0]).toHaveProperty("status");
  });

  it("returns accepted response for POST /forecasting/dsr", async () => {
    const promise = handleStubRequest("/api/v1/forecasting/dsr", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ rows: [], filename: "test.xlsx", uploadDate: "2025-01-01" }),
    });
    vi.advanceTimersByTime(1000);
    const response = await promise;

    expect(response).not.toBeNull();
    expect(response?.status).toBe(200);

    const data = await response?.json();
    expect(data).toHaveProperty("id");
    expect(data).toHaveProperty("timestamp");
    expect(data).toHaveProperty("rowCount");
    expect(data.status).toBe("accepted");
  });

  it("returns auth login response for POST /auth/login", async () => {
    const promise = handleStubRequest("/api/v1/auth/login", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ email: "test@example.com", password: "password123" }),
    });
    vi.advanceTimersByTime(1000);
    const response = await promise;

    expect(response).not.toBeNull();
    expect(response?.status).toBe(200);

    const data = await response?.json();
    expect(data).toHaveProperty("user");
    expect(data).toHaveProperty("accessToken");
    expect(data.user).toHaveProperty("id");
    expect(data.user).toHaveProperty("fullName");
    expect(data.user).toHaveProperty("email");
    expect(data.user).toHaveProperty("roles");
    expect(data.user.roles).toContain("Admin");
    expect(data.user.primaryRole).toBe("Admin");
  });

  it("returns auth refresh response for POST /auth/refresh", async () => {
    const promise = handleStubRequest("/api/v1/auth/refresh", {
      method: "POST",
    });
    vi.advanceTimersByTime(1000);
    const response = await promise;

    expect(response).not.toBeNull();
    expect(response?.status).toBe(200);

    const data = await response?.json();
    expect(data).toHaveProperty("user");
    expect(data).toHaveProperty("accessToken");
  });

  it("simulates latency within configured range", async () => {
    // Math.random() = 0.5 → delay = floor(0.5 * 601) + 200 = 500ms
    vi.spyOn(Math, "random").mockReturnValue(0.5);

    const promise = handleStubRequest("/api/v1/dashboard/metrics");

    // Should not resolve before 500ms
    vi.advanceTimersByTime(499);
    const before = Promise.race([promise.then(() => "resolved"), Promise.resolve("pending")]);
    expect(await before).toBe("pending");

    // Should resolve at 500ms
    vi.advanceTimersByTime(1);
    const response = await promise;
    expect(response).not.toBeNull();
    expect(response?.status).toBe(200);

    vi.spyOn(Math, "random").mockRestore();
  });

  it("defaults to GET method when no options provided", async () => {
    const promise = handleStubRequest("/api/v1/dashboard/metrics");
    vi.advanceTimersByTime(1000);
    const response = await promise;
    expect(response).not.toBeNull();
    expect(response?.status).toBe(200);
  });

  it("does not match wrong HTTP method", async () => {
    const promise = handleStubRequest("/api/v1/dashboard/metrics", {
      method: "POST",
    });
    vi.advanceTimersByTime(1000);
    const result = await promise;
    expect(result).toBeNull();
  });
});
