# Plan: Kirim Vendor Request ke vendor + terima/tolak di VendorPortal (Phase 2.2b)

Status: **accepted — 2026-10-09** (user: "diterima, mulai S1"). Reads: `spec.md` (accepted 2026-10-09, S1–S5).
Stage: 3 Build. Kerja dipecah menjadi **9 slice**; tiap slice = satu diff kecil, build + test hijau, berhenti & minta OK user.

## Titik sentuh yang sudah dicek di kode
- State machine: `backend/internal/service/vendor_request.go:487` (`transitions`), `checkActor` :525 (cancel per status :544).
- Kirim otomatis: `VaultPlanService.ReviewRequest` `vault_plan_actions.go:282` (tx ATM-SPV vault-approve/reject).
- Plan area: `CreateMissingVaultPlans` (`vault_plan_create.go:19`), `ResetVaultPlansToDraft`, `CancelVaultPlansForRequest`
  (`queries/vault_plans.sql`). Approve `pending_approval → vault_assignment` membuat plan yang belum ada saja.
- Laporan selesai: `completionTx` `vendor_request_completion.go:89`; reject kembali ke `ready` bila `VaultFlow` (:112).
- Edit: `UpdateItems` `vendor_request_actions.go:732` hanya di `draft` oleh pembuat → tidak berubah (FR6.5).
- Cancel: `vendor_request_actions.go:863` (+ `CancelVaultPlansForRequest`).
- Notifikasi: `internal/notification/notification.go:50` `Recipients{UserIDs, Roles, VendorIDs}`;
  `queries/notifications.sql` (`ListActiveUserRecipientsByVendors`, `ListVendorNotificationPicEmails`).
- Auth vendor: claim `vendor_id` (`pkg/auth/token_service.go:26`), **tanpa** `vendor_branch_id` → dibaca dari `users` (S3).
  Role vendor = `VENDOR-USER` (seed 002).
- Data FR3: `vendor_request_items` (`terminal_id`, `denom`, `amount_replenish` bigint, `lokasi_atm`), tiket aktif
  (`ListActiveVendorRequestTickets`), `vendor_request_vault_assignments` (`replenish_branch_id`, `vault_branch_id`),
  `vendor_branches.location_id → locations` (alamat).
- VendorPortal: `src/routes/orders.tsx` + `features/orders/useOrders.ts` (mock `@/data/orders.json`), `orders.$id.evidence.tsx`
  (mock evidence, tidak disentuh), pola API `features/notifications/api.ts` + `lib/api/client.ts`.
- CompanyPortal: `features/vendor-request/` (`types.ts`, `StatusBadge.tsx`, `VendorRequestDetail.tsx`, `api.ts`, `hooks.ts`),
  `features/cit` (layar Penetapan Vault 2.2a).

## Slices (urutan kerja)

### S1 — Migrasi 029 + sqlc — ✅ DONE 2026-10-09
> `029_vendor_request_send.sql` applied to dev (localhost). `queries/vendor_parties.sql` (isi pihak, tulis pihak/event,
> internal list, vendor-scoped list/count/get, `GetUserVendorScope`); `UpdateVendorRequestStatus` mengisi `vendor_sent`,
> `sent_at`, `vendor_accepted_at`. sqlc v1.31.1 (diff: `models.go`, `vendor_request.sql.go`, baru `vendor_parties.sql.go`;
> tidak ada `UserLeafe`). `no_hard_delete_test` + 2 tabel. `go build` OK; `go test -tags integration ./...` hijau semua;
> query baca dieksekusi di dev OK (dev belum punya penetapan vault → isi pihak diuji di S3). Uncommitted.
- `backend/migrations/029_vendor_request_send.sql` (satu tx): `vendor_requests_status_chk` + `sent_to_vendor`,
  `vendor_accepted`, `vendor_rejected`; kolom `vendor_sent`, `sent_at`, `vendor_accepted_at`; tabel
  `vendor_request_vendor_parties` + `vendor_request_vendor_party_events` (spec Data model).
- `queries/vendor_parties.sql` (baru): upsert/list/lock pihak, insert event, build isi per pihak (set-based, satu query per
  peran), list vendor-scoped (replica) + count, get by id scoped, list per request (internal).
- Terapkan ke dev (`localhost:5432`), `sqlc generate` v1.31.1 + fix `UserLeafe → UserLeave`; `no_hard_delete_test` + tabel baru.
- **Proof:** migrasi bersih; `go build ./...`; `go test ./internal/repository/...`.

### S2 — Notifikasi per branch vendor — ✅ DONE 2026-10-09
> `queries/notifications.sql` + `ListActiveUserRecipientsByVendorBranches`, `ListVendorBranchNotificationPicEmails` (branch
> itu atau vendor-wide); `Recipients.VendorBranchIDs` di `resolveUsers` + PIC email di `Send` (hanya bila `Email`). Tidak
> difilter role: `users.vendor_id` hanya terisi untuk user vendor (sama dengan `VendorIDs`). Tests: unit
> `TestSend_VendorBranchRecipients` (dedupe, tanpa query PIC bila in-app saja), integration
> `TestIntegration_SendScopesToVendorBranch` (user/PIC branch saudara tidak ikut). `go test -tags integration ./...` hijau;
> coverage `internal/notification` 81.1%. Uncommitted.
- `Recipients.VendorBranchIDs []int64`; query `ListActiveUserRecipientsByVendorBranches` (user `VENDOR-USER` aktif dengan
  `vendor_branch_id` = branch **atau** NULL pada vendor branch itu) + `ListVendorBranchNotificationPicEmails` (S4).
- **Proof:** unit test `resolveUsers` + integration test query (user branch lain tidak ikut).

### S3 — Kirim otomatis + sinkronisasi pihak (FR1, FR2, FR8) — ✅ DONE 2026-10-09
> `service/vendor_party.go`: `buildPartyContents` (isi per pihak, kolom whitelist), `syncVendorParties` (insert `sent` /
> resend bila isi berubah, ditolak, ditarik, atau `forceAll` / withdraw yang hilang; event per perubahan), `reapprovedSinceSent`
> (forceAll = `approved_at` > semua `sent_at` → hanya lewat edit; **tanpa** kolom/query tambahan, `CountVendorPartyEvents`
> dihapus), `notifyParties` (FR7.1, `VendorBranchIDs`, link `/orders/{partyID}`). `ReviewRequest` approve → `sent_to_vendor`
> + sync + audit `after.vendor_parties` + notifikasi, satu tx; `SetVendorRequestVaultReview` mengisi `vendor_sent`/`sent_at`.
> Ditarik maju dari S5 agar state machine konsisten: transisi `sent_to_vendor`/`vendor_accepted`/`vendor_rejected` (cancel;
> laporan selesai hanya dari `vendor_accepted`), `checkActor` cancel = checker, reject laporan → `vendor_accepted` bila
> `vendor_sent`, filter status list. Sisa S5: notifikasi cancel ke vendor, vendor-return, approve ulang, cleanup penetapan saat edit.
> Tests: unit `TestBuildPartyContents`, `TestSameContent`, `TestReapprovedSinceSent`, `TestNextState` (+7 baris); integration
> `TestIntegration_VendorParty_SendAndResync` (4 pihak, isi tanpa data internal, notifikasi hanya user branch sendiri, audit;
> 3 skenario resync dalam tx rollback). FullFlow expect `sent_to_vendor`. Fixture: `cleanupVendorParties` (kegagalan pertama
> sempat meninggalkan request 2500 + vendor 2628/2629 di dev → dibersihkan satu kali, scoped). `go test -tags integration ./...`
> hijau. Uncommitted.
- `service/vendor_party.go`: `syncVendorParties(ctx, q, actor, req, forceAll bool)` — bangun isi per pihak dari penetapan vault,
  bandingkan dengan pihak yang ada (FR2.2/FR2.3), tulis status + event + audit, kembalikan pihak yang perlu dinotifikasi.
- `transitions`: `vault_review` `actionVaultApprove → sent_to_vendor` (S1); `ReviewRequest` approve memanggil sync + set
  `vendor_sent=true`, `sent_at`; notifikasi vendor (FR7.1). `forceAll` = request pernah lewat `rejected → draft` setelah terkirim
  (dibaca dari event pihak / `vendor_sent`).
- **Proof:** integration test: kirim pertama (2 replenish + 1 vault vendor lain → 3 pihak, isi benar, data internal tidak ada di
  `content`); kirim ulang setelah vault diganti (hanya pihak berubah → `pending`, pihak lama → `withdrawn`, accepted tak berubah);
  kirim ulang setelah edit (semua `pending`).

### S4 — Keputusan vendor (FR4, FR5) — ✅ DONE 2026-10-09
> `service/vendor_order.go` `VendorOrderService` (List di replica; scope/Get/aksi di primary): scope = role `VENDOR-USER` +
> `users.vendor_id` harus sama dengan claim (beda → `ErrNotAuthorized`) + `vendor_branch_id`; di luar cakupan →
> `ErrVendorOrderNotFound`. `decide`: kunci request lalu pihak; hanya `sent_to_vendor` + `pending` (selain itu
> `ErrInvalidTransition`); event + audit pihak; accept terakhir → `vendor_accepted` (+ notifikasi pembuat + ATM-SPV); tolak
> vault → `vault_assignment` via `SetVendorRequestStatusOnly` (**tidak** mengubah `approved_at`, kalau berubah kirim ulang
> akan memaksa semua pihak) + query baru `ReturnVaultPlansForVaultBranch` (plan terdampak → draft, `rejected_by` = user
> vendor untuk label UI) + notifikasi ACM-USER/ACM-SPV area; tolak replenish → `vendor_rejected` (+ ATM-SPV,
> BRANCH-ATM-SPV, pembuat). Alasan 10–500 (trim). Tests `vendor_order_integration_test.go`: cakupan + **kebocoran** (vendor
> lain / branch saudara → 404 juga untuk aksi; isi vault hanya ATM-nya), claim basi / user internal → not authorized,
> accept semua + **2 accept terakhir paralel** → 1 transisi (`-count=5` stabil; `-race` tidak tersedia: CGO_ENABLED=0),
> tolak replenish (pihak lain terkunci), tolak vault → ACM → kirim ulang hanya pihak berubah. Cleanup fixture + notifikasi
> request `vendor_order.*`. `go test -tags integration ./...` hijau, dev bersih. Uncommitted.
- `service/vendor_order.go`: `List/Get` (cakupan dari claim + `users.vendor_branch_id`), `Accept`, `Reject`.
  Lock request → pihak; hanya `sent_to_vendor` + `pending`; semua accepted → `vendor_accepted`; reject vault → `vault_assignment`
  + plan area terdampak `draft` dengan `rejection_reason`; reject replenish → `vendor_rejected` (didahulukan). Audit + event +
  notifikasi internal (FR7.2).
- **Proof:** table-driven unit + integration: cakupan (user branch vs vendor-wide, vendor lain → 404), **test kebocoran** (vault
  vendor B tidak melihat ATM lain/isi replenish vendor A), accept terakhir paralel → satu transisi, reject kedua → 409,
  decision setelah request keluar `sent_to_vendor` → 409.

### S5 — Tindak lanjut internal (FR6) — ✅ DONE 2026-10-09
> `VendorReturn` (`actionVendorReturn`: `vendor_rejected → rejected`, checker ≠ pembuat, alasan 1–500 via `checkReason`,
> `rejected_*` di-stamp, audit `vendor_return`, notifikasi pembuat `vendor_request.vendor_returned`). Cancel dari status
> baru menulis event `cancelled` per pihak non-withdrawn + notifikasi vendor (`cancelVendorParties`), status pihak tetap.
> Approve ulang: `resetVaultPlansForReapproval` membuka **semua** plan (termasuk yang cancelled; query baru
> `ListAllVaultPlansForRequest`, karena unique (request, area) mencegah dibuat ulang) ke draft, lalu plan tanpa ATM →
> `cancelled` (gate `vault_review` berbasis cakupan ATM, jadi tidak macet; hanya mencegah draft kosong di layar ACM);
> audit `vault_plan_reset` + notifikasi ACM-USER. `UpdateItems` menghapus penetapan ATM yang dihapus (query
> `DeleteVaultAssignmentsOfRemovedAtms`, dicatat di audit `update_items`). **Bug ditemukan & diperbaiki**:
> `UpdateVendorRequestCompletion` hanya men-stamp `completion_rejected_*` untuk `approved`/`ready` → + `vendor_accepted`.
> Grep status lain: filter kapasitas (`NOT IN ('cancelled','rejected')`) sudah mencakup status baru — sesuai keputusan 2.2a.
> Tests: `TestIntegration_VendorOrder_ReturnEditResend` (maker/alasan kosong ditolak, return → revise → edit hapus terminal2
> → approve ulang: plan1 draft, plan2 cancelled → kirim ulang: semua pihak tersisa pending, yang hilang withdrawn),
> `TestIntegration_VendorOrder_CompletionGateAndCancel` (laporan sebelum diterima → 409, setelah `vendor_accepted` boleh,
> tolak laporan → `vendor_accepted` + alasan ter-stamp, cancel maker ditolak / checker → 4 event + notifikasi vendor,
> tampilan vendor: dibatalkan, tanpa aksi). `go test -tags integration ./...` hijau, dev bersih. Uncommitted.
- `actionVendorReturn`: `vendor_rejected → rejected` (checker ≠ pembuat, alasan 1–500, audit, notifikasi pembuat).
- `checkActor` cancel: + `sent_to_vendor`, `vendor_accepted`, `vendor_rejected` (checker); cancel menotifikasi pihak aktif (FR6.4).
- Approve ulang `pending_approval → vault_assignment`: `ResetVaultPlansToDraft` + `CreateMissingVaultPlans`; `UpdateItems` menghapus
  penetapan vault ATM yang sudah tidak ada (diaudit).
- `completionTx`: gate `vendor_accepted` (transitions) + reject laporan → `vendor_accepted` bila `vendor_sent`.
- **Proof:** integration: return → revise → edit → submit → approve → plan draft lagi; cancel dari 3 status baru; laporan selesai
  ditolak dari `sent_to_vendor`, boleh dari `vendor_accepted` / legacy `ready` / `approved`.

### S6 — Handler + wiring — ✅ DONE 2026-10-09
> `handler/vendor_order_handler.go` (`/api/v1/vendor/replenish-orders`: `GET /`, `GET /{id}`, `POST /{id}/accept`,
> `POST /{id}/reject {reason}`; `RequireRoles("VENDOR-USER")` di router + claim `vendor_id` diteruskan ke service; filter
> `party_status` whitelist, `from`/`to` `YYYY-MM-DD` → 422; 404 di luar cakupan, 409 transisi, 422 alasan).
> `handler/vendor_request_vendor_handler.go`: `WithVendorSide` me-mount `GET /{id}/vendor-parties` (viewer roles,
> `VendorOrderService.RequestParties` di replica) dan `POST /{id}/vendor-return {rejection_reason}` (checker roles,
> `VendorRequestService.VendorReturn`) — `VendorRequestServicer` tidak diubah (interface kecil terpisah, mock lama utuh).
> `cmd/api/main.go`: `vendorOrderService` (primary + replica, notifier) + dua mount. Tests `vendor_order_handler_test.go`:
> RBAC route (internal → 403 di route vendor; VENDOR-USER → 403 di route internal; maker → 403 return), claim diteruskan,
> 404/409/422, parsing filter. `go test -tags integration ./...` hijau; coverage `internal/handler` 66.8% (paket lama, di
> bawah 80% sebelum fitur ini). Uncommitted.
- `handler/vendor_order_handler.go`: `/api/v1/vendor/replenish-orders` (`RequireRoles("VENDOR-USER")`), flat JSON, money + `currency`.
- `vendor_request_handler.go`: `GET /{id}/vendor-parties`, `POST /{id}/vendor-return`; filter status baru di list.
- `cmd/api/main.go`: mount + notifier.
- **Proof:** handler tests (RBAC: internal user → 403 di route vendor, VENDOR-USER → 403 di route internal; 404 lintas vendor;
  validasi alasan); `go test -tags integration ./...`.

### S7 — VendorPortal Orders (FR9) — ✅ DONE 2026-10-09
> `features/orders/api.ts` (tipe + fetch/accept/reject), `useOrders.ts` (`useOrders(filter)` server paging +
> `keepPreviousData`, `useOrder(id)`, `useDecideOrder`), `labels.ts` (peran, status teks + varian — "Dibatalkan" mengalahkan
> status pihak, "Sedang direvisi CIMB" bila pending tapi request bukan `sent_to_vendor`), `OrdersPage.tsx` (tab status,
> rentang tanggal, prev/next; kolom FR9.1; link ke detail), `OrderDetailPage.tsx` + `routes/orders.$id.tsx` (total per denom,
> tabel ATM dengan kolom denom `tabular-nums` rata kanan, branch lawan + alamat vault, tiket hanya untuk replenish; Terima /
> Tolak + form alasan 10–500, pesan 409). Heading/nav tetap "CIT Orders". Dihapus: `OrderSummaryBar.tsx` (hitungan per status
> butuh query tambahan — tidak di FR9), tipe `CITOrder`. `data/orders.json` **tetap** (fixture `dataFilters.property.test.ts`,
> tidak lagi di-import aplikasi). Tests: `features/orders/__tests__/OrdersPages.test.tsx` (8: daftar, filter server + reset
> page, kosong, detail vault + terima, alasan min 10 + tolak, 409, direvisi tanpa aksi, 404); `integration.test.tsx` memakai
> respon API mock (cek: tidak ada `vendor_id` di query). **Flaky test lama**: `routeGuard.property.test.tsx` — (1) 40 run login
> interaktif per `it` melewati timeout 5 s saat suite penuh lebih ramai → timeout eksplisit 30 s + `findByTestId` setelah
> `settle()`; (2) counterexample fast-check `redirect=0` → `SearchParamError` karena `z.string()` di `routes/login.tsx` —
> bug lama di luar 2.2b, dijadikan task terpisah (chip "Fix VendorPortal login crash on numeric redirect"). `tsc`, `build`
> OK; `pnpm test` 113/113 (dengan sisa flaky (2) sesekali); lint: 4 error lama di dsr/FileUpload, 0 di file baru. Cek manual
> browser = outstanding (user). Uncommitted.
- `features/orders/api.ts` + `useOrders.ts` (data nyata, paging server), `OrdersPage.tsx` (kolom FR9.1, filter), detail baru
  `routes/orders.$id.tsx` + `features/orders/OrderDetailPage.tsx` (isi FR3, Terima / Tolak + dialog alasan); hapus
  `data/orders.json` + tipe `CITOrder` bila tak dipakai (cek `OrderSummaryBar`, dashboard, evidence).
- **Proof:** component tests (daftar, detail replenish vs vault, tombol hanya saat aktif, dialog alasan min 10); `pnpm test`,
  `lint`, `build`.

### S8 — CompanyPortal (FR6.6, FR4.3) — ✅ DONE 2026-10-09
> Status baru di `types.ts` (+ `VENDOR_REQUEST_STATUSES` → filter list), `StatusBadge` ("Terkirim ke Vendor" info, "Diterima
> Vendor" success, "Ditolak Vendor" danger — teks + ikon), label list. `vendorSide.ts` (API + hook `useVendorParties`,
> `useVendorReturn`; file terpisah agar `api.ts`/`hooks.ts` tidak membesar). `VendorPartiesPanel.tsx` ("Status Vendor": peran,
> branch, vendor, status, diputuskan oleh/pada, alasan + riwayat `<details>`; tidak tampil bila belum pernah dikirim).
> `VendorRequestDetail.tsx`: laporan selesai dari `vendor_accepted` (+ legacy `ready`/`approved`), cancel checker untuk 3 status
> baru, tombol **Kembalikan ke Pembuat** (`vendor_rejected`, checker ≠ pembuat, `ReasonModal` 1–500), panel vault tampil di
> status baru. Layar Penetapan Vault: banner "Ditolak vendor (branch vault) — pilih vault lain" — backend `GetVaultPlan` +
> kolom `rejected_by_vendor` (`EXISTS` user penolak ber-`vendor_id`), JSON `rejected_by_vendor`. **Temuan**: property test
> `TestProperty8_StateMachineTransitionSet` (rapid, tabel spec sendiri) masih mengharapkan `vault_review → ready` — lolos
> sebelumnya hanya karena undian rapid tidak mengenai pasangan itu; tabel diperbarui (+3 status, `vendor_return`), `-count=20`
> hijau. Tests: `VendorRequestDetail.test.tsx` +5 (badge, panel + riwayat, Kembalikan: pembuat tidak bisa / checker dengan
> alasan, laporan selesai hanya `vendor_accepted`, cancel), `VaultPlan.test.tsx` +1 (label vendor); integration Go assert
> `RejectedByVendor`. CompanyPortal `pnpm test` 1077/1077, `lint` bersih, `tsc`, `build` OK; `go test -tags integration ./...`
> hijau. Cek manual browser = outstanding (user). Uncommitted.
- `types.ts`/`StatusBadge.tsx` status baru; panel **"Status Vendor"** + riwayat di `VendorRequestDetail.tsx`; tombol
  **Kembalikan ke pembuat** di `vendor_rejected`; filter list; layar Penetapan Vault menampilkan "Ditolak vendor" + alasan.
- **Proof:** component tests; `pnpm test`, `lint`, `build`.

### S9 — Dokumen — ✅ DONE 2026-10-09
> CLAUDE.md Sec 3 (bullet "CIT — kirim ke vendor") + Sec 12 (keputusan + deviasi state machine vendor) + baseline migrasi
> `029_`; `docs/data-map.md` (§ Send to vendor (migration 029)); `docs/decisions.md` (Phase 2.2b); `tests.md` (commands +
> trace FR → test, manual browser = outstanding); `development-progress.md`, `sdlc/README.md` (stage 4),
> `.kiro/steering/development-plan.md` (2.2a committed, 2.2b built); `graphify update .` dijalankan. Stage 4 selesai; berikut:
> `review.md` (stage 5).
- CLAUDE.md Sec 3 (tabel/status baru, mig 029) + Sec 12 (keputusan + deviasi state machine vendor); `docs/data-map.md`;
  `development-progress.md`; `sdlc/README.md`; `tests.md` (trace FR → test; cek manual browser = outstanding, user).
- `graphify update .`

## Risiko
- **Kebocoran antar vendor** (Sec 4 #7): filter cakupan di SQL, ID URL = `partyID`, `content` dibangun dari kolom yang
  di-whitelist (tidak `SELECT *` dari penetapan). Test kebocoran wajib (S4).
- **Status baru di alur live**: semua `switch status` di frontend/backend harus kenal 3 status baru — grep `"ready"` dan
  `vault_review` saat S5/S8.
- **Accept terakhir paralel**: kunci request dulu, lalu pihak (urutan tetap) → tidak deadlock, transisi sekali.
- **Approve ulang setelah edit**: plan lama + penetapan ATM yang dihapus — ditangani S5, ditest end-to-end.

## Di luar plan
Cash pickup FSD, Schedule, SLA, pindah buku, 2.2c, perubahan JWT.
