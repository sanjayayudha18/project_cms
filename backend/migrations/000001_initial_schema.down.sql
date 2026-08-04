-- =============================================================================
-- CMS ATM & CIT — Rollback Initial Schema
-- Drop in reverse dependency order
-- =============================================================================

-- Drop triggers first
DO $$
DECLARE
    tbl text;
BEGIN
    FOR tbl IN
        SELECT unnest(ARRAY[
            'users', 'vendors', 'vendor_vaults', 'atms',
            'replenishment_instructions', 'cit_orders',
            'invoice_uploads', 'escrow_reconciliation_results'
        ])
    LOOP
        EXECUTE format('DROP TRIGGER IF EXISTS set_updated_at ON %I', tbl);
    END LOOP;
END;
$$;

DROP FUNCTION IF EXISTS trigger_set_updated_at();

-- EOD
DROP TABLE IF EXISTS eod_runs;

-- Integration
DROP TABLE IF EXISTS escrow_reconciliation_results;
DROP TABLE IF EXISTS escrow_batch_rows;
DROP TABLE IF EXISTS escrow_batch_files;

-- Finance
DROP TABLE IF EXISTS invoice_reconciliation_results;
DROP TABLE IF EXISTS invoice_items;
DROP TABLE IF EXISTS invoice_uploads;

-- CIT
DROP TABLE IF EXISTS cit_reconciliation_results;
DROP TABLE IF EXISTS cit_dsr_uploads;
DROP TABLE IF EXISTS cit_journals;
DROP TABLE IF EXISTS cit_handover_evidences;
DROP TABLE IF EXISTS cit_orders;

-- ATM Operations
DROP TABLE IF EXISTS replenishment_instructions;
DROP TABLE IF EXISTS forecast_results;
DROP TABLE IF EXISTS forecast_runs;
DROP TABLE IF EXISTS atm_dsr_rows;
DROP TABLE IF EXISTS atm_dsr_uploads;

-- Core
DROP TABLE IF EXISTS export_jobs;
DROP TABLE IF EXISTS import_jobs;
DROP TABLE IF EXISTS notifications;
DROP TABLE IF EXISTS documents;
DROP TABLE IF EXISTS approval_requests;
DROP TABLE IF EXISTS audit_logs;

-- Master Data
DROP TABLE IF EXISTS vendor_assignments;
DROP TABLE IF EXISTS atms;
DROP TABLE IF EXISTS vendor_vaults;
DROP TABLE IF EXISTS locations;
DROP TABLE IF EXISTS vendor_pics;

-- Drop FK before dropping vendors (users.vendor_id)
ALTER TABLE users DROP CONSTRAINT IF EXISTS fk_users_vendor;

DROP TABLE IF EXISTS vendors;

-- Auth
DROP TABLE IF EXISTS users;
DROP TABLE IF EXISTS roles;

-- Extensions (optional — usually kept)
-- DROP EXTENSION IF EXISTS pg_trgm;
-- DROP EXTENSION IF EXISTS pgcrypto;
