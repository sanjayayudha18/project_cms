-- 028: comment-only follow-up to 027 (cit-acm-plan S2). 027 added flow 'saldo_akhir' to
-- dsr_daily_rows_flow_chk but left the column comment listing the four original values.
-- No data or constraint change.
BEGIN;

COMMENT ON COLUMN public.dsr_daily_rows.flow IS 'saldo_awal = opening balance (d0 only). penerimaan / pengeluaran = receipt / disbursement line, sign stored verbatim as printed. status_cadangan = STATUS UANG CADANGAN ATM lines (Layak Edar / Rusak), d1 only. saldo_akhir = closing balance per vault block (d0, located blocks only; since 027, feeds vault saldo via dsr_location_vault_maps).';

COMMIT;
