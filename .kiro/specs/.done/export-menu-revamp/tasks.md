# Implementation Plan

## Overview

Urutan implementasi: backend dulu (query hitung → refaktor producer → Previewer → route → kontrak OpenAPI), lalu frontend (tipe dan pemetaan filter → halaman pratinjau → katalog → rute dan navigasi), lalu hapus UI lama, gate kualitas, dokumentasi, dan cek browser manual.

Pola yang diikuti:
- Producer keyset dan snapshot replica: `backend-exq/internal/exportjob/producers.go`, `worker.go` (`pgReader`).
- Guard per kind dan peta galat: `backend-exq/internal/handler/exports_handler.go` (`guard`, `writeServiceError`).
- Integration test producer: `producers_integration_test.go`. Test kontrak: `openapi_contract_test.go`.
- Frontend: `useExportFlow`, `ExportProgressDialog`, `MyExportsTable`, `exportFormParts.tsx` di `features/exports/`; layar dengan filter + tabel: `ReplenishmentScreen.tsx`.

Tidak ada migrasi dan library baru (D7, Req 9.2). sqlc wajib v1.31.1.

### Model Split

**Sonnet-runnable**
- 1. Query hitung pratinjau + sqlc: Sonnet, Medium
- 2. Refaktor producer `tableProducer` (Page / Count / Produce): Sonnet, High
- 3.2 Unit test Previewer: Sonnet, Medium (butuh 3.1)
- 3.3 Integration test jumlah total: Sonnet, Medium (butuh 3.1)
- 4.2 Handler test pratinjau: Sonnet, Medium (butuh 4.1)
- 5. Kontrak OpenAPI + `gen:api:exq`: Sonnet, Low
- 6. Checkpoint backend: Sonnet, Medium
- 7. Frontend tipe, `masterPath`, `datasets.ts`: Sonnet, Medium
- 8. Frontend ExportPreviewPage: Sonnet, Medium
- 9. Frontend ExportCatalogPage: Sonnet, Medium
- 10. Frontend rute, navigasi, tautan: Sonnet, Medium
- 11. Hapus UI ekspor lama: Haiku, Low (Haiku)
- 12. Checkpoint penuh: Sonnet, Medium
- 13. Dokumentasi: Haiku, Low (Haiku)
- 14. Cek browser manual: Haiku, Low (Haiku)

**Opus-required**
- 3.1 Previewer (cek peran, scope, snapshot replica): Opus, High. Alasan: RBAC, karena endpoint ini menampilkan baris master data dan hak lihatnya harus sama persis dengan hak ekspor.
- 4.1 Route pratinjau + guard per kind + wiring main: Opus, Medium. Alasan: RBAC, karena guard yang salah membuka data master ke peran non-admin.

Dependensi lintas grup:
- Opus 3.1 butuh Sonnet 2.1 selesai (`PreviewProducer`).
- Sonnet 3.2 dan 3.3 butuh Opus 3.1. Opus 4.1 butuh Opus 3.1.
- Sonnet 4.2 dan 5.1 butuh Opus 4.1.
- Semua task frontend butuh Sonnet 5.2 (tipe `ExportPreview` hasil `gen:api:exq`).

## Tasks

- [ ] 1. Query hitung pratinjau
  - [x] 1.1 Tulis `backend-exq/queries/export_preview.sql`
    - Delapan query `:one`: `CountVendors`, `CountVendorBranches`, `CountVendorVaults`, `CountVendorPics`, `CountATMs`, `CountATMAssignments`, `CountDsrLocationMaps`, `CountPackagePrices`.
    - Kondisi `WHERE` disalin dari query batch padanannya (`backend/queries/master_data_export.sql`, `backend-exq/queries/vendor_export.sql`) tanpa `id > after_id` dan `LIMIT`. Argumen sesuai tabel design "Data Models".
    - Komentar di kepala file: kondisi harus diubah bersama query batch; Property 2 menangkap penyimpangan.
    - _Requirements: 2.2, 4.7_
  - [x] 1.2 Generate sqlc
    - `cd backend-exq && sqlc generate` (v1.31.1) menghasilkan `internal/repository/export_preview.sql.go`.
    - `backend/queries/master_data_export.sql` tidak diubah, sehingga `backend/internal/db/` tidak perlu di-generate ulang. Pastikan `git diff backend/` kosong.
    - _Requirements: 4.7, 9.2_
  - _Model: Sonnet, Medium — SQL salinan kondisi yang sudah ada; penyimpangan ditangkap integration test 3.3._

- [x] 2. Refaktor producer master data menjadi `tableProducer`
  - [x] 2.1 Implementasi `PreviewProducer` dan `tableProducer[R]` di `producers.go`
    - Interface `PreviewProducer` (`Producer` + `Page` + `Count`) dan struct generik `tableProducer[R]` (`header`, `fetch`, `id`, `cells`, `count`) sesuai design.
    - `Produce` = loop `Page(after, PageSize)` sampai halaman lebih pendek dari `PageSize`; `paged` dihapus bila tidak dipakai lagi.
    - Pindahkan delapan producer. `fetch` dan `cells` = isi closure sekarang, `BatchSize` diganti argumen `limit`. `vendorsProducer(forceAll=true)` memakai status `all` di `fetch` dan `count`. `packagePricesProducer` memakai `wibDate(asOf)` di `fetch` dan `count`.
    - `Page` mengembalikan sel mentah (tanpa `CSVSafe`); `Querier` ditambah delapan method hitung. Producer laporan tidak berubah.
    - _Requirements: 4.4, 4.5, 4.7, 5.3, 9.1_
  - [x] 2.2 Property test `producers_property_test.go` (rapid, `Querier` palsu di memori)
    - **Property 1: Pratinjau berhalaman sama dengan isi file.** Gabungan `Page` berulang (limit acak 1..100) = baris `Produce`, dan baris file = `CSVSafe` sel pratinjau.
    - **Property 3: Paging keyset terbatas dan lengkap** (bagian halaman): paling banyak `limit` baris, `id > after`, urut naik, halaman terakhir dikenali benar.
    - Fake menerapkan `id > after ORDER BY id LIMIT n` dan filter status/scope acak.
    - _Requirements: 2.3, 4.2, 4.4, 4.5, 5.3, 5.10_
  - [x] 2.3 Non-regresi producer dan worker
    - Jalankan test producer, worker, golden header, dan integration test producer yang ada tanpa mengubah ekspektasinya.
    - _Requirements: 9.1_
  - _Model: Sonnet, High — refaktor generik delapan producer yang menulis file ekspor (termasuk nominal); banyak kasus tepi paging, tapi pemetaan sel tidak berubah dan dijaga test._

- [x] 3. Previewer di `backend-exq/internal/exportjob/`
  - [x] 3.1 Implementasi `preview.go`, `errors.go`, `service.go`
    - `resolveScope` dijadikan fungsi `resolveScope(ctx, q createQueries, p)` yang dipakai `Service.Create` dan `Previewer.Preview`.
    - `ErrUnavailable` baru di `errors.go`.
    - `ParsePaging(rawAfter, rawLimit)`: `after` ≥ 0 (default 0), `limit` 1..100 (default 50), selain itu `ErrValidation`.
    - `Previewer` + `NewPreviewer(primary, replica, producers)` + `Preview(ctx, a, p, after, limit)`: `UserID` 0 → `ErrUnauthenticated`; `!allowed(kind, role)` → `ErrForbidden` sebelum menyentuh DB; producer bukan `PreviewProducer` → `ErrUnknownEntity`; `resolveScope` di primary; satu snapshot `REPEATABLE READ READ ONLY` di replica untuk `Count` lalu `Page(after, limit+1)`; `next_after` dari baris ke-`limit`; konflik pemulihan → `ErrUnavailable`; timeout `PreviewTimeout` 10 detik; `asOf = now()`.
    - Tidak menulis audit.
    - _Requirements: 4.1, 4.2, 4.3, 4.6, 4.8, 4.9, 4.11, 7.1_
  - [x] 3.2 Unit test `preview_test.go`
    - **Property 5: Hak pratinjau sama dengan hak ekspor.** Tabel semua kind master data × semua peran: diizinkan jika dan hanya jika `allowed`; peran ditolak tidak membuka snapshot (reader palsu menghitung panggilan).
    - **Property 3** (bagian validasi): `ParsePaging` menolak `after` negatif / bukan angka dan `limit` di luar 1..100.
    - Kasus: `ErrUnavailable` dari galat 40001, `ErrUnknownEntity` untuk kind laporan, `asOf` = jam palsu, tanpa panggilan audit, `next_after` kosong di halaman terakhir.
    - _Requirements: 4.3, 4.8, 4.9, 4.11, 7.1_
  - [x] 3.3 Integration test `preview_integration_test.go` (`//go:build integration`)
    - **Property 2: Jumlah total sama dengan jumlah baris file.** Delapan entity × scope (global, vendor, cabang bila berlaku) × status (`active`, `disabled`, `all`) × tingkat harga (Harga Paket): `Count` = jumlah baris `Produce` di snapshot yang sama.
    - PIC scope cabang hanya PIC cabang itu, scope vendor memuat PIC level vendor + semua cabangnya (D3).
    - `Count` dan `Page` dari satu snapshot (baris yang disisipkan di tengah tidak terlihat).
    - _Requirements: 2.2, 3.6, 4.6, 4.7_
  - _Model: Opus, High — RBAC pada endpoint yang menampilkan baris master data, plus konsistensi snapshot replica._
  - _Model (3.2, 3.3): Sonnet, Medium — test atas perilaku yang sudah ditentukan 3.1._

- [x] 4. Route pratinjau di `ExportsHandler`
  - [x] 4.1 Implementasi route dan wiring
    - `create(...)` dipecah menjadi `parseMaster(...)` + aksi; route `POST` yang ada memakai `parseMaster` lalu `createJob` tanpa perubahan perilaku.
    - Empat route `GET /preview/...` (tabel design) dengan `guard(kind)` yang sama dengan route `POST` padanannya; `ParsePaging`; respons `200` envelope `pkg/response` berisi `Preview`.
    - `writeServiceError`: `ErrUnavailable` → `503 preview_unavailable` "Data sedang diperbarui, coba lagi".
    - `NewExportsHandler(svc, preview)`; `cmd/api/main.go` membuat `NewProducers(dsrLateStart)` sekali untuk `NewWorker` dan `NewPreviewer(dbPool, dbReadPool, producers)`.
    - _Requirements: 4.1, 4.8, 4.10, 8.3_
  - [x] 4.2 Handler test `exports_handler_preview_test.go`
    - **Property 4: Validasi pratinjau sama dengan validasi ekspor.** Generator parameter mentah (kind, entity, id vendor/cabang, status, tingkat harga) dikirim ke route `POST` dan `GET /preview` padanannya dengan service palsu; kode status 400/404 dan diterima/ditolak harus sama.
    - Kasus: envelope `{success, data}`, 403 dari guard untuk peran non-admin, 503 dari `ErrUnavailable`, 400 untuk `after`/`limit` salah.
    - Test handler `POST` dan `/jobs*` yang ada tetap lulus tanpa perubahan ekspektasi.
    - _Requirements: 4.1, 4.8, 4.10, 8.3_
  - _Model: Opus, Medium — wiring guard peran per route; salah satu guard membuka data master ke peran lain._
  - _Model (4.2): Sonnet, Medium — test perbandingan route._

- [x] 5. Kontrak OpenAPI `backend-exq`
  - [x] 5.1 Perbarui `backend-exq/api/openapi.yaml`
    - Empat path `GET /exports/preview/...`, parameter `PreviewAfter` dan `PreviewLimit`, schema `ExportPreview` dan `PreviewEnvelope`, response `Unavailable` (503).
    - _Requirements: 8.2_
  - [x] 5.2 Test kontrak dan tipe frontend
    - `go test ./internal/handler -run OpenAPI` lulus (route chi ↔ path spec).
    - `pnpm --dir frontend/CompanyPortal-Vite run gen:api:exq`, commit `src/lib/api/exq-schema.d.ts`; vitest stale-check lulus.
    - _Requirements: 8.2_
  - _Model: Sonnet, Low — kontrak mengikuti design; test kontrak menangkap selisih._

- [x] 6. Checkpoint backend
  - `cd backend-exq && go test ./...` dan `go test -tags integration ./...` (DB dev), `cd pkg && go test ./...`, `golangci-lint run --build-tags integration ./...` di `backend-exq` dan `pkg`, coverage Go ≥ 80% untuk `backend-exq/internal/*` dan `pkg/`.
  - Test audit `export_requested` / `master_data_exported` / `report_exported` yang ada lulus tanpa perubahan ekspektasi.
  - Triase dan perbaiki galat sebelum lanjut.
  - _Requirements: 7.3, 9.1, 9.3_
  - _Model: Sonnet, Medium — triase galat build/test._

- [x] 7. Frontend: tipe, path, dan pemetaan filter
  - [x] 7.1 `types.ts` dan `api.ts`
    - Tipe `ExportPreview` dari `@/lib/api/exq-schema`; `MasterExportParams` = `ExportRequestParams` tanpa varian `report`.
    - `masterPath(p)` (path + query tanpa prefix); `requestExport` untuk kind master data memakai `post("/exports" + masterPath(p))`; `previewExport(p, after, limit)` memanggil `/exports/preview` + `masterPath(p)` + paging.
    - _Requirements: 3.5, 5.2_
  - [x] 7.2 `datasets.ts`
    - `EXPORT_DATASETS`: delapan data master (slug = nama entity `pkg/csvexport`, label, deskripsi, aturan filter, `numericColumns`, `moneyColumns`, `roles` ADMIN/ADMIN_PARAM) dan dua data laporan (`/dsr-late-report` dengan peran `DsrLateViewerRoles`, `/replenishment` dengan peran `ReplenishmentViewerRoles`).
    - `datasetsForRole(role)` tanpa bypass ADMIN/ADMIN_PARAM.
    - `toParams(id, filter)` sesuai tabel Req 3.1; `null` bila vendor wajib kosong atau cabang tanpa vendor.
    - `parseFilterSearch(id, search)` dan `toSearch(filter)`: nilai tidak valid jatuh ke default (status `active`, tingkat harga `all`, id non-positif = kosong, cabang tanpa vendor = kosong).
    - _Requirements: 1.1, 1.2, 1.3, 1.5, 3.1, 3.2, 3.4, 7.2_
  - [x] 7.3 Property test `datasets.property.test.ts` (fast-check)
    - **Property 6: Satu pemetaan filter untuk pratinjau dan ekspor.** `null` jika dan hanya jika filter wajib kosong atau cabang tanpa vendor; kind/entity sesuai tabel Req 3.1; path pratinjau tanpa `/preview` dan `after`/`limit` = path permintaan.
    - **Property 7: Filter URL bulat-balik.** `parseFilterSearch` selalu valid; `parseFilterSearch(toSearch(f)) = f`.
    - **Property 8: Katalog mengikuti peran ekspor.** `datasetsForRole` memuat data jika dan hanya jika peran ada di `roles`; `ADMIN_PARAM` tidak melihat data laporan; `roles` tiap data sama dengan salinan `KindRoles` (daftar peran ditulis eksplisit di test).
    - _Requirements: 1.2, 2.5, 3.1, 3.2, 3.3, 3.4, 3.5, 5.2, 6.1, 7.2_
  - [x] 7.4 `api.test.ts`
    - `masterPath` untuk semua varian kind master data; `previewExport` memakai prefix `/exports/preview` dan menyertakan `after`/`limit`; `requestExport` master data tetap memanggil path yang sama dengan sebelumnya.
    - _Requirements: 3.5, 5.2_
  - _Model: Sonnet, Medium — pemetaan murni dengan property test; backend tetap menolak 403 bila daftar peran basi._

- [x] 8. Frontend: ExportPreviewPage
  - [x] 8.1 Hook `useExportPreview(params | null, after)` di `hooks.ts`
    - TanStack Query, `enabled: params !== null`, `placeholderData: keepPreviousData`, key `["exports","preview", params, after]`. `useExportFlow` tidak berubah.
    - _Requirements: 2.4, 2.6_
  - [x] 8.2 `PreviewFilterBar.tsx`
    - `VendorSelect`, `BranchSelect` (hanya saat vendor terpilih), `FilterSelect` status dan tingkat harga, tombol "Terapkan". Isian = state draft; "Terapkan" menulis search URL lewat `navigate({ search })`.
    - Mengosongkan vendor ikut mengosongkan cabang. Petunjuk "Pilih vendor dulu" saat vendor wajib kosong. Setiap filter berlabel.
    - _Requirements: 2.4, 2.5, 2.11, 3.2, 3.3, 3.4_
  - [x] 8.3 `PreviewTable.tsx`
    - `DataTable` tanpa `pageSize`; kolom dari `columns` (nama kolom CSV apa adanya); `numericColumns` rata kanan + `tabular-nums`, `moneyColumns` + `font-mono`; `is_active` sebagai `Badge` ikon + "Aktif"/"Nonaktif"; caption "Pratinjau <nama data>".
    - Paging tumpukan kursor (`[0]`, reset saat Applied_Filter berubah), tombol Sebelumnya / Berikutnya.
    - Keadaan memuat (tabel lama tetap tampil + indikator), kosong (`EmptyState` ikon + teks), galat (`NoticeBanner` ikon + teks + "Coba lagi").
    - _Requirements: 2.1, 2.3, 2.6, 2.7, 2.8, 2.9, 2.11_
  - [x] 8.4 `PreviewExportBar.tsx` dan `exportButtonState.ts`
    - Tombol primer "Ekspor {total} baris"; `exportButtonState({dirty, loading, total})` menentukan aktif/nonaktif dan pesan (filter belum diterapkan, total 0 / memuat, lebih dari 1.000.000 baris).
    - Klik → `request(toParams(id, applied))` lalu `ExportProgressDialog`; penolakan 400/403/429 lewat `RequestError`; catatan "File diambil saat ekspor diproses; perubahan data setelah tabel dimuat bisa ikut masuk."
    - _Requirements: 5.1, 5.2, 5.3, 5.4, 5.5, 5.6, 5.7, 5.8, 5.9_
  - [x] 8.5 `ExportPreviewPage.tsx`
    - Tombol kembali ke `/exports`, judul data, jumlah total (`Intl.NumberFormat("id-ID")`, `tabular-nums`), komposisi 8.2-8.4 dengan Applied_Filter dari search URL.
    - _Requirements: 2.2, 2.10_
  - [x] 8.6 Property test `exportButtonState.property.test.ts`
    - **Property 9: Status tombol ekspor.** Aktif jika dan hanya jika `!dirty && !loading && 0 < total <= 1_000_000`; nonaktif karena `dirty` atau batas baris selalu membawa pesan.
    - _Requirements: 5.4, 5.5, 5.6_
  - [x] 8.7 Component test `ExportPreviewPage.test.tsx` (API dimock)
    - Kolom = `columns`, total tampil, Berikutnya/Sebelumnya memakai kursor, terapkan filter mereset ke halaman pertama, petunjuk vendor wajib tanpa panggilan API, keadaan kosong dan galat + coba lagi.
    - Tombol ekspor mengirim Applied_Filter (bukan draft), dialog terbuka, 429 tampil inline tanpa dialog, catatan waktu pengambilan data tampil.
    - Badge `is_active`, rata kanan + monospace nominal, caption tabel, label filter, navigasi keyboard.
    - _Requirements: 2.1, 2.2, 2.3, 2.4, 2.6, 2.7, 2.8, 2.9, 2.10, 2.11, 3.3, 3.4, 5.1, 5.2, 5.7, 5.8, 5.9_
  - _Model: Sonnet, Medium — UI di atas pola yang ada; logika tombol diuji property test._

- [x] 9. Frontend: ExportCatalogPage
  - [x] 9.1 `ExportCatalogPage.tsx`
    - `PageHeader` "Export Data"; section "Data master" dan "Laporan" sebagai daftar `<ul>` (ikon, nama, deskripsi, ringkasan filter, chevron), bukan grid kartu; grup kosong tidak dirender.
    - Baris data master = `<Link>` ke `/exports/<slug>`; baris laporan = `<Link>` ke layar laporan. Bagian "Ekspor Saya" memakai `MyExportsTable`.
    - _Requirements: 1.1, 1.3, 1.4, 1.5, 1.7, 6.2, 7.4_
  - [x] 9.2 Component test `ExportCatalogPage.test.tsx`
    - Data per peran (ADMIN, ADMIN_PARAM, ATM-USER, BRANCH-ATM-USER, BRANCH-ATM-SPV), tautan laporan ke `/dsr-late-report` dan `/replenishment`, tautan data master ke `/exports/<slug>`, Ekspor Saya tampil, navigasi keyboard dan nama tautan.
    - _Requirements: 1.1, 1.2, 1.3, 1.4, 1.5, 1.6, 1.7, 6.2, 7.4_
  - _Model: Sonnet, Medium — halaman daftar sederhana._

- [x] 10. Frontend: rute, navigasi, dan tautan
  - [x] 10.1 `routes/exports.tsx` dan `routeTree.ts`
    - `/exports` merender `ExportCatalogPage` dengan guard `EXPORTS_PAGE_ROLES` yang ada.
    - `exportPreviewRoute` path `/exports/$dataset`, `requireRoles(["ADMIN","ADMIN_PARAM"])`, `validateSearch` = `parseFilterSearch`, slug tidak dikenal → `UnknownDatasetState` lokal (ikon + teks + tautan ke `/exports`). Daftarkan di `routeTree.ts`.
    - _Requirements: 1.4, 1.6, 2.5, 6.3_
  - [x] 10.2 Pengalihan `/settings/admin/exports` dan kartu SettingsHub
    - `routes/settings/admin/exports.tsx`: `beforeLoad` melempar `redirect({ to: "/exports" })`, tanpa komponen.
    - `SettingsHubPage.tsx`: kartu `admin-exports` → `href: "/exports"`, deskripsi disesuaikan.
    - _Requirements: 6.4, 6.5_
  - [x] 10.3 Item sidebar
    - `NAV_CONFIG`: `{ id: "exports", label: "Export Data", icon: Download, href: "/exports", roles: EXPORTS_PAGE_ROLES, group: "general" }`.
    - _Requirements: 6.1_
  - [x] 10.4 Tautan Ekspor Saya di `ExportProgressDialog.tsx`
    - Satu konstanta `MY_EXPORTS_PATH = "/exports"` untuk semua kind; `ReportExportNotice` sudah menunjuk `/exports` (cek saja).
    - _Requirements: 6.6_
  - [x] 10.5 `MasterDataIOPage.tsx`
    - Hapus select "Status data ekspor", tombol "Minta ekspor", `useExportFlow`, dan dialog; tambah tautan "Ekspor data ini" ke `/exports/${entity}`. Impor dan unduh templat tidak berubah.
    - _Requirements: 6.7_
  - [x] 10.6 Test rute dan penyesuaian test yang ada
    - Baru: `/settings/admin/exports` mengalihkan ke `/exports`; slug tidak dikenal = `UnknownDatasetState`; `/exports/$dataset` Forbidden untuk ATM-USER; `/exports` Forbidden untuk peran tanpa hak ekspor (mis. ACM-USER).
    - Sesuaikan: `MasterDataIOPage.test.tsx` (tautan baru, tanpa tombol ekspor), `SettingsHubPage.exportsCard.test.tsx` (href `/exports`), `ExportProgressDialog.test.tsx` (satu path), `navigation.property.test.ts` (item `exports`).
    - _Requirements: 1.6, 6.1, 6.3, 6.4, 6.5, 6.6, 6.7_
  - _Model: Sonnet, Medium — wiring rute dan navigasi; guard frontend hanya UX, backend tetap menolak._

- [x] 11. Hapus UI ekspor lama
  - Hapus `ExportDataPage.tsx`, `VendorExportForm.tsx`, `PackagePriceExportForm.tsx`, `ExportForms.test.tsx`, `MyExportsPage.tsx`, `MyExportsPage.test.tsx`; rapikan ekspor di `features/exports/index.ts`. `exportFormParts.tsx` tetap.
  - Grep CompanyPortal untuk nama komponen yang dihapus dan `"/settings/admin/exports"` (hanya boleh tersisa di file rute pengalihan dan test-nya).
  - _Requirements: 8.1_
  - _Model: Haiku, Low — penghapusan file mekanis setelah 10 selesai._

- [x] 12. Checkpoint penuh
  - CompanyPortal: `pnpm run lint`, `pnpm run test:coverage` (84/80/80/85), `pnpm run build` + `check-bundle.mjs`, `gen:api:exq` tanpa diff.
  - Test `DsrLateReportScreen`, `ReplenishmentScreen`, dan `exportRequest.property.test.tsx` lulus tanpa perubahan ekspektasi.
  - Ulangi gate backend task 6 bila ada perubahan backend setelahnya. Pastikan tidak ada file baru di `backend/migrations/` dan tidak ada dependency baru di `package.json` / `go.mod`.
  - _Requirements: 7.2, 9.1, 9.2, 9.3_
  - _Model: Sonnet, Medium — triase galat lint/test/build._

- [x] 13. Dokumentasi
  - `development-plan.md` X.9: status built + catatan cek manual. `.claude/development-progress.md`: entri fitur. `CLAUDE.md` Sec 3 tidak berubah (tidak ada tabel baru).
  - _Requirements: 9.2_
  - _Model: Haiku, Low — pembaruan dokumen._

- [ ] 14. Cek browser manual (pengguna, GR#10)
  - Siapkan checklist untuk pengguna: katalog per peran (ADMIN, ADMIN_PARAM, ATM-USER), pratinjau tiap data master, filter + Terapkan + URL dibagikan, paging, tombol ekspor nonaktif saat filter belum diterapkan, pop-up + unduh otomatis, file sama dengan tabel, Ekspor Saya, pengalihan `/settings/admin/exports`, tautan dari Ekspor & Impor CSV, keyboard dan screen reader.
  - _Requirements: 1.7, 2.11, 5.7_
  - _Model: Haiku, Low — agen hanya menyiapkan checklist; pengecekan dilakukan pengguna._

## Notes

- Spec ini tidak mengubah endpoint permintaan ekspor, siklus `export_jobs`, worker, janitor, batas job, format CSV, maupun audit (Req 7.3, 8.3, 9.1).
- `backend/queries/master_data_export.sql` tidak disentuh. Kalau kondisi `WHERE` query batch berubah di masa depan, ubah query hitung di `export_preview.sql` juga; Property 2 (task 3.3) gagal bila lupa.
- Daftar peran di `datasets.ts` adalah salinan `KindRoles`. Kalau peran ekspor berubah di backend, perbarui `datasets.ts` dan test Property 8 bersama.
- Header tabel memakai nama kolom CSV apa adanya (design T7).

### Definition of Done

- Semua task 1-13 dicentang, gate task 6 dan 12 hijau.
- Setiap Correctness Property 1-9 punya test yang lulus (2.2, 3.2, 3.3, 4.2, 7.3, 8.6).
- Tanpa migrasi dan dependency baru.
- Cek browser manual task 14 dilakukan pengguna.

## Task Dependency Graph

- **Wave 0:** 1.1 (query hitung).
- **Wave 1:** 1.2 (sqlc, butuh 1.1).
- **Wave 2:** 2.1 (refaktor producer, butuh method hitung dari 1.2).
- **Wave 3:** 2.2, 2.3 (test atas 2.1), 3.1 (Previewer, butuh `PreviewProducer` 2.1).
- **Wave 4:** 3.2, 3.3 (test atas 3.1), 4.1 (route, butuh 3.1).
- **Wave 5:** 4.2 (test atas 4.1), 5.1 (OpenAPI, butuh route 4.1).
- **Wave 6:** 5.2 (test kontrak + `gen:api:exq`, butuh 5.1).
- **Wave 7:** 6 (checkpoint backend, butuh 2.2-5.2), 7.1 (tipe frontend, butuh 5.2).
- **Wave 8:** 7.2, 7.4, 8.1 (butuh 7.1); 10.2, 10.3, 10.4, 10.5 (file terpisah, tanpa ketergantungan halaman baru).
- **Wave 9:** 7.3 (butuh 7.2), 8.2, 8.3, 8.4 (butuh 7.2 + 8.1), 9.1 (butuh 7.2).
- **Wave 10:** 8.5 (butuh 8.2-8.4), 8.6 (butuh 8.4), 9.2 (butuh 9.1).
- **Wave 11:** 8.7 (butuh 8.5), 10.1 (rute, butuh 8.5 + 9.1).
- **Wave 12:** 10.6 (test rute dan tautan, butuh 10.1-10.5).
- **Wave 13:** 11 (hapus UI lama, butuh rute 10.1-10.2 tidak lagi mengimpor komponen lama).
- **Wave 14:** 12 (checkpoint penuh).
- **Wave 15:** 13 (dokumentasi).
- **Wave 16:** 14 (cek browser manual).

```json
{
  "waves": [
    { "id": 0, "tasks": ["1.1"] },
    { "id": 1, "tasks": ["1.2"] },
    { "id": 2, "tasks": ["2.1"] },
    { "id": 3, "tasks": ["2.2", "2.3", "3.1"] },
    { "id": 4, "tasks": ["3.2", "3.3", "4.1"] },
    { "id": 5, "tasks": ["4.2", "5.1"] },
    { "id": 6, "tasks": ["5.2"] },
    { "id": 7, "tasks": ["6", "7.1"] },
    { "id": 8, "tasks": ["7.2", "7.4", "8.1", "10.2", "10.3", "10.4", "10.5"] },
    { "id": 9, "tasks": ["7.3", "8.2", "8.3", "8.4", "9.1"] },
    { "id": 10, "tasks": ["8.5", "8.6", "9.2"] },
    { "id": 11, "tasks": ["8.7", "10.1"] },
    { "id": 12, "tasks": ["10.6"] },
    { "id": 13, "tasks": ["11"] },
    { "id": 14, "tasks": ["12"] },
    { "id": 15, "tasks": ["13"] },
    { "id": 16, "tasks": ["14"] }
  ]
}
```