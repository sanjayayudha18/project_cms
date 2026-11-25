# Implementation Plan: DSR Universal Template

> Status: accepted 2026-11-25 (PO)
## Overview

Urutan implementasi mengikuti design: kontrak dan generator template lebih dulu, lalu perubahan parser, lalu validator, lalu integrasi ke dry-run dan commit. Endpoint unduh Go, seed pemetaan, dan frontend berjalan paralel karena tidak berbagi berkas dengan jalur Python.

Pola yang dirujuk: dry-run/confirm yang sudah ada (`backend_python/dsr/dsr_etl.py`, `backend/internal/handler/dsr_upload_handler.go`, `DsrUploadDialog.tsx`), migrasi berurut yang dijalankan lewat `go run ./cmd/migrate up`, dan pola `go:embed` untuk berkas statis.

Prasyarat yang sudah dicek (2026-11-25): `openpyxl==3.1.5` dan `xlrd==2.0.2` sudah ada di `backend_python/dsr/requirements.txt`, jadi tidak ada dependensi baru (Golden Rule #1). Nomor migrasi `048` belum dipakai (terakhir `047_role_portal.sql`); cek ulang sebelum membuat berkas.

### Model Split

**Sonnet-runnable**
- 1. Kontrak template dan generator — Sonnet, Medium
  - 1.1 Modul kontrak (konstanta dan tipe) — Sonnet, Medium
  - 1.2 Generator template — Sonnet, Medium (needs 1.1)
  - 1.3 Tes generator dan template basi — Sonnet, Medium (needs 1.2)
- 2.2 Tes parser (Property 1) — Sonnet, Medium (needs Opus 2.1 dan 1.2)
- 3.4 Tes validator (Property 1, 3, 5, 6, 8) — Sonnet, Medium (needs Opus 3.3)
- 4.2 Tes dry-run dan commit konsisten (Property 2) — Sonnet, Medium (needs Opus 4.1)
- 5. Endpoint unduh template (Go) — Sonnet, Medium (5.1 needs 1.2)
  - 5.1 Paket `dsrtemplate` dan embed
  - 5.2 Handler `Template`
  - 5.3 Tes handler (Property 7)
- 6.2 Tes seed pemetaan — Sonnet, Medium (needs Opus 6.1)
- 7. Frontend: unduh template dan alert validasi — Sonnet, Medium
  - 7.1 API client dan tipe
  - 7.2 `DsrTemplateButton`
  - 7.3 Alert di `DsrUploadDialog`
  - 7.4 Tes frontend
- 8. Checkpoint: build, lint, dan semua tes — Sonnet, Medium (needs semua task kode dan tes)
- 9. Dokumentasi dan graph — Haiku, Low (Haiku) (needs 8)
- 10. Cek browser manual — Haiku, Low (Haiku) (dikerjakan user, needs 8)

**Opus-required**
- 2.1 Perubahan parser: field `vault` dan baris `saldo_akhir` — Opus, High — uang dan input alokasi vault; salah parse mengubah kapasitas vault
- 3.1, 3.2, 3.3 Validator `read_validated` — Opus, High — integritas total dan nominal rupiah, penolakan file; salah aturan menolak DSR sah atau menerima DSR salah
- 4.1 Integrasi dry-run dan commit — Opus, High — gerbang commit, idempotensi, dan jalur yang menulis `dsr_uploads`/`dsr_receipts`
- 6.1 Migrasi seed `048_dsr_location_seed.sql` — Opus, High — migrasi data pada data live; menentukan vault mana yang dihitung di laporan keterlambatan (dasar penalti FLM)

Dependensi lintas grup: Opus 2.1 dan Sonnet 1.1 harus selesai sebelum Opus 3.1. Sonnet 1.2 harus selesai sebelum Sonnet 2.2 dan Sonnet 5.1. Opus 3.3 harus selesai sebelum Opus 4.1 dan Sonnet 3.4.

## Tasks

- [ ] 1. Kontrak template dan generator
  - Satu sumber kebenaran untuk label baris, dipakai generator dan validator.
  - _Requirements: 2.1, 2.2, 2.3, 2.4, 2.5, 2.6, 2.7, 3.3, 4.2, 6.1, 6.3, 8.2_
  - _Model: Sonnet, Effort: Medium — pola sudah jelas dari prototipe; tidak menyentuh uang secara langsung._
  - [ ] 1.1 Buat `backend_python/dsr/template_contract.py` berisi konstanta `REQUIRED_META`, `DAILY_LINES`, `RENCANA_HEADERS` dan dataclass `Issue` dan `Validated` (belum ada logika validasi)
    - _Requirements: 2.1, 2.3, 2.5, 2.6, 4.2_
  - [ ] 1.2 Ubah `make_template.py`: ambil label dari kontrak, tambah sel `Vault` (`D7` label, `E7` input), perbarui sheet `Petunjuk` (satu file satu vault, isi `Vault` persis `vault_name`), ubah `OUT` ke `backend/internal/service/dsrtemplate/Template_DSR_Universal.xlsx`, hapus folder prototipe `backend_python/dsr/templates/`, lalu hasilkan berkas
    - _Requirements: 2.1, 2.2, 2.3, 2.4, 2.5, 2.6, 2.7, 6.1, 6.3_
  - [ ] 1.3 Tes generator di `backend_python/dsr/test_make_template.py` (Property 4): setiap label di `DAILY_LINES` dan `RENCANA_HEADERS` ada di berkas; berkas yang di-commit sama isi sel dan rumusnya dengan keluaran generator (bukan byte); tidak ada data nyata vendor
    - _Requirements: 2.1, 2.3, 2.5, 2.6, 2.7, 3.3, 8.2_
    - _Model: Sonnet, Effort: Medium — tes struktural._

- [ ] 2. Perubahan parser untuk file satu vault
  - Hanya aktif bila header memuat `Vault`; tidak boleh mengubah hasil parse lain.
  - _Requirements: 3.1, 3.2, 3.4, 3.5, 6.3, 8.1_
  - _Model: Opus, Effort: High — parser nominal rupiah; baris `saldo_akhir` menjadi sumber kapasitas vault di `vault_allocations`._
  - [ ] 2.1 Ubah `dsr_etl.py`: tambah `"vault": "vault"` ke `HEADER_FIELD_KEYS`; pada cabang `SALDO AKHIR` untuk blok tanpa nama, keluarkan baris `flow='saldo_akhir'`, `section='d0'` (denominasi dan total) bila `file_fields.get("vault")` terisi, sambil tetap mengisi `saldo_akhir_0000_total_idr`
    - _Requirements: 3.2, 3.5, 6.3_
  - [ ] 2.2 Tes parser (Property 1) di `backend_python/dsr/test_template_parser.py`: nominal acak (integer dan dua desimal, seed tetap) terbaca sebagai `Decimal` yang sama; mapping `(section, flow, label)` benar; baris `saldo_akhir` ada dan totalnya sama dengan hitungan manual; template terisi menghasilkan 0 error
    - _Requirements: 3.1, 3.2, 3.4, 3.5, 8.1_
    - _Model: Sonnet, Effort: Medium — tes di atas perilaku yang sudah ditentukan task 2.1._

- [ ] 3. Validator kontrak `read_validated`
  - Mengumpulkan semua `Issue` dalam satu hasil; tidak berhenti di kesalahan pertama.
  - _Requirements: 3.1, 3.3, 3.4, 4.1, 4.2, 4.3, 4.4, 4.5, 4.6, 4.9, 4.10, 5.1, 5.2, 6.2, 6.3, 6.4, 6.5, 8.1, 8.3, 8.5, 8.6_
  - _Model: Opus, Effort: High — aturan penolakan atas data uang; kesalahan menolak DSR yang sah atau meloloskan DSR yang salah._
  - [ ] 3.1 Kerangka `read_validated(path, vendor_id, conn)`: cek sheet `Daily` dan `Rencana Isi` lewat `pd.ExcelFile(...).sheet_names`; bungkus pemanggilan parser sehingga exception menjadi `Issue`; kembalikan `Validated`; tanpa cabang khusus per vendor
    - _Requirements: 4.1, 4.6, 5.2_
  - [ ] 3.2 Tambah pemeriksaan meta (`company`, `report_date`, `vault`), `MULTI_BLOCK` (lebih dari satu blok `SALDO HARIAN ATM`), pencocokan `Vault` ke `vendor_vaults.vault_name` aktif milik vendor (query read-only; `location` diisi nama kanonik dari DB), label wajib `DAILY_LINES` tepat satu kali, header `Rencana Isi` dan tanggal rencana, dan warning `VAULT_NOT_MAPPED`
    - _Requirements: 4.2, 4.3, 4.6, 5.1, 6.2, 6.3, 6.4, 6.5_
  - [ ] 3.3 Tambah pemeriksaan sel nominal (`CELL_INVALID` dengan sheet, baris, kolom), total baris (`TOTAL_MISMATCH`, `TOTAL_ZERO`), dan deteksi rumus tanpa nilai tersimpan pada `.xlsx` (buka workbook dengan `data_only=False` dan `True`) yang menghasilkan warning `FORMULA_NOT_CALCULATED` dan total dihitung dari jumlah denominasi; prioritas: rumus tanpa cache dicek dulu
    - _Requirements: 3.4, 4.4, 4.5, 4.6, 4.9, 4.10_
  - [ ] 3.4 Tes validator di `backend_python/dsr/test_template_contract.py` dengan `conn` palsu (Property 1, 3, 5, 6, 8): satu kasus untuk setiap kondisi gagal Requirement 4; mutasi k pelanggaran terpisah menghasilkan k error; total sama, beda, nol, dan rumus tanpa cache; satu blok, banyak blok, `Vault` kosong, `Vault` milik vendor lain, vault belum dipetakan; dua file sampel format lama (TAG JAKARTA, BIJAK JAKARTA di `FTP_DATA/DSR_DATA`, skip bila berkas tidak ada) menghasilkan error
    - _Requirements: 3.1, 3.3, 4.1, 4.2, 4.3, 4.4, 4.5, 4.6, 4.9, 4.10, 5.1, 6.2, 6.3, 6.4, 6.5, 8.1, 8.3, 8.5, 8.6_
    - _Model: Sonnet, Effort: Medium — tes mengikuti daftar kondisi di design; tidak menentukan aturan._

- [ ] 4. Integrasi ke dry-run dan commit
  - Satu jalur `read_validated` untuk keduanya; commit menolak file ber-error.
  - _Requirements: 4.7, 4.8, 4.11, 5.1, 5.3, 7.3_
  - _Model: Opus, Effort: High — gerbang commit ke `dsr_uploads`/`dsr_receipts` dan idempotensi per checksum._
  - [ ] 4.1 Ubah `dry_run_file` memakai `read_validated` dan menambah `result["validation"] = {"errors": [...], "warnings": [...]}`; ubah `_upsert_upload` memakai `read_validated` yang sama dan melempar `TemplateRejected` sebelum penulisan bila `errors` tidak kosong; hapus pemanggilan langsung ke parser dari kedua jalur; dry-run tidak menulis ke tabel DSR mana pun
    - _Requirements: 4.7, 4.8, 4.11, 5.1, 5.3, 7.3_
  - [ ] 4.2 Tes konsistensi (Property 2) di `test_template_contract.py` dan `test_dsr_receipts_db.py`: hasil dry-run dan commit identik untuk file yang sama; commit menolak file ber-error tanpa menulis baris; commit file yang sama dua kali tetap idempoten per checksum (tes DB di-skip bila `DATABASE_URL` kosong)
    - _Requirements: 4.7, 4.8, 4.11, 7.3, 8.3_
    - _Model: Sonnet, Effort: Medium — tes integrasi di atas perilaku task 4.1._

- [ ] 5. Endpoint unduh template (Go)
  - _Requirements: 1.2, 1.3, 1.4, 1.5, 7.1, 7.2, 7.4, 8.4_
  - _Model: Sonnet, Effort: Medium — handler kecil mengikuti pola handler DSR yang ada._
  - [ ] 5.1 Buat paket `backend/internal/service/dsrtemplate` dengan `embed.go` (`//go:embed Template_DSR_Universal.xlsx`, `var Bytes []byte`) memakai berkas dari task 1.2
    - _Requirements: 1.2, 7.1, 7.2_
  - [ ] 5.2 Tambah rute `GET /template` di `DsrUploadHandler.Routes()` dan handler `Template`: tolak bila `authCtx.VendorID == nil` (403), tanpa `vendorContext()` (tanpa query DB) dan tanpa `audit_logs`; kirim `Bytes` dengan `Content-Type` xlsx, `Content-Disposition: attachment; filename="Template_DSR_Universal.xlsx"`, `X-Content-Type-Options: nosniff`, `Cache-Control: no-store`; bila `Bytes` kosong kembalikan 500 `internal_error` tanpa menyebut path; tidak ada log isi atau token
    - _Requirements: 1.2, 1.3, 1.4, 1.5, 7.2, 7.4_
  - [ ] 5.3 Tes handler (Property 7) di `backend/internal/handler/dsr_upload_handler_template_test.go`: 200 dengan byte sama dengan `dsrtemplate.Bytes` dan header yang benar; 401 tanpa token; 403 tanpa `VendorID`; 500 saat `Bytes` kosong; query string tidak mengubah respons; tidak ada panggilan repository
    - _Requirements: 1.2, 1.3, 1.4, 1.5, 8.4_

- [ ] 6. Seed pemetaan lokasi ke vault
  - _Requirements: 8.6, 9.1, 9.2, 9.3, 9.4_
  - _Model: Opus, Effort: High — migrasi data pada data live; menentukan vault yang dihitung di laporan keterlambatan DSR (dasar penalti FLM)._
  - [ ] 6.1 Buat `backend/migrations/048_dsr_location_seed.sql` (cek nomor dulu): `INSERT ... SELECT` satu baris per vault aktif (cabang aktif) ke `dsr_location_vault_maps` dengan `dsr_location = btrim(vault_name)`, hanya bila belum ada baris aktif untuk `(vendor_id, lower(label))` (`NOT EXISTS`); dalam transaksi; komentar rollback di berkas; tanpa DDL dan tanpa menyentuh baris lama
    - _Requirements: 9.1, 9.2, 9.3, 9.4_
  - [ ] 6.2 Tes seed (integrasi DB, skip bila `DATABASE_URL` kosong): idempoten pada eksekusi kedua, pemetaan lama tidak berubah atau nonaktif, vault tidak aktif dilewati, vault dengan pemetaan beda huruf besar kecil tidak diduplikasi
    - _Requirements: 8.6, 9.1, 9.2, 9.3, 9.4_
    - _Model: Sonnet, Effort: Medium — tes SQL idempoten mengikuti aturan di task 6.1._

- [ ] 7. Frontend: unduh template dan alert validasi
  - _Requirements: 1.1, 1.6, 4.8, 4.9, 5.1, 6.5, 8.4_
  - _Model: Sonnet, Effort: Medium — komponen dan alur dialog mengikuti pola `DsrUploadDialog` yang ada._
  - [ ] 7.1 Di `dsrUploadApi.ts`: tambah `downloadDsrTemplate()` (fetch dengan token, hasil `Blob`, unduh lewat anchor sementara bernama `Template_DSR_Universal.xlsx`; tambahkan helper blob tipis di client bila client hanya mengurai JSON) dan tipe `validation?: { errors: Issue[]; warnings: Issue[] }` pada `DsrDryRunResponse`
    - _Requirements: 1.2, 4.8_
  - [ ] 7.2 Buat `DsrTemplateButton.tsx`: tombol "Unduh template", keadaan memuat, dan pesan error dengan ikon dan teks (bukan hanya warna)
    - _Requirements: 1.1, 1.6_
  - [ ] 7.3 Ubah `DsrUploadDialog.tsx`: pasang `DsrTemplateButton` di tahap `pick` dan di tampilan penolakan; bila `validation.errors` tidak kosong tampilkan alert berisi daftar error dan tautan unduh, tanpa tombol "Konfirmasi & Simpan"; bila hanya `warnings` tampilkan alert kuning dan pertahankan tombol; tanpa `validation` perilaku tetap; hapus kalimat "sel bermasalah tetap bisa disimpan" untuk template
    - _Requirements: 1.1, 4.8, 4.9, 5.1, 6.5_
  - [ ] 7.4 Tes Vitest: `DsrTemplateButton` (sukses, gagal), dialog dengan `errors` (alert tampil, tombol konfirmasi tidak ada), dengan `warnings` saja (tombol ada), tanpa `validation` (perilaku lama); `pnpm run test` dan `lint` hijau
    - _Requirements: 1.1, 1.6, 4.8, 4.9, 8.4_

- [ ] 8. Checkpoint: build, lint, dan semua tes
  - Jalankan: `python -m unittest discover -s dsr` (dari `backend_python/`), `go test ./...` dan `golangci-lint run --build-tags integration ./...` (dari `backend/`), `pnpm --dir frontend/VendorPortal-Vite run test` dan `run lint`, `go run ./cmd/migrate status` (tidak ada MISMATCH). Catat hasil di `### Test Results` pada bagian Notes. Cek tidak ada migrasi struktur (R7.1).
  - _Requirements: 7.1, 8.1, 8.2, 8.3, 8.4, 8.5, 8.6_
  - _Model: Sonnet, Effort: Medium — triase kegagalan tes lintas bahasa; tanpa penalaran mendalam._

- [ ] 9. Dokumentasi dan graph
  - Perbarui `.claude/development-progress.md` dan catatan terkait (format lama tidak diterima, template universal, satu file satu vault, pemetaan seed); jalankan `graphify update .`.
  - _Requirements: 5.1, 7.1_
  - _Model: Haiku, Effort: Low — edit dokumen dan perintah satu langkah._

- [ ] 10. Cek browser manual (dikerjakan user)
  - Unduh template dari portal vendor, isi, unggah, dan lihat alert untuk: file benar, vault salah, total tidak cocok, dan file format lama. AI tidak menandai task ini selesai.
  - _Requirements: 1.1, 1.2, 4.8, 4.9, 5.1, 6.4_
  - _Model: Haiku, Effort: Low — daftar cek manual tanpa kode._

## Notes

### Asumsi dan risiko

- **Pemetaan vault baru.** Seed hanya mengisi vault yang sudah ada saat migrasi dijalankan. Vault baru tanpa pemetaan memunculkan warning `VAULT_NOT_MAPPED` di dry-run, dan admin harus membuat pemetaannya lewat layar yang ada. Pemetaan otomatis saat vault dibuat berada di luar spec ini.
- **Rumus tanpa nilai tersimpan** hanya dideteksi untuk `.xlsx` (openpyxl). File `.xls` tidak punya jalur ini; template resmi berformat `.xlsx`.
- **Berkas ditolak tetap di-staging** di `pending_confirmation` seperti dry-run yang ditinggalkan vendor; tidak ada jalur pembersihan baru.
- **Nomor migrasi** `048` harus dicek ulang sebelum task 6.1.
- Bila `openpyxl` atau `xlrd` ternyata tidak tersedia di lingkungan service `service_dsr_etl`, STOP dan minta persetujuan sebelum menambah dependensi.

### Definition of Done

- [ ] Sesuai requirement dan tidak ada tabel atau kolom baru; satu migrasi data saja (048)
- [ ] Jalur auth benar (`vendor-dsr`, `VendorID` ada) untuk endpoint template
- [ ] Nominal sebagai `Decimal`/numeric, tidak ada float
- [ ] Dry-run tidak menulis ke tabel DSR; commit menolak file ber-error
- [ ] Semua tes lulus (Python, Go, Vitest), lint bersih
- [ ] Tidak ada secret atau path server di respons dan log
- [ ] Cek browser manual selesai oleh user (task 10)

### Test Results

Belum dijalankan. Diisi pada task 8 dengan perintah, tanggal, angka lulus dan cakupan, serta jejak ke Requirement dan Property.

## Task Dependency Graph

**Wave 0:** 1.1 (kontrak), 2.1 (parser), 6.1 (migrasi seed), 7.1 (API client frontend). Tidak berbagi berkas.
**Wave 1:** 1.2 (generator, needs 1.1), 6.2 (tes seed, needs 6.1), 7.2 (tombol, needs 7.1).
**Wave 2:** 1.3 (tes generator, needs 1.2), 2.2 (tes parser, needs 2.1 dan 1.2), 3.1 (kerangka validator, needs 1.1 dan 2.1), 5.1 (embed, needs 1.2), 7.3 (dialog, needs 7.1 dan 7.2).
**Wave 3:** 3.2 (validator, needs 3.1), 5.2 (handler, needs 5.1), 7.4 (tes FE, needs 7.3).
**Wave 4:** 3.3 (validator total, needs 3.2), 5.3 (tes handler, needs 5.2).
**Wave 5:** 3.4 (tes validator, needs 3.3), 4.1 (integrasi, needs 3.3).
**Wave 6:** 4.2 (tes konsistensi, needs 4.1).
**Wave 7:** 8 (checkpoint, needs semua task kode dan tes).
**Wave 8:** 9 (dokumentasi, needs 8), 10 (cek browser manual, needs 8).

```json
{
  "waves": [
    { "id": 0, "tasks": ["1.1", "2.1", "6.1", "7.1"] },
    { "id": 1, "tasks": ["1.2", "6.2", "7.2"] },
    { "id": 2, "tasks": ["1.3", "2.2", "3.1", "5.1", "7.3"] },
    { "id": 3, "tasks": ["3.2", "5.2", "7.4"] },
    { "id": 4, "tasks": ["3.3", "5.3"] },
    { "id": 5, "tasks": ["3.4", "4.1"] },
    { "id": 6, "tasks": ["4.2"] },
    { "id": 7, "tasks": ["8"] },
    { "id": 8, "tasks": ["9", "10"] }
  ]
}
```
