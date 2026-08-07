-- =========================================================
-- Verification Test Script for Migration 003_add_local_dev_auth
-- Run AFTER applying migrations 001, 002, and 003 (up).
--
-- Usage:
--   psql -d <your_db> -f 003_add_local_dev_auth_test.sql
--
-- Expected output: All tests should print PASS via RAISE NOTICE.
-- =========================================================

DO $$
DECLARE
    v_role_id BIGINT;
    v_vendor_id BIGINT;
    v_user_count INT;
    v_test_name TEXT;
BEGIN
    -- Setup: get a valid role_id and vendor_id for test inserts
    SELECT id INTO v_role_id FROM roles WHERE role = 'ADMIN' LIMIT 1;
    SELECT id INTO v_vendor_id FROM vendors WHERE code = 'VND001' LIMIT 1;

    IF v_role_id IS NULL THEN
        RAISE EXCEPTION 'Setup failed: no ADMIN role found. Ensure migration 002 has been applied.';
    END IF;
    IF v_vendor_id IS NULL THEN
        RAISE EXCEPTION 'Setup failed: no VND001 vendor found. Ensure migration 002 has been applied.';
    END IF;

    RAISE NOTICE '=========================================================';
    RAISE NOTICE 'Migration 003 Constraint Verification Tests';
    RAISE NOTICE '=========================================================';
    RAISE NOTICE '';

    -- =========================================================
    -- SECTION 1: Verify existing data was updated correctly
    -- =========================================================
    RAISE NOTICE '--- Section 1: Verify seed data migration ---';

    -- Check all internal users now have auth_source='local_dev'
    SELECT COUNT(*) INTO v_user_count
    FROM users
    WHERE is_karyawan = TRUE AND auth_source != 'local_dev';

    IF v_user_count = 0 THEN
        RAISE NOTICE 'PASS: All internal users have auth_source=local_dev';
    ELSE
        RAISE NOTICE 'FAIL: % internal user(s) still do NOT have auth_source=local_dev', v_user_count;
    END IF;

    -- Check all internal users have password_hash set
    SELECT COUNT(*) INTO v_user_count
    FROM users
    WHERE is_karyawan = TRUE AND password_hash IS NULL;

    IF v_user_count = 0 THEN
        RAISE NOTICE 'PASS: All internal users have password_hash set';
    ELSE
        RAISE NOTICE 'FAIL: % internal user(s) have NULL password_hash', v_user_count;
    END IF;

    -- Check vendor user still has auth_source='local'
    SELECT COUNT(*) INTO v_user_count
    FROM users
    WHERE is_karyawan = FALSE AND auth_source = 'local' AND vendor_id IS NOT NULL AND password_hash IS NOT NULL;

    IF v_user_count > 0 THEN
        RAISE NOTICE 'PASS: Vendor user(s) still have auth_source=local with password_hash and vendor_id';
    ELSE
        RAISE NOTICE 'FAIL: No vendor users found with expected auth_source=local configuration';
    END IF;

    RAISE NOTICE '';

    -- =========================================================
    -- SECTION 2: VALID combinations (should succeed)
    -- =========================================================
    RAISE NOTICE '--- Section 2: Valid combinations (should INSERT successfully) ---';

    -- Test 2.1: is_karyawan=TRUE, auth_source='local_dev', password_hash NOT NULL, vendor_id=NULL
    v_test_name := '2.1 karyawan + local_dev + password + no vendor';
    BEGIN
        INSERT INTO users (role_id, username, full_name, email, is_karyawan, auth_source, password_hash, vendor_id)
        VALUES (v_role_id, 'test_valid_2_1', 'Test Valid 2.1', 'test_valid_2_1@test.local',
                TRUE, 'local_dev', '$2a$12$testhashabcdefghijklmnopqrstuvwxyz012345678', NULL);
        RAISE NOTICE 'PASS: % → INSERT succeeded', v_test_name;
        -- Cleanup
        DELETE FROM users WHERE username = 'test_valid_2_1';
    EXCEPTION WHEN OTHERS THEN
        RAISE NOTICE 'FAIL: % → INSERT rejected: %', v_test_name, SQLERRM;
    END;

    -- Test 2.2: is_karyawan=FALSE, auth_source='local', password_hash NOT NULL, vendor_id NOT NULL
    v_test_name := '2.2 non-karyawan + local + password + vendor';
    BEGIN
        INSERT INTO users (role_id, username, full_name, email, is_karyawan, auth_source, password_hash, vendor_id)
        VALUES (v_role_id, 'test_valid_2_2', 'Test Valid 2.2', 'test_valid_2_2@test.local',
                FALSE, 'local', '$2a$12$testhashabcdefghijklmnopqrstuvwxyz012345678', v_vendor_id);
        RAISE NOTICE 'PASS: % → INSERT succeeded', v_test_name;
        -- Cleanup
        DELETE FROM users WHERE username = 'test_valid_2_2';
    EXCEPTION WHEN OTHERS THEN
        RAISE NOTICE 'FAIL: % → INSERT rejected: %', v_test_name, SQLERRM;
    END;

    -- Test 2.3: is_karyawan=TRUE, auth_source='ldap', password_hash=NULL, vendor_id=NULL
    v_test_name := '2.3 karyawan + ldap + no password + no vendor';
    BEGIN
        INSERT INTO users (role_id, username, full_name, email, is_karyawan, auth_source, password_hash, vendor_id)
        VALUES (v_role_id, 'test_valid_2_3', 'Test Valid 2.3', 'test_valid_2_3@test.local',
                TRUE, 'ldap', NULL, NULL);
        RAISE NOTICE 'PASS: % → INSERT succeeded', v_test_name;
        -- Cleanup
        DELETE FROM users WHERE username = 'test_valid_2_3';
    EXCEPTION WHEN OTHERS THEN
        RAISE NOTICE 'FAIL: % → INSERT rejected: %', v_test_name, SQLERRM;
    END;

    RAISE NOTICE '';

    -- =========================================================
    -- SECTION 3: INVALID combinations (should be REJECTED)
    -- =========================================================
    RAISE NOTICE '--- Section 3: Invalid combinations (should be REJECTED by constraints) ---';

    -- Test 3.1: is_karyawan=TRUE, auth_source='local_dev', password_hash=NULL, vendor_id=NULL
    -- Should FAIL: password_hash required for local_dev (users_password_hash_chk)
    v_test_name := '3.1 karyawan + local_dev + NO password → should FAIL (password_hash_chk)';
    BEGIN
        INSERT INTO users (role_id, username, full_name, email, is_karyawan, auth_source, password_hash, vendor_id)
        VALUES (v_role_id, 'test_invalid_3_1', 'Test Invalid 3.1', 'test_invalid_3_1@test.local',
                TRUE, 'local_dev', NULL, NULL);
        -- If we get here, the constraint did NOT reject it
        RAISE NOTICE 'FAIL: % → INSERT succeeded (should have been rejected)', v_test_name;
        DELETE FROM users WHERE username = 'test_invalid_3_1';
    EXCEPTION WHEN check_violation THEN
        RAISE NOTICE 'PASS: % → correctly rejected: %', v_test_name, SQLERRM;
    WHEN OTHERS THEN
        RAISE NOTICE 'FAIL: % → unexpected error: %', v_test_name, SQLERRM;
    END;

    -- Test 3.2: is_karyawan=TRUE, auth_source='local', password_hash NOT NULL, vendor_id=NULL
    -- Should FAIL: vendor_logic_chk (karyawan can only be ldap or local_dev, not 'local')
    v_test_name := '3.2 karyawan + local + password + no vendor → should FAIL (vendor_logic_chk)';
    BEGIN
        INSERT INTO users (role_id, username, full_name, email, is_karyawan, auth_source, password_hash, vendor_id)
        VALUES (v_role_id, 'test_invalid_3_2', 'Test Invalid 3.2', 'test_invalid_3_2@test.local',
                TRUE, 'local', '$2a$12$testhashabcdefghijklmnopqrstuvwxyz012345678', NULL);
        RAISE NOTICE 'FAIL: % → INSERT succeeded (should have been rejected)', v_test_name;
        DELETE FROM users WHERE username = 'test_invalid_3_2';
    EXCEPTION WHEN check_violation THEN
        RAISE NOTICE 'PASS: % → correctly rejected: %', v_test_name, SQLERRM;
    WHEN OTHERS THEN
        RAISE NOTICE 'FAIL: % → unexpected error: %', v_test_name, SQLERRM;
    END;

    -- Test 3.3: is_karyawan=FALSE, auth_source='local_dev', password_hash NOT NULL, vendor_id NOT NULL
    -- Should FAIL: vendor_logic_chk (non-karyawan must use 'local', not 'local_dev')
    v_test_name := '3.3 non-karyawan + local_dev + password + vendor → should FAIL (vendor_logic_chk)';
    BEGIN
        INSERT INTO users (role_id, username, full_name, email, is_karyawan, auth_source, password_hash, vendor_id)
        VALUES (v_role_id, 'test_invalid_3_3', 'Test Invalid 3.3', 'test_invalid_3_3@test.local',
                FALSE, 'local_dev', '$2a$12$testhashabcdefghijklmnopqrstuvwxyz012345678', v_vendor_id);
        RAISE NOTICE 'FAIL: % → INSERT succeeded (should have been rejected)', v_test_name;
        DELETE FROM users WHERE username = 'test_invalid_3_3';
    EXCEPTION WHEN check_violation THEN
        RAISE NOTICE 'PASS: % → correctly rejected: %', v_test_name, SQLERRM;
    WHEN OTHERS THEN
        RAISE NOTICE 'FAIL: % → unexpected error: %', v_test_name, SQLERRM;
    END;

    -- Test 3.4: is_karyawan=TRUE, auth_source='ldap', password_hash NOT NULL, vendor_id=NULL
    -- Should FAIL: password_hash_chk (ldap must have NULL password_hash)
    v_test_name := '3.4 karyawan + ldap + password NOT NULL → should FAIL (password_hash_chk)';
    BEGIN
        INSERT INTO users (role_id, username, full_name, email, is_karyawan, auth_source, password_hash, vendor_id)
        VALUES (v_role_id, 'test_invalid_3_4', 'Test Invalid 3.4', 'test_invalid_3_4@test.local',
                TRUE, 'ldap', '$2a$12$testhashabcdefghijklmnopqrstuvwxyz012345678', NULL);
        RAISE NOTICE 'FAIL: % → INSERT succeeded (should have been rejected)', v_test_name;
        DELETE FROM users WHERE username = 'test_invalid_3_4';
    EXCEPTION WHEN check_violation THEN
        RAISE NOTICE 'PASS: % → correctly rejected: %', v_test_name, SQLERRM;
    WHEN OTHERS THEN
        RAISE NOTICE 'FAIL: % → unexpected error: %', v_test_name, SQLERRM;
    END;

    RAISE NOTICE '';
    RAISE NOTICE '=========================================================';
    RAISE NOTICE 'Verification complete.';
    RAISE NOTICE '=========================================================';

END $$;
