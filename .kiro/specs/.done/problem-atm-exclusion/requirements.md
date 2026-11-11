# Requirements Document

> **Status: ⏸ ON HOLD (user, 2026-11-02)** — data ATM bermasalah belum diberikan. Jangan lanjut ke design/tasks sampai data itu ada dan D1-D19 diputuskan.
>
> Draft 2026-11-02, menunggu PO. Dipindahkan dari `.claude/sdlc/problem-atm-exclusion/intent.md` (draft 2026-11-02). Ke-19 open question intent dijawab dengan saran Kiro (D1-D19, lihat `## Open Questions`); semuanya berstatus **menunggu PO** dan harus diputuskan sebelum `design.md` ditulis.

## Introduction

### Masalah

Order harian sekarang murni dari file DMAA. `backend_python/dmaa/dmaa_etl.py` memuat `dmaa_atm_forecast`, lalu pengguna memilih barisnya di Forecast Browser untuk dijadikan Vendor Request. CROWN belum punya cara meninjau rekomendasi itu. Akibatnya:

- ATM yang tidak bisa bertransaksi (URS: pending part, vandalisme, dll.) tetap bisa dipesan.
- List dari tim complaint handling/rekonsiliasi dan list project ATM dari unit bisnis tidak punya jalur masuk ke order.
- Nominal DMAA tidak bisa diganti lewat list adjustment yang eksplisit.
- Aturan URS "tidak ada order tambahan bila ATM sudah punya order emergency/adhoc sampai H-1 atau aktif di H0" tidak ditegakkan.

Keputusan user 2026-10-08 menetapkan pekerjaan ini tugas CROWN, bukan DMAA. Roadmap: Phase 2.4 (`development-plan.md`). Sumber: URS v0.3 Phase 1 "ATM Cash Forecasting Harian" butir 1-2, FSD v1.0 (`.claude/docs/fsd-v1-alignment-2026-10-01.md` §5), `.claude/feature-flows/03-atm-cash-forecasting/feature-flow.md`.

### Kondisi kode saat ini (dicek 2026-11-02)

**DMAA**
- `dmaa_atm_forecast` (`001_baseline_schema.sql`): `terminal_id, dmaa_file_id, periode_pred, denom, amount_replenish bigint, amount_refund bigint, created_at`. Tidak ada kolom jenis order (emergency/planned) dan tidak ada kolom `is_requested`.
- `dmaa_etl.py`: kolom wajib `atm_id, periode_pred, denomination, amount_replenish, location` (`location` tidak disimpan). `amount_refund` selalu 0. Idempoten lewat `dmaa_files.checksum`, **tidak** lewat `import_jobs`. Terminal yang tidak ada di `atms` dilewati dan dicatat.

**Forecast Browser** (`frontend/CompanyPortal-Vite/src/features/vendor-request/ForecastBrowser.tsx`, route `/replenishment/forecast-browser`)
- Memanggil `GET /api/v1/vendor-requests/forecast` (paging, `page_size` ≤ 100) dan `GET /vendor-requests/forecast/summary`. Query `ListForecastForDate`, `CountForecastForDate`, `SummarizeForecastForDate` di `backend/queries/vendor_request.sql`.
- `is_requested` dihitung saat baca: ada item Vendor Request dengan tuple `(terminal_id, periode_pred, denom)` yang sama dan request tidak `cancelled`/`rejected`/`is_canceled`. Baris `is_requested` atau tanpa vendor tidak bisa dipilih; satu pilihan harus satu vendor.
- Query tidak memfilter `atms.is_active`, `blacklisted`, atau status masalah apa pun. `atms` tidak punya kolom status masalah.
- Pilihan diteruskan lewat Zustand `selectionStore` ke `/replenishment/vendor-requests/new`, lalu `POST /vendor-requests` dengan `request_category` dan `is_manual`.

**Vendor Request** (`backend/internal/service/vendor_request*.go`, `backend/internal/handler/vendor_request_handler.go`)
- State machine sendiri (`transitions` di `vendor_request.go`), **tidak** memakai `approval.Orchestrator`. Four-eyes: `ErrSelfApproval` bila checker = pembuat.
- `request_category` wajib: `planned` (replenish H+1), `emergency` (H+0), `additional` (H+0/H+1/H+2), kalender WIB.
- Item `planned` wajib cocok dengan baris DMAA (`ForecastRowExists`); nominal yang diubah tetap diterima dan dicatat sebagai `amount_overrides` di audit. Item `emergency`/`additional` tidak perlu cocok DMAA (`acceptItems`: nominal > 0, refund 0).
- Semua item harus satu vendor dan satu region. Duplikat tuple hanya dicek **di dalam satu payload** (`ErrDuplicateItems`); tidak ada cek lintas request di server.
- `vendor_request_items`: `terminal_id, periode_pred, denom, amount_replenish, amount_refund, brand, lokasi_atm`. Tidak ada kolom sumber item atau nominal asal DMAA.
- Peran route: maker `ADMIN, ATM-USER, BRANCH-ATM-USER`; checker `ADMIN, ATM-SPV, BRANCH-ATM-SPV`; viewer gabungan keduanya.

**Infrastruktur yang dipakai ulang**
- `import_jobs` (mig `019`/`020`): idempoten per `(source, file_hash)`, versi per `(source, processing_date)`, status `superseded`. Helper hanya ada di Python (`backend_python/lib/import_jobs.py`); belum ada helper Go.
- `internal/notification` (in-app + SMTP outbox), `internal/audit` (append-only writer), `approval.Orchestrator` (rantai approval berbasis `users.supervisor_id` + `approval_level`, bukan berbasis peran).
- Migrasi terakhir `035_export_jobs_reports.sql`; nomor bebas berikutnya **`036`**.

### Ruang lingkup

- Status ATM bermasalah dengan masa berlaku, maker-checker, dan riwayat (D2, D3).
- Upload CSV untuk empat list input: ATM bermasalah, complaint/rekonsiliasi, project ATM, adjustment order (D17).
- Aturan order ganda ditegakkan di Forecast Browser dan di server (D12, D13).
- Forecast Browser menjadi tampilan Draft Order ATM: baris DMAA + efek keempat list, dengan alasan per baris (D14).
- Jejak sumber item dan nominal asal DMAA di Vendor Request.
- Notifikasi untuk approval status ATM bermasalah (D19).

### Tidak termasuk

- Forecast engine atau rumus order apa pun di CROWN. DMAA `amount_replenish` tetap final.
- Refund DMAA (menunggu DMAA mengirim datanya).
- Pemenuhan dana / cash pickup (FSD, fase berikutnya).
- Approval kedua untuk order; approval order tetap approval Vendor Request yang sudah ada (D15).
- Penegakan jendela waktu FSD oleh sistem (D16).
- XLSX (excelize belum diadopsi, Golden Rule #1). Hanya CSV.
- Role baru (D18).
- Perubahan pada `dmaa_etl.py` dan isi `dmaa_atm_forecast`.

### Area sensitif (project-context Sec 3 rule 7)

- **Money:** adjustment mengganti nominal order; nominal asal DMAA wajib tetap tersimpan dan terlihat.
- **Maker-checker:** status ATM bermasalah berlaku lintas hari dan mengubah order berikutnya, jadi wajib maker-checker. Mekanisme (`approval.Orchestrator` vs pola Vendor Request) diputuskan di design (D3).
- **State machine Vendor Request:** create, update-items, dan submit mendapat guard baru (ATM bermasalah, order ganda). Order yang sudah berjalan tidak boleh berubah.
- **Migrasi:** tabel baru dan kolom aditif di `vendor_request_items` (`036`).

## Glossary

- **Order_Date (H)**: tanggal rekomendasi yang dilihat di Forecast Browser (`forecast_date` = `dmaa_atm_forecast.periode_pred`).
- **Replenish_Date (R)**: `vendor_requests.replenish_date`, tanggal pengisian yang dipilih maker.
- **Problem_ATM_Flag**: catatan bahwa satu ATM bermasalah, dengan kategori, keterangan, `berlaku_mulai`, `berlaku_sampai` (opsional), dan status `pending_approval | active | rejected | revoke_pending | revoked`.
- **Effective_Flag**: Problem_ATM_Flag berstatus `active` (atau `revoke_pending`) dengan `berlaku_mulai ≤ D ≤ berlaku_sampai` (atau `berlaku_sampai` kosong) untuk tanggal D yang dicek.
- **Input_List**: satu file CSV yang diunggah untuk satu List_Kind. Untuk complaint, project, dan adjustment, satu Input_List berlaku untuk satu Order_Date.
- **List_Kind**: `problem_atm | complaint | project | adjustment`.
- **Current_List**: Input_List terbaru yang `completed` untuk pasangan (List_Kind, Order_Date); versi lama berstatus `superseded`.
- **Active_Emergency_Order**: Vendor Request berkategori `emergency` atau `additional` yang memuat ATM T, status bukan `cancelled`/`rejected`, `is_canceled = false`, dan `replenish_date` di rentang [R-1, R] (D12).
- **Draft_Order**: tampilan gabungan untuk satu Order_Date: baris DMAA (dengan adjustment) + baris complaint + baris project, masing-masing dengan Row_Source, nominal efektif, dan Exclusion_Reason.
- **Row_Source**: `dmaa | adjustment | complaint | project | manual`.
- **Exclusion_Reason**: `problem_atm | duplicate_order | complaint_covered`, atau kosong bila baris bisa dipilih.
- **ProblemAtmAPI**: endpoint backend untuk membuat, mencabut, menyetujui, menolak, dan melihat Problem_ATM_Flag.
- **InputListAPI**: endpoint backend untuk unggah, unduh template, dan riwayat Input_List.
- **ForecastAPI**: `GET /api/v1/vendor-requests/forecast` dan `/forecast/summary`.
- **VendorRequestService**: service create/update-items/submit/approve Vendor Request di `backend/`.
- **ForecastBrowser**: layar Forecast Browser di CompanyPortal, yang menampilkan Draft_Order.
- **ImportJobHelper**: helper Go untuk `import_jobs` (belum ada; dibangun di spec ini).
- **AuditWriter**: `internal/audit` writer, menulis `audit_logs`.
- **NotificationService**: `internal/notification`.
- **Maker_Role**: `ADMIN, ATM-USER, BRANCH-ATM-USER`. **Checker_Role**: `ADMIN, ATM-SPV, BRANCH-ATM-SPV`. **Viewer_Role**: gabungan keduanya (D18).

## Requirements

### Requirement 1: Status ATM bermasalah

**User Story:** Sebagai maker ATM, saya ingin menandai ATM yang tidak bisa bertransaksi untuk rentang tanggal tertentu, supaya ATM itu tidak dipesan selama masalahnya belum selesai.

#### Acceptance Criteria

1. WHEN a Maker_Role user submits a Problem_ATM_Flag with `terminal_id`, kategori (`pending_part | vandalisme | lainnya`), keterangan, `berlaku_mulai`, and optional `berlaku_sampai`, THE ProblemAtmAPI SHALL store it with status `pending_approval`.
2. IF `terminal_id` does not exist in `atms` or the ATM is soft-deleted, THEN THE ProblemAtmAPI SHALL reject the request with 400 and name the field.
3. IF `berlaku_mulai` is before today (Asia/Jakarta) or `berlaku_sampai` is before `berlaku_mulai`, THEN THE ProblemAtmAPI SHALL reject the request with 400.
4. IF keterangan is empty or longer than 500 characters, THEN THE ProblemAtmAPI SHALL reject the request with 400.
5. IF the same ATM already has a flag in status `pending_approval`, `active`, or `revoke_pending` whose date range overlaps the new range, THEN THE ProblemAtmAPI SHALL reject the request with 409.
6. WHEN a Checker_Role user who is not the maker approves a `pending_approval` flag, THE ProblemAtmAPI SHALL set it to `active`.
7. WHEN a Checker_Role user who is not the maker rejects a `pending_approval` flag with a reason, THE ProblemAtmAPI SHALL set it to `rejected` and keep the reason.
8. IF the approving or rejecting user is the maker of that flag or that revoke, THEN THE ProblemAtmAPI SHALL reject the action with 403.
9. WHEN a Maker_Role user requests revocation of an `active` flag with a reason, THE ProblemAtmAPI SHALL set it to `revoke_pending`, and the flag SHALL stay effective until the revocation is approved.
10. WHEN a Checker_Role user who is not the revoke maker approves a revocation, THE ProblemAtmAPI SHALL set the flag to `revoked` and record `revoked_at`; WHEN the revocation is rejected, THE ProblemAtmAPI SHALL return the flag to `active`.
11. THE ProblemAtmAPI SHALL treat a flag as effective for date D only when it is an Effective_Flag for D; flags in `pending_approval`, `rejected`, or `revoked` SHALL have no effect on orders.
12. WHEN any Problem_ATM_Flag changes status, THE AuditWriter SHALL write an `audit_logs` row (actor, action, before, after, IP) in the same database transaction; IF the audit write fails, THEN the status change SHALL roll back.
13. THE ProblemAtmAPI SHALL never hard-delete a Problem_ATM_Flag, and SHALL list flags with filters on `terminal_id`, status, and tanggal berlaku.
14. IF two approve, reject, or revoke actions on the same flag run concurrently, THEN THE ProblemAtmAPI SHALL apply exactly one and answer the other with 409.

### Requirement 2: Upload list input (mekanisme bersama)

**User Story:** Sebagai maker ATM, saya ingin mengunggah list dari tim complaint, unit bisnis, dan analisa saya sendiri dalam satu format yang jelas, supaya list itu masuk ke draft order tanpa diketik ulang.

#### Acceptance Criteria

1. THE InputListAPI SHALL accept CSV uploads (UTF-8, header row) for each List_Kind and SHALL provide a downloadable CSV template per List_Kind.
2. IF a file is larger than 1 MiB or has more than 1000 data rows, THEN THE InputListAPI SHALL reject it with 400 and save nothing.
3. WHEN a file is uploaded, THE InputListAPI SHALL validate every row before saving; IF any row is invalid, THEN THE InputListAPI SHALL reject the whole file, save no rows, and return every error with row number, column, and reason.
4. THE InputListAPI SHALL parse nominal and denom columns as whole IDR integers; IF a value has decimals, a currency symbol, or is not a number, THEN that row SHALL be invalid.
5. IF a file contains the same row key twice (problem_atm: `terminal_id`, Req 3.1; complaint, project, adjustment: (`terminal_id`, `denom`), Req 6.1, 7.1, 8.1), THEN every duplicate row SHALL be invalid.
6. WHEN the same file content (SHA-256) is uploaded again for the same List_Kind and Order_Date, THE InputListAPI SHALL return the existing result without creating new rows, through `import_jobs`.
7. WHEN a different file is uploaded for the same List_Kind (complaint, project, or adjustment) and Order_Date, THE InputListAPI SHALL make it the Current_List and mark the previous list `superseded`, keeping the previous rows readable.
8. IF the Order_Date of a complaint, project, or adjustment upload is before today (Asia/Jakarta), THEN THE InputListAPI SHALL reject it with 400.
9. THE InputListAPI SHALL record per upload: List_Kind, Order_Date (when applicable), uploader, upload time, original file name, file hash, row count, and status, and SHALL show this history to Viewer_Role users.
10. WHEN an upload completes or is rejected, THE AuditWriter SHALL write an `audit_logs` row with List_Kind, Order_Date, file hash, row count, and result.
11. THE ImportJobHelper SHALL register each upload in `import_jobs` with `runtime = 'go'` and `detection_source = 'upload'`, using one `source` value per List_Kind.

### Requirement 3: Upload ATM bermasalah

**User Story:** Sebagai maker ATM, saya ingin menandai banyak ATM bermasalah sekaligus dari satu file, supaya saya tidak perlu mengisi form satu per satu.

#### Acceptance Criteria

1. THE InputListAPI SHALL accept problem_atm rows with columns `terminal_id`, `kategori`, `keterangan`, `berlaku_mulai`, and optional `berlaku_sampai`; the key is `terminal_id`. Each row SHALL be validated with the rules of Req 1.2-1.5.
2. WHEN a problem_atm file passes validation, THE InputListAPI SHALL create one Problem_ATM_Flag per row in `pending_approval`, grouped as one batch.
3. WHEN a Checker_Role user who is not the uploader approves or rejects a batch, THE ProblemAtmAPI SHALL apply the same decision to every flag in that batch, in one transaction.
4. THE ProblemAtmAPI SHALL write one `audit_logs` row per flag for a batch decision.

### Requirement 4: Efek ATM bermasalah pada draft order dan Vendor Request

**User Story:** Sebagai maker dan checker ATM, saya ingin ATM bermasalah otomatis tidak bisa dipesan dan terlihat alasannya, supaya tidak ada order ke ATM yang tidak bisa bertransaksi.

#### Acceptance Criteria

1. WHILE an ATM has an Effective_Flag for the Order_Date, THE ForecastAPI SHALL return every row of that ATM with `exclusion_reason = problem_atm`, the flag kategori and keterangan, and `selectable = false`.
2. THE ForecastBrowser SHALL show such rows with an icon and the text "ATM bermasalah" plus the kategori, and SHALL not allow them to be selected.
3. IF a create or update-items payload contains an ATM with an Effective_Flag on the request's Replenish_Date, THEN THE VendorRequestService SHALL reject it with 400 and name the terminal IDs, for every request_category including manual input.
4. IF a draft Vendor Request contains an ATM with an Effective_Flag on its Replenish_Date at submit time, THEN THE VendorRequestService SHALL reject the submit with 400 and name the terminal IDs.
5. WHILE a Vendor Request is in `pending_approval` or a later status, THE VendorRequestService SHALL not change it because of a Problem_ATM_Flag, and the request detail SHALL show a warning on each item whose ATM has an Effective_Flag on the Replenish_Date.

### Requirement 5: Aturan order ganda

**User Story:** Sebagai checker ATM, saya ingin sistem mencegah order tambahan ke ATM yang sudah punya order emergency/adhoc, supaya ATM tidak diisi dua kali.

#### Acceptance Criteria

1. WHILE ATM T has an Active_Emergency_Order relative to R = Order_Date, THE ForecastAPI SHALL return every DMAA, adjustment, and project `planned` row of T with `exclusion_reason = duplicate_order`, the referenced request numbers, and `selectable = false`.
2. IF a `planned` create or update-items payload contains an ATM with an Active_Emergency_Order relative to the request's Replenish_Date, THEN THE VendorRequestService SHALL reject it with 409 and name the terminal IDs and referenced request numbers.
3. IF an `emergency` or `additional` create payload contains an ATM with an Active_Emergency_Order relative to its Replenish_Date and the payload does not set `acknowledge_duplicate = true`, THEN THE VendorRequestService SHALL reject it with 409 and name the terminal IDs and referenced request numbers.
4. WHEN an `emergency` or `additional` create sets `acknowledge_duplicate = true`, THE VendorRequestService SHALL accept it and record the acknowledged terminal IDs and referenced request numbers in the create audit entry.
5. THE VendorRequestService SHALL evaluate the rule inside the create/update-items transaction so that two concurrent creates for the same ATM cannot both pass.
6. THE VendorRequestService SHALL exclude the request being updated from its own Active_Emergency_Order check.
7. THE ForecastAPI SHALL keep returning `is_requested` with its current meaning.

### Requirement 6: List complaint handling / rekonsiliasi

**User Story:** Sebagai maker ATM, saya ingin ATM dari list complaint otomatis menjadi kandidat order emergency bila DMAA belum merekomendasikannya, supaya keluhan nasabah tertangani tanpa order ganda.

#### Acceptance Criteria

1. THE InputListAPI SHALL accept complaint rows with columns `terminal_id`, `denom`, optional `nominal`, `no_referensi`, and `keterangan`; the key is (`terminal_id`, `denom`).
2. IF `terminal_id` does not exist in `atms` or has no active vendor package on the Order_Date, THEN that row SHALL be invalid.
3. WHEN the Current_List complaint row's ATM already has a DMAA row on the Order_Date (any denom) or an Active_Emergency_Order relative to the Order_Date, THE ForecastAPI SHALL return it with `exclusion_reason = complaint_covered`, the covering source, and `selectable = false`.
4. WHEN a Current_List complaint row is not covered, THE ForecastAPI SHALL return it as a Draft_Order row with Row_Source `complaint` and category `emergency`.
5. WHERE a complaint row has no nominal, THE ForecastBrowser SHALL mark it "Nominal belum diisi", and the Vendor Request create form SHALL require the maker to fill a nominal > 0 before saving.

### Requirement 7: List project ATM

**User Story:** Sebagai maker ATM, saya ingin project ATM dari unit bisnis (replace, ATM baru, relokasi) masuk sebagai kandidat order, supaya ATM itu terisi pada tanggal project.

#### Acceptance Criteria

1. THE InputListAPI SHALL accept project rows with columns `terminal_id`, `jenis_project` (`replace | new | relokasi | lainnya`), `kategori` (`emergency | planned`), `denom`, optional `nominal`, and `keterangan`; the key is (`terminal_id`, `denom`).
2. IF `terminal_id` does not exist in `atms` or has no active vendor package on the Order_Date, THEN that row SHALL be invalid with the reason "ATM belum terdaftar di master" or "ATM belum punya paket vendor aktif".
3. WHEN a Current_List project row is valid, THE ForecastAPI SHALL return it as a Draft_Order row with Row_Source `project` and the row's kategori.
4. WHERE a project row has no nominal, THE ForecastBrowser and the Vendor Request create form SHALL behave as in Req 6.5.

### Requirement 8: Adjustment order

**User Story:** Sebagai maker ATM, saya ingin mengganti nominal DMAA untuk ATM tertentu lewat list adjustment beralasan, supaya order mengikuti kondisi lapangan tanpa menghilangkan angka DMAA.

#### Acceptance Criteria

1. THE InputListAPI SHALL accept adjustment rows with columns `terminal_id`, `denom`, `nominal`, and `alasan`; the key is (`terminal_id`, `denom`).
2. IF no `dmaa_atm_forecast` row exists for (`terminal_id`, Order_Date, `denom`), THEN that row SHALL be invalid.
3. IF `nominal` is not a whole number greater than 0, or `alasan` is empty or longer than 500 characters, THEN that row SHALL be invalid.
4. WHEN a Current_List adjustment row exists for a DMAA row, THE ForecastAPI SHALL return that row with Row_Source `adjustment`, `amount` = adjustment nominal, `amount_dmaa` = DMAA `amount_replenish`, the difference, and the alasan.
5. THE InputListAPI SHALL never modify `dmaa_atm_forecast`.

### Requirement 9: Draft Order ATM di Forecast Browser

**User Story:** Sebagai maker ATM, saya ingin melihat satu draft order yang menggabungkan DMAA dan keempat list beserta alasannya, supaya saya tahu kenapa ATM dikeluarkan, ditambahkan, atau nominalnya diganti sebelum membuat Vendor Request.

#### Acceptance Criteria

1. THE ForecastAPI SHALL return, for one Order_Date, DMAA rows plus Current_List complaint and project rows, each with Row_Source, category, `amount` (effective), `amount_dmaa` (null for non-DMAA rows), Exclusion_Reason, and `selectable`.
2. WHEN more than one Exclusion_Reason applies to a row, THE ForecastAPI SHALL return the first in the order `problem_atm`, `duplicate_order`, `complaint_covered`.
3. THE ForecastAPI SHALL keep every existing field and filter, and SHALL add filters on Row_Source and on "dikecualikan / bisa dipilih".
4. THE ForecastAPI SHALL compute `count` and pagination over the same merged row set it returns.
5. THE ForecastAPI summary SHALL sum only selectable rows using the effective amount, and SHALL show the number of excluded ATMs per (vendor, region) group.
6. THE ForecastBrowser SHALL show each row's Row_Source and Exclusion_Reason with an icon and text (never color alone), and the adjustment difference against DMAA.
7. IF a selection mixes categories, THEN THE ForecastBrowser SHALL block "Buat Vendor Request" and explain that one request holds one category; the existing single-vendor rule stays.
8. THE ForecastBrowser SHALL show the FSD daily time windows (upload 10:00-11:30, adjustment 11:45-12:00, merge 11:35-12:15, final order 12:30 WIB) as guidance text only.
9. All amounts SHALL be shown as IDR, `tabular-nums`, right-aligned.

### Requirement 10: Jejak sumber item di Vendor Request

**User Story:** Sebagai checker ATM, saya ingin melihat dari mana tiap item order berasal dan berapa angka DMAA aslinya, supaya saya bisa menyetujui order dengan dasar yang jelas.

#### Acceptance Criteria

1. WHEN a Vendor Request item is created, THE VendorRequestService SHALL store its Row_Source, its `amount_dmaa` (DMAA `amount_replenish` when a DMAA row exists for the tuple, else null), and a reference to the Input_List row when the source is a list.
2. IF an item claims Row_Source `adjustment`, `complaint`, or `project` but the referenced row is not in the Current_List for that Order_Date, THEN THE VendorRequestService SHALL reject the payload with 400.
3. THE VendorRequestService SHALL keep the existing `amount_overrides` audit for `planned` items whose amount differs from both `amount_dmaa` and the adjustment nominal.
4. THE Vendor Request detail SHALL show per item the Row_Source, `amount_dmaa`, the adjustment alasan, and the list reference.
5. Existing Vendor Request items SHALL keep their values; their Row_Source and `amount_dmaa` SHALL be null and shown as "-".
6. THE create audit entry SHALL record the count of items per Row_Source.

### Requirement 11: Otorisasi peran

**User Story:** Sebagai pemilik sistem, saya ingin hanya peran ATM yang berwenang yang bisa mengubah list dan status ATM bermasalah, supaya order tidak bisa dipengaruhi pihak lain.

#### Acceptance Criteria

1. THE ProblemAtmAPI and InputListAPI SHALL allow upload, create flag, and request revocation only to Maker_Role.
2. THE ProblemAtmAPI SHALL allow approve and reject only to Checker_Role.
3. THE ProblemAtmAPI and InputListAPI SHALL allow read, history, and template download only to Viewer_Role.
4. THE system SHALL enforce these rules both at route middleware and in the service layer; any other role, including every vendor role, SHALL receive 403.

### Requirement 12: Notifikasi

**User Story:** Sebagai checker ATM, saya ingin tahu saat ada status ATM bermasalah yang menunggu persetujuan, supaya keputusan tidak tertunda.

#### Acceptance Criteria

1. WHEN a Problem_ATM_Flag, batch, or revocation enters a pending status, THE NotificationService SHALL send an in-app and email notification to active Checker_Role users other than the maker.
2. WHEN a flag, batch, or revocation is approved or rejected, THE NotificationService SHALL notify the maker in-app.
3. THE notification SHALL be written in the same transaction as the status change through the existing outbox; IF email delivery fails later, THEN the status change SHALL stay committed.
4. Upload validation results SHALL be returned in the upload response only, without notification.

### Requirement 13: Kontrak API, data, dan non-regresi

**User Story:** Sebagai tim pengembang, saya ingin fitur ini menambah perilaku tanpa merusak order yang sudah berjalan, supaya rilis aman.

#### Acceptance Criteria

1. THE new ProblemAtmAPI and InputListAPI endpoints and the changed ForecastAPI and Vendor Request fields SHALL be described in `backend/api/openapi.yaml`, and the CompanyPortal types SHALL be regenerated with `gen:api`.
2. WHILE no Problem_ATM_Flag, no Input_List, and no Active_Emergency_Order exist, THE ForecastAPI and VendorRequestService SHALL return the same rows, amounts, and decisions as before this feature, apart from the new fields.
3. THE migration `036` SHALL be additive only: new tables plus nullable columns on `vendor_request_items`; it SHALL not change existing rows of `vendor_requests`, `vendor_request_items`, or `dmaa_atm_forecast`.
4. All money SHALL be stored as `bigint` whole IDR (same unit as `dmaa_atm_forecast`) and sent as JSON numbers in the same form as existing Vendor Request amounts; no float.
5. All timestamps SHALL be `timestamptz`; all date rules (today, H, R, H-1) SHALL use the Asia/Jakarta calendar.
6. Every new behavior SHALL ship with tests: service unit tests for Req 1, 4, 5, 8, 10; property tests for the exclusion order and the Active_Emergency_Order window; integration tests against a real Postgres for the migration and the Vendor Request guards; frontend component tests for the Draft_Order view.

## Open Questions

Saran Kiro untuk ke-19 open question di intent. Status semua: **menunggu PO**. Requirements di atas ditulis sesuai saran ini; bila PO memilih lain, requirement terkait diubah dulu sebelum design.

| ID | Pertanyaan intent | Saran Kiro | Dipakai di | Status |
|---|---|---|---|---|
| D1 | Q1 Keempat list sekaligus? | Satu spec untuk keempatnya. Urutan task: ATM bermasalah + aturan order ganda dulu (bisa dirilis sendiri), lalu adjustment, complaint, project. | Seluruh | menunggu PO |
| D2 | Q2 List harian atau status? | Status dengan `berlaku_mulai` dan `berlaku_sampai` opsional, berlaku sampai berakhir atau dicabut. Masalah ATM biasanya berhari-hari, jadi list harian membuat upload berulang. | Req 1, 3 | menunggu PO |
| D3 | Q3 Siapa menetapkan, maker-checker? | Maker_Role menetapkan, Checker_Role menyetujui, termasuk pencabutan. Mekanisme dipilih di design: `approval.Orchestrator` (aturan project-context, tapi checker-nya atasan di `users.supervisor_id`) atau pola peran seperti Vendor Request (checker = SPV mana pun). Kiro condong ke pola peran supaya checker sama dengan checker order; ini perlu persetujuan karena menyimpang dari "jangan buat state machine approval kedua". | Req 1, 3, 11 | menunggu PO |
| D4 | Q4 Disembunyikan atau ditandai? | Tetap tampil dengan penanda dan alasan, tidak bisa dipilih. Pengguna perlu tahu kenapa ATM tidak ada di order. | Req 4.1-4.2, 9.6 | menunggu PO |
| D5 | Q5 ATM ditandai saat sudah di Vendor Request berjalan? | Draft: create/update-items/submit ditolak sampai item dihapus. `pending_approval` dan sesudahnya: tidak diubah otomatis, tampil peringatan; checker bisa reject atau cancel. | Req 4.3-4.5 | menunggu PO |
| D6 | Q6 Blok input manual juga? | Ya, semua kategori termasuk manual, tanpa override. Yang perlu dikecualikan cukup dicabut lewat maker-checker. | Req 4.3 | menunggu PO |
| D7 | Q7 Nominal kosong, siapa mengisi? | Kolom nominal opsional di list. Bila kosong, maker mengisi di form Vendor Request (form sudah mewajibkan nominal > 0 untuk emergency/additional). | Req 6.5, 7.4 | menunggu PO |
| D8 | Q8 ATM baru wajib di master? | Wajib ada di `atms` dan punya paket vendor aktif pada Order_Date, karena Vendor Request harus tahu vendor dan region. Bila belum, baris ditolak dengan alasan jelas; ATM didaftarkan dulu lewat master data. | Req 6.2, 7.2 | menunggu PO |
| D9 | Q9 Emergency vs planned? | Pakai `request_category` yang ada: emergency = H+0, planned = H+1. Complaint selalu emergency (URS). Project memilih lewat kolom `kategori` di file. | Req 6.4, 7.1, 7.3 | menunggu PO |
| D10 | Q10 Unit adjustment? | Per (ATM, denom) untuk Order_Date, sama dengan identitas baris DMAA. "Area tertentu" dipenuhi dengan mengunggah list berisi ATM area itu. | Req 8.1-8.2 | menunggu PO |
| D11 | Q11 Batas nominal, alasan wajib? | Alasan wajib (≤ 500 karakter), nominal bulat > 0, tanpa batas atas. Untuk mengeluarkan ATM pakai status ATM bermasalah, bukan adjustment 0. Selisih terhadap DMAA selalu ditampilkan. | Req 8.3-8.4 | menunggu PO |
| D12 | Q12 Definisi "aktif di H0"? | Active_Emergency_Order: request `emergency`/`additional` memuat ATM itu (denom apa pun), status bukan `cancelled`/`rejected`, `is_canceled = false` (sama dengan `is_requested`), `replenish_date` di [R-1, R], R = Replenish_Date order baru (di Forecast Browser R = Order_Date). Draft ikut dihitung. | Glossary, Req 5 | menunggu PO |
| D13 | Q13 Otomatis atau peringatan? | Otomatis untuk baris planned (DMAA/adjustment/project planned). Untuk create emergency/additional: ditolak kecuali maker mengonfirmasi (`acknowledge_duplicate`), dan konfirmasi itu tercatat di audit. | Req 5.1-5.4 | menunggu PO |
| D14 | Q14 Draft order menggantikan Forecast Browser? | Forecast Browser menjadi tampilan Draft_Order. Tidak ada entitas draft baru; Vendor Request draft yang sudah ada tetap jadi draft order resmi. | Req 9 | menunggu PO |
| D15 | Q15 Approval berjenjang? | Approval order = approval Vendor Request yang sudah ada (satu level, SPV). List complaint/project/adjustment tidak punya approval sendiri karena efeknya baru berlaku lewat Vendor Request yang di-approve checker; status ATM bermasalah punya maker-checker sendiri karena berlaku lintas hari. | Tidak termasuk, Req 1, 10 | menunggu PO |
| D16 | Q16 Jendela waktu ditegakkan? | Panduan saja, ditampilkan di layar. Penegakan bisa ditambah nanti bila operasional memintanya. | Req 9.8 | menunggu PO |
| D17 | Q17 Format file? | CSV dengan template dari CROWN (pola import master data). Kolom per list di Req 3, 6, 7, 8. Perlu contoh file nyata dari tim complaint dan unit bisnis untuk dicocokkan sebelum design final. | Req 2, 3, 6, 7, 8 | menunggu PO |
| D18 | Q18 ATM Forecasting Unit? | Dipetakan ke peran yang ada: maker ATM-USER/BRANCH-ATM-USER, checker ATM-SPV/BRANCH-ATM-SPV, plus ADMIN, sama dengan Vendor Request. Tidak ada role baru. | Glossary, Req 11 | menunggu PO |
| D19 | Q19 Siapa menerima notifikasi? | Checker_Role saat status ATM bermasalah menunggu approval; maker saat diputuskan. Hasil validasi upload hanya di layar. Tidak ada notifikasi "draft siap" karena tidak ada entitas draft baru (D14). | Req 12 | menunggu PO |

## Dependency & Scope Note

- Bergantung pada 2.1 (DMAA load) dan 0.2 (`import_jobs`), keduanya selesai.
- Mengubah query Forecast Browser yang juga dipakai summary vendor × region (`7f6d16b`) dan guard Vendor Request yang dipakai alur vault (2.2a-2.2d). Regresi alur itu masuk Req 13.2.
- Helper Go `import_jobs` pertama dibangun di sini; Phase 3 (invoice) dan 4 (escrow) nanti memakainya.
