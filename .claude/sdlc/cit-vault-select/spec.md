# Spec: Pemenuhan vault ACM — bagi nominal request ke vault + tiket pemenuhan vault

Author: Claude, dari `intent.md` v3 (accepted 2026-10-13, "accept intent, lanjut spec").
Status: **v3 accepted — 2026-10-13** ("accept spec, lanjut plan"). (v1/v2 per-ATM + tiket per branch/area: superseded, lihat intent "Riwayat".)
Stage: 2 Design. Stage berikut: `plan.md` (v3).

Mengganti di `cit-acm-plan/spec.md` (accepted): **FR3** (saldo/kapasitas), **FR4.1–FR4.4** (rencana per area, penetapan per ATM),
**FR5.1**, **FR7.1–FR7.4** (area per cabang), **FR9.1–FR9.3**; di `cit-send-vendor/spec.md`: **FR1.2/FR3** (isi pihak), alur
"vault reject → hanya plan area terdampak". Yang **tetap**: status request (`vault_assignment → vault_review → sent_to_vendor …`),
state machine plan (`draft → pending_acm_approval → acm_approved`, `cancelled`), maker ≠ checker, aturan reject/cancel,
notifikasi per event, RBAC route.

**Flag Sec 4 #7**: migrasi skema (030 v3), perhitungan uang (total alokasi ≥ order, kelipatan pecahan, kapasitas), alur approval
(satu rencana per request), data lintas vendor di VendorPortal (cabang pengisi melihat vault vendor lain). Tidak menyentuh auth.

## Keputusan (intent v3)
V1 bagi nominal per request (bukan per ATM) · V2 satu area per request · V3 area dari `region_code` request · V4 total ≥ order,
lebih = buffer + alasan · V5 tier per vault · V6 cabang pengisi lihat semua vault · V7 tiket per vault · V8 kapasitas = saldo −
alokasi tersimpan · V9 tiket saat submit, vault sama = nomor tetap · V10 tiket di CompanyPortal + VendorPortal · V11 on-prem di luar
scope · V12 kelipatan pecahan · V13 satu alasan buffer per rencana · Q2 vault `CASH`/`ATM_CASH` · dari v1: U6 branch tanpa vault
eligible disembunyikan, U8 kode + alamat vault ke vendor (alamat vault, fallback branch).

## Functional requirements

### FR1 — Area ACM per region (mengganti 2.2a FR7.1–FR7.4)
- FR1.1 ADMIN memetakan **`region_code` → Area ACM**: satu region tepat di satu area aktif (unik DB); satu area boleh banyak region.
  Kode region mengikuti `vendor_branches.region_code` (`^[A-Z0-9]{2,10}$`); layar menawarkan daftar kode yang dipakai cabang aktif.
- FR1.2 Sama seperti admin area 2.2a: **langsung berlaku** + `audit_logs` dalam tx, `ADMIN` di route + service (deviasi GR#3 yang
  sudah terdokumentasi). Anggota area (`acm_area_members`) tidak berubah.
- FR1.3 Memetakan region ke area → request di `vault_assignment` dengan region itu yang belum punya rencana → rencana dibuat
  (dalam tx yang sama). Memindah/menghapus region dari area **tidak** mengubah rencana yang sudah ada.
- FR1.4 Peringatan ADMIN (layar Area ACM): request di `vault_assignment` tanpa rencana karena **region tanpa area** atau
  **`region_code` request NULL** (nomor format lama, mis. 1 request di dev).
- FR1.5 Mapping cabang → area (`acm_area_branches`, tab "Cabang") **tidak dipakai lagi** untuk rencana baru; tab diganti "Region".
  Tabel tidak dihapus (out of scope).

### FR2 — Rencana pemenuhan vault per request (mengganti 2.2a FR4.1)
- FR2.1 ATM-SPV approve (`pending_approval → vault_assignment`, juga approve ulang setelah edit 2.2b) membuat **satu** rencana `draft`
  untuk area pemilik `vendor_requests.region_code`, dalam tx approve. Tidak ada area → request tetap `vault_assignment` + FR1.4.
- FR2.2 Maksimal satu rencana **bukan `cancelled`** per request (unik DB parsial). Rencana yang sudah ada dipakai ulang saat approve ulang
  (seperti sekarang: kembali `draft`).
- FR2.3 Rencana menampilkan ATM request + order per denom (read-only) dan **total order per denom** (Σ `vendor_request_items`).

### FR3 — Kandidat vault
- FR3.1 Vault eligible = `vendor_vaults` aktif (`is_active`, `deleted_at IS NULL`), `category IN ('CASH','ATM_CASH')`, milik branch aktif
  `CASH`/`ATM_CASH` ber-`region_code`. Branch tanpa vault eligible tidak tampil (U6).
- FR3.2 **Tier per vault** terhadap **vendor + region request** (V5): 1 = vendor sama + region sama; 2 = vendor lain + region sama;
  3 = region lain, hanya bila toggle **urgent** di daftar kandidat aktif.
- FR3.3 Per vault: saldo per denom (FR5.1), sisa kapasitas per denom (FR5.3), tanda tidak diketahui.
- FR3.4 Urutan: tier, saldo diketahui dulu, sisa kapasitas terbesar (Σ denom yang dipesan request), `vault_code`. Dikelompokkan per
  branch vendor di UI (pilih branch → vault, permintaan awal user).

### FR4 — Alokasi (simpan / submit)
- FR4.1 ACM-USER anggota area menyimpan (replace-all selama `draft`): daftar **baris vault** `{vendor_vault_id, nominal per denom,
  is_urgent, urgent_reason}` + `buffer_reason` rencana.
- FR4.2 Validasi **simpan** (422): vault eligible (FR3.1); vault tidak dobel; tiap baris punya ≥ 1 nominal > 0; nominal bilangan bulat
  IDR, **> 0 dan kelipatan pecahannya** (V12; juga CHECK DB); denom ∈ {100000, 50000, 20000, 10000, 5000, 2000, 1000}; tier 3 wajib
  `is_urgent` + `urgent_reason` 10–500; `urgent_reason` hanya bila `is_urgent`. Total boleh belum lengkap saat simpan.
- FR4.3 Validasi **submit** (422), dihitung ulang dari data saat itu: semua aturan FR4.2; untuk **setiap denom** Σ alokasi **≥ total
  order** (kurang → "Alokasi 100K kurang Rp…"); bila ada denom dengan Σ alokasi **> order** (termasuk denom yang tidak dipesan) →
  `buffer_reason` wajib 10–500 (V4, V13); bila tidak ada kelebihan → `buffer_reason` disimpan NULL.
- FR4.4 Snapshot per baris saat simpan + submit: saldo vault, sisa kapasitas, `capacity_warning` (saldo tidak diketahui atau sisa <
  nominal baris untuk salah satu denom). Hanya **peringatan**, tidak memblokir.
- FR4.5 Submit → plan `pending_acm_approval` + tiket (FR6), satu tx. ACM-SPV approve/reject, ATM-SPV review, cancel: aturan 2.2a
  tetap (maker ≠ checker lintas lapis).
- FR4.6 Plan disetujui ACM-SPV → request `vault_review` (satu plan per request, jadi langsung).

### FR5 — Saldo & kapasitas per vault (mengganti 2.2a FR3)
- FR5.1 **Saldo** vault V tanggal X per denom = Σ `saldo_akhir` DSR `report_date` = X (`daily_status='completed'`, section `d0`)
  dari lokasi DSR yang dipetakan ke V. **Tidak diketahui** (NULL, bukan 0) bila tak ada baris terpetakan atau ada denom NULL.
- FR5.2 **Alokasi** ke V tanggal X = Σ nominal baris alokasi ke V dari rencana bukan `cancelled` milik request `vault_flow` dengan
  `replenish_date` = X, tidak batal/ditolak (status yang dihitung = revisi 2.2a 2026-10-08). Order yang **belum** dialokasikan tidak
  dihitung (V8).
- FR5.3 **Sisa kapasitas** = saldo − alokasi, **tidak menghitung rencana yang sedang diedit** (agar ACM melihat ruang sebelum
  alokasinya sendiri).

### FR6 — Tiket pemenuhan vault (V7, V9, V10)
- FR6.1 Satu tiket **aktif** per (request × vault). Nomor `<vendor_vaults.vault_code>_<YYYYMMDD replenish_date>_<NNN>`; NNN 001–999
  berurut **global per (vault, tanggal replenish)**, tidak pernah dipakai ulang; penerbitan dengan advisory lock per (vault, tanggal)
  dalam tx submit (pola tiket 026). > 999 → 409.
- FR6.2 Sinkronisasi saat **submit**: vault di alokasi + tiket aktif → **tetap** (nominal berubah tidak mengganti nomor); vault di
  alokasi + tiket nonaktif (request sama) → **diaktifkan lagi**, nomor sama; vault baru → nomor baru; tiket aktif yang vault-nya tidak
  lagi dialokasikan → **nonaktif** (`deactivated_at`).
- FR6.3 Reject (ACM-SPV, ATM-SPV, vendor) dan cancel **tidak** mengubah tiket. Tidak ada hard delete.
- FR6.4 Audit submit `after.vault_tickets = {issued, kept, reactivated, deactivated}`.

### FR7 — Pihak vendor (mengubah 2.2b FR1.2/FR3)
- FR7.1 Pihak tetap per (request × branch × peran): **replenish** = cabang pengisi ATM request; **vault** = branch pemilik vault yang
  dialokasikan. Kunci pihak & state machine 2.2b tidak berubah.
- FR7.2 Isi **pihak vault**: nomor request, tanggal, **vault-vault miliknya** di request ini `{ticket_number, vault {id, code,
  address}, amounts per denom}`, total per denom, daftar **cabang pengisi** request (nama, vendor) — tanpa ATM.
- FR7.3 Isi **pihak replenish**: ATM miliknya (terminal, lokasi, tiket replenish, order per denom — seperti sekarang, **tanpa** branch
  vault per ATM) + **semua vault request** (V6): `{ticket_number, vault {code, address}, branch {code, name, vendor_name},
  amounts per denom}`.
- FR7.4 Alamat vault = lokasi `vendor_vaults.location_id`, fallback lokasi branch (U8). Tidak ditampilkan: saldo/kapasitas, tier,
  urgent, alasan buffer.
- FR7.5 Vendor vault menolak → request `vault_assignment`, rencana → `draft` (satu-satunya plan); resend hanya pihak yang isinya berubah
  (aturan 2.2b tetap).

### FR8 — Audit, RBAC, notifikasi
- FR8.1 Audit dalam tx: simpan (`vault_allocations_saved`, baris + `buffer_reason`), submit (+ tiket), approve/reject, area-region.
- FR8.2 RBAC route tetap (`/api/v1/vault-plans`: ACM-USER, ACM-SPV, ADMIN read-only; keanggotaan area dicek di service, non-anggota → 404).
- FR8.3 Notifikasi per event 2.2a tetap; penerima "ACM area terkait" = area rencana (satu area).

### FR9 — Frontend
- FR9.1 CompanyPortal **Penetapan Vault**: header (request, region, area, tanggal), tabel ATM + order (read-only), **total order per
  denom**; editor alokasi: tambah baris → pilih **branch vendor** → **vault** (tier, saldo/sisa kapasitas), isi nominal per denom
  (input rupiah, kelipatan pecahan), toggle urgent + alasan; ringkasan per denom **order vs alokasi vs selisih** (kurang = merah +
  teks, lebih = "buffer"); kolom alasan buffer muncul bila ada lebih; peringatan kapasitas per baris. Submit dinonaktifkan + alasan
  bila ada kekurangan. Nomor tiket per baris setelah submit.
- FR9.2 Detail Vendor Request (ATM-SPV review): rencana + baris alokasi + tiket + buffer.
- FR9.3 **Pengaturan → Area ACM**: tab **Region** (tambah/hapus kode region), panel peringatan FR1.4; tab Cabang disembunyikan.
- FR9.4 VendorPortal **Orders**: pihak vault — tiket + nominal per vault; pihak replenish — daftar vault request (tiket, alamat, nominal).
- FR9.5 Uang `tabular-nums`, rata kanan, IDR eksplisit; status tidak hanya warna.

## Non-functional
- NFR1 Kandidat + kapasitas set-based (satu query saldo per vault, satu query alokasi per tanggal); detail ≤ 3 s p95.
- NFR2 Tulis primary; daftar/kandidat replica; read-after-write primary.
- NFR3 Uang `bigint` IDR di DB/Go, string desimal di JSON (QC F7); tidak ada float.
- NFR4 Transisi `SELECT … FOR UPDATE` pada request + plan (pola 2.2a); penerbitan tiket aman paralel (advisory lock).
- NFR5 Coverage ≥ 80% kode baru; integration test Postgres nyata.

## API (flat JSON) — perubahan
| Method | Path | Isi |
|---|---|---|
| GET | `/api/v1/vault-plans/{id}` | + `region_code`, `order_totals`, `allocations[] {id, vendor_vault_id, vault_code, vault_category, branch {..}, tier, is_urgent, urgent_reason, amounts, saldo_snapshot, capacity_snapshot, capacity_warning, ticket_number}`, `buffer_reason`, `vault_tickets[]` (aktif + nonaktif). Field penetapan per ATM dihapus. |
| GET | `/api/v1/vault-plans/{id}/candidates?urgent=` | branch[] → `vaults[] {vendor_vault_id, vault_code, category, tier, saldo_known, saldo, capacity}` (tanpa `terminal_id`) |
| PUT | `/api/v1/vault-plans/{id}/allocations` | body `{allocations: [{vendor_vault_id, amounts: {"100000": "500000000"}, is_urgent, urgent_reason}], buffer_reason}`; **menggantikan** `PUT …/assignments` |
| POST | `/api/v1/vault-plans/{id}/submit` · `/approve` · `/reject` | tetap |
| GET | `/api/v1/vendor-requests/{id}/vault-assignments` | → rencana + alokasi + tiket (bentuk sama dengan detail plan) |
| GET/PUT | `/api/v1/admin/acm-areas/{id}/regions` | baru (ADMIN); `…/branches` tidak dipakai UI lagi |
| GET | `/api/v1/admin/acm-areas/warnings` | isi diganti FR1.4 |
| GET | `/api/v1/vendor/replenish-orders[/{id}]` | `content` bentuk baru FR7.2/FR7.3 |

## Data model — migrasi `030` v3 (menggantikan 030 v1 yang belum di-commit; lihat plan untuk revert di dev)
- `acm_area_regions` (`id`, `acm_area_id` FK, `region_code text` CHECK `^[A-Z0-9]{2,10}$`, `created_by` FK users, `created_at`);
  `UNIQUE (region_code)`. Link table murni (hapus baris = lepas region; pola `acm_area_branches`).
- `vendor_request_vault_plans`: + `buffer_reason text` CHECK 10–500; unik parsial `(vendor_request_id) WHERE status <> 'cancelled'`.
- `vendor_vaults`: `UNIQUE (id, vendor_branch_id)` (target FK komposit).
- `vendor_request_vault_allocations` (`id`, `vault_plan_id` FK, `vendor_request_id` FK, `vendor_vault_id`, `vault_branch_id`,
  FK komposit → `vendor_vaults (id, vendor_branch_id)`, `tier smallint` 1–3, `is_urgent`, `urgent_reason` + CHECK seperti
  `vendor_request_vault_assignments`, `saldo_snapshot`/`capacity_snapshot jsonb`, `capacity_warning`, timestamps);
  `UNIQUE (vault_plan_id, vendor_vault_id)`.
- `vendor_request_vault_allocation_amounts` (`allocation_id` FK ON DELETE CASCADE, `denom integer` CHECK ∈ 7 pecahan, `amount bigint`
  CHECK `amount > 0 AND amount % denom = 0`); PK `(allocation_id, denom)`.
- `vendor_request_vault_tickets` (`id`, `vendor_request_id` FK, `vendor_vault_id` FK, `replenish_date date`, `seq smallint` 1–999,
  `ticket_number text`, `is_active`, `created_at`, `deactivated_at`); `UNIQUE (ticket_number)`, `UNIQUE (vendor_vault_id,
  replenish_date, seq)`, `UNIQUE (vendor_request_id, vendor_vault_id)`, CHECK `is_active OR deactivated_at IS NOT NULL`.
- `vendor_request_vault_assignments` (per ATM, 027): **tidak ditulis lagi**, tidak dihapus (0 baris di dev; fitur belum produksi).
- `no_hard_delete_test`: + tiket. Alokasi = replace-all saat `draft` (hard delete terdokumentasi, sama dengan assignments 2.2a).

## Out of scope
- ATM on-prem (V11); pemetaan ATM/cabang pengisi → vault (V1, V6); drop `acm_area_branches` / `vendor_request_vault_assignments`.
- `dsr_location_vault_maps`, ETL DSR, pindah buku/escrow, alur cash pickup FSD.
- Data lama per ATM: tidak dimigrasi, tidak ditampilkan (0 baris di dev).

## Open questions
- Tidak ada. (Request dev tanpa `region_code` akan tampil di peringatan FR1.4 — perlu dibuat ulang atau diberi region oleh user.)
