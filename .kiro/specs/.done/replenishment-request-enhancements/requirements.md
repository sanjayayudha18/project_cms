# Requirements Document

## Introduction

Spec ini adalah **enhancement bertarget** pada alur ATM replenishment-request di aplikasi internal CMS (`frontend/CompanyPortal-Vite`, tema "Merah Sirih") dan ATM backend (`backend/`, port 8080, flat JSON). Alur dasarnya — Forecast Browser (`/replenishment/forecast-browser`), Vendor Request create / list / detail, dan state machine maker-checker (`draft → pending_approval → approved/rejected → …`) — sudah ada dari spec `request-replenish-to-vendor` (di `.kiro/specs/done/`) dan diperluas oleh spec `cit-vendor-request-enhancements` ("CIT-2"). Spec ini **membangun di atas keduanya** dan tidak menspesifikasi ulang perilaku yang sudah ada.

Lima perubahan diminta:

1. **Pemilihan tipe request (Request Type) saat membuat replenishment request.** Operator saat ini belum bisa memilih tipe. Tambahkan tiga tipe — Planned (berbasis data DMAA, isi H+1), Emergency (mungkin/tidak ada di DMAA, isi H+0), Additional (tidak ada di DMAA, isi H+0/H+1/H+2 sesuai pilihan operator) — dengan perilaku tanggal yang dapat dipilih per tipe dan aturan keterkaitan DMAA per tipe. Ini **memakai ulang** kolom `request_category` yang sudah ada (migration 034: `planned | emergency | additional`); fokusnya pada UI pemilihan tipe dan aturan tanggal.

2. **Tombol Tolak (Reject) hilang untuk role ATM-SPV.** Saat login sebagai role SPV-ATM (`ATM-SPV` / `BRANCH-ATM-SPV`) dan melihat request yang berstatus dapat-ditolak (`pending_approval`), tombol Tolak tidak muncul. Tombol harus terlihat dan berfungsi untuk role Checker sesuai aturan four-eyes yang sudah ada.

3. **SPV membatalkan request yang sudah di-approve.** Request yang sudah berstatus `approved` harus dapat dibatalkan oleh SPV (saat ini `checkActor` hanya mengizinkan cancel pada `draft`/`pending_approval`). Pembatalan langsung oleh SPV (Checker), wajib mengisi alasan, dan tercatat di audit_log — tanpa gate maker-checker kedua.

4. **Perubahan format Request-ID.** Format sekarang (dari CIT-2) tanpa pemisah: `REP` + prefix vendor 3 karakter + `YYYYMMDD` + urutan 3 digit, digabung tanpa separator (mis. `REPTAG20260914001`). Format baru menyisipkan tanda hubung: `REP-<vendorPrefix 3 digit>-<YYYYMMDD>-<urutan 3 digit>` (mis. `REP-TAG-20260914-001`). Ini murni perubahan format/tampilan string generator; semantik keunikan, scope urutan (per-vendor-per-hari), dan retry yang sudah ada dipertahankan.

5. **Kolom tabel Forecast Browser.** Pada tabel `/replenishment/forecast-browser`: HAPUS kolom "Amount Refund", dan TAMBAH tiga kolom — "PriorityClass", "Paket", dan "Escrow".

Scope adalah halaman Forecast Browser, halaman Vendor Request create / detail, modul `vendor_requests` di ATM backend (SQL, service, handler), dan migrasi additive yang diperlukan. Eksekusi downstream (`approved → processing → completed`) di luar scope.

## Dependency & Scope Note

- **Memakai ulang** rantai yang ada: SQL `backend/queries/vendor_request.sql` → service `internal/service/vendor_request.go` + `vendor_request_actions.go` → handler `internal/handler/vendor_request_*.go` → frontend `features/vendor-request/` (`ForecastBrowser.tsx`, `ForecastTable.tsx`, `VendorRequestCreate.tsx`, `VendorRequestDetail.tsx`, `VendorRequestList.tsx`, `StatusBadge.tsx`, `hooks.ts`, `api.ts`, `types.ts`).
- **Overlap dengan `cit-vendor-request-enhancements` (CIT-2).** Kolom `request_category`, `replenish_date`, `is_manual`, dan `is_canceled` sudah ditambahkan oleh **migration 034** (`034_vendor_requests_cit2_columns.sql`); `request_category` punya CHECK yang menerima `NULL` atau `planned | emergency | additional`. Generator nomor `REP<prefix><date><seq>` dan tabel `vendor_request_number_seq` (scope per-vendor-per-hari, retry sampai 5x) juga sudah ada. Spec ini **tidak** membuat tabel baru untuk hal-hal tersebut — ia memakai ulang kolom, generator, dan pola yang sudah ada.
- **Klarifikasi yang sudah diselesaikan bersama user (dipakai sebagai keputusan, bukan open question):**
  - **Request-ID (Req 4):** cukup mengubah format string ke ber-hyphen; scope/urutan/keunikan tetap seperti sekarang. Nomor lama tanpa separator TIDAK ditulis ulang.
  - **Cancel-after-approval (Req 3):** SPV (Checker) langsung membatalkan, wajib alasan + audit_log, tanpa gate maker-checker kedua.
  - **Sumber kolom Forecast (Req 5):** PriorityClass = `atms.priority_class` (migration 030); Paket = `vendor_packages.code` (label 'PAKET 3/4/5' via `atm_vendor_packages → vendor_packages`); Escrow = saldo escrow per ATM (`itm_replenish` / query `atm_portal.sql`).
  - **Role & Request Type (Req 1, 2):** role Checker adalah `ATM-SPV` dan `BRANCH-ATM-SPV`; bug reject murni tombol Tolak tidak muncul. Request Type memakai ulang `request_category`.
- **Perubahan skema** (jika ada) HARUS additive, forward-only, di bawah `backend/migrations/`, diusulkan dan dikonfirmasi lebih dulu (Golden Rule). Berdasarkan klarifikasi di atas, kolom yang dibutuhkan sudah ada di migration 034 — desain akan mengonfirmasi tidak ada migrasi baru yang wajib kecuali untuk memperluas himpunan status yang dapat dibatalkan (Req 3) bila diperlukan; hal ini diputuskan di design.md.
- **ATM backend tetap flat JSON** (project-context Sec 2 / Sec 4): endpoint `vendor_requests` diperluas **secara additive** — tanpa rename atau penghapusan field — untuk kompatibilitas wire dengan frontend.
- **sqlc caveat** (project-context): `sqlc generate` bisa menulis ulang file generated lain yang tidak terkait. Regenerate lalu simpan hanya diff `vendor_request*.sql.go`, atau edit manual mengikuti konvensi output sqlc seperti spec sebelumnya. Ditandai untuk tasks.md.

## Open Questions (untuk diselesaikan di design.md)

- **OQ1 — Migrasi untuk cancelable status.** State machine `nextState` saat ini menentukan transisi `actionCancel`. Untuk mengizinkan cancel pada `approved` (Req 3), desain harus memutuskan apakah cukup memperluas tabel transisi state machine di service (tanpa migrasi) atau perlu perubahan skema. Diputuskan di design.md; keputusan awal: perluasan di service saja, tanpa migrasi baru.
- **OQ2 — Presisi & unit nilai Escrow.** Nilai Escrow disimpan sebagai numeric/integer minor units per project-context Sec 5. Desain menetapkan tipe wire (string vs number) dan pembulatan tampilan pada kolom Forecast, konsisten dengan pola `atm_portal.sql`.
- **OQ3 — Sumber Paket saat satu ATM punya >1 paket.** `031_seed_vendor_packages.sql` menunjukkan satu branch dapat memiliki beberapa label PAKET. Desain menetapkan aturan pemilihan paket aktif per ATM (mis. paket aktif pada `atm_vendor_packages`) agar kolom "Paket" deterministik per baris forecast.

## Glossary

- **Company_Portal**: Aplikasi internal CMS `frontend/CompanyPortal-Vite`, tema "Merah Sirih", login LDAP.
- **Forecast_Browser_Page**: Route `/replenishment/forecast-browser`, dirender oleh `ForecastBrowser.tsx`, role-gated ke `ATM-USER`, `ATM-SPV`, `BRANCH-ATM-USER`, `BRANCH-ATM-SPV`.
- **Forecast_Table**: Instance TanStack Table v8 di `ForecastTable.tsx` yang menampilkan baris rekomendasi DMAA (Forecast_Row).
- **Forecast_Row**: Satu baris rekomendasi — kunci `(terminal_id, periode_pred, denom)` dengan `amount_replenish`, dan kolom konteks per ATM.
- **Forecast_API**: Endpoint ATM backend `GET /api/v1/vendor-requests/forecast`.
- **Vendor_Request**: Satu baris `vendor_requests` — order replenishment kas ke vendor CIT dengan state machine maker-checker.
- **Vendor_Request_Create_Page**: Route `/replenishment/vendor-requests/new`, dirender oleh `VendorRequestCreate.tsx`.
- **Vendor_Request_Detail_Page**: Route `/replenishment/vendor-requests/{id}`, dirender oleh `VendorRequestDetail.tsx`.
- **Vendor_Request_List_Page**: Route `/replenishment/vendor-requests`, dirender oleh `VendorRequestList.tsx`.
- **Vendor_Request_Service**: Backend `VendorRequestService` (`internal/service/vendor_request*.go`) yang memiliki validasi, state machine, dan penulisan audit.
- **Request_Type**: Klasifikasi request yang dipilih operator — tepat satu dari Planned_Type, Emergency_Type, atau Additional_Type; dipetakan ke kolom `request_category` (`planned | emergency | additional`).
- **Planned_Type**: Request_Type berbasis data DMAA untuk pengisian H+1 (hari berikutnya).
- **Emergency_Type**: Request_Type untuk pengisian H+0 (tanggal request itu sendiri); item mungkin ada atau tidak ada di DMAA.
- **Additional_Type**: Request_Type yang tidak ada di DMAA, untuk pengisian H+0, H+1, atau H+2 sesuai pilihan operator.
- **Replenish_Date**: Tanggal pengiriman kas ke vendor/ATM yang dipilih operator; disimpan pada `vendor_requests.replenish_date`.
- **H_Zero / H_Plus_One / H_Plus_Two**: Tanggal kalender hari-ini (H+0), hari-berikutnya (H+1), dan dua-hari-berikutnya (H+2) dalam zona waktu Asia/Jakarta.
- **Request_Number**: Identifier unik yang bisa dibaca pada `vendor_requests.request_number`; formatnya berubah per Requirement 4.
- **Vendor_Prefix**: Kode 3 karakter dalam Request_Number yang mengidentifikasi vendor CIT (mis. TAG / ADV / BJK), diturunkan dari vendor request per pemetaan CIT-2 yang sudah ada.
- **Number_Sequence**: Urutan 3 digit dalam Request_Number, di-scope per-vendor-per-hari melalui tabel `vendor_request_number_seq`.
- **Maker_Role**: Himpunan role yang membuat/submit/revisi request — `ADMIN`, `ATM-USER`, `BRANCH-ATM-USER`.
- **Checker_Role**: Himpunan role yang approve/reject/cancel — `ADMIN`, `ATM-SPV`, `BRANCH-ATM-SPV`; role SPV.
- **Reject_Action**: Aksi Checker yang memindahkan Vendor_Request dari `pending_approval` ke `rejected`.
- **Cancel_Action**: Aksi yang membatalkan Vendor_Request (mengeset `is_canceled = true` dan/atau status `cancelled`) tanpa menghapus baris.
- **Priority_Class**: Klasifikasi prioritas per ATM — `atms.priority_class` (migration 030); tampil sebagai kolom "PriorityClass".
- **Paket**: Label paket layanan vendor pada ATM — `vendor_packages.code` (mis. 'PAKET 3'/'PAKET 4'/'PAKET 5') via `atm_vendor_packages → vendor_packages`; tampil sebagai kolom "Paket".
- **Escrow_Balance**: Saldo escrow per ATM (`itm_replenish` / query `atm_portal.sql`); tampil sebagai kolom "Escrow".
- **Audit_Log**: Trail append-only `audit_logs` yang ditulis pada setiap perubahan state (project-context Sec 4), via `audit.Writer` transaction-scoped.

## Requirements

### Requirement 1: Pemilihan Request Type pada pembuatan replenishment request

**User Story:** As an ATM operator, I want to choose a request type (Planned, Emergency, or Additional) when creating a replenishment request, so that the allowed replenish date and DMAA-linkage follow the operational intent of the request.

#### Acceptance Criteria

1. THE Vendor_Request_Create_Page SHALL provide a Request_Type control that requires the operator to select exactly one of Planned_Type, Emergency_Type, or Additional_Type before submitting a replenishment request.
2. IF the operator attempts to submit a replenishment request without any Request_Type selected, THEN THE Vendor_Request_Create_Page SHALL block submission and display an error indication identifying the missing Request_Type selection.
3. WHEN the operator selects Planned_Type, THE Vendor_Request_Create_Page SHALL set the Replenish_Date to H_Plus_One in the Asia/Jakarta time zone and SHALL treat the request as backed by DMAA data.
4. WHEN the operator selects Emergency_Type, THE Vendor_Request_Create_Page SHALL set the Replenish_Date to H_Zero in the Asia/Jakarta time zone.
5. WHEN the operator selects Additional_Type, THE Vendor_Request_Create_Page SHALL allow the operator to choose a Replenish_Date of exactly H_Zero, H_Plus_One, or H_Plus_Two in the Asia/Jakarta time zone, and SHALL offer no other selectable date value.
6. WHEN a Planned_Type request is submitted, IF one or more items do not match a `dmaa_atm_forecast` row, THEN THE Vendor_Request_Service SHALL reject the request, identify each non-matching item, and SHALL NOT persist the Vendor_Request.
7. WHEN an Emergency_Type request is submitted, THE Vendor_Request_Service SHALL accept items whether or not each item matches a `dmaa_atm_forecast` row.
8. WHEN an Additional_Type request is submitted, THE Vendor_Request_Service SHALL accept items that do not match any `dmaa_atm_forecast` row.
9. WHEN the operator saves or submits a request, THE Vendor_Request_Service SHALL persist the selected Request_Type in the existing `request_category` column as exactly one of the values `planned`, `emergency`, or `additional`.
10. IF the submitted Replenish_Date is inconsistent with the selected Request_Type (an Emergency_Type request with a Replenish_Date other than H_Zero, a Planned_Type request with a Replenish_Date other than H_Plus_One, or an Additional_Type request with a Replenish_Date outside {H_Zero, H_Plus_One, H_Plus_Two}), THEN THE Vendor_Request_Service SHALL reject the request with a validation error naming the conflicting Replenish_Date and Request_Type, and SHALL NOT persist the Vendor_Request.
11. IF a submitted Request_Type value is not one of `planned`, `emergency`, or `additional`, THEN THE Vendor_Request_Service SHALL reject the request with a validation error identifying the Request_Type field and SHALL NOT persist the Vendor_Request.
12. THE Vendor_Request_Service SHALL enforce the Request_Type and item rules at both the middleware role gate and the service layer.
13. WHEN a Vendor_Request is created, THE Vendor_Request_Service SHALL write exactly one Audit_Log entry recording the operator identity, the selected Request_Type, and the UTC timestamp.
14. THE Request_Type control SHALL use the design-system tokens and existing form components consistent with the Merah Sirih theme, with a minimum 44px by 44px touch target on every interactive Request_Type control.
15. THE Vendor_Request_Detail_Page SHALL display the selected Request_Type as a labeled field using plain text, and SHALL NOT signal the Request_Type by color alone.

### Requirement 2: Tombol Tolak terlihat dan berfungsi untuk role SPV-ATM

**User Story:** As an ATM SPV (Checker), I want the Reject button to be visible and working on a request awaiting my approval, so that I can reject a replenishment request that should not proceed.

#### Acceptance Criteria

1. WHILE a Vendor_Request has status `pending_approval`, THE Vendor_Request_Detail_Page SHALL display a Reject control to an authenticated actor whose role is in Checker_Role and who is not the request creator.
2. WHEN the current actor holds the `ATM-SPV` or `BRANCH-ATM-SPV` role and is viewing a `pending_approval` Vendor_Request they did not create, THE Vendor_Request_Detail_Page SHALL render the Reject control alongside the Approve control.
3. WHEN a Checker_Role actor activates the Reject control, THE Vendor_Request_Detail_Page SHALL require a rejection reason of 1 to 500 non-whitespace characters before allowing the reject to be confirmed.
4. IF a Checker_Role actor attempts to confirm a reject with a reason that is empty, whitespace-only, or exceeds 500 characters, THEN THE Vendor_Request_Detail_Page SHALL block confirmation, display an error identifying the invalid reason, and SHALL NOT submit the reject to the Vendor_Request_Service.
5. WHEN a Checker_Role actor confirms a reject with a valid rejection reason on a `pending_approval` request, THE Vendor_Request_Service SHALL transition the Vendor_Request to `rejected`, persist the rejection reason, and write exactly one Audit_Log entry recording the actor, the action, the prior status, the resulting status, the actor IP, and the UTC timestamp.
6. IF a reject is submitted against a Vendor_Request whose status is no longer `pending_approval`, THEN THE Vendor_Request_Service SHALL reject the action with a conflict error and preserve the current status.
7. IF the current actor is the request creator, THEN THE Vendor_Request_Detail_Page SHALL NOT display the Reject control, and THE Vendor_Request_Service SHALL reject a reject attempt by the creator with an authorization error (four-eyes rule preserved).
8. IF a role that is not in Checker_Role attempts to reject a Vendor_Request, THEN THE Vendor_Request_Service SHALL reject the action with an authorization error, enforced at both the middleware role gate and the service layer, and SHALL leave the Vendor_Request status unchanged.
9. WHILE a Vendor_Request has a status other than `pending_approval`, THE Vendor_Request_Detail_Page SHALL NOT display the Reject control.
10. THE Reject control and its confirmation dialog SHALL pair the reject affordance with visible text and an icon, and SHALL NOT signal the destructive nature of the action by color alone, using the danger button pattern reserved for destructive actions.

### Requirement 3: SPV membatalkan request yang sudah di-approve

**User Story:** As an ATM SPV (Checker), I want to cancel a replenishment request that has already been approved, so that an approved order that must not proceed is withdrawn while its record and audit trail are preserved.

#### Acceptance Criteria

1. WHILE a Vendor_Request has status `approved`, THE Vendor_Request_Detail_Page SHALL display a Cancel control to an authenticated actor whose role is in Checker_Role, and SHALL hide the Cancel control from all other authenticated actors.
2. WHEN a Checker_Role actor activates the Cancel control on an `approved` Vendor_Request, THE Vendor_Request_Detail_Page SHALL require a cancellation reason of 1 to 500 non-whitespace characters before allowing the cancel to be confirmed.
3. IF a Checker_Role actor attempts to confirm a cancel with a reason that is empty, whitespace-only, or exceeds 500 characters, THEN THE Vendor_Request_Detail_Page SHALL block confirmation, display an error identifying the invalid reason, and SHALL NOT submit the cancel to the Vendor_Request_Service.
4. WHEN a Checker_Role actor confirms a cancel with a valid reason on an `approved` Vendor_Request, THE Vendor_Request_Service SHALL set `is_canceled` to true, record the cancellation reason, and SHALL NOT delete the `vendor_requests` row or any of its `vendor_request_items`.
5. WHEN a cancel of an approved request completes successfully, THE Vendor_Request_Service SHALL apply the effect directly without requiring a second maker-checker approval gate.
6. WHEN a cancel of an approved request completes successfully, THE Vendor_Request_Service SHALL write exactly one Audit_Log entry recording the actor identifier, the action value `cancel`, the prior status and prior `is_canceled` state, the resulting state, the cancellation reason, the actor IP, and the UTC timestamp.
7. THE Vendor_Request_Service SHALL permit cancel of an `approved` Vendor_Request only for a Checker_Role actor, and SHALL deny the action for all other roles at both the middleware role gate and the service layer, returning an authorization error and preserving the existing state.
8. IF an actor attempts to cancel a Vendor_Request whose `is_canceled` is already true, THEN THE Vendor_Request_Service SHALL reject the action with a conflict error, preserve the existing state, and SHALL NOT write a duplicate cancel Audit_Log entry.
9. WHILE a Vendor_Request has `is_canceled` true, THE Vendor_Request_Service SHALL exclude it from active-flow reads, defined as approval queues and default active listings, unless the request explicitly sets a canceled filter to true.
10. WHEN the Vendor_Request_Detail_Page or Vendor_Request_List_Page renders a Vendor_Request with `is_canceled` true, THE page SHALL display a canceled badge or label paired with visible text and SHALL NOT signal the canceled state by color alone.
11. THE list and detail responses SHALL expose the `is_canceled` state and the cancellation reason additively in the existing flat JSON shape, with no renames or removals of existing fields.
12. THE Vendor_Request_Service SHALL continue to permit the existing cancel behavior on `draft` and `pending_approval` requests unchanged, extending the set of cancelable statuses to include `approved` for Checker_Role actors only.

### Requirement 4: Perubahan format Request-ID (ber-hyphen)

**User Story:** As an operator and an auditor, I want the request number to be displayed with hyphen separators, so that the vendor prefix, date, and sequence are visually distinct and easier to read.

#### Acceptance Criteria

1. WHEN a Vendor_Request is created, THE Vendor_Request_Service SHALL generate a Request_Number matching the pattern `REP-<Vendor_Prefix>-<YYYYMMDD>-<Number_Sequence>`, where Vendor_Prefix is exactly 3 uppercase alphabetic characters (A to Z), the date segment is exactly 8 numeric digits in `YYYYMMDD` format representing the creation date in Asia/Jakarta time, and Number_Sequence is exactly 3 numeric digits, zero-padded, in the range `001` to `999` (for example `REP-TAG-20260914-001`).
2. THE Vendor_Request_Service SHALL derive the Vendor_Prefix, the date segment, and the Number_Sequence using the same source values and rules as the current generator, changing only the assembled string format to insert a single hyphen (`-`) separator between the literal `REP`, the Vendor_Prefix, the date segment, and the Number_Sequence.
3. THE Vendor_Request_Service SHALL scope the Number_Sequence to per-vendor-per-day via `vendor_request_number_seq`, SHALL assign `001` to the first Vendor_Request in a given vendor-and-date scope, and SHALL increment the Number_Sequence by exactly 1 for each subsequent Vendor_Request created within that same vendor-and-date scope.
4. WHEN two or more Vendor_Requests are created concurrently within the same sequence scope, THE Vendor_Request_Service SHALL assign a distinct Request_Number to each Vendor_Request.
5. IF a unique-constraint conflict on the request-number uniqueness constraint occurs during creation, THEN THE Vendor_Request_Service SHALL retry number generation up to a maximum of 5 total attempts, and IF all 5 attempts fail, THEN THE Vendor_Request_Service SHALL NOT persist the Vendor_Request and SHALL return an error response indicating request-number generation failure.
6. IF the Number_Sequence for a scope would exceed 999, THEN THE Vendor_Request_Service SHALL reject the creation, SHALL NOT persist any Request_Number for that request, and SHALL return an error response indicating sequence exhaustion for that scope.
7. THE Vendor_Request_Service SHALL enforce that each Request_Number is unique across all Vendor_Requests and SHALL store the Request_Number in the `vendor_requests.request_number` field.
8. WHEN a Vendor_Request is created after this format change takes effect, THE Vendor_Request_Service SHALL apply the hyphenated format only to that new Vendor_Request and SHALL NOT modify the stored Request_Number of any Vendor_Request created before the change.
9. WHEN a list or detail response is returned, THE Vendor_Request_Service SHALL return the Request_Number in the existing `request_number` field with no change to the field name or structure, and WHEN a request-number search is submitted on the Vendor_Request_List_Page, THE Vendor_Request_Service SHALL match the search input against the stored Request_Number value including its hyphen separators.

### Requirement 5: Kolom tabel Forecast Browser — hapus Amount Refund, tambah PriorityClass, Paket, Escrow

**User Story:** As an ATM operator, I want the Forecast Browser table to drop the Amount Refund column and show PriorityClass, Paket, and Escrow, so that I can review the operational priority, service package, and escrow balance while composing a request.

#### Acceptance Criteria

1. THE Forecast_Table SHALL NOT render an "Amount Refund" column and SHALL NOT display the `amount_refund` field in any column.
2. THE Forecast_Table SHALL display a "PriorityClass" column whose value for each Forecast_Row is the Priority_Class (`atms.priority_class`) of the row's ATM.
3. THE Forecast_Table SHALL display a "Paket" column whose value for each Forecast_Row is the Paket (`vendor_packages.code`) resolved for the row's ATM via the active `atm_vendor_packages → vendor_packages` chain.
4. IF more than one active package chain resolves for a Forecast_Row's ATM, THEN the Forecast_API SHALL resolve the Paket deterministically to a single active package, and IF no single active package can be determined, THEN THE Forecast_Table SHALL render a plain hyphen "-" placeholder in the "Paket" cell.
5. THE Forecast_Table SHALL display an "Escrow" column whose value for each Forecast_Row is the Escrow_Balance of the row's ATM.
6. THE Forecast_API SHALL return, for each Forecast_Row, the Priority_Class, Paket, and Escrow_Balance fields additively in the existing flat JSON shape, and SHALL NOT rename or remove any existing field.
7. WHERE a Priority_Class value is empty or null for a Forecast_Row, THE Forecast_Table SHALL render a plain hyphen "-" placeholder in the "PriorityClass" cell rather than a blank cell, and SHALL NOT render an em-dash.
8. WHERE a Paket value is empty or null for a Forecast_Row, THE Forecast_Table SHALL render a plain hyphen "-" placeholder in the "Paket" cell rather than a blank cell, and SHALL NOT render an em-dash.
9. THE Forecast_Table SHALL render the Escrow column as a right-aligned monetary value using `tabular-nums` with the currency shown as IDR, consistent with the existing money-column convention.
10. THE Vendor_Request_Service SHALL treat the Escrow_Balance as numeric / integer minor units and SHALL NOT represent it as a floating-point value.
11. WHERE an Escrow_Balance is unavailable for a Forecast_Row, THE Forecast_Table SHALL render a plain hyphen "-" placeholder in the "Escrow" cell and SHALL NOT render a zero as a substitute for an unknown value.
12. THE addition of the PriorityClass, Paket, and Escrow columns SHALL NOT change the Forecast_Row identity `(terminal_id, periode_pred, denom)` used for row selection, and SHALL NOT change which fields the create payload persists as line items.
13. THE new columns SHALL use the design-system tokens and existing Forecast_Table markup consistent with the Merah Sirih theme, preserving the existing sort, selection, pagination, loading, error, and empty-state behavior.

### Requirement 6: Tanpa regresi pada alur yang sudah ada

**User Story:** As a maintainer, I want these enhancements to preserve current contracts and behavior, so that the existing Forecast Browser, Vendor Request flow, maker-checker gates, and wire compatibility are not broken.

#### Acceptance Criteria

1. THE ATM backend endpoints SHALL keep their existing flat JSON response shapes; WHERE any new field is added it SHALL be added additively without renaming or removing any existing field, and every field present in the pre-enhancement response SHALL remain present with the same name, type, and nesting position.
2. THE existing role gates (`ATM-USER`, `ATM-SPV`, `BRANCH-ATM-USER`, `BRANCH-ATM-SPV`) on both the routes and the endpoints SHALL remain unchanged, granting and denying the same role-to-endpoint access as before the enhancement.
3. IF a checker attempts to act on a request the checker created (checker identity equals maker identity), THEN THE ATM backend SHALL reject the action, preserve the request state unchanged, and return an error response indicating the four-eyes rule violation.
4. THE existing maker-checker state machine SHALL continue to permit exactly the transitions `draft → pending_approval`, `pending_approval → approved`, `pending_approval → rejected`, revise, and the current cancel transition from `draft` and `pending_approval`, extended only by the approved-cancel transition of Requirement 3, and IF a transition not in this set is requested, THEN THE ATM backend SHALL reject it and preserve the current state.
5. THE reads for browse, list, and detail SHALL use the same read/write routing as the existing implementation, writes and read-after-write within the same flow targeting the primary and reporting-style reads targeting the replica only where the existing code already does so.
6. THE monetary amounts SHALL remain stored as numeric or integer minor units with currency code stored explicitly, and THE timestamps SHALL remain stored as timestamptz in UTC.
7. WHERE any new schema field is required, THE change SHALL be introduced only via additive, forward-only migrations under `backend/migrations/`, each proposed and confirmed before migrating, applying no update, transformation, or backfill to existing rows.
8. THE enhancement SHALL ship with automated tests that pass and that cover: the Request_Type selection and its category-driven date rules including the inconsistent-date rejection; the Reject control visibility and reject transition for Checker_Role including creator denial; the approved-cancel behavior (row preserved, is_canceled set, reason recorded, audit written, idempotent on repeat, and Checker_Role authorization); the hyphenated Request_Number generation preserving per-scope sequence, uniqueness, and exhaustion; and the Forecast_Table column changes (Amount Refund removed; PriorityClass, Paket, Escrow present with correct source mapping and placeholders).
