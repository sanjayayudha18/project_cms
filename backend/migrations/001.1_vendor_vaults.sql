BEGIN;

CREATE TABLE IF NOT EXISTS public.vendor_vaults
(
    id bigint NOT NULL GENERATED ALWAYS AS IDENTITY ( INCREMENT 1 START 1 MINVALUE 1 MAXVALUE 9223372036854775807 CACHE 1 ),
    vendor_branch_id bigint NOT NULL,
    vault_code text COLLATE pg_catalog."default" NOT NULL,
    type text COLLATE pg_catalog."default" NOT NULL,
    amount numeric(20, 2),
    max_capacity_amount numeric(20, 2),
    min_capacity_amount numeric(20, 2),
    currency_code character(3) COLLATE pg_catalog."default" NOT NULL,
    is_active boolean NOT NULL DEFAULT true,
    created_at timestamp with time zone NOT NULL DEFAULT now(),
    updated_at timestamp with time zone NOT NULL DEFAULT now(),
    CONSTRAINT vendor_vaults_pkey PRIMARY KEY (id),
    CONSTRAINT vendor_vaults_vault_code_uq UNIQUE (vault_code)
);

COMMENT ON COLUMN public.vendor_vaults.type
    IS 'Type of vault (e.g. main, transit, buffer)';

COMMENT ON COLUMN public.vendor_vaults.currency_code
    IS 'ISO 4217 currency code (e.g. IDR, USD)';

ALTER TABLE IF EXISTS public.vendor_vaults
    ADD CONSTRAINT fk_vendor_vaults_vendor_branch FOREIGN KEY (vendor_branch_id)
    REFERENCES public.vendor_branches (id) MATCH SIMPLE
    ON UPDATE NO ACTION
    ON DELETE NO ACTION;

CREATE INDEX IF NOT EXISTS vendor_vaults_vendor_branch_idx
    ON public.vendor_vaults(vendor_branch_id);

END;
