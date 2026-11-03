# import-export-jobs — Review (stage 5)

Reviewed 2026-09-29 by code-reviewer, database-reviewer, python-reviewer (report-only agents). No CRITICAL findings.

## Fixed (T8.2)
| Finding | Fix |
|---|---|
| Job stranded in `processing` when anything raises after `start()` (missing script, DB blip, cancel) | `SchedulerService._run_attempt` fails the job (shielded) before re-raising; `RetryExecutor` timeout (0.9 × stale limit) kills a hung ETL |
| Manual retry accepted another service's job → `KeyError` 500 + stranded job | `source not in file_types` → 404; history/`/audit`/`/late`/summary late count scoped to the service's `FILE_TYPES`; `/retry` of a `pending` row → 409 |
| Stale-swept rows never counted toward `max_retries` (retry forever) | `mark_stale`: stale `processing` row = +1 `auto_retry_count`, exhausts at `max_retries`; stale `pending` does not count |
| ETL finishing after the stale sweep → uncaught `IllegalTransitionError`, job stays `failed` | `_finish_retry` re-enters `processing` → `completed` (or leaves `failed`), late detection resolved |
| No RBAC on retry/monitoring (Sec 5) | `require_eod_role`: jwt mode needs ADMIN/APPACCESS access token; api_key mode is a service credential |
| JWT: refresh tokens, no `exp`, non-bearer scheme, non-constant-time key compare, default secret | role claim required (rejects refresh tokens), `require_exp`, bearer scheme in jwt mode, `hmac.compare_digest`, startup refuses empty/`change_me` secret |
| Event loop blocked by detector scan; unbounded stderr stored; fabricated `job_id`; bigint overflow → 500; broken `__main__` smoke check; misplaced comment | `asyncio.to_thread`; error detail = last 1000 chars; `job_id` removed (API + frontend type); `Path(ge=1, le=2**63-1)`; smoke check fixed |

Tests added: executor error/other-service/pending/late-finish (scheduler), stale counting (import_jobs), `test_retry_executor.py` (timeout, bounded detail), role gate + token hardening + id bounds (API). 55 lib tests + DSR 7 + ITM 3 green.

## Fixed after decision A (2026-09-29)
- **A. Several files per source+date superseded each other** (DB H1, code M3, python M7): migration `020` narrows `import_jobs_current_uq` to `detection_source = 'upload'`; `complete()` only supersedes for uploads (and then all other completed rows of the date); detected files never supersede. Test: `test_detected_files_of_one_date_all_complete_and_an_upload_supersedes_them`; index probed on dev (detected rows allowed, second completed upload rejected with 23505).

## Open
- **C. Path traversal** in `service_dsr_etl` `/process/dsr/{dry-run,commit}`: fixed (pre-existing code, outside import_jobs). `DsrFileRequest.filename` must be a bare file name (no separators, drive colon, control chars, leading `.`/`-`; 422 otherwise) and `dsr_etl.safe_child()` re-checks it for `--mode dry_run|commit`. Tests: `SafeChildTests`, `DsrProcessFilenameTest`.

## Deferred (D, decided 2026-09-29)
DB-enforced status machine trigger; `version`/`supersedes_job_id` for `detect` rows; `file_type` varchar(20) → text; index tuning (`retry_idx`, `list_idx`, unused indexes); preflight `to_regclass('retry_file_tracking')` in deploy runbook.

## Accepted / not done
- DSR legacy "done" check (`dsr_uploads` any vendor completed) follows C3/FR14 literally — confirm intent with PO.
- `detect` files under today's `processing_date` even if older (M6) — documented behaviour, revisit with 2.1.
- `/audit` unbounded, `mark_stale` writes no `retry_audit_logs` (FR18 minor), `test_eod_api` uses real sources `dmaa`/`dsr` at 2099 dates (a live scheduler on the same DB could pick them up).
- Deploy note: services now refuse to start until `RETRY_AUTH_SECRET` / `DSR_ETL_AUTH_SECRET` is set (dev `backend_python/.env` has neither).
