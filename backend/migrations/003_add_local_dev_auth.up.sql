BEGIN;

-- =========================================================
-- Migration: Add 'local_dev' auth source for development
-- Purpose: Allow internal (karyawan) users to login with
--          local bcrypt credentials during development,
--          bypassing LDAP dependency.
-- =========================================================

-- 1) Expand auth_source constraint to include 'local_dev'
ALTER TABLE users DROP CONSTRAINT users_auth_source_chk;
ALTER TABLE users ADD CONSTRAINT users_auth_source_chk
    CHECK (auth_source IN ('ldap', 'local', 'local_dev'));

-- 2) Expand password_hash constraint to allow local_dev
ALTER TABLE users DROP CONSTRAINT users_password_hash_chk;
ALTER TABLE users ADD CONSTRAINT users_password_hash_chk
    CHECK (
        (auth_source = 'local' AND password_hash IS NOT NULL)
        OR (auth_source = 'local_dev' AND password_hash IS NOT NULL)
        OR (auth_source = 'ldap' AND password_hash IS NULL)
    );

-- 3) Expand vendor logic constraint to allow local_dev for internal users
ALTER TABLE users DROP CONSTRAINT users_vendor_logic_chk;
ALTER TABLE users ADD CONSTRAINT users_vendor_logic_chk
    CHECK (
        (is_karyawan = TRUE AND auth_source = 'ldap' AND vendor_id IS NULL)
        OR (is_karyawan = TRUE AND auth_source = 'local_dev' AND vendor_id IS NULL)
        OR (is_karyawan = FALSE AND auth_source = 'local' AND vendor_id IS NOT NULL)
    );

-- 4) Update internal users to local_dev with default dev password
-- bcrypt hash of "Password123!" with cost 12
UPDATE users
SET auth_source = 'local_dev',
    password_hash = '$2a$12$LJ3m4sMKfRzL7P8bN5Q2kuXjVnZ8Y1p6w3dK9RtHmQvWuC0xOyGNi'
WHERE is_karyawan = TRUE AND auth_source = 'ldap';

COMMIT;
