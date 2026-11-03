# Tests — dsr-late-report (Phase 2.3)

Stage 4 Test, checkpoint P9, 2026-10-27, branch `dev1`, nothing committed. All gates ran against a **temporary** database `cms_p9_tmp` on `localhost:5432` (built by `go run ./cmd/migrate up`, dropped afterwards). Dev DB was only read (`migrate status`).

## Gate results

| # | Gate | Result |
|---|---|---|
| 1a | Go `build` / `vet` / `test ./...` — `pkg`, `backend-exq`, `backend` | green, 0 failures (14 packages `ok` in the unit run) |
| 1b | `go test -tags integration -p 1 -count=1 -cover -v ./...` (backend, temp DB) | **2136 PASS, 0 SKIP, 0 FAIL**. `pkg` and `backend-exq` with the integration tag: green (`backend-exq` has no tests) |
| 1c | Coverage vs `backend/coverage-thresholds.txt` (`scripts/check-coverage.sh`) | all `ok`: migrate 81.8≥81 · approval 88.6≥88 · audit 86.7≥86 · auth 91.3≥90 · handler 81.9≥80 · notification 83.9≥83 · repository 86.5≥86 · rolemgmt 93.5≥93 · service 82.6≥82 (vendorrole 100.0, no threshold) |
| 1d | `golangci-lint run --build-tags integration ./...` (3 modules) | 0 issues ×3 |
| 1e | `govulncheck ./...` (3 modules) | **1 finding per module, same one**: GO-2026-6809 `crypto/x509` in the Go **standard library** (found go1.26.9, fixed go1.26.10). Not in feature code; fix = upgrade the local/CI Go toolchain to ≥ 1.26.10. No module-dependency findings that are called |
| 1f | `sqlc generate` (v1.31.1) + `UserLeafe`→`UserLeave` fix | no diff in `backend/internal/db/` (checksums identical before/after); `go build ./...` ok |
| 2 | Python `py -3 -m unittest` with temp DB | `dsr` 19 · `itm/cashpos` 3 · `replenish_classify` 12 · `dsr_late_notify` 13 · `lib` 66 → **113 tests, 0 skipped, OK** (CI-style `discover` from `backend_python/` = 91; `dsr` and `itm/cashpos` are not importable packages so they run with `-s`, as in CLAUDE.md Sec 2a) |
| 3a | CompanyPortal `gen:api` | no diff (`schema.d.ts`, `routeTree.ts` checksums unchanged) |
| 3b | `pnpm run build` (tsc + vite + `check-bundle.mjs`) | ok; initial JS (entry + /login + /) **178.8 KB gzip, budget 300.0 KB**. Vite warns one chunk is 510.55 kB minified (154 kB gzip), pre-existing warning |
| 3c | `pnpm run test` / `test:coverage` | 167 files, **1627 tests passed**, 0 failed; overall coverage 85.66% stmts / 81.59% branch / 82.13% funcs / 86.77% lines |
| 3d | `pnpm run lint` (biome) | 531 files checked, no errors |
| 4 | Dev DB `go run ./cmd/migrate status` (read-only) | **31 applied, 0 pending**; `033_dsr_receipts.sql` applied, ok |
| 5 | Docker image builds (`backend`, `backend_python`) | **NOT RUN** — Docker Desktop daemon is not running (`dockerDesktopLinuxEngine` pipe missing). Outstanding |

Not reproduced locally:
- CI runs Go tests with `-race`; this host has no `gcc` (`-race requires cgo`), so it ran **without** `-race`.
- Everything on this list is a local run on Windows, not CI.

## Test → FR trace

| Requirement | Evidence |
|---|---|
| FR1.1 receipt in the same tx, FR1.2 not on failed/rollback, FR1.3 append-only, FR1.4 vendor resolve | `backend_python/dsr/test_dsr_receipts_db.py`: `test_commit_writes_one_receipt_with_locations_and_received_at`, `test_vendor_id_resolved_from_vendors_name`, `test_failure_after_receipt_rolls_back_receipt_and_upload`, `test_recommit_same_file_keeps_one_receipt`, `test_revision_adds_row_and_first_received_at_is_kept`; parser/receipt unit tests in `dsr/test_dsr_etl.py`; append-only guard `internal/repository/no_hard_delete_test.go` (`dsr_receipts` listed) |
| FR1.5, 2.5, 4.1 start date | Go `TestParseDsrLateStartDate`, `TestDsrLateReport_NoStartDateIsEmpty`, repository `FiltersAndWindow` "empty window: month before start date, start after month"; Python `test_no_start_date_and_before_start_do_nothing` (K5) |
| FR2 obliged branches (FLM + active vault, ROH excluded, branch/vault active-at-date, unmapped vault excluded) and FR3 hit / first receipt / status / late minutes, K1 inclusive 09:00:00, K4 BELUM JATUH TEMPO, R4 WIB day change | `TestDsrLateReportRepository_SharedCases` (shared fixture `internal/repository/testdata/`, same cases used by Python), `..._FiltersAndWindow` incl. "as_of 00:30 WIB is already the next day (UTC still the day before)" |
| FR3.4 warnings | `..._FiltersAndWindow` "warnings vendor filter"; UI `shows the warnings panel with the excluded-branch marker and admin link` |
| FR4.2/4.3 daily + summary, filters, paging | `..._FiltersAndWindow` "vendor filter, count and paging agree", "branch + status filter"; `TestDsrLateHandler`, `TestDsrLateHandler_FilterParsing`; `TestDsrLateReport_MonthAndStatusValidation`, `TestParseDsrLateMonth` |
| FR4.4 CSV export from replica + audit `report_exported` | `TestDsrLateReportRepository_ReadsReplica`, `TestIntegration_DsrLateReport_ExportAudit`; FE `api.test.tsx` "calls the CSV endpoint with format=csv and saves the file" |
| FR4.5 internal only | no VendorPortal route (no change under `frontend/VendorPortal-Vite`) |
| FR4.6 RBAC route + service | `TestDsrLateReport_Authorize`, `TestDsrLateHandler` (role cases); OpenAPI contract test registers `/dsr-late-reports` |
| FR5.2 only unsent, unmapped excluded | `test_unsent_matches_shared_fixture` |
| FR5.3 recipients (users + PICs, vendor-wide, one notification per recipient), 5.4 ATM-USER summary | `test_recipients_idempotent_and_readable_by_go`, `test_email_pending_when_smtp_on`, `test_internal_message_counts`, `test_vendor_message_lists_branches` |
| FR5.5 length limits, "dan N lainnya" | `test_long_list_is_cut_with_dan_n_lainnya`, `test_fit_list_falls_back_to_count`, `test_normalize_email` |
| FR5.6/5.7 idempotent per date, once only, no past/future abuse | `test_recipients_idempotent_and_readable_by_go`, `test_nothing_unsent_still_writes_success_row`, `test_future_date_is_refused` |
| FR5.8 job audit in `retry_audit_logs`, rollback on failure | `test_failure_rolls_back_every_row`, `test_failure_writes_failed_row_and_nothing_else`; scheduler registration in `lib/services/test_scheduler_service.py` |
| FR6.1 menu | FE `sits in the monitoring group at /dsr-late-report` |
| FR6.2–6.6 screen (filters, tabs, export, columns, status text+icon, warnings, loading/error/empty/403) | FE `DsrLateReport.test.tsx` (10 cases: status labels+icons, WIB time/late minutes right-aligned, summary tab, filter→query reset page, export, export failure toast, warnings panel, non-admin link hidden, loading/empty-before-start/feature-off, 403 + retry) and `api.test.tsx` (7 cases) |
| API contract | `go test ./internal/handler -run OpenAPI` (inside the 2136); `gen:api` no diff |
| FR7 audit | export audit test above; job audit tests above |
| Migration 033 | `migrate up` on empty temp DB applies 001–033; dev `status` 0 pending |

Gaps, stated plainly: `go test -race`, the Docker builds, and every item under *Manual steps* were not run. `pnpm test:coverage` was read only as an overall figure; per-folder coverage of `features/dsr-late-report` was not itemised.

## Review fixes R1–R3 (2026-10-27)

Run on a throwaway DB `dsrfix_tmp` built with `go run ./cmd/migrate up` (31 applied), dropped afterwards; dev untouched.

| Finding | Regression test | Result |
|---|---|---|
| R1 vendor by code | `test_dsr_receipts_db.test_two_vendors_with_same_name_receipt_gets_id_of_the_code`, `test_dsr_etl.InsertReceiptTests.test_never_looks_up_vendor_by_name` | pass |
| R2 no run before 09:00 WIB | `dsr_late_notify/test_job_deadline_guard.py` (3 cases) | pass |
| R3 CSV columns | `TestDsrLateHandler/daily_csv` (header, checksum, `CSVSafe` on labels) | pass |

Gates: `discover -s dsr` 19 tests OK / 0 skip; `discover -s dsr_late_notify -t .` 16 OK / 0 skip; `go test ./internal/handler ./internal/service` ok; `-run OpenAPI` ok; `golangci-lint run --build-tags integration ./...` 0 issues. `schema.d.ts` unchanged, so the frontend suite was not re-run. Not captured: a recorded run of the new tests against the pre-fix code (the pre-fix signature/header/guard make them fail by construction: TypeError, header mismatch, no ValueError).

## Manual steps (outstanding — user)

Dev data: `backend/scripts/dev/dsr_late_report_seed.sql` (vendor Bijak, upload `report_date` 2026-07-15). Seed is **not** run by the AI; user runs it on dev. Manual browser check of the screens is **outstanding (user)** — Golden Rule #10. The real 09:00 WIB notification run and a real vendor upload around the deadline are also **outstanding (user)**.

1. **Upload before 09:00 WIB**: vendor confirms a DSR upload at 08:30 → the branch is not TELAT for that date (on time); `dsr_receipts` has one row with `received_at` before 09:00:00.
2. **Upload after 09:00 WIB**: vendor confirms at 09:00:01 → "TELAT". At exactly 09:00:00 → on time (K1 inclusive).
3. **Revision**: vendor uploads a revised file for the same date → a new `dsr_receipts` row; the earlier first-received time still decides the status (first = MIN).
4. **Same file again**: confirm the same file twice → no new row (checksum unique).
5. **Wrong location label**: a DSR block label not in `dsr_location_vault_maps` → warning on the screen, the branch is not counted as TIDAK KIRIM.
6. **Today before 09:00**: status `BELUM JATUH TEMPO` (K4); after 09:00 without upload → `TIDAK KIRIM`.
7. **Role without access**: vendor user and `VENDOR-*` roles do not see the menu and get 403 on `/api/v1/dsr-late-reports`; `ATM-USER`, `ATM-SPV`, `BRANCH-ATM-SPV`, `ADMIN` see all vendors and branches (S4).
8. **Notifications at 09:00 WIB**: branches not sent get the vendor notification (users + PICs with `is_notification_recipient`, K3); `ATM-USER` gets one summary; email appears in the outbox and is sent by the Go worker.
9. **Start date**: with `DSR_LATE_START_DATE` (Go) and `RETRY_DSR_LATE_START_DATE` (Python) equal and set, the job runs only for report dates on/after that date (K5).
