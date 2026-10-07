# Intent: Modul notifikasi (in-app + email SMTP) — Phase 0.3

Author: user (product owner), ditulis bersama Claude. Status: **accepted 2026-10-07 (PO, via chat)**.
Stage: 1 Plan. Stage berikut: `spec.md` (setelah intent diterima).
Menyentuh Sec 4 #7: **skema (tabel baru `notifications`, migrasi baru)** → wajib AI-DLC penuh.

## Problem
Beberapa fitur sudah atau akan butuh notifikasi, tapi modulnya belum ada (`.kiro/steering/development-plan.md` 0.3):

- `dsr-late-report` (intent, keputusan #5): DSR telat/tidak masuk → notifikasi ke PIC vendor + Operator internal, in-app + email, dicek 09:00 WIB.
- `atm-visit-quota` (intent #12): kelebihan kuota kunjungan → sekarang hanya peringatan di layar + badge; in-app/email "menyusul modul 0.3".
- `vendor-pks-cis-limit` (out of scope): email PKS mendekati expired menunggu 0.3.
- `vendor-upload-dsr` spec: vendor dapat notifikasi **in-app** soal hasil upload, lewat "existing `internal/notification`" — modul itu belum ada.
- Flow replenish (Phase 2): setelah order disetujui → terbitkan instruksi pengisian + notifikasi ke vendor.
- CLAUDE.md Sec 14: email alert ke admin/app-support saat EOD gagal / belum selesai sebelum jam kantor.

Kondisi sekarang (dicek di kode 2026-10-07):
- Tabel `notifications` **belum ada** (nama sudah tercantum di CLAUDE.md Sec 3, grup Core).
- `internal/notification` belum ada di `backend/internal/`.
- Env `SMTP_HOST/PORT/USER/PASSWORD/FROM` hanya komentar di `backend/.env.example`; tidak ada field SMTP di `pkg/config`.
- UI: CompanyPortal `Header.tsx` punya tombol lonceng + titik merah statis (mock). VendorPortal punya `NotificationBadge` (angka unread) yang belum terhubung API.

## Proposed outcome
1. Satu modul `internal/notification` yang bisa dipanggil fitur lain (DSR-late, kuota, replenish, PKS, EOD) untuk mengirim notifikasi ke **user** tertentu atau ke **role/vendor**.
2. **In-app**: notifikasi tersimpan di DB; user melihat daftar + jumlah unread di lonceng (CompanyPortal) dan badge (VendorPortal), lalu bisa menandai sudah dibaca.
3. **Email** lewat SMTP relay perusahaan untuk jenis notifikasi yang memang butuh email.
4. Kegagalan kirim email tidak membatalkan transaksi bisnis yang memicunya, dan tercatat (bisa dilihat/di-retry).
5. Fitur pertama yang memakai modul ini dipilih di spec (kandidat: DSR-late 2.3, kelebihan kuota, hasil upload DSR).

## Affected users and systems
- Users: semua role internal (penerima in-app), Vendor (VendorPortal), Admin/app-support (alert EOD).
- Systems: `backend/` (migrasi, `queries/`, `internal/notification`, handler `/api/v1/notifications`), `pkg/config` (SMTP), kedua frontend (lonceng/badge + daftar), mungkin `backend_python/` (alert EOD — lihat open question 6).

## Constraints
- Stack tetap (Sec 2): email pakai **stdlib `net/smtp`** — tidak ada dependency baru tanpa persetujuan.
- Timestamp `timestamptz` UTC, tampil Asia/Jakarta (Sec 6). Tulis ke primary; daftar/hitung unread boleh dari replica, tapi "tandai dibaca" lalu refresh = read-after-write → primary.
- Vendor hanya melihat notifikasi miliknya / vendor-nya sendiri (Sec 5 scoping). RBAC di route **dan** service.
- Jangan pernah log kredensial SMTP atau isi token (Sec 9).
- Bukan maker-checker: notifikasi adalah efek samping, bukan perubahan data finansial/master. Audit: lihat open question 4.
- NFR: 300 user bersamaan; polling unread tidak boleh membebani primary.

## Open questions (dijawab PO sebelum spec)
1. **Penerima**: alamat per user (`users.email`?), per role, per vendor (PIC vendor `vendor_pics.email`?), atau kombinasi? Kalau ke role, dikirim ke semua user aktif role itu saat event terjadi?
2. **Pengiriman email**: sinkron di request, atau ditulis ke tabel lalu dikirim worker/goroutine (outbox) dengan retry? Berapa kali retry?
3. **Real-time in-app**: cukup polling (mis. tiap 60 dtk), atau perlu push (SSE/WebSocket)?
4. **Audit**: perlu `audit_logs` untuk setiap notifikasi terkirim, atau cukup status di tabel `notifications`? "Tandai dibaca" perlu diaudit?
5. **Retensi**: notifikasi lama dihapus/arsip setelah berapa hari? (Sec 12: soft-delete only untuk master data; notifikasi boleh beda?)
6. **Alert EOD**: dikirim dari Python (`eod_retry_scheduler`) langsung via SMTP, atau Python menulis ke `notifications` dan Go yang mengirim?
7. **Template email**: bahasa (ID/EN), format (plain text vs HTML), siapa yang menyetujui isi?
8. **Preferensi user**: user boleh mematikan email per jenis notifikasi, atau semua wajib?
9. **Fitur pemakai pertama** untuk membuktikan modul ini (lihat outcome 5).

## Resolved decisions (PO, 2026-10-07)
1. **Penerima**: user tertentu, semua user aktif suatu role (di-expand saat event), atau vendor (user vendor aktif → in-app + email; `vendor_pics.is_notification_recipient = true` → email). Alamat user = `users.email`.
2. **Email**: outbox di tx yang sama dengan event bisnis + goroutine pengirim di `cmd/api`; retry 3x lalu `failed`. Email gagal tidak membatalkan transaksi.
3. **Real-time**: polling unread tiap 60 detik, tanpa SSE/WebSocket.
4. **Audit**: tidak ada `audit_logs` per notifikasi; status di tabel cukup. "Tandai dibaca" tidak diaudit.
5. **Retensi**: 90 hari, lalu dihapus otomatis (notifikasi bukan master data).
6. **Alert EOD** dari Python ditunda ke Phase 7.
7. **Template**: Bahasa Indonesia, plain text.
8. **Preferensi user**: tidak ada (semua jenis aktif).
9. **Pemakai pertama**: kelebihan kuota kunjungan (`atm-visit-quota`, saat approve laporan selesai).

## Out of scope
- Push notification mobile, SMS, WhatsApp.
- Isi notifikasi tiap fitur (ditentukan di spec fitur masing-masing).
- Scheduler 09:00 untuk DSR-late (milik `dsr-late-report`).
