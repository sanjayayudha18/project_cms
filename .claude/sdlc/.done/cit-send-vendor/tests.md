# Tests: Kirim Vendor Request ke vendor + terima/tolak di VendorPortal (Phase 2.2b)

Status: **green 2026-10-09** (Stage 4). Input: `spec.md` (FR1–FR9, NFR1–NFR4, S1–S5), `plan.md` (S1–S9). Belum di-commit.

## Commands (run 2026-10-09)
| Command | Result |
|---|---|
| `psql … -f backend/migrations/029_vendor_request_send.sql` (dev) | COMMIT (S1) |
| `cd backend && sqlc generate` (v1.31.1) + fix `UserLeafe` | hanya file query terkait berubah (`models.go`, `vendor_request.sql.go`, `vault_plans.sql.go`, `notifications.sql.go`, baru `vendor_parties.sql.go`) |
| `cd backend && go build ./... && go vet -tags integration ./...` | clean |
| `cd backend && DATABASE_URL=… go test -tags integration ./...` | all packages ok; dev bersih setelah run (0 baris `vendor_request_vendor_parties`) |
| `go test -tags integration ./internal/service -run VendorOrder_AcceptAll -count=5` | 5/5 ok (accept paralel). `-race` tidak tersedia (CGO_ENABLED=0) |
| `go test ./internal/service -run "Property8\|Property10" -count=20` | ok |
| coverage `internal/notification` | 81.1% |
| coverage `internal/handler` (unit) | 66.8% — paket lama sudah < 80% sebelum fitur ini |
| `pnpm --dir frontend/VendorPortal-Vite run test` | 18 files / 113 tests (lihat catatan flaky) |
| `pnpm --dir frontend/VendorPortal-Vite run lint` / `tsc --noEmit` / `build` | 0 error di file baru (4 error lama di dsr/FileUpload) / clean / built |
| `pnpm --dir frontend/CompanyPortal-Vite run test` | 127 files / 1077 tests passed |
| `pnpm --dir frontend/CompanyPortal-Vite run lint` / `tsc --noEmit` / `build` | clean / clean / built |

## Test → requirement
| Test | Covers |
|---|---|
| `no_hard_delete_test.go` (+2 tabel) | Data model: pihak + events tidak pernah di-hard-delete |
| `TestSend_VendorBranchRecipients`, `TestIntegration_SendScopesToVendorBranch` (`internal/notification`) | FR7.1 + S4: user/PIC branch itu atau vendor-wide; branch saudara tidak ikut; PIC hanya bila email |
| `TestBuildPartyContents`, `TestSameContent`, `TestReapprovedSinceSent` (`vendor_party_test.go`) | FR1.1–FR1.3 pihak per (branch, peran), isi + total per denom, pihak lawan; perbandingan isi tahan urutan key jsonb; FR2.3 deteksi approve ulang |
| `TestIntegration_VendorParty_SendAndResync` | FR2.1 kirim otomatis (4 pihak, vault vendor lain), FR3.3 **tidak ada data internal di `content`**, FR7.1 notifikasi hanya user branch sendiri, FR8.1 audit `after.vendor_parties`; FR2.2 resync hanya pihak berubah + withdrawn, pihak ditolak dikirim ulang; FR2.3 semua pihak setelah approve ulang |
| `TestIntegration_VendorOrder_ScopeAndNoLeak` | FR5.1–FR5.4 + **NFR4 kebocoran**: vendor-wide vs branch-pinned, vendor lain / branch saudara → 404 juga untuk aksi, isi vault hanya ATM-nya, user internal / claim basi → not authorized |
| `TestIntegration_VendorOrder_AcceptAll` | FR4.1 alasan 10–500, keputusan kedua → 409; FR4.2 + NFR3 dua accept terakhir paralel → tepat satu `vendor_accepted` + audit + notifikasi pembuat; `vendor_accepted_at` |
| `TestIntegration_VendorOrder_ReplenishReject` | FR4.4 → `vendor_rejected` + notifikasi; FR4.5 pihak lain terkunci |
| `TestIntegration_VendorOrder_VaultRejectBackToAcm` | FR4.3 hanya plan area terdampak → draft (alasan + `rejected_by` vendor, `RejectedByVendor`), `approved_at` tidak berubah, notifikasi ACM; resend setelah ACM ganti vault: accepted tetap, pihak berubah pending, vault lama withdrawn + notifikasi |
| `TestIntegration_VendorOrder_ReturnEditResend` | FR6.1 Kembalikan (maker ditolak, alasan wajib, notifikasi pembuat); FR6.2 edit hapus penetapan ATM yang dihapus, approve ulang: plan draft / plan kosong cancelled; FR2.3 semua pihak tersisa pending |
| `TestIntegration_VendorOrder_CompletionGateAndCancel` | FR6.3 laporan selesai hanya dari `vendor_accepted`, tolak laporan → `vendor_accepted` + `completion_rejection_*` (bug S5 diperbaiki); FR6.4 cancel maker ditolak / checker → event `cancelled` + notifikasi vendor, tampilan vendor "Dibatalkan" tanpa aksi |
| `TestNextState`, `TestProperty8_StateMachineTransitionSet` | transisi baru (`sent_to_vendor`, `vendor_accepted`, `vendor_rejected`, `vendor_return`), legacy `ready`/`approved` |
| `TestIntegration_VaultPlan_FullFlow` (diubah) | S1: vault approve → `sent_to_vendor` |
| `vendor_order_handler_test.go` | API: RBAC route (internal → 403 di route vendor, VENDOR-USER → 403 di route internal, maker → 403 return), claim vendor diteruskan, 404/409/422, parsing filter |
| `OrdersPages.test.tsx` (VendorPortal, 8) | FR9.1 daftar + filter server + paging + kosong; FR9.2 detail vault (total, pihak lawan, tanpa tiket), terima, tolak alasan ≥ 10, 409, "Sedang direvisi CIMB" tanpa aksi, 404 |
| `integration.test.tsx` (VendorPortal, diubah) | halaman Orders memakai API; request tanpa parameter vendor |
| `VendorRequestDetail.test.tsx` (+5) | FR6.6 badge status baru, panel Status Vendor + riwayat; FR6.1 Kembalikan (pembuat tidak bisa, checker + alasan); FR6.3 Laporkan Selesai hanya `vendor_accepted`; FR6.4 cancel |
| `VaultPlan.test.tsx` (+1) | FR4.3 label "Ditolak vendor (branch vault)" |

## Catatan
- **Flaky VendorPortal (lama, di luar 2.2b):** `routeGuard.property.test.tsx` — (1) timeout 5 s saat suite penuh lebih ramai → diperbaiki (timeout eksplisit 30 s + `findByTestId`); (2) counterexample fast-check `redirect=0` → `SearchParamError` (`z.string()` di `routes/login.tsx`) — bug lama, task terpisah ("Fix VendorPortal login crash on numeric redirect"). Sampai diperbaiki, suite VendorPortal sesekali gagal 1 test ini.
- **Manual browser check: outstanding (user, Golden Rule #10).** Alur yang perlu dicoba: ATM-SPV approve penetapan vault → request "Terkirim ke Vendor"; login VendorPortal sebagai user branch replenish dan user branch vault (vendor lain) → Orders/detail, Terima/Tolak; tolak vault → layar Penetapan Vault menampilkan "Ditolak vendor"; tolak replenish → "Kembalikan ke Pembuat"; semua terima → Laporkan Selesai; notifikasi in-app kedua portal. Dev belum punya Area ACM/mapping DSR + user vendor per branch yang siap — perlu seed dulu.
- Build Docker belum dijalankan.
