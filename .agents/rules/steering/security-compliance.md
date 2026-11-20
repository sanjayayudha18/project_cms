# Steering: Security, Cross-Cutting Domain Rules & Resolved Decisions

> **Scope**: Auth split, RBAC, maker-checker (`approval_requests` & `master_data_change_requests`), atomic `audit_logs`, documented deviations dari Golden Rule #3, serta seluruh keputusan arsitektur/domain (Sec 5 & Sec 12).  
> **Source of Truth**: Disinkronkan dari `.claude/CLAUDE.md` (Sec 5 & Sec 12) & [decisions.md](file:///c:/Users/RB%20Yudha%20Rangga/OneDrive/Documents/Development/CMS2/.claude/docs/decisions.md).  
> **Referenced by**: [GEMINI.md](file:///c:/Users/RB%20Yudha%20Rangga/OneDrive/Documents/Development/CMS2/.agents/rules/GEMINI.md)

---

## 1. Cross-Cutting Domain Rules (Sec 5 — NON-NEGOTIABLE)

*   **Auth Split**:
    *   **Internal**: LDAP bind (tanpa password lokal di DB CMS2).
    *   **Vendor**: Kredensial lokal (bcrypt/argon2), frontend & rute login terpisah.
    *   Keduanya menerbitkan JWT yang sama dengan claim `id` (user ID — bukan `sub`), `role`, dan `auth_source` (`ldap` | `local`).
    *   **Scoping Vendor**: User vendor dibatasi ketat hanya pada kelolaan/assignment miliknya sendiri (`users.vendor_id` dari claim JWT + `users.vendor_branch_id` yang dibaca dari DB setiap panggilan; akses di luar cakupan mengembalikan `404`).
*   **Maker-Checker**: Setiap create/update/delete pada data finansial atau master data wajib melalui `approval_requests` (`pending → approved / rejected`). Efek baru berlaku setelah disetujui. **`Maker != Checker`** wajib ditegakkan.
*   **Atomic Audit Logs**: Setiap perubahan state wajib menulis ke `audit_logs` (`who`, `what`, `before/after`, `when`, `ip`) di dalam **transaksi database (`tx`) yang sama**. Jika penulisan audit gagal, seluruh transaksi wajib di-rollback.
*   **RBAC Ganda**: Penegakan izin role wajib dilakukan di **middleware HTTP DAN di dalam service layer**.
    *   **Dua Sistem Otorisasi Paralel (Kiro Steering Sync)**:
        1.  *Katalog DB `role_permissions`* (`menu_features` $\times$ `roles`): Dikelola dinamis via layar "Manajemen Peran", saat ini **hanya mengontrol visibilitas navigasi frontend + UI Role Management**.
        2.  *Hardcoded `RequireRoles(...)`*: Penegakan riil backend di `backend/cmd/api/main.go` dan `Routes()` masing-masing handler via `pkg/middleware/rbac.go`. Evaluasi backend mencocokkan string claim `role` dari JWT (case-insensitive allow-list), bukan runtime DB permission query.
    *   **Pemisahan Tegas: Role vs Hirarki Supervisor**:
        *   `Role` = kapabilitas fitur (siapa boleh mengakses endpoint/menu apa).
        *   `supervisor_id` + `approval_level` = struktur pelaporan (siapa melapor ke siapa, pure tree hierarchy, 1 atasan langsung).
        *   Maker-checker finansial/transaksional bertumpu pada hirarki supervisor & nominal (`approval_policies`), bukan sekadar role. Endpoint `/api/v1/approvals` hanya dijaga `RequireAuth` karena validasi approver dilakukan dinamis terhadap rantai supervisor.
        *   **Delegasi & Cuti Approver (Block-Leave Fallback)**: Jika approver sedang cuti aktif (`user_leaves` beririsan dengan waktu approval), permintaan dialihkan ke penerima mandat aktif di `approval_delegations`, dengan guard mutlak: **`delegate != maker`**.
*   **Katalog 14 Role Resmi CMS2**:
    *   *Internal (11)*: `ADMIN` (Superadmin), `ADMIN_PARAM` (Parameter & Master ATM/Vendor), `ATM-USER` (Maker replenish ATM), `ATM-SPV` (Checker replenish ATM & vault review), `BRANCH-USER` (Operasional kas cabang), `BRANCH-SPV` (Supervisor kas cabang), `BRANCH-ATM-USER` (Maker ATM lingkup cabang), `BRANCH-ATM-SPV` (Checker ATM lingkup cabang), `ACM-USER` (Alokasi & penetapan vault CIT), `ACM-SPV` (Persetujuan vault plan area), `APPACCESS` (User provisioning & konfigurasi peran/delegasi).
    *   *Vendor (3)*: `VENDOR-USER` (Akses penuh vendor), `VENDOR-ATM-USER` (Pihak replenish ATM saja, ter-scope `vendor_branch_id`), `VENDOR-VAULT-USER` (Pihak penyedia uang vault saja, ter-scope `vendor_branch_id`).
    *   *Service Client (1)*: `EXT-CLIENT` (Khusus CIMB IDM OAuth2 di `backend-extapi` :8082).
    *   *Larangan*: Selalu gunakan string kanonis role, **dilarang hardcode ID integer role** karena ID dibuat sekuensial per migrasi. Role `CMOC-*` berstatus rencana/belum di-seed.
*   **Trusted Client IP (QC F13, 2026-10-12)**:
    *   Client IP (untuk rate limit login dan `audit_logs.ip`) hanya diambil dari `r.RemoteAddr` yang diatur oleh `pkg/middleware.TrustedRealIP(cfg.TrustedProxies)` di `backend/cmd/api/main.go`.
    *   Header `X-Forwarded-For` hanya dibaca dari kanan ke kiri jika peer berada di dalam `TRUSTED_PROXIES`; header `X-Real-IP` dan `True-Client-IP` diabaikan. **Jangan pernah membaca header IP secara langsung di dalam handler**, dan jangan gunakan `chi.RealIP`.
*   **Idempotensi Ingest File**: Seluruh ingest file (DSR, DMAA, ITM, invoice, escrow) wajib idempoten per hash SHA-256 file → dilacak di `import_jobs` (atau `dsr_uploads` + `dsr_receipts` untuk DSR).
*   **Invoicing Trip (Keputusan 2026-11-12, FSD v1.0 §6)**:
    *   CROWN **menghitung** tagihan bulanan trip per vendor: upload skema harga (menggantikan skema sebelumnya, tanggal upload = tanggal efektif), master ATM dari ESQ, realisasi trip per ATM vs minimum paket, prorata untuk aktivasi/deaktivasi tengah bulan, dan penalti; vendor me-review dan dapat mengajukan banding (appeal), lalu masuk ke approval.
    *   **Tanpa Eksekusi Pembayaran**: Pembayaran dieksekusi di SMART (di luar CROWN). Titik akhir di CROWN adalah invoice final yang diunduh oleh VOC.
*   **Escrow Reconciliation**: Sumber = batch file dari Corebanking → ingest & parse ke `escrow_batch_rows` → rekonsiliasi terhadap posisi kas CMS → simpan delta di `escrow_reconciliation_results` (idempoten per hash file).

---

## 2. Master-Data Maker-Checker Engine (Sec 12 D1–D4)

Setiap create/update/disable/enable pada entitas master data (`vendors`, `vendor_branches`, `vendor_vaults`, `vendor_pics`, `vendor_packages_branch`, `vendor_package_prices`, `atms`, `atm_vendor_packages`, `dsr_location_vault_maps`, `vendor_pks`, `vendor_pks_types`, `cash_count_*` parameter/area/PIC) di-stage melalui alur baku:
1. `MasterDataChangeService.Submit` menulis ke `master_data_change_requests` + `approval_requests`.
2. Endpoint HTTP mengembalikan status **`202 Accepted`**.
3. Baris tabel utama hanya berubah saat approval final melalui `Applier` per-entitas yang diregistrasikan di `backend/cmd/api/main.go`, di dalam **satu transaksi (`tx`)** bersama penulisan `audit_logs`.
4. Data stale → tidak diterapkan; kegagalan apply → mengembalikan **`409 Conflict`**.
5. Impor CSV master data = satu batch (`master_data_import_batches`), satu approval, all-or-nothing (upsert).
6. **Checklist Penambahan Entitas Master Baru**: `queries → read-only repo → service Submit → Applier di main.go → handler 202 → tests → frontend`. Selalu jalankan skill `cms2-master-data` sebelum menyentuh entitas master data.

---

## 3. Documented Deviations dari Golden Rule #3 (Sec 12)

Operasi berikut telah disetujui secara resmi untuk **langsung berlaku (tanpa `approval_requests`)**, namun **WAJIB menulis `audit_logs` di dalam transaksi (`tx`) yang sama** (jika audit gagal = rollback) dan memeriksa RBAC di route + service layer:

| # | Fitur / Endpoint | Guard Role (Route + Service) | Catatan Audit & Aturan Khusus |
|---|---|---|---|
| **1** | **Manajemen User (`admin/users`)** | `APPACCESS` | Berlaku langsung; pembuatan user lokal menghasilkan password sementara + `must_change_password=true`. |
| **2** | **Role Management & Region Management** | `APPACCESS` / `ADMIN` | Berlaku langsung; `audit_logs` dalam tx yang sama. |
| **3** | **Peran PIC Vendor (`PUT .../vendors/{vendorID}/pics/{id}/roles`)** | `ADMIN` / `ADMIN_PARAM` | Mengubah `vendor_pics.roles` (`replenish/vault/cashcount`) berlaku langsung + `audit_logs` (`vendor_pic_roles_updated`, before/after) dalam tx yang sama. *(Create PIC, impor CSV PIC, dan field PIC lain tetap maker-checker).* |
| **4** | **Reminder PKS (`PUT /api/v1/admin/pks/{pksID}/reminder`)** | `ADMIN` / `ADMIN_PARAM` | Mengubah `renewal_days` berlaku langsung untuk semua baris PKS (termasuk replaced/terminated) + `audit_logs` (`vendor_pks_reminder_updated`, before/after) dalam tx yang sama. *(Create PKS & field lain tetap maker-checker).* |
| **5** | **Reset Kuota Kunjungan ATM & Cancel Visit** | `ADMIN` / `ATM-SPV` / `BRANCH-ATM-SPV` | Berlaku langsung + `audit_logs` dalam tx yang sama. |
| **6** | **Admin Area ACM & Kalender Holidays** | `ADMIN` | Pengelolaan area/region/member ACM dan tabel `holidays` berlaku langsung + `audit_logs` dalam tx yang sama. |
| **7** | **External User API / CIMB IDM (`backend-extapi` :8082)** | OAuth2 Client (`api_clients`) | Berlaku langsung + `audit_logs` dalam tx yang sama dengan `actor_id` = `service_user_id` client; role istimewa (`ADMIN`, `APPACCESS`, `ADMIN_PARAM`, `EXT-CLIENT`) dilarang disentuh; dibatasi `assignable_roles` per client. |
| **8** | **State Machine Domain Khusus** | Sesuai Role Alur | **Vendor Request**, **Vault Plans** (`vendor_request_vault_plans`), dan **Keputusan Pihak Vendor** (`vendor_request_vendor_parties`) menggunakan kolom state machine pada tabelnya masing-masing (bukan `approval_requests`), tetap enforcing `Maker != Checker` dan `audit_logs` dalam tx yang sama. |
| **9** | **Aksi Sistem/Scheduler EOD & Notifikasi** | System / Self-scoped | Aksi user menulis ke `audit_logs`; aksi scheduler otomatis menulis ke `retry_audit_logs` (karena `audit_logs.actor_id` adalah NOT NULL). Tabel `notifications` dihapus permanen (hard-delete) setelah 90 hari dan mark-read tidak menulis `audit_logs`. |

---

## 4. Keputusan Alur Domain Operasional (Sec 12 Summary)

*   **Tiket Replenish & Region (`026`)**:
    *   Nomor request baru: `REP-<vendor prefix>-<region_code>-<YYYYMMDD>-<NNN>`. Seluruh ATM dalam satu request harus merujuk pada satu `region_code` aktif yang sama (cabang tanpa kode → `422`). Nomor lama (`REP-<v>-<date>-<n>`, `VR-…`) tidak pernah ditulis ulang.
    *   Setiap `(request, ATM)` mendapat satu tiket aktif saat pembuatan request di bawah advisory lock per `(terminal_id, replenish_date)`. Edit hanya menonaktifkan/mengaktifkan kembali/menerbitkan tiket baru dan **tidak pernah menggunakan ulang nomor tiket**.
*   **Penetapan & Pemenuhan Vault ACM v3 (`027`, `030`)**:
    *   Menggantikan alokasi per-ATM dari `027`: kini **satu plan per request** (`vendor_request_vault_plans`), dimiliki oleh Area ACM yang memegang `region_code` request tersebut (`acm_area_regions`).
    *   `ACM-USER` membagi order per denominasi ke satu atau lebih vault (`CASH`/`ATM_CASH`), masing-masing kelipatan denominasi; total per denominasi harus $\ge$ order (kelebihan = buffer dengan satu alasan 10–500 karakter per plan).
    *   Tier per vault terhadap vendor + region request (Tier 1: vendor & region sama; Tier 2: vendor beda, region sama; Tier 3: region beda = wajib flag urgent + alasan).
    *   Kapasitas = `saldo_akhir` DSR dikurangi alokasi plan aktif lain pada tanggal tersebut (hanya peringatan, tidak memblokir).
    *   Submit menerbitkan **tiket pemenuhan vault** (`<vault_code>_<YYYYMMDD>_<NNN>`) di bawah advisory lock.
    *   `ACM-SPV` menyetujui plan → `ATM-SPV` melakukan vault review (`Maker != Checker` di dalam ACM maupun lintas layer).
*   **Kirim Vendor Request ke Vendor (`029`)**:
    *   Saat `ATM-SPV` menyetujui vault review, status langsung menjadi `sent_to_vendor`.
    *   Setiap cabang replenish dan cabang vault pada request menjadi pihak (`vendor_request_vendor_parties`) yang melakukan accept/reject (alasan 10–500 karakter) di VendorPortal (`/api/v1/vendor/replenish-orders`).
    *   Semua pihak accept → `vendor_accepted`. Jika pihak vault reject → kembali ke `vault_assignment` (hanya plan terdampak kembali ke `draft`); jika pihak replenish reject → `vendor_rejected` → `ATM-SPV` melakukan `vendor-return` atau cancel.
*   **Realisasi Replenish vs Order — 2.2c (`031`)**:
    *   Laporan selesai untuk request dengan `vendor_sent=true` dilaporkan oleh **vendor** (hanya pihak replenish di VendorPortal `POST /api/v1/vendor/replenish-orders/{id}/realization`: berhasil/gagal + tanggal + nominal per denominasi untuk seluruh ATM sekaligus → `completion_pending`); `ATM-SPV`/`BRANCH-ATM-SPV` melakukan approve/reject (approve menghitung 1 kunjungan di `atm_visits` per ATM berhasil dalam tx yang sama; over-quota memberi peringatan + notifikasi ke maker & seluruh `ATM-SPV`). Request legacy (`vendor_sent=false`) tetap dilaporkan oleh `ATM-USER`.
    *   Klasifikasi dijalankan di **Python EOD** (`backend_python/replenish_classify/`): mencocokkan pengisian ITM ±2 hari kerja (terdekat menang) → `on_schedule | early | late | not_done | no_order | void`. Angka ITM menang atas laporan vendor (selisih menjadi flag). Python menulis notifikasi langsung ke `notifications` + `notification_emails`.
*   **Laporan DSR Telat — 2.3 (`033`)**:
    *   Penerimaan DSR = commit ETL yang sukses (`dsr_receipts`, ditulis `dsr_etl.py` dalam tx yang sama dengan `dsr_uploads`); batas waktu **09:00:00 WIB inklusif**.
    *   Cabang wajib kirim = cabang FLM aktif dengan $\ge 1$ vault aktif.
    *   Variabel env `DSR_LATE_START_DATE` (Go) dan `RETRY_DSR_LATE_START_DATE` (Python) **wajib bernilai sama**.
*   **Kelolaan ATM — Sumber Paket (`023`)**:
    *   Memilih dari **paket khusus cabang** (`vendor_packages_branch`) atau **paket vendor-wide** (`vendor_package_prices`); cabang pengelola selalu dipilih di kedua mode.
    *   Submit vendor-wide divalidasi saat submit **dan** saat apply (harus ada tarif yang cocok dengan `price_machine_group`/`price_class` ATM; jika gagal saat apply → `409 ErrATMAssignmentSourceInvalid`).
    *   Pembaca me-resolve label paket sebagai `COALESCE(p.package_code, avp.package)` dan cabang sebagai `COALESCE(p.vendor_branch_id, avp.vendor_branch_id)`.
