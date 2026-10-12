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

## 2026-10-12 — A11y VendorPortal: sort, baris DSR, dialog DSR, baris invoice lewat keyboard
- **Symptom**: 9 warning biome a11y. Yang nyata: sort kolom `DataTable` dan buka detail DSR (klik baris) tidak bisa lewat keyboard; dialog DSR tidak tertutup dengan Escape; baris invoice `<tr role="button">` merusak semantik tabel untuk screen reader.
- **Root cause**: handler `onClick` dipasang di `<th>`/`<tr>`/overlay tanpa jalur keyboard; baris invoice dijadikan tombol.
- **Fix** (keputusan user 2026-10-12, semua opsi rekomendasi): `components/ui/DataTable.tsx` — header sortable jadi `<button>` di dalam `<th>` (`aria-sort` tetap di `<th>`), baris dengan `onRowClick` dapat `tabIndex=0` + Enter/Space + focus ring. `features/dsr/DsrDetailDialog.tsx` + `DsrUploadDialog.tsx` — Escape menutup dialog (upload: tidak saat `uploading`/`confirming`, sama dengan klik overlay); `<dialog>` native ditunda (biome-ignore beralasan). `features/invoices/InvoicesPage.tsx` — `role="button"`/`aria-expanded`/`aria-label` pindah ke `<button>` chevron di sel pertama; klik tombol (juga Enter/Space) bubble ke `onToggle` baris; `handleRowKeyDown` + prop `onKeyDown` dihapus. `biome-ignore` beralasan untuk `FileUpload` drop zone (sudah punya Enter/Space) dan backdrop `AppShell` (`aria-hidden`, Escape sudah ada).
- **Tests**: `DataTable.test.tsx` +3 (sort via tombol header + `aria-sort`, Enter/Space di baris klik, baris tanpa `onRowClick` tidak fokusabel), `DsrUploadDialog.test.tsx` +1 (Escape menutup). `lint` 0 error / 0 warning, `test` 18 file / 127 test hijau, `build` hijau. Catatan: `routeGuard.property.test.tsx` sekali gagal flaky saat full run, lolos 5/5 terpisah dan 3/3 di HEAD — tidak terkait. Manual browser verification (Tab ke header & baris DSR, Escape di dialog, expand invoice dengan keyboard, tampilan header/baris tidak berubah): outstanding.
- **Commit**: _(belum)_

## 2026-10-12 — Lint VendorPortal gagal (format test + warning ringan)
- **Symptom**: `pnpm --dir frontend/VendorPortal-Vite run lint` exit 1 (sudah sejak sebelum QC F7): 1 error + 16 warning.
- **Root cause**: error = `src/features/dsr/__tests__/DsrUploadDialog.test.tsx` belum diformat biome (baris fixture terlalu panjang). Warning ringan: 2 unused type param, 4 non-null assertion (`!`), 1 key index array.
- **Fix** (cakupan dipilih user: error + warning ringan, a11y dibiarkan): format test file (hanya pemecahan baris). `DsrDetailDialog.tsx`: `queryFn` dijaga `item.daily ? … : Promise.reject(...)` (dan `rencana_isi`) — tidak berubah di runtime karena query hanya jalan saat `enabled`. `SchedulePage.tsx` + `__tests__/grouping.property.test.ts`: `groupMap.get(date) ?? []` (key selalu ada dari `groupMap.keys()`). `biome-ignore` dengan alasan untuk 2 hal yang memang benar: `DataTable.tsx` `ColumnMeta<TData, TValue>` (declaration merging wajib memakai type parameter TanStack yang sama) dan `FileUpload.tsx` `key={i}` (daftar error dibangun ulang tiap render, pesan bisa sama).
- **Tests**: `lint` exit 0 (0 error, sisa 9 warning a11y: `useKeyWithClickEvents`/`useSemanticElements` di `DataTable`, `AppShell`, `FileUpload`, dialog DSR, `InvoicesPage` — belum dikerjakan, perlu keputusan markup/keyboard). `test` 18 file / 123 test hijau, `build` hijau. Manual browser verification (detail DSR + halaman Schedule tampil sama): outstanding.
- **Commit**: `aed93e7`

## 2026-10-12 — IP client retry manual di Python EOD (lanjutan QC F13)
- **Symptom**: `POST /retry/{file_id}` di `eod_retry_scheduler` dan `service_dsr_etl` mencatat `request.client.host` sebagai IP audit retry; di belakang Nginx nilainya selalu IP Nginx/gateway Docker, bukan IP user.
- **Root cause**: `uvicorn.run` di kedua `run.py` tanpa konfigurasi proxy: default uvicorn hanya memercayai `X-Forwarded-For` dari `127.0.0.1`, jadi IP Nginx tidak pernah diganti. Entri yang rusak di `forwarded_allow_ips` juga diam-diam diabaikan uvicorn.
- **Fix**: helper baru `backend_python/lib/utils/proxy.py` (`parse_trusted_proxies`, `uvicorn_proxy_kwargs`). Setting `trusted_proxies` (`RETRY_TRUSTED_PROXIES` / `DSR_ETL_TRUSTED_PROXIES`, validator = gagal start bila rusak) di kedua `config.py`, dan kedua `run.py` meneruskan `proxy_headers` + `forwarded_allow_ips`. Kosong = proxy header dimatikan total (juga untuk 127.0.0.1); terisi = middleware uvicorn ≥ 0.30 membaca XFF dari kanan dan melewati hop tepercaya, sama dengan Go `TrustedRealIP`. `retry.py` tidak diubah. Dokumen: kedua `.env.example`, `.claude/docs/deployment.md`, `python_eod_client_ip_todo.md` (selesai).
- **Tests**: `python -m unittest lib.utils.test_proxy` (8 test: parser, fail-fast kedua `Settings`, kosong = header diabaikan bahkan dari localhost, klien tidak tepercaya tidak bisa memalsukan XFF, Nginx → IP paling kanan, hop LB dilewati; memakai `ProxyHeadersMiddleware` asli uvicorn 0.52.4). `lib` 65 test (dengan DB dev), `dsr` + `itm/cashpos` hijau. Manual (retry manual dari halaman EOD Monitoring lewat Nginx → `retry_audit_logs`/`audit_logs` berisi IP browser): outstanding.
- **Commit**: `5f0ced8`

## 2026-10-12 — QC F17: harga yang belum berlaku tidak bisa di-disable (approve gagal 409)
- **Symptom**: disable harga vendor-wide (`vendor_package_prices`) atau harga khusus cabang (`vendor_packages_branch`) yang mulai hari ini atau nanti lolos submit (202), tapi approve-nya selalu gagal 409.
- **Root cause**: disable mengisi `effective_end_date = CURRENT_DATE - 1` (`queries/vendor_package_prices_admin.sql`, `queries/vendor_packages_admin.sql`), sehingga untuk start ≥ hari ini melanggar `*_period_chk` (`end >= start`). Submit tidak mengecek hal ini.
- **Fix** (keputusan PO 2026-10-12: tolak saat submit): helper `rejectNotYetEffective` di `backend/internal/service/vendor_package_price_admin.go`, dipanggil di `Disable` kedua service (`vendor_package_price_admin.go`, `vendor_package_admin.go`). Harga dengan start ≥ hari ini (WIB, `jakartaCalendarDate`) → `ValidationError` field `effective_start_date` → 422, tanpa staging. Harga seperti itu dipendekkan lewat Ubah (`effective_end_date`). SQL dan skema tidak berubah.
- **Tests**: `TestVendorPackageAdminService_Disable_NotYetEffective` (kemarin → di-stage; hari ini / besok → 422 tanpa submit) dan kasus serupa di `TestVendorPackagePriceAdminService_UpdateAndDisable`. Komentar F17 di `masterdata_applier_price_integration_test.go` diperbarui. `go test -tags integration ./...` (backend) hijau. Disable yang lolos submit tetap aman di-approve kapan pun (start < hari ini ≤ tanggal approve); sisa celah hanya sekitar tengah malam bila zona waktu DB (`CURRENT_DATE`) ≠ WIB → tetap 409 seperti sebelumnya. Manual browser verification (tombol Nonaktifkan pada harga yang belum berlaku menampilkan pesan error): outstanding.
- **Commit**: `491a61f`

## 2026-10-12 — QC F10 (sebagian): kompleksitas `UserAdminService` Create/Update
- **Symptom**: golangci-lint gocyclo: `UserAdminService.Create` dan `Update` cyclo 34 (> 30).
- **Root cause**: validasi profil, aturan `auth_source`, cek unik, dan resolusi referensi ditulis inline dan terduplikasi di kedua fungsi.
- **Fix** (refactor, perilaku tidak berubah): `backend/internal/auth/user_admin.go`. Logika diekstrak ke helper privat `validateProfile`, `localCredentials` (hash bcrypt hanya untuk local; `must_change_password` = ada hash), `uniqueness`, dan `resolveReferences`. Urutan error, field error, dan pesan wrap tetap sama. API dan signature publik tidak berubah. Auth (Sec 4 #7) sudah di-flag; user memilih dokumentasi bugfix.
- **Tests**: 17 test `user_admin_test.go` tetap hijau tanpa diubah. Coverage Create 89 → 97%, Update 84 → 90%. Lint sekarang tinggal 3 temuan: gocyclo `masterdata_import` (ditunda, menunggu FSD replace-all) dan F13. Manual browser verification (form tambah/ubah user): outstanding.
- **Commit**: `f4d9b67`

## 2026-10-09 — QC F8 batch 10 (selesai): submit harga vendor-wide, helper import, `pkg/response`
- **Symptom**: `VendorPackagePriceAdminService` Create/Update/Disable/`checkVendor` 0% (jalur submit maker-checker harga); helper import CSV (`vaultFields`, `picFields`, `atmFields`, `stageToggle`, `rowRejection`) + `ParseExportEntity`/`WriteExportTemplate` 0%; baca admin `MasterDataChangeService`/`VendorBranchAdminService`/`DsrLocationMapAdminService` 0%; `pkg/response` 0%. service 78.9% (< 80%).
- **Root cause**: belum ada test langsung.
- **Fix** (test saja, kode produksi tidak berubah): `vendor_package_price_admin_test.go` (unit: vendor tak dikenal / INTERNAL (ROH) / label kosong / harga rusak / gagal cek vendor → ditolak tanpa `Submit`; update row vendor lain / tidak ada / sudah berakhir → `ErrVendorPackagePriceNotFound`, end < start → 422; update + disable mengirim `EntityID` + snapshot `Before`). `masterdata_import_helpers_test.go` (unit: nominal sel tetap string eksak, sel opsional kosong → nil, PIC tanpa `branch_code` = vendor-wide, hanya `"true"` persis yang mem-blacklist; baris toggle → enable/disable bukan update; error validasi/konflik → error baris 422, error lain tetap 500; template = BOM + header saja). `admin_reads_integration_test.go` (repo asli, tx rollback: change request pending, cabang, peta lokasi DSR + label DSR yang belum dipetakan). `pkg/response/response_test.go` (envelope success/created/error/validation, field kosong tidak dikirim).
- **Tests**: coverage `internal/service` 78.9% → 80.6%, `pkg/response` 0 → 100%. **F8 selesai**: semua package `internal/*` ≥ 80% (`internal/db` generated dikecualikan). `go test -tags integration -p 1 ./...` lulus, lint bersih (sisa gocyclo = F10). Manual browser verification: tidak relevan.
- **Commit**: `b21f2e4`

---

## 2026-10-09 — QC F8 batch 9: test service DSR vendor + baca harga admin
- **Symptom**: `DsrService` (`ValidateUpload`, `sanitizeFilename`, `ResolveVendor`, `CheckUploadRate`, `GetSaldoFile`, `GetRencanaIsiFile`, `ListUploads` + mapper) 0% di package `service` (test handler di package lain tidak dihitung); List/Count/Get `VendorPackageAdminService` + `VendorPackagePriceAdminService` (mapper `base_price`) 0%.
- **Root cause**: belum ada test di package `service` untuk jalur ini.
- **Fix** (test saja, kode produksi tidak berubah): `dsr_upload_read_test.go` (unit: xlsx/xls diterima, kosong / > 10MB / bukan workbook / nama `..` atau kosong ditolak, `../../etc/dsr.xlsx` → `dsr.xlsx`; vendor tidak ada → `ErrDsrVendorNotFound`, error DB lain dibungkus; upload ke-11 per vendor per jam → `ErrDsrRateLimited`, vendor lain punya counter sendiri, key ber-TTL ≤ 1 jam, Redis mati → error biasa; detail daily/rencana isi hanya untuk vendor pemilik (lain → `ErrDsrFileNotFound`), success = row − error, nominal NULL tetap nil; list meneruskan vendor + paging, validasi page/page_size/tanggal). `vendor_price_read_integration_test.go` (repo asli, tx rollback: `base_price` `1250000.50` / `99999999.99` eksak sebagai string, periode berakhir tidak terhitung `active`, Get oleh vendor lain → nil, nil).
- **Catatan F7**: nominal DSR di test unit masih diassert sebagai `*float64` (perilaku sekarang); ikut diganti saat F7.
- **Tests**: coverage `internal/service` 75.6% → 78.9% (`-tags integration`). `go test -tags integration -p 1 ./...` lulus, lint bersih (sisa gocyclo = F10). Manual browser verification: tidak relevan.
- **Commit**: `b9d8793`

---

## 2026-10-09 — QC F8 batch 8: handler ≥ 80% (pemetaan error login, toggle admin)
- **Symptom**: `handleAuthError` (pemetaan error login → HTTP) 52.9%; cabang id rusak di toggle disable/enable handler admin ~53%; handler 79.6%, kurang dari target 80%.
- **Root cause**: belum ada test untuk cabang-cabang ini.
- **Fix** (test saja, kode produksi tidak berubah): `auth_error_mapping_test.go` (tabel: validasi 422, rate limit 429 + `Retry-After`, kredensial salah 401 termasuk yang di-wrap, akun nonaktif/portal salah/password kedaluwarsa/terkunci 403 dengan kode masing-masing, LDAP/layanan tidak tersedia 503, error tak dikenal 500 tanpa membocorkan detail internal). `admin_update_handlers_test.go`: id rusak di disable/enable → 400 untuk vault/pic/branch/kelolaan ATM/paket cabang; paket cabang tidak punya route enable → 404 (by design, effective-dated). `extractClientIP` sengaja tidak diuji: akan diubah di F13 (proxy tepercaya).
- **Tests**: coverage `internal/handler` 79.6% → 80.3% (`-tags integration`). `go test -tags integration -p 1 ./...` lulus, lint handler 0 issue. Manual browser verification: tidak relevan.
- **Commit**: `d1e2ca1`

---

## 2026-10-09 — QC F8 batch 7: sisa handler 0% (RetryApply, /auth/me, update admin, dll.)
- **Symptom**: fungsi handler 0%: `RetryApply`/`WithApplyRetry` (terapkan ulang perubahan master-data approved), `AuthHandler.Me`, ACM area `EligibleUsers`/`BranchOptions`/`Rename`, `Update` admin vault/pic/branch/kelolaan ATM + `Enable` branch, Vendor Request `UpdateItems`/`Revise`, `VaultPlanHandler.Audit`, respons sukses upload/confirm DSR.
- **Root cause**: belum ada test HTTP untuk jalur ini.
- **Fix** (test saja, kode produksi tidak berubah): `retry_apply_me_handler_test.go` (retry hanya untuk `master_data` + `approved` → retrier dipanggil dengan change id + actor; pending / dokumen lain / approval tidak ada → 409 tanpa retry; retrier tidak terpasang → 404; `ErrNotAuthorized` → 403. `/me`: profil tanpa field password, user terhapus → 401, DB error → 503, tanpa token → 401). `admin_update_handlers_test.go` (PUT 202 + `change_request_id` di 4 handler admin, body/id rusak 400, role non-admin 403; enable branch 202 / pending 409; UpdateItems 200, periode_pred wajib → 400, bukan creator 403, checker 403 di route; Revise 200 / state salah 409). Baris tabel baru di `admin_acm_area_handler_test.go` dan `vault_plan_handler_test.go`. `dsr_upload_handler_test.go`: dry-run + confirm terhadap ETL tiruan (`httptest`), memeriksa kontrak nama file `<vendor_code>__<user_id>__<basename>` (path dibuang) dan confirm tanpa checksum → 400.
- **Tests**: coverage `internal/handler` 75.0% → 79.6% (`-tags integration`), tidak ada lagi fungsi 0%. `go test -tags integration -p 1 ./...` lulus, lint handler 0 issue. Manual browser verification: tidak relevan.
- **Commit**: `151d2d2`

---

## 2026-10-09 — QC F8 batch 6: test handler harga vendor-wide, upload DSR, review vault
- **Symptom**: `AdminVendorPackagePriceHandler` 0% seluruhnya (endpoint money + maker-checker); `DsrUploadHandler` Upload/GetRencanaIsi/ListUploads/`handleServiceError` 0% (endpoint vendor); `VaultAssignments`/`VaultApprove`/`VaultReject` (review vault ATM-SPV) 0%; `writeTooManyRequests` 0%.
- **Root cause**: belum ada test HTTP untuk handler ini.
- **Fix** (test saja, kode produksi tidak berubah): `admin_vendor_package_price_handler_test.go` (fake service: list meneruskan filter + offset, `base_price` tetap string desimal, create/update/disable → 202 + `change_request_id`, 400 path/body rusak, pemetaan error 422/404/409/403/500, role non-admin 403, tanpa token 401). `dsr_upload_handler_test.go` (fake repo terscope vendor + miniredis: detail daily/rencana isi + list, upload id vendor lain → 404, query rusak → 400, non-vendor 403; upload tanpa file/kosong/bukan workbook → 400, dry-run gagal → 502, upload ke-11 dalam satu jam → 429 + `Retry-After: 3600`; nominal dicek sebagai digit agar tahan perubahan F7). `vendor_request_vault_handler_test.go` (fake reviewer: snapshot kosong → `null`, approve/reject meneruskan actor + alasan, maker 403 di route, `ErrSelfApproval`/`ErrNotChecker` 403, `ErrInvalidTransition` 409, alasan kosong 422).
- **Tests**: coverage `internal/handler` 68.1% → 75.0% (`-tags integration`). `go test -tags integration -p 1 ./...` lulus, lint handler 0 issue. Manual browser verification: tidak relevan.
- **Commit**: `dfa53f9`

---

## 2026-10-09 — QC F8 batch 5: test repo import batch + guard admin master-data
- **Symptom**: sisa fungsi 0% di `repository`: `MasterDataImportBatchRepository` (staging CSV import), cek submit kelolaan ATM (`HasOverlap`, `ATMActive`, `PackageActive`, lookup vendor-wide), repo peta lokasi DSR, `CountActiveByBranch`/`BranchVendorID`/`FindByCode` (guard disable cabang), `VendorKind`, `FindByEmployeeID*`, set referensi import.
- **Root cause**: hanya teruji tidak langsung lewat service; belum ada test repo langsung.
- **Fix** (test saja, kode produksi tidak berubah): `masterdata_import_batch_repository_test.go` (helper `integrationTx`; batch dibuat atomik, upload ulang file sama → batch lama `existing=true` tanpa row baru, satu approval untuk semua row, row bentrok dengan pending lain → `BatchRowError` index 1 dan seluruh batch batal). `admin_guard_repository_test.go` (overlap open-ended / exclude diri / periode sebelum mulai; ATM & paket aktif vs berakhir vs tidak ada; label DSR case-insensitive + exclude diri; vault aktif → vendor; hitung anak cabang; pemilik cabang; `VendorKind` FLM_VENDOR; `employee_id` termasuk exclude diri; lokasi baru muncul di referensi import).
- **Tests**: coverage `internal/repository` 57.1% → 86.9% (`-tags integration`), tidak ada lagi fungsi 0%. `go test -tags integration -p 1 ./...` lulus, lint repository 0 issue. Manual browser verification: tidak relevan.
- **Commit**: `a571719`

---

## 2026-10-09 — QC F8 batch 4: test repo auth + staging maker-checker
- **Symptom**: `AuthRepository` (jalur login: cari user by email/username/id, lockout, set password, soft-delete) dan `MasterDataChangeRepository` (staging maker-checker) 0% coverage di package `repository` — hanya teruji tidak langsung lewat service/handler.
- **Root cause**: belum ada test repo langsung.
- **Fix** (test saja, kode produksi tidak berubah): `auth_repository_test.go` (integration, tx rollback: user tidak ada → `(nil, nil)`; increment/lock/reset lockout; `SetInitialPassword` memaksa ganti, `SetPassword` membersihkan paksa-ganti + counter; `MarkPasswordExpired`; `UpdateLastLogin`; `Deactivate` menutup login + profil, `Reactivate` membuka lagi). `TestAuthRepository_PropagatesDBErrors` (unit, `poolProbe`): error DB selain no-rows diteruskan, bukan dibaca "user tidak ada". `masterdata_change_repository_test.go` (integration: guard pending per entity, link `approval_request_id`, approved-but-not-applied menyimpan error, stale, rejected, List/Count filter, ListByBatch).
- **Tests**: coverage `internal/repository` 42.9% → 57.1% (`-tags integration`); semua fungsi kedua repo 100% (kecuali `FindPending` 83.3%: cabang error DB). `go test -tags integration -p 1 ./...` lulus, lint repository 0 issue. Manual browser verification: tidak relevan.
- **Commit**: `aaedccf`

---

## 2026-10-09 — QC F8 batch 3: guard vault plan ACM, `UpdateItems`, cancel party withdrawn
- **Symptom**: cabang guard yang tidak tersentuh test alur utama: `SaveAssignments`/`validateAssignments` (ATM duplikat, branch vault tidak valid, alasan urgent < 10 / > 500 / kosong, edit plan yang sudah pending), `Submit` ulang, `Reject` (alasan kosong, plan masih draft, maker menolak plan sendiri); `UpdateItems` (item kosong, request tidak ada, bukan creator, bukan draft); `cancelVendorParties` melewati party `withdrawn`.
- **Root cause**: belum ada test untuk jalur gagal / four-eyes ini.
- **Fix** (test saja, kode produksi tidak berubah): `vault_plan_guards_integration_test.go` (fixture `newVaultPlanFixture`), `vendor_request_update_items_integration_test.go` (harness nomor request), `TestIntegration_VendorOrder_CancelSkipsWithdrawnParty` di `vendor_order_integration_test.go` (party withdrawn → tanpa event `cancelled`, tanpa notifikasi ke vendornya).
- **Tests**: coverage `internal/service` 75.3% → 75.6% (`-tags integration`); `Reject` 62.5→87.5%, `SaveAssignments` 68.2→72.7%, `validateAssignments` 93.8%, `UpdateItems` 64.8→74.1%, `cancelVendorParties` 72.7→81.8% (sisa = error DB). `go test -tags integration -p 1 ./...` lulus, lint bersih (sisa gocyclo = F10). Manual browser verification: tidak relevan.
- **Commit**: `01e2472`

---

## 2026-10-09 — QC F8 batch 2: test `CurrentState` applier + opsi/audit Vendor Request
- **Symptom**: `CurrentState` (snapshot stale-check saat approve, maker-checker) di 6 applier — atm_assignment, dsr_location_map, vendor_branch, vendor_package, vendor_pic, vendor_vault — 0% coverage; `VendorRequestService.ListVendorOptions` dan `AuditLog` 0%.
- **Root cause**: belum ada test langsung.
- **Fix** (test saja, kode produksi tidak berubah): `masterdata_applier_current_state_integration_test.go` (tx rollback: seed tiap entity, `CurrentState` mengembalikan tipe row `Get*AdminByIDRow` yang sama dengan Before; id hilang → `pgx.ErrNoRows`). `vendor_request_options_audit_test.go` (unit, fake repo: region NULL dilewati, error diteruskan; `AuditLog` request hilang → `ErrNotFound`, error lain dibungkus, `state`/metadata dipetakan, query `entity_type=vendor_request`).
- **Tests**: coverage `internal/service` 74.3% → 75.3% (`-tags integration`); 6× `CurrentState` 100%, `ListVendorOptions` 92.3%, `AuditLog` 90.9%. `go test -tags integration -p 1 ./...` lulus, lint bersih (sisa gocyclo = F10). Manual browser verification: tidak relevan.
- **Commit**: `44dabe8`

---

## 2026-10-09 — QC F8 batch 1: test applier harga vendor-wide (money, maker-checker)
- **Symptom**: `VendorPackagePriceApplier.Apply` (apply-on-approve harga vendor-wide) 0% coverage, termasuk generator `package_code` (`nextPackageCode`, `digitsOrFallback`) dan pemetaan error DB (`mapPriceDBError`, `mapPackageDBError`).
- **Root cause**: hanya diuji tidak langsung lewat seed SQL; tidak ada test applier.
- **Fix** (test saja, kode produksi tidak berubah): `masterdata_applier_price_test.go` (unit: digit label → kode, termasuk fallback `0`; exclusion → `ErrVendorPackagePriceOverlap`/`ErrVendorPackageOverlap`, unique `package_code` → `ErrVendorPackageCodeConflict`, error lain diteruskan; Apply menolak op tak dikenal, update/disable tanpa `entity_id`, payload/base_price/tanggal rusak). `masterdata_applier_price_integration_test.go` (tx rollback: create → `PKG3_ITPX_001` lalu `PKG0_ITPX_002`, `base_price` desimal eksak `1250000.50`; update hanya content + money eksak; disable menutup periode; `CurrentState`; overlap grain sama → `ErrVendorPackagePriceOverlap`).
- **Temuan**: disable harga yang mulai hari ini/di masa depan gagal (`CURRENT_DATE - 1 < effective_start_date` melanggar `vpp_period_chk`), juga di `vendor_packages_branch` → **QC F17**, butuh keputusan produk (Sec 4 #7), tidak diperbaiki di batch ini.
- **Tests**: coverage `internal/service` 73.0% → 74.3% (`-tags integration`); `VendorPackagePriceApplier.Apply` 0% → 83.3%, `mapPriceDBError`/`mapPackageDBError`/`digitsOrFallback` 100%. Lint bersih (sisa gocyclo = F10). Manual browser verification: tidak relevan.
- **Commit**: `67c9e54`

---

## 2026-10-09 — QC F16: `page` tanpa batas atas → OFFSET negatif / overflow int4 (500)
- **Symptom**: semua list endpoint hanya mengecek `page ≥ 1`. `page=3000000000` → `int32(params.Page)` wrap ke negatif, atau `(page-1)*page_size` overflow int4 di SQL (`queries/atm_portal.sql:103,252,314`) → Postgres error → 500, bukan 400.
- **Root cause**: parser `page` tersebar (shared `parseIntParam`, `parsePageParams` admin, `parseRegionPageParams`, `Atoi` di vendor order) tanpa batas atas; service mengonversi ke int32.
- **Fix**: `maxPage = 1_000_000` + helper `parsePageParam` (`handler/atm_portal_handler.go`) dipakai 9 pemanggil yang dulu `parseIntParam(q, "page", …)` (ATM portal ×4, audit log, DMAA forecast, DSR uploads, vendor request ×2) — pesan 400 jadi "page harus berupa angka (maksimal 1000000)". `parsePageParams` (12 handler admin) dan `parseRegionPageParams`: `page > maxPage` → "page tidak valid". Vendor order: tetap lenient untuk non-angka, tapi `page > maxPage` → 422 `page`. Notifikasi sudah dibatasi (`parseBoundedInt`, 2²⁰). Semua konversi `int32` page/page_size di service + 2 batch `len(rows)` + audit handler diberi `//nolint:gosec` dengan alasan. gosec kini bersih.
- **Tests**: `TestPageParsers_RejectAboveMaxPage` (RED tanpa fix: `maxPage`/`parsePageParam` undefined). build + vet + golangci-lint (gosec 0) + `go test -tags integration -p 1 ./...` (dev DB) lulus. Manual browser verification: tidak relevan (UI tidak mengirim page sebesar itu); opsional cek `GET /api/v1/atm-portal/atms?page=3000000000` → 400.
- **Commit**: `255c6a8`

---

## 2026-10-09 — QC F6: repo admin master-data membaca list dari replica
- **Symptom**: `dbReadPool` sudah di-wire di `cmd/api/main.go` (Phase 0.1), tetapi 7 repo admin masih primary-only dengan komentar TODO basi "swap List/Count to dbRead when DATABASE_REPLICA_URL wiring lands".
- **Root cause**: Phase 0.1 (2026-10-07) hanya memindahkan Vendor/ATM/Region/DSR-location-map; repo lain tertinggal.
- **Fix**: `VendorVault`, `VendorPic`, `VendorPackage`, `VendorPackagePrice`, `VendorBranch`, `ATMAssignment` AdminRepository → constructor `(primary, replica)` seperti `VendorAdminRepository`; **hanya List/Count** ke replica, GetByID + pre-check submit tetap primary (snapshot "before"). `main.go` memberi `dbReadPool`. Aman terhadap replica lag karena master-data maker-checker: row baru berubah setelah approve. **`UserAdminRepository` sengaja tetap primary-only** — create/update user apply langsung dan layar me-list ulang tepat setelah write (alasan sama dengan `MasterDataChangeRepository`); TODO diganti komentar alasan.
- **Tests**: `TestAdminRepositories_ReadTopology` (18 kasus, probe DBTX tanpa DB: List/Count → replica, GetByID → primary). build + vet + `go test -tags integration -p 1 ./...` (dev DB, primary=replica) lulus. Tidak ada UI berubah; manual browser verification: outstanding (smoke list cabang/vault/PIC/paket/harga/kelolaan di admin; di dev replica = primary).
- **Commit**: `4298546`

---

## 2026-10-09 — QC F5: permission file upload DSR (diterima, didokumentasikan)
- **Symptom**: gosec G301/G306 di `dsr_upload.go` (`MkdirAll 0755`, `WriteFile 0644`) untuk file DSR vendor (data cash).
- **Root cause**: bukan bug — container Go (`appuser`) menulis ke bind mount `FTP_DATA`, dan `backend_python` (user host lain, tidak di-container-kan) harus membaca + `shutil.move` file itu ke `backups/` (`dsr_etl.py:117,680`). 0640 akan memutus flow DSR di server kecuali ada grup bersama.
- **Fix**: keputusan user 2026-10-09 — **terima 0644/0755**. Komentar alasan + `//nolint:gosec` di `backend/internal/service/dsr_upload.go`; catatan deploy di `.claude/docs/deployment.md` (VM khusus aplikasi; `FTP_DATA` harus writable user Python; perketat ke 0640/0750 + `group_add` saat Python di-container-kan). Perilaku tidak berubah.
- **Tests**: build + gosec bersih untuk `dsr_upload.go`; `go test -run Dsr ./internal/...` lulus. Manual browser verification: tidak relevan.
- **Commit**: `e69e4ef`

---

## 2026-10-09 — QC F4: tier paket/harga tanpa batas atas (int64 → int32 terpotong)
- **Symptom**: gosec G115 di applier `vendor_packages_branch` / `vendor_package_prices` (`int32(p.TierMin)`, `tierMaxInt32`). Validasi submit hanya mengecek `tier_min ≥ 1` dan `tier_max ≥ tier_min`, jadi tier > 2³¹ lolos dan saat approve ter-wrap diam-diam ke angka lain di kolom int4 — tarif bisa jatuh ke tier yang salah.
- **Root cause**: payload memakai `int64`, kolom DB `integer`, tanpa batas atas di `validatePackageGrain` / `validatePriceGrain`.
- **Fix**: konstanta `maxTier = 999_999_999` (`vendor_package_admin.go`) = sentinel range terbuka di exclusion constraint migration 009 (`COALESCE(tier_max, 999999999)`), muat di int4. `tier_min` > maxTier → 422 `tier_min` ("harus antara 1 dan 999999999"; pesan untuk < 1 ikut berubah), `tier_max` > maxTier → 422 `tier_max`. Berlaku di kedua validator (paket khusus cabang + harga vendor-wide). Import CSV tidak membawa tier; Update tidak mengubah tier. Konversi yang batasnya kini terbukti diberi `//nolint:gosec` + alasan: kedua applier, dan `RowCount int32(len(rows))` (≤ `MasterDataImportMaxRows` 2000). Konversi `int32(Page)` **tidak** di-nolint — page tanpa batas atas dipisah jadi QC F16.
- **Tests**: `TestPackageAndPriceGrain_TierUpperBound` (7 kasus × 2 validator; gagal compile/RED tanpa fix). build + vet + `go test -tags integration -p 1 ./...` (dev DB) lulus. Manual browser verification: outstanding (form paket/harga admin — isi tier 1000000000 → pesan error di field tier).
- **Commit**: `ebcd87a`

---

## 2026-10-09 — QC F3: errcheck di kode produksi
- **Symptom**: golangci-lint errcheck melaporkan return error yang tidak dicek di 5 file produksi.
- **Root cause**: pemanggilan Close/Write/Encode best-effort ditulis tanpa `_ =`, berbeda dengan konvensi yang sudah ada (`error_response.go:26`, `dsr_upload_handler.go:103`, `backend/cmd/api/main.go:89,108`).
- **Fix**: ikut konvensi itu, perilaku tidak berubah — `admin_master_data_import_handler.go:59,106` `defer func() { _ = file.Close() }()`; `notification/smtp.go:51,56,59` `_ = conn.Close()` / `defer func() { _ = c.Close() }()`; `pkg/middleware/rbac.go:113` + `pkg/response/response.go:42` `_ = json.NewEncoder(w).Encode(...)` (header sudah terkirim, tidak ada yang bisa dilakukan); `backend-cit/cmd/api/main.go:51,70` `redisClient.Close` / `w.Write`.
- **Tests**: errcheck bersih di 3 modul; build + vet + `go test ./...` (pkg, backend-cit) + `go test -tags integration -p 1 ./...` (backend, dev DB) lulus. Manual browser verification: tidak relevan.
- **Commit**: `e2de53d`

---

## 2026-10-09 — QC F15: context build Docker backend membawa seluruh repo + `.env`
- **Symptom**: compose kedua backend memakai `context: ..` (root repo), tapi tidak ada `.dockerignore` di root; `backend/.dockerignore` tidak pernah dibaca Docker (Docker hanya membaca yang ada di root context). Seluruh repo ikut jadi context, dan `COPY backend/` membawa `backend/.env` ke stage builder (tidak ke image final, tapi ada di cache layer builder). CLAUDE.md Sec 9 menyebut root `docker-compose.yml` yang tidak ada.
- **Root cause**: `.dockerignore` ditaruh per folder backend, padahal context build dipindah ke root saat split `pkg/` + `go.work`.
- **Fix**: `.dockerignore` root berbentuk allowlist (`go.work`, `go.work.sum`, `pkg/`, `backend/`, `backend-cit/`) + exclude `**/.env`, `**/.env.*`, `**/bin/`, `*.exe`, `*.test`, `*.out`, `*.md`. `backend/.dockerignore` (mati) dihapus. CLAUDE.md Sec 9: teks compose + dockerignore diperbaiki. Frontend tidak terpengaruh (context folder sendiri).
- **Tests**: Dockerfile probe (`COPY . /ctx`) dari working tree yang berisi `.env` asli → context 8.1MB, hanya `backend/ backend-cit/ go.work go.work.sum pkg/`, tanpa `.env*` dan `*.md`. `docker build` kedua backend berhasil. Manual browser verification: tidak relevan.
- **Commit**: `ce9fa0f`

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
