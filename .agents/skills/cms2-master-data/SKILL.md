---
name: cms2-master-data
description: >-
  Checklist for adding or changing a CMS2 master-data entity behind the
  maker-checker engine (queries -> read-only repo -> service Submit -> Applier
  -> main.go registry -> handler 202 -> tests -> frontend). Use when touching
  vendors, vendor_branches, vendor_vaults, vendor_pics, vendor_packages_branch,
  vendor_package_prices, atms, atm_vendor_packages, dsr_location_vault_maps, or
  adding a new master-data table/screen.
---

# CMS2 Master-Data Maker-Checker

Aturan utama (`.agents/rules/GEMINI.md`): operasi create/update/disable/enable pada data master tidak pernah menulis langsung ke tabel entitas utama. Perubahan di-stage sebagai baris `master_data_change_requests` + `approval_requests`, endpoint mengembalikan HTTP **202 Accepted**, dan baris tabel utama hanya berubah ketika checker menyetujui — melalui `Applier` entitas tersebut di dalam satu transaksi database bersama pencatatan `audit_logs`.

**Pengecualian terdokumentasi** (jangan salin pola mereka untuk master-data standar): `admin/users`, Role Management, Area ACM admin, dan Holidays (berlaku langsung, namun wajib menulis `audit_logs` dalam transaksi yang sama).

**Sebelum memulai**: entitas atau kolom baru merupakan perubahan skema -> wajib melalui alur SDLC di `.agents/specs/<feature>/` (gunakan skill `cms2-sdlc`). Tanyakan kepada user terlebih dahulu.

## Implementasi Referensi

Contoh paling ringkas dan lengkap yang dapat dijadikan acuan pola: `dsr_location_map` (migrasi 027).

| Layer | File |
| :--- | :--- |
| SQL | `backend/queries/dsr_location_maps_admin.sql` |
| Repo (read-only) | `backend/internal/repository/dsr_location_map_admin_repository.go` |
| Service (validate + Submit) | `backend/internal/service/dsr_location_map_admin.go` |
| Applier | `backend/internal/service/masterdata_applier_dsr_location_map.go` |
| Registry + route | `backend/cmd/api/main.go` (`masterDataApplierRegistry`, `masterDataAdmin.Mount`) |
| Handler | `backend/internal/handler/admin_dsr_location_map_handler.go` |
| Tests | `*_test.go` di samping masing-masing file, + `masterdata_applier_dsr_location_map_integration_test.go` |

Engine inti (jangan ubah kecuali diminta): `masterdata_change.go` (`Submit`, `SubmitRequest`), `masterdata_applier.go` (`Applier`, `ApplierRegistry`), `masterdata_approval.go` (apply-on-approve, staleness check, audit).

## Checklist 9 Langkah

### 1. Queries — `backend/queries/<entity>_admin.sql`
- `List…Admin :many` dengan filter `status` = `active|disabled|all`.
- `Get…AdminByID :one` **termasuk baris yang disabled**. Query tunggal ini digunakan sebagai snapshot `Before` pada service sekaligus `CurrentState` pada applier — bentuknya harus identik agar update tidak selalu dianggap stale.
- Query pengecekan awal yang dibutuhkan service (keunikan, kepemilikan, FK masih aktif).
- Mutasi `Create… :one RETURNING`, `Update… :one RETURNING`, `Disable… :exec`, `Enable… :exec` — dipanggil **hanya** dari applier.
- Guard ulang FK/kepemilikan di dalam `INSERT`/`UPDATE` (`INSERT … SELECT … WHERE active`): state bisa berubah antara waktu submit dan approve; jika tidak ada baris -> error applier -> HTTP 409.
- Disable = `is_active=false, deleted_at=now()`. **Dilarang `DELETE FROM`**; tambahkan nama tabel ke dalam kedua daftar pada `backend/internal/repository/no_hard_delete_test.go`. Tabel effective-dated (`vendor_packages_branch`, `vendor_package_prices`) menutup `effective_end_date`.
- Jalankan `cd backend && sqlc generate` (wajib v1.31.1) -> perbaiki `UserLeafe` menjadi `UserLeave` di `backend/internal/db/` -> pastikan `git diff --stat internal/db/` hanya menampilkan file terkait.

### 2. Repository — Read-Only
- `New…Repository(dbConn, readConn db.DBTX)`: layar daftar menggunakan `readQueries` (replica); lookup saat submit menggunakan `queries` (primary — read-after-write).
- `pgx.ErrNoRows` -> `nil, nil`. Tidak boleh ada method mutasi di repository.

### 3. Service — Validasi, lalu `Submit`
- Gunakan interface kecil (`VendorVaultSubmitter`: cukup method `Submit`).
- Normalisasi input (`trim`), paksa ID dari path (`req.VendorID = vendorID`), kembalikan `&ValidationError{Field, Message}` (pesan bahasa Indonesia) untuk HTTP 422.
- `load()` memperlakukan ID milik parent lain sebagai **not found** (mencegah kebocoran lintas vendor).
- Sentinel errors: `Err<Entity>NotFound` / `Err<Entity>Conflict`.
- `Submit(ctx, makerID, SubmitRequest{EntityType, Op, EntityID, Payload, Before}, actorIP)`:
  - create -> `EntityID=nil`, tanpa `Before`
  - update/disable/enable -> `EntityID` + `Before` dari `Get…AdminByID`
  - `Submit` memeriksa ulang role `ADMIN`/`ADMIN_PARAM` dan menolak perubahan pending kedua pada entitas yang sama (`ErrMasterDataChangePending`).
- Nilai yang dihasilkan server (misalnya `vendor_package_prices.package_code`) dibuat saat **apply**, bukan diterima dari client.

### 4. Applier — Penulisan Mekanis Saja
- `type <Entity>Applier struct{}` dengan `Apply(ctx, tx, change)` berdasarkan `change.Op` dan `CurrentState(ctx, tx, id)` = `Get…AdminByID`.
- Unmarshal `change.Payload` ke tipe payload service; tidak perlu validasi bisnis ulang, tetapi petakan kegagalan guard database (`pgx.ErrNoRows`, unique violation) ke error yang jelas -> HTTP 409.
- Kembalikan `(entityID, after, err)`; disable/enable mengembalikan `map[string]bool{"is_active": …}`.

### 5. Registrasi — `backend/cmd/api/main.go`
- Daftarkan `"<entity_type>": service.<Entity>Applier{}` ke `masterDataApplierRegistry`.
- Mount handler pada grup `masterDataAdmin` (`RequireAuth` + `ADMIN`/`ADMIN_PARAM`).

### 6. Handler — Flat JSON, HTTP 202
- Routes: `GET /`, `POST /`, `PUT /{id}`, `POST /{id}/disable`, `POST /{id}/enable`.
- Sukses -> `writeJSON(w, http.StatusAccepted, changeRequestAcceptedResponse(change))`.
- Pemetaan error: `ValidationError` -> 422, NotFound -> 404, Conflict / `ErrMasterDataChangePending` -> 409, `ErrMasterDataForbidden` -> 403, lainnya -> `writeUnexpectedError`. Client IP hanya melalui `extractClientIP(r)`.
- Jangan gunakan envelope `pkg/response` di ATM `backend/`.

### 7. Pengujian (Tests)
- Unit test service (mock repo + submitter): validasi, not-found lintas parent, conflict, `Before` terisi saat update/disable/enable dan `nil` saat create.
- Unit test handler: respons 202, pemetaan setiap error ke HTTP status code, tanpa auth -> 401.
- Integration test (`//go:build integration`, skip jika `DATABASE_URL` kosong): siklus penuh melewati `approveChange` (create/update/disable/enable), stale `Before` -> tidak diterapkan, guard saat apply -> error.
- Perbarui daftar tabel di `no_hard_delete_test.go`.
- Jalankan: `cd backend && go test ./...` dan `golangci-lint run --build-tags integration ./...`.

### 8. Frontend — `frontend/CompanyPortal-Vite`
- Tambahkan `entity_type` ke `PendingEntityType` di `src/features/master-data/pending.ts` serta label di `ENTITY_LABELS` pada `src/features/approvals/lib/changeDiff.ts`.
- Gunakan `usePendingEntityIds` / `usePendingCreates` untuk menandai baris pending dan tampilkan `pendingApprovalMessage()` setelah menerima HTTP 202.
- Verifikasi browser manual adalah tugas user — catat sebagai `outstanding (user)`.

### 9. Opsional (Tanyakan Terlebih Dahulu)
- Impor/ekspor CSV (`masterdata_import*.go`, `masterdata_export.go`).
- Perbarui dokumentasi data-map dan `.agents/development-progress.md`.
