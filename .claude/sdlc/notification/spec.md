# Spec: Modul notifikasi (in-app + email SMTP) — Phase 0.3

Status: **accepted 2026-10-07** (user: "S1 maker + ATM-SPV saja, S2 in-app + email, lanjut plan"). Stage: 2 Design — selesai.
Input: `intent.md` (accepted 2026-10-07, keputusan 1–9).

## Definisi
- **Notifikasi in-app** = satu baris `notifications` per **user penerima** (fan-out saat kirim). Dibaca di lonceng
  (CompanyPortal) / badge + halaman Notifikasi (VendorPortal).
- **Email** = satu baris `notification_emails` (outbox) per **alamat**. Dikirim goroutine pengirim di `cmd/api`, terpisah
  dari request yang memicunya.
- **Penerima** (`Recipients`) = gabungan dari:
  - `UserIDs` — user tertentu;
  - `Roles` — semua user **aktif** (`users.is_active`, `deleted_at IS NULL`) dengan role itu, di-expand **saat kirim**;
  - `VendorIDs` — semua user aktif dengan `users.vendor_id` itu, **plus** email ke `vendor_pics` aktif vendor itu yang
    `is_notification_recipient = true` (PIC bukan user → email saja, tanpa in-app).
  User yang muncul lebih dari sekali hanya dapat satu notifikasi; alamat email yang sama (case-insensitive) hanya satu email.

## Functional requirements

### FR1 Kirim (API Go internal, dipanggil fitur lain)
- FR1.1 `notification.Send(ctx, q *db.Queries, msg Message, to Recipients) error` — `q` adalah query **di dalam tx
  pemanggil**, jadi notifikasi + outbox ikut commit/rollback bersama event bisnis (keputusan 2).
- FR1.2 `Message{Type, Title, Body, Link, EntityType, EntityID, Email bool}`. `Type` = kode stabil (mis.
  `visit_quota.over_quota`); `Title` ≤ 120 karakter; `Body` ≤ 1000; `Link` = path relatif di portal penerima (boleh kosong).
- FR1.3 `Email = true` → satu baris outbox per alamat unik penerima. `SMTP_HOST` kosong → baris outbox tetap ditulis
  dengan status `skipped` (dev tidak menumpuk antrian yang nanti terkirim mendadak).
- FR1.4 Penerima kosong setelah expand → bukan error; tidak menulis apa pun, log `info`.
- FR1.5 Validasi gagal (Type/Title kosong, terlalu panjang) → error; pemanggil memutuskan rollback. Fitur pemakai
  memakai konstanta, jadi ini bug pemrograman, bukan input user.

### FR2 Pengirim email (goroutine di `cmd/api`)
- FR2.1 Jalan hanya bila `SMTP_HOST` terisi. Tiap `NOTIFICATION_EMAIL_POLL_INTERVAL` (default `60s`) ambil maks 50 baris
  `status = 'pending' AND next_attempt_at <= now()` dengan `FOR UPDATE SKIP LOCKED` (aman bila ada >1 instance).
- FR2.2 Kirim via **stdlib `net/smtp`** (`SendMail` — STARTTLS bila server mendukung; `PlainAuth` hanya bila `SMTP_USER`
  terisi). Plain text, UTF-8, `From: SMTP_FROM`.
- FR2.3 Sukses → `sent`, `sent_at = now()`. Gagal → `attempts + 1`, `last_error` (dipotong 500 karakter, tanpa
  kredensial), `next_attempt_at = now() + backoff` (1 m, 5 m, 15 m). Setelah percobaan ke-3 gagal → `failed`.
- FR2.4 Berhenti rapi saat shutdown (context dari `main`).

### FR3 Retensi
- FR3.1 Goroutine yang sama, sekali per hari: hapus `notifications` dengan `created_at < now() - 90 hari` dan
  `notification_emails` dengan status final (`sent`/`failed`/`skipped`) yang lebih tua dari 90 hari (keputusan 5).
  Dijalankan juga bila SMTP kosong (retensi tidak bergantung email).

### FR4 HTTP API — `/api/v1/notifications` (RequireAuth, semua role termasuk `VENDOR-USER`)
Bentuk JSON flat, mengikuti handler ATM lain (Sec 5). User selalu dari JWT claim `id` — tidak ada parameter user.
- FR4.1 `GET /?unread_only=true|false&page=1&page_size=20` (page_size maks 100) →
  `{ "items": [{ "id", "type", "title", "body", "link", "is_read", "created_at" }], "total", "page", "page_size" }`,
  urut `created_at DESC, id DESC`. **Replica.**
- FR4.2 `GET /unread-count` → `{ "unread_count": n }`. **Replica.** Dipolling frontend tiap 60 dtk.
- FR4.3 `POST /{id}/read` → `204`. Hanya milik sendiri; milik orang lain / tidak ada → `404` (tidak membocorkan
  keberadaan). Sudah dibaca → tetap `204` (idempotent). **Primary.**
- FR4.4 `POST /read-all` → `{ "updated": n }`. **Primary.**
- FR4.5 Setelah FR4.3/FR4.4 frontend memperbarui cache sendiri (optimistic), **tidak** langsung refetch dari replica
  (hindari baca basi karena lag).

### FR5 Frontend
- FR5.1 **CompanyPortal** `components/layout/Header.tsx`: titik merah statis diganti angka unread (1–99, "99+", sembunyi
  bila 0; teks + `aria-label`, bukan warna saja). Klik → panel 10 notifikasi terbaru, item klik → tandai dibaca + navigasi
  ke `link`; tombol "Tandai semua dibaca". Polling 60 dtk (`refetchInterval`).
- FR5.2 **VendorPortal** `features/notifications/useNotifications.ts`: ganti `data/notifications.json` (mock) dengan API
  FR4; `useMarkAsRead`/`useMarkAllAsRead` memanggil FR4.3/FR4.4. Tipe `Notification` di `lib/types.ts` disesuaikan
  (`type` jadi string kode, tambah `title`; `vendorId` dihapus — scoping di server). `NotificationBadge` tetap.
  Mock `data/notifications.json` dihapus bila tidak dipakai lagi.

### FR6 Pemakai pertama — kelebihan kuota kunjungan (keputusan 9)
- FR6.1 Di `VendorRequestService.ApproveCompletion` (`internal/service/vendor_request_completion.go`), **di dalam tx yang
  sama**, bila `overQuota` tidak kosong → satu `Send`:
  - `Type = "visit_quota.over_quota"`, `Title = "Kelebihan kuota kunjungan"`,
  - `Body = "Vendor request <request_number>: ATM <daftar terminal> melebihi kuota kunjungan replenish."`,
  - `Link = "/replenishment/vendor-requests/<id>"`, `EntityType = "vendor_request"`, `EntityID = id`, `Email = true`,
  - Penerima: user `completion_submitted_by` (maker laporan) + role `ATM-SPV` (S1).
- FR6.2 Gagal `Send` → approve ikut gagal (rollback), sama seperti kegagalan DB lain di tx itu. Gagal **kirim email**
  tidak memengaruhi approve (outbox).
- FR6.3 Peringatan di layar + `over_quota_terminals` di response tetap seperti sekarang.

## Non-functional
- Polling unread: query ber-index `(recipient_user_id) WHERE is_read = false`, dari **replica**; target p95 < 50 ms
  (300 user × 1 req/menit ≈ 5 req/dtk).
- Jangan log alamat email lengkap, isi body, kredensial SMTP (Sec 9). Log: `notification_email_id`, status, attempts.
- Timestamp `timestamptz` UTC; tampil Asia/Jakarta di frontend.

## Config (ATM-only → dibaca di `cmd/api/main.go` seperti `DSR_*`, bukan `pkg/config` yang dipakai bersama backend-cit)
`SMTP_HOST`, `SMTP_PORT` (default 587), `SMTP_USER`, `SMTP_PASSWORD`, `SMTP_FROM` (wajib bila `SMTP_HOST` terisi →
startup error kalau kosong), `NOTIFICATION_EMAIL_POLL_INTERVAL` (default `60s`). `backend/.env.example` di-update
(blok SMTP yang sekarang komentar).

## Data model (migrasi baru `025_notifications.sql`)
> Nomor: `022` dicadangkan `vendor-pks-cis-limit`, `024` sudah dipakai seed dev (diarsipkan) → `025`.
> `notifications` sudah ada di CLAUDE.md Sec 3 (Core). **`notification_emails` = nama tabel baru → diajukan ke Sec 3 (Core)
> lewat spec ini.**

```sql
CREATE TABLE public.notifications (
  id                bigserial PRIMARY KEY,
  recipient_user_id bigint NOT NULL REFERENCES public.users(id),
  type              text NOT NULL,
  title             text NOT NULL CHECK (char_length(title) BETWEEN 1 AND 120),
  body              text NOT NULL DEFAULT '' CHECK (char_length(body) <= 1000),
  link              text,
  entity_type       text,
  entity_id         bigint,
  is_read           boolean NOT NULL DEFAULT false,
  read_at           timestamptz,
  created_at        timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX notifications_recipient_created_idx ON public.notifications (recipient_user_id, created_at DESC, id DESC);
CREATE INDEX notifications_unread_idx ON public.notifications (recipient_user_id) WHERE is_read = false;
CREATE INDEX notifications_created_idx ON public.notifications (created_at);  -- retensi

CREATE TABLE public.notification_emails (
  id              bigserial PRIMARY KEY,
  notification_id bigint REFERENCES public.notifications(id) ON DELETE SET NULL, -- NULL untuk PIC vendor
  to_address      text NOT NULL,
  subject         text NOT NULL,
  body            text NOT NULL,
  status          text NOT NULL DEFAULT 'pending'
                  CHECK (status IN ('pending','sent','failed','skipped')),
  attempts        integer NOT NULL DEFAULT 0,
  last_error      text,
  next_attempt_at timestamptz NOT NULL DEFAULT now(),
  sent_at         timestamptz,
  created_at      timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX notification_emails_pending_idx ON public.notification_emails (next_attempt_at) WHERE status = 'pending';
CREATE INDEX notification_emails_created_idx ON public.notification_emails (created_at);
```

Kode: `backend/queries/notifications.sql` + `sqlc generate` (v1.31.1, perbaiki `UserLeafe`), paket
`backend/internal/notification/` (service `Send`, sender, handler-facing read/mark), handler
`internal/handler/notification_handler.go`, wiring di `cmd/api/main.go` (repo dengan `dbPool` + `dbReadPool`).

## Sec 4 #7 flags
- **Migrasi**: 2 tabel baru (`025`), satu nama tabel baru (`notification_emails`) → perlu approval di sini lalu dicatat di Sec 3.
- **Delete**: retensi 90 hari = **hard delete** `notifications`/`notification_emails` — deviasi dari pola soft-delete
  (Sec 12 berlaku untuk master data; keputusan PO #5). Dicatat di Sec 12 setelah diterapkan.
- **Audit**: tidak ada `audit_logs` per notifikasi / tandai dibaca (keputusan PO #4) — notifikasi efek samping, bukan
  perubahan data finansial/master. Event pemicunya (approve laporan selesai) tetap diaudit seperti sekarang.
- Auth/money/recon: tidak tersentuh. Scoping: user hanya melihat & mengubah notifikasinya sendiri (FR4).

## Acceptance criteria
1. Approve laporan selesai yang membuat ≥1 ATM kelebihan kuota → maker laporan + semua ATM-SPV aktif dapat 1 notifikasi
   in-app masing-masing, dan 1 baris outbox per alamat unik; tanpa kelebihan kuota → tidak ada notifikasi.
2. Tx approve di-rollback (mis. error DB setelah `Send`) → tidak ada baris `notifications`/`notification_emails`.
3. SMTP kosong → outbox `skipped`, approve sukses. SMTP mati → 3 percobaan (1 m/5 m/15 m) lalu `failed`; approve tidak terpengaruh.
4. User A memanggil `POST /notifications/{id milik B}/read` → `404`; `GET` hanya mengembalikan milik A.
5. VENDOR-USER melihat notifikasinya lewat API nyata (badge + halaman), bukan mock.
6. List + unread-count lewat replica; read/read-all lewat primary (tes topologi seperti `rolemgmt`).
7. Notifikasi > 90 hari terhapus oleh job retensi.
8. Lonceng CompanyPortal menampilkan angka unread (teks, bukan hanya warna) dan bisa tandai dibaca.

## Resolved (PO, 2026-10-07)
- **S1**: penerima over-quota = maker laporan (`completion_submitted_by`) + semua `ATM-SPV` aktif. **Tanpa** `BRANCH-ATM-SPV`, tanpa vendor.
- **S2**: in-app + email.
- Nama tabel `notification_emails` dan hard delete retensi 90 hari disetujui bersama spec ini.

## Out of scope
- Alert EOD dari Python (Phase 7), notifikasi DSR-late (`dsr-late-report`), PKS expired, publish instruksi replenish (Phase 2).
- Preferensi per user, SSE/WebSocket, HTML email, lampiran email, layar admin outbox / retry manual.
- Halaman "semua notifikasi" di CompanyPortal (panel lonceng cukup untuk sekarang).
