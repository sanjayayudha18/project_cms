-- =============================================================================
-- CMS ATM & CIT — Initial Database Schema
-- PostgreSQL 16 | Extensions: pgcrypto, pg_trgm
-- Follows: PROJECT_CONTEXT.md canonical table map (Sec 3)
-- Conventions:
--   - bigint GENERATED ALWAYS AS IDENTITY for PKs
--   - numeric for all monetary amounts (IDR)
--   - timestamptz for all timestamps (stored UTC)
--   - text over varchar
--   - lowercase_snake_case identifiers
--   - All FKs indexed
-- =============================================================================

CREATE EXTENSION IF NOT EXISTS pgcrypto;
CREATE EXTENSION IF NOT EXISTS pg_trgm;

-- =============================================================================
-- AUTH GROUP
-- =============================================================================

CREATE TABLE roles (
    id          bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    name        text NOT NULL UNIQUE,
    description text,
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE users (
    id              bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    employee_id     text UNIQUE,
    username        text NOT NULL UNIQUE,
    full_name       text NOT NULL,
    email           text NOT NULL,
    auth_source     text NOT NULL CHECK (auth_source IN ('ldap', 'local')),
    password_hash   text,
    role_id         bigint NOT NULL REFERENCES roles(id),
    vendor_id       bigint,
    is_active       boolean NOT NULL DEFAULT true,
    last_login_at   timestamptz,
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now(),
    deleted_at      timestamptz
);

CREATE INDEX idx_users_role_id ON users(role_id);
CREATE INDEX idx_users_auth_source ON users(auth_source);
CREATE INDEX idx_users_vendor_id ON users(vendor_id) WHERE vendor_id IS NOT NULL;
CREATE INDEX idx_users_active ON users(is_active) WHERE deleted_at IS NULL;
CREATE INDEX idx_users_email_trgm ON users USING gin(email gin_trgm_ops);

-- =============================================================================
-- MASTER DATA GROUP
-- =============================================================================

CREATE TABLE vendors (
    id              bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    code            text NOT NULL UNIQUE,
    name            text NOT NULL,
    contact_email   text,
    contact_phone   text,
    address         text,
    is_active       boolean NOT NULL DEFAULT true,
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now(),
    deleted_at      timestamptz
);

CREATE INDEX idx_vendors_active ON vendors(is_active) WHERE deleted_at IS NULL;

-- FK: users.vendor_id -> vendors.id
ALTER TABLE users
    ADD CONSTRAINT fk_users_vendor
    FOREIGN KEY (vendor_id) REFERENCES vendors(id);

CREATE TABLE vendor_pics (
    id          bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    vendor_id   bigint NOT NULL REFERENCES vendors(id),
    full_name   text NOT NULL,
    email       text,
    phone       text,
    position    text,
    is_primary  boolean NOT NULL DEFAULT false,
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX idx_vendor_pics_vendor_id ON vendor_pics(vendor_id);

CREATE TABLE locations (
    id          bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    code        text NOT NULL UNIQUE,
    name        text NOT NULL,
    region      text,
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE vendor_vaults (
    id              bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    vendor_id       bigint NOT NULL REFERENCES vendors(id),
    location_id     bigint NOT NULL REFERENCES locations(id),
    vault_code      text NOT NULL UNIQUE,
    address         text,
    capacity        numeric,
    is_active       boolean NOT NULL DEFAULT true,
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX idx_vendor_vaults_vendor_id ON vendor_vaults(vendor_id);
CREATE INDEX idx_vendor_vaults_location_id ON vendor_vaults(location_id);

CREATE TABLE atms (
    id                  bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    terminal_id         text NOT NULL UNIQUE,
    location_name       text NOT NULL,
    address             text,
    location_id         bigint NOT NULL REFERENCES locations(id),
    vendor_id           bigint NOT NULL REFERENCES vendors(id),
    vault_id            bigint REFERENCES vendor_vaults(id),
    machine_type        text DEFAULT 'ATM' CHECK (machine_type IN ('ATM', 'CRM')),
    capacity            numeric,
    low_threshold       numeric,
    critical_threshold  numeric,
    is_active           boolean NOT NULL DEFAULT true,
    created_at          timestamptz NOT NULL DEFAULT now(),
    updated_at          timestamptz NOT NULL DEFAULT now(),
    deleted_at          timestamptz
);

CREATE INDEX idx_atms_location_id ON atms(location_id);
CREATE INDEX idx_atms_vendor_id ON atms(vendor_id);
CREATE INDEX idx_atms_vault_id ON atms(vault_id) WHERE vault_id IS NOT NULL;
CREATE INDEX idx_atms_terminal_trgm ON atms USING gin(terminal_id gin_trgm_ops);
CREATE INDEX idx_atms_active ON atms(is_active) WHERE deleted_at IS NULL;

CREATE TABLE vendor_assignments (
    id              bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    vendor_id       bigint NOT NULL REFERENCES vendors(id),
    atm_id          bigint NOT NULL REFERENCES atms(id),
    assigned_at     timestamptz NOT NULL DEFAULT now(),
    unassigned_at   timestamptz,
    created_at      timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX idx_vendor_assignments_vendor_id ON vendor_assignments(vendor_id);
CREATE INDEX idx_vendor_assignments_atm_id ON vendor_assignments(atm_id);
CREATE UNIQUE INDEX idx_vendor_assignments_active
    ON vendor_assignments(atm_id) WHERE unassigned_at IS NULL;

-- =============================================================================
-- CORE GROUP (audit, approvals, documents, notifications, import/export)
-- =============================================================================

CREATE TABLE audit_logs (
    id              bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    user_id         bigint REFERENCES users(id),
    action          text NOT NULL,
    entity_type     text NOT NULL,
    entity_id       bigint,
    before_state    jsonb,
    after_state     jsonb,
    ip_address      inet,
    user_agent      text,
    metadata        jsonb,
    created_at      timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX idx_audit_logs_user_id ON audit_logs(user_id);
CREATE INDEX idx_audit_logs_entity ON audit_logs(entity_type, entity_id);
CREATE INDEX idx_audit_logs_created_at ON audit_logs(created_at DESC);

CREATE TABLE approval_requests (
    id              bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    entity_type     text NOT NULL,
    entity_id       bigint NOT NULL,
    action          text NOT NULL,
    status          text NOT NULL DEFAULT 'pending'
                    CHECK (status IN ('pending', 'approved', 'rejected')),
    maker_id        bigint NOT NULL REFERENCES users(id),
    checker_id      bigint REFERENCES users(id),
    request_data    jsonb NOT NULL,
    rejection_reason text,
    requested_at    timestamptz NOT NULL DEFAULT now(),
    decided_at      timestamptz,
    CONSTRAINT chk_maker_ne_checker
        CHECK (maker_id != checker_id OR checker_id IS NULL)
);

CREATE INDEX idx_approval_pending ON approval_requests(status)
    WHERE status = 'pending';
CREATE INDEX idx_approval_entity ON approval_requests(entity_type, entity_id);
CREATE INDEX idx_approval_maker ON approval_requests(maker_id);
CREATE INDEX idx_approval_checker ON approval_requests(checker_id)
    WHERE checker_id IS NOT NULL;

CREATE TABLE documents (
    id              bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    entity_type     text NOT NULL,
    entity_id       bigint NOT NULL,
    file_name       text NOT NULL,
    file_path       text NOT NULL,
    file_size       bigint,
    mime_type       text,
    uploaded_by     bigint NOT NULL REFERENCES users(id),
    created_at      timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX idx_documents_entity ON documents(entity_type, entity_id);
CREATE INDEX idx_documents_uploaded_by ON documents(uploaded_by);

CREATE TABLE notifications (
    id              bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    user_id         bigint NOT NULL REFERENCES users(id),
    title           text NOT NULL,
    body            text,
    category        text NOT NULL,
    entity_type     text,
    entity_id       bigint,
    is_read         boolean NOT NULL DEFAULT false,
    read_at         timestamptz,
    channel         text NOT NULL DEFAULT 'in_app'
                    CHECK (channel IN ('in_app', 'email', 'both')),
    sent_at         timestamptz,
    created_at      timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX idx_notifications_user_unread ON notifications(user_id)
    WHERE is_read = false;
CREATE INDEX idx_notifications_user_created ON notifications(user_id, created_at DESC);

CREATE TABLE import_jobs (
    id              bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    job_type        text NOT NULL,
    file_name       text NOT NULL,
    file_hash       text NOT NULL,
    file_path       text,
    status          text NOT NULL DEFAULT 'processing'
                    CHECK (status IN ('processing', 'success', 'failed', 'duplicate')),
    rows_processed  integer DEFAULT 0,
    rows_failed     integer DEFAULT 0,
    error_details   jsonb,
    uploaded_by     bigint NOT NULL REFERENCES users(id),
    started_at      timestamptz NOT NULL DEFAULT now(),
    finished_at     timestamptz,
    created_at      timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX idx_import_jobs_idempotent
    ON import_jobs(job_type, file_hash) WHERE status != 'failed';
CREATE INDEX idx_import_jobs_uploaded_by ON import_jobs(uploaded_by);

CREATE TABLE export_jobs (
    id              bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    job_type        text NOT NULL,
    format          text NOT NULL DEFAULT 'xlsx'
                    CHECK (format IN ('xlsx', 'csv', 'pdf')),
    status          text NOT NULL DEFAULT 'processing'
                    CHECK (status IN ('processing', 'success', 'failed')),
    file_path       text,
    parameters      jsonb,
    requested_by    bigint NOT NULL REFERENCES users(id),
    started_at      timestamptz NOT NULL DEFAULT now(),
    finished_at     timestamptz,
    expires_at      timestamptz,
    created_at      timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX idx_export_jobs_requested_by ON export_jobs(requested_by);
CREATE INDEX idx_export_jobs_processing ON export_jobs(status)
    WHERE status = 'processing';

-- =============================================================================
-- ATM OPERATIONS GROUP (DSR, replenishment, forecast)
-- =============================================================================

CREATE TABLE atm_dsr_uploads (
    id              bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    import_job_id   bigint NOT NULL REFERENCES import_jobs(id),
    report_date     date NOT NULL,
    vendor_id       bigint NOT NULL REFERENCES vendors(id),
    total_rows      integer NOT NULL DEFAULT 0,
    created_at      timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX idx_atm_dsr_uploads_date ON atm_dsr_uploads(report_date);
CREATE INDEX idx_atm_dsr_uploads_vendor ON atm_dsr_uploads(vendor_id);
CREATE INDEX idx_atm_dsr_uploads_import ON atm_dsr_uploads(import_job_id);

CREATE TABLE atm_dsr_rows (
    id                  bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    upload_id           bigint NOT NULL REFERENCES atm_dsr_uploads(id),
    atm_id              bigint NOT NULL REFERENCES atms(id),
    report_date         date NOT NULL,
    beginning_balance   numeric NOT NULL,
    cash_in             numeric NOT NULL DEFAULT 0,
    cash_out            numeric NOT NULL DEFAULT 0,
    ending_balance      numeric NOT NULL,
    status              text NOT NULL DEFAULT 'normal'
                        CHECK (status IN ('normal', 'low', 'critical')),
    created_at          timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX idx_dsr_rows_upload ON atm_dsr_rows(upload_id);
CREATE INDEX idx_dsr_rows_atm_date ON atm_dsr_rows(atm_id, report_date DESC);
CREATE UNIQUE INDEX idx_dsr_rows_unique ON atm_dsr_rows(atm_id, report_date);

CREATE TABLE forecast_runs (
    id                  bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    processing_date     date NOT NULL,
    status              text NOT NULL DEFAULT 'running'
                        CHECK (status IN ('running', 'success', 'failed')),
    horizon_days        integer NOT NULL DEFAULT 2,
    records_processed   integer DEFAULT 0,
    error_details       text,
    started_at          timestamptz NOT NULL DEFAULT now(),
    finished_at         timestamptz,
    created_at          timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX idx_forecast_runs_date_success
    ON forecast_runs(processing_date) WHERE status = 'success';

CREATE TABLE forecast_results (
    id                          bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    run_id                      bigint NOT NULL REFERENCES forecast_runs(id),
    atm_id                      bigint NOT NULL REFERENCES atms(id),
    processing_date             date NOT NULL,
    current_balance             numeric NOT NULL,
    predicted_usage_h1          numeric NOT NULL DEFAULT 0,
    predicted_usage_h2          numeric NOT NULL DEFAULT 0,
    recommended_replenishment   numeric NOT NULL DEFAULT 0,
    priority                    text NOT NULL DEFAULT 'low'
                                CHECK (priority IN ('low', 'medium', 'high')),
    created_at                  timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX idx_forecast_results_run ON forecast_results(run_id);
CREATE INDEX idx_forecast_results_atm ON forecast_results(atm_id, processing_date DESC);
CREATE UNIQUE INDEX idx_forecast_results_unique
    ON forecast_results(run_id, atm_id);

CREATE TABLE replenishment_instructions (
    id                  bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    forecast_result_id  bigint REFERENCES forecast_results(id),
    atm_id              bigint NOT NULL REFERENCES atms(id),
    vendor_id           bigint NOT NULL REFERENCES vendors(id),
    scheduled_date      date NOT NULL,
    amount              numeric NOT NULL,
    route_code          text,
    window_start        time,
    window_end          time,
    status              text NOT NULL DEFAULT 'scheduled'
                        CHECK (status IN ('scheduled', 'pending_vendor',
                            'in_transit', 'completed', 'delayed', 'cancelled')),
    origin              text NOT NULL DEFAULT 'auto'
                        CHECK (origin IN ('auto', 'manual')),
    completed_at        timestamptz,
    notes               text,
    created_by          bigint NOT NULL REFERENCES users(id),
    created_at          timestamptz NOT NULL DEFAULT now(),
    updated_at          timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX idx_replenishment_atm ON replenishment_instructions(atm_id);
CREATE INDEX idx_replenishment_vendor ON replenishment_instructions(vendor_id);
CREATE INDEX idx_replenishment_date ON replenishment_instructions(scheduled_date);
CREATE INDEX idx_replenishment_status ON replenishment_instructions(status)
    WHERE status NOT IN ('completed', 'cancelled');

-- =============================================================================
-- CIT GROUP
-- =============================================================================

CREATE TABLE cit_orders (
    id              bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    order_code      text NOT NULL UNIQUE,
    atm_id          bigint NOT NULL REFERENCES atms(id),
    vendor_id       bigint NOT NULL REFERENCES vendors(id),
    order_date      date NOT NULL,
    scheduled_date  date NOT NULL,
    amount          numeric NOT NULL,
    status          text NOT NULL DEFAULT 'scheduled'
                    CHECK (status IN ('scheduled', 'in_transit', 'completed', 'failed', 'cancelled')),
    origin          text NOT NULL DEFAULT 'auto'
                    CHECK (origin IN ('auto', 'manual')),
    failure_reason  text,
    completed_at    timestamptz,
    created_by      bigint NOT NULL REFERENCES users(id),
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX idx_cit_orders_atm ON cit_orders(atm_id);
CREATE INDEX idx_cit_orders_vendor ON cit_orders(vendor_id);
CREATE INDEX idx_cit_orders_scheduled ON cit_orders(scheduled_date);
CREATE INDEX idx_cit_orders_status ON cit_orders(status)
    WHERE status NOT IN ('completed', 'failed', 'cancelled');

CREATE TABLE cit_handover_evidences (
    id              bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    cit_order_id    bigint NOT NULL REFERENCES cit_orders(id),
    evidence_type   text NOT NULL DEFAULT 'photo'
                    CHECK (evidence_type IN ('photo', 'pdf', 'signature')),
    file_path       text NOT NULL,
    file_name       text,
    uploaded_by     bigint NOT NULL REFERENCES users(id),
    notes           text,
    created_at      timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX idx_cit_evidence_order ON cit_handover_evidences(cit_order_id);

CREATE TABLE cit_journals (
    id              bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    cit_order_id    bigint NOT NULL REFERENCES cit_orders(id),
    atm_id          bigint NOT NULL REFERENCES atms(id),
    journal_date    date NOT NULL,
    amount_loaded   numeric NOT NULL,
    amount_retrieved numeric NOT NULL DEFAULT 0,
    net_change      numeric GENERATED ALWAYS AS (amount_loaded - amount_retrieved) STORED,
    operator_name   text,
    notes           text,
    created_at      timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX idx_cit_journals_order ON cit_journals(cit_order_id);
CREATE INDEX idx_cit_journals_atm_date ON cit_journals(atm_id, journal_date DESC);

CREATE TABLE cit_dsr_uploads (
    id              bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    import_job_id   bigint NOT NULL REFERENCES import_jobs(id),
    report_date     date NOT NULL,
    vendor_id       bigint NOT NULL REFERENCES vendors(id),
    total_rows      integer NOT NULL DEFAULT 0,
    created_at      timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX idx_cit_dsr_uploads_date ON cit_dsr_uploads(report_date);
CREATE INDEX idx_cit_dsr_uploads_vendor ON cit_dsr_uploads(vendor_id);

CREATE TABLE cit_reconciliation_results (
    id              bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    cit_order_id    bigint NOT NULL REFERENCES cit_orders(id),
    dsr_row_id      bigint REFERENCES atm_dsr_rows(id),
    journal_id      bigint REFERENCES cit_journals(id),
    order_amount    numeric NOT NULL,
    dsr_amount      numeric,
    journal_amount  numeric,
    variance        numeric NOT NULL DEFAULT 0,
    status          text NOT NULL DEFAULT 'open'
                    CHECK (status IN ('open', 'matched', 'exception', 'resolved')),
    resolved_by     bigint REFERENCES users(id),
    resolved_at     timestamptz,
    resolution_note text,
    created_at      timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX idx_cit_recon_order ON cit_reconciliation_results(cit_order_id);
CREATE INDEX idx_cit_recon_status ON cit_reconciliation_results(status)
    WHERE status IN ('open', 'exception');

-- =============================================================================
-- FINANCE GROUP (invoices)
-- =============================================================================

CREATE TABLE invoice_uploads (
    id                  bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    import_job_id       bigint REFERENCES import_jobs(id),
    vendor_id           bigint NOT NULL REFERENCES vendors(id),
    invoice_number      text NOT NULL UNIQUE,
    period_start        date NOT NULL,
    period_end          date NOT NULL,
    total_amount        numeric NOT NULL,
    line_items_count    integer NOT NULL DEFAULT 0,
    validation_status   text NOT NULL DEFAULT 'uploaded'
                        CHECK (validation_status IN ('uploaded', 'validated',
                            'mismatch_detected', 'approved', 'rejected')),
    validator_id        bigint REFERENCES users(id),
    validated_at        timestamptz,
    approver_id         bigint REFERENCES users(id),
    approved_at         timestamptz,
    rejection_reason    text,
    created_at          timestamptz NOT NULL DEFAULT now(),
    updated_at          timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX idx_invoice_uploads_vendor ON invoice_uploads(vendor_id);
CREATE INDEX idx_invoice_uploads_status ON invoice_uploads(validation_status)
    WHERE validation_status NOT IN ('approved', 'rejected');
CREATE INDEX idx_invoice_uploads_period ON invoice_uploads(period_start, period_end);

CREATE TABLE invoice_items (
    id                  bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    invoice_id          bigint NOT NULL REFERENCES invoice_uploads(id) ON DELETE CASCADE,
    description         text NOT NULL,
    invoiced_amount     numeric NOT NULL,
    matched_order_id    bigint REFERENCES cit_orders(id),
    expected_amount     numeric,
    variance            numeric NOT NULL DEFAULT 0,
    match_status        text NOT NULL DEFAULT 'pending_review'
                        CHECK (match_status IN ('pending_review', 'matched',
                            'mismatch', 'unmatched')),
    created_at          timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX idx_invoice_items_invoice ON invoice_items(invoice_id);
CREATE INDEX idx_invoice_items_order ON invoice_items(matched_order_id)
    WHERE matched_order_id IS NOT NULL;

CREATE TABLE invoice_reconciliation_results (
    id              bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    invoice_id      bigint NOT NULL REFERENCES invoice_uploads(id),
    total_invoiced  numeric NOT NULL,
    total_expected  numeric NOT NULL,
    total_variance  numeric NOT NULL DEFAULT 0,
    matched_count   integer NOT NULL DEFAULT 0,
    mismatch_count  integer NOT NULL DEFAULT 0,
    unmatched_count integer NOT NULL DEFAULT 0,
    reconciled_at   timestamptz NOT NULL DEFAULT now(),
    reconciled_by   bigint REFERENCES users(id),
    created_at      timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX idx_invoice_recon_invoice ON invoice_reconciliation_results(invoice_id);

-- =============================================================================
-- INTEGRATION GROUP (escrow / corebanking)
-- =============================================================================

CREATE TABLE escrow_batch_files (
    id              bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    import_job_id   bigint NOT NULL REFERENCES import_jobs(id),
    file_date       date NOT NULL,
    file_hash       text NOT NULL UNIQUE,
    total_rows      integer NOT NULL DEFAULT 0,
    total_amount    numeric NOT NULL DEFAULT 0,
    status          text NOT NULL DEFAULT 'processing'
                    CHECK (status IN ('processing', 'processed', 'failed')),
    created_at      timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX idx_escrow_batch_date ON escrow_batch_files(file_date);

CREATE TABLE escrow_batch_rows (
    id              bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    batch_file_id   bigint NOT NULL REFERENCES escrow_batch_files(id),
    atm_id          bigint REFERENCES atms(id),
    terminal_id_raw text NOT NULL,
    transaction_date date NOT NULL,
    escrow_balance  numeric NOT NULL,
    currency        text NOT NULL DEFAULT 'IDR',
    row_number      integer,
    created_at      timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX idx_escrow_rows_batch ON escrow_batch_rows(batch_file_id);
CREATE INDEX idx_escrow_rows_atm_date ON escrow_batch_rows(atm_id, transaction_date);

CREATE TABLE escrow_reconciliation_results (
    id                  bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    batch_file_id       bigint NOT NULL REFERENCES escrow_batch_files(id),
    atm_id              bigint NOT NULL REFERENCES atms(id),
    reconciliation_date date NOT NULL,
    cms_balance         numeric NOT NULL,
    escrow_balance      numeric NOT NULL,
    difference          numeric NOT NULL DEFAULT 0,
    severity            text NOT NULL DEFAULT 'low'
                        CHECK (severity IN ('low', 'medium', 'high')),
    status              text NOT NULL DEFAULT 'open'
                        CHECK (status IN ('open', 'resolved', 'acknowledged')),
    owner_id            bigint REFERENCES users(id),
    resolved_by         bigint REFERENCES users(id),
    resolved_at         timestamptz,
    resolution_note     text,
    created_at          timestamptz NOT NULL DEFAULT now(),
    updated_at          timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX idx_escrow_recon_batch ON escrow_reconciliation_results(batch_file_id);
CREATE INDEX idx_escrow_recon_atm ON escrow_reconciliation_results(atm_id);
CREATE INDEX idx_escrow_recon_open ON escrow_reconciliation_results(status)
    WHERE status = 'open';
CREATE INDEX idx_escrow_recon_date ON escrow_reconciliation_results(reconciliation_date DESC);

-- =============================================================================
-- EOD BATCH TRACKING
-- =============================================================================

CREATE TABLE eod_runs (
    id                  bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    processing_date     date NOT NULL,
    run_type            text NOT NULL DEFAULT 'full'
                        CHECK (run_type IN ('full', 'partial', 'retry')),
    status              text NOT NULL DEFAULT 'running'
                        CHECK (status IN ('running', 'success', 'failed')),
    steps_completed     jsonb,
    records_processed   integer DEFAULT 0,
    error_details       text,
    started_at          timestamptz NOT NULL DEFAULT now(),
    finished_at         timestamptz,
    created_at          timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX idx_eod_runs_date ON eod_runs(processing_date DESC);
CREATE UNIQUE INDEX idx_eod_runs_date_success
    ON eod_runs(processing_date) WHERE status = 'success';

-- =============================================================================
-- SEED: Default Roles
-- =============================================================================

INSERT INTO roles (name, description) VALUES
    ('admin', 'System administrator — full access, EOD monitoring, user management'),
    ('operator', 'ATM operations — DSR upload, replenishment monitoring, CIT tracking'),
    ('manager', 'Approver — maker-checker approval authority for financial/master data'),
    ('vendor', 'Vendor portal user — scoped to own assignments, upload evidence'),
    ('branch_user', 'Branch/internal user — read-only dashboards, cash monitoring'),
    ('finance', 'Finance team — invoice validation, reconciliation, approval'),
    ('app_support', 'Application support — EOD monitoring, system health, alerts'),
    ('auditor', 'Auditor — read-only access to audit logs and all data');

-- =============================================================================
-- HELPER: updated_at trigger function
-- =============================================================================

CREATE OR REPLACE FUNCTION trigger_set_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = now();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Apply updated_at triggers to relevant tables
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
        EXECUTE format(
            'CREATE TRIGGER set_updated_at BEFORE UPDATE ON %I
             FOR EACH ROW EXECUTE FUNCTION trigger_set_updated_at()',
            tbl
        );
    END LOOP;
END;
$$;
