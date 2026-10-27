# Spec: Laporan DSR telat / tidak kirim — dasar penalti FLM (Phase 2.3)

Author: Claude, dari `intent.md` (accepted 2026-10-27, keputusan #1–#19). Status: **accepted 2026-10-27** (PO). Jawaban PO atas Q1–Q5 dan persetujuan K1–K5 (2026-10-27) sudah dimasukkan sebagai keputusan S1–S5 dan K1–K5.
Stage: 2 Design. Stage berikut: `plan.md`.

## ⚠ Flagged (Sec 4 #7)
- **Skema baru**: migrasi `033_dsr_receipts.sql`, berisi tabel append-only `dsr_receipts` (S1). File migrasi **belum dibuat**. Dibuat di stage Build dan diterapkan ke dev lewat `go run ./cmd/migrate up` dengan OK user.
- **Ingest DSR disentuh**: `backend_python/dsr/dsr_etl.py` (`--mode commit`) menulis satu baris `dsr_receipts` dalam tx yang sama dengan `dsr_uploads`. Form dan alur upload vendor tidak berubah (#9), tapi jalur commit ETL berubah.
- **Python menulis notifikasi** (pola K3 2.2c): menulis langsung ke `notifications` + `notification_emails`; email dikirim worker outbox Go.
- **Data dasar penalti FLM** (bukan uang, tapi dipakai untuk penalti): status harus reproducible dan bisa dijelaskan (Sec 6).

## Temuan verifikasi kode (2026-10-27) — alasan keputusan #10 diganti (S1)
Keputusan #10 (waktu diterima pertama dibaca dari `import_jobs`, `source = dsr`) tidak bisa dijalankan dengan kode sekarang:
1. Alur upload vendor (Go `ConfirmUpload` → `service_dsr_etl` `POST /process/dsr/commit` → `run_manual_process` → subprocess `dsr_etl.py --mode commit`) **tidak memanggil** `lib/import_jobs.register/complete`. Baris `import_jobs` untuk `dsr` hanya muncul dari detector scheduler (`detect()`, status awal `failed`). Dev 2026-10-27: `import_jobs` **0 baris**.
2. `import_jobs` tidak punya kolom vendor. Indeks `import_jobs_current_uq (source, processing_date)` dan `complete()` yang men-*supersede* semua baris lain di tanggal yang sama berarti upload vendor B akan men-supersede vendor A.
3. Upload DSR tidak menulis `audit_logs`. Saat revisi, baris `dsr_uploads` di-*replace* (`DELETE` + `INSERT`). Akibatnya waktu upload sukses pertama tidak tersimpan di mana pun.

PO memilih tabel baru `dsr_receipts` (S1). `import_jobs` tetap tidak diubah.

## Konsep & status
Satu baris laporan = **(tanggal laporan X, cabang vendor B)** untuk setiap cabang yang wajib kirim pada X (FR2).
```
vault terhitung (counted) cabang B pada X = vault aktif B, created_at (WIB) ≤ X, punya ≥1 map lokasi DSR aktif (FR2.2, S5)
first_received_at = MIN(dsr_receipts.received_at) dari receipt vendor B untuk report_date = X
    yang punya ≥1 label lokasi ter-map ke vault terhitung cabang B
deadline(X) = X 09:00:00 Asia/Jakarta (#1, tiap hari termasuk libur, #12), inklusif (K1)

first_received_at ≤ deadline           → OK
first_received_at > deadline           → TELAT  (+ menit keterlambatan)
tidak ada receipt, sekarang ≤ deadline → BELUM JATUH TEMPO (hanya tanggal hari ini, K4)
tidak ada receipt, sekarang > deadline → TIDAK KIRIM
```
Status dihitung saat dibaca (#7, laporan hidup, tanpa kunci). DSR yang masuk setelah deadline mengubah TIDAK KIRIM menjadi TELAT. Revisi setelah receipt pertama tidak mengubah apa pun (#2).

## Functional requirements

### FR1 — Receipt DSR (#2, #4, #8, #9, S1, K2)
1.1 Setiap `dsr_etl.py --mode commit` yang menulis `dsr_uploads` dengan `daily_status = 'completed'` juga menulis satu baris `dsr_receipts` **dalam tx yang sama**. Isinya: vendor, `report_date`, `checksum`, `received_at = now()` (saat konfirmasi/commit, K2), `uploaded_by_user_id`, dan `locations` = daftar label `dsr_daily_rows.location` dari upload itu (distinct, `btrim`, non-kosong).
1.2 Receipt tidak ditulis bila: `daily_status = 'failed'` (#4), checksum sama dengan upload yang ada (`skipped`), mode dry-run, atau commit gagal/rollback.
1.3 Receipt tidak pernah diubah atau dihapus. Revisi menambah baris baru, tidak menimpa (#2). `dsr_uploads` tetap di-replace seperti sekarang.
1.4 Vendor receipt di-resolve saat commit **lewat `vendors.code` yang unik dari nama file** (bukan `vendors.name`, yang tidak unik; review R1, 2026-10-27): `vendor_id` diteruskan dari `resolve_vendor` ke `_insert_receipt`, `vendor_label` tetap berisi name. Kode vendor tak dikenal menggagalkan commit (Go sudah menolaknya di upload), jadi `vendor_id` selalu terisi; baris lama bisa NULL dan tetap muncul sebagai peringatan (FR3.4c).
1.5 Laporan hanya menghitung `report_date ≥ DSR_LATE_START_DATE` (#16). Receipt sebelum tanggal itu boleh tersimpan tapi diabaikan. Tidak ada backfill dari `dsr_uploads` lama.

### FR2 — Cabang wajib kirim (#3, #8, #15, S2, S3)
2.1 Kandidat (S2) = `vendor_branches` milik vendor dengan `vendors.kind = 'FLM_VENDOR'` yang punya **≥1 `vendor_vaults` aktif** (`is_active`). Cabang vendor `INTERNAL`/ROH dikecualikan (#15).
2.2 Vault aktif yang **belum punya map lokasi DSR aktif** (`dsr_location_vault_maps.is_active`) dikeluarkan dari hitungan dan ditampilkan sebagai peringatan, bukan TIDAK KIRIM (S2). Cabang yang **semua** vault aktifnya belum ter-map juga dikeluarkan dari laporan dan masuk peringatan (FR3.4). Cabang wajib kirim = kandidat yang punya ≥1 vault terhitung.
2.3 Cabang aktif pada X (#15, S3), dihitung dengan perkiraan berikut (WIB): `created_at::date ≤ X` **dan** (`is_active` **atau** `deleted_at::date > X`). Celah nonaktif yang kemudian diaktifkan lagi tidak terlihat (risiko diterima, R7).
2.4 Vault hanya dihitung untuk X bila `vendor_vaults.created_at::date` (WIB) ≤ X (S5). Status aktif vault (`is_active`) dan map lokasi tetap dibaca **saat laporan dibuka**, karena tidak ada riwayat. Risikonya diterima (R2).
2.5 Hanya tanggal X dari `DSR_LATE_START_DATE` s.d. hari ini (WIB). Tanggal ke depan tidak ditampilkan.

### FR3 — Pemetaan cabang dan hitung status (#8)
3.1 Label receipt dipetakan ke cabang lewat `lower(label) = lower(dsr_location_vault_maps.dsr_location)`, dengan `vendor_id` yang sama, map `is_active`, dan vault terhitung (FR2.2), lalu ke `vendor_vaults.vendor_branch_id`.
3.2 Cabang B dianggap kirim untuk X bila ada receipt vendor B dengan `report_date = X` yang ≥1 labelnya ter-map ke vault terhitung cabang B. `first_received_at` = receipt paling awal yang memenuhi syarat itu. Label yang tidak ter-map tidak dihitung ke cabang mana pun (#8).
3.3 Kolom penjelas per baris (reproducible): `first_received_at` (WIB), `deadline`, `late_minutes` (hanya untuk TELAT), `receipt_checksum`, dan label lokasi yang membuat cabang dianggap kirim.
3.4 Peringatan untuk bulan yang dipilih (bukan baris laporan): (a) vault aktif tanpa map lokasi DSR, per cabang, ditandai bila **semua** vault cabang itu belum ter-map sehingga cabang dikeluarkan; (b) label lokasi di receipt yang tidak ter-map; (c) receipt yang vendornya tidak ter-resolve (FR1.4). Tujuannya agar ADMIN melengkapi `dsr_location_vault_maps` lewat master-data maker-checker yang sudah ada.

### FR4 — Laporan bulanan (#6, #17, #18, #19, S4)
4.1 Satu bulan per tampilan (`month=YYYY-MM`, wajib, #18). Bulan sebelum `DSR_LATE_START_DATE` menghasilkan data kosong.
4.2 **Detail harian**: satu baris per (tanggal, cabang) berisi tanggal laporan, vendor, kode + nama cabang, region cabang, waktu diterima pertama (WIB), status, dan menit telat. Filter: vendor, cabang, status. Paging server-side, urut tanggal, lalu vendor, lalu cabang.
4.3 **Rekap per cabang** (dasar penalti, tanpa nominal): per cabang di bulan itu berisi jumlah hari wajib, OK, TELAT, TIDAK KIRIM, dan total menit telat. Filter: vendor, cabang.
4.4 Ekspor CSV (stdlib `encoding/csv`, #6) untuk detail dan rekap, dengan filter yang sama dengan layar, dibaca dari replica. `audit_logs` `report_exported` (entity `dsr_late_report`, `after` = jenis laporan + filter) ditulis sebelum CSV dikirim (pola `ReplenishmentReadService.RecordExport`). Bila audit gagal: 500 dan CSV tidak dikirim.
4.5 Internal saja (#17): tidak ada endpoint/route di VendorPortal.
4.6 RBAC (#19): `ATM-USER`, `ATM-SPV`, `BRANCH-ATM-SPV`, `ADMIN`, dicek di route (`RequireRoles`) **dan** di service. Role lain mendapat 403. Keempat role melihat semua vendor dan cabang; `BRANCH-ATM-SPV` tidak dibatasi (S4).

### FR5 — Notifikasi 09:00 WIB (#5, #13, #14, #16, K3)
5.1 Runtime **Python** `eod_retry_scheduler`: modul baru `backend_python/dsr_late_notify/` (job + `run.py` untuk manual `python -m dsr_late_notify.run [--date YYYY-MM-DD]`), cron **09:00 WIB** setiap hari, termasuk libur (#12). Job hanya didaftarkan bila `RETRY_DSR_LATE_START_DATE` terisi. Bila kosong, job tidak jalan (#16).
5.2 Untuk X = hari ini (WIB), job mengambil cabang wajib kirim (FR2) yang **belum punya receipt** (FR3.2) saat job jalan. Cabang yang dikeluarkan karena belum ter-map (FR2.2) tidak diberi notifikasi.
5.3 Penerima vendor per cabang (#13, K3):
    - User vendor aktif (`VENDOR-USER`, `VENDOR-ATM-USER`, `VENDOR-VAULT-USER`) milik vendor itu dengan `vendor_branch_id` = cabang itu atau NULL (vendor-wide): in-app + email.
    - PIC `vendor_pics` aktif dengan `is_notification_recipient`, `roles` memuat `replenish` atau `vault`, dan `vendor_branch_id` = cabang itu atau NULL: email saja (PIC bukan user).
    - Aturannya sama dengan `ListActiveUserRecipientsByVendorBranches` / `ListVendorBranchNotificationPicEmails` (029/032).
    - Penerima yang terkait beberapa cabang mendapat **satu** notifikasi berisi daftar cabangnya.
5.4 Penerima internal (#19): semua user aktif dengan role `ATM-USER` mendapat satu notifikasi ringkasan (jumlah cabang + vendor yang belum kirim, link ke layar FR6).
5.5 `type = 'dsr_late'`. Title dan body mengikuti batas `notifications` (title ≤120, body ≤1000; daftar dipotong lalu diberi "dan N lainnya"). Email: baris `notification_emails` berstatus `pending` bila SMTP aktif, `skipped` bila tidak (sama dengan `replenish_classify._notify_late`).
5.6 **Idempoten per tanggal**: sebelum kirim, job cek `retry_audit_logs` (`event_type = 'dsr_late_notify'`, `processing_date = X`, `outcome = 'success'`). Bila sudah ada, job tidak mengirim lagi. Notifikasi dan baris audit sukses ditulis dalam **satu tx** di bawah `pg_advisory_xact_lock` per job, sehingga cron dan run manual yang bersamaan tidak menghasilkan kiriman ganda. Bila tidak ada cabang yang belum kirim, baris sukses tetap ditulis (0 penerima).
5.7 Notifikasi hanya sekali. Tidak ada notifikasi kedua saat DSR akhirnya masuk (#14). Run manual untuk tanggal lampau hanya boleh bila belum ada baris sukses untuk tanggal itu dan tanggal itu ≥ start date.
5.8 Aksi job dicatat di `retry_audit_logs` (`file_type = 'dsr_late_notify'`, `trigger_type` `auto|manual`, outcome, durasi, error) + structured log. Job **tidak** menulis `audit_logs` (pola S4/FR3.10 2.2c). Bila gagal: baris `failed`, notifikasi di-rollback, run manual berikutnya boleh mencoba lagi.
5.9 `late_detections` tidak disentuh (#11).

### FR6 — Layar CompanyPortal (#6, #18, Sec 13)
6.1 Menu baru di grup **Monitoring** pada `lib/config/navigation.ts`: **"Laporan DSR Telat"**, route `/dsr-late-report`, roles `ATM-USER`, `ATM-SPV`, `BRANCH-ATM-SPV`, `ADMIN`. Akses sidebar diatur oleh `roles` di `NAV_CONFIG`, jadi tidak butuh baris `menu_features` baru.
6.2 Filter: bulan (wajib, default bulan berjalan), vendor, cabang (bergantung pada vendor), status. Dua tab: **Rekap per cabang** (FR4.3) dan **Detail harian** (FR4.2). Tombol **Ekspor CSV** di tiap tab.
6.3 Kolom detail: Tanggal laporan · Vendor · Cabang (kode – nama) · Region · Diterima pertama (WIB, `dd/MM/yyyy HH:mm:ss`) · Status · Telat (menit, `tabular-nums`, rata kanan).
6.4 Status **selalu berupa label teks + ikon**; warna hanya pelengkap (Sec 13): `OK` (ikon centang), `TELAT` (ikon jam), `TIDAK KIRIM` (ikon silang), `BELUM JATUH TEMPO` (ikon jam pasir, netral). Tooltip/aria-label menjelaskan deadline.
6.5 Panel peringatan FR3.4 tampil bila ada: vault/cabang belum ter-map (dengan penanda "cabang dikeluarkan" bila semua vault-nya belum ter-map), label tak ter-map, dan vendor tak ter-resolve. Untuk ADMIN ada link ke master data DSR location map.
6.6 Keadaan loading, error, dan kosong ditangani eksplisit. Keadaan kosong sebelum start date menampilkan tanggal mulai.

### FR7 — Audit
Ekspor: FR4.4. Job notifikasi: FR5.8. Receipt ditulis oleh ETL sistem (tidak ada aksi user baru). Upload DSR itu sendiri tidak berubah.

## Non-functional
- Laporan + ekspor dibaca dari **replica** (Sec 6). Receipt ditulis ke primary oleh ETL. Job notifikasi membaca dan menulis primary (data baru saja di-commit, jadi replica lag dihindari).
- Waktu disimpan `timestamptz` UTC; deadline dan tampilan memakai Asia/Jakarta. Host Python `TZ=Asia/Jakarta` (prasyarat Sec 14 yang sudah ada); perhitungan tetap memakai zona eksplisit.
- Skala: kira-kira cabang wajib × 31 baris per bulan (dev: 1.978 cabang aktif, mayoritas vendor uji; yang wajib hanya yang punya vault ter-map). Karena itu detail memakai paging server-side dan rekap diagregasi di SQL. Laporan ≤ 3 s p95.
- Tidak ada dependency baru (Go stdlib CSV; Python asyncpg/APScheduler yang sudah ada).
- Logika "sudah kirim" ada di dua tempat (laporan Go dan job Python). SQL keduanya dibuat sama dan diuji dengan kasus yang sama (R3).

## API (flat JSON, pola handler `backend/`, mount baru `/api/v1/dsr-late-reports`)
| Method | Path | Role | Catatan |
|---|---|---|---|
| GET | `/api/v1/dsr-late-reports/daily?month=&vendor_id=&vendor_branch_id=&status=&page=&page_size=` | FR4.6 | FR4.2; `status` ∈ `ok,late,not_sent,pending` |
| GET | `/api/v1/dsr-late-reports/daily?…&format=csv` | FR4.6 | FR4.4, tanpa paging |
| GET | `/api/v1/dsr-late-reports/summary?month=&vendor_id=&vendor_branch_id=` | FR4.6 | FR4.3 |
| GET | `/api/v1/dsr-late-reports/summary?…&format=csv` | FR4.6 | FR4.4 |
| GET | `/api/v1/dsr-late-reports/warnings?month=&vendor_id=` | FR4.6 | FR3.4: `unmapped_vaults[]` (cabang, vault, `branch_excluded`), `unmapped_labels[]`, `unresolved_vendors[]` |

- `month` hilang atau formatnya salah → 400. Bulan setelah bulan berjalan → 400.
- Respons detail: `{ "items": [...], "total": n, "page": p, "page_size": s, "start_date": "YYYY-MM-DD" }`. Status di JSON memakai kode (`ok|late|not_sent|pending`); label Indonesia ada di frontend.
- Tanpa filter cakupan per user: semua role FR4.6 menerima data yang sama (S4).
- Layering: `queries/dsr_late_report.sql` → repo read-only (dbRead) → `service` (authorize + validasi) → `handler`. Didaftarkan di `cmd/api/main.go` dengan `RequireAuth` + `RequireRoles`.
- **OpenAPI**: ya, masuk `backend/api/openapi.yaml`. Ini modul baru, dan DoD Sec 11 berbunyi "a new module starts with its spec", jadi butuh contract test `-run OpenAPI` + `gen:api` CompanyPortal (`schema.d.ts`). (`/api/v1/replenishment` 2.2c belum ada di spec dan tidak diubah di fitur ini.)

### Bentuk query (ditulis rinci di `plan.md`)
```
counted_vaults(X) = vendor_vaults v JOIN vendor_branches b JOIN vendors vd (kind='FLM_VENDOR')
                  WHERE v.is_active AND v.created_at::date(WIB) ≤ X                                   -- S5
                    AND EXISTS map aktif (vendor_id = b.vendor_id, vendor_vault_id = v.id)
obliged(X, B)   = B punya ≥1 counted_vault(X), X ∈ [max(start, bulan awal), min(hari ini, bulan akhir)],
                  b.created_at::date(WIB) ≤ X AND (b.is_active OR b.deleted_at::date(WIB) > X)     -- S3
hits(X, B)      = dsr_receipts r, unnest(r.locations) l
                  JOIN map aktif m ON m.vendor_id = r.vendor_id AND lower(m.dsr_location) = lower(l)
                  JOIN counted_vaults(X) v ON v.id = m.vendor_vault_id AND v.vendor_branch_id = B
                  WHERE r.report_date = X
first_received_at(X, B) = MIN(hits.received_at)
```
Hari dibangkitkan dengan `generate_series` untuk bulan itu, lalu LEFT JOIN ke `first_received_at`. Rekap = agregasi atas hasil yang sama. Job Python memakai `obliged` + `hits` yang sama untuk X = hari ini.

## Data model (migrasi `033_dsr_receipts.sql` — S1, file belum dibuat)
- **`dsr_receipts`** (append-only, tanpa update/delete):
  `id bigint identity PK`, `vendor_label text NOT NULL` (= `dsr_uploads.vendor`), `vendor_id bigint NULL → vendors(id)`, `report_date date NOT NULL`, `checksum text NOT NULL` (sha256 hex, CHECK seperti `import_jobs_hash_chk`), `locations text[] NOT NULL DEFAULT '{}'`, `uploaded_by_user_id bigint NULL → users(id)`, `received_at timestamptz NOT NULL DEFAULT now()`.
  UNIQUE `(vendor_label, report_date, checksum)` (commit ulang file yang sama tidak menghasilkan baris ganda). Indeks `(report_date, vendor_id)`.
  Tanpa FK ke `dsr_uploads`, karena baris itu dihapus saat revisi.
- Dipakai tanpa perubahan: `dsr_location_vault_maps` (027, indeks `(vendor_id, lower(dsr_location)) WHERE is_active` + `(vendor_vault_id)`), `vendor_vaults` (`vendor_branch_id`, `is_active`), `vendor_branches` (`is_active`, `deleted_at`, `created_at`), `vendors.kind`, `users.vendor_branch_id` (032), `vendor_pics.roles` (032), `notifications` + `notification_emails` (025), `retry_audit_logs` (019; `file_type varchar(20)` cukup untuk `dsr_late_notify`), `audit_logs`.
- **Tidak diubah**: `dsr_uploads`, `dsr_daily_rows`, `import_jobs`, `late_detections`, `holidays`.
- Env baru (K5): `RETRY_DSR_LATE_START_DATE` (Python `eod_retry_scheduler`) dan `DSR_LATE_START_DATE` (Go `backend`); **nilainya harus sama**. `.env.example` keduanya diperbarui.
- Setelah migrasi diterapkan: CLAUDE.md Sec 3 + `docs/data-map.md` diperbarui.

## Keputusan
### Jawaban PO atas open question (2026-10-27)
- **S1 (Q1)** Waktu diterima pertama berasal dari tabel append-only baru `dsr_receipts` (migrasi 033) yang ditulis oleh commit `dsr_etl.py`. Menggantikan sumber `import_jobs` di keputusan intent #10; `import_jobs` tidak diubah.
- **S2 (Q2)** Cabang wajib kirim = cabang aktif milik vendor FLM yang punya ≥1 `vendor_vaults` aktif. Vault tanpa map lokasi DSR dikeluarkan dari hitungan dan masuk peringatan, bukan TIDAK KIRIM. Cabang yang semua vault-nya belum ter-map juga dikeluarkan dan masuk peringatan.
- **S3 (Q3)** Aktif pada X = `created_at ≤ X` dan (`is_active` atau `deleted_at > X`). Celah aktif-ulang adalah risiko yang diterima (R7).
- **S4 (Q4)** `BRANCH-ATM-SPV` boleh melihat semua vendor dan cabang.
- **S5 (Q5)** Vault dihitung untuk X hanya bila `vendor_vaults.created_at` (WIB) ≤ X. Status aktif vault dan map lokasi tetap dibaca saat laporan dibuka; risikonya diterima seperti S3 (R2).

### Keputusan spec (disetujui PO 2026-10-27)
- **K1** Batas inklusif: diterima tepat 09:00:00 WIB = OK; 09:00:01 = TELAT.
- **K2** "Waktu diterima" = saat vendor **konfirmasi** (commit ETL berhasil), bukan saat file di-stage untuk dry-run.
- **K3** Penerima vendor mengikuti pola 029/032: user + PIC cabang itu **atau vendor-wide**; PIC hanya bila `is_notification_recipient`.
- **K4** Status tambahan `BELUM JATUH TEMPO` untuk tanggal hari ini sebelum 09:00.
- **K5** Dua env start date (Go + Python) dengan nilai sama, bukan tabel konfigurasi (R4).

## Open questions
Tidak ada.

## Risiko
- **R1** Jalur commit `dsr_etl.py` berubah. Bila receipt gagal ditulis, commit DSR ikut gagal (satu tx). Diuji di `test_dsr_etl.py` + integrasi.
- **R2** (diterima, S5) Status aktif vault dan map dibaca saat laporan dibuka; hanya `vendor_vaults.created_at` yang membatasi per tanggal. Menambah atau mengubah map, atau menonaktifkan vault, bisa mengubah status dan daftar cabang wajib di hari-hari lalu. Contoh: menonaktifkan satu-satunya vault sebuah cabang hari ini mengeluarkan cabang itu dari bulan-bulan lalu. Ini sesuai #7 (laporan hidup), tapi angka penalti bisa bergeser setelah rekap diambil. Ekspor CSV yang diaudit menjadi jejaknya.
- **R3** Logika "sudah kirim" ada di Go dan Python. Perbedaan kecil berarti notifikasi tidak cocok dengan laporan. Mitigasi: SQL `obliged`/`hits` yang sama, satu set kasus uji untuk kedua sisi.
- **R4** Dua env start date bisa berbeda nilai, sehingga ada notifikasi untuk tanggal yang tidak ada di laporan (atau sebaliknya).
- **R5** Label lokasi DSR berupa teks bebas dari vendor. Salah ketik berarti label tidak ter-map; bila tidak ada label lain yang ter-map, cabang menjadi TIDAK KIRIM. Mitigasi: peringatan FR3.4(b).
- **R6** Cabang yang belum ter-map tidak masuk laporan maupun notifikasi (S2). Selama map belum lengkap, laporan terlihat lebih "bersih" daripada kenyataannya. Mitigasi: panel peringatan FR6.5 selalu terlihat. Saat go-live, map harus dilengkapi dulu (dev: 0 baris `dsr_location_vault_maps`).
- **R7** (diterima, S3) Cabang yang dinonaktifkan lalu diaktifkan lagi dihitung wajib di seluruh celah nonaktifnya, karena tidak ada riwayat. Cabang yang dinonaktifkan dengan `deleted_at` yang kemudian dikosongkan saat aktif ulang juga kehilangan riwayatnya.

## Data dev (dicek 2026-10-27)
- `dsr_uploads`: 1 baris (vendor `Bijak`, report_date 2026-07-15, completed). `import_jobs`: 0 baris. `dsr_location_vault_maps`: 0 baris, jadi saat ini tidak ada cabang yang wajib kirim.
- `vendor_branches` aktif: 1.978 (sebagian besar vendor uji `ITESTNUM-*`/`ITV2-*`; Prosegur 9).
- User aktif per role: ATM-USER 1, ATM-SPV 1, BRANCH-ATM-SPV 1, ADMIN 1, VENDOR-USER 9, VENDOR-ATM-USER 2, VENDOR-VAULT-USER 2.
- Uji manual butuh map lokasi DSR untuk minimal satu vendor dan workbook DSR uji (disiapkan di `plan.md`).

## Out of scope
Perhitungan nominal penalti FLM; perubahan form/alur upload DSR (#9); tampilan di VendorPortal (#17); ekspor XLSX/PDF (Phase 0.5); perubahan `late_detections` / EOD Monitoring (#11); kalender libur (#12); backfill sebelum start date (#16); notifikasi kedua saat DSR masuk (#14); keterlambatan sheet Rencana Isi (status hanya dari sheet Daily); approval/penguncian rekap (#7); riwayat aktif cabang/vault/map (S3, S5).
