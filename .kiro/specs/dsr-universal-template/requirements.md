# Requirements Document

> Status: accepted 2026-11-25 (PO)
## Introduction

**Masalah (kata pemohon, 2026-11-25).** Format laporan DSR berbeda-beda per vendor, dan format yang dipakai terlalu rumit. Sistem belum punya fitur untuk mengunduh format dokumen upload DSR. Vendor sudah menyetujui satu template universal "karena tidak ada pilihan lain" (konfirmasi PO, 2026-11-25).

**Outcome yang diinginkan.** Semua vendor mengunggah DSR memakai satu template universal yang bisa diunduh dari portal vendor, dan sistem menolak file yang menyimpang dari kontrak template dengan pesan yang jelas.

**Kondisi kode saat ini (dicek 2026-11-25):**
- Tidak ada fitur unduh template DSR. `DsrUploadDialog`/`DsrPage` (`frontend/VendorPortal-Vite/src/features/dsr/`) tidak punya tombol unduh. Tidak ada endpoint template DSR di `backend/`, `backend-exq/`, maupun `backend_python/service_dsr_etl/`. Satu-satunya template yang ada adalah template CSV master data (`GET /api/v1/exports/master-data/{entity}/template`, `backend-exq`).
- Parser `backend_python/dsr/dsr_etl.py` (`read_daily_rows`, `read_rencana_isi_rows`) membaca sheet `Daily` dan `Rencana Isi` secara heuristik (deteksi header denominasi, pencocokan teks label baris). Parser terbukti hanya pada dua file sampel (TAG JAKARTA `.xls`, BIJAK JAKARTA `.xlsx`).
- Pada file sampel TAG JAKARTA, 20 baris `Daily` berisi tanggal pada field `location` (bukan nama vault), dan sheet `Rencana Isi` terbaca 1 baris. Penyebab belum diselidiki (di luar ruang lingkup: file format lama ditolak, D2).
- Tabel penyimpanan sudah ada: `dsr_uploads`, `dsr_daily_rows`, `dsr_rencana_isi_rows` (baseline `001`), `dsr_location_vault_maps` (`027`), `dsr_receipts` (`033`). Tidak ada tabel atau kolom baru yang dibutuhkan.
- Alur upload: `/api/v1/dsr` (Go, `vendor-dsr`) menyimpan file lalu memanggil `service_dsr_etl` (`/process/dsr/dry-run` dan `/process/dsr/commit`). Ada konfirmasi vendor di antara dry-run dan commit, idempoten per checksum.
- Prototipe template sudah dibuat di luar spec: `backend_python/dsr/make_template.py` dan `backend_python/dsr/templates/Template_DSR_Universal.xlsx`. Terbukti terbaca parser (15 baris, 0 error) memakai file sampel berisi nilai literal. Prototipe ini menjadi dasar spec, bukan keputusan final.

**Ruang lingkup:**
- Template `.xlsx` universal (sheet `Daily`, `Rencana Isi`, `Petunjuk`) dengan tata letak tetap.
- Unduh template dari portal vendor.
- Validasi kontrak template pada tahap dry-run, dengan pesan error yang bisa ditindaklanjuti vendor.
- Penolakan file yang bukan Template_Universal, tanpa masa transisi (D2).
- Satu file DSR untuk satu vault (D5).

**Tidak termasuk:**
- Perubahan skema DB (struktur tabel atau kolom). Seed data pemetaan lokasi ke vault termasuk ruang lingkup (D11).
- Parser format baru (tabel datar) selain parser yang sudah ada.
- Perbaikan bug `location` berisi tanggal dan `Rencana Isi` 1 baris pada file TAG JAKARTA (format lama tidak diterima lagi).
- Perubahan alur konfirmasi, laporan keterlambatan DSR, atau pemetaan lokasi ke vault.

**Area sensitif (project-context Sec 3 rule 7):** parser memproses nominal uang (rupiah penuh, `Decimal`, tidak boleh float). Perubahan validasi tidak boleh mengubah hasil parse file yang sah.

**Keputusan PO:**
- **D1 ✅ DIPUTUSKAN PO 2026-11-25:** vendor menyetujui satu template universal.
- **D2 ✅ DIPUTUSKAN PO 2026-11-25:** tidak ada masa transisi. Semua file wajib memakai template baru karena aplikasi belum live.
- **D3 ✅ DIPUTUSKAN PO 2026-11-25:** file yang menyimpang dari TemplateContract ditolak dan vendor diberi alert.
- **D4 ✅ DIPUTUSKAN PO 2026-11-25:** vendor mengunduh template dari portal. Cara penyajian: mengikuti saran (Open Question 3).
- **D5 ✅ DIPUTUSKAN PO 2026-11-25:** satu file DSR untuk satu vault.
- **D6 ✅ DIPUTUSKAN PO 2026-11-25:** template tidak memakai penanda versi.
- **D7 ✅ DIPUTUSKAN PO 2026-11-25:** file yang rumusnya tidak punya nilai tersimpan diberi alert.
- **D8 ✅ DIPUTUSKAN PO 2026-11-25:** penyetuju spec adalah PO sendiri (pemohon).
- **D9 ✅ DIPUTUSKAN PO 2026-11-25:** template disajikan lewat endpoint GET /api/v1/dsr/template di backend Go, berkas di-embed (go:embed), dijaga permission endor-dsr.
- **D10 ✅ DIPUTUSKAN PO 2026-11-25:** template memuat satu sel wajib Vault (nilai = endor_vaults.vault_name) di header sheet Daily; parser memakainya sebagai location; sistem memvalidasinya terhadap vault milik vendor pengunggah.
- **D11 ✅ DIPUTUSKAN PO 2026-11-25 (opsi A):** pemetaan awal `dsr_location_vault_maps` (`dsr_location = vault_name`) untuk semua vault aktif diisi dengan seed data satu kali yang idempoten, ditambah warning `VAULT_NOT_MAPPED` pada dry-run bila pemetaan belum ada. Seed melewati maker-checker, diterima pada tahap greenfield (project-context Sec 0a).

## Open Questions

1. ✅ DIPUTUSKAN PO 2026-11-25 (D2): tidak ada masa transisi, file format lama ditolak.
2. ✅ DIPUTUSKAN PO 2026-11-25 (D3): file yang menyimpang ditolak dengan alert.
3. ✅ DIPUTUSKAN PO 2026-11-25 (D9, setuju dengan saran AI): endpoint API di backend Go `GET /api/v1/dsr/template`, berkas di-embed ke binary (`go:embed`), dijaga permission `vendor-dsr`. Alasan: sama-sama lewat autentikasi yang sudah ada, tidak butuh perubahan Nginx, dan berkas ikut versi rilis backend. Aset statis Nginx lebih sederhana tetapi terbuka tanpa login dan perubahan template harus mengubah image frontend.
4. ✅ DIPUTUSKAN PO 2026-11-25 (D5): satu file per vault.
5. ✅ DIPUTUSKAN PO 2026-11-25 (D6): tanpa penanda versi.
6. ✅ DIPUTUSKAN PO 2026-11-25 (D7): beri alert. Asumsi AI, disetujui PO: alert ini peringatan yang tidak memblokir konfirmasi, kecuali total yang terbaca 0 padahal ada nominal per denominasi (kasus ini ditolak).
7. ✅ DIPUTUSKAN PO 2026-11-25 (D8): PO sendiri. Belum disebut file nyata vendor untuk uji penerimaan; sampai ada, uji memakai file sintetis dari generator.
8. ✅ DIPUTUSKAN PO 2026-11-25 (D10, setuju dengan usulan AI): sel wajib "Vault" di header sheet `Daily`. Pertanyaan semula: dengan satu file per vault, dari mana sistem tahu file itu milik vault mana? Saat ini `dsr_daily_rows.location` diisi dari judul blok vault di file, lalu `dsr_receipts.locations` dan `dsr_location_vault_maps` memetakannya ke `vendor_vaults`. Template satu blok sekarang mengisi `location` kosong. Usulan AI: tambah satu sel wajib "Vault" di header sheet `Daily` yang berisi nama vault (`vendor_vaults.vault_name`), dipakai parser sebagai `location`, dan divalidasi terhadap vault milik vendor pengunggah. 

## Glossary

- **DSR**: Daily Status Report, laporan saldo harian vendor per ATM/vault.
- **Template_Universal**: berkas `Template_DSR_Universal.xlsx` dengan sheet `Daily`, `Rencana Isi`, `Petunjuk`.
- **DsrTemplateAPI**: endpoint backend yang menyajikan Template_Universal untuk diunduh.
- **DsrTemplateButton**: tombol unduh template pada halaman/dialog upload DSR di portal vendor.
- **DsrParser**: `backend_python/dsr/dsr_etl.py` (`read_daily_rows`, `read_rencana_isi_rows`).
- **DsrDryRun**: tahap baca-saja `/process/dsr/dry-run` yang menampilkan hasil parse untuk dikonfirmasi vendor sebelum disimpan.
- **TemplateContract**: aturan tata letak tetap Template_Universal (nama sheet, label baris, kolom denominasi, satuan rupiah penuh).
- **TemplateValidator**: logika yang memeriksa file unggahan terhadap TemplateContract pada DsrDryRun.
- **VendorUser**: pengguna portal vendor dengan permission `vendor-dsr`.
- **DsrLocationSeed**: skrip migrasi data satu kali yang mengisi pemetaan awal `dsr_location_vault_maps` dari `vendor_vaults.vault_name`.

## Requirements

### Requirement 1: Unduh template DSR dari portal vendor

**User Story:** Sebagai VendorUser, saya ingin mengunduh template DSR dari halaman upload, supaya saya mengisi format yang sama dengan yang diterima sistem.

#### Acceptance Criteria

1. WHEN VendorUser membuka dialog upload DSR, THE DsrTemplateButton SHALL menampilkan tombol "Unduh template".
2. WHEN VendorUser menekan DsrTemplateButton, THE DsrTemplateAPI SHALL mengembalikan Template_Universal sebagai berkas `.xlsx` dengan Content-Type `application/vnd.openxmlformats-officedocument.spreadsheetml.sheet` dan nama berkas `Template_DSR_Universal.xlsx`.
3. IF pemanggil tidak punya permission `vendor-dsr` atau tidak terautentikasi, THEN THE DsrTemplateAPI SHALL menolak dengan status 401 atau 403.
4. THE DsrTemplateAPI SHALL melayani unduhan tanpa query ke database dan tanpa menulis `audit_logs`.
5. IF berkas template tidak tersedia di server, THEN THE DsrTemplateAPI SHALL mengembalikan status 500 dengan pesan error umum dan tanpa membocorkan path server.
6. WHEN unduhan gagal, THE DsrTemplateButton SHALL menampilkan pesan error beserta ikon dan teks, bukan hanya warna.

### Requirement 2: Tata letak Template_Universal tetap

**User Story:** Sebagai petugas Cash Processing Center, saya ingin semua vendor mengisi tata letak yang sama, supaya data DSR bisa dibaca seragam.

#### Acceptance Criteria

1. THE Template_Universal SHALL memuat sheet `Daily`, `Rencana Isi`, dan `Petunjuk`, dengan nama sheet persis seperti itu.
2. THE Template_Universal SHALL menyatakan nominal dalam rupiah penuh dan memberi label header yang menyebutkan satuan itu, tanpa menyebut "x1.000".
3. THE Template_Universal SHALL menyediakan kolom nominal untuk tujuh denominasi: 100.000, 50.000, 20.000, 10.000, 5.000, 2.000, dan 1.000.
4. THE Template_Universal SHALL menandai sel input vendor dan sel rumus dengan pewarnaan berbeda yang dijelaskan di sheet `Petunjuk`.
5. THE Template_Universal SHALL berisi rumus untuk total per baris, subtotal penerimaan, subtotal pengeluaran, saldo akhir 00:00, saldo sementara 09:00, dan total status uang cadangan.
6. THE Template_Universal SHALL memuat sheet `Rencana Isi` dengan kolom ATM ID, Lokasi ATM, 50/100, Jumlah Isi Denom 100rb, Jumlah Isi Denom 50rb, Saldo Splank Pukul 08:00, dan Keterangan.
7. THE Template_Universal SHALL tidak memuat nama, nomor rekening, atau data nyata vendor mana pun.

### Requirement 3: Template_Universal terbaca oleh DsrParser

**User Story:** Sebagai engineer, saya ingin template terbukti terbaca parser, supaya file yang diisi sesuai petunjuk tidak ditolak.

#### Acceptance Criteria

1. WHEN Template_Universal yang diisi nilai literal dibaca oleh DsrParser, THE DsrParser SHALL menghasilkan 0 error dan mengisi metadata (perusahaan, tanggal laporan, perihal), tanggal status H+1, serta total saldo akhir 00:00, saldo sementara 09:00, dan status cadangan.
2. THE DsrParser SHALL memetakan setiap baris Daily ke `section` (`d0` atau `d1`) dan `flow` (`saldo_awal`, `penerimaan`, `pengeluaran`, `status_cadangan`) sesuai posisinya di Template_Universal.
3. WHEN Template_Universal kosong dibaca oleh DsrParser, THE DsrParser SHALL menghasilkan 0 error.
4. FOR ANY nominal per denominasi di Template_Universal, THE DsrParser SHALL menyimpannya sebagai `Decimal` rupiah penuh tanpa pembulatan dan tanpa skala x1.000.
5. WHEN file Template_Universal memuat sel `Vault` terisi, THE DsrParser SHALL menghasilkan satu baris `flow='saldo_akhir'`, `section='d0'` berisi nominal per denominasi dan total dari baris "SALDO AKHIR SAMPAI PUKUL 00:00", dengan `location` sama dengan `vault_name`, supaya alokasi vault (`vault_allocations`) tetap menerima saldo akhir per vault.

### Requirement 4: Validasi kontrak template saat dry-run

**User Story:** Sebagai VendorUser, saya ingin tahu persis apa yang salah pada file saya, supaya saya bisa memperbaikinya sebelum konfirmasi.

#### Acceptance Criteria

1. WHEN file unggahan tidak memuat sheet `Daily` atau `Rencana Isi`, THE TemplateValidator SHALL menolak file pada DsrDryRun dengan pesan berbahasa Indonesia yang menyebut nama sheet yang hilang.
2. WHEN file unggahan tidak memuat satu atau lebih label baris wajib TemplateContract, THE TemplateValidator SHALL melaporkan setiap label yang hilang pada DsrDryRun.
3. WHEN tanggal laporan atau Company kosong, THE TemplateValidator SHALL menolak file pada DsrDryRun dengan pesan yang menyebut field yang kosong.
4. WHEN sel nominal berisi teks yang bukan angka atau berisi error rumus (misalnya `#REF!`), THE TemplateValidator SHALL melaporkan sel tersebut dengan sheet, baris, dan kolomnya.
5. WHEN total per baris tidak sama dengan jumlah nominal per denominasi, THE TemplateValidator SHALL menolak file pada DsrDryRun dan melaporkan baris itu.
6. THE TemplateValidator SHALL melaporkan semua kesalahan yang ditemukan dalam satu hasil DsrDryRun, bukan hanya kesalahan pertama.
7. WHEN file lolos validasi, THE DsrDryRun SHALL menampilkan hasil baca yang sama dengan sebelum spec ini, dan alur konfirmasi serta commit SHALL tidak berubah.
8. WHEN file tidak sesuai TemplateContract, THE DsrDryRun SHALL menampilkan alert yang berisi daftar kesalahan dan tautan ke DsrTemplateButton, dan SHALL tidak menyediakan tombol konfirmasi.
9. WHEN sel total berupa rumus tanpa nilai tersimpan (file belum dibuka dan disimpan di Excel) dan nominal per denominasi pada baris itu terisi, THE TemplateValidator SHALL menampilkan alert peringatan pada DsrDryRun dan tetap mengizinkan konfirmasi, dengan total dihitung dari nominal per denominasi.
10. WHEN total yang terbaca 0 padahal nominal per denominasi pada baris itu tidak 0, THE TemplateValidator SHALL menolak file pada DsrDryRun.
11. THE TemplateValidator SHALL tidak menulis ke tabel `dsr_uploads`, `dsr_daily_rows`, `dsr_rencana_isi_rows`, maupun `dsr_receipts` selama DsrDryRun.

### Requirement 5: File format lama ditolak

**User Story:** Sebagai PO, saya ingin semua DSR memakai template baru sejak aplikasi live, supaya data seragam sejak awal.

#### Acceptance Criteria

1. WHEN file unggahan bukan Template_Universal (tidak memenuhi TemplateContract), THE TemplateValidator SHALL menolak file pada DsrDryRun dengan alert yang mengarahkan vendor ke DsrTemplateButton.
2. THE DsrParser SHALL tidak memiliki cabang pembacaan khusus per vendor.
3. THE DsrDryRun SHALL tidak mempertahankan jalur pemrosesan untuk file format lama (TAG JAKARTA `.xls`, BIJAK JAKARTA `.xlsx`) setelah spec ini selesai.

### Requirement 6: Satu file untuk satu vault

**User Story:** Sebagai VendorUser dengan beberapa vault, saya ingin tahu bahwa setiap vault dilaporkan dengan file terpisah.

#### Acceptance Criteria

1. THE Template_Universal SHALL menjelaskan di sheet `Petunjuk` bahwa satu file hanya untuk satu vault dan vendor dengan beberapa vault mengunggah satu file per vault.
2. WHEN file memuat lebih dari satu blok `SALDO HARIAN ATM`, THE TemplateValidator SHALL menolak file pada DsrDryRun.
3. THE Template_Universal SHALL memuat satu sel wajib berlabel `Vault` di header sheet `Daily` yang berisi `vendor_vaults.vault_name`, dan THE DsrParser SHALL memakai nilainya sebagai `location` seluruh baris Daily pada file itu (tabel Rencana Isi tidak punya kolom `location`).
4. IF sel `Vault` kosong atau berisi nilai yang bukan `vault_name` aktif milik vendor pengunggah, THEN THE TemplateValidator SHALL menolak file pada DsrDryRun dan menyebut nilai yang tidak dikenali.
5. WHEN `vault_name` valid tetapi belum ada baris aktif di `dsr_location_vault_maps` untuk vendor dan nama itu (tanpa membedakan huruf besar kecil), THE TemplateValidator SHALL menampilkan warning `VAULT_NOT_MAPPED` pada DsrDryRun dan tetap mengizinkan konfirmasi.

### Requirement 7: Tidak ada perubahan skema dan keamanan data

**User Story:** Sebagai engineer, saya ingin perubahan ini tidak menyentuh skema dan tidak membuka celah baru.

#### Acceptance Criteria

1. THE DsrTemplateAPI SHALL tidak memerlukan migrasi DB.
2. THE DsrTemplateAPI SHALL melayani hanya berkas Template_Universal yang tetap dan tidak menerima nama berkas atau path dari parameter request.
3. WHEN file yang sama (checksum sama) dikonfirmasi dua kali, THE DsrParser SHALL tetap idempoten seperti sebelum spec ini.
4. THE DsrTemplateAPI SHALL tidak mencatat isi berkas atau token ke log.

### Requirement 8: Pengujian

**User Story:** Sebagai engineer, saya ingin cakupan uji yang memastikan template dan parser tetap sinkron.

#### Acceptance Criteria

1. THE test suite SHALL memuat uji yang membangun Template_Universal dari generator, mengisinya dengan nilai literal, dan memastikan DsrParser membacanya tanpa error.
2. THE test suite SHALL memuat uji bahwa isi sel dan rumus berkas template yang di-embed sama dengan keluaran generator (mendeteksi template yang basi; perbandingan isi, bukan byte, karena metadata `.xlsx` berubah tiap dibuat).
3. THE test suite SHALL memuat uji TemplateValidator untuk setiap kondisi gagal pada Requirement 4.
4. THE test suite SHALL memuat uji DsrTemplateAPI untuk status 200, 401, dan 403 dan uji DsrTemplateButton untuk keadaan sukses dan gagal.
5. THE test suite SHALL memuat uji bahwa file sampel format lama (TAG JAKARTA dan BIJAK JAKARTA) ditolak oleh TemplateValidator.
6. THE test suite SHALL memuat uji DsrLocationSeed untuk idempotensi, tidak mengubah pemetaan yang ada, dan melewati vault tidak aktif, serta uji warning `VAULT_NOT_MAPPED`.

### Requirement 9: Pemetaan awal lokasi ke vault

**User Story:** Sebagai PO, saya ingin DSR yang diunggah langsung dihitung di laporan keterlambatan dan alokasi vault, supaya vendor tidak tampak terlambat hanya karena pemetaan belum dibuat.

#### Acceptance Criteria

1. THE DsrLocationSeed SHALL menambahkan satu baris aktif di `dsr_location_vault_maps` dengan `dsr_location` sama dengan `vault_name` untuk setiap vault aktif (pada cabang aktif) yang belum punya baris pemetaan aktif dengan label itu untuk vendor yang sama (tanpa membedakan huruf besar kecil).
2. THE DsrLocationSeed SHALL tidak mengubah, menonaktifkan, atau menghapus baris pemetaan yang sudah ada.
3. WHEN DsrLocationSeed dijalankan lebih dari sekali, THE DsrLocationSeed SHALL tidak menambah baris tambahan pada eksekusi berikutnya.
4. THE DsrLocationSeed SHALL tidak membuat pemetaan untuk vault yang tidak aktif.