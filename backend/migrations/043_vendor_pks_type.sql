-- 043_vendor_pks_type.sql
-- PKS per type (FLM/CIT) + No. WOD (.kiro/specs/vendor-pks-type/, requirements accepted PO 2026-11-18,
-- design accepted engineer 2026-11-18; task 1.1). Req 1.8, 1.10, 2.7, 2.8, 2.9, 9.1, 9.2.
--
--   1. Guard: berhenti bila vendor_pks berisi data atau ada change vendor_pks pending (kolom baru NOT NULL tanpa
--      nilai yang bisa ditebak; change pending lama tidak punya pks_type_id).
--   2. vendor_pks_types: master type PKS (maker-checker, entity_type vendor_pks_type), soft-disable saja.
--      Seed FLM + CIT aktif.
--   3. vendor_pks.pks_type_id (FK RESTRICT) + wod_number (trim, 1-100, unik global case-insensitive).
--      Keduanya tidak pernah di-UPDATE (Req 2.9; dijaga di applier).
--   4. vendor_pks_active_uq: satu PKS aktif per (vendor, type) -- menggantikan per vendor (022).
--   5. master_data_change_requests: pending vendor_pks per (vendor, type) -- menggantikan D24 per vendor (022);
--      pending create unik per WOD; pending create vendor_pks_type unik per code.
--   Tidak berubah: vendors, vendor_pks_status(), vendor_pks_validity_months(), export_jobs (kind vendor_pks dari 037).
--
-- SAFETY (Sec 4 #7: migrasi skema):
--   * Guard menjamin vendor_pks kosong, jadi ADD COLUMN NOT NULL tanpa default aman dan index dibuat inline
--     (tabel kosong; master_data_change_requests kecil, pola 022). Satu tx.
--   * master_data_change_requests tidak punya CHECK pada entity_type (003): vendor_pks_type cukup didaftarkan
--     di registry applier.
--   * Forward-only: no down migration (project convention). Rollback dev =
--     backend/scripts/dev/rollback_043_vendor_pks_type.sql (task 1.2), aman selama data PKS kosong.
BEGIN;

-- 1. Guard (Req 9.2) ---------------------------------------------------------------------------------------------------
DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM public.vendor_pks) THEN
    RAISE EXCEPTION '043: vendor_pks berisi data; isi pks_type/wod_number dulu (tidak ditebak)';
  END IF;
  IF EXISTS (SELECT 1 FROM public.master_data_change_requests
             WHERE entity_type = 'vendor_pks' AND status = 'pending') THEN
    RAISE EXCEPTION '043: ada change vendor_pks pending tanpa pks_type_id; selesaikan dulu';
  END IF;
END
$$;

-- 2. vendor_pks_types (Req 1.8, 1.10) ----------------------------------------------------------------------------------
CREATE TABLE public.vendor_pks_types (
    id         bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    code       text NOT NULL CONSTRAINT vendor_pks_types_code_chk CHECK (code ~ '^[A-Z0-9]{2,10}$'),
    name       text NOT NULL CONSTRAINT vendor_pks_types_name_chk CHECK (char_length(btrim(name)) BETWEEN 1 AND 100),
    is_active  boolean NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX vendor_pks_types_code_uq ON public.vendor_pks_types (upper(code));

CREATE TRIGGER trg_vendor_pks_types_set_updated_at
    BEFORE UPDATE ON public.vendor_pks_types
    FOR EACH ROW WHEN ((old.* IS DISTINCT FROM new.*))
    EXECUTE FUNCTION public.set_updated_at();

INSERT INTO public.vendor_pks_types (code, name) VALUES ('FLM', 'FLM'), ('CIT', 'CIT');

COMMENT ON TABLE public.vendor_pks_types IS
    'Master type PKS (FLM, CIT, ...). Ditulis hanya lewat maker-checker master data (entity_type vendor_pks_type, ADMIN/ADMIN_PARAM). Tidak pernah di-hard-delete: disable = is_active false, diblok selama ada vendor_pks aktif (replaced_at IS NULL, termasuk terminated) dengan type ini. Bukan vendors.kind. Migrasi 043.';
COMMENT ON COLUMN public.vendor_pks_types.code IS
    'Kode type, ^[A-Z0-9]{2,10}$, unik (upper). Tidak berubah setelah dibuat; update hanya name.';
COMMENT ON COLUMN public.vendor_pks_types.name IS
    'Nama tampilan type, 1-100 karakter setelah trim.';
COMMENT ON COLUMN public.vendor_pks_types.is_active IS
    'false = type nonaktif: tidak bisa dipilih untuk PKS baru dan tidak tampil sebagai kolom/filter daftar vendor.';

-- 3. vendor_pks: type + No. WOD (Req 2.7, 2.9) -------------------------------------------------------------------------
ALTER TABLE public.vendor_pks
    ADD COLUMN pks_type_id bigint NOT NULL REFERENCES public.vendor_pks_types(id) ON DELETE RESTRICT,
    ADD COLUMN wod_number  text   NOT NULL
        CONSTRAINT vendor_pks_wod_chk CHECK (wod_number = btrim(wod_number) AND char_length(wod_number) BETWEEN 1 AND 100);

-- 4. Index vendor_pks (Req 2.7) ----------------------------------------------------------------------------------------
DROP INDEX public.vendor_pks_active_uq;
CREATE UNIQUE INDEX vendor_pks_active_uq
    ON public.vendor_pks (vendor_id, pks_type_id)
    WHERE replaced_at IS NULL;

CREATE UNIQUE INDEX vendor_pks_wod_uq ON public.vendor_pks (lower(wod_number));

-- Cek in-use saat disable type + FK pks_type_id.
CREATE INDEX vendor_pks_type_active_idx
    ON public.vendor_pks (pks_type_id)
    WHERE replaced_at IS NULL;

COMMENT ON TABLE public.vendor_pks IS
    'PKS (perjanjian kerja sama) per vendor per type dengan riwayat (D1; 043). Satu PKS aktif per (vendor, type) (replaced_at IS NULL, vendor_pks_active_uq); vendor boleh punya subset type. Ditulis hanya lewat maker-checker master data (entity_type vendor_pks). pks_name/effective_date/expiry_date/pks_type_id/wod_number tidak pernah di-UPDATE; tidak pernah di-hard-delete. Riwayat batalkan terminate hanya di audit_logs.';
COMMENT ON COLUMN public.vendor_pks.replaced_at IS
    'NULL = PKS aktif vendor untuk type ini. Diisi saat PKS baru dengan type sama disetujui dan menggantikannya (D17; 043).';
COMMENT ON COLUMN public.vendor_pks.pks_type_id IS
    'Type PKS (vendor_pks_types). Wajib; tidak pernah di-UPDATE (Req 2.9). Migrasi 043.';
COMMENT ON COLUMN public.vendor_pks.wod_number IS
    'No. WOD, teks bebas 1-100 tanpa spasi tepi. Unik global case-insensitive termasuk riwayat (vendor_pks_wod_uq); tidak pernah di-UPDATE (Req 2.9). Migrasi 043.';

-- 5. Pending change (Req 2.8, menggantikan D24 vendor-pks) -------------------------------------------------------------
DROP INDEX public.master_data_change_requests_vendor_pks_pending_uq;
CREATE UNIQUE INDEX master_data_change_requests_vendor_pks_pending_uq
    ON public.master_data_change_requests (((payload->>'vendor_id')::bigint), ((payload->>'pks_type_id')::bigint))
    WHERE entity_type = 'vendor_pks' AND status = 'pending';

CREATE UNIQUE INDEX master_data_change_requests_vendor_pks_wod_pending_uq
    ON public.master_data_change_requests (lower(btrim(payload->>'wod_number')))
    WHERE entity_type = 'vendor_pks' AND op = 'create' AND status = 'pending';

CREATE UNIQUE INDEX master_data_change_requests_pks_type_code_pending_uq
    ON public.master_data_change_requests (upper(payload->>'code'))
    WHERE entity_type = 'vendor_pks_type' AND op = 'create' AND status = 'pending';

COMMIT;
