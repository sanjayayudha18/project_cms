-- ITM ATM cash position snapshots
-- Source: FTP_DATA/ITM/atm_caspos/ATM_Cashpos_*.csv
-- One row per ATM snapshot per source file.

BEGIN;

-- ============================================================
-- 1. File tracking (idempotent ingest per file)
-- ============================================================
CREATE TABLE IF NOT EXISTS public.itm_cashpos_files
(
    id bigint NOT NULL GENERATED ALWAYS AS IDENTITY ( INCREMENT 1 START 1 MINVALUE 1 MAXVALUE 9223372036854775807 CACHE 1 ),
    filename text COLLATE pg_catalog."default" NOT NULL,
    file_date date NOT NULL,
    checksum text COLLATE pg_catalog."default",
    status text COLLATE pg_catalog."default" NOT NULL DEFAULT 'pending',
    row_count integer,
    success_count integer,
    error_count integer,
    error_message text COLLATE pg_catalog."default",
    processed_at timestamp with time zone,
    created_at timestamp with time zone NOT NULL DEFAULT now(),
    updated_at timestamp with time zone NOT NULL DEFAULT now(),
    CONSTRAINT itm_cashpos_files_pkey PRIMARY KEY (id),
    CONSTRAINT itm_cashpos_files_checksum_uq UNIQUE (checksum)
);

COMMENT ON COLUMN public.itm_cashpos_files.status
    IS 'pending | processing | completed | failed';

COMMENT ON COLUMN public.itm_cashpos_files.file_date
    IS 'Business date parsed from the source filename';

-- ============================================================
-- 2. ATM cash position snapshots (parsed from CSV)
-- ============================================================
CREATE TABLE IF NOT EXISTS public.itm_cashpos
(
    id bigint NOT NULL GENERATED ALWAYS AS IDENTITY ( INCREMENT 1 START 1 MINVALUE 1 MAXVALUE 9223372036854775807 CACHE 1 ),
    file_id bigint NOT NULL,
    cashpos_date date NOT NULL,
    terminal_id text COLLATE pg_catalog."default" NOT NULL,
    machine_type text COLLATE pg_catalog."default" NOT NULL,
    teller_id text COLLATE pg_catalog."default" NOT NULL,
    branch_code text COLLATE pg_catalog."default" NOT NULL,
    starting_cash_10k numeric(20, 2) NOT NULL DEFAULT 0,
    cash_in_10k numeric(20, 2) NOT NULL DEFAULT 0,
    cash_out_10k numeric(20, 2) NOT NULL DEFAULT 0,
    cash_position_10k numeric(20, 2) NOT NULL DEFAULT 0,
    starting_cash_20k numeric(20, 2) NOT NULL DEFAULT 0,
    cash_in_20k numeric(20, 2) NOT NULL DEFAULT 0,
    cash_out_20k numeric(20, 2) NOT NULL DEFAULT 0,
    cash_position_20k numeric(20, 2) NOT NULL DEFAULT 0,
    starting_cash_50k numeric(20, 2) NOT NULL DEFAULT 0,
    cash_in_50k numeric(20, 2) NOT NULL DEFAULT 0,
    cash_out_50k numeric(20, 2) NOT NULL DEFAULT 0,
    cash_position_50k numeric(20, 2) NOT NULL DEFAULT 0,
    starting_cash_100k numeric(20, 2) NOT NULL DEFAULT 0,
    cash_in_100k numeric(20, 2) NOT NULL DEFAULT 0,
    cash_out_100k numeric(20, 2) NOT NULL DEFAULT 0,
    cash_position_100k numeric(20, 2) NOT NULL DEFAULT 0,
    position_source text COLLATE pg_catalog."default" NOT NULL,
    created_at timestamp with time zone NOT NULL DEFAULT now(),
    CONSTRAINT itm_cashpos_pkey PRIMARY KEY (id)
);

COMMENT ON COLUMN public.itm_cashpos.cashpos_date
    IS 'Business date parsed from the source filename';

COMMENT ON COLUMN public.itm_cashpos.machine_type
    IS 'ATM50K | ATM100K | CRM';

COMMENT ON COLUMN public.itm_cashpos.position_source
    IS 'Source indicator from POSITION_SOURCE, for example CURRENT or REPLENISH';

-- ============================================================
-- 3. Foreign key and indexes
-- ============================================================
ALTER TABLE IF EXISTS public.itm_cashpos
    ADD CONSTRAINT fk_itm_cashpos_file FOREIGN KEY (file_id)
    REFERENCES public.itm_cashpos_files (id) MATCH SIMPLE
    ON UPDATE NO ACTION
    ON DELETE CASCADE;

CREATE INDEX IF NOT EXISTS itm_cashpos_file_idx
    ON public.itm_cashpos(file_id);

CREATE INDEX IF NOT EXISTS itm_cashpos_terminal_date_idx
    ON public.itm_cashpos(terminal_id, cashpos_date);

CREATE INDEX IF NOT EXISTS itm_cashpos_cashpos_date_idx
    ON public.itm_cashpos(cashpos_date);

CREATE INDEX IF NOT EXISTS itm_cashpos_branch_code_idx
    ON public.itm_cashpos(branch_code);

COMMIT;
