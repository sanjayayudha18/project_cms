-- =====================================================
-- 006_create_atms_and_related.sql
-- =====================================================

BEGIN;

CREATE TABLE IF NOT EXISTS public.atms (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    terminal_id TEXT NOT NULL,
    location_id BIGINT NOT NULL,
    machine_type TEXT NOT NULL,
    brand TEXT NOT NULL,
    model TEXT NOT NULL,
    operation_hours TEXT NOT NULL,
    deployment_type TEXT NOT NULL,
    capacity_amount NUMERIC(20,2),
    low_threshold_amount NUMERIC(20,2),
    critical_threshold_amount NUMERIC(20,2),
    blacklisted BOOLEAN NOT NULL DEFAULT false,
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at TIMESTAMPTZ,

    CONSTRAINT atms_terminal_id_uq
        UNIQUE (terminal_id),

    CONSTRAINT fk_atms_location
        FOREIGN KEY (location_id)
        REFERENCES public.locations(id),

    CONSTRAINT atms_machine_type_chk
        CHECK (machine_type IN ('ATM', 'CRM')),

    CONSTRAINT atms_deployment_type_chk
        CHECK (deployment_type IN ('On-Premise', 'Off-Premise')),

    CONSTRAINT atms_capacity_amount_chk
        CHECK (capacity_amount IS NULL OR capacity_amount >= 0),

    CONSTRAINT atms_low_threshold_amount_chk
        CHECK (low_threshold_amount IS NULL OR low_threshold_amount >= 0),

    CONSTRAINT atms_critical_threshold_amount_chk
        CHECK (critical_threshold_amount IS NULL OR critical_threshold_amount >= 0),

    CONSTRAINT atms_threshold_order_chk
        CHECK (
            critical_threshold_amount IS NULL
            OR low_threshold_amount IS NULL
            OR critical_threshold_amount <= low_threshold_amount
        ),

    CONSTRAINT atms_capacity_vs_low_chk
        CHECK (
            low_threshold_amount IS NULL
            OR capacity_amount IS NULL
            OR low_threshold_amount <= capacity_amount
        )
);

CREATE INDEX IF NOT EXISTS atms_location_idx
    ON public.atms(location_id);

CREATE INDEX IF NOT EXISTS atms_status_idx
    ON public.atms(is_active, blacklisted);


CREATE TABLE IF NOT EXISTS public.atm_denoms (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    atm_id BIGINT NOT NULL,
    denom_id BIGINT NOT NULL,

    CONSTRAINT fk_atm_denoms_atm
        FOREIGN KEY (atm_id)
        REFERENCES public.atms(id)
        ON DELETE CASCADE,

    CONSTRAINT fk_atm_denoms_denom
        FOREIGN KEY (denom_id)
        REFERENCES public.denoms(id),

    CONSTRAINT atm_denoms_atm_denom_uq
        UNIQUE (atm_id, denom_id)
);

CREATE INDEX IF NOT EXISTS atm_denoms_atm_idx
    ON public.atm_denoms(atm_id);

CREATE INDEX IF NOT EXISTS atm_denoms_denom_idx
    ON public.atm_denoms(denom_id);


CREATE TABLE IF NOT EXISTS public.atm_vendor_packages (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    atm_id BIGINT NOT NULL,
    vendor_package_id BIGINT NOT NULL,
    effective_start_date DATE NOT NULL,
    effective_end_date DATE,
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT fk_atm_vendor_packages_atm
        FOREIGN KEY (atm_id)
        REFERENCES public.atms(id)
        ON DELETE CASCADE,

    CONSTRAINT fk_atm_vendor_packages_vendor_package
        FOREIGN KEY (vendor_package_id)
        REFERENCES public.vendor_packages(id),

    CONSTRAINT atm_vendor_packages_date_chk
        CHECK (
            effective_end_date IS NULL
            OR effective_end_date >= effective_start_date
        ),

    CONSTRAINT atm_vendor_packages_atm_package_start_uq
        UNIQUE (atm_id, vendor_package_id, effective_start_date)
);

CREATE INDEX IF NOT EXISTS atm_vendor_packages_atm_idx
    ON public.atm_vendor_packages(atm_id);

CREATE INDEX IF NOT EXISTS atm_vendor_packages_vendor_package_idx
    ON public.atm_vendor_packages(vendor_package_id);

CREATE INDEX IF NOT EXISTS atm_vendor_packages_active_idx
    ON public.atm_vendor_packages(atm_id, is_active);


COMMIT;