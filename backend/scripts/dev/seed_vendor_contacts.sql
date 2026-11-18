-- DEV ONLY: random contact data for vendors (email, phone, Direktur Utama).
-- Idempotent: fills only empty vendors.contact_email/contact_phone and adds a
-- director PIC only for FLM vendors that have none. Bypasses maker-checker on
-- purpose (dev data). Emails use the reserved example.com domain.
-- Run: psql "$DATABASE_URL" -X -v ON_ERROR_STOP=1 -f backend/scripts/dev/seed_vendor_contacts.sql
BEGIN;

UPDATE vendors
SET contact_email = 'ops.' || lower(code) || '@vendor.example.com'
WHERE deleted_at IS NULL AND coalesce(contact_email, '') = '';

UPDATE vendors
SET contact_phone = '021-' || lpad((floor(random() * 10000000))::int::text, 7, '0')
WHERE deleted_at IS NULL AND coalesce(contact_phone, '') = '';

INSERT INTO vendor_pics (vendor_id, vendor_branch_id, name, position, phone, email,
                         is_notification_recipient, roles, is_director)
SELECT v.id, NULL,
       (ARRAY['Budi Santoso','Hendra Wijaya','Siti Rahmawati','Agus Prasetyo','Dewi Lestari',
              'Rudi Hartono','Linda Kusuma','Andi Saputra','Maya Anggraini','Joko Susilo'])
         [1 + floor(random() * 10)::int],
       'Direktur Utama',
       '0812' || lpad((floor(random() * 100000000))::int::text, 8, '0'),
       'dirut.' || lower(v.code) || '@vendor.example.com',
       false, '{}'::text[], true
FROM vendors v
WHERE v.deleted_at IS NULL AND v.kind = 'FLM_VENDOR'
  AND NOT EXISTS (SELECT 1 FROM vendor_pics p
                  WHERE p.vendor_id = v.id AND p.is_director AND p.deleted_at IS NULL);

COMMIT;
