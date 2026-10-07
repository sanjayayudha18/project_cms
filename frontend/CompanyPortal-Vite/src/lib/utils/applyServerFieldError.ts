/**
 * Maps a create/update ApiError to a form field error, so an RHF-backed
 * dialog can keep itself open and show the error next to the offending
 * input instead of a generic toast. Shared by admin-users and admin-vendors
 * (both backend handlers use the same two error shapes, per
 * backend/internal/handler/{admin_user,admin_vendor}_handler.go's
 * handleXxxAdminError):
 *  - 422 validation_error: `details: [{field, message}]` (writeValidationError)
 *  - 409 conflict / 400 invalid_reference: `message` is "field: message"
 *    (ConflictError/ReferenceError.Error() -> fmt.Sprintf("%s: %s", ...))
 */

import type { ApiError } from "@/lib/api/client";

const FIELD_MESSAGE_PATTERN = /^([a-z_]+): (.+)$/;

function isApiError(err: unknown): err is ApiError {
  return (
    typeof err === "object" &&
    err !== null &&
    "status" in err &&
    typeof (err as { status: unknown }).status === "number" &&
    "message" in err
  );
}

/**
 * Returns true when a field-level error was applied via setError (dialog
 * should stay open, no toast needed); false means the caller should fall
 * back to a generic error toast.
 */
export function applyServerFieldError(
  error: unknown,
  setError: (field: string, error: { message: string }) => void,
): boolean {
  if (!isApiError(error)) return false;

  if (error.status === 422 && Array.isArray(error.details) && error.details.length > 0) {
    const first = error.details[0] as { field?: string; message?: string };
    if (first.field) {
      setError(first.field, { message: first.message ?? "Tidak valid" });
      return true;
    }
  }

  if (error.status === 409 || error.status === 400) {
    const match = FIELD_MESSAGE_PATTERN.exec(error.message);
    const field = match?.[1];
    const message = match?.[2];
    if (field && message) {
      setError(field, { message });
      return true;
    }
  }

  return false;
}
