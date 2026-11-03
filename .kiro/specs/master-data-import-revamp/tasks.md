# Implementation Plan

## Overview

Urutan implementasi: data dulu (migrasi `036` + query sqlc), lalu backend per lapis (dry-run, confirm, notifier + hook approval, riwayat, detail approval berpaging, wiring), lalu integration test invarian persetujuan dan gate backend. Frontend jalan paralel setelah kontrak tipe (`api.ts`) ditulis: halaman bertahap, riwayat + detail batch, ReviewDialog, lalu gate FE. Dokumentasi dan cek browser manual di akhir.

Referensi pola:
- Dry-run/confirm: `backend/internal/service/masterdata_import.go`, `masterdata_import_confirm.go`.
- Apply/reject batch: `backend/internal/service/masterdata_approval.go`.
- Notifikasi: `backend/internal/notification/notification.go` (`Service.Send`), contoh pemanggil `vault_plan_create.go`.
- Migrasi: `backend/migrations/005_master_data_import_batches.sql` (gaya aditif, forward-only).
- FE state URL: `features/admin-vendors/useAdminVendorsUrlState.ts`; diff: `features/approvals/lib/changeDiff.ts`; mock: `ui-mock.html` Layar A-H.

### Model Split

**Sonnet-runnable**
- 1. Migrasi `036` + query sqlc — Sonnet, Medium
- 2. Dry-run Change_Preview — Sonnet, Medium
- 3. Confirm: nama file, audit, panggilan notifier — Sonnet, Medium (3.2 butuh 4.1)
- 4.1 Implementasi `ImportNotifier` — Sonnet, Medium
- 5. Riwayat + detail batch (service + handler) — Sonnet, Medium (butuh 1, 3.1 untuk file handler yang sama)
- 6. Detail approval berpaging — Sonnet, Medium
- 7. Wiring `cmd/api/main.go` — Sonnet, Medium (butuh 3.2, Opus 4.2, 5.2, 6.1)
- 9. Checkpoint backend — Sonnet, Medium (butuh Opus 8)
- 10. FE kontrak API, status batch, route — Sonnet, Medium
- 11. FE tab Impor baru — Sonnet, Medium
- 12. FE Riwayat impor + detail batch — Sonnet, Medium
- 13. FE ReviewDialog checker — Sonnet, Medium
- 14. Checkpoint frontend — Sonnet, Medium
- 15. Dokumentasi — Haiku, Low (Haiku)
- 16. Cek browser manual (user) — Sonnet, Low

**Opus-required**
- 4. Hook notifikasi + pesan rollback di `MasterDataApprovalService` (4.2, 4.3) — Opus, High — jalur approve/apply/reject maker-checker; salah urutan bisa menerapkan atau menolak batch dengan keliru.
- 8. Integration test invarian persetujuan (Property 1, 2, 3, 9) — Opus, High — inilah bukti syarat PO "data impor di-approve sebelum update DB"; test yang lemah bisa meloloskan regresi maker-checker.

Dependensi lintas grup: Sonnet 3.2 butuh Sonnet 4.1 (interface). Sonnet 7 butuh Opus 4.2. Opus 8 butuh Sonnet 7. Sonnet 9 butuh Opus 8 dan 4.3.

## Tasks

- [ ] 1. Migrasi `036` dan query sqlc
  - [x] 1.1 Tulis `backend/migrations/036_master_data_import_batches_file_name.sql` sesuai design (kolom `file_name text NULL` + CHECK 1-255 + COMMENT), terapkan ke dev dengan `go run ./cmd/migrate up`, cek `go run ./cmd/migrate status`
    - Cek dulu `036` belum terpakai di `backend/migrations/`.
    - _Requirements: 4.5, 10.3_
  - [x] 1.2 Di `backend/queries/master_data.sql`: tambah `sqlc.narg('file_name')` ke `CreateMasterDataImportBatch`; tambah `ListMasterDataImportBatches`, `CountMasterDataImportBatches`, `GetMasterDataImportBatch`, `ListPendingApproversForRequest` (assigned approver step `pending` dengan `step_level` terkecil); jalankan `sqlc generate` (v1.31.1) lalu hand-fix `UserLeafe` → `UserLeave`
    - Di `masterdata_import_batch_repository.go` sekaligus: `Create` menerima `fileName *string`; method baca `ListBatches`, `GetBatch`, `ListByBatch`, `ListPendingApprovers` (primary). Semua perubahan file repo ada di sub-task ini supaya task lain tidak bentrok di file yang sama.
    - _Requirements: 4.5, 5.2, 5.3, 5.4, 6.1, 8.1_
  - [x] 1.3 Integration test repository (`//go:build integration`): status turunan dari baris kepala, `decided_at`/`decided_by_name` dari step yang sudah bertindak, filter entity/status/maker, CHECK `file_name` menolak string kosong dan > 255
    - _Requirements: 5.4, 10.3_
  - _Model: Sonnet, Medium — migrasi aditif nullable dan query baca; pola sudah ada di 005 dan FindOpen._

- [ ] 2. Backend: dry-run dengan Change_Preview
  - [x] 2.1 Di `masterdata_import.go`: tambah `DataUpdates`, `Enables`, `Disables`, `Changes` ke `ImportPreview`; tipe `ImportChangePreview`/`ImportFieldChange`; peta `importKeyColumns`; fungsi pure `buildChangePreview(entity, header, row, cur)`; `Changes` hanya diisi bila tanpa kesalahan; `Updates` tetap mencakup toggle
    - Tidak ada query atau tulis baru.
    - _Requirements: 1.1, 3.1, 3.2, 3.4, 3.5_
  - [x] 2.2 Unit test table-driven per entitas (create, update, enable, disable, kunci dengan kolom kosong) dan **Property 4** dengan `pgregory.net/rapid`: jumlah hitungan = `len(changes)`, `updates == data_updates + enables + disables`, op pratinjau = op yang di-stage confirm, setiap field punya `before != after`
    - _Requirements: 3.1, 3.2_
  - _Model: Sonnet, Medium — perluasan respons read-only; logika diff murni dan bisa di-property-test._

- [ ] 3. Backend: confirm menyimpan nama file, audit, dan memanggil notifier
  - [x] 3.1 Handler `admin_master_data_import_handler.go`: `uploadedCSV` mengembalikan `*multipart.FileHeader`, `Confirm` meneruskan `Filename`; service: `sanitizeImportFileName`, signature `Confirm(ctx, makerID, entity, fileName, r, ip)`, panggil `Batches.Create` dengan nama file (repo sudah diubah di 1.2), audit `submit_batch` menambah `file_name`; batch yang sudah ada tetap memakai nama lamanya
    - _Requirements: 4.4, 4.5, 9.1_
  - [x] 3.2 Tambah `Notifier ImportNotifier` ke `ImportConfirmDeps`; panggil `BatchSubmitted` di `finish` hanya setelah `SubmitForApproval` + `SetApprovalRequestID` + audit sukses pada cabang yang benar-benar submit (bukan saat `ApprovalRequestID` sudah ada); nil = tanpa notifikasi
    - _Requirements: 8.1, 8.4_
  - [x] 3.3 Test: **Property 6** (rapid: hasil nil atau 1-255 rune, tanpa karakter kontrol, tanpa `/` dan `\`, idempoten); unit confirm: audit memuat `file_name`, notifier dipanggil sekali untuk batch baru dan tidak dipanggil untuk `existing`; handler meneruskan nama file
    - _Requirements: 4.4, 4.5, 8.4, 9.1_
  - _Model: Sonnet, Medium — perubahan kecil di jalur submit tanpa mengubah urutan staging/approval; idempotensi yang ada tetap dipakai._

- [ ] 4. Backend: notifikasi impor dan hook di `MasterDataApprovalService`
  - [x] 4.1 `internal/service/masterdata_import_notify.go`: interface `ImportNotifier` dan implementasi `notificationImportNotifier` (`notification.Service.Send` via `db.New(pool)`, empat type pesan sesuai tabel design, body berisi entitas, nomor batch, nama file, jumlah baris); error hanya di-log `slog.Warn`, method tanpa return value
    - _Requirements: 8.1, 8.2, 8.3_
    - _Model: Sonnet, Medium — pemanggil notification yang sudah dipakai modul lain; tidak menyentuh state approval._
  - [x] 4.2 Di `masterdata_approval.go`: `WithImportNotifier`; panggil `BatchSubmitted` saat request masih `pending` setelah approve (level berikut), `BatchApplied`/`BatchApplyFailed` setelah apply di `Approve` dan `RetryApply`, `BatchRejected` setelah `Reject` selesai; hanya untuk kepala dengan `BatchID != nil`; pesan rollback: baris penyebab menyimpan error aslinya, baris lain `BatchRollbackPrefix + "change request %d failed"` (konstanta dari 5.1); status, transaksi, audit, dan nilai kembali tidak berubah
    - _Requirements: 1.4, 1.5, 6.2, 8.1, 8.2, 9.2_
  - [x] 4.3 Test dengan fake notifier dan fake orchestrator/repo: titik panggil per skenario (step berikut, applied, apply gagal, reject, retry sukses/gagal), tidak dipanggil untuk perubahan satu-record; pesan rollback baris penyebab vs baris lain; **Property 9** level unit: notifier/querier yang gagal tidak mengubah hasil `Approve`/`Reject`/`RetryApply` maupun audit
    - _Requirements: 8.2, 8.3, 8.4, 9.2_
  - _Model: Opus, High — menyentuh jalur approve/apply/reject maker-checker; hook di posisi yang salah bisa mengubah hasil persetujuan._

- [ ] 5. Backend: riwayat dan detail batch
  - [x] 5.1 `internal/service/masterdata_import_history.go`: `MasterDataImportHistoryService.List`/`Get` di pool primary, cek ulang peran `authorizeMasterDataAdmin`, `pageSlice` (pure), `counts` atas seluruh batch, konstanta ekspor `BatchRollbackPrefix` + flag `failed` (error tidak kosong dan tanpa prefix itu), `ErrImportBatchNotFound`; tidak menulis audit; memakai method repo dari 1.2
    - _Requirements: 5.1, 5.2, 5.4, 5.5, 6.1, 6.2, 6.5, 9.3_
  - [x] 5.2 Handler: pecah `Routes()` (rate limit hanya untuk dry-run dan confirm), `WithHistory`, `GET /batches` (validasi `entity`, `status`, `mine`, `page`, `page_size` ≤ 100 → 400) dan `GET /batches/{id}` (`page_size` default 50; 404 bila tidak ada)
    - _Requirements: 5.1, 5.3, 6.5, 10.1_
  - [x] 5.3 Test: handler (403 non-admin di service, 400 filter salah, 404, bentuk JSON), `/batches` tidak menaikkan counter rate limit, **Property 7** (rapid: gabungan semua halaman `pageSlice` = n baris urut id tepat sekali; `counts` selalu n), **Property 8** bagian riwayat (peran selain ADMIN/ADMIN_PARAM → 403 di route dan service)
    - _Requirements: 5.1, 6.1, 9.3, 10.1_
  - _Model: Sonnet, Medium — endpoint baca dengan pola handler/service yang ada._

- [ ] 6. Backend: detail approval master data berpaging
  - [x] 6.1 `approval_master_data_detail.go`: query `page` (default 1) dan `page_size` (default 200, maks. 200) memakai `pageSlice`; respons menambah `page`, `page_size`, dan objek `batch` (`id`, `entity`, `file_name`, `maker_name`, `row_count`) untuk batch; `MasterDataApprovalDetailReader` ditambah `GetBatch`; otorisasi tidak diubah
    - _Requirements: 7.1, 7.2, 7.3, 7.4_
  - [x] 6.2 Test: tanpa parameter = 200 baris pertama + `truncated` seperti sekarang; halaman 2 dengan `page_size=50`; satu-record tanpa objek `batch`; **Property 8** bagian approval (bukan maker dan tanpa step pending → 403, dengan atau tanpa paging)
    - _Requirements: 7.2, 7.4, 7.5_
  - _Model: Sonnet, Medium — paging di atas data yang sudah dimuat; cek otorisasi yang ada dipertahankan dan dikunci test._

- [x] 7. Backend: wiring `cmd/api/main.go`
  - Pindahkan `loadNotificationConfig()` + `notification.NewService` ke atas sebelum approval handler; buat `service.NewImportNotifier(notificationService, dbPool)`; teruskan ke `ImportConfirmDeps.Notifier` dan `masterDataApprovalService.WithImportNotifier`; handler impor `.WithHistory(...)`; reader detail approval memakai repo impor untuk `GetBatch`
  - Pastikan urutan konstruksi lain tidak berubah dan `go build ./...` bersih.
  - _Requirements: 5.1, 5.5, 7.3, 8.1_
  - _Model: Sonnet, Medium — wiring; perlu teliti urutan konstruksi tapi tanpa logika bisnis baru._

- [x] 8. Integration test invarian persetujuan (Postgres nyata, `//go:build integration`)
  - **Property 1:** snapshot isi tabel entitas (`vendors`, `vendor_branches`, `vendor_vaults`, `vendor_pics` + tabel peran PIC, `atms`, `atm_vendor_packages`) sebelum/sesudah urutan dry-run → confirm → confirm ulang → approve oleh maker sendiri (403) → approve step non-final (policy 2 level di dalam tx test bila seed tidak punya) → reject; isi harus identik; dry-run juga memastikan tidak ada baris baru di batch/change/approval/audit.
  - **Property 2:** batch dengan satu baris dibuat stale di antara confirm dan approve → tidak ada perubahan entitas, semua baris `stale`, baris penyebab tanpa prefix rollback; batch bersih → semua `applied`.
  - **Property 3:** confirm bersamaan (goroutine) file yang sama → satu batch, satu `approval_requests`, satu audit `submit_batch`, satu set notifikasi `master_data_import.submitted`.
  - **Property 9:** notifier yang selalu gagal → status batch, approval, entitas, dan audit sama dengan run normal; respons sukses.
  - _Requirements: 1.1, 1.2, 1.3, 1.4, 1.5, 1.6, 1.7, 3.5, 4.4, 8.3, 8.4, 9.2, 10.4_
  - _Model: Opus, High — bukti utama syarat PO; harus menutup semua jalur yang bisa mengubah data sebelum persetujuan final._

- [x] 9. Checkpoint backend
  - Jalankan di `backend/`: `go test ./...`, `go test -tags integration ./...` (dengan `DATABASE_URL` via `localhost`), `golangci-lint run --build-tags integration ./...`, `go run ./cmd/migrate status`. Pastikan test impor/approval/CRUD satu-record yang lama tetap hijau tanpa diubah perilakunya. Laporkan hasil; tanya user bila ada kegagalan yang tidak jelas.
  - _Requirements: 10.1, 10.2, 10.3, 10.4_
  - _Model: Sonnet, Medium — triase kegagalan gate._

- [ ] 10. Frontend: kontrak API, status batch, route
  - [x] 10.1 `features/master-data-io/api.ts`: perluas `ImportPreview` (`data_updates`, `enables`, `disables`, `changes`), tipe `ImportBatchSummary`/`ImportBatchDetail`, `listImportBatches`, `getImportBatch`, `retryApplyImport(approvalRequestId)`; `hooks.ts` baru (`masterDataImportKeys`, `useImportBatches`, `useImportBatch`, `useRetryApply` dengan invalidasi list + detail); perbarui `api.test.ts`
    - _Requirements: 3.1, 3.2, 5.2, 6.1, 6.3_
  - [x] 10.2 `batchStatus.ts` (peta status → label, varian badge, ikon; `canRetry`) + test **Property 5** dengan fast-check atas semua kode status
    - _Requirements: 5.4, 5.6, 6.3, 6.4_
  - [x] 10.3 Route `settings/admin/master-data-io.tsx`: `validateSearch` (`tab`, `batch`, `h_entity`, `h_status`, `h_mine`, `h_page`), guard peran tidak berubah; test key asing dibuang dan peran lain mendapat Forbidden
    - _Requirements: 2.5, 2.6_
  - _Model: Sonnet, Medium — tipe dan util pure mengikuti kontrak design._

- [ ] 11. Frontend: tab Impor baru (mock Layar A-D)
  - [x] 11.1 `MasterDataIOPage.tsx` jadi shell: `PageHeader` "Impor CSV Data Master" + deskripsi baru, tab "Impor baru"/"Riwayat impor" (`role="tablist"`, `aria-selected`, panah kiri/kanan) terhubung ke `?tab`; tab Impor baru merender `ImportWizard`; ganti judul + deskripsi kartu di `SettingsHubPage.tsx`
    - _Requirements: 2.1, 2.5_
  - [x] 11.2 `ImportWizard.tsx` + `ImportSteps.tsx`: tiga langkah (`aria-current="step"`, nomor + teks + ikon selesai), isi langkah 1 dipindah apa adanya (entitas, Unduh templat, Ekspor data ini, catatan aturan), langkah 2 file + Periksa file, reset ke langkah 2 saat entitas/file berubah
    - _Requirements: 2.2, 2.3, 2.4_
  - [x] 11.3 `ImportSummary.tsx` (7 angka, `tabular-nums`), `ImportChangeTable.tsx` (filter operasi, paging 50 di klien, badge ikon + label, nominal rata kanan monospace), `ImportErrorTable.tsx` (dipindah dari halaman lama); pasang di langkah 3
    - _Requirements: 3.2, 3.3, 3.4_
  - [x] 11.4 `ImportConfirmDialog.tsx` (entitas, nama file, hitungan per operasi, kalimat "Data belum berubah sampai pengajuan disetujui.") dan `ImportResultPanel.tsx` (batch, baris, status, pesan `existing`, "Lihat di riwayat", "Impor file lain"); 422 saat ajukan kembali ke tabel kesalahan; tombol ajukan aktif hanya bila valid dan ada perubahan
    - _Requirements: 4.1, 4.2, 4.3, 4.4, 4.6_
  - [x] 11.5 Perbarui `MasterDataIOPage.test.tsx` dan tambah test komponen: judul + kartu Pengaturan, tab, langkah + reset, elemen langkah 1, ringkasan, filter + paging tabel, kesalahan, dialog konfirmasi, panel hasil baru/existing, 422 saat ajukan
    - _Requirements: 2.1, 2.2, 2.3, 2.4, 2.5, 3.2, 3.3, 3.4, 4.1, 4.2, 4.3, 4.4, 4.6_
  - _Model: Sonnet, Medium — komponen React mengikuti mock dan design system._

- [ ] 12. Frontend: Riwayat impor dan detail batch (mock Layar E-G)
  - [x] 12.1 `ImportHistoryTab.tsx`: filter Entitas, Status, "Hanya milik saya" ke URL; tabel kolom sesuai mock, waktu WIB, badge dari `batchStatus.ts`; paging 20; `EmptyState`; pasang di tab Riwayat impor pada shell
    - _Requirements: 5.2, 5.3, 5.6, 5.7_
  - [x] 12.2 `ImportBatchDialog.tsx` dibuka dari `?batch=ID` (dipasang di shell): ringkasan, hitungan per operasi, tabel Change_Row (paging 50 server) dengan disclosure diff (`diffFields`/`changeSubject`), label "Gagal" + pesan untuk baris `failed` dan "Dibatalkan" untuk baris stale lain, `NoticeBanner` penyebab + boleh ajukan ulang, catatan pengajuan belum lengkap bila `approval_request_id` kosong, tombol "Coba terapkan lagi" hanya bila `canRetry`, 404 → pesan + hapus `batch` dari URL
    - _Requirements: 6.1, 6.2, 6.3, 6.4, 6.5_
  - [x] 12.3 Test: filter → parameter API, paging, empty state, badge ikon + label; dialog: diff, penyebab stale, tombol retry hanya `approved` dan memanggil API + refetch, 404
    - _Requirements: 5.2, 5.3, 5.6, 5.7, 6.1, 6.2, 6.3, 6.4, 6.5_
  - _Model: Sonnet, Medium — layar daftar/detail dengan hook yang sudah ada._

- [ ] 13. Frontend: ReviewDialog checker (mock Layar H)
  - [x] 13.1 `features/approvals/api.ts` + `hooks.ts`: `getMasterDataDetail(requestId, page, pageSize)`, key query menyertakan page, tipe `batch`; `MasterDataReviewDialog.tsx` untuk batch: kepala (entitas, nama file, pengaju, hitungan Baru/Ubah/Aktifkan/Nonaktifkan), baris dengan disclosure `ChangeDiffTable` (`aria-expanded`), paging 50; satu-record tidak berubah
    - _Requirements: 7.1, 7.3, 7.5_
  - [x] 13.2 Perbarui `MasterDataReviewDialog.test.tsx` dan `apiHooks.test.tsx`: paging, kepala batch, disclosure, Setujui/Tolak tetap bekerja di halaman mana pun, tampilan satu-record sama
    - _Requirements: 7.1, 7.3, 7.5_
  - _Model: Sonnet, Medium — perluasan komponen yang ada; keputusan approve/reject tidak berubah._

- [x] 14. Checkpoint frontend
  - Jalankan di `frontend/CompanyPortal-Vite`: `pnpm run lint`, `pnpm run test:coverage` (84/80/80/85), `pnpm run build`, `node scripts/check-bundle.mjs` (atau script bundle yang dipakai repo). Pastikan halaman ekspor dan approval dokumen lain tidak berubah. Laporkan hasil; tanya user bila ada kegagalan yang tidak jelas.
  - _Requirements: 10.2, 10.5_
  - _Model: Sonnet, Medium — triase kegagalan gate._

- [x] 15. Dokumentasi
  - `.claude/CLAUDE.md` Sec 3 (kolom `master_data_import_batches.file_name`, mig `036`); `.kiro/steering/development-plan.md` (X.10 status, next free migration `037`, snapshot); `.claude/development-progress.md`; `.kiro/steering/rbac-role-feature-matrix.md` (judul menu "Impor CSV Data Master"); ganti status spec di header `requirements.md`/`design.md`.
  - _Requirements: 2.1, 10.3_
  - _Model: Haiku, Low — edit dokumen mekanis._

- [ ] 16. Cek browser manual (user, Golden Rule #10)
  - Agent menyiapkan checklist; user menjalankan: impor tiap entitas sampai diterapkan; file berisi kesalahan; file sama diajukan dua kali; riwayat + filter + detail; batch stale dan tombol retry pada batch `approved`; checker meninjau batch besar (paging, diff); notifikasi bell + baris `notification_emails` approver; peran non-admin ditolak; navigasi keyboard dan screen reader pada tab, langkah, disclosure.
  - _Requirements: 2.6, 4.2, 5.6, 7.1, 8.1, 8.2_
  - _Model: Sonnet, Low — menyusun checklist; verifikasi dilakukan user._

## Notes

- Task bertanda `*` (1.3) opsional, tetapi disarankan karena menguji status turunan yang dipakai riwayat.
- Syarat PO "data impor di-approve sebelum update DB" sudah berjalan sebelum spec ini; task 8 menguncinya dengan test. Jangan ubah urutan staging → approval → apply di task mana pun.
- Pesan rollback (task 4.2) hanya mengubah teks `error`; status `stale` dan transaksi tetap. Batch lama yang sudah `stale` sebelum perubahan tidak punya penanda baris penyebab; FE menampilkan semua barisnya sebagai "Dibatalkan" dengan pesan apa adanya.
- `sqlc generate` wajib v1.31.1, lalu hand-fix `UserLeafe` → `UserLeave` di `backend/internal/db/`.
- Endpoint impor dan riwayat tidak masuk `backend/api/openapi.yaml` (modul lama yang belum tercakup tetap hand-typed, tech.md prinsip 6).
- Tidak ada library baru di backend maupun frontend.

### Definition of Done

- Semua task kecuali 16 selesai, gate 9 dan 14 hijau, migrasi `036` tercatat di `schema_migrations` dev.
- Property 1-9 masing-masing punya test yang lolos (2.2, 3.3, 4.3, 5.3, 6.2, 8, 10.2).
- Dokumen di task 15 diperbarui; cek browser task 16 dikonfirmasi user.

## Task Dependency Graph

- **Wave 0:** 1.1 (migrasi), 2.1 (dry-run), 10.2 (status batch FE), 10.3 (route FE). Tanpa dependensi.
- **Wave 1:** 1.2 (butuh 1.1), 2.2 (butuh 2.1), 10.1 (kontrak FE dari design).
- **Wave 2:** 1.3, 3.1, 4.1, 5.1 (semua butuh query + repo 1.2); 11.1 (butuh 10.1).
- **Wave 3:** 3.2 (butuh 3.1, 4.1), 4.2 (butuh 4.1, konstanta 5.1), 5.2 (butuh 5.1; file handler sama dengan 3.1), 6.1 (butuh `pageSlice` 5.1), 11.2 (butuh 11.1), 13.1 (kontrak detail dari design).
- **Wave 4:** 3.3 (butuh 3.2), 4.3 (butuh 4.2), 5.3 (butuh 5.2), 6.2 (butuh 6.1), 7 (butuh 3.2, 4.2, 5.2, 6.1), 11.3 (butuh 11.2), 13.2 (butuh 13.1).
- **Wave 5:** 8 (butuh 7), 11.4 (butuh 11.3; file `ImportWizard` sama), 12.1 (butuh 10.1, 10.2, 11.1).
- **Wave 6:** 9 (butuh 1.3, 2.2, 3.3, 4.3, 5.3, 6.2, 8), 11.5 (butuh 11.4), 12.2 (butuh 12.1; file shell sama).
- **Wave 7:** 12.3 (butuh 12.2).
- **Wave 8:** 14 (butuh 10.1-10.3, 11.5, 12.3, 13.2).
- **Wave 9:** 15 (butuh 9, 14).
- **Wave 10:** 16 (butuh 15).

```json
{
  "waves": [
    { "id": 0, "tasks": ["1.1", "2.1", "10.2", "10.3"] },
    { "id": 1, "tasks": ["1.2", "2.2", "10.1"] },
    { "id": 2, "tasks": ["1.3", "3.1", "4.1", "5.1", "11.1"] },
    { "id": 3, "tasks": ["3.2", "4.2", "5.2", "6.1", "11.2", "13.1"] },
    { "id": 4, "tasks": ["3.3", "4.3", "5.3", "6.2", "7", "11.3", "13.2"] },
    { "id": 5, "tasks": ["8", "11.4", "12.1"] },
    { "id": 6, "tasks": ["9", "11.5", "12.2"] },
    { "id": 7, "tasks": ["12.3"] },
    { "id": 8, "tasks": ["14"] },
    { "id": 9, "tasks": ["15"] },
    { "id": 10, "tasks": ["16"] }
  ]
}
```
