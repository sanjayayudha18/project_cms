# Spec: Penetapan branch vault per ATM oleh tim ACM (Phase 2.2a)

Status: **draft revisi 2 — 2026-10-08** (ditulis ulang mengikuti `intent.md` revisi 2; spec "rencana CIT harian" digantikan).
**Accepted 2026-10-08** (user: "Diterima, lanjut plan"). Stage: 2 Design — selesai. Stage berikut: `plan.md`.

## ⚠ Flagged (Sec 4 #7)
- **Alur approval Vendor Request berubah** (status baru, laporan selesai dipindah ke status `ready`) — alur yang sudah live.
- **Auth/RBAC**: role baru `ACM-USER`, `ACM-SPV`; scoping per Area ACM.
- **Skema**: migrasi `027`.
- **ETL DSR** berubah (simpan SALDO AKHIR per vault per denom).
- **Uang**: saldo & kapasitas vault (tampilan + snapshot, tidak memindahkan uang).
- **Deviasi terdokumentasi** (dicatat di CLAUDE.md Sec 12 saat diterapkan):
  (a) maker-checker rekomendasi vault memakai state machine sendiri (pola Vendor Request), bukan `approval_requests`;
  (b) Area ACM dikelola ADMIN **langsung + audit** (pola Role Management, GR#3).

## Alur & status

```
Vendor Request (vendor_requests.status)
draft → pending_approval --ATM-SPV approve--> vault_assignment --semua area acm_approved--> vault_review
vault_review --ATM-SPV approve--> ready            (fase 2.2a selesai; laporan selesai boleh dari ready)
vault_review --ATM-SPV reject(alasan)--> vault_assignment  (semua bagian area kembali ke draft, alasan ditampilkan)
approved = status lama: request yang di-approve sebelum rilis; laporan selesai tetap boleh dari approved (F12)

Rekomendasi vault per (request, Area ACM) (vendor_request_vault_plans.status)
draft --ACM-USER submit--> pending_acm_approval --ACM-SPV approve--> acm_approved
pending_acm_approval --ACM-SPV reject(alasan)--> draft
acm_approved --ATM-SPV reject request--> draft
(request dibatalkan) → cancelled
```

## Functional requirements

### FR1 — DSR: saldo akhir per vault per denom
- FR1.1 `dsr_etl.py` menyimpan baris `SALDO AKHIR` tiap blok sebagai `dsr_daily_rows` (`section='d0'`, `flow='saldo_akhir'`,
  `location` = label blok, `denom_*_idr`, `line_total_idr`). Perilaku ETL lain tidak berubah.
- FR1.2 Blok tanpa lokasi (`SALDO HARIAN ATM`) diabaikan. DSR lama tidak di-backfill.

### FR2 — Master data: mapping lokasi DSR → vault
- FR2.1 Entitas `dsr_location_map`: (`vendor_id`, `dsr_location`) → `vendor_vault_id` (vault aktif milik branch vendor yang sama).
  DSR dikaitkan ke vendor lewat `dsr_uploads.vendor` = `vendors.name`.
- FR2.2 Unik (`vendor_id`, lower(trim(`dsr_location`))) di antara yang aktif; satu vault boleh punya beberapa label.
- FR2.3 Pola master-data maker-checker (Sec 12): `Submit` → 202 → `DsrLocationMapApplier`; RBAC `ADMIN`/`ADMIN_PARAM` di route + `Submit`.
- FR2.4 Tab "Mapping DSR" di detail vendor + daftar lokasi DSR yang belum terpetakan.

### FR3 — Saldo & kapasitas branch vault
- FR3.1 **Saldo** branch B untuk tanggal X per denom = Σ `saldo_akhir` DSR `report_date` = X dari semua vault aktif B (via FR2).
  **Tidak diketahui** (NULL, bukan 0) bila: tidak ada DSR vendor B untuk X, DSR tanpa baris `saldo_akhir`, atau ada vault aktif B
  yang tidak punya lokasi terpetakan di DSR X.
- FR3.2 **Kebutuhan sendiri** B = Σ order ATM yang branch replenish-nya B, request ber-tiket aktif `replenish_date` X, status
  `vault_assignment`/`vault_review`/`ready` dan belum ditetapkan ke vault lain. (ATM yang vault-nya B sudah dihitung di FR3.3.)
  **Revisi user 2026-10-08 (opsi a):** status yang dihitung = semua request `vault_flow` tanggal X yang tidak batal/ditolak
  (termasuk `completion_pending`/`completed`), agar kas yang sudah keluar tetap mengurangi kapasitas.
- FR3.3 **Alokasi** ke B = Σ order ATM (request mana pun, tanggal X) yang sudah ditetapkan ke vault B pada rekomendasi berstatus
  bukan `cancelled`.
- FR3.4 **Sisa kapasitas** per denom = saldo − kebutuhan sendiri − alokasi (kecuali ATM yang sedang diedit). Hanya untuk
  **peringatan**; tidak memblokir (F5).

### FR4 — Rekomendasi vault oleh ACM
- FR4.1 Saat ATM-SPV approve request (`pending_approval → vault_assignment`), sistem membuat satu **rencana vault per Area ACM**
  (`draft`) berdasarkan branch replenish tiap ATM (F10/F10b), dalam tx approve yang sama. ATM yang branch replenish-nya **tanpa
  area** → request tetap di `vault_assignment` + peringatan ADMIN (FR7.4); tidak bisa maju sampai branch itu di-assign ke area
  (rencana dibuat saat itu juga oleh aksi ADMIN assign).
- FR4.2 Untuk tiap ATM, sistem menampilkan **kandidat branch vault** (`category` ∈ `CASH`,`ATM_CASH`, aktif) berurut tier (F6, F7):
  **Tier 1** vendor sama dengan vendor request + `region_code` sama (termasuk branch replenish bila `ATM_CASH`);
  **Tier 2** vendor lain, `region_code` sama; **Tier 3** region lain — hanya bila `urgent=true`. Dalam tier diurut sisa kapasitas
  terbesar; saldo tidak diketahui ditampilkan paling bawah dengan label. Usulan default = kandidat teratas Tier 1 (bila ada).
- FR4.3 ACM-USER anggota area menyimpan penetapan (replace-all selama `draft`): per ATM tepat **satu** `vault_branch_id`
  (F13); Tier 3 wajib `is_urgent` + `urgent_reason` 10–500 karakter. Snapshot saldo & sisa kapasitas per denom saat simpan
  disimpan bersama penetapan; `capacity_warning` = sisa < order ATM untuk salah satu denom atau saldo tidak diketahui.
- FR4.4 **Submit** (ACM-USER anggota area): semua ATM bagian area sudah punya vault. Server menghitung ulang kandidat/tier
  (vault harus masih aktif & bertipe benar; Tier 3 harus urgent). Peringatan kapasitas tidak memblokir, tapi dihitung ulang.
- FR4.5 **ACM-SPV** anggota area approve / reject (alasan 1–500). Maker ≠ checker (user yang submit ≠ approver).
- FR4.6 Saat **semua** rencana area untuk request itu `acm_approved` → request `vault_assignment → vault_review` (dalam tx approve terakhir).

### FR5 — Review tim ATM
- FR5.1 ATM-SPV / BRANCH-ATM-SPV melihat penetapan vault per ATM (tier, urgent + alasan, saldo/kapasitas snapshot, peringatan)
  di detail Vendor Request.
- FR5.2 **Approve** → request `ready`. **Reject** (alasan 1–500) → request `vault_assignment`, semua rencana area → `draft`,
  alasan tercatat dan tampil ke ACM.
- FR5.3 Checker tidak boleh user yang submit/approve rencana ACM request itu (maker ≠ checker lintas lapis).
- FR5.4 Laporan selesai (`completion_pending`) hanya dari `ready` (request baru) atau `approved` (request lama, F12).
- FR5.5 Cancel request (aturan cancel yang ada) juga berlaku di `vault_assignment` / `vault_review` / `ready` (checker); rencana
  area → `cancelled`.

### FR6 — Audit, RBAC, notifikasi
- FR6.1 Setiap create/simpan/submit/approve/reject/cancel → `audit_logs` dalam tx (entity `vendor_request` untuk status request,
  `vendor_request_vault_plan` untuk rencana area; urgent reason di `after`).
- FR6.2 Route `/api/v1/vault-plans` `RequireRoles("ACM-USER","ACM-SPV","ADMIN")`; service cek keanggotaan area; non-anggota → 404.
  ADMIN read-only. Review vault di route Vendor Request yang ada (`ATM-SPV`, `BRANCH-ATM-SPV`).
- FR6.3 Notifikasi in-app + email (`internal/notification.Send`, dalam tx pemicu) — **dikonfirmasi PO (N1)**:
  request masuk `vault_assignment` → ACM-USER area terkait; submit rencana → ACM-SPV area; ACM-SPV reject → ACM-USER area;
  ATM-SPV approve/reject → ACM-USER area + pembuat request. **Tidak ada** notifikasi saat request masuk `vault_review`.

### FR7 — Area ACM (R2–R8)
- FR7.1 ADMIN CRUD area (nama unik), pilih branch **satu-satu** (vendor mana pun), anggota (`ACM-USER`/`ACM-SPV` aktif).
  Langsung berlaku + `audit_logs` dalam tx.
- FR7.2 Satu branch aktif hanya di satu area aktif (unik DB); user boleh di beberapa area.
- FR7.3 Assign branch ke area → request di `vault_assignment` yang punya ATM branch itu tanpa rencana area → rencana dibuat.
  Mengeluarkan branch tidak mengubah rencana yang sudah ada.
- FR7.4 Layar Area ACM: peringatan **branch replenish dengan request di `vault_assignment` tanpa area**.

### FR8 — Role & menu
- Role `ACM-USER`, `ACM-SPV`; `menu_features` `cit` → `cit.vault-plan` ("Penetapan Vault"), `settings.acm-areas`;
  `role_permissions` ACM-USER/ACM-SPV/ADMIN.

### FR9 — Frontend (CompanyPortal-Vite)
- FR9.1 **CIT → Penetapan Vault** (ACM): daftar rencana area milik user (filter status/tanggal); detail per ATM: terminal,
  branch replenish, order per denom, pilihan vault (dropdown bertier + kapasitas), toggle urgent + alasan, peringatan kapasitas;
  aksi submit / approve / reject sesuai role.
- FR9.2 **Detail Vendor Request**: status baru + panel "Penetapan Vault" (read-only untuk ATM; approve/reject di `vault_review`).
- FR9.3 **Pengaturan → Area ACM** (ADMIN) + panel peringatan; **tab Mapping DSR** di detail vendor.
- FR9.4 Uang `tabular-nums`, rata kanan, IDR eksplisit; status tidak hanya warna.

## Non-functional
- NFR1 Detail rencana (≤ 1000 ATM/request) ≤ 3 s p95; kandidat & kapasitas dihitung set-based.
- NFR2 Tulis primary; daftar dari replica; read-after-write dari primary.
- NFR3 Transisi `SELECT … FOR UPDATE` pada request + rencana; transisi "semua area approved" aman terhadap dua approve paralel.
- NFR4 Coverage ≥ 80% service baru; integration test Postgres nyata.

## API (flat JSON, pola handler `backend/`)
| Method | Path | Role |
|---|---|---|
| GET | `/api/v1/vault-plans?status=&from=&to=&area_id=` | ACM-USER, ACM-SPV (area sendiri), ADMIN |
| GET | `/api/v1/vault-plans/{id}` | idem — detail + ATM + penetapan |
| GET | `/api/v1/vault-plans/{id}/candidates?terminal_id=&urgent=` | ACM-USER |
| PUT | `/api/v1/vault-plans/{id}/assignments` | ACM-USER anggota (`draft`) |
| POST | `/api/v1/vault-plans/{id}/submit` · `/approve` · `/reject` | ACM-USER / ACM-SPV anggota |
| GET | `/api/v1/vault-plans/{id}/audit` | semua di atas |
| POST | `/api/v1/vendor-requests/{id}/vault-approve` · `/vault-reject` | ATM-SPV, BRANCH-ATM-SPV |
| GET | `/api/v1/vendor-requests/{id}/vault-assignments` | role Vendor Request yang ada |
| CRUD | `/api/v1/admin/acm-areas` (+ `/branches`, `/members`, `/warnings`) | ADMIN |
| CRUD (202) | `/api/v1/admin/vendors/{vendorID}/dsr-location-maps` (+ `/dsr-locations/unmapped`) | ADMIN, ADMIN_PARAM |

## Data model (migrasi `027`)
- `vendor_requests_status_chk` + `vault_assignment`, `vault_review`, `ready`; kolom `vault_reviewed_by`, `vault_reviewed_at`,
  `vault_rejection_reason`. + `vault_flow boolean NOT NULL DEFAULT false` (P1, plan.md: true = request lewat alur vault; menentukan
  kembali ke `ready` vs `approved` saat laporan selesai ditolak).
- `dsr_daily_rows_flow_chk` + `saldo_akhir`.
- `dsr_location_vault_maps` (`vendor_id`, `dsr_location`, `vendor_vault_id`, `is_active`, `deleted_at`, timestamps).
- `acm_areas` (`name`, `is_active`, `deleted_at`); `acm_area_branches` (`acm_area_id`, `vendor_branch_id` UNIQUE);
  `acm_area_members` (`acm_area_id`, `user_id`) — link table, perubahan diaudit.
- `vendor_request_vault_plans` — `id`, `vendor_request_id`, `acm_area_id`, `status`, `submitted_by/at`, `approved_by/at`,
  `rejected_by/at`, `rejection_reason`, timestamps; UNIQUE (`vendor_request_id`, `acm_area_id`).
- `vendor_request_vault_assignments` — `id`, `vault_plan_id`, `vendor_request_id`, `terminal_id`, `replenish_branch_id`,
  `vault_branch_id`, `tier` (1–3), `is_urgent`, `urgent_reason`, `saldo_snapshot` jsonb (per denom, NULL = tidak diketahui),
  `capacity_snapshot` jsonb, `capacity_warning` bool, timestamps; UNIQUE (`vendor_request_id`, `terminal_id`).
  Replace selama `draft` = hard delete baris draft (S1, diaudit).
- Seed: roles, `menu_features`, `role_permissions`.

## Out of scope
Kirim request ke vendor / VendorPortal (2.2b); pindah buku / escrow; realisasi, `holidays` (2.2c); ubah kelolaan; CSV mapping;
backfill DSR; rename `backend-cit`.

## Resolved (PO, 2026-10-08)
- N1 — Notifikasi: ACM-USER (request masuk), ACM-SPV (submit), ACM-USER + pembuat request (hasil review ACM-SPV / ATM-SPV). Tanpa notifikasi `vault_review`.
- N2 — Request `approved` sebelum rilis **cukup dilewati** (tanpa tombol "kirim ke ACM").
