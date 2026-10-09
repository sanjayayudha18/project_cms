# Bug Fix Log

Small fixes that the user decided don't need a full AI-DLC chain (CLAUDE.md Sec 4a rule 0). Newest first. One entry per fix.

Template:

```markdown
## YYYY-MM-DD — <short title>
- **Symptom**: what the user saw
- **Root cause**: why it happened
- **Fix**: what changed (files)
- **Tests**: automated check added/run; manual browser verification: outstanding
- **Commit**: <sha>
```

---

## 2026-10-09 — QC F15: context build Docker backend membawa seluruh repo + `.env`
- **Symptom**: compose kedua backend memakai `context: ..` (root repo), tapi tidak ada `.dockerignore` di root; `backend/.dockerignore` tidak pernah dibaca Docker (Docker hanya membaca yang ada di root context). Seluruh repo ikut jadi context, dan `COPY backend/` membawa `backend/.env` ke stage builder (tidak ke image final, tapi ada di cache layer builder). CLAUDE.md Sec 9 menyebut root `docker-compose.yml` yang tidak ada.
- **Root cause**: `.dockerignore` ditaruh per folder backend, padahal context build dipindah ke root saat split `pkg/` + `go.work`.
- **Fix**: `.dockerignore` root berbentuk allowlist (`go.work`, `go.work.sum`, `pkg/`, `backend/`, `backend-cit/`) + exclude `**/.env`, `**/.env.*`, `**/bin/`, `*.exe`, `*.test`, `*.out`, `*.md`. `backend/.dockerignore` (mati) dihapus. CLAUDE.md Sec 9: teks compose + dockerignore diperbaiki. Frontend tidak terpengaruh (context folder sendiri).
- **Tests**: Dockerfile probe (`COPY . /ctx`) dari working tree yang berisi `.env` asli → context 8.1MB, hanya `backend/ backend-cit/ go.work go.work.sum pkg/`, tanpa `.env*` dan `*.md`. `docker build` kedua backend berhasil. Manual browser verification: tidak relevan.
- **Commit**: _(belum)_

---

## 2026-10-09 — QC F14: `go.work` di-gitignore padahal di-COPY Dockerfile
- **Symptom**: `backend/Dockerfile` & `backend-cit/Dockerfile` menjalankan `COPY go.work go.work.sum ./`, tetapi kedua file di-gitignore → `docker build` dari clone bersih (CI/Cloud Build) gagal; hanya jalan di mesin dev yang kebetulan punya file lokal.
- **Root cause**: `.gitignore` memakai default template Go ("go.work tidak di-commit"), sedangkan repo ini sengaja memakai workspace (CLAUDE.md Sec 3).
- **Fix**: `.gitignore` tidak lagi mengabaikan `go.work`/`go.work.sum`; keduanya di-commit (`go.work` berisi `toolchain go1.26.9` dari F12).
- **Tests**: export index git (`git checkout-index`, setara clone bersih) → `docker build -f backend/Dockerfile .` dan `-f backend-cit/Dockerfile .` berhasil (67.1MB / 37.7MB). Tidak ada perubahan kode; manual browser verification: tidak relevan.
- **Commit**: `21f8cff`

---

## 2026-10-09 — QC F12: dependency + toolchain Go yang kena CVE
- **Symptom**: `govulncheck` (QC F2) melaporkan CVE yang reachable: chi v5.2.1 (GO-2025-3770 open redirect `RedirectSlashes`, GO-2026-5777), pgx v5.7.4 (GO-2026-5004), `golang.org/x/text` v0.40.0 (GO-2026-6629), dan 11 advisory stdlib (crypto/tls, x509, asn1, xml, net/http, textproto, url) di go1.26.3 lokal. Dockerfile memakai `golang:1.25-alpine` — seri 1.25 sudah tidak didukung (rilis yang didukung: 1.26.9, 1.27.2), beberapa CVE tidak punya fix di 1.25.
- **Root cause**: versi dependency tidak pernah di-scan/di-upgrade; image builder memakai tag mengambang di seri yang sudah EOL.
- **Fix**: `backend/` + `backend-cit/` go.mod/go.sum: chi v5.3.0, pgx v5.9.2, x/text v0.41.0 (`go mod tidy` juga menghapus require indirect testify/x/crypto yang tak terpakai di backend-cit). `toolchain go1.26.9` di `go.mod` ketiga modul (+ `go.work` lokal; `go.work` di-gitignore). `go` directive tetap `1.25.0`. Dockerfile `backend/` & `backend-cit/` → `golang:1.26.9-alpine`.
- **Tests**: govulncheck 3 modul dengan go1.26.9 → "No vulnerabilities found". build + vet + `go test ./...` (3 modul) + `go test -tags integration -p 1 ./internal/...` (dev DB) lulus. Docker build diverifikasi di F14. Tidak ada UI berubah; manual browser verification: outstanding (smoke login + 1 halaman admin).
- **Catatan**: chi v5.3 menandai `middleware.RealIP` deprecated (IP spoofing) → lint SA1019 di `cmd/api/main.go` kedua backend; ditangani terpisah sebagai QC F13 (auth, Sec 4 #7).
- **Commit**: `9f44cf0`

---

## 2026-10-08 — Import CSV cabang vendor selalu gagal "category: harus ATM, CASH, atau ATM_CASH"
- **Symptom**: import master-data `vendor-branches` (create maupun update) ditolak per baris dengan error category, walaupun file hasil export sendiri.
- **Root cause**: `stageCreate`/`stageUpdate` (`masterdata_import_confirm.go`) membangun `VendorBranchPayload`/`VendorBranchUpdatePayload` tanpa `Category`, sedangkan `VendorBranchAdminService.Create/Update` mewajibkan category. Export `vendor-branches` juga tidak punya kolom `category`, jadi round-trip tidak bisa membawanya.
- **Fix**: query `ExportVendorBranchesBatch` (`backend/queries/master_data_export.sql` + sqlc regen) + header export kini punya `category` (setelah `region_code`). Importer: `category` divalidasi (uppercase, ATM/CASH/ATM_CASH) di `validateFields` dan diteruskan di stageCreate/stageUpdate. File lama tanpa `category` (dengan atau tanpa `region_code`) tetap diterima: update mempertahankan category (dan region_code) saat ini, create → error baris `category`. Mekanisme legacy header digeneralisasi: `exportSpec.legacyHeaders [][]string` + `legacyFill map[kolom]nilai` (sebelumnya satu header kurang satu kolom); sentinel `keepRegionCode` → `keepCurrentCell`.
- **Tests**: `TestImport_VendorBranchCategory` (normalisasi, wajib saat create, invalid, dua bentuk file lama) + `TestImport_VendorBranchStagesCategory` (dry-run → stageRow lewat service asli; gagal tanpa fix), `TestExport_HeaderContract` diperbarui. `go test ./...` + `go test -tags integration ./...` (dev DB localhost) lulus. Manual browser verification: outstanding (export → import cabang vendor di halaman admin).
- **Commit**: _(belum)_

---

## 2026-10-07 — Phase 0.1: read-replica routing selesai (Go + Python)
- **Symptom**: read untuk list/viewer/laporan masih ke primary (TODO `ponytail: swap dbPool for the dbRead pool` di `cmd/api/main.go`); service EOD Python tidak punya pool replica (deferred dari import-export-jobs C2).
- **Root cause**: pool replica baru dipakai export master-data, region, dan branch-ATM; repo lain belum dipisah.
- **Fix**: Go — `main.go` memakai `dbReadPool` untuk ATM Portal, DMAA Forecast viewer, Role Management (list/catalog/permission, sesuai spec Req 8), RBAC list views, Audit Log viewer. `VendorAdminRepository`/`ATMAdminRepository` jadi `(primary, replica)`: List/Count (+ ATM ListLocations) ke replica; GetByID/pre-check tetap primary (snapshot "before" saat Submit). Sengaja **tetap primary**: DSR upload (read-after-write hasil commit Python), `MasterDataChangeRepository` List/Count (badge pending dibaca tepat setelah Submit). Python — `lib/database.create_read_pool` (`*_DATABASE_REPLICA_URL`, kosong = primary) → `app.state.db_read_pool` dipakai `/status`, `/status/{id}/history`, `/summary`, `/late`, `/audit` di `eod_retry_scheduler` + `service_dsr_etl`; `/health` + proses/retry tetap primary. `.env.example` keduanya ditambah.
- **Tests**: `go test ./...` + `go test -tags integration ./internal/...` (dev DB) lulus; `rolemgmt` `TestRepository_WriteReadTopology` lulus; Python `lib.test_eod_api` + `lib.test_import_jobs` (34, dev DB), `dsr`, `itm/cashpos`, `python -m lib.database` (fallback check) lulus. Tidak ada UI berubah; manual browser verification: outstanding (smoke list pages + EOD monitoring).
- **Commit**: `d7ede9e`

---

## 2026-10-07 — Kelolaan ATM: dropdown "Paket seluruh vendor" hanya label + tabel terpotong
- **Symptom**: di `/settings/admin/atms` → Kelolaan ATM, mode "Paket seluruh vendor" hanya menampilkan label (PAKET 3/4/5) tanpa kode paket; modal terlalu sempit sehingga kolom tabel riwayat kelolaan terpotong.
- **Root cause**: `ListATMPackageOptions` mengembalikan `DISTINCT package` saja; `ATMAssignmentsDialog` memakai lebar default `Dialog` (`max-w-lg`).
- **Fix**: query `ListATMPackageOptions` (`backend/queries/atm_assignments_admin.sql` + sqlc regen) kini mengembalikan baris tarif (`package`, `package_code`, `tier_min`, `tier_max`, `base_price` sebagai teks, `currency`); signature repo/service/handler ikut. `PackageSourceFields.tsx` menampilkan `kode · label` per opsi (mis. `PKG3_ABA_001 · PAKET 3`); yang dikirim saat submit tetap label (`avp.package`) — kontrak API create tidak berubah. `ATMAssignmentsDialog.tsx` → `max-w-4xl` + tabel dibungkus `overflow-x-auto`. Endpoint `GET .../assignment-package-options` berubah shape: `packages` kini array objek (satu-satunya konsumen adalah dialog ini).
- **Tests**: Go handler/service/repository unit + `-tags integration TestIntegration_VendorWideAssignment*` lulus; vitest `admin-atms` 84/84, lint + build lulus; manual browser verification: outstanding.
- **Commit**: _(belum)_

---

## 2026-09-30 — Halaman bisa di-scroll ke area kosong (forecast-browser)
- **Symptom**: `/replenishment/forecast-browser` masih bisa scroll ke bawah melewati konten; seluruh shell (sidebar + main) naik dan menyisakan area kosong.
- **Root cause**: elemen `sr-only` (`position: absolute`) di `ForecastSummary.tsx`/`ForecastBrowser.tsx` tidak punya ancestor ber-`position`, jadi containing block-nya = viewport. Elemen itu lolos dari clipping `overflow` `<main>`/grid dan memperpanjang tinggi dokumen.
- **Fix**: `AppShell.tsx` — `<main>` diberi `relative` (berlaku untuk semua halaman protected). Grid rows `auto minmax(0, 1fr)` ikut di diff yang sama.
- **Tests**: `AppShell.test.tsx` assert `<main>` punya `relative` — 11/11 lulus; manual browser verification: outstanding.
- **Commit**: _(belum)_
