# Plan: Modul notifikasi (in-app + email SMTP) — Phase 0.3

Status: **accepted 2026-10-07** (user: "plan diterima, lanjut implementasi"; D1–D4 disetujui). Stage: 3 Build.
Input: `spec.md` (accepted 2026-10-07, user: "S1 maker + ATM-SPV saja, S2 in-app + email, lanjut plan").

## Keputusan yang perlu disetujui (code ≠ spec / detail yang spec tidak atur)
**D1 — Notifier masuk ke `VendorRequestService` lewat setter.** `NewVendorRequestService(pool)` dipanggil di 20 tempat
(kebanyakan tes). **Usulan:** `func (s *VendorRequestService) WithNotifier(n Notifier) *VendorRequestService` (pola sama
dengan `approvalHandler.WithMasterDataDetail`). Notifier `nil` → tidak mengirim apa pun (tes lama tidak berubah).
`Notifier` = interface 1 method (`Send(ctx, q, msg, to) error`) yang didefinisikan di `service` (tempat dipakai) supaya
`service` tidak import `internal/notification` secara langsung di tes.

**D2 — Timeout SMTP.** `net/smtp.SendMail` tidak punya timeout dial/IO; SMTP yang menggantung akan membekukan sender.
**Usulan:** `net.DialTimeout` (10 s) + `conn.SetDeadline` (30 s) + `smtp.NewClient` → `StartTLS` bila `Extension("STARTTLS")`
→ `Auth` bila user terisi → `Mail/Rcpt/Data`. Masih stdlib, ±30 baris.

**D3 — Subject non-ASCII.** Header email di-encode `mime.QEncoding.Encode("utf-8", subject)` (stdlib) +
`Content-Type: text/plain; charset=UTF-8`.

**D4 — Retensi di goroutine yang sama.** Satu goroutine `notification.Worker` selalu jalan; tiap tick: (a) kirim batch
email bila SMTP aktif, (b) retensi bila retensi terakhir > 24 jam lalu (disimpan di memori — restart cukup menjalankannya
lagi; DELETE idempotent). Tanpa tabel/state tambahan.

## Urutan kerja

### M1 — Migrasi `backend/migrations/025_notifications.sql`
Persis DDL di spec (2 tabel + 5 index), `BEGIN/COMMIT`, komentar tabel. Diterapkan manual ke dev
(`psql ... localhost:5432 ...`).

### B1 — SQL `backend/queries/notifications.sql` → `sqlc generate` (v1.31.1, perbaiki `UserLeafe` → `UserLeave`)
- Penerima: `ListActiveUserRecipientsByIDs(ids)`, `ListActiveUserRecipientsByRoles(role_names)` (join `roles`),
  `ListActiveUserRecipientsByVendors(vendor_ids)` → `(id, email)`; `ListVendorNotificationPicEmails(vendor_ids)` →
  `email` dari `vendor_pics` aktif, `is_notification_recipient`, email tidak kosong.
- Tulis: `InsertNotification` (returning id), `InsertNotificationEmail`.
- Baca (replica): `ListNotificationsForUser(user, unread_only, limit, offset)`, `CountNotificationsForUser(user, unread_only)`,
  `CountUnreadNotifications(user)`.
- Ubah (primary): `MarkNotificationRead(id, user)` = `UPDATE ... SET is_read = true, read_at = COALESCE(read_at, now())
  WHERE id = $1 AND recipient_user_id = $2 RETURNING id` (0 baris → 404; sudah dibaca → tetap 1 baris → 204 idempotent),
  `MarkAllNotificationsRead(user)` → count.
- Sender: `ClaimPendingNotificationEmails(limit)` (`FOR UPDATE SKIP LOCKED`), `MarkNotificationEmailSent(id)`,
  `MarkNotificationEmailRetry(id, err, next_at)`, `MarkNotificationEmailFailed(id, err)`.
- Retensi: `DeleteOldNotificationEmails(before)`, `DeleteOldNotifications(before)`.

### B2 — Paket `backend/internal/notification/` (baru)
- `notification.go`: `Message`, `Recipients`, `Service{smtpEnabled bool}`, `Send(ctx, q, msg, to)` — validasi (FR1.5),
  expand penerima (spec "Definisi"), dedupe user + email (lowercase, trim), insert notifikasi + outbox (`pending` atau
  `skipped`). Tidak log alamat/body.
- `repository.go`: `Repository{db, dbRead *db.Queries}` (pola `rolemgmt.Repository`): List/Count/Unread → `dbRead`;
  MarkRead/MarkAll → `db`.
- `worker.go`: `Worker{pool, mailer, interval, now}`; `Run(ctx)` (ticker), `sendBatch(ctx)` (tx: claim → kirim → mark; backoff
  1/5/15 m, failed setelah 3), `purge(ctx)`. `Mailer` interface (`Send(to, subject, body) error`) supaya tes pakai fake.
- `smtp.go`: `SMTPMailer` (D2, D3).

### B3 — Handler `backend/internal/handler/notification_handler.go` (baru)
`GET /`, `GET /unread-count`, `POST /{id}/read`, `POST /read-all` (FR4), aktor via `actorFromRequest`
(`vendor_request_handler.go:509`), JSON flat; page_size default 20 maks 100; `id` bukan angka → 400.

### B4 — Pemakai pertama `internal/service/vendor_request_completion.go`
D1 setter; di closure `ApproveCompletion`, setelah loop kunjungan dan bila `len(overQuota) > 0` dan notifier tidak nil →
`Send` dengan `q` tx yang sama (FR6.1: maker `req.CompletionSubmittedBy` + role `ATM-SPV`, `Email: true`). Error → return
(rollback).

### B5 — Wiring `backend/cmd/api/main.go` + `backend/.env.example`
Baca `SMTP_*`, `NOTIFICATION_EMAIL_POLL_INTERVAL` (`getenvDefault`, seperti `DSR_*`); `SMTP_HOST` terisi tapi `SMTP_FROM`
kosong → `slog.Error` + `os.Exit(1)`. `notification.NewService(smtpEnabled)`,
`vendorRequestService.WithNotifier(...)`, mount `/api/v1/notifications` (RequireAuth saja), `go worker.Run(ctx)` dengan
context yang dibatalkan saat shutdown. `.env.example`: buka blok SMTP + var interval.

### F1 — CompanyPortal
- `features/notifications/` (baru): `api.ts` (lewat `lib/api` client), `hooks.ts` (`useUnreadCount` refetch 60 s,
  `useNotificationList`, `useMarkRead`, `useMarkAllRead` — optimistic, tanpa refetch langsung, FR4.5), `NotificationBell.tsx`
  (angka 1–99/"99+", `aria-label`, panel 10 terbaru, tandai semua dibaca, klik item → mark + navigate `link`).
- `components/layout/Header.tsx`: ganti tombol + titik statis dengan `<NotificationBell />`.

### F2 — VendorPortal
- `features/notifications/useNotifications.ts`: query/mutation ke API via `lib/api/client.ts`; unread dari `/unread-count`
  (refetch 60 s). `lib/types.ts` `Notification` disesuaikan (FR5.2). `NotificationsPage.tsx` disesuaikan ke field baru.
  Hapus `data/notifications.json` bila tidak ada pemakai lain (cek grep).

### D — Docs
CLAUDE.md Sec 3 (Core: tambah `notification_emails`, catat migrasi `025`), Sec 12 (deviasi: hard delete retensi 90 hari,
tanpa audit per notifikasi), `docs/data-map.md` (kolom), `development-progress.md`, `sdlc/README.md`,
`development-plan.md` 0.3.

## Tests (TDD — ditulis sebelum implementasi tiap langkah)
| Tes | Jenis | Membuktikan |
|---|---|---|
| `notification/notification_test.go` | unit (fake querier) | validasi FR1.5; dedupe user (user ∈ UserIDs ∩ role) + email case-insensitive; PIC → email tanpa in-app; penerima kosong → 0 insert; SMTP off → `skipped` |
| `notification/worker_test.go` | unit (fake mailer, jam palsu) | sukses → sent; gagal 1/2 → retry +1 m/+5 m; gagal ke-3 → failed; purge hanya jalan bila > 24 jam |
| `notification/smtp_test.go` | unit (server SMTP palsu di `net.Listen`, stdlib) | header UTF-8 Q-encoded, timeout tidak menggantung |
| `notification/repository_topology_test.go` | unit | List/Count/Unread → replica; MarkRead/MarkAll → primary (pola `rolemgmt`) |
| `notification/integration_test.go` (`-tags integration`) | DB dev | Send dalam tx lalu rollback → 0 baris; expand role/vendor nyata; MarkRead milik orang lain → not found; retensi hapus > 90 hari |
| `handler/notification_handler_test.go` | unit | 401 tanpa token; 404 milik orang lain; 400 id/page tidak valid; bentuk JSON |
| `service/vendor_request_completion_test.go` (+integration) | unit + DB | over-quota → 1 `Send` (maker + ATM-SPV, Email true); tanpa over-quota → 0; Send error → approve gagal, kunjungan tidak tercatat |
| CompanyPortal `NotificationBell.test.tsx` | vitest | 0 → badge hilang; 120 → "99+"; tandai semua → angka 0 tanpa refetch |
| VendorPortal `useNotifications.test.ts` | vitest | memanggil API (bukan mock JSON); mark read memperbarui cache |

Lalu: `go test ./...`, `go test -tags integration ./internal/...`, `pnpm --dir ... run test|lint|build` kedua portal.
Manual browser check = **user** (Golden Rule #10).

## Risiko
- **Beban polling**: 300 user × 1/menit ke replica — kecil; index partial `is_read = false`.
- **Lag replica** setelah tandai dibaca → diatasi update cache optimistic (FR4.5).
- **Dua instance `cmd/api`** (rolling deploy) → `SKIP LOCKED` mencegah email ganda; retensi ganda aman (idempotent).
- **Email ke ATM-SPV** → satu email per approve (bukan per ATM).
- **Relay SMTP perusahaan** belum diketahui (port, TLS, auth) → dev memakai `SMTP_HOST` kosong (`skipped`); uji kirim nyata
  menunggu kredensial relay (dicatat outstanding di `tests.md`).

## Out of scope
Sama dengan spec.

## Catatan implementasi (2026-10-07)
Semua langkah M1, B1–B5, F1, F2, D selesai. Deviasi kecil dari teks plan:
- **D1**: interface `service.Notifier` memakai tipe `notification.Message/Recipients/Querier`, jadi paket `service` meng-import `internal/notification` (tanpa siklus: `notification` hanya bergantung pada `internal/db`). Tes lama tidak berubah karena notifier `nil` = tanpa notifikasi.
- **Retry**: "retry 3x" (intent #2) diterapkan sebagai 1 percobaan + 3 retry (tunggu 1/5/15 m), `failed` setelah percobaan ke-4 — memakai ketiga backoff di spec.
- **F1**: `NotificationBell` dipasang sebagai slot `notifications` (`routes/_protected.tsx` → `AppShell` → `Header`), bukan langsung di `Header`, supaya tes layout yang ada (tanpa QueryClientProvider) tidak berubah. Titik merah statis lama dihapus. Panel memakai `<dialog open>` (lint a11y).
- **F2**: tipe API baru `VendorNotification` di `features/notifications/api.ts`. `lib/types.ts` `Notification` + `data/notifications.json` **tidak** dihapus: masih dipakai `routing.property.test.ts` dan `lib/__tests__/dataFilters.property.test.ts` (keduanya menguji isi mock, bukan perilaku app). Halaman memuat 100 terbaru tanpa paging (`ponytail:` di hook). Badge sidebar memakai `useUnreadCount` (tidak memuat daftar).
- **B5**: konfigurasi SMTP dibaca di `main.go` (`loadNotificationConfig`) seperti `DSR_*`; `backend/.env.example` blok SMTP dibuka + `NOTIFICATION_EMAIL_POLL_INTERVAL`.
