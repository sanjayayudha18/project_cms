# Implementation Plan

> Status: accepted 2026-11-17 (engineer) — "Accept, run Wave 0"

## Overview

Urutan: komponen `BackLink` dan prop `ReportExportNotice` dulu, query sqlc `kind` dan seed vendor paralel; lalu pemasangan BackLink, handler, URL state, dan seed PIC; lalu test halaman dan dropdown; terakhir pemakaian prop di halaman Vendor, gate, dokumentasi, dan cek browser oleh user. Tanpa migrasi, tanpa perubahan `backend-exq`.

`AdminVendorsPage.tsx` disentuh task 2.1, 4.2, dan 5.2 → diserialkan ke wave berbeda.

Pola yang diikuti:
- Filter list `pks_status` di `admin_vendor_handler.go` (validasi → pointer → List + Count) untuk `kind`.
- `ADMIN_VENDORS_SEARCH_SCHEMA` + `useAdminVendorsUrlState.test.ts` untuk field URL baru.
- Header dan idempotensi `backend/scripts/dev/seed_vendor_users.sql` untuk seed.
- Gaya tombol ghost `components/ui/Button.tsx` (min 44×44 px) untuk `BackLink`.

### Model Split

**Sonnet-runnable**
- 1. Komponen `BackLink` + test: Sonnet, Low
- 2. Pasang BackLink di 12 halaman (2.1–2.3): Sonnet, Low
- 3. Filter `kind` backend (3.1–3.3): Sonnet, Medium
- 4. Filter Jenis vendor frontend (4.1–4.2): Sonnet, Medium
- 5. "Lihat Ekspor Saya" (5.1–5.2): Sonnet, Low
- 6. Seed dev (6.1–6.3): Sonnet, Medium
- 7. Checkpoint gate: Sonnet, Medium
- 8. Dokumentasi: Haiku, Low
- 9. Cek browser manual oleh user: Haiku, Low

**Opus-required**
- Tidak ada. Tidak menyentuh auth, uang, rekonsiliasi, migrasi, atau delete (requirements "Area sensitif"); filter `kind` hanya baca di replica dengan RBAC yang sudah ada.

Dependensi lintas grup: 2.1 dan 2.2 butuh 1; 3.2 butuh 3.1; 4.2 butuh 4.1 dan 2.1 (file sama); 5.2 butuh 5.1 dan 4.2; 6.2 butuh 6.1; 6.3 butuh 6.2 dan 3.2; 7 butuh semua task kode.

## Tasks

- [x] 1. Komponen `BackLink`
  - Buat `frontend/CompanyPortal-Vite/src/components/ui/BackLink.tsx`: props `to`, `label`, `search?`; satu TanStack `<Link>` bergaya tombol ghost (min 44×44 px), teks `← {label}` dengan panah `aria-hidden`; tanpa `history.back`.
  - `BackLink.test.tsx`: href = `to`, nama aksesibel = label, bisa difokus dengan keyboard.
  - _Requirements: 1.1, 1.3, 1.5_
  - _Model: Sonnet, Effort: Low — komponen presentasi kecil_

- [x] 2. Pasang BackLink di halaman Pengaturan
  - [x] 2.1 Halaman `/settings/admin/*`
    - Tambah `<BackLink to="/settings" label="Kembali ke Pengaturan" />` di atas judul: `AdminUsersPage`, `AdminVendorsPage`, `AdminATMsPage`, `AdminAcmAreasPage`, `AdminHolidaysPage`, halaman `admin-regions`, `MasterDataIOPage` (tabel Penempatan di design).
    - Tidak mengubah `VendorHeader`, `VendorBranchDetailPage`, `VendorBranchEditPage`, `SettingsHubPage`.
    - _Requirements: 1.1, 1.2, 1.4, 1.6_
  - [x] 2.2 Halaman `/settings/roles` dan `/settings/rbac/*`
    - Sama untuk halaman `role-management`, `RbacUsersPage`, `RbacDelegationsPage`, `RbacLeavesPage`, `RbacPoliciesPage`.
    - _Requirements: 1.1, 1.2, 1.4, 1.6_
  - [x] 2.3 Test Property 3
    - Satu test berparameter: render tiap halaman "tambah" (12), assert tepat satu tautan bernama "Kembali ke Pengaturan" dengan `href="/settings"`; render `SettingsHubPage`, assert tidak ada.
    - Test halaman yang sudah ada tetap lulus.
    - _Requirements: 1.1, 1.2, 1.3, 1.6, 6.3_
  - _Model: Sonnet, Effort: Low — perubahan mekanis per halaman_

- [x] 3. Filter `kind` di `GET /api/v1/admin/vendors`
  - [x] 3.1 Query sqlc
    - `backend/queries/vendors_admin.sql`: tambah `AND (sqlc.narg('kind')::text IS NULL OR v.kind = sqlc.narg('kind')::text)` di `ListVendorsAdmin` **dan** `CountVendorsAdmin`.
    - `cd backend && sqlc generate` (v1.31.1), lalu hand-fix `UserLeafe` → `UserLeave` di `backend/internal/db/`.
    - _Requirements: 2.1, 2.2, 2.4, 2.5_
  - [x] 3.2 Handler
    - `AdminVendorHandler.List`: baca `kind`; kosong → `nil`; selain `FLM_VENDOR`/`INTERNAL` → 400 `bad_request` "kind tidak valid" sebelum memanggil service; teruskan ke `ListVendorsAdminParams.Kind` dan `CountVendorsAdminParams.Kind`.
    - Route group, RBAC, replica, dan bentuk respons tidak berubah.
    - _Requirements: 2.1, 2.2, 2.3, 2.4, 2.5_
  - [x] 3.3 Test Property 1 + 2
    - `admin_vendor_handler_test.go` table-driven: `kind` = `FLM_VENDOR`, `INTERNAL`, kosong (→ `nil`), `xyz` (400, fake service tidak dipanggil), dan kombinasi dengan `status` + `pks_status` + paging; fake service merekam `Kind` di argumen List dan Count.
    - Bila sudah ada integration test untuk `ListVendorsAdmin` (`//go:build integration`), tambah kasus `kind`; jika tidak ada, catat di Test Results dan andalkan cek manual 6.3.
    - _Requirements: 2.1, 2.2, 2.3, 2.4, 6.1_
  - _Model: Sonnet, Effort: Medium — query + handler mengikuti pola `pks_status`_

- [x] 4. Filter Jenis vendor di `AdminVendorsPage`
  - [x] 4.1 URL state
    - `ADMIN_VENDORS_SEARCH_SCHEMA`: `kind: z.enum(["FLM_VENDOR", "INTERNAL"]).optional()` dengan nilai tidak valid dibuang, sama dengan field lain.
    - `useAdminVendorsUrlState.test.ts`: `kind` valid tersimpan, tidak valid dibuang.
    - _Requirements: 2.6, 2.7, 6.2_
  - [x] 4.2 Dropdown + API
    - `api.ts`: kirim `kind` bila ada. `requestPksExport` **tidak** mengirim `kind` (Q9).
    - `AdminVendorsPage`: dropdown "Jenis vendor" (Semua / Vendor FLM / ROH), default Semua; mengubahnya me-reset `page` ke 1, seperti filter status.
    - Component test: tiga opsi; pilih ROH → request list berisi `kind=INTERNAL` dan `page=1`; ekspor PKS tidak membawa `kind`.
    - _Requirements: 2.6, 2.7, 6.2_
  - _Model: Sonnet, Effort: Medium — form state + URL search_

- [x] 5. Hapus "Lihat Ekspor Saya" di halaman Vendor
  - [x] 5.1 `ReportExportNotice`
    - Prop `showMyExportsLink?: boolean` (default `true`); `false` → tautan tidak dirender, pesan error tetap.
    - Test: default menampilkan tautan; `false` tidak menampilkan tautan tetapi tetap menampilkan error.
    - _Requirements: 3.2, 3.3_
  - [x] 5.2 Pakai di `AdminVendorsPage`
    - `<ReportExportNotice error={…} showMyExportsLink={false} />`.
    - Component test: tidak ada teks "Lihat Ekspor Saya"; error permintaan ekspor tetap tampil. Test `DsrLateReport` dan `Replenishment` yang memeriksa tautan tetap lulus.
    - _Requirements: 3.1, 3.2, 3.3, 6.2_
  - _Model: Sonnet, Effort: Low — satu prop opsional_

- [x] 6. Seed dev `backend/scripts/dev/seed_settings_review.sql`
  - [x] 6.1 Vendor dummy + cabang
    - Header "DEV ONLY — not a migration" + cara jalan (`psql -v ON_ERROR_STOP=1 … -f …`); satu `BEGIN … COMMIT`; tanpa `audit_logs`.
    - `DMY1`…`DMY5` (`FLM_VENDOR`, prefix `DMA`…`DME`) + `DMYROH` (`INTERNAL`, prefix NULL), aktif, `ON CONFLICT (code) DO NOTHING`.
    - 2 cabang per vendor: `<code>_001` "Cabang Jakarta" `JKT`, `<code>_002` "Cabang Bandung" `BDG`, `ON CONFLICT DO NOTHING`.
    - Tanpa vault, ATM, paket, PKS.
    - _Requirements: 4.1, 4.2, 4.3, 4.4, 4.5, 4.6_
  - [x] 6.2 PIC vendor-wide + cabang
    - Di file yang sama, setelah 6.1: 2 SeedPic per EligibleVendor (`vendor_branch_id` NULL) dan 2 per `vendor_branches` aktif milik EligibleVendor, query di design "Bagian 2".
    - `roles` ketiganya, `is_notification_recipient = false`, `is_director = false`, `is_active = true`, email `@pic.dev.local`; `NOT EXISTS` per vendor/cabang; hanya `INSERT`.
    - _Requirements: 5.1, 5.2, 5.3, 5.4, 5.5, 5.6_
  - [x] 6.3 Verifikasi Property 4 di DB dev
    - **Minta OK user dulu** sebelum menjalankan script di DB dev (menulis data).
    - Jalankan 2×; query hitung: 6 DummyVendor, 12 cabang dummy, SeedPic = 2 per EligibleVendor dan 2 per cabang aktif, jumlah PIC non-seed sebelum = sesudah; filter `kind=INTERNAL` di API mengembalikan `DMYROH`. Catat hasil di Test Results.
    - _Requirements: 2.1, 4.1, 4.2, 4.4, 5.1, 5.2, 5.4, 5.5_
  - _Model: Sonnet, Effort: Medium — SQL idempoten, perlu teliti constraint_

- [x] 7. Checkpoint gate
  - `cd backend && go test ./...`, `golangci-lint run --build-tags integration ./...`, `pnpm --dir frontend/CompanyPortal-Vite run test`, `lint`, `build`. Semua hijau; catat hasil di `## Notes` → `### Test Results`.
  - _Requirements: 6.1, 6.2, 6.3, 6.4_
  - _Model: Sonnet, Effort: Medium — menjalankan dan memperbaiki sampai hijau_

- [x] 8. Dokumentasi
  - Update baris fitur di `.claude/development-progress.md`; `graphify update .`.
  - _Requirements: 6.4_
  - _Model: Haiku, Effort: Low — teks status_

- [ ] 9. Cek browser manual oleh user
  - BackLink di 12 halaman (klik + keyboard, buka langsung dari URL); filter Jenis vendor (Semua/FLM/ROH, URL, paging); tidak ada "Lihat Ekspor Saya" di halaman Vendor, masih ada di DSR telat/Replenishment; data dummy + PIC tampil.
  - Dicatat "outstanding (user)" — tidak pernah ditandai selesai oleh AI.
  - _Requirements: 6.5_
  - _Model: Haiku, Effort: Low — dikerjakan user_

## Notes

### Keputusan yang tercermin

- Q1–Q2 → task 1, 2. Q3, Q5, Q7, Q8 → task 6. Q4 → task 3, 4. Q6 → task 5. Q9 → task 4.2 (ekspor tidak membawa `kind`).

### Correctness Properties → task

- Property 1 → 3.3 (+ 6.3 cek API di dev). Property 2 → 3.3. Property 3 → 2.3. Property 4 → 6.3.

### Definition of Done

- CLAUDE.md §11: RBAC tidak berubah; tanpa write baru di layar (maker-checker tidak tersentuh); read di replica; tanpa migrasi; respons datar sama dengan tetangga; OpenAPI tidak berlaku (`/admin/vendors` belum di spec).

### Test Results

**6.3 (2026-11-17, dev DB via localhost, user OK "Run Wave 2, seed OK")** — `seed_settings_review.sql` dijalankan 2×, run 1 dan 2 sukses (tanpa error):
- Sebelum: DummyVendor 0, cabang dummy 0, PIC non-seed 0, SeedPic 0.
- Setelah run 1 = setelah run 2: 6 DummyVendor, 12 cabang dummy, SeedPic 868 = 2×(EligibleVendor) + 2×(cabang aktif EligibleVendor) (868 dihitung dari DB), PIC non-seed 0 = 0 (tidak berubah). Run 2 menambah 0 baris (idempoten).
- Filter `kind=INTERNAL`: API tidak berjalan; diverifikasi lewat SQL predikat query (`v.kind='INTERNAL'`, tidak terhapus) → `DMYROH` dan `ROH` (ROH sudah ada sebelumnya). Cek lewat API tersisa untuk user.
**7 Checkpoint gate (2026-11-17, ecc:verification-loop)**:
- `cd backend && go build ./... && go vet ./...` → pass.
- `cd backend && go test ./...` → pass (semua paket ok). Integration test (termasuk 3.3 `kind`) skip: `DATABASE_URL` kosong — dilaporkan skipped, bukan lulus.
- R1 (2026-11-17, setelah review): `go test -tags integration ./internal/service -run TestVendorAdminListPks_Property_Integration -v -count=1` ke DB dev (localhost, satu tx di-rollback) → PASS, rapid 100 kasus (termasuk kasus `kind` dari 3.3).
- `cd backend && golangci-lint run --build-tags integration ./...` → 0 issues.
- `pnpm --dir frontend/CompanyPortal-Vite run test` → run 1: 2 gagal (`import-alias-compliance.property.test.ts`: `rbac-settings/__tests__/backLinkPlacement.test.tsx` (file baru spec ini) memakai import relatif lintas-feature). Diperbaiki → `@/features/...`. Run ulang: 210 file / 2007 tes pass.
- `pnpm --dir frontend/CompanyPortal-Vite run lint` (Biome) → run 1: 11 error format-only di file yang diubah spec ini (AdminAcmAreas, AdminATMs, AdminHolidays, AdminRegions, AdminUsers pages + tests, MasterDataIOPage). `biome format --write` pada file-file itu saja (11 file diperbaiki). Run ulang: 631 file, 0 error.
- `pnpm --dir frontend/CompanyPortal-Vite run build` → pass (budget initial JS 200.4 KB gzip ≤ 300 KB; dijalankan sebelum format-only fix; `tsc --noEmit` setelahnya bersih).
- Cek browser manual: outstanding (user) — task 9.

## Task Dependency Graph

- **Wave 0:** 1 (BackLink), 3.1 (query sqlc), 5.1 (prop notice), 6.1 (seed vendor) — tanpa dependensi.
- **Wave 1:** 2.1, 2.2 (butuh 1), 3.2 (butuh 3.1), 4.1, 6.2 (butuh 6.1).
- **Wave 2:** 2.3 (butuh 2.1, 2.2), 3.3 (butuh 3.2), 4.2 (butuh 4.1; `AdminVendorsPage` setelah 2.1), 6.3 (butuh 6.2 + 3.2).
- **Wave 3:** 5.2 (butuh 5.1; `AdminVendorsPage` setelah 4.2).
- **Wave 4:** 7 (butuh semua task kode).
- **Wave 5:** 8, 9 (butuh 7).

```json
{
  "waves": [
    { "id": 0, "tasks": ["1", "3.1", "5.1", "6.1"] },
    { "id": 1, "tasks": ["2.1", "2.2", "3.2", "4.1", "6.2"] },
    { "id": 2, "tasks": ["2.3", "3.3", "4.2", "6.3"] },
    { "id": 3, "tasks": ["5.2"] },
    { "id": 4, "tasks": ["7"] },
    { "id": 5, "tasks": ["8", "9"] }
  ]
}
```
