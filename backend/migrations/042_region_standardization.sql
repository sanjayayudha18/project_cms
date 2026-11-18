-- 042_region_standardization.sql
-- Standarisasi region ATM + cabang vendor (.kiro/specs/region-standardization/,
-- pemetaan region-mapping.md accepted PO 2026-11-18). Forward-only, satu transaksi.
--
-- Perubahan:
--   1. Guard: berhenti bila masih ada Vendor Request belum selesai
--      (status bukan completed/cancelled/failed) — request berjalan di-clear dulu (Q7).
--   2. Guard: berhenti bila ada kode region cabang / Area ACM / cash count yang
--      tidak tercakup peta, cabang aktif tanpa kode yang tidak dipetakan, atau
--      penggabungan kode yang menyatukan Area ACM / area cash count berbeda.
--   3. Backup nilai lama ke migration_042_backup (dipakai blok ROLLBACK di bawah).
--   4. regions: 13 baris di-rename di tempat (code + nama), id tetap; atms/locations
--      tidak disentuh (Q6). Dicocokkan lewat kode lama, bukan id.
--   5. region_cities (baru): 22 kota di bawah satu region (Q1).
--   6. vendor_branches.region_code diisi ulang (override per branch_code dulu, lalu
--      per kode lama), lalu FK ke regions(code) (Q5a); kolom vendor_branches.region
--      di-drop, nama region dibaca dari regions.region (Q5b).
--   7. acm_area_regions, cash_count_area_regions, cash_count_pics.region_code
--      dipetakan ulang (kode yang tergabung dilebur menjadi satu baris).
--
-- Tidak menyentuh atms, locations, vendor_requests, vendor_request_tickets,
-- vendor_request_number_seq. Tidak menulis audit_logs (perubahan sistem; jejak =
-- file ini + schema_migrations + migration_042_backup).
--
-- Catatan:
--   * Kode baru yang tidak punya Area ACM lama (mis. TGR — tidak ada kode cabang
--     lama TANGERANG) tidak mendapat baris acm_area_regions; request dari cabangnya
--     menunggu sampai ADMIN memetakan region itu (perilaku yang sudah ada).
--   * Update vendor_branches menyentuh updated_at (trigger), dan snapshot change
--     vendor_branch yang pending masih memuat kunci `region`: change itu akan
--     stale saat di-approve; maker submit ulang.
--
-- ROLLBACK (documented, forward-only shop; juga di
-- backend/scripts/dev/rollback_042_region_standardization.sql):
--   BEGIN;
--   ALTER TABLE public.vendor_branches DROP CONSTRAINT IF EXISTS vendor_branches_region_code_fk;
--   ALTER TABLE public.vendor_branches ADD COLUMN region text;
--   UPDATE public.vendor_branches b
--      SET region = k.payload->>'region', region_code = k.payload->>'region_code'
--     FROM public.migration_042_backup k
--    WHERE k.entity = 'vendor_branches' AND k.entity_key = b.id::text;
--   UPDATE public.regions r
--      SET code = k.payload->>'code', region = k.payload->>'region'
--     FROM public.migration_042_backup k
--    WHERE k.entity = 'regions' AND k.entity_key = r.id::text;
--   DELETE FROM public.acm_area_regions;
--   INSERT INTO public.acm_area_regions
--   SELECT (jsonb_populate_record(NULL::public.acm_area_regions, k.payload)).*
--     FROM public.migration_042_backup k WHERE k.entity = 'acm_area_regions';
--   DELETE FROM public.cash_count_area_regions;
--   INSERT INTO public.cash_count_area_regions
--   SELECT (jsonb_populate_record(NULL::public.cash_count_area_regions, k.payload)).*
--     FROM public.migration_042_backup k WHERE k.entity = 'cash_count_area_regions';
--   UPDATE public.cash_count_pics p SET region_code = k.payload->>'region_code'
--     FROM public.migration_042_backup k
--    WHERE k.entity = 'cash_count_pics' AND k.entity_key = p.id::text;
--   DROP TABLE public.region_cities;
--   DROP TABLE public.migration_042_backup;
--   DELETE FROM public.schema_migrations WHERE version = '042_region_standardization.sql';
--   COMMIT;

BEGIN;

-- 1. Guard: Vendor Request belum selesai --------------------------------------
DO $$
DECLARE n bigint;
BEGIN
    SELECT count(*) INTO n FROM public.vendor_requests
     WHERE status NOT IN ('completed', 'cancelled', 'failed');
    IF n > 0 THEN
        RAISE EXCEPTION 'region std 042: % unfinished vendor requests; clear them first (spec Q7)', n;
    END IF;
END $$;

-- 2. Peta (region-mapping.md) --------------------------------------------------
-- Temp tables are created through EXECUTE inside DO blocks so sqlc (which
-- reads backend/migrations/ as the schema) does not turn them into models.
-- A. regions: kode lama -> kode baku + nama baku
DO $$ BEGIN EXECUTE 'CREATE TEMP TABLE m_region (old_code text PRIMARY KEY, new_code text NOT NULL UNIQUE, name text NOT NULL) ON COMMIT DROP'; END $$;
INSERT INTO m_region VALUES
    ('BALI_NUSRA',  'BALNUS',   'Bali Nusra'),
    ('INDTIM',      'INDTIM',   'Indonesia Timur'),
    ('JABAR',       'JABAR',    'Jawa Barat'),
    ('JATENG',      'JATENG',   'Jawa Tengah'),
    ('JATIM',       'JATIM',    'Jawa Timur'),
    ('JKT_CENTRAL', 'JKTPST',   'Jakarta Pusat'),
    ('JKT_EAST',    'JKTTIM',   'Jakarta Timur'),
    ('JKT_NORTH',   'JKTUTR',   'Jakarta Utara'),
    ('JKT_SOUTH',   'JKTSEL',   'Jakarta Selatan'),
    ('JKT_WEST',    'JKTBAR',   'Jakarta Barat'),
    ('KALIMANTAN',  'KAL',      'Kalimantan'),
    ('SUMATERA',    'SUMATERA', 'Sumatera'),
    ('TANGERANG',   'TGR',      'Tangerang');

-- B. kode region cabang lama -> kode baku (34 kode). JKT -> JKTPST sebagai default
--    (BIJAK_001, M3); dua cabang JKT lain di-override di m_branch. INDTIM tetap
--    INDTIM; 16 cabang Kalimantan-nya di-override ke KAL (M1).
DO $$ BEGIN EXECUTE 'CREATE TEMP TABLE m_code (old_code text PRIMARY KEY, new_code text NOT NULL) ON COMMIT DROP'; END $$;
INSERT INTO m_code VALUES
    ('JKTBAR', 'JKTBAR'), ('JKTPST', 'JKTPST'), ('JKTSEL', 'JKTSEL'), ('JKTTIM', 'JKTTIM'), ('JKTUTR', 'JKTUTR'),
    ('JABAR', 'JABAR'), ('JATENG', 'JATENG'), ('JATIM', 'JATIM'), ('SUMATERA', 'SUMATERA'),
    ('BALNUS', 'BALNUS'), ('INDTIM', 'INDTIM'),
    ('BALI', 'BALNUS'), ('MTR', 'BALNUS'), ('KUPANG', 'BALNUS'),
    ('ACEH', 'SUMATERA'), ('JAMBI', 'SUMATERA'), ('LAMPUNG', 'SUMATERA'), ('MDN', 'SUMATERA'),
    ('PGK', 'SUMATERA'), ('PKU', 'SUMATERA'), ('PLG', 'SUMATERA'), ('TNJ', 'SUMATERA'),
    ('AMBON', 'INDTIM'), ('KDI', 'INDTIM'), ('MDO', 'INDTIM'), ('MKS', 'INDTIM'),
    ('BJM', 'KAL'), ('BPN', 'KAL'), ('PKY', 'KAL'), ('PNK', 'KAL'), ('SMD', 'KAL'),
    ('JEMBER', 'JATIM'), ('MADIUN', 'JATIM'),
    ('JKT', 'JKTPST');

-- C/D + M1. override per cabang (branch_code unik global).
DO $$ BEGIN EXECUTE 'CREATE TEMP TABLE m_branch (branch_code text PRIMARY KEY, new_code text NOT NULL) ON COMMIT DROP'; END $$;
INSERT INTO m_branch VALUES
    -- C: 51 cabang tanpa kode (Q8)
    ('ADVANTAGE_019', 'JABAR'), ('ADVANTAGE_031', 'JABAR'), ('BRINKS_002', 'JABAR'), ('BRINKS_004', 'JABAR'),
    ('KEJAR_001', 'JABAR'), ('KEJAR_002', 'JABAR'), ('ADVANTAGE_021', 'JABAR'),
    ('ADVANTAGE_026', 'JATENG'), ('ADVANTAGE_029', 'JATENG'), ('ADVANTAGE_032', 'JATENG'), ('BRINKS_007', 'JATENG'),
    ('BRINKS_008', 'JATENG'), ('BRINKS_014', 'JATENG'), ('BRINKS_015', 'JATENG'), ('KEJAR_004', 'JATENG'),
    ('PROSEGUR_008', 'JATENG'),
    ('ADVANTAGE_024', 'JATIM'), ('ADVANTAGE_025', 'JATIM'), ('ADVANTAGE_028', 'JATIM'), ('ADVANTAGE_030', 'JATIM'),
    ('BRINKS_016', 'JATIM'), ('KEJAR_005', 'JATIM'), ('PROSEGUR_002', 'JATIM'), ('PROSEGUR_004', 'JATIM'),
    ('PROSEGUR_005', 'JATIM'), ('PROSEGUR_009', 'JATIM'),
    ('ADVANTAGE_020', 'BALNUS'), ('BRINKS_005', 'BALNUS'),
    ('BRINKS_003', 'SUMATERA'), ('BRINKS_009', 'SUMATERA'), ('BRINKS_012', 'SUMATERA'), ('BRINKS_013', 'SUMATERA'),
    ('PROSEGUR_003', 'SUMATERA'), ('PROSEGUR_006', 'SUMATERA'), ('PROSEGUR_007', 'SUMATERA'), ('SSI_005', 'SUMATERA'),
    ('SSI_007', 'SUMATERA'),
    ('ABACUS_009', 'INDTIM'), ('ADVANTAGE_027', 'INDTIM'), ('BRINKS_010', 'INDTIM'), ('BRINKS_011', 'INDTIM'),
    ('SSI_006', 'INDTIM'),
    ('BRINKS_001', 'KAL'),
    ('ADVANTAGE_022', 'JKTBAR'), ('ADVANTAGE_023', 'JKTTIM'), ('TAG_018', 'JKTSEL'), ('TAG_019', 'JKTSEL'),
    ('TAG_020', 'TGR'),
    ('BRINKS_006', 'JKTPST'), ('KEJAR_003', 'JKTPST'), ('PROSEGUR_001', 'JKTPST'),
    -- D: cabang kode JKT
    ('ADVANTAGE_017', 'TGR'), ('TAG_004', 'JKTTIM'), ('BIJAK_001', 'JKTPST'),
    -- M1: cabang Kalimantan yang berkode INDTIM
    ('ROH_010', 'KAL'), ('ROH_011', 'KAL'), ('ROH_012', 'KAL'), ('ROH_028', 'KAL'), ('ROH_029', 'KAL'),
    ('ROH_030', 'KAL'), ('ROH_151', 'KAL'), ('ROH_211', 'KAL'), ('ROH_227', 'KAL'), ('ROH_228', 'KAL'),
    ('ROH_235', 'KAL'), ('ROH_236', 'KAL'), ('ROH_237', 'KAL'), ('ROH_238', 'KAL'), ('ROH_249', 'KAL'),
    ('ROH_289', 'KAL');

-- B. kota (kode kota lama) di bawah region baku
DO $$ BEGIN EXECUTE 'CREATE TEMP TABLE m_city (code text PRIMARY KEY, name text NOT NULL, region_code text NOT NULL) ON COMMIT DROP'; END $$;
INSERT INTO m_city VALUES
    ('ACEH', 'Aceh', 'SUMATERA'), ('JAMBI', 'Jambi', 'SUMATERA'), ('LAMPUNG', 'Lampung', 'SUMATERA'),
    ('MDN', 'Medan', 'SUMATERA'), ('PGK', 'Pangkal Pinang', 'SUMATERA'), ('PKU', 'Pekanbaru', 'SUMATERA'),
    ('PLG', 'Palembang', 'SUMATERA'), ('TNJ', 'Tanjung Pinang', 'SUMATERA'),
    ('AMBON', 'Ambon', 'INDTIM'), ('KDI', 'Kendari', 'INDTIM'), ('MDO', 'Manado', 'INDTIM'), ('MKS', 'Makassar', 'INDTIM'),
    ('BALI', 'Bali', 'BALNUS'), ('MTR', 'Mataram', 'BALNUS'), ('KUPANG', 'Kupang', 'BALNUS'),
    ('BJM', 'Banjarmasin', 'KAL'), ('BPN', 'Balikpapan', 'KAL'), ('PKY', 'Palangkaraya', 'KAL'),
    ('PNK', 'Pontianak', 'KAL'), ('SMD', 'Samarinda', 'KAL'),
    ('JEMBER', 'Jember', 'JATIM'), ('MADIUN', 'Madiun', 'JATIM');

-- 3. Guard cakupan --------------------------------------------------------------
DO $$
DECLARE bad text;
BEGIN
    SELECT string_agg(DISTINCT vb.region_code, ', ') INTO bad
      FROM public.vendor_branches vb
     WHERE vb.region_code IS NOT NULL
       AND NOT EXISTS (SELECT 1 FROM m_code c WHERE c.old_code = vb.region_code)
       AND NOT EXISTS (SELECT 1 FROM m_branch b WHERE b.branch_code = vb.branch_code);
    IF bad IS NOT NULL THEN
        RAISE EXCEPTION 'region std 042: vendor_branches.region_code not in mapping: %', bad;
    END IF;

    SELECT string_agg(vb.branch_code, ', ') INTO bad
      FROM public.vendor_branches vb
     WHERE vb.region_code IS NULL AND vb.is_active
       AND NOT EXISTS (SELECT 1 FROM m_branch b WHERE b.branch_code = vb.branch_code);
    IF bad IS NOT NULL THEN
        RAISE EXCEPTION 'region std 042: active vendor branches without region_code and no mapping: %', bad;
    END IF;

    SELECT string_agg(DISTINCT x.region_code, ', ') INTO bad
      FROM (SELECT region_code FROM public.acm_area_regions
            UNION SELECT region_code FROM public.cash_count_area_regions
            UNION SELECT region_code FROM public.cash_count_pics WHERE region_code IS NOT NULL) x
     WHERE NOT EXISTS (SELECT 1 FROM m_code c WHERE c.old_code = x.region_code);
    IF bad IS NOT NULL THEN
        RAISE EXCEPTION 'region std 042: acm/cash count region_code not in mapping: %', bad;
    END IF;

    SELECT string_agg(new_code, ', ') INTO bad FROM (
        SELECT c.new_code FROM public.acm_area_regions a JOIN m_code c ON c.old_code = a.region_code
         GROUP BY c.new_code HAVING count(DISTINCT a.acm_area_id) > 1) s;
    IF bad IS NOT NULL THEN
        RAISE EXCEPTION 'region std 042: merged codes belong to different Area ACM: %', bad;
    END IF;

    SELECT string_agg(new_code, ', ') INTO bad FROM (
        SELECT c.new_code FROM public.cash_count_area_regions a JOIN m_code c ON c.old_code = a.region_code
         GROUP BY c.new_code HAVING count(DISTINCT a.area_id) > 1) s;
    IF bad IS NOT NULL THEN
        RAISE EXCEPTION 'region std 042: merged codes belong to different cash count areas: %', bad;
    END IF;

    SELECT string_agg(DISTINCT d.new_code, ', ') INTO bad FROM (
        SELECT p.pic_role, p.user_id, c.new_code
          FROM public.cash_count_pics p JOIN m_code c ON c.old_code = p.region_code
         WHERE p.is_active
         GROUP BY p.pic_role, p.user_id, c.new_code HAVING count(*) > 1) d;
    IF bad IS NOT NULL THEN
        RAISE EXCEPTION 'region std 042: merged codes duplicate an active cash count PIC: %', bad;
    END IF;
END $$;

-- 4. Backup --------------------------------------------------------------------
CREATE TABLE public.migration_042_backup (
    entity     text  NOT NULL,
    entity_key text  NOT NULL,
    payload    jsonb NOT NULL,
    PRIMARY KEY (entity, entity_key)
);
COMMENT ON TABLE public.migration_042_backup IS
    'Temporary: pre-042 values for the documented rollback of 042_region_standardization.sql. Drop in a later migration once the release is confirmed.';

INSERT INTO public.migration_042_backup
SELECT 'regions', r.id::text, jsonb_build_object('id', r.id, 'code', r.code, 'region', r.region)
  FROM public.regions r;
INSERT INTO public.migration_042_backup
SELECT 'vendor_branches', vb.id::text, jsonb_build_object('region', vb.region, 'region_code', vb.region_code)
  FROM public.vendor_branches vb;
INSERT INTO public.migration_042_backup
SELECT 'acm_area_regions', a.acm_area_id::text || ':' || a.region_code, to_jsonb(a)
  FROM public.acm_area_regions a;
INSERT INTO public.migration_042_backup
SELECT 'cash_count_area_regions', a.region_code, to_jsonb(a)
  FROM public.cash_count_area_regions a;
INSERT INTO public.migration_042_backup
SELECT 'cash_count_pics', p.id::text, jsonb_build_object('region_code', p.region_code)
  FROM public.cash_count_pics p WHERE p.region_code IS NOT NULL;

-- 5. regions: rename di tempat -------------------------------------------------
UPDATE public.regions r
   SET code = m.new_code, region = m.name
  FROM m_region m
 WHERE r.code = m.old_code;

-- 6. region_cities --------------------------------------------------------------
CREATE TABLE public.region_cities (
    id         bigint      GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    code       text        NOT NULL CONSTRAINT region_cities_code_chk CHECK (code ~ '^[A-Z0-9]{2,10}$'),
    name       text        NOT NULL CONSTRAINT region_cities_name_chk CHECK (btrim(name) <> ''),
    region_id  bigint      NOT NULL CONSTRAINT region_cities_region_fk REFERENCES public.regions(id) ON DELETE RESTRICT,
    is_active  boolean     NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    deleted_at timestamptz NULL,
    CONSTRAINT region_cities_code_uq UNIQUE (code)
);
CREATE INDEX region_cities_region_idx ON public.region_cities (region_id);
CREATE TRIGGER trg_region_cities_set_updated_at BEFORE UPDATE ON public.region_cities
    FOR EACH ROW WHEN (old.* IS DISTINCT FROM new.*) EXECUTE FUNCTION public.set_updated_at();
COMMENT ON TABLE public.region_cities IS
    'Kota di bawah tepat satu region baku (spec region-standardization Q1). Tabel saja: belum ada layar/API/CSV.';

INSERT INTO public.region_cities (code, name, region_id)
SELECT c.code, c.name, r.id
  FROM m_city c JOIN public.regions r ON r.code = c.region_code;

-- 7. vendor_branches.region_code + FK + drop region ---------------------------
UPDATE public.vendor_branches vb
   SET region_code = COALESCE(
        (SELECT b.new_code FROM m_branch b WHERE b.branch_code = vb.branch_code),
        (SELECT c.new_code FROM m_code c WHERE c.old_code = vb.region_code))
 WHERE vb.region_code IS NOT NULL
    OR EXISTS (SELECT 1 FROM m_branch b WHERE b.branch_code = vb.branch_code);

ALTER TABLE public.vendor_branches
    ADD CONSTRAINT vendor_branches_region_code_fk FOREIGN KEY (region_code)
        REFERENCES public.regions(code) ON UPDATE RESTRICT ON DELETE RESTRICT;

ALTER TABLE public.vendor_branches DROP COLUMN region;

COMMENT ON COLUMN public.vendor_branches.region_code IS
    'Region baku cabang (FK regions.code, migration 042). Nama region dibaca dari regions.region. Segmen nomor request REP-<vendor>-<region_code>-...';

-- 8. Tabel turunan -------------------------------------------------------------
DO $$ BEGIN EXECUTE '
CREATE TEMP TABLE m_acm ON COMMIT DROP AS
SELECT a.acm_area_id, c.new_code AS region_code, min(a.created_by) AS created_by, min(a.created_at) AS created_at
  FROM public.acm_area_regions a JOIN m_code c ON c.old_code = a.region_code
 GROUP BY a.acm_area_id, c.new_code'; END $$;
DELETE FROM public.acm_area_regions;
INSERT INTO public.acm_area_regions (acm_area_id, region_code, created_by, created_at)
SELECT acm_area_id, region_code, created_by, created_at FROM m_acm;

DO $$ BEGIN EXECUTE '
CREATE TEMP TABLE m_cc_area ON COMMIT DROP AS
SELECT c.new_code AS region_code, min(a.area_id) AS area_id, min(a.created_by) AS created_by, min(a.created_at) AS created_at
  FROM public.cash_count_area_regions a JOIN m_code c ON c.old_code = a.region_code
 GROUP BY c.new_code'; END $$;
DELETE FROM public.cash_count_area_regions;
INSERT INTO public.cash_count_area_regions (region_code, area_id, created_by, created_at)
SELECT region_code, area_id, created_by, created_at FROM m_cc_area;

UPDATE public.cash_count_pics p
   SET region_code = c.new_code
  FROM m_code c
 WHERE p.region_code = c.old_code AND p.region_code <> c.new_code;

COMMIT;
