-- ITM ATM Cash Position data
-- Source: FTP_DATA/ITM/ATM_Cashpos_*.csv
-- One row per replenishment event per ATM per day.
-- Same ATM can appear multiple times in a single file (multiple replenishments).

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
    IS 'Business date the file represents (from filename)';

-- ============================================================
-- 2. ATM cash position rows (parsed from CSV)
-- ============================================================
CREATE TABLE IF NOT EXISTS public.itm_cashpos
(
    id bigint NOT NULL GENERATED ALWAYS AS IDENTITY ( INCREMENT 1 START 1 MINVALUE 1 MAXVALUE 9223372036854775807 CACHE 1 ),
    file_id bigint NOT NULL,
    replenish_date date NOT NULL,
    replenish_time time without time zone NOT NULL,
    terminal_id text COLLATE pg_catalog."default" NOT NULL,
    machine_type text COLLATE pg_catalog."default" NOT NULL,
    teller_id text COLLATE pg_catalog."default" NOT NULL,
    branch_code text COLLATE pg_catalog."default" NOT NULL,
    escrow numeric(20, 2) NOT NULL DEFAULT 0,
    refund_denom_50k numeric(20, 2) NOT NULL DEFAULT 0,
    refund_denom_100k numeric(20, 2) NOT NULL DEFAULT 0,
    refund_total numeric(20, 2) NOT NULL DEFAULT 0,
    replenish_denom_50k numeric(20, 2) NOT NULL DEFAULT 0,
    replenish_denom_100k numeric(20, 2) NOT NULL DEFAULT 0,
    replenish_total numeric(20, 2) NOT NULL DEFAULT 0,
    created_at timestamp with time zone NOT NULL DEFAULT now(),
    CONSTRAINT itm_cashpos_pkey PRIMARY KEY (id)
);

COMMENT ON COLUMN public.itm_cashpos.replenish_date
    IS 'Parsed from TGLREPLENISH raw format (1YYMMDD)';

COMMENT ON COLUMN public.itm_cashpos.replenish_time
    IS 'Parsed from JAMREPLENISH raw format (HHMMSS without separators)';

COMMENT ON COLUMN public.itm_cashpos.machine_type
    IS 'ATM50K | ATM100K | CRM';

COMMENT ON COLUMN public.itm_cashpos.escrow
    IS 'Escrow balance at time of replenishment (19-digit raw value from ITM)';

-- ============================================================
-- 3. Foreign keys and indexes
-- ============================================================
ALTER TABLE IF EXISTS public.itm_cashpos
    ADD CONSTRAINT fk_itm_cashpos_file FOREIGN KEY (file_id)
    REFERENCES public.itm_cashpos_files (id) MATCH SIMPLE
    ON UPDATE NO ACTION
    ON DELETE CASCADE;

CREATE INDEX IF NOT EXISTS itm_cashpos_file_idx
    ON public.itm_cashpos(file_id);

CREATE INDEX IF NOT EXISTS itm_cashpos_terminal_date_idx
    ON public.itm_cashpos(terminal_id, replenish_date);

CREATE INDEX IF NOT EXISTS itm_cashpos_replenish_date_idx
    ON public.itm_cashpos(replenish_date);

CREATE INDEX IF NOT EXISTS itm_cashpos_branch_code_idx
    ON public.itm_cashpos(branch_code);

END;
