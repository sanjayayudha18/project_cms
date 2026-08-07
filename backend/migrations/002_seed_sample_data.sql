BEGIN;

-- =========================================================
-- 1) MINIMAL ROLES NEEDED BY THE SAMPLE USERS
-- =========================================================
INSERT INTO roles (role, description)
VALUES
  ('ADMIN', 'System administrator'),
  ('ADMIN_PARAM', 'Parameter administrator'),
  ('ATM-USER', 'ATM operator user'),
  ('BRANCH-USER', 'Branch user'),
  ('VENDOR-USER', 'Vendor user')
ON CONFLICT (role) DO UPDATE
SET
  description = EXCLUDED.description,
  updated_at = now();

-- =========================================================
-- 2) SAMPLE VENDORS
-- =========================================================
INSERT INTO vendors (
  code,
  name,
  contact_email,
  contact_phone,
  address,
  is_active
)
VALUES
  (
    'VND001',
    'PT Crown Teknologi',
    'ops@crownvendor.co.id',
    '+62 21 555 1001',
    'Jakarta',
    TRUE
  ),
  (
    'VND002',
    'PT Mitra ATM Service',
    'support@mitraatm.co.id',
    '+62 21 555 1002',
    'Bandung',
    TRUE
  )
ON CONFLICT (code) DO UPDATE
SET
  name = EXCLUDED.name,
  contact_email = EXCLUDED.contact_email,
  contact_phone = EXCLUDED.contact_phone,
  address = EXCLUDED.address,
  is_active = EXCLUDED.is_active,
  updated_at = now();

-- =========================================================
-- 3) SAMPLE USERS (5 DATA)
-- Rules respected:
-- - internal user  => is_karyawan=true, auth_source='ldap', vendor_id=null
-- - vendor user    => is_karyawan=false, auth_source='local', vendor_id not null
-- =========================================================

-- 1. Internal admin
INSERT INTO users (
  role_id,
  employee_id,
  username,
  full_name,
  email,
  is_karyawan,
  auth_source,
  password_hash,
  vendor_id,
  is_active
)
VALUES (
  (SELECT id FROM roles WHERE role = 'ADMIN'),
  'EMP001',
  'john.admin',
  'John Admin',
  'john.admin@crown.local',
  TRUE,
  'ldap',
  NULL,
  NULL,
  TRUE
)
ON CONFLICT (username) DO UPDATE
SET
  role_id = EXCLUDED.role_id,
  employee_id = EXCLUDED.employee_id,
  full_name = EXCLUDED.full_name,
  email = EXCLUDED.email,
  is_karyawan = EXCLUDED.is_karyawan,
  auth_source = EXCLUDED.auth_source,
  password_hash = EXCLUDED.password_hash,
  vendor_id = EXCLUDED.vendor_id,
  is_active = EXCLUDED.is_active,
  updated_at = now();

-- 2. Internal parameter admin
INSERT INTO users (
  role_id,
  employee_id,
  username,
  full_name,
  email,
  is_karyawan,
  auth_source,
  password_hash,
  vendor_id,
  is_active
)
VALUES (
  (SELECT id FROM roles WHERE role = 'ADMIN_PARAM'),
  'EMP002',
  'siti.param',
  'Siti Param',
  'siti.param@crown.local',
  TRUE,
  'ldap',
  NULL,
  NULL,
  TRUE
)
ON CONFLICT (username) DO UPDATE
SET
  role_id = EXCLUDED.role_id,
  employee_id = EXCLUDED.employee_id,
  full_name = EXCLUDED.full_name,
  email = EXCLUDED.email,
  is_karyawan = EXCLUDED.is_karyawan,
  auth_source = EXCLUDED.auth_source,
  password_hash = EXCLUDED.password_hash,
  vendor_id = EXCLUDED.vendor_id,
  is_active = EXCLUDED.is_active,
  updated_at = now();

-- 3. Internal ATM user
INSERT INTO users (
  role_id,
  employee_id,
  username,
  full_name,
  email,
  is_karyawan,
  auth_source,
  password_hash,
  vendor_id,
  is_active
)
VALUES (
  (SELECT id FROM roles WHERE role = 'ATM-USER'),
  'EMP003',
  'budi.atm',
  'Budi ATM',
  'budi.atm@crown.local',
  TRUE,
  'ldap',
  NULL,
  NULL,
  TRUE
)
ON CONFLICT (username) DO UPDATE
SET
  role_id = EXCLUDED.role_id,
  employee_id = EXCLUDED.employee_id,
  full_name = EXCLUDED.full_name,
  email = EXCLUDED.email,
  is_karyawan = EXCLUDED.is_karyawan,
  auth_source = EXCLUDED.auth_source,
  password_hash = EXCLUDED.password_hash,
  vendor_id = EXCLUDED.vendor_id,
  is_active = EXCLUDED.is_active,
  updated_at = now();

-- 4. Internal branch user
INSERT INTO users (
  role_id,
  employee_id,
  username,
  full_name,
  email,
  is_karyawan,
  auth_source,
  password_hash,
  vendor_id,
  is_active
)
VALUES (
  (SELECT id FROM roles WHERE role = 'BRANCH-USER'),
  'EMP004',
  'rina.branch',
  'Rina Branch',
  'rina.branch@crown.local',
  TRUE,
  'ldap',
  NULL,
  NULL,
  TRUE
)
ON CONFLICT (username) DO UPDATE
SET
  role_id = EXCLUDED.role_id,
  employee_id = EXCLUDED.employee_id,
  full_name = EXCLUDED.full_name,
  email = EXCLUDED.email,
  is_karyawan = EXCLUDED.is_karyawan,
  auth_source = EXCLUDED.auth_source,
  password_hash = EXCLUDED.password_hash,
  vendor_id = EXCLUDED.vendor_id,
  is_active = EXCLUDED.is_active,
  updated_at = now();

-- 5. Vendor user
INSERT INTO users (
  role_id,
  employee_id,
  username,
  full_name,
  email,
  is_karyawan,
  auth_source,
  password_hash,
  vendor_id,
  is_active
)
VALUES (
  (SELECT id FROM roles WHERE role = 'VENDOR-USER'),
  NULL,
  'andi.vendor',
  'Andi Vendor',
  'andi.vendor@crownvendor.co.id',
  FALSE,
  'local',
  'seed-local-password-hash-001',
  (SELECT id FROM vendors WHERE code = 'VND001'),
  TRUE
)
ON CONFLICT (username) DO UPDATE
SET
  role_id = EXCLUDED.role_id,
  employee_id = EXCLUDED.employee_id,
  full_name = EXCLUDED.full_name,
  email = EXCLUDED.email,
  is_karyawan = EXCLUDED.is_karyawan,
  auth_source = EXCLUDED.auth_source,
  password_hash = EXCLUDED.password_hash,
  vendor_id = EXCLUDED.vendor_id,
  is_active = EXCLUDED.is_active,
  updated_at = now();

COMMIT;