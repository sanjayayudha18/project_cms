# DSR Tables Drop - Code Reference Map

> Companion to `backend/migrations/019_drop_dsr_tables.sql`.
> Maps every place in the codebase that references the four DSR tables being dropped:
> `atm_dsr_saldo_files`, `atm_dsr_saldo_rows`, `atm_dsr_rencana_isi_files`,
> `atm_dsr_rencana_isi_rows`.
>
> Purpose: know the full blast radius before applying the drop. Dropping the tables
> WITHOUT handling these references will break the Go build (sqlc), the Python ETL,
> and leave the vendor-portal DSR feature calling dead endpoints.

## Summary

The four tables are the backing store for the **vendor-upload-dsr** feature. They are
referenced across four layers:

1. **DB migrations** (define the tables)
2. **Go backend** (sqlc schema + queries + generated models + service + handler + route)
3. **Python ETL** (separate ingest path that writes the same tables directly)
4. **Frontend** (vendor portal DSR page - currently on mock JSON, not the live API)

Dropping the tables is DDL-only and reversible via re-applying 013/017/018, but the
**Go build will fail** the moment sqlc regenerates against a schema missing these tables,
because `backend/queries/dsr.sql` selects from them. See "Ordering" at the bottom.

---

## 1. DB migrations (source of the schema)

| File | What it does |
|------|--------------|
| `backend/migrations/013_dsr.sql` | Creates `atm_dsr_saldo_files` (L51) and `atm_dsr_saldo_rows` (L112) + FK saldo_rows -> saldo_files |
| `backend/migrations/017_atm_dsr_location_and_rencana_isi.sql` | Adds `location` / `saldo_gabungan_total_idr` to saldo tables; creates `atm_dsr_rencana_isi_files` (L42) and `atm_dsr_rencana_isi_rows` (L76) + FKs to files, atms, atms.terminal_id |
| `backend/migrations/018_dsr_upload_accountability.sql` | Adds `uploaded_by_user_id` (+ FK -> users) to both *_files tables |
| `backend/migrations/014_updated_at_and_fk_indexes.sql` | References `atm_dsr_saldo_files` in the set_updated_at trigger loop (L72) and rollback comment (L27). NOT dropped, but the trigger on the dropped table vanishes with it - harmless. |
| `backend/migrations/019_drop_dsr_tables.sql` | NEW - the drop migration itself |

Note: the drop reverses 013 + 017 + the 018 columns (columns die with the tables).

---

## 2. Go backend - the live feature (biggest blast radius)

### 2a. sqlc schema + queries (build breaks here first)
| File | Reference |
|------|-----------|
| `backend/queries/dsr.sql` | 11 named queries SELECT/read from all four tables. Named: `GetDsrSaldoFileByChecksum`, `GetDsrSaldoFileByReportDateVendor`, `GetDsrRencanaIsiFileByChecksum`, `GetDsrRencanaIsiFileByReportDateVendor`, `GetDsrSaldoFileByIDForVendor`, `GetDsrRencanaIsiFileByIDForVendor`, `ListDsrSaldoRowErrors`, `ListDsrRencanaIsiRowErrors`, `ListDsrSaldoRows`, `ListDsrRencanaIsiRows`, `ListDsrReportDatesByVendor`, `CountDsrReportDatesByVendor`. **These must be removed/rewritten BEFORE `sqlc generate`, or codegen fails.** |

### 2b. sqlc-generated code (regenerated from 2a)
| File | Reference |
|------|-----------|
| `backend/internal/db/models.go` | Structs `AtmDsrRencanaIsiFile` (L37), `AtmDsrRencanaIsiRow` (L59), `AtmDsrSaldoFile` (L81), `AtmDsrSaldoRow` (L123) |
| `backend/internal/db/dsr.sql.go` | All 12 generated `func (q *Queries) *Dsr*` methods + their param/row structs |

### 2c. Hand-written service + handler + wiring
| File | Reference |
|------|-----------|
| `backend/internal/service/dsr_upload.go` | `DsrRepository` interface (L49-63) lists all 12 query methods; `DsrService` methods `GetSaldoFile`, `GetRencanaIsiFile`, list/count, DTO mappers (`saldoRowsToDto`, `rencanaIsiRowsToDto`, etc.) |
| `backend/internal/service/dsr_upload_test.go` | `fakeDsrRepository` mocks all 12 methods |
| `backend/internal/handler/dsr_upload_handler.go` | `DsrUploadHandler`, `NewDsrUploadHandler`, `Upload`, `Routes()` |
| `backend/cmd/api/main.go` | L136-138: builds `dsrService`, `dsrHandler`, mounts `/api/v1/dsr` |

---

## 3. Python ETL (separate ingest path, writes the tables directly)

| File | Reference |
|------|-----------|
| `backend_python/dsr/dsr_etl.py` | Raw SQL: SELECT/DELETE/INSERT/UPDATE on `atm_dsr_saldo_files` (L468/476/482/516), `atm_dsr_saldo_rows` (L505), `atm_dsr_rencana_isi_files` (L532/540/546/581), `atm_dsr_rencana_isi_rows` (L570) |
| `backend_python/dsr/test_dsr_etl.py` | Tests for the ETL above (check for table refs) |

This path does NOT go through sqlc - it will silently fail at runtime (not build time)
against a missing table. Decide whether it is being kept, rewritten, or retired.

---

## 4. Frontend - vendor portal DSR feature

Currently reads MOCK JSON (`src/data/dsr.json`), not the live `/api/v1/dsr` API, so it
will NOT break at build time. But it is the eventual consumer of these tables.

| File | Reference |
|------|-----------|
| `frontend/VendorPortal-Vite/src/features/dsr/DsrPage.tsx` | DSR Monitor page (Saldo Harian / Rencana Isi columns) |
| `frontend/VendorPortal-Vite/src/features/dsr/DsrUploadDialog.tsx` | Upload dialog |
| `frontend/VendorPortal-Vite/src/features/dsr/DsrDetailDialog.tsx` | Detail view |
| `frontend/VendorPortal-Vite/src/features/dsr/dsrUploadApi.ts` | API client types (`DsrUploadListItem`, `DsrUploadSheetSummary`) |
| `frontend/VendorPortal-Vite/src/features/dsr/useDsrUploads.ts` | TanStack Query hook |
| `frontend/VendorPortal-Vite/src/routes/dsr.tsx` | `/dsr` route |
| `frontend/VendorPortal-Vite/src/main.tsx` | Route registration |
| `frontend/VendorPortal-Vite/src/lib/constants.ts` | `ROUTES.DSR`, NAV "DSR Menu" |
| `frontend/VendorPortal-Vite/src/lib/types.ts` | `DsrRecord` interface |
| test files under `src/routes/__tests__/`, `src/lib/__tests__/`, `src/test/` | route + data-filter property tests |

---

## 5. Non-code / doc references (informational only, safe to ignore)

- `graphify-out/` (graph.json, graph.html, GRAPH_REPORT.md) - stale after drop; run `graphify update .`
- `.claude/sdlc/vendor-upload-dsr/*` - the feature's own SDLC docs

---

## Ordering (to avoid a broken build)

If you intend to keep the code but drop only the data, DO NOT run the migration in
isolation. Recommended sequence:

1. Decide the fate of the Go feature + Python ETL + FE (keep / rewrite / retire).
2. If retiring: remove `backend/queries/dsr.sql` DSR queries FIRST, then remove/adjust
   `dsr_upload.go`, handler, route in `main.go`, and tests.
3. Run `sqlc generate` and `go build ./...` to confirm green BEFORE touching the DB.
4. Only then apply `019_drop_dsr_tables.sql`.
5. Retire or rewrite `backend_python/dsr/dsr_etl.py` (runtime, not build).
6. `graphify update .` to refresh the knowledge graph.

If instead you just want a clean slate on the DATA (keep schema + code), you do NOT want
019 at all - you want `TRUNCATE` the four tables (or DELETE the *_files rows, children
cascade). Flag which of these you actually intend.
