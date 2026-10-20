# Tests: Modul notifikasi (in-app + email SMTP) — Phase 0.3

Status: build + automated tests **green** (2026-10-07). Stage: 4 Test.
Input: `plan.md` (accepted 2026-10-07). Stage berikut: review (`review.md`) → code owner merge.
**Manual browser check: OUTSTANDING — dilakukan user** (Golden Rule #10). Tidak ditandai selesai di sini.
**Kirim email nyata ke relay SMTP perusahaan: OUTSTANDING** — menunggu host/port/TLS/kredensial relay; dev memakai `SMTP_HOST` kosong (outbox `skipped`).

## Perintah & hasil (2026-10-07)
| Perintah | Hasil |
|---|---|
| `psql … -f backend/migrations/025_notifications.sql` (dev, localhost) | OK, 2 tabel + 5 index |
| `cd backend && sqlc generate` (v1.31.1) + fix `UserLeafe`→`UserLeave` | OK; diff `internal/db`: `models.go`, baru `notifications.sql.go` (`role_mgmt.sql.go` hand-written tidak tersentuh) |
| `cd backend && go build ./... && go vet` (cmd, notification, handler, service) | OK |
| `cd backend && go test ./...` | OK (semua paket) |
| `cd backend && DATABASE_URL=<dev, localhost> go test -tags integration -count=1 ./internal/...` | OK (semua paket, termasuk 4 `TestIntegration_*` notification + 2 over-quota) |
| `gofmt -l` pada file Go yang dibuat/diubah | bersih (`cmd/hashpw` memang sudah tidak rapi sebelumnya) |
| `go test -race` | tidak bisa di mesin ini (Windows tanpa cgo) |
| `pnpm --dir frontend/CompanyPortal-Vite run test` | 123 file, 1050 test lulus (1051 setelah review R1) |
| `pnpm --dir frontend/CompanyPortal-Vite run lint` | bersih |
| `pnpm --dir frontend/CompanyPortal-Vite run build` | OK (warning chunk >500 kB sudah ada) |
| `pnpm --dir frontend/VendorPortal-Vite run test` | 104 test; 103 lulus + `routeGuard.property.test.tsx` flaky **sudah ada sebelumnya** (gagal 2 dari 3 run pada kode tanpa perubahan ini) |
| `pnpm --dir frontend/VendorPortal-Vite run lint` | 21 diagnostik **sudah ada** (jumlah sama sebelum/sesudah); file yang diubah fitur ini bersih kecuali `AppShell.tsx:136` (lama) |
| `pnpm --dir frontend/VendorPortal-Vite run build` | OK |
| Smoke `cmd/api` lokal | tidak selesai: start berhenti di koneksi Redis (Redis tidak jalan di mesin dev ini) — tidak terkait fitur |

## Traceability test → spec
| Spec | Test |
|---|---|
| FR1.1 Send di tx pemanggil; rollback → 0 baris | `notification/integration_test.go` `TestIntegration_SendRollsBackWithCallerTx`; `service/…_OverQuotaNotifyFailureRollsBackApprove` |
| FR1.2/FR1.5 validasi Type/Title/Body | `notification_test.go` `TestSend_ValidatesMessage` |
| Definisi penerima: user/role/vendor, dedupe user + email case-insensitive, PIC email-only, alamat invalid dilewati | `TestSend_DedupesUsersAndEmails`, `TestSend_SkipsUnusedRecipientQueries`; DB nyata `TestIntegration_SendExpandsVendorUsersAndPICs` |
| FR1.3 SMTP kosong → `skipped`; Email=false → tanpa outbox | `TestSend_SMTPDisabledWritesSkipped`, `TestSend_EmailFalseWritesNoOutbox` |
| FR1.4 penerima kosong bukan error | `TestSend_NoRecipientsIsNotAnError` |
| FR2.1–2.3 sender: sent / retry +1/+5/+15 m / failed setelah percobaan ke-4; tanpa mailer tidak menyentuh outbox | `worker_test.go` `TestWorker_SendBatch`, `TestWorker_TickWithoutMailerSendsNothing`, `TestWorker_TruncatesLastError` |
| FR2.2 / plan D2–D3 SMTP stdlib: subject UTF-8 Q-encoded, body plain text QP, injeksi header ditolak, timeout tidak menggantung | `smtp_test.go` (server SMTP palsu) |
| FR3 retensi 90 hari, sekali per 24 jam, pending tidak dihapus | `TestWorker_TickPurgesOncePerDay`; DB nyata `TestIntegration_PurgeDeletesOnlyOldRows` |
| FR4 API: default/parse query, 400, 401, 404 milik orang lain, 204, read-all, 500 | `handler/notification_handler_test.go` (8 test); DB nyata `TestIntegration_RepositoryScopesToOwner` |
| AC6 replica untuk list/unread-count, primary untuk mark read | `notification/repository_topology_test.go` |
| FR5.1 lonceng CompanyPortal: angka 1–99/99+ sebagai teks + nama aksesibel, panel, tandai semua tanpa refetch, klik → mark + navigasi, Escape | `features/notifications/NotificationBell.test.tsx` (5) |
| FR5.2 VendorPortal memakai API (bukan mock), mark read/all memperbarui cache tanpa refetch | VendorPortal `features/notifications/__tests__/useNotifications.test.tsx` (3) |
| FR6 / S1–S2 over-quota → maker + semua ATM-SPV aktif, in-app + 1 outbox per alamat; tanpa over-quota → 0; isi pesan + batas 1000 karakter | `service/vendor_request_completion_test.go` `TestOverQuotaNotification`; integration `TestIntegration_OverQuotaNotifiesMakerAndATMSPV` |

## Outstanding (user)
- Manual browser: lonceng CompanyPortal (angka, panel, tandai dibaca, link ke detail vendor request) setelah approve laporan selesai yang over-quota; badge + halaman Notifikasi VendorPortal (kosong untuk sekarang — belum ada pemakai yang mengirim ke vendor).
- Email nyata lewat relay perusahaan setelah `SMTP_*` tersedia.
