# Follow-ups — admin-atm-management

Recorded at the close of this spec (Task 10, 2026-09-16).

## 1. `applyServerFieldError` doesn't actually match this backend's error messages

The shared frontend util `src/lib/utils/applyServerFieldError.ts` maps a 409/400 error to a form field by regex-matching a `"field: message"` shaped backend message. But `ATMAdminService`'s real Go sentinel errors (`ErrATMTerminalIDConflict`, `ErrATMInvalidReference`, in `backend/internal/service/atm_admin.go`) are plain messages with no field prefix (e.g. `"atm terminal_id already exists"`) — the regex never matches. The same gap already exists in the sibling `admin-user-vendor-management` spec's vendor/user forms (their backend errors are equally prefix-less), so this isn't new, just newly hit again.

Worked around locally in `ATMFormDialog.tsx` via `applyATMConflictOrReferenceError`, which maps by HTTP status instead (409→`terminal_id`, 400→`location_id`) — safe here only because `terminal_id` is never sent on update, so a 400 from this form can only mean `invalid_reference`. That reasoning is specific to this form and won't automatically hold for a future admin CRUD screen with more than one possible 400 case.

Real fix, not done here (out of scope): either (a) change the Go sentinel error messages to include a field prefix (`"terminal_id: already exists"`), which is a public-facing string change reviewers should sign off on, or (b) give each `ATMAdminService`/`VendorAdminService`/`UserAdminService` error a structured `Field()` method the handler can serialize instead of relying on message-string parsing at all. (b) is the more robust fix if a fourth admin CRUD screen is ever added.

## 2. Filter dropdown options are a live-DB snapshot, not derived data

`ATMFilterBar.tsx`'s Brand/Tipe Mesin/Deployment options are hardcoded lists taken from `SELECT DISTINCT` against the real `atms` table on 2026-09-16 (6 brands, 3 machine types, 2 deployment types) — same "fixed dropdown is fine at current volumes" call as the deferred-then-approved search-index decision (Task 0). If a new brand/machine type/deployment value is ever added to production data, it will filter correctly via `q`/direct query but won't appear as a dropdown option until this file is updated by hand. Revisit with a `DISTINCT`-backed endpoint (mirroring `ListLocationsForSelect`) if these sets start changing.

## 3. Frontend coverage tooling was previously missing project-wide

`@vitest/coverage-v8` did not exist as a devDependency anywhere in `CompanyPortal-Vite` before this spec's Task 9 — not even for the sibling `admin-user-vendor-management` spec, whose task checklist has the identical "coverage ≥ 80%" wording that was apparently never actually measured. Installed here (with explicit user approval, since CLAUDE.md requires approval for new deps) pinned to the exact `vitest` version in use (4.1.10). It's now available for any future frontend spec's quality gate — no more excuse to skip the measurement.

## Not a follow-up here, for context

- No money/amount column exists in the `ATMsTable` list view — this is a resolved reading of the spec, not an oversight: design.md's own frontend section and tasks.md's own explicit column list both omit an amount column from the list table (money only appears in the create/edit form, Task 8). See Task 7's tasks.md entry for the full reasoning.
- The `sqlc generate` drift (unrelated files reformatted on every regenerate) and migration `017`'s pre-existing bug are already tracked in the sibling spec's own `follow-ups.md` (`admin-user-vendor-management`) — not duplicated here. This spec hit the same drift in Tasks 2 and 5 and used the same revert-and-keep-only-the-new-file workaround.
