# Tests / feedback loop: vendor upload DSR

Status: draft. Stage: 4 Test. Reads: `plan.md`.
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
| T4 | FR5 | Stated SALDO AKHIR vs SUM() | cross-check recorded / flagged on mismatch |
| T5 | FR8 | Vendor A reads Vendor B upload | 403 / not found (RBAC denial) |
| T6 | FR7 | GET status during processing | returns pending|processing|completed|failed |
| T7 | money | negative pengeluaran sign | stored verbatim, saldo math holds |

## Verification block (goes to CLAUDE.md if not already there)
Run build + test + lint before reporting any task complete; paste output.
If a test fails, fix the code, not the test.
