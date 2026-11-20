# Steering: Database Architecture, Canonical Data Map, SQLC & Migrations

> **Scope**: PostgreSQL topology (`db` vs `dbRead`), monetary precision, sqlc v1.31.1, migration runner (`cmd/migrate`), dan Canonical DB Table Map (Migrations `001`–`045`).  
> **Source of Truth**: Disinkronkan dari `.claude/CLAUDE.md` (Sec 0a, 2a, 3, 6, 12) & [data-map.md](file:///c:/Users/RB%20Yudha%20Rangga/OneDrive/Documents/Development/CMS2/.claude/docs/data-map.md).  
> **Referenced by**: [GEMINI.md](file:///c:/Users/RB%20Yudha%20Rangga/OneDrive/Documents/Development/CMS2/.agents/rules/GEMINI.md)

---

## 1. Money, Data Integrity & DB Topology (Sec 6)

*   **Monetary Precision**: Nilai uang/kas wajib disimpan sebagai **`numeric`** atau integer minor units (**`bigint` IDR**). **DILARANG KERAS** menggunakan `float` atau `double` di SQL, Go, Python, maupun TypeScript.
*   **Currency**: Default `IDR`, selalu simpan kode mata uang secara eksplisit.
*   **Timestamps**: Gunakan `timestamptz`, simpan dalam UTC, tampilkan dalam zona waktu `Asia/Jakarta` (WIB).
*   **Read/Write Split (`db` vs `dbRead`)**:
    *   **Primary Pool (`db`)**: Semua operasi write/update + transactional reads (*read-after-write* dalam alur yang sama wajib menggunakan primary untuk menghindari replica lag).
    *   **Read Replica Pool (`dbRead`)**: Reporting, dashboard, cash monitoring, async exports, dan query read berat. Repository mengekspos `db` (primary) vs `dbRead` (replica). **Jangan pernah melakukan write ke replica.**
*   **Reconciliation Integrity**: Hasil rekonsiliasi harus reproducible & explainable (simpan raw inputs + deltas). Pisahkan tabel baris mentah (`atm_dsr_rows`, `escrow_batch_rows`) dari tabel ringkasan/summaries.
*   **Soft-Delete Policy**: Tabel `users`, `vendors`, `atms`, dan master data lainnya menggunakan **soft-delete** (`is_active=false` + `deleted_at`), tidak pernah `DELETE FROM` (ditegakkan oleh `internal/repository/no_hard_delete_test.go`). Pengecualian terdokumentasi ada di bagian *Master Data Map* di bawah.

---

## 2. SQLC Codegen & Migration Runner (Sec 0a, 2a, 12)

### 2.1 Development Stage & Schema Policy (Sec 0a)
*   **Fase Saat Ini**: Early development (greenfield) — **bukan lagi MVP** (target penuh FSD v1.0 §A).
*   **Schema is MUTABLE**: Perubahan skema database (tabel, kolom, migrasi baru) diperbolehkan seiring berkembangnya kebutuhan, namun keputusan struktural yang memengaruhi peta modul/tabel wajib dicatat di dokumentasi konteks setelah diterapkan.

### 2.2 SQLC v1.31.1 & Inflector Quirk
*   Versi terkunci: **`v1.31.1`** (`backend/sqlc.yaml`, query di `backend/queries/*.sql`).
*   **Quirk Wajib Diperbaiki**: `sqlc v1.31.1` meng-generate `UserLeafe` untuk tabel `user_leaves`. Setiap selesai menjalankan `cd backend && sqlc generate`, **wajib ubah manual `UserLeafe` → `UserLeave`** di `backend/internal/db/` sebelum build/test.

### 2.3 Migration Runner (`backend/cmd/migrate`)
*   **Perintah**:
    ```bash
    cd backend && go run ./cmd/migrate status   # cek applied/pending + verifikasi checksum SHA-256 (exit != 0 jika MISMATCH)
    cd backend && go run ./cmd/migrate up       # terapkan file pending via psql -f di bawah pg_advisory_lock
    ```
*   **Aturan Migrasi (BE-03)**:
    1. **Dilarang `psql -f` manual**: Menjalankan manual tidak akan tercatat di `schema_migrations`.
    2. **Immutability**: Jangan pernah mengedit file migrasi yang sudah applied (perubahan SHA-256 akan menghentikan seluruh eksekusi `migrate`). Buat file baru `NNN_*.sql`.
    3. **Mekanisme Internal**: Hanya membaca top-level `NNN_*.sql` di `backend/migrations/` (bukan `archives/`). Setiap file dijalankan utuh melalui `psql -v ON_ERROR_STOP=1 -X -q -f` di bawah `pg_advisory_lock(hashtext('cms.schema_migrations'))`. Kredensial dikirim via env `PG*` dari `pgx.ParseConfig`.
    4. **Host Dev**: `backend/.env` memakai `host.docker.internal` (hanya resolve di dalam container). Dari shell host lokal, gunakan `localhost:5432`.
    5. **Urutan Nomor Migrasi**: Saat ini terpakai hingga **`045_drop_old_vendor_pks_functions.sql`** (44 applied, 0 pending di dev per 2026-11-19). Nomor bebas berikutnya adalah **`046`**. Baseline awal: `001_baseline_schema.sql` + `002_baseline_seed.sql` (migrasi lama 001–040 pra-baseline diarsipkan di `backend/migrations/archives/2026-09-18_pre-baseline/`).

---

## 3. Canonical DB Logical Groups & Table Map (Sec 3)

> **Aturan**: Jangan pernah mengarang tabel atau kolom di luar daftar ini tanpa persetujuan. Detail skema per kolom tersedia di [.claude/docs/data-map.md](file:///c:/Users/RB%20Yudha%20Rangga/OneDrive/Documents/Development/CMS2/.claude/docs/data-map.md).

### 3.1 Auth & Role Management
*   **Auth Dasar**: `roles`, `users` (`auth_source` = `ldap` | `local` | `service`; password hash hanya untuk `local`).
*   **Role Management** (bagian dari `001_baseline_schema.sql` + `002_baseline_seed.sql`): `menu_features`, `role_permissions` (keberadaan baris = grant akses; role baru mulai dengan 0 akses).
*   **External User API / CIMB IDM (`039_ext_api_clients.sql`)**:
    *   Role `EXT-CLIENT` (tanpa `role_permissions`, tidak bisa login).
    *   `users.auth_source='service'` = akun layanan client (inactive, tidak bisa login, tidak tampil di admin user list).
    *   Tabel `api_clients`: `client_id` (`crown_` + 26 char base32), `secret_hash` (bcrypt), `credential_version`, `scopes` ⊆ `{users:read, users:write}`, `assignable_roles` (CHECK sesuai `pkg/userrules.PrivilegedRoles`), `allowed_cidrs`, `service_user_id` UNIQUE → akun layanan, `is_active`/`disabled_at`, tanpa DELETE.

### 3.2 Platform Core
*   **Audit & Maker-Checker**: `audit_logs`, `approval_requests`, `master_data_change_requests` (staged master-data changes), `master_data_import_batches` (impor CSV satu approval per batch, idempotent per SHA-256; migrasi `036` menambah kolom `file_name` text NULL CHECK 1–255).
*   **Dokumen & Kalender**: `documents`, `holidays` (migrasi `031`: hari kerja = Senin–Jumat dikurangi baris aktif, satu baris aktif per tanggal, ADMIN-only).
*   **Notifikasi (`025_notifications.sql`)**: `notifications` (satu baris in-app per user penerima) + `notification_emails` (outbox email per alamat, dikirim worker Go via `net/smtp`, retry 1/5/15 menit lalu `failed`; jika `SMTP_HOST` kosong → `skipped`). Retensi = hard delete setelah 90 hari.
*   **Import & EOD Tracking (`019`, `020`)**:
    *   `import_jobs`: satu baris per versi file, unik per hash, state machine `pending → processing → completed → superseded` / `failed → max_retries_exhausted` (**menggantikan `retry_file_tracking`**).
    *   `late_detections`, `scan_runs`, `retry_audit_logs` (`retry_audit_logs.file_id` → `import_jobs.id`).
*   **Async Export (`034_export_jobs.sql`, `035_export_jobs_reports.sql`, `037`)**:
    *   `export_jobs`: satu baris per request ekspor, dedupe per user + `params_hash`, status `queued → running → succeeded → downloading → downloaded / expired`, `report_params jsonb` (report kinds: `dsr_late_daily`, `dsr_late_summary`, `replenishment_amount_mismatch`, `replenishment_trips`, `vendor_pks`, `cash_count_pics`, serta 6 entitas master data).
    *   `export_job_chunks`: byte CSV per chunk, dihapus setelah diunduh, gagal, atau 10 menit setelah ready.
*   **Migrasi**: `schema_migrations` (`version` PK, `checksum` SHA-256, `applied_at`, `applied_by`, `baseline` bool; dibuat otomatis oleh `cmd/migrate`).

### 3.3 Master Data
*   **Entitas Utama**:
    *   `vendors` (termasuk `legal_name`, `npwp`, `kind` = `FLM_VENDOR` | `INTERNAL`). **`ROH` (`vendors.kind='INTERNAL'`)** adalah unit cabang internal CIMB, bukan vendor FLM, tidak pernah ditagih; cabangnya berada di `cimb_branches`.
    *   `vendor_branches` (memiliki `region_code` FK ke `regions.code` sejak `042`; kolom `region` lama sudah di-drop; NULL = request untuk cabang tersebut ditolak).
    *   `branch_coverage_areas` (migrasi `007`: tabel link murni, **tanpa kolom soft-delete**, dihapus dengan `DELETE` baris / FK `ON DELETE CASCADE` dari `vendor_branches`; saat ini read-only).
    *   `vendor_vaults` (migrasi `008`, revamp `040`: `category` = `ATM` | `CASH` | `ATM_CASH`; kolom `type` selalu dicerminkan dari `category` — jangan pernah diset independen).
    *   `vendor_pics` (migrasi `032`: `roles text[]` ⊆ `{replenish, vault, cashcount}`, default ketiganya, kosong = kontak saja; migrasi `037`: `is_director` = Direktur Utama tingkat vendor, tanpa login/roles/notifikasi).
    *   `cimb_branches`, `package_frequencies` (`cr_frequency` read-only).
    *   `locations`, `regions` (migrasi `018` soft-delete, `042_region_standardization.sql`: satu daftar region baku untuk ATM `locations.region_id` dan `vendor_branches.region_code`; 13 kode baku gaya cabang vendor: `BALNUS, INDTIM, JABAR, JATENG, JATIM, JKTBAR, JKTPST, JKTSEL, JKTTIM, JKTUTR, KAL, SUMATERA, TGR`, format `^[A-Z0-9]{2,10}$`; nonaktif diblokir di dalam tx setelah `FOR UPDATE` jika masih direferensikan oleh `locations` atau `vendor_branches` aktif), `region_cities` (`042`: 22 bekas kode kota, tabel saja tanpa layar/API).
    *   `atms` (`terminal_id` immutable setelah dibuat; `cimb_branch_id`; `price_machine_group` & `price_class` adalah **generated columns** — jika tidak terpetakan menghasilkan `NULL` sehingga lookup harga gagal keras dan tidak salah hitung).
*   **Tabel Harga & Kelolaan ATM (Effective-Dated)**:
    *   **Dua tabel harga terpisah (jangan dicampur)**:
        1. `vendor_package_prices` (migrasi `009`, `017`): pohon harga tingkat PT/cabang/ATM-override vendor-wide. Kolom `package` = label (mis. `"PAKET 3"`, join ke `package_frequencies.package_code`); `package_code` = kode unik per baris yang **di-generate server saat apply, tidak pernah dikirim klien**.
        2. `vendor_packages_branch` (migrasi `011`, `016`): tabel harga khusus tingkat cabang ("harga khusus cabang").
        *Keduanya bersifat effective-dated: menonaktifkan ("disable") dilakukan dengan menutup `effective_end_date`, bukan toggle boolean dan tidak pernah hard-delete.*
    *   `atm_vendor_packages` (= kelolaan/assignment ATM; **tidak ada tabel `vendor_assignments`**): effective-dated, tidak boleh ada periode aktif yang tumpang tindih per ATM. Sejak migrasi `023`, memiliki dua sumber paket (tepat satu via constraint `avp_source_chk`):
        1. `vendor_package_id` → `vendor_packages_branch` (paket khusus cabang), **atau**
        2. `vendor_id` + `vendor_branch_id` + `package` (label paket vendor-wide dari `vendor_package_prices.package`). Cabang pengelola selalu diketahui di kedua mode.
*   **Vendor PKS (`022`, `037`, `043_vendor_pks_type.sql`, `044_vendor_pks_list.sql`, `045`)**:
    *   `vendor_pks_types` (`043`): tabel master (maker-checker `vendor_pks_type`, `code` unik, `name`, seed default `CIT` & `FLM`; nonaktif diblokir jika masih dipakai PKS aktif/terminated).
    *   `vendor_pks`: `pks_type_id` (FK NOT NULL), `wod_number` (NOT NULL, unik global case-insensitive pada PKS aktif via `vendor_pks_wod_uq`), satu PKS aktif per `(vendor_id, pks_type_id)` via `vendor_pks_active_uq`, riwayat via `replaced_at`, terminasi via `terminated_at`, `renewal_days` (`044`: integer NOT NULL DEFAULT 270, CHECK 30–3666 dan ≤ masa PKS).
    *   Fungsi status: `vendor_pks_record_status(replaced_at, terminated_at, effective, expiry, renewal_days, today)` (`044`; fungsi lama `vendor_pks_status` & `vendor_pks_validity_months` sudah di-drop di `045`).
*   **Cakupan Pihak Vendor (`032_vendor_party_scope.sql`)**:
    *   Roles `VENDOR-ATM-USER` (hanya replenish) & `VENDOR-VAULT-USER` (hanya vault).
    *   `users.vendor_branch_id` wajib merupakan cabang dari vendor yang sama (composite FK `users_vendor_branch_owner_fk` → `vendor_branches (id, vendor_id)`).
*   **Belum Dimigrasi (`vendor-cis-limit`)**: `vendor_branches.cis_limit_amount/cis_limit_currency`, `vendor_vaults.escrow_account`.

### 3.4 ATM Operations
*   **DSR & Instruksi**: `atm_dsr_uploads`, `atm_dsr_rows`, `replenishment_instructions`, `dsr_receipts` (`033`: append-only, satu baris per commit ETL DSR yang sukses, ditulis `dsr_etl.py` dalam tx yang sama dengan `dsr_uploads`).
*   **Kuota Kunjungan Replenish (`021_atm_visit_quota.sql`)**:
    *   `atm_visit_quotas` (sisa per ATM; `remaining` boleh negatif = over-quota; tidak ada reset otomatis).
    *   `atm_visits` (satu kunjungan terhitung per `(vendor_request, ATM)`, UNIQUE, hanya soft-cancel).
    *   Kuota berasal dari `package_frequencies.cr_frequency` paket aktif (read-only).
*   **Tiket Replenish & Nomor Request per Region (`026_replenish_ticket_region.sql`)**:
    *   `vendor_requests`: nomor request baru `REP-<vendor prefix>-<region_code>-<YYYYMMDD>-<NNN>`, menyimpan snapshot `region_code`, status lifecycle termasuk `completion_pending` / `completed`.
    *   `vendor_request_number_seq`: counter per `vendor + region_code + date`.
    *   `vendor_request_tickets`: satu tiket **aktif** per `(request, ATM)` = satu trip, format `<terminal_id>_<50K|100K|MIX>_<YYYYMMDD replenish_date>_<NNN>` (NNN global per terminal_id + replenish_date, tidak pernah dipakai ulang; FK `request_number` → `vendor_requests.request_number`; menyimpan laporan selesai `result`).
*   **Realisasi Replenish vs Order (`031_replenish_realization.sql`)**:
    *   `vendor_request_realizations` (+ `_amounts`): laporan realisasi dari vendor, satu per tiket replenish.
    *   `replenish_classifications` (+ `_amounts`): per tiket atau `no_order` untuk baris `itm_replenish` tanpa order; hanya ditulis oleh job Python `replenish_classify`; baris final tidak pernah dihitung ulang; menyimpan nominal order/ITM/vendor + generated `diff` (`bigint` IDR).

### 3.5 CIT & Vault ACM
*   **Penetapan & Alokasi Vault ACM (`027`, `028`, `030_vault_allocation.sql`)**:
    *   `dsr_location_vault_maps`: pemetaan label lokasi DSR → `vendor_vaults` (master-data maker-checker).
    *   `acm_areas`, `acm_area_regions` (`030`: `region_code` → satu Area ACM), `acm_area_members` (`acm_area_branches` & `vendor_request_vault_assignments` dari `027` dipertahankan tapi tidak dipakai lagi untuk plan baru).
    *   `vendor_request_vault_plans`: satu plan aktif per request (partial unique) + `buffer_reason`.
    *   `vendor_request_vault_allocations` (+ `_amounts`, kelipatan denominasi): alokasi per vault (`CASH`/`ATM_CASH`) dalam satu plan.
    *   `vendor_request_vault_tickets`: tiket pemenuhan vault per `request × vault`, format `<vault_code>_<YYYYMMDD>_<NNN>`, tidak pernah dipakai ulang.
*   **Kirim ke Vendor (`029_vendor_request_send.sql`)**:
    *   `vendor_request_vendor_parties`: satu baris per `request × branch × role (replenish|vault)`, menyimpan `content` jsonb snapshot, status `pending/accepted/rejected/withdrawn`, tidak pernah dihapus.
    *   `vendor_request_vendor_party_events`: log event append-only.
*   **Tabel CIT Lainnya**: `cit_orders`, `cit_handover_evidences`, `cit_journals`, `cit_dsr_uploads`, `cit_reconciliation_results`.

### 3.6 Cash Count, Finance & Integration
*   **Cash Count Parameters (`041_cash_count_parameters.sql` — Applied)**:
    *   `cash_count_category_params` (satu set aktif, histori via `superseded_at`), `cash_count_anomaly_categories`, `cash_count_checklist_items`, `cash_count_areas` + `cash_count_area_regions` (satu region satu area), `cash_count_pics` (`COORDINATOR` per `region_code` / `EXECUTOR` per area).
*   **Cash Count Execution (Nama Tabel Disetujui ScDD 5.0 K8, Kolom Belum Dimigrasi)**:
    *   `cash_count_schedules`, `cash_count_evidences`, `cash_count_reschedule_requests`, `cash_count_machine_assignments`, `cash_count_machine_assignment_atms`, `cash_count_vault_results`, `cash_count_machine_cassettes`, `cash_count_checklist_answers`, `cash_count_reconciliations`.
*   **Finance**: `invoice_uploads`, `invoice_items`, `invoice_reconciliation_results`.
*   **Integration**: `escrow_batch_files`, `escrow_batch_rows`, `escrow_reconciliation_results`.
