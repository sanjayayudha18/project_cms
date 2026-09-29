# Spec: `import_jobs` — satu registry file ingest (menyatukan `retry_file_tracking`)

Status: draft (revisi 2, 2026-09-28; koreksi review 2026-09-29: FR8, FR10, FR12, NFR6, A11, flagged concerns) — **diterima PO 2026-09-29**. Stage: 2 Design (accepted). Reads: `intent.md` (Resolved decisions + Amendment).
Trigger ke stage berikut: product owner menerima spec ini → Claude Code plan mode (`plan.md`).
Sec 4 #7 flag: **migrasi** (tabel baru) + **modul baru** + perubahan perilaku retry scheduler — lihat "Flagged concerns".

> **Revisi 2:** revisi 1 merancang `import_jobs` terpisah dari retry scheduler. Ternyata
> `backend_python/lib/services/` sudah punya registry file generik `retry_file_tracking`
> (+ `late_detections`, `scan_runs`, `retry_audit_logs`) yang dipakai `eod_retry_scheduler` dan
> `service_dsr_etl` serta dibaca halaman `features/eod-monitoring` — tetapi skemanya hanya ada di
> `backend/migrations/archives/2026-09-18_pre-baseline/012_retry_scheduler.sql` dan **tidak pernah
> diterapkan** (tidak ada di baseline; kode Python juga tidak membuatnya). User memutuskan
> (2026-09-28): **satukan** — `import_jobs` menggantikan `retry_file_tracking`, dan monitoring memakai
> **API Python yang sudah ada**, tanpa endpoint Go baru.

## Ringkasan
- `import_jobs` = satu-satunya tabel tracking file: dipakai (a) helper idempotensi untuk ingest
  **baru** (Go + Python) dan (b) retry scheduler Python (deteksi file gagal, auto/manual retry, cek SLA).
- `retry_file_tracking` **tidak dibuat**. Tabel pendamping retry (`late_detections`, `scan_runs`,
  `retry_audit_logs`) ikut dimigrasi di `019` agar retry scheduler dan halaman EOD monitoring bisa jalan.
- ETL lama (`dmaa_etl.py`, `itm_*`, `dsr_etl.py`) dan tabel `*_files`/`dsr_uploads`/
  `master_data_import_batches` **tidak diubah** (intent #1 tetap untuk sisi ETL).
- Hash sama → skip. Hash beda untuk sumber + tanggal sama → versi baru, lama `superseded`.
  File gagal → dicoba ulang pada baris yang sama sampai `max_retries`.

## Requirements

### Functional — registry & idempotensi
- **FR1 — Register.** Pemanggil mendaftarkan file sebelum memproses: `source`, `processing_date`
  (opsional; NULL = sumber tidak per tanggal), `file_hash` (SHA-256 hex isi file),
  `original_filename`, `file_path` (opsional), `runtime` (`go`|`python`), `detection_source`
  (`upload` untuk ingest normal), `created_by` (user id; NULL bila sistem).
- **FR2 — Satu baris per hash.** Paling banyak satu baris **non-`superseded`** per `(source, file_hash)`.
  Register hash yang sudah punya baris `pending`/`processing`/`completed` → tidak membuat baris baru,
  kembalikan baris itu + `duplicate=true`; pemanggil berhenti (no-op).
- **FR3 — Hash yang pernah gagal.** Register hash yang barisnya `failed`/`max_retries_exhausted` →
  **memakai ulang baris itu** (status → `pending`, dihitung sebagai retry manual), bukan baris baru.
- **FR4 — Satu proses aktif per sumber+tanggal.** Paling banyak satu baris `pending`/`processing` per
  `(source, processing_date)`. Pelanggaran → `ErrJobInProgress` (Go) / `JobInProgressError` (Python) → HTTP 409.
- **FR4a — Tanpa tanggal.** Baris dengan `processing_date IS NULL` hanya tunduk pada FR2/FR3. FR4, FR5,
  FR6 (supersede), FR9 tidak berlaku (NULL selalu berbeda di unique index Postgres; tanpa kode khusus).
- **FR5 — Versi baru (revisi).** Hash berbeda untuk `(source, processing_date)` yang sudah punya baris
  `completed` → baris baru `version = previous.version + 1`, `supersedes_job_id = previous.id`.
  Baris lama tetap `completed` sampai yang baru selesai.
- **FR5a — Revert.** Hash yang barisnya sudah `superseded` diupload lagi → baris baru versi berikutnya
  (A → B → A = versi 3).
- **FR6 — Complete.** Sukses → `completed` (`row_count`, `error_count`, `finished_at`) dan, **dalam
  transaksi yang sama**, baris `completed` sebelumnya untuk `(source, processing_date)` → `superseded`.
  Tersedia varian yang menerima transaksi pemanggil (tulis data bisnis + complete atomik).
- **FR7 — Fail.** Gagal → `failed` + `error_message` (dipotong, tanpa secret/PII). Baris `completed`
  sebelumnya tidak di-supersede.
- **FR8 — Job macet.** Baris `pending`/`processing` dengan `updated_at` lebih tua dari batas stale
  **per sumber** (default `IMPORT_JOB_STALE_AFTER` = **15 menit** untuk ingest baru; sumber ETL batch
  `dmaa`/`itm_cashpos`/`itm_replenish`/`dsr` mendapat batas sendiri di config Python, usulan 60 menit —
  keputusan C1) → register/retry berikutnya untuk
  `(source, processing_date)` yang sama menandainya `failed` ("stale") lalu melanjutkan. Siklus
  auto-retry juga menjalankan penandaan stale di awal setiap siklus (baris `processing` yang macet
  tidak menunggu register baru).
- **FR9 — Current version.** Lookup baris `completed` terkini per `(source, processing_date)`.
- **FR10 — Status machine** (ditegakkan helper + CHECK):
  `pending → processing → completed → superseded` ·
  `pending|processing → failed` ·
  `failed → processing` (retry) · `failed → max_retries_exhausted` ·
  `max_retries_exhausted → processing` (hanya retry manual).
  Status awal: `pending` (register, FR1) atau `failed` (deteksi file gagal, FR12 — lewat `detect`, bukan `register`).
- **FR11 — Kontrak Go = Python. DITUNDA ke forecast 2.1 (R4, 2026-09-29).** Fitur ini hanya membangun
  helper Python. Helper Go + satu set test kontrak di kedua runtime dibangun di 2.1 (pemakai Go pertama),
  dengan perilaku helper Python sebagai acuan. Referensi ke Go (`ErrJobInProgress`, dll.) di spec ini
  berlaku untuk 2.1.

### Functional — adopsi retry scheduler (Python)
- **FR12 — Deteksi file gagal.** `FileDetector.persist_detected_files` menulis ke `import_jobs`
  (`source` = `file_type` existing: `dmaa`, `itm_cashpos`, `itm_replenish`, `dsr`; `runtime='python'`;
  `detection_source` = `not_processed`|`input_remaining`; status awal `failed`) lewat fungsi helper
  terpisah **`detect`** = insert bila belum ada; bentrok `import_jobs_hash_uq` (23505) → no-op.
  `detect` **tidak** memakai `register` dan **tidak** menerapkan FR3: baris `failed`/
  `max_retries_exhausted`/`completed` yang sudah ada tidak di-reset (kalau di-reset, setiap scan 15 menit
  akan menghidupkan lagi file gagal dan `max_retries` tak pernah berlaku). Perilaku scan, jadwal, dan
  lock tidak berubah.
- **FR13 — Auto/manual retry.** `SchedulerService` membaca/menulis `import_jobs` dengan transisi FR10;
  `auto_retry_count`/`max_retries`/`last_retry_at` pindah ke `import_jobs`. Retry manual tetap menolak
  baris `completed` (409) dan tetap melewati `max_retries`. Bila FR4 menolak (ada proses lain untuk
  sumber+tanggal yang sama), siklus auto-retry melewati baris itu dan mencoba lagi di siklus berikutnya.
- **FR14 — Cek SLA.** `_run_late_check` menganggap sumber+tanggal **selesai** bila salah satu benar
  (keputusan C3, hanya query baca, ETL lama tidak diubah):
  - `import_jobs` punya baris `completed` untuk `(source, processing_date)`, **atau**
  - tabel sumber lama punya file sukses untuk tanggal itu:
    `dmaa` → `dmaa_files.status='success' AND file_date = $date` ·
    `itm_cashpos` → `itm_cashpos_files.status='completed' AND file_date = $date` ·
    `itm_replenish` → `itm_replenish_files.status='completed' AND file_date = $date` ·
    `dsr` → `dsr_uploads.daily_status='completed' AND report_date = $date`.
  Pemetaan ini hidup di satu tempat (dict per `file_type` di config/helper), bukan tersebar.
  Menutup false-positive `late_detections` untuk ETL lama yang sukses tanpa pernah gagal
  (menggantikan catatan `ponytail:` di `scheduler_service._run_late_check`).
- **FR15 — Tabel pendamping.** `late_detections`, `scan_runs`, `retry_audit_logs` dibuat di `019` dengan
  skema dari `012_retry_scheduler.sql` (arsip), kecuali `retry_audit_logs.file_id` menjadi `bigint`
  FK → `import_jobs(id)`.
- **FR16 — API Python tetap.** Endpoint `/status`, `/status/{file_id}/history`, `/retry/{file_id}`,
  `/late`, `/summary`, `/audit` di **kedua** service (`eod_retry_scheduler`, `service_dsr_etl`) membaca
  `import_jobs`. `file_id` menjadi bigint di DB tetapi **diserialisasi sebagai string** di JSON, agar
  `features/eod-monitoring/types.ts` (`file_id: string`) tidak berubah. Path param menerima string angka.
- **FR17 — Monitoring.** Tidak ada endpoint Go baru. Halaman `features/eod-monitoring` menampilkan data
  `import_jobs` lewat API Python di atas (intent #5). Status ditampilkan dengan label/ikon, bukan warna saja.

### Functional — audit
- **FR18 — Audit.** Transisi dengan actor user (register oleh user, retry manual) → `audit_logs`
  (`entity_type='import_job'`, before/after status) dalam transaksi yang sama. Transisi sistem
  (deteksi, auto-retry, stale) → `retry_audit_logs` (sudah ada di desain retry scheduler) + structured log,
  karena `audit_logs.actor_id` NOT NULL. Deviasi dari Sec 5 disetujui (S4).

### Non-functional
- **NFR1** Konkurensi ditegakkan oleh partial unique index; `23505` dipetakan ke FR2/FR4, bukan 500.
- **NFR2** Tulis ke primary. API Python membaca pool yang sama seperti sekarang (belum ada replica di
  sisi Python — lihat Flagged concern #2).
- **NFR3** Tidak ada dependency baru (Go: pgx/sqlc; Python: `asyncpg` yang sudah dipakai `lib/`).
- **NFR4** Timestamps `timestamptz` UTC; tampilan Asia/Jakarta (Python sudah memakai `lib/utils/timezone.WIB`).
- **NFR5** Retensi permanen (keputusan #6).
- **NFR6** Coverage ≥ 80% helper Python (helper Go + kontrak: 2.1). `lib/services` belum punya test
  sama sekali — A11–A13 butuh harness pytest baru (DB nyata, `retry_executor` di-mock).

## Data model — migrasi `019_import_jobs.sql` (additive, forward-only)

```sql
CREATE TABLE public.import_jobs (
    id                bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    source            text        NOT NULL,           -- 'dmaa' | 'itm_cashpos' | 'itm_replenish' | 'dsr' | sumber baru
    processing_date   date        NULL,               -- NULL = sumber tidak per tanggal (FR4a)
    file_hash         text        NOT NULL,
    original_filename text        NOT NULL,
    file_path         text        NULL,
    runtime           text        NOT NULL,
    detection_source  text        NOT NULL DEFAULT 'upload',
    status            text        NOT NULL DEFAULT 'pending',
    version           integer     NOT NULL DEFAULT 1,
    supersedes_job_id bigint      NULL REFERENCES public.import_jobs(id),
    row_count         integer     NULL,
    error_count       integer     NULL,
    error_message     text        NULL,
    auto_retry_count  integer     NOT NULL DEFAULT 0,
    max_retries       integer     NOT NULL DEFAULT 3,
    last_retry_at     timestamptz NULL,
    created_by        bigint      NULL REFERENCES public.users(id),
    started_at        timestamptz NULL,
    finished_at       timestamptz NULL,
    created_at        timestamptz NOT NULL DEFAULT now(),
    updated_at        timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT import_jobs_source_chk    CHECK (source ~ '^[a-z][a-z0-9_]{1,62}$'),
    CONSTRAINT import_jobs_hash_chk      CHECK (file_hash ~ '^[0-9a-f]{64}$'),
    CONSTRAINT import_jobs_runtime_chk   CHECK (runtime IN ('go','python')),
    CONSTRAINT import_jobs_detection_chk CHECK (detection_source IN ('upload','not_processed','input_remaining')),
    CONSTRAINT import_jobs_status_chk    CHECK (status IN ('pending','processing','completed','failed','max_retries_exhausted','superseded')),
    CONSTRAINT import_jobs_version_chk   CHECK (version >= 1),
    CONSTRAINT import_jobs_retry_chk     CHECK (auto_retry_count >= 0 AND max_retries >= 0),
    CONSTRAINT import_jobs_counts_chk    CHECK (coalesce(row_count,0) >= 0 AND coalesce(error_count,0) >= 0)
);

-- FR2/FR3/FR5a: satu baris non-superseded per hash.
CREATE UNIQUE INDEX import_jobs_hash_uq ON public.import_jobs (source, file_hash)
    WHERE status <> 'superseded';
-- FR4: satu proses berjalan per sumber+tanggal.
CREATE UNIQUE INDEX import_jobs_inflight_uq ON public.import_jobs (source, processing_date)
    WHERE status IN ('pending','processing');
-- FR5/FR6/FR9: satu versi terkini per sumber+tanggal.
CREATE UNIQUE INDEX import_jobs_current_uq ON public.import_jobs (source, processing_date)
    WHERE status = 'completed';
-- Retry scheduler + monitoring.
CREATE INDEX import_jobs_retry_idx ON public.import_jobs (status, source) WHERE status = 'failed';
CREATE INDEX import_jobs_date_idx  ON public.import_jobs (processing_date, source);
CREATE INDEX import_jobs_list_idx  ON public.import_jobs (created_at DESC, id DESC);
```
+ trigger `updated_at` sesuai pola baseline.
+ `late_detections`, `scan_runs`, `retry_audit_logs` dari `012_retry_scheduler.sql` (arsip), dengan
`retry_audit_logs.file_id bigint NULL REFERENCES public.import_jobs(id)`. `retry_file_tracking` tidak dibuat.

## Komponen

| Komponen | Lokasi | Perubahan |
|---|---|---|
| Migrasi | `backend/migrations/019_import_jobs.sql` | Baru: `import_jobs` + 3 tabel pendamping |
| Model sqlc | `sqlc generate` (v1.31.1; hand-fix `UserLeafe`) → struct baru di `models.go`, tanpa queries | Regenerate |
| Helper Go | `backend/internal/importjob/` (S1) | **Ditunda ke 2.1** (R4) |
| Helper Python | `backend_python/lib/import_jobs.py` | Baru: `register`, `detect`, `start`, `complete`, `fail`, `current`, `mark_stale`; dipakai `lib/services` dan ingest Python baru |
| Deploy | `.claude/docs/deployment.md` | Syarat TZ `Asia/Jakarta` untuk host ETL Python (R2a) |
| Retry scheduler | `backend_python/lib/services/{detector,scheduler_service,audit_service}.py` | `retry_file_tracking` → `import_jobs` lewat helper |
| API Python | `backend_python/{eod_retry_scheduler,service_dsr_etl}/routers/*.py`, `lib/schemas.py` | Baca `import_jobs`; `file_id` bigint diserialisasi string |
| UI | `frontend/CompanyPortal-Vite/src/features/eod-monitoring/` | Tidak berubah kecuali label status `max_retries_exhausted`/`superseded` bila belum ada |
| Docs | `.claude/CLAUDE.md` Sec 3 + Sec 12 | Modul `internal/importjob`; deviasi audit S4; `retry_file_tracking` digantikan `import_jobs` |

## API surface
Tidak ada endpoint baru. Endpoint Python yang ada (FR16) mempertahankan path & bentuk respons;
perubahan yang terlihat klien hanya: nilai `file_id` berupa string angka (bukan UUID) dan status baru
`superseded`.

## Out of scope
- `export_jobs` (keputusan #4).
- Mengubah ETL lama (`dmaa_etl.py`, `itm_*`, `dsr_etl.py`) agar menulis `import_jobs` saat sukses —
  keputusan #1; gap SLA ditutup lewat pembacaan tabel lama (C3/FR14).
- Perbaikan routing `/api/eod/*` (task terpisah S6 — **prasyarat** agar halaman monitoring jalan).
- Replica pool di sisi Python, worker async, penyimpanan file (0.4), purge/retensi.

## Resolved spec decisions (tanya-jawab dengan user, 2026-09-28)
- **S1** Modul Go di `backend/internal/importjob`; ditambahkan ke CLAUDE.md Sec 3 saat dibangun (2.1, R4).
- **S2** `processing_date` nullable (FR4a).
- **S3** Revert = versi baru (FR5a).
- **S4** Audit: user → `audit_logs`; sistem → `retry_audit_logs` + structured log (FR18).
- **S5** `IMPORT_JOB_STALE_AFTER` = 15 menit.
- **S6** Routing `/api/eod/*` diselidiki di task terpisah — kini **prasyarat** (karena S8).
- **S7** Satukan `retry_file_tracking` ke `import_jobs` (revisi 2).
- **S8** Monitoring memakai API Python yang ada; tidak ada endpoint Go baru.

- **C1** Batas stale **per sumber**: 15 menit default (ingest baru), batas sendiri untuk sumber ETL batch
  (usulan 60 menit) karena retry executor menjalankan ETL per `file_type`, bukan per file (FR8).
- **C2** Replica untuk API Python **ditunda** — dicatat sebagai item di `development-plan.md` Phase 0.1;
  API Python tetap membaca primary di fitur ini (NFR2).
- **C3** Cek SLA juga membaca tabel sumber lama (`dmaa_files`, `itm_*_files`, `dsr_uploads`) — FR14.
- **C4** Kunci idempotensi `(source, file_hash)` disetujui — file identik di tanggal berbeda = duplikat (FR2).

## Flagged concerns (sisa)
- [x] **Verifikasi saat plan (FR14 tanggal):** ITM = tanggal di nama file = tanggal tiba (P1); DSR
  `report_date` = saldo 00:00 hari itu (`dsr-late-report/intent.md` #1) → cocok tanpa offset.
- [x] **R2 DMAA timezone (review 2026-09-29) — diputuskan (a):** `dmaa_etl.py:139` mengisi `file_date`
  dari `datetime.fromtimestamp(st_mtime).date()` = TZ **host**; `processing_date` = WIB. Host yang
  menjalankan ETL Python + retry scheduler **wajib** TZ `Asia/Jakarta` (syarat deploy). FR14 `dmaa` tetap
  tanpa offset. ETL tidak diubah (keputusan #1).
- [x] **R4 Helper Go (review 2026-09-29) — diputuskan: ditunda ke forecast 2.1.** FR11, A10, dan sisi Go
  A16 pindah ke spec 2.1.
- [ ] Nilai 60 menit untuk sumber ETL batch adalah usulan; ukur durasi ETL nyata di dev.

## Acceptance (tiap baris → test di `tests.md`)
| ID | Uji | Req |
|---|---|---|
| A1 | Register file baru → `pending`, versi 1 | FR1 |
| A2 | Register hash sama saat `completed` → `duplicate=true`, tidak ada baris baru | FR2 |
| A3 | Register hash yang `failed` → baris sama kembali `pending` | FR3 |
| A4 | Dua register paralel sumber+tanggal sama → satu sukses, satu `ErrJobInProgress`; tanpa 500 | FR4, NFR1 |
| A4a | `processing_date` NULL: hash beda → dua baris v1 independen | FR4a |
| A5 | Hash beda untuk tanggal yang `completed` → v2, `supersedes_job_id` terisi; v1 tetap `completed` | FR5 |
| A5a | Revert A → B → A → A jadi v3, B `superseded` | FR5a |
| A6 | Complete v2 → v1 `superseded`, atomik (rollback pemanggil membatalkan keduanya) | FR6 |
| A7 | Fail v2 → v1 tetap `completed` | FR7 |
| A8 | Baris `processing` lebih tua dari batas sumbernya → ditandai `failed` lalu lanjut; sumber ETL batch memakai batasnya sendiri, bukan 15 menit | FR8 |
| A9 | Transisi ilegal ditolak (mis. `completed → processing`) | FR10 |
| A10 | Suite kontrak hijau di Go & Python, baris identik — **ditunda ke 2.1** | FR11 |
| A11 | Scan detector dua kali atas file gagal yang sama → satu baris; baris `failed`/`max_retries_exhausted` yang sudah ada **tidak** berubah status/`auto_retry_count` | FR12 |
| A12 | Auto-retry: gagal ×3 → `max_retries_exhausted`; retry manual tetap jalan; baris `completed` → 409 | FR13 |
| A13 | Late check: tidak `late` bila `import_jobs` `completed` **atau** tabel lama sukses untuk tanggal itu (4 sumber, masing-masing diuji); `late` bila keduanya kosong; sukses retry me-resolve `late_detections` | FR14 |
| A14 | `019` diterapkan di DB kosong: 4 tabel + index + FK `retry_audit_logs.file_id` ada | FR15 |
| A15 | `/status`, `/status/{id}/history`, `/retry/{id}` di kedua service mengembalikan `file_id` string angka; test router Python hijau | FR16 |
| A16 | Transisi oleh user menulis `audit_logs` dalam transaksi yang sama; transisi sistem menulis `retry_audit_logs` | FR18 |
| A17 | Manual browser check halaman EOD monitoring (setelah S6 selesai) | — **outstanding, dikerjakan user** (Golden Rule #10) |
