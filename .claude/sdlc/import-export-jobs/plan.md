# Plan & Task List: `import_jobs` (menyatukan `retry_file_tracking`)

Status: **draft — menunggu OK engineer** (2026-09-28). Stage: 3 Build. Reads: `spec.md` revisi 2 (S1–S8, C1–C4).
Aturan: belum ada kode ditulis. Setiap task selesai → centang di sini; diff tetap sinkron dengan file ini (Sec 4a).
Sec 4 #7: T1.2 (apply migrasi ke dev DB) **STOP & minta izin** sebelum dijalankan.

## Keputusan tambahan saat plan (2026-09-28)
| # | Topik | Keputusan |
|---|---|---|
| P1 | Tanggal ITM | Tanggal di nama file ITM = **tanggal tiba (X)** — jawaban user. FR14: `itm_*_files.file_date = processing_date`, tanpa offset. |
| P2 | Tanggal DMAA/DSR | Dari kode: `dmaa_files.file_date` = tanggal mtime file (≈ tanggal tiba); `dsr_uploads.report_date` = hari itu (keputusan DSR #1). Keduanya dicocokkan ke `processing_date` tanpa offset. |
| P3 | Driver Python helper | `asyncpg` saja (yang dipakai `lib/services`). ETL lama memakai psycopg sinkron, tapi tidak mengadopsi helper (keputusan #1) → tidak perlu versi sinkron sekarang (YAGNI). |
| P4 | Actor audit dari API Python | `require_auth` mengembalikan `sub` JWT (string) atau `"api_key_user"`. Retry manual menulis `audit_logs` **hanya** bila `sub` adalah `users.id` numerik yang ada; selain itu cukup `retry_audit_logs` (+ log). |
| P5 | Konsumen helper Go | Belum ada pemanggil (forecast 2.1 belum dibangun). Helper Go dibangun + dites sekarang karena kontrak Go=Python (FR11) harus terbukti sejak awal; pemakaian pertama di 2.1. |

## Kondisi awal (kode yang ada sekarang)
- `backend_python/lib/services/detector.py` — `persist_detected_files` INSERT ke `retry_file_tracking` (`ON CONFLICT (file_checksum, processing_date) DO NOTHING`), `record_scan_run` → `scan_runs`, `LateDetector` → `late_detections`.
- `backend_python/lib/services/scheduler_service.py` — `_run_auto_retries`, `process_auto_retry`, `process_manual_retry`, `_run_late_check` (catatan `ponytail:` L161), semua ke `retry_file_tracking`; id `UUID`.
- `backend_python/lib/services/audit_service.py` — append-only ke `retry_audit_logs`, `file_id: UUID`.
- `backend_python/{eod_retry_scheduler,service_dsr_etl}/routers/{status,retry,audit,late,summary}.py` + `lib/schemas.py` — `file_id: UUID`. Kedua service punya salinan router yang hampir identik.
- Tabel `retry_file_tracking`, `late_detections`, `scan_runs`, `retry_audit_logs` **tidak ada di DB** (hanya di arsip `012_retry_scheduler.sql`).
- Go: pola `internal/audit.Writer` (`Write(ctx, Entry)` pada `db.DBTX`), sqlc v1.31.1 (`queries/` + `migrations/`), mapping `23505` sudah dipakai di `internal/repository/masterdata_import_batch_repository.go`, integration test repo memakai `DATABASE_URL` (skip bila kosong).
- Frontend `features/eod-monitoring/types.ts` — `ProcessingStatus` sudah punya `max_retries_exhausted`; **belum** punya `superseded`. `file_id: string`.
- Migrasi terakhir: `018_regions_soft_delete.sql` → berikutnya `019`.

## Inti perubahan (satu paragraf)
Buat `import_jobs` + 3 tabel pendamping retry di `019`. Satu helper per runtime (Go `internal/importjob`, Python `lib/import_jobs.py`) memegang semua transisi status, dengan partial unique index sebagai penjaga konkurensi. Retry scheduler Python berhenti menyentuh SQL `retry_file_tracking` dan memanggil helper Python. Cek SLA membaca `import_jobs` **atau** tabel sumber lama. Router Python tetap berbentuk sama, hanya `file_id` jadi string angka. Frontend hanya menambah label `superseded`.

---

## Task list
Legenda: `[ ]` todo · `[~]` in progress · `[x]` done · **Validate** = bukti task selesai.

### Fase 1 — Skema
- [ ] **T1.1** Tulis `backend/migrations/019_import_jobs.sql` sesuai spec (tabel, 3 partial unique index, 3 index biasa, trigger `updated_at` mengikuti pola baseline) + `late_detections`, `scan_runs`, `retry_audit_logs` dari `012` (arsip) dengan `retry_audit_logs.file_id bigint NULL REFERENCES import_jobs(id)`; id tabel pendamping tetap `uuid` (tidak dirujuk FK lain). `BEGIN/COMMIT`, forward-only, header SAFETY seperti `005_…`.
  - Validate: file ter-parse oleh `sqlc generate` (T1.3); review manual index predicate vs FR2/FR4/FR6.
- [ ] **T1.2** ⛔ **STOP — minta izin user**, lalu apply `019` ke dev DB via `localhost:5432`.
  - Validate: `\d import_jobs` menampilkan 3 partial unique index; 4 tabel ada; `retry_file_tracking` tidak ada.
- [ ] **T1.3** `backend/queries/import_jobs.sql`: `InsertImportJob`, `GetLiveByHash` (non-superseded), `GetInflight`, `GetCurrentCompleted`, `MarkProcessing`, `MarkCompleted`, `MarkFailed`, `SupersedeCurrent`, `MarkStaleFailed`, `ResetFailedToPending`. `sqlc generate` (v1.31.1) → hand-fix `UserLeafe`→`UserLeave`.
  - Validate: `git diff --stat backend/internal/db/` hanya file yang diharapkan + `import_jobs.sql.go`/`models.go`; `go build ./...` hijau.

### Fase 2 — Helper Go (`backend/internal/importjob`)
- [ ] **T2.1** `errors.go`: `ErrJobInProgress`, `ErrIllegalTransition`, `ErrNotFound`.
- [ ] **T2.2** `service.go`: `Register(ctx, RegisterInput) (Job, duplicate bool, err)` — urutan: tandai stale (FR8) → cari baris hidup per hash (FR2; `failed`/`max_retries_exhausted` → reset `pending`, FR3) → hitung versi dari `completed` terkini (FR5/FR5a) → insert; `23505` pada `import_jobs_inflight_uq` → `ErrJobInProgress`, pada `import_jobs_hash_uq` → re-read + `duplicate=true`.
  - Validate: unit test table-driven untuk pemetaan constraint-name → error.
- [ ] **T2.3** `Start`, `Complete`/`CompleteTx(ctx, tx, …)` (supersede + complete dalam tx pemanggil, FR6), `Fail` (potong `error_message` ≤ 1000 char, FR7), `Current` (FR9). Transisi divalidasi terhadap tabel transisi FR10 (satu map, bukan if-bertingkat).
- [ ] **T2.4** Audit (FR18): bila `created_by`/actor terisi → `audit.Writer.Write` dalam tx yang sama (`entity_type='import_job'`, before/after `status`); actor kosong → `slog` terstruktur saja.
- [ ] **T2.5** Stale threshold: `IMPORT_JOB_STALE_AFTER` di `pkg/config` (default 15m, guard minimum seperti `SessionMaxLifetime`), diteruskan ke `importjob.New(...)`. `.env.example` diperbarui.
  - Validate: `pkg/config` test — kosong → 15m; `30m` → 30m; ngawur → 15m.
- [ ] **T2.6** Integration test `importjob` (skip bila `DATABASE_URL` kosong, pola `region_admin_repository_test.go`): A1–A9, A4a, A5a, A16 (sisi Go). A4 memakai dua goroutine + `sync.WaitGroup`.
  - Validate: `go test ./internal/importjob/... -cover` hijau, coverage ≥ 80%.

### Fase 3 — Helper Python (`backend_python/lib/import_jobs.py`)
- [ ] **T3.1** Fungsi async setara T2.2–T2.4 di atas `asyncpg` (`register`, `start`, `complete(conn=…)`, `fail`, `current`, `mark_stale`); exception `JobInProgressError`, `IllegalTransitionError`; mapping `asyncpg.UniqueViolationError.constraint_name`. Batas stale **per sumber** (C1): `IMPORT_JOB_STALE_AFTER` default + override per `file_type` dari config service (usulan 60m untuk `dmaa`/`itm_*`/`dsr`).
- [ ] **T3.2** Pytest integration (skip bila `DATABASE_URL` kosong): A1–A9, A4a, A5a sisi Python.
  - Validate: `pytest backend_python/lib/test_import_jobs.py` hijau.

### Fase 4 — Kontrak Go = Python (FR11)
- [ ] **T4.1** `backend/internal/importjob/testdata/contract_cases.json`: daftar skenario (langkah register/start/complete/fail + ekspektasi status/version/duplicate/error). Satu file, dibaca oleh kedua runner.
- [ ] **T4.2** Runner Go (`contract_test.go`) dan runner Python (`backend_python/lib/test_import_jobs_contract.py`, path relatif ke JSON), masing-masing membersihkan baris uji dengan `source` prefix `test_contract_`.
  - Validate: A10 — kedua runner hijau terhadap DB yang sama.

### Fase 5 — Adopsi retry scheduler (Python)
- [ ] **T5.1** `detector.py`: `persist_detected_files` → `import_jobs.register(..., detection_source=…, status='failed', runtime='python')` (idempotent via FR2). `record_scan_run`/`LateDetector` tetap (tabelnya kini ada).
  - Validate: A11 — scan dua kali → satu baris.
- [ ] **T5.2** `scheduler_service.py`: `_run_auto_retries`, `process_auto_retry`, `process_manual_retry` → helper Python (transisi FR10; `JobInProgressError` di auto-retry = lewati baris, log info). Hapus SQL `retry_file_tracking` langsung.
  - Validate: A12 — gagal ×3 → `max_retries_exhausted`; manual retry lewat batas; `completed` → `RetryConflictError` (409).
- [ ] **T5.3** `_run_late_check` (FR14/C3): selesai bila `import_jobs` `completed` **atau** tabel lama sukses. Pemetaan satu dict per `file_type` (P1/P2):
  `dmaa → dmaa_files status='success' file_date` · `itm_cashpos → itm_cashpos_files status='completed' file_date` · `itm_replenish → itm_replenish_files status='completed' file_date` · `dsr → dsr_uploads daily_status='completed' report_date`. Nama tabel/kolom dari dict konstanta, bukan input user (tidak ada SQL injection). Hapus catatan `ponytail:` L161.
  - Validate: A13 — per sumber: sukses lama → tidak late; kosong keduanya → late; retry sukses → `late_detections` resolved.
- [ ] **T5.4** `audit_service.py` + actor (P4): `file_id: int | None`; retry manual dengan `sub` numerik yang ada di `users` → juga `audit_logs` via helper.
- [ ] **T5.5** Router kedua service (`status`, `retry`, `audit`, `late`, `summary`) + `lib/schemas.py`: `file_id` path param `int` (FastAPI menolak non-angka → 422), respons `str(id)`; query `retry_file_tracking` → `import_jobs` (`processing_status` di respons = kolom `status`, nama field JSON tidak berubah). Summary menghitung `superseded` terpisah, tidak masuk `failed`.
  - Validate: A15 — test router (httpx `AsyncClient`) untuk kedua service; `file_id` string angka.
- [ ] **T5.6** Config per service: override stale per `file_type` (C1) di `eod_retry_scheduler/config.py` + `service_dsr_etl/config.py`.

### Fase 6 — Frontend (`features/eod-monitoring`)
- [ ] **T6.1** `types.ts`: tambah `superseded` ke `ProcessingStatus`, `STATUS_*` label ("Digantikan") + ikon (bukan warna saja, Sec 13). `RetryDrawer`: `superseded` tidak bisa di-retry.
  - Validate: `npm test` (component test badge `superseded` + tombol retry tersembunyi), `npm run typecheck`, `npm run build` hijau.

### Fase 7 — Dokumentasi
- [ ] **T7.1** CLAUDE.md Sec 3: `internal/importjob` di Platform Core; Core tables: `import_jobs` (kolom ringkas) + `late_detections`/`scan_runs`/`retry_audit_logs`; catat `retry_file_tracking` digantikan.
- [ ] **T7.2** CLAUDE.md Sec 12: keputusan S7 (satukan), S4 (deviasi audit sistem), C3 (cek SLA baca tabel lama).
- [ ] **T7.3** `.claude/development-progress.md`, `.claude/sdlc/README.md`, `.kiro/steering/development-plan.md` (0.2 → done); `graphify update .`.

### Fase 8 — Verifikasi (→ `tests.md`)
- [ ] **T8.1** `go test ./...` (backend + pkg), `pytest backend_python`, `npm test && npm run typecheck && npm run build` (CompanyPortal) — semua hijau.
- [ ] **T8.2** Review: `code-reviewer` + `database-reviewer` (migrasi/index) + `python-reviewer`; perbaiki CRITICAL/HIGH.
- [ ] **T8.3** A17 manual browser check halaman EOD monitoring — **outstanding, dikerjakan user**, setelah task routing `/api/eod` (S6) selesai.

## Urutan & ketergantungan
`T1.1 → T1.2 (izin) → T1.3 → Fase 2 ∥ Fase 3 → Fase 4 → Fase 5 → Fase 6 → Fase 7 → Fase 8`.
Fase 2 dan 3 independen setelah skema ada. Fase 5 butuh Fase 3. Fase 6 tidak bergantung pada backend (hanya tipe).

## Risiko
| Risiko | Mitigasi |
|---|---|
| Dua implementasi (Go/Python) drift | Satu file kontrak JSON dijalankan kedua runtime (Fase 4); perubahan perilaku wajib menambah kasus di sana. |
| Partial unique index + `ON CONFLICT` di asyncpg butuh predicate yang sama persis | Helper memakai tangkap `UniqueViolationError` + re-read, bukan `ON CONFLICT … WHERE`, agar tidak bergantung pada pencocokan predicate. |
| Retry executor menjalankan ETL per `file_type`, bukan per file: satu run sukses tidak berarti file baris itu yang diproses | Perilaku lama, tidak diubah; dicatat. Status baris mengikuti hasil run seperti sekarang. |
| Stale 60 menit untuk ETL batch hanya usulan | Nilai via config, ukur durasi ETL di dev sebelum prod (flag spec). |
| Halaman EOD tidak bisa dicek manual sebelum S6 | A17 tetap outstanding; test otomatis router + komponen tetap wajib hijau. |
| Mengubah tipe `file_id` UUID → int di Python | Respons tetap string; FastAPI validasi path int (422 untuk UUID lama) — tidak ada data lama karena tabel tak pernah ada. |

## Bukti selesai
Semua task `[x]`; output test T8.1 dicatat di `tests.md` dengan trace A1–A17 → test; migrasi `019` tercatat applied di CLAUDE.md; A17 tertulis **outstanding (user)**.
