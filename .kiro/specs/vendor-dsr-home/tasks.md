# Implementation Plan: Vendor DSR Home

## Overview

Rencana implementasi ini menjadikan menu DSR sebagai landing page Vendor Portal. Pekerjaan dipecah menjadi langkah-langkah inkremental yang saling membangun: dimulai dari fungsi murni + property test (fondasi kebenaran), lalu backend read-only (endpoint identitas/cabang dan DSR ter-scope vendor), kemudian frontend (hooks, komponen, routing/guard), dan diakhiri dengan aksesibilitas serta integrasi menyeluruh.

Konvensi:
- Frontend: `frontend/VendorPortal-Vite/` — React 19 + TS + Vite 6, TanStack Router/Query/Table, Tailwind 4, Lucide, tema "Merah Menyala". Kode fitur di `src/features/dsr-home/`, logika murni di `dsr.logic.ts`, test di `__tests__/` (Vitest + fast-check).
- Backend: ATM backend `backend/` (Go 1.23+, Chi v5), modul `internal/dsr`, `internal/vendor`, `internal/assignment`, `internal/location`, `internal/atm`. sqlc + pgx/pgxpool, baca dari read replica (`dbRead`). Respons JSON datar (BUKAN `pkg/response`). Auth via `pkg/middleware` `RequireAuth` + `RequireRoles`, `vendor_id` dari klaim JWT. Test co-located `*_test.go` (Go testing + rapid).
- Tidak memperkenalkan tabel/kolom DB baru. Pemetaan per-denominasi → per-ATM dilakukan di service; kembalikan `rows: []` bila data per-ATM tidak tersedia.

## Tasks

- [ ] 1. Siapkan fondasi fitur dan tipe bersama (frontend)
  - [ ] 1.1 Buat struktur folder dan tipe bersama
    - Buat struktur folder `src/features/dsr-home/` sesuai desain
    - Definisikan `types.ts`: `VendorIdentity`, `AssignedBranch`, `BalanceStatus`, `DsrRow`, `DsrSummary`, `DsrByDateResponse`, dan props komponen (`VendorIdentityPanelProps`, `DsrTableProps`, `DateSelectorProps`)
    - Pastikan konfigurasi Vitest + fast-check + jsdom + @testing-library aktif di `VendorPortal-Vite`
    - _Requirements: 3.1, 6.4_

- [ ] 2. Implementasi fungsi murni DSR (`dsr.logic.ts`)
  - [ ] 2.1 Implementasi `getBalanceStatus`, `formatIDR`, `summarize`
    - `getBalanceStatus(endingBalance)`: Critical < 50.000.000; Low 50.000.000–150.000.000 inklusif; Normal > 150.000.000
    - `formatIDR(amount)`: `"IDR "` + angka locale id-ID (pemisah ribuan titik, tanpa desimal)
    - `summarize(rows)`: `atmCount` (atmId unik), `criticalCount`, `lowCount`, `totalEndingBalance`; nol untuk masukan kosong
    - _Requirements: 3.3, 3.8, 4.1, 4.2, 4.3, 4.7_
  - [ ] 2.2 Implementasi `pickDefaultDate`, `sortRows`, `truncate`
    - `pickDefaultDate(dates)`: tanggal maksimum jika ada, else hari ini Asia/Jakarta
    - `sortRows(rows, column, direction)`: default asc by `atmId`, toggle asc/desc, keluaran permutasi input
    - `truncate(name, maxLength)`: potong + elipsis bila melebihi, sediakan teks penuh
    - _Requirements: 2.2, 2.3, 2.8, 3.6, 3.7, 3.12, 4.6_
  - [ ]* 2.3 Property test klasifikasi & format & agregasi (`__tests__/dsrLogic.property.test.ts`)
    - **Property 2: Klasifikasi Status Saldo dengan batas yang benar** (generator mencakup 49.999.999, 50.000.000, 150.000.000, 150.000.001)
    - **Property 3: Format mata uang IDR (format + round-trip)** (mencakup amount = 0)
    - **Property 4: Agregasi ringkasan DSR** (mencakup input kosong)
    - **Validates: Requirements 3.3, 3.8, 4.1, 4.2, 4.3, 4.7**
  - [ ]* 2.4 Property test tanggal default, sorting, cabang, truncate (`__tests__/dsrLogic.property.test.ts`)
    - **Property 5: Pemilihan tanggal default**
    - **Property 6: Kebenaran sorting kolom**
    - **Property 7: Daftar cabang — urutan dan jumlah**
    - **Property 8: Pemotongan nama vendor**
    - **Validates: Requirements 2.2, 2.3, 2.8, 3.6, 3.7, 3.12, 4.6**

- [ ] 3. Checkpoint - fungsi murni frontend
  - Ensure all tests pass, ask the user if questions arise.

- [ ] 4. Backend: query read-only dan scoping vendor→ATM (sqlc)
  - [ ] 4.1 Definisikan query sqlc untuk identitas vendor + cabang assigned
    - Query `GetVendorByID` (dari `vendors` by id, hanya aktif/belum dihapus) untuk `vendor_name`
    - Query `ListVendorBranches` (dari `vendor_branches WHERE vendor_id = $1`, urut `branch_name` asc)
    - Baca dari `dbRead` (read replica)
    - _Requirements: 2.2, 2.3, 6.5, 6.7_
  - [ ] 4.2 Definisikan query sqlc pusat scoping ATM milik vendor + tanggal DSR tersedia
    - Query bernama `ListVendorATMs` sebagai satu sumber kebenaran rantai `vendors → vendor_branches → (vendor_packages/atm_vendor_packages) → atms`, filter `vendor_id = $1`
    - Query `ListVendorDsrDates` untuk `available_dates` ter-scope vendor
    - Baca dari `dbRead`
    - _Requirements: 3.2, 3.6, 5.1, 6.5, 6.7_
  - [ ] 4.3 Regenerasi kode sqlc dan wire repository ke `dbRead`
    - Jalankan `sqlc generate`, pastikan repository memakai pool replica untuk semua read di alur ini
    - _Requirements: 6.5_

- [ ] 5. Backend: service layer DSR ter-scope vendor
  - [ ] 5.1 Implementasi `VendorIdentityService.GetIdentity(ctx, vendorID)`
    - Ambil nama vendor + daftar cabang; assert `vendorID != nil` dan vendor aktif
    - Susun `branch_count = len(branches)`, cabang terurut `branch_name` asc
    - _Requirements: 2.2, 2.3, 2.4, 5.1, 5.2, 5.5_
  - [ ] 5.2 Implementasi `VendorDsrService.ListVendorDSR(ctx, vendorID, date)` dengan pemetaan per-denominasi → per-ATM
    - Pilih `selected_date`: `date` bila diberi, else tanggal DSR terbaru vendor, else hari ini Asia/Jakarta
    - Petakan skema `atm_dsr_saldo_files/rows` (per-denominasi, kunci teks bebas `vendor`) ke bentuk per-ATM (`DsrRow`); TANPA tabel/kolom baru
    - Bila data per-ATM tidak tersedia untuk tanggal itu, kembalikan `rows: []` (jangan mengarang data)
    - Hitung `balance_status` per baris (ambang server = sumber tunggal) dan `summary` hanya dari rows ter-scope vendor; default urut `atm_id` asc
    - _Requirements: 3.2, 3.6, 3.7, 3.8, 4.1, 4.2, 4.4, 4.5, 4.7, 6.7_
  - [ ] 5.3 Enforcement scope + audit di service layer
    - Tolak akses resource yang tidak tertaut `vendorID` dari `AuthContext` walau lolos middleware
    - Pada akses lintas vendor: tulis satu entri `audit_logs` (user terautentikasi, identifikasi resource, timestamp UTC)
    - Abaikan `vendor_id` yang dikirim klien; selalu pakai klaim JWT
    - _Requirements: 5.1, 5.2, 5.4, 6.6_
  - [ ]* 5.4 Property test isolasi vendor sisi backend (`internal/service/vendor_dsr_scope_property_test.go`, rapid)
    - **Property 1: Isolasi data vendor tak tergantung input klien** — acak `vendorId` JWT vs `vendor_id` klien yang bertentangan; keluaran hanya bergantung klaim JWT
    - **Validates: Requirements 2.4, 3.2, 4.4, 4.5, 5.1, 5.2, 6.6, 6.7**
  - [ ]* 5.5 Unit test service (mock repo) (`internal/service/vendor_dsr_service_test.go`)
    - Scope benar, sesi invalid (vendor_id nil / vendor tidak aktif), penulisan audit lintas vendor, pemetaan empty rows
    - _Requirements: 3.9, 5.4, 5.5_

- [ ] 6. Backend: handler + routing (JSON datar, terproteksi)
  - [ ] 6.1 Implementasi handler `GET /api/v1/vendor/me/branches`
    - Ambil `vendor_id` dari `AuthContext`; kembalikan JSON datar `{vendor_id, vendor_name, branch_count, branches[]}`
    - _Requirements: 2.1, 2.2, 2.3, 6.4, 6.7_
  - [ ] 6.2 Implementasi handler `GET /api/v1/vendor/dsr?date=YYYY-MM-DD`
    - Ambil `vendor_id` dari `AuthContext`; abaikan `vendor_id` klien; kembalikan JSON datar `{vendor_id, selected_date, available_dates, currency, summary, rows}`
    - _Requirements: 3.2, 3.6, 4.1, 6.4, 6.6, 6.7_
  - [ ] 6.3 Registrasi route di grup terproteksi dan mapping error datar
    - Mount di bawah `RequireAuth` + `RequireRoles("VENDOR-USER")`
    - Map error: 401 unauthorized (token hilang/invalid), 403 invalid_session (vendor_id nil/tidak aktif), 403 forbidden (role salah), 403 access_denied + audit (resource vendor lain), 503 service_unavailable (replica down)
    - _Requirements: 5.4, 5.5, 5.6, 6.1, 6.3_
  - [ ]* 6.4 Integration test handler (`internal/handler/vendor_dsr_integration_test.go`, Postgres nyata)
    - Verifikasi bentuk JSON datar, header Bearer wajib, 401/403 sesuai kondisi, pembacaan dari read replica
    - _Requirements: 6.2, 6.4, 6.5, 6.3_

- [ ] 7. Checkpoint - backend endpoint
  - Ensure all tests pass, ask the user if questions arise.

- [ ] 8. Frontend: API client + hooks TanStack Query
  - [ ] 8.1 Implementasi fetcher dengan Bearer JWT dan timeout 10s
    - Sisipkan Authorization Bearer pada setiap request ke ATM backend :8080; timeout 10s memicu error fetch
    - Interseptor 401 memicu pembersihan state auth + query cache
    - _Requirements: 6.1, 6.2, 6.9, 1.8_
  - [ ] 8.2 Implementasi `useVendorIdentity.ts` (GET /vendor/me/branches)
    - `queryKey` menyertakan `vendorId` hanya sebagai cache key (bukan dikirim ke server); expose loading/error/empty
    - _Requirements: 2.4, 2.5, 6.7_
  - [ ] 8.3 Implementasi `useDsrByDate.ts` (GET /vendor/dsr?date=...)
    - Kirim hanya `date` (tanpa `vendor_id`); `retry` manual via `refetch`; `staleTime` wajar untuk data harian
    - _Requirements: 3.2, 3.5, 6.6, 6.9_
  - [ ]* 8.4 Property test filter scope sisi klien (`__tests__/vendorScope.property.test.ts`)
    - **Property 1: Isolasi data vendor tak tergantung input klien** (bagian filter murni klien)
    - **Validates: Requirements 5.1, 6.6, 6.7**

- [ ] 9. Frontend: komponen DSR Home
  - [ ] 9.1 Implementasi `VendorIdentityPanel.tsx`
    - Tampilkan nama vendor (+ elipsis via `truncate` dan atribut `title` bila > 40 char), daftar cabang terurut, `branch_count`
    - Empty state "belum ada cabang", error inline + tombol coba lagi; tema Merah Menyala
    - _Requirements: 2.1, 2.2, 2.3, 2.6, 2.7, 2.8, 2.9_
  - [ ] 9.2 Implementasi `DsrSummaryCards.tsx`
    - 4 kartu: ATM dipantau, ATM Kritis, ATM Rendah, total Saldo Akhir (via `summarize`, `formatIDR`, tabular-nums rata kanan); nol saat kosong
    - _Requirements: 4.1, 4.3, 4.5, 4.7_
  - [ ] 9.3 Implementasi `DateSelector.tsx`
    - Bungkus DatePicker bersama; opsi dari `available_dates`; `onChange` memicu reload tanggal
    - _Requirements: 3.5, 3.6_
  - [ ] 9.4 Implementasi `DsrTable.tsx` (TanStack Table v8)
    - Kolom: ID ATM, Lokasi, Tanggal (Asia/Jakarta), Saldo Awal, Cash In, Cash Out, Saldo Akhir, Status Saldo
    - Uang rata kanan tabular-nums via `formatIDR`; default sort `atmId` asc; toggle sort per header
    - Badge Status Saldo: warna semantik + ikon Lucide unik per status + label teks (bersamaan)
    - Empty state "tidak ada data DSR untuk tanggal ini" / "tidak ada ATM yang di-assign"; error inline + retry, pertahankan tanggal
    - _Requirements: 3.1, 3.3, 3.4, 3.7, 3.8, 3.9, 3.10, 3.11, 3.12, 4.2, 6.9, 7.1_
  - [ ] 9.5 Komposisi `DsrHomePage.tsx`
    - `<main>` berisi IdentityPanel + SummaryCards + DsrTable; kelola state tanggal terpilih; loading skeleton
    - Pastikan tidak ada kontrol pemilih/filter/pencarian vendor apa pun di UI
    - _Requirements: 3.5, 5.3, 7.2_
  - [ ]* 9.6 Unit/komponen test (`__tests__/VendorIdentityPanel.test.tsx`, `DsrTable.test.tsx`, `DsrHomePage.test.tsx`)
    - Nama vendor + jumlah cabang + elipsis/title, empty/error, interaksi sorting header, badge 3-elemen, render kolom, empty/error/loading, tanpa kontrol vendor
    - _Requirements: 2.6, 2.7, 2.8, 3.1, 3.9, 3.10, 3.11, 5.3, 7.1_

- [ ] 10. Frontend: routing, guard, dan redirect landing (TanStack Router)
  - [ ] 10.1 Susun route terproteksi dan default landing ke /dsr
    - `__root.tsx` (Query + Auth + Outlet), `_authed.tsx` (AppShell + guard), `_authed/index.tsx` (`/` → redirect `/dsr`), `_authed/dsr.tsx`
    - Sidebar menandai item DSR aktif (gaya Merah Menyala) saat pathname `/dsr`
    - _Requirements: 1.1, 1.2, 1.4_
  - [ ] 10.2 Implementasi guard + preserved-URL + redirect pasca-login
    - `_authed.beforeLoad`: jika belum auth → `redirect({to:'/login', search:{redirect: location.href}})`
    - `/login` jika sudah auth → redirect `/dsr` (kecuali ada `redirect` valid); pasca-login navigate ke preserved URL, fallback `/dsr` bila invalid/usang
    - Sesi kedaluwarsa (401) saat di `/dsr` → simpan URL, redirect `/login`
    - Prefetch query DSR/identity pada `loader` route `_authed/dsr.tsx`
    - _Requirements: 1.1, 1.3, 1.5, 1.6, 1.8, 1.9_
  - [ ]* 10.3 Property test route guard round-trip (`__tests__/routeGuard.property.test.ts`)
    - **Property 9: Round-trip guard route dengan preserved URL**
    - **Validates: Requirements 1.5, 1.6, 1.9**

- [ ] 11. Frontend: aksesibilitas dan responsivitas
  - [ ] 11.1 Landmark semantik, badge redundan, dan scroll responsif
    - `<main>` untuk konten utama, `<table>`/`<th>`/`<tr>` untuk data tabular
    - Status Saldo selalu warna + ikon unik + label teks (Req 7.1)
    - Bungkus tabel dalam kontainer scroll horizontal terlihat saat viewport < 1024px
    - _Requirements: 7.1, 7.2, 7.3_
  - [ ] 11.2 Indikator fokus, tab order, dan kontras
    - Indikator fokus terlihat (kontras non-teks ≥ 3:1), tab order berurutan tanpa keyboard trap, kontras teks/badge WCAG AA
    - _Requirements: 7.4, 7.5, 7.6_
  - [ ]* 11.3 Test aksesibilitas (`__tests__/a11y.test.tsx`, axe-core + user-event)
    - Landmark, kontras via axe, indikator fokus, tab order tanpa trap, scroll <1024px
    - _Requirements: 7.2, 7.3, 7.4, 7.5, 7.6_

- [ ] 12. Integrasi akhir dan wiring end-to-end
  - [ ] 12.1 Wire DsrHomePage ke hooks, DateSelector, dan guard route
    - Hubungkan `useVendorIdentity` + `useDsrByDate` ke panel/kartu/tabel; state tanggal terpilih dipertahankan saat error/retry
    - Pastikan alur login → redirect `/dsr` → render lengkap terhubung tanpa kode menggantung
    - _Requirements: 1.1, 1.7, 2.5, 3.5, 3.11, 4.8, 6.8, 6.9_
  - [ ]* 12.2 Test integrasi alur frontend (`__tests__/DsrHomePage.test.tsx`)
    - Alur pemilihan tanggal memuat ulang tabel + summary; error/retry mempertahankan tanggal; empty states tampil benar
    - _Requirements: 3.5, 3.9, 3.11, 6.9_

- [ ] 13. Checkpoint akhir - seluruh test lulus
  - Ensure all tests pass, ask the user if questions arise.

## Notes

- Task berpostfiks `*` bersifat opsional (test) dan bisa dilewati untuk MVP lebih cepat, namun tetap masuk dependency graph.
- Setiap task merujuk requirement spesifik untuk traceability; property test merujuk nomor Property dari design.
- Pemetaan DSR per-denominasi → per-ATM (Task 5.2) TIDAK menambah tabel/kolom; kembalikan `rows: []` bila data per-ATM tak tersedia. Penambahan kolom penaut vendor→ATM→DSR adalah perubahan skema terpisah yang butuh approval.
- Semua data ter-scope keras ke `vendor_id` dari klaim JWT; parameter `vendor_id` klien diabaikan (middleware + service).
- Reads memakai read replica (`dbRead`); respons ATM backend berupa JSON datar (bukan `pkg/response`).

## Task Dependency Graph

```json
{
  "waves": [
    { "id": 0, "tasks": ["1.1", "4.1", "4.2"] },
    { "id": 1, "tasks": ["2.1", "2.2", "4.3"] },
    { "id": 2, "tasks": ["2.3", "2.4", "5.1", "5.2"] },
    { "id": 3, "tasks": ["5.3", "8.1"] },
    { "id": 4, "tasks": ["5.4", "5.5", "6.1", "6.2", "8.2", "8.3"] },
    { "id": 5, "tasks": ["6.3", "8.4", "9.1", "9.2", "9.3"] },
    { "id": 6, "tasks": ["6.4", "9.4", "10.1"] },
    { "id": 7, "tasks": ["9.5", "10.2"] },
    { "id": 8, "tasks": ["9.6", "10.3", "11.1"] },
    { "id": 9, "tasks": ["11.2", "12.1"] },
    { "id": 10, "tasks": ["11.3", "12.2"] }
  ]
}
```
