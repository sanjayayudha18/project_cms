import { describe, expect, it, vi } from "vitest";
import { applyServerFieldError } from "./applyServerFieldError";

describe("applyServerFieldError", () => {
  it("maps a 422 validation_error to its field", () => {
    const setError = vi.fn();
    const handled = applyServerFieldError(
      {
        status: 422,
        message: "Validasi gagal",
        details: [{ field: "email", message: "format email tidak valid" }],
      },
      setError,
    );
    expect(handled).toBe(true);
    expect(setError).toHaveBeenCalledWith("email", { message: "format email tidak valid" });
  });

  it("maps a weak-password 422 to temporary_password", () => {
    const setError = vi.fn();
    const handled = applyServerFieldError(
      {
        status: 422,
        message: "Validasi gagal",
        details: [{ field: "temporary_password", message: "password terlalu lemah" }],
      },
      setError,
    );
    expect(handled).toBe(true);
    expect(setError).toHaveBeenCalledWith("temporary_password", {
      message: "password terlalu lemah",
    });
  });

  it("maps a 409 conflict 'field: message' to its field", () => {
    const setError = vi.fn();
    const handled = applyServerFieldError(
      { status: 409, message: "username: sudah digunakan" },
      setError,
    );
    expect(handled).toBe(true);
    expect(setError).toHaveBeenCalledWith("username", { message: "sudah digunakan" });
  });

  it("maps a vendor 409 'code' conflict to its field (Task 9)", () => {
    const setError = vi.fn();
    const handled = applyServerFieldError(
      { status: 409, message: "code: sudah digunakan" },
      setError,
    );
    expect(handled).toBe(true);
    expect(setError).toHaveBeenCalledWith("code", { message: "sudah digunakan" });
  });

  it("maps a 400 invalid_reference 'field: message' to its field", () => {
    const setError = vi.fn();
    const handled = applyServerFieldError(
      { status: 400, message: "vendor_id: tidak ditemukan" },
      setError,
    );
    expect(handled).toBe(true);
    expect(setError).toHaveBeenCalledWith("vendor_id", { message: "tidak ditemukan" });
  });

  it("returns false for an unparseable error, leaving it to the caller's toast fallback", () => {
    const setError = vi.fn();
    const handled = applyServerFieldError(
      { status: 500, message: "Terjadi kesalahan internal" },
      setError,
    );
    expect(handled).toBe(false);
    expect(setError).not.toHaveBeenCalled();
  });

  it("returns false for a non-ApiError value", () => {
    const setError = vi.fn();
    expect(applyServerFieldError(new Error("boom"), setError)).toBe(false);
    expect(setError).not.toHaveBeenCalled();
  });
});
