# Design Document

> Status: accepted 2026-11-19 (engineer) — termasuk drop fungsi lama di migrasi `045` dan `DEFAULT 270` ("Setuju, lanjut tasks").

## Overview

Spec ini memindahkan pengelolaan PKS ke satu layar **Daftar PKS** (`/settings/admin/pks`), menambah ambang Need Renewal per PKS dalam hari (`vendor_pks.renewal_days`), menambah dua label status hitung (`belum_berlaku`, `diganti`), meringkas daftar vendor menjadi satu kolom PKS berupa link, menjadikan tab PKS detail vendor read-only, dan membuka akses baca untuk `ATM-SPV`. Tabel `vendor_pks_types` dan layar Type PKS tidak berubah (D2).

Fakta yang dicek saat menulis design (2026-11-19, DB dev):
- `vendor_pks`: **0 baris**; change `vendor_pks` pending: **0**. Backfill `renewal_days` (D8) dan guard migrasi tidak menyentuh data nyata di dev.
- `ATM-SPV` punya menu `dashboard, cash-flow, monitoring, forecasting, replenish`, **tanpa** `settings`. Menu feature `settings.*` yang ada: `settings`, `settings.roles(.create/.edit)`, `settings.acm-areas`.
- Pemakai `vendor_pks_validity_months()`: `backend/queries/vendor_pks.sql` (`ListVendorPksByVendor`) dan `vendor_pks_report.sql` (`ExportVendorPksBatch`). Pemakai `vendor_pks_status()`: `vendors_admin.sql`, `vendor_pks.sql`, `vendor_pks_report.sql` (backend + salinan sqlc di `backend-exq`).
- Pola pagination daftar admin: `LIMIT/OFFSET` + query `Count*` (`vendors_admin.sql`).
- Endpoint PKS sudah ada di `backend/api/openapi.yaml` (`/admin/vendors/{vendorID}/pks*`), jadi endpoint baru wajib masuk spec + contract test.

## Architecture

```mermaid
flowchart LR
    subgraph CP[CompanyPortal-Vite]
      PL[PksListPage<br/>features/admin-pks]
      PT[PksTable<br/>shared]
      VT[VendorsTable<br/>kolom PKS link]
      VD[VendorDetailPage<br/>tab PKS read-only]
    end
    subgraph BE[backend :8080]
      PH[AdminPksHandler<br/>/api/v1/admin/pks]
      VH[AdminVendorHandler<br/>/api/v1/admin/vendors]
      VPH[AdminVendorPksHandler<br/>/admin/vendors/id/pks writes]
      PLS[PksListService]
      PRS[UpdateReminder]
      VPS[VendorPksAdminService<br/>Submit create/terminate]
      AP[VendorPksApplier]
    end
    subgraph EXQ[backend-exq :8081]
      EX[vendor_pks producer]
    end
    subgraph DB[(PostgreSQL)]
      VP[vendor_pks<br/>+ renewal_days]
      FN[vendor_pks_record_status]
      AL[audit_logs]
      MC[master_data_change_requests]
    end
    PL --> PT
    VD --> PT
    PT -->|GET list| PH --> PLS -->|replica| VP
    PL -->|PUT reminder| PH --> PRS -->|primary tx| VP
    PRS --> AL
    PL -->|POST create/terminate| VPH --> VPS --> MC
    MC -->|approve| AP --> VP
    VT -->|GET list pks_count| VH
    PL -->|POST export| EX -->|replica| VP
    PLS -.-> FN
    EX -.-> FN
```

Alur data:
- **Baca** (Daftar PKS, tab PKS vendor, daftar vendor, ekspor): replica; status dihitung di SQL oleh satu fungsi `vendor_pks_record_status()` dengan `today` dari Go (Req 3.3, 3.4).
- **Tulis PKS** (catat, ganti, terminasi, batalkan): endpoint dan maker-checker yang sudah ada, tidak berubah kecuali field `renewal_days` di create (Req 1.2, 1.3, 11.2).
- **Ubah reminder**: endpoint baru, primary, satu tx `UPDATE vendor_pks` + `INSERT audit_logs` (Req 2, deviasi GR#3).

## Components and Interfaces

### Backend (`backend/`)

#### Migrasi `044_vendor_pks_list.sql` (Req 1.1, 3.1, 3.4, 9.1)

Lihat Data Models.

#### `queries/vendor_pks_list.sql` (baru) — Req 4, 7.1, 8.1

- `ListPksAdmin :many` (replica): `vendor_pks p JOIN vendors v JOIN vendor_pks_types t`, kolom Req 4.1, `status = vendor_pks_record_status(p.replaced_at, p.terminated_at, p.effective_date, p.expiry_date, p.renewal_days, @today)`, `remaining_days = p.expiry_date - @today`. Filter:
  - status: `terkini` → `p.replaced_at IS NULL`; `semua` → semua; nilai lain → `status = @status`.
  - `pks_type` → `t.code = @pks_type`; `vendor` → `v.code = @vendor`; `q` → `p.pks_name ILIKE '%'||@q||'%' OR p.wod_number ILIKE '%'||@q||'%'` (A4).
  - `v.kind = 'FLM_VENDOR'` selalu (Req 4.5).
  - `ORDER BY v.name, t.code, p.effective_date DESC, p.id DESC LIMIT/OFFSET` (A3).
- `CountPksAdmin :one`: filter sama, tanpa LIMIT (Req 4.6).
- `ListPendingPksChanges :many` (replica): change `vendor_pks` pending (op create/disable/enable) dengan `payload`, `vendor_id`, `pks_type_id`, filter `vendor`/`pks_type` yang sama. Dipakai untuk (a) baris `menunggu_persetujuan` dari op create (Req 4.2) dan (b) penanda pending per (vendor, type) (Req 4.1, 5.6, 6.6).
- `GetVendorPksForUpdate :one` (primary): `SELECT … FROM vendor_pks WHERE id = $1 FOR UPDATE` (Req 2.1, 2.6).
- `UpdateVendorPksRenewalDays :one` (primary): `UPDATE vendor_pks SET renewal_days = $2 WHERE id = $1 RETURNING …` (Req 2.1, 2.7).

#### `queries/vendor_pks.sql` (ubah)

- `InsertVendorPks`: tambah `renewal_days` (Req 1.3).
- `ListVendorPksByVendor`, `GetVendorPksStatus`: dihapus (pemakainya, `GET /admin/vendors/{id}/pks`, dihapus; lihat AdminVendorPksHandler).

#### `queries/vendors_admin.sql` (ubah) — Req 7.1, 7.2

- `ListVendorsAdmin` / `CountVendorsAdmin`: hapus join PKS aktif + filter `pks_type`/`pks_status`; tambah `pks_count = (SELECT count(*) FROM vendor_pks p WHERE p.vendor_id = v.id AND p.replaced_at IS NULL)` (0 untuk INTERNAL).
- `ListVendorPksSummaries`: dihapus.

#### `internal/service/pks_list.go` — `PksListService` (baru) — Req 4

```go
type PksListFilter struct { Status, PksType, Vendor, Q string; Page, PageSize int }
func (s *PksListService) List(ctx context.Context, role string, f PksListFilter) (*PksListPage, error)
```
- Role ∉ `reports.PksViewerRoles` (`ADMIN`, `ADMIN_PARAM`, `ATM-SPV`) → `ErrForbidden` (Req 4.8).
- Filter divalidasi lewat `reports.ParsePksListFilter` (satu validator untuk list dan ekspor, Property 6); `pks_type` harus kode yang ada di `vendor_pks_types` (aktif atau nonaktif, karena riwayat bisa memakai type nonaktif), `vendor` harus kode vendor yang ada; selain itu 422 (Req 4.4).
- `today = reports.TodayWIB(s.now())` (Req 3.3).
- Baris dari `ListPksAdmin`; bila status `terkini`/`semua`, baris pending create ditambahkan (tanpa id, `status = menunggu_persetujuan`) (Req 4.2). Baris pending tidak ikut pagination DB: total = `CountPksAdmin` + jumlah pending create yang lolos filter, dan baris pending tampil di halaman 1 di atas baris PKS. Jumlahnya kecil karena dibatasi satu per (vendor, type) oleh index yang ada.
- Tiap baris PKS diberi `pending` = op change pending untuk (vendor, type)-nya bila ada (Req 4.1).
- Repo memakai pool replica (Req 4.7).

#### `internal/service/vendor_pks_admin.go` — `VendorPksAdminService` (ubah) — Req 1, 2, 6.7

- `SubmitVendorPksRequest` + `RenewalDays *int json:"renewal_days"`; `VendorPksCreatePayload` + `RenewalDays int json:"renewal_days"`.
- `validateVendorPksCreate`: `renewal_days` wajib, 30–3666, ≤ `expiry − effective` (hari); kalau tidak, 422 field `renewal_days` (Req 1.2). Satu fungsi `validateRenewalDays(days int, eff, exp time.Time) error` dipakai juga oleh reminder (Req 2.3, Property 2).
- Field dan parameter constructor `renewalMonths` dihapus (Req 1.5). Method `Get` dihapus.
- **Baru** `UpdateReminder(ctx, actorID int64, role string, pksID int64, days int, ip string) (VendorPksRecord, error)` (Req 2):
  1. Role ∉ {ADMIN, ADMIN_PARAM} → `ErrForbidden` (Req 2.5, service layer).
  2. Tx primary: `GetVendorPksForUpdate`; tidak ada → `ErrNotFound` (404, Req 2.6).
  3. `validateRenewalDays(days, row.EffectiveDate, row.ExpiryDate)` → 422 (Req 2.3). Berlaku juga untuk Replaced_PKS dan terminated (Req 2.2).
  4. Nilai sama dengan sekarang → kembalikan row tanpa UPDATE dan tanpa audit (no-op).
  5. `UpdateVendorPksRenewalDays`, lalu `audit.Write` di tx yang sama: action `vendor_pks_reminder_updated`, entity `vendor_pks`/id, before `{renewal_days}`, after `{renewal_days}`, ip. Error audit → rollback (Req 2.4).
- `ErrForbidden` di Submit* tetap (Req 6.7).

#### `internal/service/masterdata_applier_vendor_pks.go` (ubah) — Req 1.3

- `InsertVendorPks` menerima `payload.RenewalDays`. Payload tanpa `renewal_days` (change yang dibuat sebelum rilis) → 270. Dev: 0 pending, tetapi tetap ditangani agar aman.

#### `internal/service/vendor_admin.go` (ubah) — Req 7, 9.2

- List: hapus filter PKS dan `pks` per type; kembalikan `pks_count`. Hapus `pksToday` dan `renewalMonths`.
- Role check service layer: `List` dan `Get` menerima `ADMIN`, `ADMIN_PARAM`, `ATM-SPV`; method tulis tetap `ADMIN`/`ADMIN_PARAM` (Req 9.2, 9.4).

#### `internal/handler/admin_pks_handler.go` — `AdminPksHandler` (baru) — Req 2, 4

| Method | Path | Guard route | Respons |
|---|---|---|---|
| GET | `/api/v1/admin/pks?status=&pks_type=&vendor=&q=&page=&page_size=` | ADMIN, ADMIN_PARAM, ATM-SPV | `{items:[…], total, page, page_size}` |
| PUT | `/api/v1/admin/pks/{pksID}/reminder` body `{renewal_days}` | ADMIN, ADMIN_PARAM | 200 `{pks}` (bukan 202, Req 6.5) |

Item: `{id|null, vendor_id, vendor_code, vendor_name, pks_type, wod_number, pks_name, effective_date, expiry_date, renewal_days, remaining_days, status, pending: {op, change_id}|null}`. Flat JSON seperti handler tetangga (CLAUDE.md Sec 5).

#### `internal/handler/admin_vendor_pks_handler.go` (ubah) — Req 1.2, 6

- `GET /` dihapus (tab PKS memakai `GET /admin/pks?vendor=`). `POST /`, `/terminate`, `/unterminate` tetap; `POST /` menerima `renewal_days`.

#### `internal/handler/admin_vendor_handler.go` + `cmd/api/main.go` (ubah) — Req 9.2

- `AdminVendorHandler` dipindah dari grup `masterDataAdmin` ke grup baru `vendorRead` = `RequireAuth + RequireRoles("ADMIN","ADMIN_PARAM","ATM-SPV")`. Di `Routes()`, `GET /` dan `GET /{id}` tanpa guard tambahan; `POST /`, `PUT /{id}`, `POST /{id}/disable|enable` dibungkus `r.With(RequireRoles("ADMIN","ADMIN_PARAM"))`.
- Mount sub-resource vendor lain (`/{vendorID}/vaults`, `/pics`, `/packages`, `/dsr-location-maps`, `/pks`, cabang, dll.) **tetap** di `masterDataAdmin` (Req 9.2).
- `AdminPksHandler` di-mount di `vendorRead` pada `/api/v1/admin/pks`.
- Pembacaan env `VENDOR_PKS_RENEWAL_MONTHS` dihapus (Req 1.5).

#### `api/openapi.yaml` (ubah)

- Tambah `/admin/pks` (GET) dan `/admin/pks/{pksID}/reminder` (PUT); `renewal_days` di body create; hapus GET `/admin/vendors/{vendorID}/pks`; skema daftar vendor `pks` → `pks_count`. Contract test `-run OpenAPI` + `gen:api`.

### Shared (`pkg/reports/vendor_pks.go`) — Req 3, 4.4, 10

- Hapus `VendorPksRenewalMonthsDefault`, `ParseVendorPksRenewalMonths`, `ParsePksFilter`, dan `belum_diisi`.
- `PksViewerRoles = []string{"ADMIN","ADMIN_PARAM","ATM-SPV"}`, dipakai route + service di backend dan backend-exq (menggantikan `VendorPksViewerRoles`).
- `PksStatusLabels`: `aktif` Aktif, `need_renewal` Need Renewal, `belum_berlaku` Belum Berlaku, `expired` Expired, `terminated` Terminated, `diganti` Diganti.
- `PksListStatusFilters`: `terkini`, `semua` + kunci `PksStatusLabels`.
- `ParsePksListFilter(status, pksType, vendor, q string) (PksListFilter, error)`: trim; status kosong → `terkini`; status di luar whitelist → error; `pks_type`/`vendor` upper-case; `q` maksimal 100 karakter.
- `RenewalDaysMin = 30`, `RenewalDaysMax = 3666`, `RenewalDaysDefault = 270`.
- `VendorPksHeader` baru (Req 10.3): … `Tanggal Expired PKS`, `Masa Berlaku (hari)`, `Reminder (hari)`, `Status PKS`.

### `backend-exq` — Req 10

- `queries/vendor_pks_report.sql`: `ExportVendorPksBatch` / `CountVendorPksReport` ditulis ulang di atas semua baris `vendor_pks`, dengan filter yang sama dengan `ListPksAdmin` (tanpa baris pending, A2) dan `vendor_pks_record_status()`. Keyset: `ORDER BY v.code, t.code, (DATE '9999-12-31' - p.effective_date), p.id`, dengan pembanding tuple yang sama (efektif terbaru dulu tanpa arah urutan campuran). `ExportVendorPksMissingBatch` / `CountVendorPksMissing` dihapus (A1).
- `internal/exportjob/params.go`: `report_params` kind `vendor_pks` = `{status, pks_type, vendor, q}` lewat `reports.ParsePksListFilter`; filter lama (`q` vendor, `status` vendor, `pks_status`) ditolak 422 (Req 10.2).
- `producers_report.go`: hapus `renewalMonths`; `today` dari `reports.TodayWIB(asOf)`.
- Guard route + service: `reports.PksViewerRoles` (Req 10.5). Audit ekspor dan pembacaan replica tidak berubah.
- `cmd/api/main.go`: hapus env `VENDOR_PKS_RENEWAL_MONTHS`.

### Frontend (`frontend/CompanyPortal-Vite/`)

#### `src/features/admin-pks/` (baru) — Req 5, 6, 8

- `api.ts` / `hooks.ts`: `usePksList(filter)`, `useUpdatePksReminder()`; mutasi create/terminate/unterminate dipindah dari `admin-vendors/pksApi.ts`.
- `components/PksTable.tsx` (shared, Req 5.2, 8.1): kolom Status, Type, Nama Vendor, Judul PKS, No. WOD, Tgl Efektif, Tgl Masa Akhir, Masa Berlaku, Reminder (hari). Prop `vendorCode?` (mengunci filter vendor; kolom Nama Vendor disembunyikan di tab vendor) dan `actions?: boolean`. Baris dapat diaktifkan (klik/Enter) → `/settings/admin/vendors/{vendor_id}?tab=pks` (Req 5.5), kecuali saat dipakai di tab vendor.
- `components/PksFilterBar.tsx` (Req 5.3): Status (default Terkini), Type, Vendor, cari; disimpan di URL search params.
- `components/PksListPage.tsx`: judul, filter, tabel, tombol "Catat PKS baru" dan "Ekspor" (Req 6.1, 10.4).
- `components/PksFormDialog.tsx` (pindahan + ubah, Req 6.2, 6.3): field Vendor (select vendor aktif FLM_VENDOR), Type, No. WOD, Judul, Tgl Efektif, Tgl Masa Akhir, Reminder (hari, default 270, wajib). Mode Ganti: Vendor + Type terkunci.
- `components/PksReasonDialog.tsx` (pindahan, Req 6.4).
- `components/PksReminderDialog.tsx` (baru, Req 6.5): angka 30–3666 + validasi ≤ masa PKS di klien (server tetap memvalidasi).
- `components/PksStatusBadge.tsx` (pindahan + `belum_berlaku`, `diganti`, `menunggu_persetujuan`; ikon + teks, Req 5.4).
- `lib/pks.ts`: `formatMasaBerlaku(status, remainingDays)` → "N hari" / "lewat N hari" / "-" (Req 3.5).
- Aksi per baris (Req 6.1, 6.4–6.6): Ganti + Terminasi/Batalkan hanya di Current_PKS; Ubah reminder di semua baris PKS; tombol nonaktif + teks alasan bila `pending` ada; semua kontrol tulis tidak dirender untuk ATM-SPV.

#### Route — Req 5.1, 9.3

- `src/routes/settings/admin/pks.tsx` (baru): `/settings/admin/pks`, `requireRoles(["ATM-SPV"])` (ADMIN/ADMIN_PARAM lolos lewat bypass yang ada), `validateSearch` untuk filter.
- `routes/settings/admin/vendors.tsx` dan `vendor-detail.tsx`: `requireRoles` ditambah `ATM-SPV`; `vendor-detail` mendapat search param `tab`.
- `routeTree.ts`: daftarkan route baru.

#### `SettingsHubPage.tsx` — Req 5.1, 9.3

- Kartu baru "Daftar PKS" (`/settings/admin/pks`, kategori master).
- Untuk ATM-SPV hanya kartu "Daftar PKS" dan "Manajemen Vendor" yang tampil.

#### `features/admin-vendors/` (ubah) — Req 7, 8, 9.3, 10.4

- `VendorsTable.tsx`: hapus grup kolom per type; kolom "PKS" berisi "N PKS" / "Belum ada" sebagai link ke `?tab=pks`, INTERNAL "-" (Req 7.3, 7.4).
- `VendorFilterBar.tsx`: hapus filter type + status PKS. `AdminVendorsPage.tsx`: hapus tombol ekspor PKS; untuk ATM-SPV sembunyikan Tambah/Impor/Ekspor/aksi tulis (Req 9.3).
- `VendorDetailPage.tsx`: tab dibaca dari `?tab=` (default `info`; ATM-SPV dipaksa `pks`). Untuk ATM-SPV hanya tab PKS yang tampil, dan `SummaryStrip` serta kontrol tulis di `VendorHeader` tidak dirender, karena endpoint sumbernya tertutup untuk ATM-SPV (Req 9.3).
- `PksPanel.tsx` diganti `VendorPksTab.tsx`: `PksTable vendorCode=… actions={false}` + link "Kelola di Daftar PKS" hanya untuk ADMIN/ADMIN_PARAM; teks "Need Renewal mulai 9 bulan…" hilang (Req 8).
- Hapus `pksApi.ts`, `PksFormDialog.tsx`, `PksReasonDialog.tsx`, `PksStatusBadge.tsx`, `lib/pks.ts` dari `admin-vendors` (dipindah ke `admin-pks`); field `pks` di `types.ts` → `pks_count`.
- `src/lib/api/schema.d.ts`: regenerasi `gen:api`.

## Data Models

Tidak ada tabel baru. Migrasi `044_vendor_pks_list.sql` (satu tx):

1. **Guard**: berhenti bila ada `vendor_pks` dengan `expiry_date - effective_date < 30` (tidak bisa memenuhi CHECK minimum), dengan pesan berisi id-nya. Dev: 0 baris.
2. **Kolom**:
   ```sql
   ALTER TABLE vendor_pks ADD COLUMN renewal_days integer NOT NULL DEFAULT 270;
   UPDATE vendor_pks SET renewal_days = LEAST(270, expiry_date - effective_date);  -- D8; LEAST hanya menjaga CHECK bila ada PKS < 270 hari
   ALTER TABLE vendor_pks
     ADD CONSTRAINT vendor_pks_renewal_days_chk CHECK (renewal_days BETWEEN 30 AND 3666),
     ADD CONSTRAINT vendor_pks_renewal_days_le_term_chk CHECK (renewal_days <= expiry_date - effective_date);
   ```
   `DEFAULT 270` dipertahankan supaya binary lama yang masih berjalan antara migrasi dan deploy tetap bisa INSERT; API selalu mengirim nilai eksplisit (Req 1.2). Catatan untuk PO: D8 = 270; `LEAST` hanya berbeda dari 270 untuk PKS yang masanya kurang dari 270 hari (tidak ada di dev).
3. **Fungsi status baru** (Req 3.1, 3.2):
   ```sql
   CREATE FUNCTION vendor_pks_record_status(p_replaced_at timestamptz, p_terminated_at timestamptz,
       p_effective date, p_expiry date, p_renewal_days int, p_today date) RETURNS text
   LANGUAGE sql IMMUTABLE AS $$
     SELECT CASE
       WHEN p_replaced_at   IS NOT NULL THEN 'diganti'
       WHEN p_terminated_at IS NOT NULL THEN 'terminated'
       WHEN p_effective > p_today        THEN 'belum_berlaku'
       WHEN p_expiry    < p_today        THEN 'expired'
       WHEN p_expiry    < p_today + p_renewal_days THEN 'need_renewal'
       ELSE 'aktif' END $$;
   ```
   Fungsi diberi nama baru (bukan `CREATE OR REPLACE vendor_pks_status`) karena argumennya berbeda dan binary lama masih memanggil fungsi lama selama jendela deploy.
4. **Menu** (Req 9.1): `menu_features` `settings.pks` "Daftar PKS" (parent `settings`); `role_permissions`: `ADMIN`, `ADMIN_PARAM` → `settings.pks`; `ATM-SPV` → `settings` + `settings.pks`. `ON CONFLICT DO NOTHING`, `granted_by` = user ADMIN pertama (pola `027`).
5. `COMMENT ON COLUMN vendor_pks.renewal_days` dan `COMMENT ON TABLE vendor_pks` diperbarui (kolom ini satu-satunya kolom isi yang boleh di-UPDATE, Req 11.3).

**Penghapusan `vendor_pks_status()` dan `vendor_pks_validity_months()`** (Req 3.6): setelah rilis, keduanya tidak punya pemanggil. Keduanya tidak di-drop di `044`, karena binary lama masih memanggilnya sampai deploy selesai; drop dilakukan oleh migrasi pembersih berikutnya (`045`) setelah rilis ini berjalan. Ini penyesuaian urutan dari Req 3.6 (kondisi "tidak ada pemanggil" baru terpenuhi setelah deploy) dan perlu persetujuan engineer di gate design.

Rollback dev: `backend/scripts/dev/rollback_044_vendor_pks_list.sql` (drop constraint/kolom, fungsi baru, menu + permission baru), aman selama tidak ada data yang bergantung.

## Correctness Properties

### Property 1: Urutan status
*For any* PKS_Record (replaced_at, terminated_at, effective, expiry, renewal_days) and any Today_WIB, `vendor_pks_record_status` returns exactly the first matching label of the order diganti → terminated → belum_berlaku → expired → need_renewal → aktif, and a PKS with `expiry = today` is never `expired`.
**Validates: Requirements 3.1, 3.2**

### Property 2: Validasi reminder
*For any* `renewal_days` and dates, create submit, reminder update, and the DB CHECK all accept the value iff `30 ≤ renewal_days ≤ 3666` and `renewal_days ≤ expiry − effective`.
**Validates: Requirements 1.1, 1.2, 2.3**

### Property 3: Ubah reminder hanya mengubah satu kolom, dengan audit atomik
*For any* PKS_Record and valid new value, after `UpdateReminder` every column except `renewal_days` (and `updated_at`) is unchanged and exactly one `audit_logs` row records before/after; *for any* audit failure, the PKS_Record is unchanged.
**Validates: Requirements 2.1, 2.2, 2.4, 2.7, 11.3**

### Property 4: Filter Daftar PKS lengkap dan tepat
*For any* data set and filter combination, every returned PKS row satisfies every filter, every PKS_Record that satisfies them is returned across pages exactly once, and `total` equals their count (plus matching pending creates for terkini/semua).
**Validates: Requirements 4.3, 4.4, 4.6**

### Property 5: Arti filter Status
*For any* data set, `terkini` returns exactly the Current_PKS plus pending creates, `diganti` exactly the Replaced_PKS, and `semua` every PKS_Record plus pending creates; Vendor_Internal never appears.
**Validates: Requirements 4.2, 4.3, 4.5**

### Property 6: Layar dan ekspor sepakat
*For any* filter and Today_WIB, the set of (PKS id, status, remaining days) returned by the PksListAPI (without pending rows) equals the set exported by the ExportAPI.
**Validates: Requirements 3.4, 10.1**

### Property 7: Matriks RBAC
*For any* role and endpoint in scope, the response is allowed iff the role is in that endpoint's set: PKS list/export and vendor list/detail = PKS_Viewer_Role; PKS writes, reminder, and every other vendor endpoint = Master_Data_Role; else 403 at route and service.
**Validates: Requirements 2.5, 4.8, 6.7, 9.2, 9.4, 10.5**

### Property 8: Jumlah PKS di daftar vendor
*For any* vendor, `pks_count` equals its number of Current_PKS (0 for Vendor_Internal).
**Validates: Requirements 7.1, 7.4**

### Property 9: Masa Berlaku
*For any* PKS_Record and Today_WIB, `remaining_days = expiry − today` and the label is "N hari" for N ≥ 0, "lewat N hari" for N < 0, and "-" for diganti/terminated.
**Validates: Requirements 3.5**

## Error Handling

| Kasus | Respons |
|---|---|
| Filter tidak dikenal / `pks_type` atau `vendor` tidak ada | 422 `validation_error` + field |
| `renewal_days` hilang, bukan bilangan bulat, di luar 30–3666, atau > masa PKS | 422 field `renewal_days` |
| Reminder untuk PKS yang tidak ada | 404 `not_found` |
| Role tidak berhak | 403 (route + service) |
| Audit gagal saat ubah reminder | 500, tx di-rollback, nilai tidak berubah |
| Create/terminate: pending sudah ada, WOD terpakai, stale saat apply | tidak berubah (409/422 yang ada) |
| Ekspor dengan filter lama (`pks_status`, `status` vendor) | 422 |

Pesan UI tanpa em dash; pesan server tidak membocorkan detail SQL.

## Testing Strategy

- **Go unit (table-driven, mock repo)**: `ParsePksListFilter`, `validateRenewalDays` (Property 2), `PksListService` (role, filter, gabung pending; Property 5), `UpdateReminder` (role, 404, 422, no-op, audit gagal → rollback; Property 3), matriks role `VendorAdminService`.
- **Property test** (`pgregory.net/rapid`, sudah dipakai di `backend/`): fungsi status referensi di Go vs SQL `vendor_pks_record_status` pada tanggal acak (Property 1); `remaining_days` dan formatnya (Property 9).
- **Integration (`//go:build integration`, skip tanpa `DATABASE_URL`)**: migrasi `044` (CHECK, backfill, menu); `ListPksAdmin` vs `CountPksAdmin` pada data acak (Property 4); list API vs ekspor `backend-exq` (Property 6); `pks_count` (Property 8); reminder tx + audit (Property 3); matriks RBAC lewat router nyata (Property 7).
- **Contract**: `go test ./internal/handler -run OpenAPI`; `pnpm gen:api` + vitest stale check.
- **Frontend (vitest + RTL)**: PksTable (kolom, badge ikon+teks, keyboard), PksFilterBar (URL state, default Terkini), PksFormDialog (default 270, wajib, mode Ganti terkunci), PksReminderDialog, aksi tersembunyi untuk ATM-SPV, kolom PKS VendorsTable (N PKS / Belum ada / -), tab VendorDetailPage untuk ATM-SPV, kartu SettingsHubPage per role.
- **Gate**: `golangci-lint`, `go test ./...` di `backend`, `backend-exq`, `pkg`; `pnpm lint/test/build` CompanyPortal; coverage `internal/*` ≥ 80%.
- **Cek browser manual**: outstanding (user); AI tidak pernah menandainya selesai.

## Documented deviation

**Ubah reminder PKS (D9): deviasi Golden Rule #3**, pola sama dengan peran PIC vendor (`vendor-party-scope` D6e) dan Role Management. Perubahan berlaku langsung tanpa `master_data_change_requests`/`approval_requests`; `audit_logs` (`vendor_pks_reminder_updated`, before/after) ditulis dalam tx yang sama (audit gagal = rollback); guard `ADMIN`/`ADMIN_PARAM` di route dan service. Dasar: reminder adalah pengaturan pemantauan, bukan isi kontrak. Dicatat di CLAUDE.md Sec 12 saat build.

## Deploy

1. `go run ./cmd/migrate up` (044) di luar jam kantor.
2. Rilis bersamaan: `backend`, `backend-exq`, CompanyPortal (kontrak daftar vendor + ekspor berubah, A5).
3. Hapus `VENDOR_PKS_RENEWAL_MONTHS` dari env kedua backend.
4. Setelah rilis stabil: migrasi pembersih `045` men-drop `vendor_pks_status()` dan `vendor_pks_validity_months()`.

## Files touched

- `backend/migrations/044_vendor_pks_list.sql` (baru), `backend/scripts/dev/rollback_044_vendor_pks_list.sql` (baru)
- `backend/queries/vendor_pks_list.sql` (baru), `vendor_pks.sql`, `vendors_admin.sql`; `backend/internal/db/*` (sqlc)
- `backend/internal/service/pks_list.go` (baru), `vendor_pks_admin.go`, `masterdata_applier_vendor_pks.go`, `vendor_admin.go`
- `backend/internal/handler/admin_pks_handler.go` (baru), `admin_vendor_pks_handler.go`, `admin_vendor_handler.go`
- `backend/cmd/api/main.go`, `backend/api/openapi.yaml`, `backend/.env.example`
- `pkg/reports/vendor_pks.go`
- `backend-exq/queries/vendor_pks_report.sql`, `internal/repository/*` (sqlc), `internal/exportjob/params.go`, `producers.go`, `producers_report.go`, `cmd/api/main.go`, `.env.example`
- `frontend/CompanyPortal-Vite/src/features/admin-pks/` (baru), `features/admin-vendors/` (VendorsTable, VendorFilterBar, AdminVendorsPage, VendorDetailPage, VendorHeader, VendorPksTab baru, file PKS lama dihapus, types/api), `features/rbac-settings/components/SettingsHubPage.tsx`, `routes/settings/admin/pks.tsx` (baru), `vendors.tsx`, `vendor-detail.tsx`, `routeTree.ts`, `lib/api/schema.d.ts`
- Dokumentasi: CLAUDE.md Sec 3 + Sec 12, `.claude/docs/data-map.md`, `.claude/docs/decisions.md`
