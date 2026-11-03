# Review — dsr-late-report (Phase 2.3)

Stage 5 Deploy/Review. **Review draft 2026-10-27** (independent review session, read-only: no code changed, nothing applied to dev DB, nothing staged). Fixes wait for the user's decision; the code owner merges. Evidence of green gates: [tests.md](./tests.md).

Scope reviewed (uncommitted on `dev1`): migration `033`, `queries/dsr_late_report.sql` + sqlc output, repository/service/handler + tests, `cmd/api/main.go`, `api/openapi.yaml` + contract test, `.env.example`s, `dsr/dsr_etl.py`, `dsr_late_notify/`, `eod_retry_scheduler/config.py`, `scheduler_service.py`, CompanyPortal `features/dsr-late-report/` + route/nav/schema, docs.

**Result: 3 Important, 10 nit.** Nothing CRITICAL. Recommendation: fix R1–R3 before commit (all small, one file each plus a test), nits at the owner's discretion.

## Review passes
- [x] Bugs / correctness. Status CASE, K1 (`date_trunc('second') <= 09:00:00 WIB`), K4, late-minute ceil, WIB conversion (`AT TIME ZONE 'Asia/Jakarta'` everywhere, session TZ irrelevant), S2/S3/S5 (`counted_vaults` / `obliged`), MIN first receipt (`DISTINCT ON … ORDER BY received_at, receipt_id`; revisions append, `ON CONFLICT DO NOTHING` on recommit, so the first receipt never moves), start date / empty month / future month, job idempotency (advisory xact lock + success-row check in the same tx) all read correct. The Go and Python CTE chains (`counted_vaults` / `obliged` / `hits` / `first_hit`) are textually identical (D2). Findings: **R1, R2**, nits n1–n2, n4–n5.
- [x] Security. RBAC at route (`RequireRoles` in `DsrLateHandler.Routes`) **and** service (`authorize` in `prepare` and `RecordExport`). Vendor roles get 403 (contract case). No per-user scoping needed (S4). All SQL is parameterised (sqlc, asyncpg `$n`, psycopg `%s`); no string-built SQL. CSV: every cell goes through `service.CSVSafe` in `writeCSV` (`=`, `@`, TAB, CR always quoted; `+`/`-` unless a plain number). Filename built from the parsed month only. 500s go through `writeUnexpectedError` (generic body). No secrets in code or logs. Python `error_detail` stays in `retry_audit_logs` (internal). No new findings beyond nit n10.
- [x] Compliance (CLAUDE.md Sec 3–6, 12, 13). Reads on the replica (`NewDsrLateReportRepository(readPool)`), export audit on the primary before the CSV is written (500 and no CSV on audit failure). Job reads/writes primary. `notifications` / `notification_emails` columns match Go `InsertNotification` / `InsertNotificationEmail`; title/body limits and `EMAIL_FOOTER` match Go; recipient SQL matches `ListActiveUserRecipientsByVendorBranches` / `ListVendorBranchNotificationPicEmails`. Flat JSON like the neighbouring handlers. OpenAPI paths, params and 200 (JSON + `text/csv`) / 400 / 401 / 403 / 500 present; contract test mounts the router. Status = text + icon in the UI (Sec 13). Finding: **R3**.

## Findings — Important

### R1 — Receipt vendor resolved by non-unique `vendors.name` (wrong vendor → false TIDAK KIRIM)
- **Status: fixed 2026-10-27.** `resolve_vendor` returns `(id, name)` by code ([dsr_etl.py:707](../../../backend_python/dsr/dsr_etl.py)); `vendor_id` flows through `_upsert_upload` (:500) into `_insert_receipt` (:482); name lookup removed. Spec FR1.4 + data-map note updated; migration 033 and its column comment untouched. Regression: `test_two_vendors_with_same_name_receipt_gets_id_of_the_code` ([test_dsr_receipts_db.py:93](../../../backend_python/dsr/test_dsr_receipts_db.py)) and `test_never_looks_up_vendor_by_name` ([test_dsr_etl.py:168](../../../backend_python/dsr/test_dsr_etl.py)).
- **Where**: `backend_python/dsr/dsr_etl.py:486` (`_insert_receipt`: `SELECT id FROM public.vendors WHERE name = %s`, `fetchone()`, no `ORDER BY`, no `is_active` / `deleted_at` filter).
- **Why**: `vendors.name` has no UNIQUE constraint (only `vendors_code_key`). `process_file` (`:716-719`) already had the unique `vendor_code` from the filename and turned it into a name via `resolve_vendor_name`; the receipt then goes back from name to id.
- **Failure scenario**: two `vendors` rows share a name (e.g. an old soft-deleted "Bijak" and the active one, or two vendors renamed alike). The vendor uploads on time at 08:30; the receipt gets the other row's `vendor_id`. `hits` joins on `m.vendor_id = r.vendor_id` and finds nothing, so the branch shows **TIDAK KIRIM** (penalty basis) and gets the 09:00 notification. No warning: `vendor_id` is not NULL, so FR3.4(c) does not fire, and the label is checked against the wrong vendor's maps.
- **Fix**: make `resolve_vendor_name` return `(id, name)` by `code` and pass `vendor_id` into `_upsert_upload` / `_insert_receipt`; drop the name lookup. Keep `vendor_label` = name. Update spec FR1.4 + the `dsr_receipts.vendor_id` column comment wording ("resolved by vendor code") — comment text change needs a new migration file or is left as-is with a data-map note (033 is applied; never edit it). Test: two vendors with the same name, receipt gets the code's id.

### R2 — Manual run before 09:00 WIB sends a premature alert and suppresses the 09:00 cron
- **Status: fixed 2026-10-27.** `run_once` raises ValueError when `as_of` < 09:00 WIB of the report date, before anything is written ([job.py:144](../../../backend_python/dsr_late_notify/job.py)). Regression: `dsr_late_notify/test_job_deadline_guard.py` (08:59:59 refused with no notify/audit call; 09:00:00 runs; deadline judged in WIB for a UTC `as_of`).
- **Where**: `backend_python/dsr_late_notify/job.py:137-142` (`run_once` only refuses a future **date**), `run.py:248` (default `--date` = today).
- **Failure scenario**: an operator runs `python -m dsr_late_notify.run` at 07:30 WIB (or the scheduler is started with a test trigger). Every obliged branch without a receipt yet gets "DSR … belum diterima sampai batas 09:00 WIB … dasar penalti FLM" — including branches that then upload at 08:30 and are **OK** in the report. The run writes the `success` row, so the real 09:00 cron logs `skipped / already sent`: branches still missing at 09:00 are never told at the deadline. Spec FR5.7 allows a manual run only to replace a missing run, not to run early.
- **Fix**: in `run_once`, refuse (ValueError → exit 1, nothing written) when `as_of < datetime.combine(report_date, time(9), WIB)`. One unit test: `as_of` 08:59:59 on the report date → refused; 09:00:00 → runs.

### R3 — CSV export drops the explanatory columns the audited export is meant to preserve
- **Status: fixed 2026-10-27.** Daily CSV gained `receipt_checksum` and `matched_locations` (`; `-joined), through `writeCSV` → `CSVSafe` ([dsr_late_report_handler.go:141](../../../backend/internal/handler/dsr_late_report_handler.go)). OpenAPI declares the CSV as plain `text/csv` string, so no spec change and no `gen:api`. Regression: `daily csv` case in `TestDsrLateHandler` ([dsr_late_report_handler_test.go:87](../../../backend/internal/handler/dsr_late_report_handler_test.go)) — new header, checksum, and a `=`-prefixed label proving `CSVSafe`.
- **Where**: `backend/internal/handler/dsr_late_report_handler.go:139-144` (daily CSV header/rows).
- **Why**: FR3.3 lists `receipt_checksum` and the matched location labels as the per-row explanation (Sec 6: reproducible + explainable). Spec risk R2 says the status of past days can shift when vaults/maps change, and names "ekspor CSV yang diaudit" as the trail. The JSON has both fields; the CSV — the only artefact that survives a later shift — has neither, so a disputed TELAT/TIDAK KIRIM in an exported recap cannot be traced back to the receipt and label that decided it.
- **Failure scenario**: penalty recap exported 2026-11-01; ADMIN later deactivates a vault / edits a map; the live report changes; the exported CSV cannot show which receipt (checksum) and label made the branch OK/TELAT at export time.
- **Fix**: add `receipt_checksum` and `matched_locations` (joined with `; `) columns to the daily CSV; `CSVSafe` already covers the vendor-typed labels. Update the handler test's expected header.

## Findings — nit
- **n1** `lib/services/scheduler_service.py:138-143`: the 09:00 job uses APScheduler defaults (`misfire_grace_time` 1 s, no catch-up). A scheduler restart/stall around 09:00 means no notification that day and no `retry_audit_logs` row to show it was missed. Same pattern as the 09:15 classify job (pre-existing). Suggest `misfire_grace_time=3600, coalesce=True`, or alert on "no row by 09:30".
- **n2** `dsr_late_notify/job.py:137` + `queries.py:40`: the cron fires at ~09:00:00.0x and uses `received_at <= as_of`, while the report counts anything up to 09:00:00.999 as OK (K1). A receipt at 09:00:00.5 is notified as missing yet shows OK. Use `as_of = max(now, deadline + 1 s)` or `date_trunc('second', received_at) <= as_of`.
- **n3** `backend/queries/dsr_late_report.sql`: the same CTE chain is copied 5× in Go and 1× in Python. The shared fixture guards drift, but every change touches six places. Later option: one SQL function/view (needs its own migration).
- **n4** `dsr_late_report.sql:302-303` (`ListDsrLateUnmappedLabels`): a label whose only map points to an inactive / not-yet-created vault is not a warning, so the branch shows TIDAK KIRIM with no hint (documented in the P3 hand-over note). Consider listing it under FR3.4(b) with a reason.
- **n5** `dsr/dsr_etl.py:488`: a `completed` Daily sheet with no non-empty location label writes a receipt with `locations = '{}'`; it hits no branch and raises no warning (FR3.4 has no category for it). Log a warning or add a warning kind.
- **n6** `dsr_late_report_handler.go:125,157`: any `format` other than `csv` silently returns JSON; `format=xlsx` could be a 400. Low value.
- **n7** `.claude/development-progress.md` row still said migration 033 "not created" (fixed in this review's row update).
- **n8** The diff deletes the tracked `.claude/sdlc/.done/dsr-late-report/intent.md` (from 714d943) while the active folder is untracked. Confirm the move is intended before committing, so the history keeps one intent.
- **n9** `dsr_late_notify/job.py:64`: email dedupe key is `(address, body)`; Go `Send` dedupes per address. A person who is both a vendor user and a PIC with a different branch set gets two emails. Acceptable; note only.
- **n10** `internal/repository/no_hard_delete_test.go`: guards `DELETE FROM dsr_receipts` but not `UPDATE dsr_receipts`; "append-only" rests on review only. Consider extending the guard (or a `REVOKE UPDATE` / trigger in a later migration).

## Definition of Done (CLAUDE.md Sec 11)
- [x] Matches requirement + module/table map (Sec 3): only `dsr_receipts` added; CLAUDE.md Sec 3/12, `data-map.md`, `decisions.md` updated. Caveat R1 (FR1.4 wording).
- [x] Correct auth path (internal JWT/LDAP only, no VendorPortal route), RBAC `ATM-USER`, `ATM-SPV`, `BRANCH-ATM-SPV`, `ADMIN` at route + service.
- [x] Maker-checker + audit_log (Sec 5): no master-data write; export audited (`report_exported`, before CSV, fails closed); job audited in `retry_audit_logs` (documented S4 pattern). **No GR#3 deviation — confirmed** (the feature changes no master data).
- [x] Reads on replica, writes on primary (Sec 6): report/export on replica; export audit, ETL receipt and notify job on primary.
- [x] Money as numeric, timestamps timestamptz: no money; `received_at` timestamptz, display WIB.
- [x] Tests passing incl. auth/RBAC cases — per tests.md (Go 2136 / 0 skip, Python 113 / 0 skip, FE 1627); not re-run in this review. `-race` not run locally.
- [x] No secrets/config hardcoded; `.env.example` updated (`DSR_LATE_START_DATE`, `RETRY_DSR_LATE_START_DATE`, K5 note).
- [x] Response shape per Sec 5: flat JSON like neighbouring `backend` handlers.
- [x] New endpoints in `openapi.yaml` + contract test (`-run OpenAPI`) + `gen:api` clean (per tests.md).
- [ ] Builds cleanly in Docker — **outstanding (user)**: Docker daemon was down at P9.

## Outstanding (user)
- Docker builds of `backend` and `backend_python`.
- Manual browser check of the screens (Golden Rule #10).
- Real vendor upload around 09:00:00 WIB and the real 09:00 notification run (tests.md manual steps 1–9).
- `govulncheck` GO-2026-6809 (Go stdlib; toolchain ≥ 1.26.10), not caused by this feature.
- Go-live R6/R8: report and notifications stay empty until ADMIN fills `dsr_location_vault_maps`.

## Decision log
- 2026-10-27 review draft written. Awaiting user decision on R1–R3 and the nits; then fixes → re-run gates → code owner merge.
- 2026-10-27 user approved R1–R3 only; R1–R3 **fixed** (gates in [tests.md](./tests.md) "Review fixes R1–R3"). Nits n1–n10 left as is.
