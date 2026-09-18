--
-- PostgreSQL database dump
--


-- Dumped from database version 18.3
-- Dumped by pg_dump version 18.3

SET statement_timeout = 0;
SET lock_timeout = 0;
SET idle_in_transaction_session_timeout = 0;
SET transaction_timeout = 0;
SET client_encoding = 'UTF8';
SET standard_conforming_strings = on;
SET check_function_bodies = false;
SET xmloption = content;
SET client_min_messages = warning;
SET row_security = off;

--
-- Name: public; Type: SCHEMA; Schema: -; Owner: -
--

CREATE EXTENSION IF NOT EXISTS btree_gist WITH SCHEMA public;
CREATE EXTENSION IF NOT EXISTS pg_trgm WITH SCHEMA public;


--
-- Name: SCHEMA public; Type: COMMENT; Schema: -; Owner: -
--

COMMENT ON SCHEMA public IS 'standard public schema';


--
-- Name: set_updated_at(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.set_updated_at() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
    NEW.updated_at = now();
    RETURN NEW;
END;
$$;


--
-- Name: FUNCTION set_updated_at(); Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON FUNCTION public.set_updated_at() IS 'BEFORE UPDATE trigger: stamps updated_at = now(). Attached to every table declaring an updated_at column; see migration 014.';


SET default_tablespace = '';

SET default_table_access_method = heap;

--
-- Name: approval_delegations; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.approval_delegations (
    id bigint NOT NULL,
    from_user_id bigint NOT NULL,
    to_user_id bigint NOT NULL,
    start_at timestamp with time zone NOT NULL,
    end_at timestamp with time zone NOT NULL,
    reason text,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT approval_delegations_not_self_chk CHECK ((from_user_id <> to_user_id)),
    CONSTRAINT approval_delegations_range_chk CHECK ((start_at < end_at))
);


--
-- Name: TABLE approval_delegations; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE public.approval_delegations IS 'Explicit fallback: while from_user_id cannot act (see user_leaves), to_user_id acts in their place. Overlap-range rejection is enforced at the admin endpoint layer (Task 8), not here.';


--
-- Name: approval_delegations_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.approval_delegations_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: approval_delegations_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.approval_delegations_id_seq OWNED BY public.approval_delegations.id;


--
-- Name: approval_policies; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.approval_policies (
    id bigint NOT NULL,
    document_type text NOT NULL,
    min_amount numeric(20,2) NOT NULL,
    max_amount numeric(20,2) NOT NULL,
    required_level integer NOT NULL,
    is_active boolean DEFAULT true NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT approval_policies_amount_range_chk CHECK ((min_amount < max_amount))
);


--
-- Name: TABLE approval_policies; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE public.approval_policies IS 'Threshold config: (document_type, amount range) -> required_level for maker-checker approval routing.';


--
-- Name: COLUMN approval_policies.min_amount; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.approval_policies.min_amount IS 'Inclusive lower bound of the amount range this policy covers.';


--
-- Name: COLUMN approval_policies.max_amount; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.approval_policies.max_amount IS 'Exclusive upper bound of the amount range this policy covers.';


--
-- Name: COLUMN approval_policies.required_level; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.approval_policies.required_level IS 'Approval level (matches users.approval_level) the request must climb to when it falls in this range.';


--
-- Name: approval_policies_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.approval_policies_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: approval_policies_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.approval_policies_id_seq OWNED BY public.approval_policies.id;


--
-- Name: approval_requests; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.approval_requests (
    id bigint NOT NULL,
    maker_id bigint NOT NULL,
    document_type text NOT NULL,
    document_id bigint NOT NULL,
    amount numeric(20,2) NOT NULL,
    required_level integer NOT NULL,
    status text DEFAULT 'draft'::text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT approval_requests_status_chk CHECK ((status = ANY (ARRAY['draft'::text, 'pending'::text, 'approved'::text, 'rejected'::text])))
);


--
-- Name: TABLE approval_requests; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE public.approval_requests IS 'One row per document under approval. Idempotent per (document_type, document_id).';


--
-- Name: COLUMN approval_requests.status; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.approval_requests.status IS 'draft | pending | approved | rejected';


--
-- Name: approval_requests_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.approval_requests_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: approval_requests_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.approval_requests_id_seq OWNED BY public.approval_requests.id;


--
-- Name: approval_steps; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.approval_steps (
    id bigint NOT NULL,
    request_id bigint NOT NULL,
    step_level integer NOT NULL,
    assigned_approver_id bigint NOT NULL,
    acted_by_id bigint,
    status text DEFAULT 'pending'::text NOT NULL,
    acted_at timestamp with time zone,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT approval_steps_status_chk CHECK ((status = ANY (ARRAY['pending'::text, 'approved'::text, 'rejected'::text])))
);


--
-- Name: TABLE approval_steps; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE public.approval_steps IS 'One row per approval level a request must pass through. acted_by_id may differ from assigned_approver_id (delegate acted instead).';


--
-- Name: COLUMN approval_steps.acted_by_id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.approval_steps.acted_by_id IS 'Who actually acted on this step — the assigned approver, or their delegate if on leave. NULL until acted.';


--
-- Name: COLUMN approval_steps.status; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.approval_steps.status IS 'pending | approved | rejected';


--
-- Name: approval_steps_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.approval_steps_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: approval_steps_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.approval_steps_id_seq OWNED BY public.approval_steps.id;


--
-- Name: atm_denoms; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.atm_denoms (
    id bigint NOT NULL,
    atm_id bigint NOT NULL,
    denom_id bigint NOT NULL
);


--
-- Name: atm_denoms_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.atm_denoms ALTER COLUMN id ADD GENERATED ALWAYS AS IDENTITY (
    SEQUENCE NAME public.atm_denoms_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: atm_vendor_packages; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.atm_vendor_packages (
    id bigint NOT NULL,
    atm_id bigint NOT NULL,
    vendor_package_id bigint NOT NULL,
    effective_start_date date NOT NULL,
    effective_end_date date,
    is_active boolean DEFAULT true NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: atm_vendor_packages_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.atm_vendor_packages ALTER COLUMN id ADD GENERATED ALWAYS AS IDENTITY (
    SEQUENCE NAME public.atm_vendor_packages_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: atms; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.atms (
    id bigint NOT NULL,
    terminal_id text NOT NULL,
    location_id bigint NOT NULL,
    machine_type text NOT NULL,
    brand text NOT NULL,
    model text NOT NULL,
    operation_hours text NOT NULL,
    deployment_type text NOT NULL,
    capacity_amount numeric(20,2),
    low_threshold_amount numeric(20,2),
    critical_threshold_amount numeric(20,2),
    blacklisted boolean DEFAULT false NOT NULL,
    is_active boolean DEFAULT true NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    deleted_at timestamp with time zone,
    escrow_account text,
    priority_class text
);


--
-- Name: COLUMN atms.escrow_account; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.atms.escrow_account IS 'Escrow account number per ATM from MASTER_ATM_ESQ.Escrow (12-digit identifier, stored as text). See migration 030.';


--
-- Name: COLUMN atms.priority_class; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.atms.priority_class IS 'ATM priority tier from MASTER_ATM_ESQ.PriorityClass: VIP | Non VIP | Industri. Per-ATM (varies within a vendor branch), so stored on atms not vendor_packages. See migration 030.';


--
-- Name: atms_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.atms ALTER COLUMN id ADD GENERATED ALWAYS AS IDENTITY (
    SEQUENCE NAME public.atms_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: audit_logs; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.audit_logs (
    id bigint NOT NULL,
    actor_id bigint NOT NULL,
    action text NOT NULL,
    entity_type text NOT NULL,
    entity_id bigint NOT NULL,
    before jsonb,
    after jsonb,
    ip text,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: TABLE audit_logs; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE public.audit_logs IS 'Append-only audit trail: who (actor_id) did what (action) to which entity, before/after state, from where (ip). No update/delete query exists in code.';


--
-- Name: COLUMN audit_logs.action; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.audit_logs.action IS 'Free-form verb, e.g. submit | approve | reject | create | update.';


--
-- Name: COLUMN audit_logs.entity_type; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.audit_logs.entity_type IS 'Domain entity the action targets, e.g. approval_request, invoice.';


--
-- Name: audit_logs_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.audit_logs_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: audit_logs_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.audit_logs_id_seq OWNED BY public.audit_logs.id;


--
-- Name: currencies; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.currencies (
    id bigint NOT NULL,
    code text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: currencies_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.currencies ALTER COLUMN id ADD GENERATED BY DEFAULT AS IDENTITY (
    SEQUENCE NAME public.currencies_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: denoms; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.denoms (
    id bigint NOT NULL,
    curr_id bigint NOT NULL,
    denom numeric(20,2) NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: denoms_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.denoms ALTER COLUMN id ADD GENERATED BY DEFAULT AS IDENTITY (
    SEQUENCE NAME public.denoms_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: dmaa_atm_forecast; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.dmaa_atm_forecast (
    terminal_id text NOT NULL,
    dmaa_file_id bigint NOT NULL,
    periode_pred date NOT NULL,
    denom integer NOT NULL,
    amount_replenish bigint DEFAULT 0 NOT NULL,
    amount_refund bigint DEFAULT 0 NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: dmaa_files; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.dmaa_files (
    id bigint NOT NULL,
    name text NOT NULL,
    status text NOT NULL,
    is_valid boolean DEFAULT true NOT NULL,
    file_date date,
    source_system text,
    checksum text,
    row_count integer,
    success_count integer,
    error_count integer,
    error_message text,
    processed_at timestamp with time zone,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: dmaa_files_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.dmaa_files ALTER COLUMN id ADD GENERATED BY DEFAULT AS IDENTITY (
    SEQUENCE NAME public.dmaa_files_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: dsr_daily_rows; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.dsr_daily_rows (
    id bigint NOT NULL,
    upload_id bigint NOT NULL,
    row_no integer NOT NULL,
    location text,
    section text NOT NULL,
    flow text NOT NULL,
    line_label text NOT NULL,
    memo_no text,
    denom_100k_idr numeric(20,2),
    denom_50k_idr numeric(20,2),
    denom_20k_idr numeric(20,2),
    denom_10k_idr numeric(20,2),
    denom_5k_idr numeric(20,2),
    denom_2k_idr numeric(20,2),
    denom_1k_idr numeric(20,2),
    line_total_idr numeric(20,2),
    remarks text,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT dsr_daily_rows_flow_chk CHECK ((flow = ANY (ARRAY['saldo_awal'::text, 'penerimaan'::text, 'pengeluaran'::text, 'status_cadangan'::text]))),
    CONSTRAINT dsr_daily_rows_section_chk CHECK ((section = ANY (ARRAY['d0'::text, 'd1'::text])))
);


--
-- Name: COLUMN dsr_daily_rows.row_no; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.dsr_daily_rows.row_no IS 'Sheet line order (1..n) across both sections, excluding headers and derived subtotal/saldo rows. Stable ordering key for reproducing the statement.';


--
-- Name: COLUMN dsr_daily_rows.location; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.dsr_daily_rows.location IS 'Vault/site block header the row belongs to (e.g. LENTENG AGUNG, BINTARO). NULL for single-vault workbooks like BIJAK, which print one block.';


--
-- Name: COLUMN dsr_daily_rows.section; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.dsr_daily_rows.section IS 'd0 = section 1 (report_date, settled, sampai pukul 00:00). d1 = section 2 (daily_status_date, status sementara).';


--
-- Name: COLUMN dsr_daily_rows.flow; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.dsr_daily_rows.flow IS 'saldo_awal = opening balance (d0 only). penerimaan / pengeluaran = receipt / disbursement line, sign stored verbatim as printed. status_cadangan = STATUS UANG CADANGAN ATM lines (Layak Edar / Rusak), d1 only.';


--
-- Name: COLUMN dsr_daily_rows.line_label; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.dsr_daily_rows.line_label IS 'Uraian text verbatim, e.g. Dari CIMB Niaga CIT, Untuk Cartridge Replenishment ATM & CRM.';


--
-- Name: COLUMN dsr_daily_rows.memo_no; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.dsr_daily_rows.memo_no IS 'Memo/reference embedded in a line label, e.g. 313/ATM/BIJAK/VII/2026. NULL when the line carries none.';


--
-- Name: COLUMN dsr_daily_rows.denom_100k_idr; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.dsr_daily_rows.denom_100k_idr IS 'Value attributable to the 100,000 denomination, verbatim as printed (the sheet labels this block "Denom (lembar)" but prints money). NULL when the source cell was an Excel error -- counts into daily_error_count.';


--
-- Name: COLUMN dsr_daily_rows.line_total_idr; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.dsr_daily_rows.line_total_idr IS 'Total Rupiah x1.000 column, verbatim as printed. Cross-check: equals SUM(denom_*_idr) when no denom cell is NULL.';


--
-- Name: dsr_daily_rows_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.dsr_daily_rows ALTER COLUMN id ADD GENERATED ALWAYS AS IDENTITY (
    SEQUENCE NAME public.dsr_daily_rows_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: dsr_rencana_isi_rows; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.dsr_rencana_isi_rows (
    id bigint NOT NULL,
    upload_id bigint NOT NULL,
    row_no integer NOT NULL,
    atm_terminal_id text NOT NULL,
    atm_id bigint,
    atm_location text,
    denom_config text,
    fill_100k_idr numeric(20,2),
    fill_50k_idr numeric(20,2),
    splank_balance_0800_idr numeric(20,2),
    remarks text,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: COLUMN dsr_rencana_isi_rows.atm_terminal_id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.dsr_rencana_isi_rows.atm_terminal_id IS 'ATM ID column as sent by the vendor (e.g. 2440, A353, ZZVX). Verbatim, never overwritten -- this is the evidence of what the vendor claimed.';


--
-- Name: COLUMN dsr_rencana_isi_rows.atm_id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.dsr_rencana_isi_rows.atm_id IS 'Resolved at ingest from atm_terminal_id. NULL = ATM not in master data; the file still ingests (counts into rencana_isi_error_count). Find unresolved rows WHERE atm_id IS NULL.';


--
-- Name: COLUMN dsr_rencana_isi_rows.atm_location; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.dsr_rencana_isi_rows.atm_location IS 'Lokasi ATM column verbatim, e.g. JKT.AMBASADOR 1.';


--
-- Name: COLUMN dsr_rencana_isi_rows.denom_config; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.dsr_rencana_isi_rows.denom_config IS 'Denom (x 1.000) column as printed: 50 | 100 | TST. TST rows are recyclers and carry both fill_100k_idr and fill_50k_idr.';


--
-- Name: COLUMN dsr_rencana_isi_rows.splank_balance_0800_idr; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.dsr_rencana_isi_rows.splank_balance_0800_idr IS 'Saldo Splank Pukul 08:00 column, verbatim as printed. 0 on TST/recycler rows in the sample.';


--
-- Name: COLUMN dsr_rencana_isi_rows.remarks; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.dsr_rencana_isi_rows.remarks IS 'Keterangan column verbatim, e.g. PENDING DSR TGL 16/07/2026.';


--
-- Name: dsr_rencana_isi_rows_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.dsr_rencana_isi_rows ALTER COLUMN id ADD GENERATED ALWAYS AS IDENTITY (
    SEQUENCE NAME public.dsr_rencana_isi_rows_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: dsr_uploads; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.dsr_uploads (
    id bigint NOT NULL,
    filename text NOT NULL,
    checksum text,
    vendor text NOT NULL,
    report_date date NOT NULL,
    uploaded_by_user_id bigint,
    bank text,
    company text,
    recipient text,
    sender text,
    subject text,
    prepared_by text,
    checked_by text,
    approved_by text,
    currency text DEFAULT 'IDR'::text NOT NULL,
    daily_status text DEFAULT 'pending'::text NOT NULL,
    daily_row_count integer,
    daily_error_count integer,
    rencana_isi_status text DEFAULT 'pending'::text NOT NULL,
    rencana_isi_row_count integer,
    rencana_isi_error_count integer,
    error_message text,
    daily_status_date date,
    rencana_isi_plan_date date,
    saldo_akhir_0000_idr numeric(20,2),
    saldo_sementara_0900_idr numeric(20,2),
    status_cadangan_idr numeric(20,2),
    saldo_gabungan_idr numeric(20,2),
    rencana_isi_subtotal_idr numeric(20,2),
    processed_at timestamp with time zone,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT dsr_uploads_daily_status_chk CHECK ((daily_status = ANY (ARRAY['pending'::text, 'processing'::text, 'completed'::text, 'failed'::text]))),
    CONSTRAINT dsr_uploads_rencana_isi_status_chk CHECK ((rencana_isi_status = ANY (ARRAY['pending'::text, 'processing'::text, 'completed'::text, 'failed'::text])))
);


--
-- Name: TABLE dsr_uploads; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE public.dsr_uploads IS 'One row per ingested DSR workbook. Idempotent per checksum; re-ingest = delete this row, both sheets child rows cascade.';


--
-- Name: COLUMN dsr_uploads.vendor; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.dsr_uploads.vendor IS 'Vendor label as printed (e.g. BIJAK JAKARTA). Not yet FK to vendors.code.';


--
-- Name: COLUMN dsr_uploads.report_date; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.dsr_uploads.report_date IS 'Daily sheet Tanggal cell (e.g. 2026-07-15). Cross-checked against the filename.';


--
-- Name: COLUMN dsr_uploads.uploaded_by_user_id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.dsr_uploads.uploaded_by_user_id IS 'users.id of the vendor user who uploaded, resolved server-side from the JWT. NULL for manual backfill.';


--
-- Name: COLUMN dsr_uploads.daily_status_date; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.dsr_uploads.daily_status_date IS 'Section-2 provisional date on Daily (STATUS SALDO SEMENTARA), normally report_date + 1.';


--
-- Name: COLUMN dsr_uploads.rencana_isi_plan_date; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.dsr_uploads.rencana_isi_plan_date IS 'Tanggal cell on Rencana Isi: the day being planned, normally report_date + 1.';


--
-- Name: COLUMN dsr_uploads.rencana_isi_subtotal_idr; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.dsr_uploads.rencana_isi_subtotal_idr IS 'Vendor-stated Sub Total of the fill plan. Must equal the Daily d1 Subtotal Pengeluaran total -- the one cross-sheet tie in the workbook.';


--
-- Name: dsr_uploads_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.dsr_uploads ALTER COLUMN id ADD GENERATED ALWAYS AS IDENTITY (
    SEQUENCE NAME public.dsr_uploads_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: itm_cashpos; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.itm_cashpos (
    id bigint CONSTRAINT itm_cashpos_id_not_null1 NOT NULL,
    file_id bigint CONSTRAINT itm_cashpos_file_id_not_null1 NOT NULL,
    cashpos_date date NOT NULL,
    terminal_id text CONSTRAINT itm_cashpos_terminal_id_not_null1 NOT NULL,
    machine_type text CONSTRAINT itm_cashpos_machine_type_not_null1 NOT NULL,
    teller_id text CONSTRAINT itm_cashpos_teller_id_not_null1 NOT NULL,
    branch_code text CONSTRAINT itm_cashpos_branch_code_not_null1 NOT NULL,
    starting_cash_10k numeric(20,2) DEFAULT 0 NOT NULL,
    cash_in_10k numeric(20,2) DEFAULT 0 NOT NULL,
    cash_out_10k numeric(20,2) DEFAULT 0 NOT NULL,
    cash_position_10k numeric(20,2) DEFAULT 0 NOT NULL,
    starting_cash_20k numeric(20,2) DEFAULT 0 NOT NULL,
    cash_in_20k numeric(20,2) DEFAULT 0 NOT NULL,
    cash_out_20k numeric(20,2) DEFAULT 0 NOT NULL,
    cash_position_20k numeric(20,2) DEFAULT 0 NOT NULL,
    starting_cash_50k numeric(20,2) DEFAULT 0 NOT NULL,
    cash_in_50k numeric(20,2) DEFAULT 0 NOT NULL,
    cash_out_50k numeric(20,2) DEFAULT 0 NOT NULL,
    cash_position_50k numeric(20,2) DEFAULT 0 NOT NULL,
    starting_cash_100k numeric(20,2) DEFAULT 0 NOT NULL,
    cash_in_100k numeric(20,2) DEFAULT 0 NOT NULL,
    cash_out_100k numeric(20,2) DEFAULT 0 NOT NULL,
    cash_position_100k numeric(20,2) DEFAULT 0 NOT NULL,
    position_source text NOT NULL,
    created_at timestamp with time zone DEFAULT now() CONSTRAINT itm_cashpos_created_at_not_null1 NOT NULL
);


--
-- Name: COLUMN itm_cashpos.cashpos_date; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.itm_cashpos.cashpos_date IS 'Business date parsed from the source filename';


--
-- Name: COLUMN itm_cashpos.machine_type; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.itm_cashpos.machine_type IS 'ATM50K | ATM100K | CRM';


--
-- Name: COLUMN itm_cashpos.position_source; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.itm_cashpos.position_source IS 'Source indicator from POSITION_SOURCE, for example CURRENT or REPLENISH';


--
-- Name: itm_cashpos_files; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.itm_cashpos_files (
    id bigint CONSTRAINT itm_cashpos_files_id_not_null1 NOT NULL,
    filename text CONSTRAINT itm_cashpos_files_filename_not_null1 NOT NULL,
    file_date date CONSTRAINT itm_cashpos_files_file_date_not_null1 NOT NULL,
    checksum text,
    status text DEFAULT 'pending'::text CONSTRAINT itm_cashpos_files_status_not_null1 NOT NULL,
    row_count integer,
    success_count integer,
    error_count integer,
    error_message text,
    processed_at timestamp with time zone,
    created_at timestamp with time zone DEFAULT now() CONSTRAINT itm_cashpos_files_created_at_not_null1 NOT NULL,
    updated_at timestamp with time zone DEFAULT now() CONSTRAINT itm_cashpos_files_updated_at_not_null1 NOT NULL
);


--
-- Name: COLUMN itm_cashpos_files.file_date; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.itm_cashpos_files.file_date IS 'Business date parsed from the source filename';


--
-- Name: COLUMN itm_cashpos_files.status; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.itm_cashpos_files.status IS 'pending | processing | completed | failed';


--
-- Name: itm_replenish_files; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.itm_replenish_files (
    id bigint CONSTRAINT itm_cashpos_files_id_not_null NOT NULL,
    filename text CONSTRAINT itm_cashpos_files_filename_not_null NOT NULL,
    file_date date CONSTRAINT itm_cashpos_files_file_date_not_null NOT NULL,
    checksum text,
    status text DEFAULT 'pending'::text CONSTRAINT itm_cashpos_files_status_not_null NOT NULL,
    row_count integer,
    success_count integer,
    error_count integer,
    error_message text,
    processed_at timestamp with time zone,
    created_at timestamp with time zone DEFAULT now() CONSTRAINT itm_cashpos_files_created_at_not_null NOT NULL,
    updated_at timestamp with time zone DEFAULT now() CONSTRAINT itm_cashpos_files_updated_at_not_null NOT NULL
);


--
-- Name: COLUMN itm_replenish_files.file_date; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.itm_replenish_files.file_date IS 'Business date the file represents (from filename)';


--
-- Name: COLUMN itm_replenish_files.status; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.itm_replenish_files.status IS 'pending | processing | completed | failed';


--
-- Name: itm_cashpos_files_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.itm_replenish_files ALTER COLUMN id ADD GENERATED ALWAYS AS IDENTITY (
    SEQUENCE NAME public.itm_cashpos_files_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: itm_cashpos_files_id_seq1; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.itm_cashpos_files ALTER COLUMN id ADD GENERATED ALWAYS AS IDENTITY (
    SEQUENCE NAME public.itm_cashpos_files_id_seq1
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: itm_replenish; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.itm_replenish (
    id bigint CONSTRAINT itm_cashpos_id_not_null NOT NULL,
    file_id bigint CONSTRAINT itm_cashpos_file_id_not_null NOT NULL,
    replenish_date date CONSTRAINT itm_cashpos_replenish_date_not_null NOT NULL,
    replenish_time time without time zone CONSTRAINT itm_cashpos_replenish_time_not_null NOT NULL,
    terminal_id text CONSTRAINT itm_cashpos_terminal_id_not_null NOT NULL,
    machine_type text CONSTRAINT itm_cashpos_machine_type_not_null NOT NULL,
    teller_id text CONSTRAINT itm_cashpos_teller_id_not_null NOT NULL,
    branch_code text CONSTRAINT itm_cashpos_branch_code_not_null NOT NULL,
    escrow numeric(20,2) DEFAULT 0 CONSTRAINT itm_cashpos_escrow_not_null NOT NULL,
    refund_denom_10k numeric(20,2) DEFAULT 0 CONSTRAINT itm_cashpos_refund_denom_10k_not_null NOT NULL,
    refund_denom_20k numeric(20,2) DEFAULT 0 CONSTRAINT itm_cashpos_refund_denom_20k_not_null NOT NULL,
    refund_denom_50k numeric(20,2) DEFAULT 0 CONSTRAINT itm_cashpos_refund_denom_50k_not_null NOT NULL,
    refund_denom_100k numeric(20,2) DEFAULT 0 CONSTRAINT itm_cashpos_refund_denom_100k_not_null NOT NULL,
    refund_total numeric(20,2) DEFAULT 0 CONSTRAINT itm_cashpos_refund_total_not_null NOT NULL,
    replenish_denom_10k numeric(20,2) DEFAULT 0 CONSTRAINT itm_cashpos_replenish_denom_10k_not_null NOT NULL,
    replenish_denom_20k numeric(20,2) DEFAULT 0 CONSTRAINT itm_cashpos_replenish_denom_20k_not_null NOT NULL,
    replenish_denom_50k numeric(20,2) DEFAULT 0 CONSTRAINT itm_cashpos_replenish_denom_50k_not_null NOT NULL,
    replenish_denom_100k numeric(20,2) DEFAULT 0 CONSTRAINT itm_cashpos_replenish_denom_100k_not_null NOT NULL,
    replenish_total numeric(20,2) DEFAULT 0 CONSTRAINT itm_cashpos_replenish_total_not_null NOT NULL,
    created_at timestamp with time zone DEFAULT now() CONSTRAINT itm_cashpos_created_at_not_null NOT NULL
);


--
-- Name: COLUMN itm_replenish.replenish_date; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.itm_replenish.replenish_date IS 'Parsed from TGLREPLENISH raw format (1YYMMDD)';


--
-- Name: COLUMN itm_replenish.replenish_time; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.itm_replenish.replenish_time IS 'Parsed from JAMREPLENISH raw format (HHMMSS without separators)';


--
-- Name: COLUMN itm_replenish.machine_type; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.itm_replenish.machine_type IS 'ATM50K | ATM100K | CRM';


--
-- Name: COLUMN itm_replenish.escrow; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.itm_replenish.escrow IS 'Escrow balance at time of replenishment (19-digit raw value from ITM)';


--
-- Name: itm_cashpos_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.itm_replenish ALTER COLUMN id ADD GENERATED ALWAYS AS IDENTITY (
    SEQUENCE NAME public.itm_cashpos_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: itm_cashpos_id_seq1; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.itm_cashpos ALTER COLUMN id ADD GENERATED ALWAYS AS IDENTITY (
    SEQUENCE NAME public.itm_cashpos_id_seq1
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: locations; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.locations (
    id bigint NOT NULL,
    region_id bigint NOT NULL,
    type text NOT NULL,
    name text NOT NULL,
    address_line1 text NOT NULL,
    address_line2 text,
    rt text,
    rw text,
    kelurahan text,
    kecamatan text,
    city_or_regency text NOT NULL,
    province text NOT NULL,
    postal_code text,
    country_code character(2) NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: locations_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.locations ALTER COLUMN id ADD GENERATED ALWAYS AS IDENTITY (
    SEQUENCE NAME public.locations_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: menu_features; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.menu_features (
    id bigint NOT NULL,
    parent_id bigint,
    key text NOT NULL,
    label text NOT NULL,
    kind text NOT NULL,
    sort_order integer DEFAULT 0 NOT NULL,
    is_active boolean DEFAULT true NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT menu_features_hierarchy_chk CHECK ((((kind = 'menu'::text) AND (parent_id IS NULL)) OR ((kind = 'feature'::text) AND (parent_id IS NOT NULL)))),
    CONSTRAINT menu_features_kind_chk CHECK ((kind = ANY (ARRAY['menu'::text, 'feature'::text])))
);


--
-- Name: menu_features_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.menu_features ALTER COLUMN id ADD GENERATED ALWAYS AS IDENTITY (
    SEQUENCE NAME public.menu_features_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: regions; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.regions (
    id bigint NOT NULL,
    code text NOT NULL,
    region text,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: regions_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.regions ALTER COLUMN id ADD GENERATED ALWAYS AS IDENTITY (
    SEQUENCE NAME public.regions_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: role_permissions; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.role_permissions (
    id bigint NOT NULL,
    role_id bigint NOT NULL,
    menu_feature_id bigint NOT NULL,
    granted_by bigint NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: role_permissions_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.role_permissions ALTER COLUMN id ADD GENERATED ALWAYS AS IDENTITY (
    SEQUENCE NAME public.role_permissions_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: roles; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.roles (
    id bigint NOT NULL,
    role text NOT NULL,
    description text,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: COLUMN roles.role; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.roles.role IS 'ADMIN | ADMIN_PARAM | ATM-USER | ATM-SPV | BRANCH-USER | BRANCH-SPV | BRANCH-ATM-USER | BRANCH-ATM-SPV | VENDOR-USER | APPACCESS';


--
-- Name: roles_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.roles_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: roles_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.roles_id_seq OWNED BY public.roles.id;


--
-- Name: user_leaves; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.user_leaves (
    id bigint NOT NULL,
    user_id bigint NOT NULL,
    start_at timestamp with time zone NOT NULL,
    end_at timestamp with time zone NOT NULL,
    reason text,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT user_leaves_range_chk CHECK ((start_at < end_at))
);


--
-- Name: TABLE user_leaves; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE public.user_leaves IS 'Explicit unavailability windows. The resolver (Task 5) checks for an overlap with now() to decide whether an approver is on leave.';


--
-- Name: user_leaves_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.user_leaves_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: user_leaves_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.user_leaves_id_seq OWNED BY public.user_leaves.id;


--
-- Name: users; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.users (
    id bigint NOT NULL,
    role_id bigint NOT NULL,
    employee_id text,
    username text NOT NULL,
    full_name text NOT NULL,
    email text NOT NULL,
    is_karyawan boolean NOT NULL,
    auth_source text NOT NULL,
    password_hash text,
    vendor_id bigint,
    is_active boolean DEFAULT true NOT NULL,
    last_login_at timestamp with time zone,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    deleted_at timestamp with time zone,
    vendor_branch_id bigint,
    supervisor_id bigint,
    approval_level integer,
    password_changed_at timestamp with time zone,
    must_change_password boolean DEFAULT false NOT NULL,
    failed_login_attempts integer DEFAULT 0 NOT NULL,
    locked_until timestamp with time zone,
    CONSTRAINT users_supervisor_not_self_chk CHECK ((supervisor_id IS DISTINCT FROM id))
);


--
-- Name: COLUMN users.employee_id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.users.employee_id IS 'Nullable for external/vendor users';


--
-- Name: COLUMN users.auth_source; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.users.auth_source IS 'ldap | local';


--
-- Name: COLUMN users.password_hash; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.users.password_hash IS 'Only for auth_source=local';


--
-- Name: COLUMN users.vendor_id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.users.vendor_id IS 'Required for vendor/local users, null for internal LDAP users';


--
-- Name: COLUMN users.vendor_branch_id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.users.vendor_branch_id IS 'Optional: pins a vendor user to one vendor_branches row. Must belong to the same vendor as vendor_id (enforced in app). NULL for internal/LDAP users.';


--
-- Name: COLUMN users.supervisor_id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.users.supervisor_id IS 'Direct supervisor in the reporting line (self-FK, tree). NULL = top of hierarchy or not yet assigned.';


--
-- Name: COLUMN users.approval_level; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.users.approval_level IS 'Explicit approval level for maker-checker routing, independent of role_id. NULL = not an approver.';


--
-- Name: COLUMN users.password_changed_at; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.users.password_changed_at IS 'Local-password policy only (auth_source=local|local_dev): last password change, basis for the 90-day expiry / 7-day warning window. NULL = not yet evaluated. Ignored for auth_source=ldap.';


--
-- Name: COLUMN users.must_change_password; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.users.must_change_password IS 'Local-password policy only: true forces a password change on next login (set by APPACCESS set-initial-password, or after admin unlock). Ignored for auth_source=ldap.';


--
-- Name: COLUMN users.failed_login_attempts; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.users.failed_login_attempts IS 'Local-password policy only: consecutive failed local-login attempts; resets to 0 on success. 3 -> account locked (see locked_until). Ignored for auth_source=ldap.';


--
-- Name: COLUMN users.locked_until; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.users.locked_until IS 'Local-password policy only: account locked until this timestamp (30 min after the 3rd consecutive failed login), or NULL if not locked. Ignored for auth_source=ldap.';


--
-- Name: users_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.users_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: users_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.users_id_seq OWNED BY public.users.id;


--
-- Name: vendor_branches; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.vendor_branches (
    id bigint NOT NULL,
    vendor_id bigint NOT NULL,
    branch_code text NOT NULL,
    branch_name text NOT NULL,
    location_id bigint,
    is_active boolean DEFAULT true NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    region text
);


--
-- Name: COLUMN vendor_branches.region; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.vendor_branches.region IS 'Vendor regional grouping from MASTER_ATM_ESQ.FLMVendorRegion (e.g. "Jakarta Timur"). Distinct from regions.region (ATM geographic area) and branch_name (vendor sub-region). See migration 030.';


--
-- Name: vendor_branches_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.vendor_branches ALTER COLUMN id ADD GENERATED ALWAYS AS IDENTITY (
    SEQUENCE NAME public.vendor_branches_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: vendor_packages; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.vendor_packages (
    id bigint NOT NULL,
    vendor_branch_id bigint NOT NULL,
    code text NOT NULL,
    priority_class text CONSTRAINT vendor_packages_type_not_null NOT NULL,
    price numeric(20,2) NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: vendor_packages_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.vendor_packages ALTER COLUMN id ADD GENERATED ALWAYS AS IDENTITY (
    SEQUENCE NAME public.vendor_packages_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: vendor_request_items; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.vendor_request_items (
    id bigint NOT NULL,
    vendor_request_id bigint NOT NULL,
    terminal_id text NOT NULL,
    periode_pred date NOT NULL,
    denom integer NOT NULL,
    amount_replenish bigint DEFAULT 0 NOT NULL,
    amount_refund bigint DEFAULT 0 NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    brand text,
    lokasi_atm text,
    CONSTRAINT vendor_request_items_denom_chk CHECK ((denom > 0)),
    CONSTRAINT vendor_request_items_refund_chk CHECK ((amount_refund >= 0)),
    CONSTRAINT vendor_request_items_replenish_chk CHECK ((amount_replenish >= 0))
);


--
-- Name: TABLE vendor_request_items; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE public.vendor_request_items IS 'Line items of a vendor_request, each referencing an ATM/date/denom row from dmaa_atm_forecast.';


--
-- Name: COLUMN vendor_request_items.terminal_id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.vendor_request_items.terminal_id IS 'ATM identifier; maps to requirements.md "atm_id" and dmaa_atm_forecast.terminal_id. Labeled "ATM ID" in the API/frontend.';


--
-- Name: COLUMN vendor_request_items.brand; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.vendor_request_items.brand IS 'Manual-request brand (Req 3, Q4); NULL for DMAA-backed rows, which derive it at read time.';


--
-- Name: COLUMN vendor_request_items.lokasi_atm; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.vendor_request_items.lokasi_atm IS 'Manual-request ATM location (Req 3, Q4); NULL for DMAA-backed rows, which derive it at read time.';


--
-- Name: vendor_request_items_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.vendor_request_items ALTER COLUMN id ADD GENERATED BY DEFAULT AS IDENTITY (
    SEQUENCE NAME public.vendor_request_items_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: vendor_request_number_seq; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.vendor_request_number_seq (
    vendor_id bigint NOT NULL,
    seq_date date NOT NULL,
    last_seq integer DEFAULT 0 NOT NULL,
    CONSTRAINT vendor_request_number_seq_last_chk CHECK (((last_seq >= 0) AND (last_seq <= 999)))
);


--
-- Name: TABLE vendor_request_number_seq; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE public.vendor_request_number_seq IS 'Atomic per-(vendor, replenish_date) sequence backing the REP<prefix><YYYYMMDD><seq> request-number format (Req 4, Q3).';


--
-- Name: vendor_requests; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.vendor_requests (
    id bigint NOT NULL,
    request_number text NOT NULL,
    forecast_date date NOT NULL,
    status text DEFAULT 'draft'::text NOT NULL,
    notes text,
    created_by bigint NOT NULL,
    approved_by bigint,
    rejected_by bigint,
    rejection_reason text,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    submitted_at timestamp with time zone,
    approved_at timestamp with time zone,
    rejected_at timestamp with time zone,
    is_canceled boolean DEFAULT false NOT NULL,
    request_category text,
    replenish_date date,
    is_manual boolean DEFAULT false NOT NULL,
    vendor_id bigint,
    cancellation_reason text,
    CONSTRAINT vendor_requests_category_chk CHECK (((request_category IS NULL) OR (request_category = ANY (ARRAY['planned'::text, 'emergency'::text, 'additional'::text])))),
    CONSTRAINT vendor_requests_status_chk CHECK ((status = ANY (ARRAY['draft'::text, 'pending_approval'::text, 'approved'::text, 'rejected'::text, 'processing'::text, 'completed'::text, 'failed'::text, 'cancelled'::text])))
);


--
-- Name: TABLE vendor_requests; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE public.vendor_requests IS 'Cash replenishment request to a CIT vendor, composed from DMAA forecast rows; state machine + maker-checker.';


--
-- Name: COLUMN vendor_requests.status; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.vendor_requests.status IS 'draft | pending_approval | approved | rejected | processing | completed | failed | cancelled';


--
-- Name: COLUMN vendor_requests.is_canceled; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.vendor_requests.is_canceled IS 'Soft-cancel flag (Req 5); row/items are never deleted on cancel.';


--
-- Name: COLUMN vendor_requests.request_category; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.vendor_requests.request_category IS 'Manual_Request classification: planned | emergency | additional. NULL for legacy rows AND for standard non-manual (Forecast Browser) requests -- see design.md Q-resolution.';


--
-- Name: COLUMN vendor_requests.replenish_date; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.vendor_requests.replenish_date IS 'Vendor-facing delivery date chosen by the operator (Req 2/3), distinct from forecast_date. NULL only for legacy rows.';


--
-- Name: COLUMN vendor_requests.is_manual; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.vendor_requests.is_manual IS 'True when items were entered directly without a matching dmaa_atm_forecast row (Req 3).';


--
-- Name: COLUMN vendor_requests.vendor_id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.vendor_requests.vendor_id IS 'Single resolved CIT vendor this request is constrained to (Req 4, Q2); drives the request-number vendor prefix. NULL only for legacy VR-... rows.';


--
-- Name: COLUMN vendor_requests.cancellation_reason; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.vendor_requests.cancellation_reason IS 'Reason captured on cancel (Req 3.2/3.3). NULL for non-canceled rows and for rows canceled before this column existed (their reason lives only in audit_logs.after).';


--
-- Name: vendor_requests_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.vendor_requests ALTER COLUMN id ADD GENERATED BY DEFAULT AS IDENTITY (
    SEQUENCE NAME public.vendor_requests_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: vendor_vaults; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.vendor_vaults (
    id bigint NOT NULL,
    vendor_branch_id bigint NOT NULL,
    vault_code text NOT NULL,
    type text NOT NULL,
    amount numeric(20,2),
    max_capacity_amount numeric(20,2),
    min_capacity_amount numeric(20,2),
    currency_code character(3) NOT NULL,
    is_active boolean DEFAULT true NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: COLUMN vendor_vaults.type; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.vendor_vaults.type IS 'Type of vault (e.g. main, transit, buffer)';


--
-- Name: COLUMN vendor_vaults.currency_code; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.vendor_vaults.currency_code IS 'ISO 4217 currency code (e.g. IDR, USD)';


--
-- Name: vendor_vaults_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.vendor_vaults ALTER COLUMN id ADD GENERATED ALWAYS AS IDENTITY (
    SEQUENCE NAME public.vendor_vaults_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: vendors; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.vendors (
    id bigint NOT NULL,
    code text NOT NULL,
    name text NOT NULL,
    contact_email text,
    contact_phone text,
    hq_address text,
    is_active boolean DEFAULT true NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    deleted_at timestamp with time zone,
    request_prefix character(3),
    CONSTRAINT vendors_request_prefix_chk CHECK (((request_prefix IS NULL) OR (request_prefix ~ '^[A-Z]{3}$'::text)))
);


--
-- Name: COLUMN vendors.request_prefix; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.vendors.request_prefix IS '3-uppercase-char prefix embedded in vendor_requests.request_number (Req 4, Q1). NULL falls back to a deterministic service-side derivation from vendors.code.';


--
-- Name: vendors_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.vendors_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: vendors_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.vendors_id_seq OWNED BY public.vendors.id;


--
-- Name: approval_delegations id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.approval_delegations ALTER COLUMN id SET DEFAULT nextval('public.approval_delegations_id_seq'::regclass);


--
-- Name: approval_policies id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.approval_policies ALTER COLUMN id SET DEFAULT nextval('public.approval_policies_id_seq'::regclass);


--
-- Name: approval_requests id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.approval_requests ALTER COLUMN id SET DEFAULT nextval('public.approval_requests_id_seq'::regclass);


--
-- Name: approval_steps id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.approval_steps ALTER COLUMN id SET DEFAULT nextval('public.approval_steps_id_seq'::regclass);


--
-- Name: audit_logs id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.audit_logs ALTER COLUMN id SET DEFAULT nextval('public.audit_logs_id_seq'::regclass);


--
-- Name: roles id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.roles ALTER COLUMN id SET DEFAULT nextval('public.roles_id_seq'::regclass);


--
-- Name: user_leaves id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_leaves ALTER COLUMN id SET DEFAULT nextval('public.user_leaves_id_seq'::regclass);


--
-- Name: users id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.users ALTER COLUMN id SET DEFAULT nextval('public.users_id_seq'::regclass);


--
-- Name: vendors id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.vendors ALTER COLUMN id SET DEFAULT nextval('public.vendors_id_seq'::regclass);


--
-- Name: approval_delegations approval_delegations_no_overlap; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.approval_delegations
    ADD CONSTRAINT approval_delegations_no_overlap EXCLUDE USING gist (from_user_id WITH =, tstzrange(start_at, end_at, '[)'::text) WITH &&);


--
-- Name: approval_delegations approval_delegations_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.approval_delegations
    ADD CONSTRAINT approval_delegations_pkey PRIMARY KEY (id);


--
-- Name: approval_policies approval_policies_no_overlap; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.approval_policies
    ADD CONSTRAINT approval_policies_no_overlap EXCLUDE USING gist (document_type WITH =, numrange(min_amount, max_amount, '[)'::text) WITH &&) WHERE (is_active);


--
-- Name: approval_policies approval_policies_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.approval_policies
    ADD CONSTRAINT approval_policies_pkey PRIMARY KEY (id);


--
-- Name: approval_requests approval_requests_document_uq; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.approval_requests
    ADD CONSTRAINT approval_requests_document_uq UNIQUE (document_type, document_id);


--
-- Name: approval_requests approval_requests_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.approval_requests
    ADD CONSTRAINT approval_requests_pkey PRIMARY KEY (id);


--
-- Name: approval_steps approval_steps_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.approval_steps
    ADD CONSTRAINT approval_steps_pkey PRIMARY KEY (id);


--
-- Name: approval_steps approval_steps_request_level_uq; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.approval_steps
    ADD CONSTRAINT approval_steps_request_level_uq UNIQUE (request_id, step_level);


--
-- Name: atm_denoms atm_denoms_atm_denom_uq; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.atm_denoms
    ADD CONSTRAINT atm_denoms_atm_denom_uq UNIQUE (atm_id, denom_id);


--
-- Name: atm_denoms atm_denoms_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.atm_denoms
    ADD CONSTRAINT atm_denoms_pkey PRIMARY KEY (id);


--
-- Name: atm_vendor_packages atm_vendor_packages_atm_package_start_uq; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.atm_vendor_packages
    ADD CONSTRAINT atm_vendor_packages_atm_package_start_uq UNIQUE (atm_id, vendor_package_id, effective_start_date);


--
-- Name: atm_vendor_packages atm_vendor_packages_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.atm_vendor_packages
    ADD CONSTRAINT atm_vendor_packages_pkey PRIMARY KEY (id);


--
-- Name: atms atms_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.atms
    ADD CONSTRAINT atms_pkey PRIMARY KEY (id);


--
-- Name: atms atms_terminal_id_uq; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.atms
    ADD CONSTRAINT atms_terminal_id_uq UNIQUE (terminal_id);


--
-- Name: audit_logs audit_logs_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.audit_logs
    ADD CONSTRAINT audit_logs_pkey PRIMARY KEY (id);


--
-- Name: currencies currencies_code_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.currencies
    ADD CONSTRAINT currencies_code_key UNIQUE (code);


--
-- Name: currencies currencies_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.currencies
    ADD CONSTRAINT currencies_pkey PRIMARY KEY (id);


--
-- Name: denoms denoms_curr_id_denom_uq; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.denoms
    ADD CONSTRAINT denoms_curr_id_denom_uq UNIQUE (curr_id, denom);


--
-- Name: denoms denoms_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.denoms
    ADD CONSTRAINT denoms_pkey PRIMARY KEY (id);


--
-- Name: dmaa_files dmaa_files_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.dmaa_files
    ADD CONSTRAINT dmaa_files_pkey PRIMARY KEY (id);


--
-- Name: dsr_daily_rows dsr_daily_rows_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.dsr_daily_rows
    ADD CONSTRAINT dsr_daily_rows_pkey PRIMARY KEY (id);


--
-- Name: dsr_daily_rows dsr_daily_rows_upload_row_uq; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.dsr_daily_rows
    ADD CONSTRAINT dsr_daily_rows_upload_row_uq UNIQUE (upload_id, row_no);


--
-- Name: dsr_rencana_isi_rows dsr_rencana_isi_rows_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.dsr_rencana_isi_rows
    ADD CONSTRAINT dsr_rencana_isi_rows_pkey PRIMARY KEY (id);


--
-- Name: dsr_rencana_isi_rows dsr_rencana_isi_rows_upload_row_uq; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.dsr_rencana_isi_rows
    ADD CONSTRAINT dsr_rencana_isi_rows_upload_row_uq UNIQUE (upload_id, row_no);


--
-- Name: dsr_uploads dsr_uploads_checksum_uq; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.dsr_uploads
    ADD CONSTRAINT dsr_uploads_checksum_uq UNIQUE (checksum);


--
-- Name: dsr_uploads dsr_uploads_date_vendor_uq; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.dsr_uploads
    ADD CONSTRAINT dsr_uploads_date_vendor_uq UNIQUE (report_date, vendor);


--
-- Name: dsr_uploads dsr_uploads_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.dsr_uploads
    ADD CONSTRAINT dsr_uploads_pkey PRIMARY KEY (id);


--
-- Name: itm_cashpos_files itm_cashpos_files_checksum_uq; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.itm_cashpos_files
    ADD CONSTRAINT itm_cashpos_files_checksum_uq UNIQUE (checksum);


--
-- Name: itm_cashpos_files itm_cashpos_files_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.itm_cashpos_files
    ADD CONSTRAINT itm_cashpos_files_pkey PRIMARY KEY (id);


--
-- Name: itm_cashpos itm_cashpos_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.itm_cashpos
    ADD CONSTRAINT itm_cashpos_pkey PRIMARY KEY (id);


--
-- Name: itm_replenish_files itm_replenish_files_checksum_uq; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.itm_replenish_files
    ADD CONSTRAINT itm_replenish_files_checksum_uq UNIQUE (checksum);


--
-- Name: itm_replenish_files itm_replenish_files_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.itm_replenish_files
    ADD CONSTRAINT itm_replenish_files_pkey PRIMARY KEY (id);


--
-- Name: itm_replenish itm_replenish_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.itm_replenish
    ADD CONSTRAINT itm_replenish_pkey PRIMARY KEY (id);


--
-- Name: locations locations_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.locations
    ADD CONSTRAINT locations_pkey PRIMARY KEY (id);


--
-- Name: menu_features menu_features_key_uq; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.menu_features
    ADD CONSTRAINT menu_features_key_uq UNIQUE (key);


--
-- Name: menu_features menu_features_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.menu_features
    ADD CONSTRAINT menu_features_pkey PRIMARY KEY (id);


--
-- Name: dmaa_atm_forecast pk_dmaa_atm_forecast; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.dmaa_atm_forecast
    ADD CONSTRAINT pk_dmaa_atm_forecast PRIMARY KEY (dmaa_file_id, terminal_id, periode_pred, denom);


--
-- Name: regions regions_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.regions
    ADD CONSTRAINT regions_pkey PRIMARY KEY (id);


--
-- Name: role_permissions role_permissions_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.role_permissions
    ADD CONSTRAINT role_permissions_pkey PRIMARY KEY (id);


--
-- Name: role_permissions role_permissions_role_feature_uq; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.role_permissions
    ADD CONSTRAINT role_permissions_role_feature_uq UNIQUE (role_id, menu_feature_id);


--
-- Name: roles roles_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.roles
    ADD CONSTRAINT roles_pkey PRIMARY KEY (id);


--
-- Name: roles roles_role_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.roles
    ADD CONSTRAINT roles_role_key UNIQUE (role);


--
-- Name: dmaa_files uq_dmaa_files_checksum; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.dmaa_files
    ADD CONSTRAINT uq_dmaa_files_checksum UNIQUE (checksum);


--
-- Name: regions uq_regions_code; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.regions
    ADD CONSTRAINT uq_regions_code UNIQUE (code);


--
-- Name: vendor_branches uq_vendor_branches_branch_code; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.vendor_branches
    ADD CONSTRAINT uq_vendor_branches_branch_code UNIQUE (branch_code);


--
-- Name: user_leaves user_leaves_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_leaves
    ADD CONSTRAINT user_leaves_pkey PRIMARY KEY (id);


--
-- Name: users users_email_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.users
    ADD CONSTRAINT users_email_key UNIQUE (email);


--
-- Name: users users_employee_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.users
    ADD CONSTRAINT users_employee_id_key UNIQUE (employee_id);


--
-- Name: users users_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.users
    ADD CONSTRAINT users_pkey PRIMARY KEY (id);


--
-- Name: users users_username_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.users
    ADD CONSTRAINT users_username_key UNIQUE (username);


--
-- Name: vendor_branches vendor_branches_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.vendor_branches
    ADD CONSTRAINT vendor_branches_pkey PRIMARY KEY (id);


--
-- Name: vendor_packages vendor_packages_branch_code_uq; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.vendor_packages
    ADD CONSTRAINT vendor_packages_branch_code_uq UNIQUE (vendor_branch_id, code);


--
-- Name: vendor_packages vendor_packages_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.vendor_packages
    ADD CONSTRAINT vendor_packages_pkey PRIMARY KEY (id);


--
-- Name: vendor_request_items vendor_request_items_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.vendor_request_items
    ADD CONSTRAINT vendor_request_items_pkey PRIMARY KEY (id);


--
-- Name: vendor_request_items vendor_request_items_uq; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.vendor_request_items
    ADD CONSTRAINT vendor_request_items_uq UNIQUE (vendor_request_id, terminal_id, periode_pred, denom);


--
-- Name: vendor_request_number_seq vendor_request_number_seq_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.vendor_request_number_seq
    ADD CONSTRAINT vendor_request_number_seq_pkey PRIMARY KEY (vendor_id, seq_date);


--
-- Name: vendor_requests vendor_requests_number_uq; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.vendor_requests
    ADD CONSTRAINT vendor_requests_number_uq UNIQUE (request_number);


--
-- Name: vendor_requests vendor_requests_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.vendor_requests
    ADD CONSTRAINT vendor_requests_pkey PRIMARY KEY (id);


--
-- Name: vendor_vaults vendor_vaults_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.vendor_vaults
    ADD CONSTRAINT vendor_vaults_pkey PRIMARY KEY (id);


--
-- Name: vendor_vaults vendor_vaults_vault_code_uq; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.vendor_vaults
    ADD CONSTRAINT vendor_vaults_vault_code_uq UNIQUE (vault_code);


--
-- Name: vendors vendors_code_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.vendors
    ADD CONSTRAINT vendors_code_key UNIQUE (code);


--
-- Name: vendors vendors_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.vendors
    ADD CONSTRAINT vendors_pkey PRIMARY KEY (id);


--
-- Name: approval_delegations_from_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX approval_delegations_from_idx ON public.approval_delegations USING btree (from_user_id);


--
-- Name: approval_delegations_to_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX approval_delegations_to_idx ON public.approval_delegations USING btree (to_user_id);


--
-- Name: approval_policies_document_type_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX approval_policies_document_type_idx ON public.approval_policies USING btree (document_type) WHERE is_active;


--
-- Name: approval_requests_maker_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX approval_requests_maker_idx ON public.approval_requests USING btree (maker_id);


--
-- Name: approval_requests_status_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX approval_requests_status_idx ON public.approval_requests USING btree (status);


--
-- Name: approval_steps_assigned_approver_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX approval_steps_assigned_approver_idx ON public.approval_steps USING btree (assigned_approver_id);


--
-- Name: approval_steps_request_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX approval_steps_request_idx ON public.approval_steps USING btree (request_id);


--
-- Name: atm_denoms_atm_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX atm_denoms_atm_idx ON public.atm_denoms USING btree (atm_id);


--
-- Name: atm_denoms_denom_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX atm_denoms_denom_idx ON public.atm_denoms USING btree (denom_id);


--
-- Name: atm_vendor_packages_atm_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX atm_vendor_packages_atm_idx ON public.atm_vendor_packages USING btree (atm_id);


--
-- Name: atm_vendor_packages_vendor_package_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX atm_vendor_packages_vendor_package_idx ON public.atm_vendor_packages USING btree (vendor_package_id);


--
-- Name: atms_location_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX atms_location_idx ON public.atms USING btree (location_id);


--
-- Name: atms_terminal_id_trgm_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX atms_terminal_id_trgm_idx ON public.atms USING gin (terminal_id public.gin_trgm_ops);


--
-- Name: audit_logs_actor_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX audit_logs_actor_idx ON public.audit_logs USING btree (actor_id);


--
-- Name: audit_logs_entity_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX audit_logs_entity_idx ON public.audit_logs USING btree (entity_type, entity_id);


--
-- Name: denoms_curr_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX denoms_curr_id_idx ON public.denoms USING btree (curr_id);


--
-- Name: dmaa_atm_forecast_terminal_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX dmaa_atm_forecast_terminal_id_idx ON public.dmaa_atm_forecast USING btree (terminal_id);


--
-- Name: idx_dsr_daily_rows_section_flow; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_dsr_daily_rows_section_flow ON public.dsr_daily_rows USING btree (upload_id, section, flow);


--
-- Name: idx_dsr_daily_rows_upload; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_dsr_daily_rows_upload ON public.dsr_daily_rows USING btree (upload_id);


--
-- Name: idx_dsr_rencana_isi_rows_atm; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_dsr_rencana_isi_rows_atm ON public.dsr_rencana_isi_rows USING btree (atm_id);


--
-- Name: idx_dsr_rencana_isi_rows_terminal; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_dsr_rencana_isi_rows_terminal ON public.dsr_rencana_isi_rows USING btree (atm_terminal_id);


--
-- Name: idx_dsr_rencana_isi_rows_upload; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_dsr_rencana_isi_rows_upload ON public.dsr_rencana_isi_rows USING btree (upload_id);


--
-- Name: itm_cashpos_branch_code_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX itm_cashpos_branch_code_idx ON public.itm_cashpos USING btree (branch_code);


--
-- Name: itm_cashpos_cashpos_date_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX itm_cashpos_cashpos_date_idx ON public.itm_cashpos USING btree (cashpos_date);


--
-- Name: itm_cashpos_file_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX itm_cashpos_file_idx ON public.itm_cashpos USING btree (file_id);


--
-- Name: itm_cashpos_terminal_date_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX itm_cashpos_terminal_date_idx ON public.itm_cashpos USING btree (terminal_id, cashpos_date);


--
-- Name: itm_replenish_branch_code_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX itm_replenish_branch_code_idx ON public.itm_replenish USING btree (branch_code);


--
-- Name: itm_replenish_file_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX itm_replenish_file_idx ON public.itm_replenish USING btree (file_id);


--
-- Name: itm_replenish_replenish_date_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX itm_replenish_replenish_date_idx ON public.itm_replenish USING btree (replenish_date);


--
-- Name: itm_replenish_terminal_date_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX itm_replenish_terminal_date_idx ON public.itm_replenish USING btree (terminal_id, replenish_date);


--
-- Name: locations_region_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX locations_region_idx ON public.locations USING btree (region_id);


--
-- Name: menu_features_parent_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX menu_features_parent_idx ON public.menu_features USING btree (parent_id);


--
-- Name: role_permissions_feature_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX role_permissions_feature_idx ON public.role_permissions USING btree (menu_feature_id);


--
-- Name: role_permissions_role_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX role_permissions_role_idx ON public.role_permissions USING btree (role_id);


--
-- Name: user_leaves_user_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX user_leaves_user_idx ON public.user_leaves USING btree (user_id);


--
-- Name: users_role_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX users_role_idx ON public.users USING btree (role_id);


--
-- Name: users_supervisor_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX users_supervisor_idx ON public.users USING btree (supervisor_id);


--
-- Name: users_vendor_branch_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX users_vendor_branch_idx ON public.users USING btree (vendor_branch_id);


--
-- Name: users_vendor_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX users_vendor_idx ON public.users USING btree (vendor_id);


--
-- Name: vendor_branches_location_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX vendor_branches_location_idx ON public.vendor_branches USING btree (location_id);


--
-- Name: vendor_branches_vendor_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX vendor_branches_vendor_idx ON public.vendor_branches USING btree (vendor_id);


--
-- Name: vendor_packages_vendor_branch_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX vendor_packages_vendor_branch_idx ON public.vendor_packages USING btree (vendor_branch_id);


--
-- Name: vendor_request_items_request_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX vendor_request_items_request_idx ON public.vendor_request_items USING btree (vendor_request_id);


--
-- Name: vendor_request_items_terminal_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX vendor_request_items_terminal_idx ON public.vendor_request_items USING btree (terminal_id, periode_pred);


--
-- Name: vendor_requests_created_by_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX vendor_requests_created_by_idx ON public.vendor_requests USING btree (created_by);


--
-- Name: vendor_requests_forecast_date_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX vendor_requests_forecast_date_idx ON public.vendor_requests USING btree (forecast_date);


--
-- Name: vendor_requests_is_canceled_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX vendor_requests_is_canceled_idx ON public.vendor_requests USING btree (is_canceled);


--
-- Name: vendor_requests_replenish_date_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX vendor_requests_replenish_date_idx ON public.vendor_requests USING btree (replenish_date);


--
-- Name: vendor_requests_status_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX vendor_requests_status_idx ON public.vendor_requests USING btree (status);


--
-- Name: vendor_requests_vendor_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX vendor_requests_vendor_id_idx ON public.vendor_requests USING btree (vendor_id);


--
-- Name: vendor_vaults_vendor_branch_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX vendor_vaults_vendor_branch_idx ON public.vendor_vaults USING btree (vendor_branch_id);


--
-- Name: atm_vendor_packages trg_atm_vendor_packages_set_updated_at; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER trg_atm_vendor_packages_set_updated_at BEFORE UPDATE ON public.atm_vendor_packages FOR EACH ROW WHEN ((old.* IS DISTINCT FROM new.*)) EXECUTE FUNCTION public.set_updated_at();


--
-- Name: atms trg_atms_set_updated_at; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER trg_atms_set_updated_at BEFORE UPDATE ON public.atms FOR EACH ROW WHEN ((old.* IS DISTINCT FROM new.*)) EXECUTE FUNCTION public.set_updated_at();


--
-- Name: currencies trg_currencies_set_updated_at; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER trg_currencies_set_updated_at BEFORE UPDATE ON public.currencies FOR EACH ROW WHEN ((old.* IS DISTINCT FROM new.*)) EXECUTE FUNCTION public.set_updated_at();


--
-- Name: denoms trg_denoms_set_updated_at; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER trg_denoms_set_updated_at BEFORE UPDATE ON public.denoms FOR EACH ROW WHEN ((old.* IS DISTINCT FROM new.*)) EXECUTE FUNCTION public.set_updated_at();


--
-- Name: dmaa_files trg_dmaa_files_set_updated_at; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER trg_dmaa_files_set_updated_at BEFORE UPDATE ON public.dmaa_files FOR EACH ROW WHEN ((old.* IS DISTINCT FROM new.*)) EXECUTE FUNCTION public.set_updated_at();


--
-- Name: itm_cashpos_files trg_itm_cashpos_files_set_updated_at; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER trg_itm_cashpos_files_set_updated_at BEFORE UPDATE ON public.itm_cashpos_files FOR EACH ROW WHEN ((old.* IS DISTINCT FROM new.*)) EXECUTE FUNCTION public.set_updated_at();


--
-- Name: itm_replenish_files trg_itm_replenish_files_set_updated_at; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER trg_itm_replenish_files_set_updated_at BEFORE UPDATE ON public.itm_replenish_files FOR EACH ROW WHEN ((old.* IS DISTINCT FROM new.*)) EXECUTE FUNCTION public.set_updated_at();


--
-- Name: locations trg_locations_set_updated_at; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER trg_locations_set_updated_at BEFORE UPDATE ON public.locations FOR EACH ROW WHEN ((old.* IS DISTINCT FROM new.*)) EXECUTE FUNCTION public.set_updated_at();


--
-- Name: regions trg_regions_set_updated_at; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER trg_regions_set_updated_at BEFORE UPDATE ON public.regions FOR EACH ROW WHEN ((old.* IS DISTINCT FROM new.*)) EXECUTE FUNCTION public.set_updated_at();


--
-- Name: roles trg_roles_set_updated_at; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER trg_roles_set_updated_at BEFORE UPDATE ON public.roles FOR EACH ROW WHEN ((old.* IS DISTINCT FROM new.*)) EXECUTE FUNCTION public.set_updated_at();


--
-- Name: users trg_users_set_updated_at; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER trg_users_set_updated_at BEFORE UPDATE ON public.users FOR EACH ROW WHEN ((old.* IS DISTINCT FROM new.*)) EXECUTE FUNCTION public.set_updated_at();


--
-- Name: vendor_branches trg_vendor_branches_set_updated_at; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER trg_vendor_branches_set_updated_at BEFORE UPDATE ON public.vendor_branches FOR EACH ROW WHEN ((old.* IS DISTINCT FROM new.*)) EXECUTE FUNCTION public.set_updated_at();


--
-- Name: vendor_packages trg_vendor_packages_set_updated_at; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER trg_vendor_packages_set_updated_at BEFORE UPDATE ON public.vendor_packages FOR EACH ROW WHEN ((old.* IS DISTINCT FROM new.*)) EXECUTE FUNCTION public.set_updated_at();


--
-- Name: vendor_requests trg_vendor_requests_set_updated_at; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER trg_vendor_requests_set_updated_at BEFORE UPDATE ON public.vendor_requests FOR EACH ROW WHEN ((old.* IS DISTINCT FROM new.*)) EXECUTE FUNCTION public.set_updated_at();


--
-- Name: vendor_vaults trg_vendor_vaults_set_updated_at; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER trg_vendor_vaults_set_updated_at BEFORE UPDATE ON public.vendor_vaults FOR EACH ROW WHEN ((old.* IS DISTINCT FROM new.*)) EXECUTE FUNCTION public.set_updated_at();


--
-- Name: vendors trg_vendors_set_updated_at; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER trg_vendors_set_updated_at BEFORE UPDATE ON public.vendors FOR EACH ROW WHEN ((old.* IS DISTINCT FROM new.*)) EXECUTE FUNCTION public.set_updated_at();


--
-- Name: approval_delegations approval_delegations_from_fk; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.approval_delegations
    ADD CONSTRAINT approval_delegations_from_fk FOREIGN KEY (from_user_id) REFERENCES public.users(id);


--
-- Name: approval_delegations approval_delegations_to_fk; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.approval_delegations
    ADD CONSTRAINT approval_delegations_to_fk FOREIGN KEY (to_user_id) REFERENCES public.users(id);


--
-- Name: approval_requests approval_requests_maker_fk; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.approval_requests
    ADD CONSTRAINT approval_requests_maker_fk FOREIGN KEY (maker_id) REFERENCES public.users(id);


--
-- Name: approval_steps approval_steps_acted_by_fk; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.approval_steps
    ADD CONSTRAINT approval_steps_acted_by_fk FOREIGN KEY (acted_by_id) REFERENCES public.users(id);


--
-- Name: approval_steps approval_steps_assigned_approver_fk; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.approval_steps
    ADD CONSTRAINT approval_steps_assigned_approver_fk FOREIGN KEY (assigned_approver_id) REFERENCES public.users(id);


--
-- Name: approval_steps approval_steps_request_fk; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.approval_steps
    ADD CONSTRAINT approval_steps_request_fk FOREIGN KEY (request_id) REFERENCES public.approval_requests(id);


--
-- Name: audit_logs audit_logs_actor_fk; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.audit_logs
    ADD CONSTRAINT audit_logs_actor_fk FOREIGN KEY (actor_id) REFERENCES public.users(id);


--
-- Name: denoms denoms_currencies_fk; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.denoms
    ADD CONSTRAINT denoms_currencies_fk FOREIGN KEY (curr_id) REFERENCES public.currencies(id);


--
-- Name: atm_denoms fk_atm_denoms_atm; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.atm_denoms
    ADD CONSTRAINT fk_atm_denoms_atm FOREIGN KEY (atm_id) REFERENCES public.atms(id) ON DELETE CASCADE;


--
-- Name: atm_denoms fk_atm_denoms_denom; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.atm_denoms
    ADD CONSTRAINT fk_atm_denoms_denom FOREIGN KEY (denom_id) REFERENCES public.denoms(id);


--
-- Name: atm_vendor_packages fk_atm_vendor_packages_atm; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.atm_vendor_packages
    ADD CONSTRAINT fk_atm_vendor_packages_atm FOREIGN KEY (atm_id) REFERENCES public.atms(id) ON DELETE CASCADE;


--
-- Name: atm_vendor_packages fk_atm_vendor_packages_vendor_package; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.atm_vendor_packages
    ADD CONSTRAINT fk_atm_vendor_packages_vendor_package FOREIGN KEY (vendor_package_id) REFERENCES public.vendor_packages(id);


--
-- Name: atms fk_atms_location; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.atms
    ADD CONSTRAINT fk_atms_location FOREIGN KEY (location_id) REFERENCES public.locations(id);


--
-- Name: dmaa_atm_forecast fk_dmaa_atm_forecast_dmaa_file; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.dmaa_atm_forecast
    ADD CONSTRAINT fk_dmaa_atm_forecast_dmaa_file FOREIGN KEY (dmaa_file_id) REFERENCES public.dmaa_files(id) ON UPDATE CASCADE ON DELETE RESTRICT;


--
-- Name: dmaa_atm_forecast fk_dmaa_atm_forecast_terminal; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.dmaa_atm_forecast
    ADD CONSTRAINT fk_dmaa_atm_forecast_terminal FOREIGN KEY (terminal_id) REFERENCES public.atms(terminal_id) ON UPDATE CASCADE ON DELETE RESTRICT;


--
-- Name: dsr_daily_rows fk_dsr_daily_rows_upload; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.dsr_daily_rows
    ADD CONSTRAINT fk_dsr_daily_rows_upload FOREIGN KEY (upload_id) REFERENCES public.dsr_uploads(id) ON DELETE CASCADE;


--
-- Name: dsr_rencana_isi_rows fk_dsr_rencana_isi_rows_atm; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.dsr_rencana_isi_rows
    ADD CONSTRAINT fk_dsr_rencana_isi_rows_atm FOREIGN KEY (atm_id) REFERENCES public.atms(id) ON DELETE SET NULL;


--
-- Name: dsr_rencana_isi_rows fk_dsr_rencana_isi_rows_upload; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.dsr_rencana_isi_rows
    ADD CONSTRAINT fk_dsr_rencana_isi_rows_upload FOREIGN KEY (upload_id) REFERENCES public.dsr_uploads(id) ON DELETE CASCADE;


--
-- Name: dsr_uploads fk_dsr_uploads_uploaded_by; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.dsr_uploads
    ADD CONSTRAINT fk_dsr_uploads_uploaded_by FOREIGN KEY (uploaded_by_user_id) REFERENCES public.users(id) ON DELETE SET NULL;


--
-- Name: itm_cashpos fk_itm_cashpos_file; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.itm_cashpos
    ADD CONSTRAINT fk_itm_cashpos_file FOREIGN KEY (file_id) REFERENCES public.itm_cashpos_files(id) ON DELETE CASCADE;


--
-- Name: itm_replenish fk_itm_replenish_file; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.itm_replenish
    ADD CONSTRAINT fk_itm_replenish_file FOREIGN KEY (file_id) REFERENCES public.itm_replenish_files(id) ON DELETE CASCADE;


--
-- Name: locations fk_locations_region; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.locations
    ADD CONSTRAINT fk_locations_region FOREIGN KEY (region_id) REFERENCES public.regions(id);


--
-- Name: vendor_branches fk_vendor_branches_location; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.vendor_branches
    ADD CONSTRAINT fk_vendor_branches_location FOREIGN KEY (location_id) REFERENCES public.locations(id);


--
-- Name: vendor_branches fk_vendor_branches_vendor; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.vendor_branches
    ADD CONSTRAINT fk_vendor_branches_vendor FOREIGN KEY (vendor_id) REFERENCES public.vendors(id);


--
-- Name: vendor_packages fk_vendor_packages_vendor_branch; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.vendor_packages
    ADD CONSTRAINT fk_vendor_packages_vendor_branch FOREIGN KEY (vendor_branch_id) REFERENCES public.vendor_branches(id);


--
-- Name: vendor_vaults fk_vendor_vaults_vendor_branch; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.vendor_vaults
    ADD CONSTRAINT fk_vendor_vaults_vendor_branch FOREIGN KEY (vendor_branch_id) REFERENCES public.vendor_branches(id);


--
-- Name: menu_features menu_features_parent_fk; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.menu_features
    ADD CONSTRAINT menu_features_parent_fk FOREIGN KEY (parent_id) REFERENCES public.menu_features(id) ON DELETE RESTRICT;


--
-- Name: role_permissions role_permissions_feature_fk; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.role_permissions
    ADD CONSTRAINT role_permissions_feature_fk FOREIGN KEY (menu_feature_id) REFERENCES public.menu_features(id) ON DELETE CASCADE;


--
-- Name: role_permissions role_permissions_granted_by_fk; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.role_permissions
    ADD CONSTRAINT role_permissions_granted_by_fk FOREIGN KEY (granted_by) REFERENCES public.users(id);


--
-- Name: role_permissions role_permissions_role_fk; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.role_permissions
    ADD CONSTRAINT role_permissions_role_fk FOREIGN KEY (role_id) REFERENCES public.roles(id) ON DELETE CASCADE;


--
-- Name: user_leaves user_leaves_user_fk; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_leaves
    ADD CONSTRAINT user_leaves_user_fk FOREIGN KEY (user_id) REFERENCES public.users(id);


--
-- Name: users users_role_fk; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.users
    ADD CONSTRAINT users_role_fk FOREIGN KEY (role_id) REFERENCES public.roles(id);


--
-- Name: users users_supervisor_fk; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.users
    ADD CONSTRAINT users_supervisor_fk FOREIGN KEY (supervisor_id) REFERENCES public.users(id);


--
-- Name: users users_vendor_branch_fk; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.users
    ADD CONSTRAINT users_vendor_branch_fk FOREIGN KEY (vendor_branch_id) REFERENCES public.vendor_branches(id);


--
-- Name: users users_vendor_fk; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.users
    ADD CONSTRAINT users_vendor_fk FOREIGN KEY (vendor_id) REFERENCES public.vendors(id);


--
-- Name: vendor_request_items vendor_request_items_request_fk; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.vendor_request_items
    ADD CONSTRAINT vendor_request_items_request_fk FOREIGN KEY (vendor_request_id) REFERENCES public.vendor_requests(id) ON DELETE CASCADE;


--
-- Name: vendor_request_number_seq vendor_request_number_seq_vendor_fk; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.vendor_request_number_seq
    ADD CONSTRAINT vendor_request_number_seq_vendor_fk FOREIGN KEY (vendor_id) REFERENCES public.vendors(id) ON DELETE RESTRICT;


--
-- Name: vendor_requests vendor_requests_approved_by_fk; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.vendor_requests
    ADD CONSTRAINT vendor_requests_approved_by_fk FOREIGN KEY (approved_by) REFERENCES public.users(id) ON DELETE RESTRICT;


--
-- Name: vendor_requests vendor_requests_created_by_fk; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.vendor_requests
    ADD CONSTRAINT vendor_requests_created_by_fk FOREIGN KEY (created_by) REFERENCES public.users(id) ON DELETE RESTRICT;


--
-- Name: vendor_requests vendor_requests_rejected_by_fk; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.vendor_requests
    ADD CONSTRAINT vendor_requests_rejected_by_fk FOREIGN KEY (rejected_by) REFERENCES public.users(id) ON DELETE RESTRICT;


--
-- Name: vendor_requests vendor_requests_vendor_fk; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.vendor_requests
    ADD CONSTRAINT vendor_requests_vendor_fk FOREIGN KEY (vendor_id) REFERENCES public.vendors(id) ON DELETE RESTRICT;


--
-- PostgreSQL database dump complete
--


