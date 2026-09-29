# Plan & Task List: `import_jobs` (menyatukan `retry_file_tracking`)

Status: **accepted — build berjalan** (draft 2026-09-28; direvisi setelah review 2026-09-29; OK engineer 2026-09-29). Stage: 3 Build. Reads: `spec.md` revisi 2 + koreksi 2026-09-29 (S1–S8, C1–C4).
Gate Sec 4a: `spec.md` diterima PO 2026-09-29.

## Hasil review 2026-09-29
- **R2 DMAA timezone — diputuskan (a):** TZ VM/container yang menjalankan ETL Python = `Asia/Jakarta` adalah **syarat deploy** (T7.4). Mapping T5.3 `dmaa` tetap `file_date = $date` tanpa offset.
- **R3 Gate spec — selesai:** PO menerima `spec.md` 2026-09-29.
- **R5 Identitas JWT di API Python — SELESAI 2026-09-29 (izin user):** `lib/dependencies.token_identity()` → `sub` bila ada, selain itu `str(payload["id"])`, selain itu `"unknown"`; validasi token tidak berubah. Diuji end-to-end di `lib/test_eod_api.py` (token berbentuk Go → `audit_logs` dengan actor + IP). Catatan asal: token akses Go (`pkg/auth/token_service.go` `AccessTokenClaims`) menyimpan user id di claim **`id`** (int64) dan tidak mengisi `sub`; `lib/dependencies.require_auth` mengembalikan `payload.get("sub", "unknown")` → selalu `"unknown"`. Asumsi P4 ("`sub` = `users.id`") salah. Usulan: `require_auth` mengembalikan `sub` bila ada, selain itu `str(payload["id"])`. Menunggu izin user; tanpa itu retry manual hanya tercatat di `retry_audit_logs` (`initiated_by="unknown"`).
- **R4 Helper Go — diputuskan: ditunda ke forecast 2.1.** Fase 2 + Fase 4 dihapus dari plan ini; T1.3 dikecilkan (model sqlc saja, tanpa queries). Kontrak Go=Python (FR11) dibangun bersama helper Go di 2.1, memakai Python helper ini sebagai acuan.
Aturan: belum ada kode ditulis. Setiap task selesai → centang di sini; diff tetap sinkron dengan file ini (Sec 4a).
Sec 4 #7: T1.2 (apply migrasi ke dev DB) **STOP & minta izin** sebelum dijalankan.

## Keputusan tambahan saat plan (2026-09-28)
| # | Topik | Keputusan |
|---|---|---|
| P1 | Tanggal ITM | Tanggal di nama file ITM = **tanggal tiba (X)** — jawaban user. FR14: `itm_*_files.file_date = processing_date`, tanpa offset. |
| P2 | Tanggal DMAA/DSR | Dari kode: `dmaa_files.file_date` = tanggal mtime file (≈ tanggal tiba) **dalam TZ host** — benar karena TZ host = `Asia/Jakarta` (R2a); `dsr_uploads.report_date` = hari itu (keputusan DSR #1). Dicocokkan ke `processing_date` tanpa offset. |
| P6 | Detector vs FR3 | Detector memakai `detect()` (insert-if-absent), bukan `register()` — `register()` akan me-reset baris `failed`/`max_retries_exhausted` ke `pending` tiap scan (FR3) sehingga `max_retries` tak pernah berlaku. (review 2026-09-29, spec FR12) |
| P3 | Driver Python helper | `asyncpg` saja (yang dipakai `lib/services`). ETL lama memakai psycopg sinkron, tapi tidak mengadopsi helper (keputusan #1) → tidak perlu versi sinkron sekarang (YAGNI). |
| P4 | Actor audit dari API Python | `require_auth` mengembalikan `sub` JWT (string) atau `"api_key_user"`. Retry manual menulis `audit_logs` **hanya** bila `sub` adalah `users.id` numerik yang ada; selain itu cukup `retry_audit_logs` (+ log). |
| P5 | Konsumen helper Go | Belum ada pemanggil (forecast 2.1 belum dibangun) → **ditunda ke 2.1** (R4, 2026-09-29). Selama hanya ada satu implementasi, tidak ada risiko drift. |

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
- [x] **T1.1** (2026-09-29; `sqlc compile` v1.31.1 OK; + `idx_retry_audit_file` untuk FK, + trigger `updated_at` di `late_detections`) Tulis `backend/migrations/019_import_jobs.sql` sesuai spec (tabel, 3 partial unique index, 3 index biasa, trigger `updated_at` mengikuti pola baseline) + `late_detections`, `scan_runs`, `retry_audit_logs` dari `012` (arsip) dengan `retry_audit_logs.file_id bigint NULL REFERENCES import_jobs(id)`; id tabel pendamping tetap `uuid` (tidak dirujuk FK lain). `BEGIN/COMMIT`, forward-only, header SAFETY seperti `005_…`.
  - Validate: file ter-parse oleh `sqlc generate` (T1.3); review manual index predicate vs FR2/FR4/FR6.
- [x] **T1.2** (applied ke dev DB 2026-09-29 atas izin user; verifikasi OK: 4 tabel, 3 partial unique index, FK `retry_audit_logs_file_id_fkey`, 2 trigger, `retry_file_tracking` absent) ⛔ **STOP — minta izin user**, lalu apply `019` ke dev DB via `localhost:5432`.
  - Validate: `\d import_jobs` menampilkan 3 partial unique index; 4 tabel ada; `retry_file_tracking` tidak ada.
- [x] **T1.3** (2026-09-29; diff = `models.go` +72, 4 struct baru; `UserLeafe` fix di `approval.sql.go`+`models.go`; `go build`/`go vet`/`go test ./...` hijau) `sqlc generate` (v1.31.1; schema = `migrations/`) → hand-fix `UserLeafe`→`UserLeave`. Tanpa `queries/import_jobs.sql` (R4); hanya struct model baru di `models.go`, supaya generate berikutnya tidak membawa diff liar.
  - Validate: `git diff --stat backend/internal/db/` hanya `models.go`; `go build ./...` hijau.

### Fase 2 — Helper Go — **DITUNDA ke forecast 2.1 (R4)**
Tidak dikerjakan di fitur ini. Di 2.1: `backend/internal/importjob` (Register/Start/Complete/CompleteTx/Fail/Current, `ErrJobInProgress`), `queries/import_jobs.sql`, `IMPORT_JOB_STALE_AFTER` di `pkg/config`, integration test + audit via `audit.Writer`, dan kontrak Go=Python (FR11) dengan helper Python di bawah sebagai acuan perilaku.

### Fase 3 — Helper Python (`backend_python/lib/import_jobs.py`) — satu-satunya implementasi
- [x] **T3.1** (2026-09-29; `lib/import_jobs.py` + demo `__main__`; smoke ke dev DB dalam tx rollback: A1/A2/A3/A4/A5/A6/A7/A9 + detect + audit OK; stale diuji lintas tx lalu dibersihkan. Catatan: transisi sistem hanya `logger` di helper — `retry_audit_logs` tetap ditulis `AuditService` untuk event retry, karena kolomnya khusus retry (`event_type`, `processing_date NOT NULL`). T3.2: test stale butuh jeda nyata di luar tx, karena trigger `set_updated_at` menimpa `updated_at` dan `now()` beku dalam tx.) Fungsi async di atas `asyncpg`; transisi divalidasi terhadap satu map FR10 (bukan if-bertingkat):
  - `register(...) -> (job, duplicate)`: tandai stale (FR8) → cari baris hidup per hash (FR2; `failed`/`max_retries_exhausted` → reset `pending`, FR3) → hitung versi dari `completed` terkini (FR5/FR5a) → insert. `UniqueViolationError.constraint_name`: `import_jobs_inflight_uq` → `JobInProgressError`; `import_jobs_hash_uq` → re-read + `duplicate=True`.
  - `detect(...)`: INSERT status `failed` (FR10 status awal); 23505 pada `import_jobs_hash_uq` → no-op; **tanpa** FR3 reset (P6).
  - `start`, `complete(conn=…)` (supersede + complete dalam tx pemanggil, FR6), `fail` (potong `error_message` ≤ 1000 char, FR7), `current` (FR9), `mark_stale`.
  - Exception: `JobInProgressError`, `IllegalTransitionError`.
  - Batas stale **per sumber** (C1): default 15m + override per `file_type` dari config service (usulan 60m untuk `dmaa`/`itm_*`/`dsr`).
  - Audit (FR18): actor user → `audit_logs` dalam tx yang sama; sistem → `retry_audit_logs` + log terstruktur.
- [x] **T3.2** (2026-09-29; `lib/test_import_jobs.py`, **unittest** stdlib `IsolatedAsyncioTestCase` — pytest/pytest-asyncio/coverage tidak terpasang, test yang ada juga unittest; 23 test hijau vs dev DB, 0 baris sisa, skip bersih tanpa `DATABASE_URL`. Coverage via stdlib `trace`: mentah 76%, tapi 40 baris "miss" = artefak signature multi-baris + 12 baris demo `__main__`; nyata 180/183 statement ≈ 98%, sisa 3 = `raise` defensif constraint lain. Jalankan: `python -m unittest lib.test_import_jobs -v`.) Pytest integration (skip bila `DATABASE_URL` kosong): A1–A9, A4a, A5a, A16. A4 memakai dua koneksi paralel (`asyncio.gather`).
  - Validate: `pytest backend_python/lib/test_import_jobs.py --cov=lib.import_jobs` hijau, coverage ≥ 80%.

### Fase 4 — Kontrak Go = Python (FR11) — **DITUNDA ke forecast 2.1 (R4)**

### Fase 5 — Adopsi retry scheduler (Python)
- [x] **T5.1** (2026-09-29; signature `persist_detected_files` tetap, `failure_reason` → `error_message`; A11 di `lib/services/test_detector.py` hijau, termasuk scan hari berikutnya tidak menghidupkan baris `max_retries_exhausted`; demo `detector.py` OK) `detector.py`: `persist_detected_files` → `import_jobs.detect(..., detection_source=…, runtime='python')` (P6 — **bukan** `register`). `record_scan_run`/`LateDetector` tetap (tabelnya kini ada).
  - Validate: A11 — scan dua kali → satu baris; baris `failed`/`max_retries_exhausted` yang ada tidak berubah status/`auto_retry_count`.
- [x] **T5.2** (2026-09-29; A12 + FR8 + FR13 di `lib/services/test_scheduler_service.py`, 7 test hijau, `retry_executor` palsu + DB nyata. Tambahan: manual retry pada baris `superseded`, pada baris yang sedang `processing`, atau saat ada run lain untuk source+date → `RetryConflictError` (409); pesan gagal kosong dari ETL → `ETL exited with code N`. `process_manual_retry(file_id: int, …)` — router masih UUID sampai T5.5.) `scheduler_service.py`: `_run_auto_retries`, `process_auto_retry`, `process_manual_retry` → helper Python (transisi FR10; `JobInProgressError` di auto-retry = lewati baris, log info). `_run_auto_retries` memanggil `mark_stale` di awal siklus (FR8 — baris `processing` macet tidak menunggu register baru). Hapus SQL `retry_file_tracking` langsung.
  - Validate: A12 — gagal ×3 → `max_retries_exhausted`; manual retry lewat batas; `completed` → `RetryConflictError` (409); baris `processing` lebih tua dari batas → `failed` di siklus berikutnya.
- [x] **T5.3** (2026-09-29; `LEGACY_DONE_SQL` + `is_source_done()` di `scheduler_service.py` — SQL utuh per sumber, bukan string yang dirakit dari nama tabel. Nilai status diverifikasi dari kode ETL: dmaa `'success'`, itm_* `'completed'`, dsr `daily_status='completed'`. A13: `IsSourceDoneTest` (4 sumber vs tabel lama nyata, semua di tx rollback; mapping == `FILE_TYPES` kedua service) + `LateCheckTest` (end-to-end, `current_processing_date` di-patch). Resolve lewat retry sukses sudah diuji di T5.2.) `_run_late_check` (FR14/C3): selesai bila `import_jobs` `completed` **atau** tabel lama sukses. Pemetaan satu dict per `file_type` (P1/P2):
  `dmaa → dmaa_files status='success' file_date` · `itm_cashpos → itm_cashpos_files status='completed' file_date` · `itm_replenish → itm_replenish_files status='completed' file_date` · `dsr → dsr_uploads daily_status='completed' report_date`. Nama tabel/kolom dari dict konstanta, bukan input user (tidak ada SQL injection). Hapus catatan `ponytail:` L161.
  - Validate: A13 — per sumber: sukses lama → tidak late; kosong keduanya → late; retry sukses → `late_detections` resolved.
- [x] **T5.4** (2026-09-29; `resolve_actor_id()` — hanya `users.id` desimal yang ada; `process_manual_retry(file_id, user_id, ip=None)` meneruskan actor+ip ke `start`/`complete`/`fail`; 3 test baru hijau. **Lihat R5**: dengan `require_auth` sekarang, token Go selalu memberi `"unknown"`, jadi di praktik belum ada `audit_logs` sampai R5 diputuskan.) `audit_service.py` + actor (P4): `file_id: int | None`; retry manual dengan `sub` numerik yang ada di `users` → juga `audit_logs` via helper.
- [x] **T5.5** (2026-09-29; `status`/`retry`/`summary`/`audit` ditulis di `eod_retry_scheduler` lalu disalin identik ke `service_dsr_etl` (sebelumnya identik; `late.py` tidak berubah — tidak menyentuh `import_jobs`). `/status` & `/summary` hanya menampilkan `FILE_TYPES` milik service (tabel kini bersama). `/retry` meneruskan IP klien. `lib/schemas.py` disinkronkan (`superseded`, `file_id: str`). A15 di `lib/test_eod_api.py`: 6 test × 2 service lewat driver ASGI stdlib — httpx tidak terpasang. Total suite Python 45 hijau, 0 baris sisa.) Router kedua service (`status`, `retry`, `audit`, `late`, `summary`) + `lib/schemas.py`: `file_id` path param `int` (FastAPI menolak non-angka → 422), respons `str(id)`; query `retry_file_tracking` → `import_jobs` (`processing_status` di respons = kolom `status`, nama field JSON tidak berubah). Summary menghitung `superseded` terpisah, tidak masuk `failed`.
  - Validate: A15 — test router (httpx `AsyncClient`) untuk kedua service; `file_id` string angka.
- [x] **T5.6** (2026-09-29, dikerjakan bersama T5.2 karena `mark_stale` di siklus retry membutuhkannya. Disederhanakan: **satu** `stale_after_minutes` per service (default 60, `ge=1`; env `RETRY_STALE_AFTER_MINUTES` / `DSR_ETL_STALE_AFTER_MINUTES`, `.env.example` diperbarui), bukan map per `file_type` — semua sumber kedua service adalah ETL batch dengan batas sama. Tambah map bila suatu saat ada sumber dengan batas berbeda.) Config per service: override stale per `file_type` (C1) di `eod_retry_scheduler/config.py` + `service_dsr_etl/config.py`.

### Fase 6 — Frontend (`features/eod-monitoring`)
- [x] **T6.1** `types.ts`: tambah `superseded` ke `ProcessingStatus`, `STATUS_*` label ("Digantikan") + ikon (bukan warna saja, Sec 13). `RetryDrawer`: `superseded` tidak bisa di-retry. _Selesai: `Layers` icon + "Digantikan"; drawer memakai allowlist (failed/exhausted/processing) sehingga superseded tanpa tombol retry; `RetryDrawer.test.tsx` 5 test; build hijau; `SummaryCounts.superseded` ditambah, kartu ringkasan belum (opsional). Suite penuh: 1 gagal `lib/auth/store.test.ts` (redirect param, tidak terkait, tidak disentuh)._
  - Validate: `npm test` (component test badge `superseded` + tombol retry tersembunyi), `npm run typecheck`, `npm run build` hijau.

### Fase 7 — Dokumentasi
- [x] **T7.1** CLAUDE.md Sec 3: Core tables: `import_jobs` (kolom ringkas) + `late_detections`/`scan_runs`/`retry_audit_logs`; catat `retry_file_tracking` digantikan; helper saat ini hanya Python (`lib/import_jobs.py`), `internal/importjob` menyusul di 2.1.
- [x] **T7.2** CLAUDE.md Sec 12: keputusan S7 (satukan), S4 (deviasi audit sistem), C3 (cek SLA baca tabel lama), R4 (helper Go ditunda).
- [x] **T7.3** `.claude/development-progress.md`, `.claude/sdlc/README.md`, `.kiro/steering/development-plan.md` (0.2 → done); `graphify update .`.
- [x] **T7.4** Syarat deploy R2: VM/container yang menjalankan ETL Python + retry scheduler memakai TZ `Asia/Jakarta` — catat di `.claude/docs/deployment.md` dan `backend_python/*/.env.example` (komentar `TZ=Asia/Jakarta`).

### Fase 8 — Verifikasi (→ `tests.md`)
- [ ] **T8.1** `go build ./...` + `go test ./...` (backend, memastikan `models.go` baru aman), `pytest backend_python`, `npm test && npm run typecheck && npm run build` (CompanyPortal) — semua hijau.
- [ ] **T8.2** Review: `code-reviewer` + `database-reviewer` (migrasi/index) + `python-reviewer`; perbaiki CRITICAL/HIGH.
- [ ] **T8.3** A17 manual browser check halaman EOD monitoring — **outstanding, dikerjakan user**, setelah task routing `/api/eod` (S6) selesai.

## Urutan & ketergantungan
`T1.1 → T1.2 (izin) → T1.3 → Fase 3 → Fase 5 → Fase 6 → Fase 7 → Fase 8` (Fase 2 & 4 ditunda, R4).
Fase 5 butuh Fase 3. Fase 6 tidak bergantung pada backend (hanya tipe) dan bisa paralel.

## Risiko
| Risiko | Mitigasi |
|---|---|
| Helper Go di 2.1 menyimpang dari perilaku Python | Kontrak FR11 dibangun bersama helper Go di 2.1; test Python T3.2 (A1–A9) jadi sumber skenario kontrak. |
| TZ host ≠ `Asia/Jakarta` di suatu environment | DMAA late check false-positive; mitigasi = syarat deploy T7.4 (R2a), cek saat setup VM. |
| Partial unique index + `ON CONFLICT` di asyncpg butuh predicate yang sama persis | Helper memakai tangkap `UniqueViolationError` + re-read, bukan `ON CONFLICT … WHERE`, agar tidak bergantung pada pencocokan predicate. |
| Retry executor menjalankan ETL per `file_type`, bukan per file: satu run sukses tidak berarti file baris itu yang diproses | Perilaku lama, tidak diubah; dicatat. Status baris mengikuti hasil run seperti sekarang. |
| Stale 60 menit untuk ETL batch hanya usulan | Nilai via config, ukur durasi ETL di dev sebelum prod (flag spec). |
| `lib/services` belum punya test sama sekali | Validasi A11–A13 butuh harness pytest baru (DB nyata, `retry_executor` di-mock) — effort Fase 5 lebih besar dari sekadar menyesuaikan test. |
| Halaman EOD tidak bisa dicek manual sebelum S6 | A17 tetap outstanding; test otomatis router + komponen tetap wajib hijau. |
| Mengubah tipe `file_id` UUID → int di Python | Respons tetap string; FastAPI validasi path int (422 untuk UUID lama) — tidak ada data lama karena tabel tak pernah ada. |

## Bukti selesai
Semua task non-ditunda `[x]`; output test T8.1 dicatat di `tests.md` dengan trace A1–A17 (kecuali A10, ditunda ke 2.1) → test; migrasi `019` tercatat applied di CLAUDE.md; A17 tertulis **outstanding (user)**.
