# Review: Pilihan sumber paket pada Kelolaan ATM

Status: review pass **selesai 2026-10-07**. Stage: 5 Deploy. Reviewer: Claude (self-review; **bukan pengganti gerbang code owner** — merge tetap keputusan manusia).
Input: diff fitur (migrasi 023, `backend/`, `frontend/CompanyPortal-Vite/src/features/admin-atms/`) terhadap `spec.md` dan `plan.md`.
**Manual browser check: OUTSTANDING — tugas user** (Golden Rule #10), lihat `tests.md`.

## Ringkasan
Tidak ada temuan **Important** yang tersisa. Review menemukan 2 celah (I1, I2), keduanya sudah diperbaiki dan diuji dalam review ini. Sisanya nit/catatan untuk code owner.

## Temuan

| # | Tingkat | Temuan | Status |
|---|---|---|---|
| I1 | Important (cakupan) | Test integrasi awal hanya mencakup 6 pembaca. `CountForecastForDate`, `SummarizeForecastForDate` dan `GetActiveVendorForTerminal` (validasi vendor saat membuat Vendor Request) ikut diubah ke `LEFT JOIN`+`COALESCE` tetapi belum diuji untuk ATM mode vendor-wide. Jalur Applier (handover otomatis, update label, apply-time 409) juga belum diuji dengan DB asli. | **Diperbaiki**: `atm_assignment_vendor_apply_integration_test.go` (2 test). Hijau. |
| I2 | Important (integritas) | `ATMAssignmentApplier` `update` hanya memeriksa mode di service (saat submit). Payload basi atau buatan tangan bermode lain bisa mengubah baris `branch` menjadi `vendor-wide` (atau sebaliknya) lewat `UPDATE` — constraint `avp_source_chk` tetap valid untuk kedua bentuk, jadi DB tidak menolaknya. | **Diperbaiki**: Applier selalu memuat baris dan menolak `cur.Source != p.source()` dengan `ErrATMAssignmentSourceInvalid` (409). Diuji di `…_VendorWideAssignment_Applier`. |
| N1 | Nit (cakupan unit) | Pemetaan `List`/`Get` di service (`Source`, `VendorID`, `VendorBranchID`) 0% terukur. | **Diperbaiki**: `TestATMAssignmentAdminService_ListAndGet_MapBothSources`. |
| N2 | Nit | `pgCheckViolation` (23514) dipetakan ke `ErrATMAssignmentSourceInvalid` untuk **semua** CHECK di `atm_vendor_packages`. Dicek ke DB: satu-satunya CHECK di tabel itu adalah `avp_source_chk`, jadi benar hari ini. Bila kelak ada CHECK lain, pesan 409-nya menyesatkan. | Dibiarkan, tercatat. |
| N3 | Nit | File baru ditulis dengan akhir baris LF, sedangkan sebagian besar file Go repo CRLF (`gofmt -l` melaporkan file lama karena CR, bukan karena isi). | Dibiarkan; tidak memengaruhi build. |
| N4 | Nit (Sec 6) | `PackageOptions` dan `List` membaca dari pool primary, bukan replica. Konsisten dengan repo kelolaan yang sudah ada (TODO "swap to dbRead" di `atm_assignment_admin_repository.go`). | Dibiarkan; ikut TODO yang sama. |
| N5 | Catatan perilaku | Mode vendor-wide hanya untuk vendor `FLM_VENDOR` (vendor internal/ROH tidak punya tarif). ATM ROH tetap memakai paket internal mode `branch`. Sengaja (spec FR9.2). | Didokumentasikan. |
| N6 | Catatan perilaku | Validasi tarif menghitung baris tarif dengan `atm_id` NULL atau milik ATM itu sendiri; override ATM lain tidak dihitung. Spec hanya menyebut "baris tarif yang cocok", ini penajaman. | Didokumentasikan (`data-map.md`). |
| N7 | Risiko diterima | Header CSV ekspor kelolaan bertambah satu kolom (`package_source`) di tengah. Alat luar yang membaca CSV berdasarkan posisi kolom terpengaruh; impor menerima header lama. | Dicatat di `decisions.md`. |

## Review per sudut pandang

**Bug / korektness**
- `COALESCE(...)` di sqlc menghasilkan tipe non-null; kolom yang bisa NULL (ATM tanpa paket aktif) sengaja dipisah menjadi `package_code` + `vendor_package_label` (kuota) dan `paket` + `paket_vendor` (forecast), digabung di Go (`packageLabel`). Dibuktikan oleh test integrasi kuota/forecast lama yang tetap hijau (ATM tanpa paket) dan test baru (ATM vendor-wide).
- Periode otomatis memakai tanggal WIB (`assignmentAsOf`), diuji untuk pergantian hari UTC→WIB.
- Normalisasi `source` kosong = `branch` menjaga permintaan pending sebelum migrasi tetap valid (diuji).
- Race submit→apply: dijaga constraint DB (no-overlap lintas mode, unique parsial) dan validasi ulang di Applier.

**Keamanan**
- Semua query berparameter (sqlc); tidak ada SQL dari input. Label `package` hanya diterima bila cocok dengan tarif vendor (nilai bebas tidak bisa masuk).
- Endpoint baru berada di grup `RequireAuth` + `RequireRoles("ADMIN","ADMIN_PARAM")`; ada test 403 untuk non-admin. Respons hanya daftar label, tanpa harga.
- Tidak ada secret/kredensial baru; tidak ada perubahan auth. Maker-checker + audit tetap lewat `MasterDataChangeService.Submit`/Applier.
- Impor CSV: mode `vendor` memvalidasi vendor/cabang/label di dry-run, lalu `Create` melewati validasi FR9 penuh saat confirm; `package_source` tidak dapat diubah lewat impor.

**Kepatuhan spec** — semua FR1–FR12 dan NFR1–NFR5 terpetakan ke test di `tests.md`. Tidak ada kode yang menyimpang dari artefak; penyimpangan tunggal (ekspor/impor CSV dua mode) diputuskan user dan direvisi di `spec.md` (FR12) sebelum dibangun.

## Sec 11 Definition of Done
- [x] Sesuai requirement + peta modul/tabel (Sec 3 diperbarui; kolom diajukan lewat spec yang diterima)
- [x] Jalur auth benar, RBAC di route **dan** service (`Submit` tetap memeriksa role)
- [x] Maker-checker + `audit_logs` (lewat `MasterDataChangeService`/Applier, tidak berubah)
- [~] Baca di replica / tulis di primary: mengikuti pola repo yang sudah ada (baca di primary, TODO replica) — lihat N4
- [x] Uang numeric / timestamp timestamptz (tidak ada kolom uang baru; tarif hanya dibaca)
- [x] Test lulus termasuk RBAC denial dan overlap lintas mode (unit, integrasi `-tags integration`, frontend)
- [x] Tidak ada secret/konfigurasi hardcoded; tidak ada env baru, `.env.example` tidak perlu diubah
- [x] Bentuk respons mengikuti handler tetangga (JSON datar, `backend/internal/handler`)
- [ ] Build di Docker: **tidak dijalankan** di sesi ini (hanya `go build`/`pnpm build` lokal). Perlu CI/penguji.

## Hasil verifikasi akhir (2026-10-07, setelah perbaikan review)
- `go build ./... && go vet ./...` OK
- `go test -count=1 ./...` (`backend`, `backend-cit`, `pkg`) OK
- `DATABASE_URL=<dev> go test -count=1 -tags integration ./internal/...` OK; DB dev bersih (0 baris `ITEST-*`)
- Cakupan file baru: `atm_assignment_source.go` 78–100% per fungsi, `Create/Update/validate` 92–96%
- Frontend: `pnpm run test` 122 file / 1045 test, `lint` bersih, `build` OK (tidak ada perubahan frontend sejak `tests.md`)

## Untuk code owner sebelum merge
1. Verifikasi browser (user): skenario di `tests.md` → "Outstanding".
2. Perubahan fitur ini bercampur dengan banyak perubahan fitur lain yang belum di-commit di working tree; **pisahkan per fitur** saat commit (berkas fitur ini: migrasi 023, `atm_assign*`/`masterdata_import*`/`masterdata_export*`/handler & repo kelolaan, `queries/*` yang disebut di plan, `features/admin-atms/`, dokumen `.claude/sdlc/atm-package-source/` + pembaruan `CLAUDE.md`/`data-map`/`decisions`/README SDLC/`development-progress`).
3. Query pembaca lain (`atms_admin`, `atm_visit_quota`, `vendor_request`) juga berstatus modifikasi dari fitur lain (`atm-visit-quota` belum di-commit); hunk fitur ini hanya bagian `COALESCE`/`LEFT JOIN` + kolom label.
4. Terapkan migrasi 023 ke lingkungan lain dengan `psql -f` (additive; rollback di akhir berkas).
