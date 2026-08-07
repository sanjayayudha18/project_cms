BEGIN;

-- =========================================================
-- 1) COMMON TRIGGER FOR updated_at
-- =========================================================
CREATE OR REPLACE FUNCTION set_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = now();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- =========================================================
-- 2) MASTER TABLES
-- =========================================================
CREATE TABLE roles (
    id BIGSERIAL PRIMARY KEY,
    role TEXT NOT NULL UNIQUE,
    description TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT roles_role_chk CHECK (
        role IN (
            'ADMIN',
            'ADMIN_PARAM',
            'ATM-USER',
            'ATM-SPV',
            'BRANCH-USER',
            'BRANCH-SPV',
            'BRANCH-ATM-USER',
            'BRANCH-ATM-SPV',
            'VENDOR-USER'
        )
    )
);

COMMENT ON COLUMN roles.role IS 'ADMIN | ADMIN_PARAM | ATM-USER | ATM-SPV | BRANCH-USER | BRANCH-SPV | BRANCH-ATM-USER | BRANCH-ATM-SPV | VENDOR-USER';

CREATE TABLE vendors (
    id BIGSERIAL PRIMARY KEY,
    code TEXT NOT NULL UNIQUE,
    name TEXT NOT NULL,
    contact_email TEXT,
    contact_phone TEXT,
    address TEXT,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at TIMESTAMPTZ
);

-- =========================================================
-- 3) USERS
-- =========================================================
CREATE TABLE users (
    id BIGSERIAL PRIMARY KEY,
    role_id BIGINT NOT NULL,
    employee_id TEXT UNIQUE,
    username TEXT NOT NULL UNIQUE,
    full_name TEXT NOT NULL,
    email TEXT NOT NULL UNIQUE,
    is_karyawan BOOLEAN NOT NULL,
    auth_source TEXT NOT NULL,
    password_hash TEXT,
    vendor_id BIGINT,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    last_login_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at TIMESTAMPTZ,

    CONSTRAINT users_role_fk
        FOREIGN KEY (role_id) REFERENCES roles(id),

    CONSTRAINT users_vendor_fk
        FOREIGN KEY (vendor_id) REFERENCES vendors(id),

    CONSTRAINT users_auth_source_chk
        CHECK (auth_source IN ('ldap', 'local')),

    CONSTRAINT users_password_hash_chk
        CHECK (
            (auth_source = 'local' AND password_hash IS NOT NULL)
            OR
            (auth_source = 'ldap' AND password_hash IS NULL)
        ),

    CONSTRAINT users_vendor_logic_chk
        CHECK (
            (is_karyawan = TRUE  AND auth_source = 'ldap'  AND vendor_id IS NULL)
            OR
            (is_karyawan = FALSE AND auth_source = 'local' AND vendor_id IS NOT NULL)
        )
);

COMMENT ON COLUMN users.employee_id IS 'Nullable for external/vendor users';
COMMENT ON COLUMN users.auth_source IS 'ldap | local';
COMMENT ON COLUMN users.password_hash IS 'Only for auth_source=local';
COMMENT ON COLUMN users.vendor_id IS 'Required for vendor/local users, null for internal LDAP users';

-- =========================================================
-- 4) INDEXES
-- =========================================================
CREATE INDEX users_role_idx ON users(role_id);
CREATE INDEX users_vendor_idx ON users(vendor_id);
CREATE INDEX users_is_active_idx ON users(is_active);
CREATE INDEX vendors_is_active_idx ON vendors(is_active);

-- Optional helpful partial indexes for soft delete usage
CREATE INDEX users_active_not_deleted_idx
    ON users(username)
    WHERE deleted_at IS NULL;

CREATE INDEX vendors_active_not_deleted_idx
    ON vendors(code)
    WHERE deleted_at IS NULL;

-- =========================================================
-- 5) updated_at TRIGGERS
-- =========================================================
CREATE TRIGGER trg_roles_set_updated_at
BEFORE UPDATE ON roles
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();

CREATE TRIGGER trg_vendors_set_updated_at
BEFORE UPDATE ON vendors
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();

CREATE TRIGGER trg_users_set_updated_at
BEFORE UPDATE ON users
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();

COMMIT;