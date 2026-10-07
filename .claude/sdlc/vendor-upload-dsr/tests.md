# Tests / feedback loop: vendor upload DSR

Status: re-verified 2026-10-07 (Phase 1.3). Stage: 4 Test. Reads: `plan.md`.
The session runs these before reporting done. For bug fixes: write the failing test first.

> **Post-plan update (2026-09-04):** see `plan.md`'s post-plan update and
> `testing.md` — the upload flow is now two-phase (dry-run/confirm) and the
> Python side is `backend_python/service_dsr_etl/` (not `retry_scheduler`). The
> test cases below still trace to `spec.md`'s FR numbers as originally
> written; re-verify against the current two-phase behavior before treating
> any of them as passing.

## Commands (must exit non-zero on failure)
- Build: `go build ./...`            (from backend/)
- Test:  `go test ./...`             (from backend/)
- Lint:  `golangci-lint run`         (from backend/)
- Front: `pnpm test` / `pnpm lint`   (from frontend/VendorPortal-Vite/)

## Quantifiable targets
- [ ] All tests in `internal/dsr/*_test.go` pass.
- [ ] Coverage >= 80% on `internal/dsr` (CLAUDE.md Sec 8).
- [ ] Parsing the sample workbook yields the expected file + row counts.

## Test cases (trace to spec FR)
| ID | Maps to | Case | Expect |
|----|---------|------|--------|
| T1 | FR1/FR3 | Upload valid sample .xlsx | file completed, rows inserted, derived rows absent |
| T2 | FR2 | Re-upload same checksum | returns existing id, no new parse |
| T3 | FR6 | Cell is `#REF!` | denom NULL, error_count += 1, status still completed |
| ~~T4~~ | ~~FR5~~ | ~~Stated SALDO AKHIR vs SUM()~~ | **Removed** — cross-check dropped in intent.md resolved decisions |
| T5 | FR8 | Vendor A reads Vendor B upload | 403 / not found (RBAC denial) |
| T6 | FR7 | GET status during processing | returns pending|processing|completed|failed |
| T7 | money | negative pengeluaran sign | stored verbatim, saldo math holds |

## Verification block (goes to CLAUDE.md if not already there)
Run build + test + lint before reporting any task complete; paste output.
If a test fails, fix the code, not the test.

---

## Re-verification against two-phase flow (Phase 1.3, 2026-10-07)

Re-checked every case above against the **current** code, not the pre-two-phase
spec. The flow is now: `POST /uploads` → `WriteAndDryRun` → service_dsr_etl
`/process/dsr/dry-run` (preview, nothing in DB) → vendor reviews →
`POST /uploads/confirm` → `ConfirmUpload` → `/process/dsr/commit` (re-parse +
persist) → `BuildUploadResult` reads back by checksum.

Code read: `backend/internal/service/dsr_upload.go`,
`backend/internal/service/dsr_upload_test.go`,
`backend/internal/handler/dsr_upload_handler.go`. The `internal/dsr` package
assumed by the original `tests.md` does **not** exist — DSR is organized by
layer (`internal/service` + `internal/handler`), per `structure.md`.

### Automated test run (actual)
```
go test ./internal/service/ -run "WriteAndDryRun|ConfirmUpload" -v
=== RUN   TestWriteAndDryRun_WritesFileAndRelaysParsedPayload   --- PASS
=== RUN   TestConfirmUpload_ReadsBackAfterCommitSucceeds        --- PASS
=== RUN   TestConfirmUpload_PropagatesCommitFailure             --- PASS
ok  github.com/cimb-niaga/cms/backend/internal/service
```
Three Go unit tests exist and pass. They were NOT written against the T1–T7
IDs below; they cover the two-phase relay/read-back contract instead.

### Case-by-case verdict (final, 2026-10-07)

| ID | Original case | Verdict | Evidence |
|----|---------------|---------|----------|
| T1 | Upload valid sample → completed, rows inserted | ✅ PASS (reshaped) | Two-phase now: `TestWriteAndDryRun_WritesFileAndRelaysParsedPayload` (dry-run, no DB write) + `TestConfirmUpload_ReadsBackAfterCommitSucceeds` (commit + read-back). Parsing: `test_read_daily_rows_header_and_leaf_lines`, `test_read_rencana_isi_rows_excludes_sub_total`. |
| T2 | Re-upload same checksum → no new parse | ⚠️ CODE-VERIFIED, no automated test | `dsr_etl.py` commit: same `(report_date, vendor)` + same checksum → `"skipped"`; different checksum → old upload deleted (rows cascade) and replaced. Needs a live-DB integration test; not run (would write to dev DB). |
| T3 | `#REF!` → NULL, error counted, still completed | ✅ PASS | `test_cell_number_ref_error_is_null_and_flagged` (Python); Go side `TestConfirmUpload_ReadsBackAfterCommitSucceeds` asserts `success = rows − errors`. |
| T4 | Stated SALDO AKHIR vs SUM() | ❌ REMOVED | Out of scope per intent.md resolved decisions. |
| T5 | Vendor A touches Vendor B data → 403 | ✅ PASS | New `backend/internal/handler/dsr_upload_handler_scope_test.go`: non-vendor caller 403; confirm of another vendor's staged file 403; own staged file passes the guard. Reads are additionally SQL-scoped (`GetDsrUploadByIDForVendor`). |
| T6 | GET status during processing | ⚠️ OBSOLETE | No polling state: dry-run/commit are synchronous (25s budget); status surfaces via list/detail endpoints. Replaced by T1's two-phase tests. |
| T7 | Negative pengeluaran stored verbatim | ✅ PASS | New `test_cell_number_keeps_negative_sign_exactly` (int, `"-1,500,000"` text, `.5` float → exact `Decimal`). DB columns are numeric. |

### Commands run (2026-10-07)
```
backend/        go build ./... && go vet ./internal/handler/ ./internal/service/   -> ok
backend/        go test ./...                                                       -> all ok (DB integration tests skip without DATABASE_URL)
backend/        go test ./internal/handler/ -run TestDsr -v                         -> 3/3 PASS (T5)
backend/        go test ./internal/service/ -run "WriteAndDryRun|ConfirmUpload" -v  -> 3/3 PASS
backend_python/ python -m unittest discover -s dsr -v                               -> 10/10 OK
backend/        golangci-lint run ./internal/handler/ ./internal/service/           -> 2 errcheck, both pre-existing in admin_master_data_import_handler.go (not DSR)
```

### Outstanding
- T2 live-DB integration test (re-upload same / changed checksum) — not run.
- Coverage target "≥80% on `internal/dsr`" is stale: that package does not exist (DSR lives in `internal/service` + `internal/handler`).
- Manual browser verification of the vendor upload flow (dry-run preview → confirm) — **outstanding, user** (Golden Rule #10).
