# Implementation Plan: Export Preview Simplify

## Overview

Empat perbaikan UI halaman pratinjau Export Data di `frontend/CompanyPortal-Vite/src/features/exports/`, tanpa perubahan backend, `datasets.ts`, atau rute. Urutan: empat perubahan kode kecil di file berbeda (paralel), lalu test, gate, dokumen, dan cek browser.

Pola rujukan: `FilterSelect` yang sudah dipakai `BranchSelect` untuk `VendorSelect`; test komponen yang ada di `ExportPreviewPage.test.tsx` (mock `useVendorsList` / API) untuk test baru `exportFormParts.test.tsx`.

### Model Split

**Sonnet-runnable**
- 1. Pesan `dirty` di `exportButtonState` + test — Haiku, Low (Haiku)
- 2. `VendorSelect` tanpa kotak pencarian + property test — Haiku, Low (Haiku); 2.2 Sonnet, Low
- 3. Tombol "Search" di `PreviewFilterBar` — Haiku, Low (Haiku)
- 4. `PreviewExportBar`: label "Ekspor", hapus catatan — Haiku, Low (Haiku)
- 5. Penyesuaian `ExportPreviewPage.test.tsx` — Sonnet, Medium (needs 1, 2.1, 3, 4)
- 6. Checkpoint gate kualitas — Sonnet, Medium (needs 1-5)
- 7. Dokumen — Haiku, Low (Haiku) (needs 6)
- 8. Cek browser manual — Haiku, Low (Haiku), dilakukan user (needs 6)

**Opus-required**
- none. Tidak menyentuh money, auth, approval, rekonsiliasi, atau migrasi; RBAC dan isi file tidak berubah.

## Tasks

- [x] 1. `exportButtonState.ts`: pesan `dirty` jadi "Klik Search untuk memperbarui tabel sebelum mengekspor."; perbarui `exportButtonState.test.ts` dan `exportButtonState.property.test.ts` (Property 1)
  - _Requirements: 2.2, 4.2_
  - _Model: Haiku, Effort: Low — ganti satu string + ekspektasi test._

- [x] 2. `VendorSelect` tanpa kotak pencarian
  - [x] 2.1 `components/exportFormParts.tsx`: `VendorSelect` tinggal satu `FilterSelect` "Vendor" (`useVendorsList({ page: 1, page_size: 100, status: "active" })`, tanpa `q`); hapus input "Cari vendor", `useState`, `useId`, `INPUT_CLASS`; perbarui komentar
    - _Requirements: 1.1, 1.2, 1.3_
  - [x] 2.2 Test baru `components/exportFormParts.test.tsx`: Property 2 (fast-check daftar vendor 0..100 → satu combobox "Vendor", tanpa searchbox, opsi placeholder + `nama (kode)` berurutan, hook dipanggil tanpa `q`)
    - _Requirements: 1.1, 1.2_
  - _Model: Haiku, Effort: Low — menghapus kontrol, satu jalur jelas. Sub-task 2.2: Sonnet, Low — menyusun mock hook + generator._

- [x] 3. `components/PreviewFilterBar.tsx`: label tombol submit "Terapkan" → "Search"; `disabled={missingVendor}`, submit Enter, dan pengosongan cabang saat vendor dikosongkan tetap; sesuaikan komentar
  - _Requirements: 1.4, 2.1, 2.3_
  - _Model: Haiku, Effort: Low — ganti label._

- [x] 4. `components/PreviewExportBar.tsx`: isi tombol ikon `Download` + "Ekspor" tanpa angka; hapus `formatCount` dan paragraf "File diambil saat ekspor diproses; …"; prop `total` tetap untuk `exportButtonState`
  - _Requirements: 3.1, 4.1, 4.2_
  - _Model: Haiku, Effort: Low — hapus teks dan angka dari JSX._

- [x] 5. Perbarui `components/ExportPreviewPage.test.tsx`
  - Tombol "Search" (tanpa "Terapkan") menerapkan filter dan mereset ke halaman 1, Enter men-submit, nonaktif saat vendor wajib kosong
  - Tidak ada label "Cari vendor"; label Vendor, Cabang, Status, Tingkat harga tetap di Harga Paket; mengosongkan vendor menyembunyikan Cabang
  - Helper `exportButton` mencari tombol bernama "Ekspor" tanpa angka; "Total 1.240 baris" tetap di kepala halaman
  - Catatan pengambilan file tidak ada
  - Draft berubah → pesan "Klik Search …"; total 0 dan > 1.000.000 tetap menonaktifkan tombol
  - Paging, `RequestError`, dan dialog tetap lulus tanpa perubahan ekspektasi
  - _Requirements: 1.1, 1.3, 1.4, 2.1, 2.2, 2.3, 3.1, 4.1, 4.2, 4.3, 5.2_
  - _Model: Sonnet, Effort: Medium — beberapa ekspektasi berubah sekaligus; perlu memisahkan regresi asli dari teks yang memang diganti._

- [x] 6. Checkpoint: gate kualitas CompanyPortal
  - `pnpm --dir frontend/CompanyPortal-Vite run lint`, `run test:coverage` (84/80/80/85), `run build` + `check-bundle.mjs`
  - `git diff --stat` hanya menyentuh `frontend/CompanyPortal-Vite/src/features/exports/` (dan dokumen spec); tidak ada perubahan di `backend/`, `backend-exq/`, `pkg/`, `datasets.ts`, `routes/`, `package.json`
  - _Requirements: 5.1, 5.2, 5.3, 5.4_
  - _Model: Sonnet, Effort: Medium — triase kegagalan lint/coverage._

- [x] 7. Dokumen
  - `.kiro/steering/development-plan.md`: follow-up `export-preview-simplify` di bawah X.9 (D5) + status snapshot
  - `.claude/development-progress.md`: entri perubahan
  - `.kiro/specs/export-menu-revamp/requirements.md`: catatan di kepala bahwa Req 5.1 dan 5.9 digantikan `export-preview-simplify`
  - _Requirements: 3.1, 4.1_
  - _Model: Haiku, Effort: Low — catatan dokumen._

- [ ] 8. Cek browser manual (user, GR#10)
  - `/exports/vendor-pics`: hanya dropdown Vendor (semua vendor aktif tampil), Cabang muncul setelah vendor dipilih, tombol "Search", tombol "Ekspor" tanpa angka, "Total N baris" tetap, catatan pengambilan file hilang
  - `/exports/package-prices` dan `/exports/dsr-location-maps`: vendor wajib, Search nonaktif sebelum vendor dipilih
  - Ekspor satu data menghasilkan CSV seperti sebelumnya
  - Keyboard dan screen reader pada dropdown Vendor dan tombol Search
  - _Requirements: 1.1, 1.2, 1.3, 2.1, 2.3, 3.1, 4.1, 4.3, 5.3_
  - _Model: Haiku, Effort: Low — dilakukan user; agen hanya menyiapkan daftar cek._

## Notes

- Requirement ini menggantikan `export-menu-revamp` Req 5.1 dan 5.9. Property 9 `export-menu-revamp` (status tombol ekspor) tetap berlaku lewat Property 1 spec ini.
- Permintaan menggabungkan menu ATM dan Kelolaan ATM dihapus PO (D1); `datasets.ts`, katalog, dan rute tidak disentuh.
- Label "Search" berbahasa Inggris adalah keputusan PO (D3), bukan pelanggaran konvensi copy.

### Definition of Done

- Task 1-7 selesai, gate task 6 hijau, tidak ada perubahan backend atau dependency.
- Cek browser task 8 dilakukan user.

## Task Dependency Graph

- **Wave 0:** 1, 2.1, 3, 4 — tidak saling bergantung, file berbeda.
- **Wave 1:** 2.2 (butuh 2.1), 5 (butuh 1, 2.1, 3, 4).
- **Wave 2:** 6 (gate semua task kode dan test).
- **Wave 3:** 7, 8 (butuh 6; file berbeda).

```json
{
  "waves": [
    { "id": 0, "tasks": ["1", "2.1", "3", "4"] },
    { "id": 1, "tasks": ["2.2", "5"] },
    { "id": 2, "tasks": ["6"] },
    { "id": 3, "tasks": ["7", "8"] }
  ]
}
```
