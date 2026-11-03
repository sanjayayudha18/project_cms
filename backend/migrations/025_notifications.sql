-- 025_notifications.sql
-- Modul notifikasi in-app + email SMTP (Phase 0.3,
-- .claude/sdlc/notification/spec.md, accepted 2026-10-07).
--
--   * notifications: satu baris per user penerima (fan-out saat kirim).
--   * notification_emails: outbox email, satu baris per alamat; dikirim
--     goroutine di cmd/api (retry 1/5/15 menit, lalu failed).
--
-- Retensi 90 hari = hard delete oleh worker (deviasi soft-delete yang
-- disetujui PO 2026-10-07; notifikasi bukan master data).
--
-- SAFETY:
--   * Additive only (2 tabel baru).
--   * Forward-only: no down migration (project convention).
BEGIN;

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
CREATE INDEX notifications_created_idx ON public.notifications (created_at);

CREATE TABLE public.notification_emails (
    id              bigserial PRIMARY KEY,
    notification_id bigint REFERENCES public.notifications(id) ON DELETE SET NULL,
    to_address      text NOT NULL,
    subject         text NOT NULL,
    body            text NOT NULL,
    status          text NOT NULL DEFAULT 'pending'
                    CHECK (status IN ('pending', 'sent', 'failed', 'skipped')),
    attempts        integer NOT NULL DEFAULT 0,
    last_error      text,
    next_attempt_at timestamptz NOT NULL DEFAULT now(),
    sent_at         timestamptz,
    created_at      timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX notification_emails_pending_idx ON public.notification_emails (next_attempt_at) WHERE status = 'pending';
CREATE INDEX notification_emails_created_idx ON public.notification_emails (created_at);

COMMENT ON TABLE public.notifications IS
  'Notifikasi in-app, satu baris per user penerima. Dibaca hanya oleh penerimanya. Dihapus setelah 90 hari (retensi). Migrasi 025.';
COMMENT ON TABLE public.notification_emails IS
  'Outbox email notifikasi, satu baris per alamat. notification_id NULL untuk PIC vendor (bukan user). status: pending|sent|failed|skipped (skipped = SMTP tidak dikonfigurasi). Migrasi 025.';

COMMIT;
