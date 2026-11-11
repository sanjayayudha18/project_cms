-- 039_ext_api_clients.sql
-- ext-user-api (.kiro/specs/ext-user-api/). Additive, forward-only:
--   1. role EXT-CLIENT (akun layanan client eksternal; tanpa role_permissions = tanpa hak menu)
--   2. komentar users.auth_source memuat nilai baru 'service' (kolom tidak berubah)
--   3. tabel api_clients (OAuth2 client credentials; secret hanya disimpan sebagai bcrypt)
-- Tidak ada DELETE untuk api_clients: client dinonaktifkan (is_active, disabled_at).
--
-- ROLLBACK (documented, forward-only shop):
--   DROP TABLE public.api_clients;
--   DELETE FROM public.roles WHERE role = 'EXT-CLIENT';  -- hanya bila tidak ada users.role_id yang memakainya
--   COMMENT ON COLUMN public.users.auth_source IS 'ldap | local';

BEGIN;

-- 1. Role tanpa hak untuk akun layanan. Sengaja tidak ada baris role_permissions.
INSERT INTO public.roles (role, description)
SELECT 'EXT-CLIENT', 'Akun layanan aplikasi eksternal (ext-user-api). Tidak bisa login, tanpa hak menu.'
WHERE NOT EXISTS (SELECT 1 FROM public.roles WHERE role = 'EXT-CLIENT');

-- 2. Nilai auth_source baru (hanya dokumentasi)
COMMENT ON COLUMN public.users.auth_source IS
  'ldap | local | local_dev | service (akun layanan ext-user-api, tidak bisa login)';

-- 3. Client eksternal
CREATE TABLE public.api_clients (
    id                 bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    client_id          text NOT NULL UNIQUE CHECK (client_id ~ '^crown_[a-z2-7]{26}$'),
    name               text NOT NULL UNIQUE CHECK (length(btrim(name)) BETWEEN 1 AND 100),
    secret_hash        text NOT NULL,
    credential_version integer NOT NULL DEFAULT 1 CHECK (credential_version > 0),
    scopes             text[] NOT NULL CHECK (cardinality(scopes) > 0
                         AND scopes <@ ARRAY['users:read','users:write']::text[]),
    assignable_roles   text[] NOT NULL DEFAULT '{}'
                         CHECK (NOT (assignable_roles && ARRAY['ADMIN','APPACCESS','ADMIN_PARAM','EXT-CLIENT']::text[])),
    allowed_cidrs      cidr[] NOT NULL DEFAULT '{}',
    service_user_id    bigint NOT NULL UNIQUE REFERENCES public.users(id),
    is_active          boolean NOT NULL DEFAULT true,
    created_by         bigint NOT NULL REFERENCES public.users(id),
    created_at         timestamptz NOT NULL DEFAULT now(),
    updated_at         timestamptz NOT NULL DEFAULT now(),
    secret_rotated_at  timestamptz,
    disabled_at        timestamptz
);

-- FK created_by tidak ter-index otomatis (service_user_id sudah lewat UNIQUE).
CREATE INDEX api_clients_created_by_idx ON public.api_clients (created_by);

COMMENT ON TABLE public.api_clients IS
  'Aplikasi eksternal (OAuth2 client credentials, ext-user-api, dilayani backend-extapi). secret_hash = bcrypt; secret asli tidak pernah disimpan.';
COMMENT ON COLUMN public.api_clients.credential_version IS
  'Naik setiap rotate; token dengan claim ver lama ditolak.';
COMMENT ON COLUMN public.api_clients.assignable_roles IS
  'Allowlist role yang boleh diberikan client (D12). CHECK = cermin pkg/userrules.PrivilegedRoles.';

COMMIT;
