BEGIN;

-- 1) Revert internal users back to ldap, remove password_hash
UPDATE users
SET auth_source = 'ldap', password_hash = NULL
WHERE is_karyawan = TRUE AND auth_source = 'local_dev';

-- 2) Restore original constraints
ALTER TABLE users DROP CONSTRAINT users_vendor_logic_chk;
ALTER TABLE users ADD CONSTRAINT users_vendor_logic_chk
    CHECK (
        (is_karyawan = TRUE AND auth_source = 'ldap' AND vendor_id IS NULL)
        OR (is_karyawan = FALSE AND auth_source = 'local' AND vendor_id IS NOT NULL)
    );

ALTER TABLE users DROP CONSTRAINT users_password_hash_chk;
ALTER TABLE users ADD CONSTRAINT users_password_hash_chk
    CHECK (
        (auth_source = 'local' AND password_hash IS NOT NULL)
        OR (auth_source = 'ldap' AND password_hash IS NULL)
    );

ALTER TABLE users DROP CONSTRAINT users_auth_source_chk;
ALTER TABLE users ADD CONSTRAINT users_auth_source_chk
    CHECK (auth_source IN ('ldap', 'local'));

COMMIT;
