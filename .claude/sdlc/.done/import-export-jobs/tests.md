# import-export-jobs — Tests (stage 4)

Status: run 2026-09-29 against dev DB (`localhost:5432`). Reads: `spec.md` (A1–A17), `plan.md` (T8.1).

## Results

| Suite | Command | Result |
|---|---|---|
| Go backend | `cd backend && go build ./... && go vet ./... && go test -count=1 ./...` | green (no code changed in Go except `models.go` + `UserLeave` inflector fix) |
| Go backend-cit, pkg | `go build ./... && go test ./...` | green |
| Python `import_jobs` | `python -m unittest lib.test_import_jobs` | 23 OK |
| Python detector | `python -m unittest lib.services.test_detector` | 1 OK |
| Python scheduler | `python -m unittest lib.services.test_scheduler_service` | 15 OK |
| Python EOD API (both services) | `python -m unittest lib.test_eod_api` | 6 OK |
| Python DSR / ITM (existing) | `python -m unittest discover -s dsr` / `-s itm/cashpos` | 7 / 3 OK |
| Frontend CompanyPortal | `pnpm run build` (includes `tsc -b`) | green |
| Frontend CompanyPortal | `pnpm run test` | 988 pass (the earlier auth-store failure was fixed, see below) |
| Biome (eod-monitoring) | `biome check src/features/eod-monitoring` | 1 pre-existing a11y warning (`RetryDrawer.tsx:81`), no errors |

DB integration tests need `DATABASE_URL` (from `backend/.env`, host `host.docker.internal` → `localhost`); they skip when empty. After the run: 0 leftover rows in `import_jobs`/`late_detections`/`retry_audit_logs` for 2099 dates.

### Former failure (fixed, not caused by this feature)
`src/lib/auth/store.test.ts > refreshToken > redirects with current path as redirect param`: expects `/login?redirect=…` but the code now appends `&reason=session_expired`. File untouched by import-export-jobs; last changed in `7a86eca` ("rbac session … limit login"). Needs its own fix (test or code); not fixed here.

## Traceability: acceptance criteria → tests

| A | Test |
|---|---|
| A1 | `test_import_jobs.test_register_new_file_is_pending_v1` |
| A2 | `test_same_hash_when_completed_is_duplicate_without_new_row`, `test_same_hash_while_pending_is_duplicate` |
| A3 | `test_failed_hash_reuses_row_back_to_pending`, `test_exhausted_hash_also_reuses_row`, `test_failed_hash_reset_blocked_by_other_inflight_run` |
| A4 | `test_second_file_same_date_while_inflight_raises`, `ImportJobsCommittedTest.test_parallel_register_same_date_one_wins_one_in_progress`, `…same_hash_one_is_duplicate` |
| A4a | `test_null_date_rows_are_independent` |
| A5 | `test_new_hash_for_completed_date_is_next_version` |
| A5a | `test_revert_to_superseded_hash_is_version_three` |
| A6 | `test_complete_supersedes_previous_version`, `test_complete_rolls_back_with_callers_transaction` |
| A7 | `test_fail_keeps_previous_version_current` |
| A8 | `test_stale_inflight_row_is_failed_only_past_its_limit`, `test_mark_stale_without_date_sweeps_all_dates`, `test_scheduler_service.test_cycle_marks_stuck_processing_row_stale_then_retries_it` |
| A9 | `test_illegal_transitions_are_rejected`, `test_unknown_job_id` |
| A10 | **Deferred to forecast 2.1** (Go helper, R4) |
| A11 | `test_detector.test_scan_twice_one_row_and_existing_row_untouched`, `test_import_jobs.test_detect_is_insert_if_absent_and_never_resets` |
| A12 | `test_scheduler_service.test_auto_retry_three_failures_exhausts_and_stops`, `test_manual_retry_past_limit_then_completed_conflict`, `test_auto_retry_skips_row_blocked_by_inflight_run` |
| A13 | `IsSourceDoneTest.test_each_source` (4 sources), `test_import_jobs_completed_counts_as_done_without_legacy_row`, `LateCheckTest.test_nothing_done_is_late`, `test_completed_import_job_is_not_late`, `test_auto_retry_success_completes_and_resolves_late_detection` |
| A14 | Migration `019` applied to the **existing** dev DB (T1.2, tables/indexes/FK checked). **Not run on an empty DB** — gap; do this on the first fresh environment. |
| A15 | `test_eod_api`: file_id as string + field names, own-sources-only, numeric-id-only (422), retry→history→409/404, summary `superseded` bucket — both services |
| A16 | `test_user_transitions_write_audit_logs_in_same_tx`, `test_system_transitions_write_no_audit_logs`, `test_manual_retry_by_real_user_writes_audit_logs`, `test_manual_retry_without_real_user_writes_no_audit_logs`, `test_eod_api.test_go_issued_jwt_identity_reaches_audit_logs` (Go-side part of A16 deferred with R4) |
| A17 | **Outstanding — manual browser check by the user** (Golden Rule #10). Unblocked: `/api/eod` routing fixed (S6, Vite + nginx proxy, verified with curl). |

Frontend (T6.1, FR16 UI): `RetryDrawer.test.tsx` — `superseded` shows "Digantikan" with icon and no retry button; retry offered only for failed/max_retries_exhausted.

## Gaps
- A14 on a fresh DB; A10 and Go side of A16 (2.1); A17 manual.
- Coverage % not measured (`coverage` not installed); earlier `trace` run put `lib/import_jobs.py` at ≈98% real.
- Summary card for `superseded` not added to the UI (type field only).

Fix: `store.ts` deliberately appends `&reason=session_expired` (login page shows the notice, from `7a86eca`); the test was stale. It now expects the `reason` param via the exported `SESSION_EXPIRED_REASON`. Suite: 118 files / 988 tests green. Separate, pre-existing: `biome check src/lib/auth` reports a format error in `store.ts` (not touched here).
