-- vendors.legal_name, vendors.npwp: legal identity fields required by
-- URS v0.3 Phase 1 "Master data (vendor)" (vendor legal identity, NPWP).
-- See .claude/sdlc/master-data/plan.md T1.1.
--
-- SAFETY:
--   * Additive, nullable columns, no default, no backfill -- existing
--     vendor rows had no legal_name/NPWP captured, so every existing row
--     reads back NULL until an admin fills it in via maker-checker (T4.1).
--   * npwp CHECK allows NULL (old/incomplete data) or exactly 15 or 16
--     digits (old 15-digit NPWP format vs. the 16-digit format effective
--     2024+); no separators enforced at the DB layer, formatting is a
--     frontend/service concern.
--   * Forward-only: no down migration ships (project convention).

BEGIN;

ALTER TABLE public.vendors
    ADD COLUMN IF NOT EXISTS legal_name text,
    ADD COLUMN IF NOT EXISTS npwp text;

ALTER TABLE public.vendors
    ADD CONSTRAINT vendors_npwp_chk CHECK (npwp IS NULL OR npwp ~ '^[0-9]{15,16}$');

COMMENT ON COLUMN public.vendors.legal_name
    IS 'Registered legal entity name (may differ from vendors.name, the operating/display name). NULL until captured for existing vendors.';

COMMENT ON COLUMN public.vendors.npwp
    IS 'Indonesian tax ID (NPWP), digits only, 15 (pre-2024 format) or 16 (2024+ format) digits. NULL for vendors not yet updated with legal identity.';

COMMIT;

-- ROLLBACK (documented, forward-only shop):
--   ALTER TABLE public.vendors DROP CONSTRAINT IF EXISTS vendors_npwp_chk;
--   ALTER TABLE public.vendors DROP COLUMN IF EXISTS npwp;
--   ALTER TABLE public.vendors DROP COLUMN IF EXISTS legal_name;

-- vendor_branches.deleted_at: soft-delete column, matching the
-- is_active+deleted_at pattern already used on vendors/atms (Golden Rule:
-- disable is soft-delete only, see no_hard_delete_test.go). See
-- .claude/sdlc/master-data/plan.md T1.2.
--
-- SAFETY:
--   * Additive, nullable, no default, no backfill -- every existing row
--     reads back NULL (not deleted), matching its current is_active=true
--     state.
--   * Forward-only: no down migration ships (project convention).

BEGIN;

ALTER TABLE public.vendor_branches
    ADD COLUMN IF NOT EXISTS deleted_at timestamp with time zone;

COMMIT;

-- ROLLBACK (documented, forward-only shop):
--   ALTER TABLE public.vendor_branches DROP COLUMN IF EXISTS deleted_at;

-- vendor_vaults: location_id/latitude/longitude/operating_hours/category/
-- deleted_at -- master-data fields required by URS v0.3 Phase 1 ("vault
-- address, coordinates (optional), operating hours, capacity, category
-- ATM/Cash"). See .claude/sdlc/master-data/plan.md T1.3.
--
-- SAFETY:
--   * location_id/latitude/longitude/operating_hours/deleted_at: additive,
--     nullable, no default -- existing rows read back NULL.
--   * category: NOT NULL with a backfill, not purely additive. T0.1 found
--     all 348 existing vendor_vaults rows carry type='ATM_CASH', which
--     does not map to a binary ATM/CASH split. Product decision
--     (2026-09-18, see Catatan implementasi): backfill every existing row
--     to category='ATM'; new/edited vaults pick ATM or CASH explicitly
--     going forward (T3.2). CHECK stays binary, no 'ATM_CASH' value.
--   * Forward-only: no down migration ships (project convention).

BEGIN;

ALTER TABLE public.vendor_vaults
    ADD COLUMN IF NOT EXISTS location_id bigint,
    ADD COLUMN IF NOT EXISTS latitude numeric(9,6),
    ADD COLUMN IF NOT EXISTS longitude numeric(9,6),
    ADD COLUMN IF NOT EXISTS operating_hours text,
    ADD COLUMN IF NOT EXISTS category text,
    ADD COLUMN IF NOT EXISTS deleted_at timestamp with time zone;

UPDATE public.vendor_vaults
    SET category = 'ATM'
    WHERE category IS NULL;

ALTER TABLE public.vendor_vaults
    ALTER COLUMN category SET NOT NULL,
    ADD CONSTRAINT vendor_vaults_category_chk CHECK (category IN ('ATM', 'CASH')),
    ADD CONSTRAINT vendor_vaults_latitude_chk CHECK (latitude IS NULL OR (latitude BETWEEN -90 AND 90)),
    ADD CONSTRAINT vendor_vaults_longitude_chk CHECK (longitude IS NULL OR (longitude BETWEEN -180 AND 180)),
    ADD CONSTRAINT fk_vendor_vaults_location FOREIGN KEY (location_id) REFERENCES public.locations(id);

COMMENT ON COLUMN public.vendor_vaults.category
    IS 'ATM or CASH. Backfilled to ATM for all pre-existing rows (their legacy type column was uniformly ATM_CASH, which does not map to this binary split) -- see plan.md T0.1/T1.3.';

COMMIT;

-- ROLLBACK (documented, forward-only shop):
--   ALTER TABLE public.vendor_vaults DROP CONSTRAINT IF EXISTS fk_vendor_vaults_location;
--   ALTER TABLE public.vendor_vaults DROP CONSTRAINT IF EXISTS vendor_vaults_longitude_chk;
--   ALTER TABLE public.vendor_vaults DROP CONSTRAINT IF EXISTS vendor_vaults_latitude_chk;
--   ALTER TABLE public.vendor_vaults DROP CONSTRAINT IF EXISTS vendor_vaults_category_chk;
--   ALTER TABLE public.vendor_vaults DROP COLUMN IF EXISTS deleted_at;
--   ALTER TABLE public.vendor_vaults DROP COLUMN IF EXISTS category;
--   ALTER TABLE public.vendor_vaults DROP COLUMN IF EXISTS operating_hours;
--   ALTER TABLE public.vendor_vaults DROP COLUMN IF EXISTS longitude;
--   ALTER TABLE public.vendor_vaults DROP COLUMN IF EXISTS latitude;
--   ALTER TABLE public.vendor_vaults DROP COLUMN IF EXISTS location_id;

-- vendor_pics: new table, vendor contact persons (jabatan/phone/email,
-- notification recipient flag). URS v0.3 Phase 1 "Master data (vendor)":
-- "PIC jabatan/phone/email". See .claude/sdlc/master-data/plan.md T1.4.
--
-- SAFETY:
--   * New table, no existing data affected.
--   * vendor_branch_id nullable: a PIC can be vendor-wide (NULL) or scoped
--     to one branch, per plan.md.
--   * Forward-only: no down migration ships (project convention).

BEGIN;

CREATE TABLE public.vendor_pics (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    vendor_id bigint NOT NULL REFERENCES public.vendors(id),
    vendor_branch_id bigint REFERENCES public.vendor_branches(id),
    name text NOT NULL,
    "position" text,
    phone text,
    email text,
    is_notification_recipient boolean NOT NULL DEFAULT false,
    is_active boolean NOT NULL DEFAULT true,
    created_at timestamp with time zone NOT NULL DEFAULT now(),
    updated_at timestamp with time zone NOT NULL DEFAULT now(),
    deleted_at timestamp with time zone
);

CREATE INDEX vendor_pics_vendor_idx ON public.vendor_pics USING btree (vendor_id);
CREATE INDEX vendor_pics_vendor_branch_idx ON public.vendor_pics USING btree (vendor_branch_id);

CREATE TRIGGER trg_vendor_pics_set_updated_at
    BEFORE UPDATE ON public.vendor_pics
    FOR EACH ROW WHEN (old.* IS DISTINCT FROM new.*)
    EXECUTE FUNCTION public.set_updated_at();

COMMENT ON TABLE public.vendor_pics IS 'Vendor contact persons (PIC). A PIC is either vendor-wide (vendor_branch_id NULL) or scoped to one branch.';
COMMENT ON COLUMN public.vendor_pics.is_notification_recipient IS 'Whether this PIC receives system notifications (late DSR, replenishment, etc.) for the vendor/branch. At least one active PIC per vendor should have this true (enforced as a warning at the service layer, not a DB constraint -- see plan.md T3.3).';

COMMIT;

-- ROLLBACK (documented, forward-only shop):
--   DROP TABLE IF EXISTS public.vendor_pics;

-- master_data_change_requests: generic maker-checker staging table for
-- master-data mutations (Fase 2 of plan.md). Every create/update/disable/
-- enable on vendor/branch/vault/PIC/package/assignment entities is staged
-- here and applied only after approval (Golden Rule #3), via
-- internal/approval.Orchestrator keyed on document_type='master_data'.
-- See .claude/sdlc/master-data/plan.md T1.5, T2.1-T2.7.
--
-- SAFETY:
--   * New table, no existing data affected.
--   * entity_id nullable: NULL means this request is a create (the entity
--     does not exist yet).
--   * payload/before are jsonb, not typed per-entity -- keeps this table
--     entity-agnostic; each applier (T2.3) interprets its own entity_type's
--     payload shape.
--   * Partial unique index enforces "at most one pending change request per
--     entity" at the DB layer (T2.2's 409 guard reads this, doesn't just
--     rely on service-layer logic).
--   * Forward-only: no down migration ships (project convention).

BEGIN;

CREATE TABLE public.master_data_change_requests (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    entity_type text NOT NULL,
    entity_id bigint,
    op text NOT NULL,
    payload jsonb NOT NULL,
    before jsonb,
    status text NOT NULL DEFAULT 'pending',
    maker_id bigint NOT NULL REFERENCES public.users(id),
    approval_request_id bigint REFERENCES public.approval_requests(id),
    batch_id bigint,
    error text,
    created_at timestamp with time zone NOT NULL DEFAULT now(),
    updated_at timestamp with time zone NOT NULL DEFAULT now(),
    CONSTRAINT master_data_change_requests_op_chk CHECK (op IN ('create', 'update', 'disable', 'enable')),
    CONSTRAINT master_data_change_requests_status_chk CHECK (status IN ('pending', 'approved', 'rejected', 'applied', 'stale')),
    CONSTRAINT master_data_change_requests_entity_id_chk CHECK ((op = 'create' AND entity_id IS NULL) OR (op != 'create' AND entity_id IS NOT NULL))
);

CREATE UNIQUE INDEX master_data_change_requests_pending_uq
    ON public.master_data_change_requests USING btree (entity_type, entity_id)
    WHERE (status = 'pending');

CREATE INDEX master_data_change_requests_batch_idx ON public.master_data_change_requests USING btree (batch_id) WHERE batch_id IS NOT NULL;
CREATE INDEX master_data_change_requests_maker_idx ON public.master_data_change_requests USING btree (maker_id);

CREATE TRIGGER trg_master_data_change_requests_set_updated_at
    BEFORE UPDATE ON public.master_data_change_requests
    FOR EACH ROW WHEN (old.* IS DISTINCT FROM new.*)
    EXECUTE FUNCTION public.set_updated_at();

COMMENT ON TABLE public.master_data_change_requests IS 'Generic maker-checker staging for master-data create/update/disable/enable. Row is written on submit; applied to the real entity only after approval_requests reaches final approval (see internal/approval.Orchestrator + internal/service/masterdata_change.go).';
COMMENT ON COLUMN public.master_data_change_requests.entity_id IS 'NULL for op=create (entity does not exist yet); required for update/disable/enable.';
COMMENT ON COLUMN public.master_data_change_requests.payload IS 'Proposed new state (for create/update) or the intended flag flip (for disable/enable). Shape is entity_type-specific, interpreted by the matching applier.';
COMMENT ON COLUMN public.master_data_change_requests.before IS 'Snapshot of the entity''s state at submit time, for diff display and staleness detection (T2.5). NULL for op=create.';
COMMENT ON COLUMN public.master_data_change_requests.batch_id IS 'Groups rows created together by a single CSV import (T5.4). NULL for single-entity submits.';
COMMENT ON COLUMN public.master_data_change_requests.error IS 'Apply failure detail when status stays approved-but-not-applied due to a transactional apply error (T2.4).';

COMMIT;

-- ROLLBACK (documented, forward-only shop):
--   DROP TABLE IF EXISTS public.master_data_change_requests;

-- atm_vendor_packages: prevent overlapping active kelolaan periods for the
-- same ATM. URS v0.3 Phase 1 requires assign-with-effective-dates without
-- silent double-booking. See .claude/sdlc/master-data/plan.md T1.6.
--
-- SAFETY:
--   * T0.2 (2026-09-18) found 0 overlapping pairs across all 1,862 existing
--     rows (all open-ended, effective_end_date IS NULL) -- safe to add
--     without a data cleanup pass.
--   * btree_gist extension already enabled in the baseline (used by
--     approval_policies_no_overlap) -- no new extension needed.
--   * daterange(..., '[]') treats a NULL effective_end_date as unbounded
--     (infinity), matching how T0.2's overlap check was run.
--   * Forward-only: no down migration ships (project convention).

BEGIN;

ALTER TABLE public.atm_vendor_packages
    ADD CONSTRAINT atm_vendor_packages_no_overlap
    EXCLUDE USING gist (
        atm_id WITH =,
        daterange(effective_start_date, effective_end_date, '[]') WITH &&
    ) WHERE (is_active);

COMMENT ON CONSTRAINT atm_vendor_packages_no_overlap ON public.atm_vendor_packages
    IS 'Prevents two active kelolaan periods for the same ATM from overlapping. Violations surface as a Postgres exclusion-constraint error; T3.5 translates that into a 409 with a clear message rather than the raw DB error.';

COMMIT;

-- ROLLBACK (documented, forward-only shop):
--   ALTER TABLE public.atm_vendor_packages DROP CONSTRAINT IF EXISTS atm_vendor_packages_no_overlap;

-- approval_policies seed: routes document_type='master_data' change
-- requests to required_level=1 (single-level approval). Master-data
-- changes have no monetary amount, so the whole conceptual range [0, 1) is
-- covered by one policy row -- masterdata_change.Submit (T2.1) always
-- calls Orchestrator.SubmitForApproval with amount=0.
-- See .claude/sdlc/master-data/plan.md T1.7.

BEGIN;

INSERT INTO public.approval_policies (document_type, min_amount, max_amount, required_level, is_active)
VALUES ('master_data', 0.00, 1.00, 1, true);

COMMIT;

-- ROLLBACK (documented, forward-only shop):
--   DELETE FROM public.approval_policies WHERE document_type = 'master_data';
