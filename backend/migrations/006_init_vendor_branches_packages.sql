BEGIN;

-- Assumptions:
-- 1) public.vendors already exists as shown in the diagram.
-- 2) public.locations already exists and location_id in vendor_branches is optional.
-- 3) vendor_packages.type is nullable because the diagram does not mark it NOT NULL.
-- 4) price is nullable because the diagram does not mark it NOT NULL.

CREATE TABLE IF NOT EXISTS public.vendor_branches (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    vendor_id BIGINT NOT NULL,
    branch_code TEXT NOT NULL,
    branch_name TEXT NOT NULL,
    location_id BIGINT,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT fk_vendor_branches_vendor
        FOREIGN KEY (vendor_id) REFERENCES public.vendors(id),
    CONSTRAINT fk_vendor_branches_location
        FOREIGN KEY (location_id) REFERENCES public.locations(id),
    CONSTRAINT uq_vendor_branches_branch_code UNIQUE (branch_code)
);

CREATE TABLE IF NOT EXISTS public.vendor_packages (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    vendor_branch_id BIGINT NOT NULL,
    code TEXT NOT NULL,
    type TEXT NOT NULL,
    price NUMERIC(20,2) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT fk_vendor_packages_vendor_branch
        FOREIGN KEY (vendor_branch_id)
        REFERENCES public.vendor_branches(id),

    CONSTRAINT vendor_packages_branch_code_uq
        UNIQUE (vendor_branch_id, code),

    CONSTRAINT vendor_packages_price_chk
        CHECK (price >= 0)
);

CREATE INDEX IF NOT EXISTS vendor_packages_vendor_branch_idx
    ON public.vendor_packages(vendor_branch_id);

COMMIT;
