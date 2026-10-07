-- 008_vendor_vault_types.sql
-- Tipe vault per cabang vendor (ATM / CASH / ATM & CASH) dari daftar master vendor.
--
-- vendor_vaults.category sebelumnya biner CHECK IN ('ATM','CASH') dan SELURUH 348
-- baris di-backfill ke 'ATM' sebagai placeholder (lihat 003_master_data.sql T0.1/T1.3).
-- Data master menyebut sebagian cabang melayani ATM sekaligus CASH, jadi constraint
-- dilonggarkan jadi tiga nilai dan seluruh category diisi nilai sebenarnya.
--
-- Cabang ROH (300 baris + 8 hub dari 007) TIDAK ada di daftar master ini; category-nya
-- dibiarkan apa adanya, begitu juga 485 vendor_packages dan 692 atm_vendor_packages-nya.

BEGIN;

-- 1. Longgarkan constraint kategori jadi tiga nilai.
ALTER TABLE public.vendor_vaults
    DROP CONSTRAINT IF EXISTS vendor_vaults_category_chk;

ALTER TABLE public.vendor_vaults
    ADD CONSTRAINT vendor_vaults_category_chk
    CHECK (category IN ('ATM', 'CASH', 'ATM_CASH'));

COMMENT ON COLUMN public.vendor_vaults.category
    IS 'ATM, CASH, atau ATM_CASH (melayani keduanya). Diisi dari daftar master vendor di migrasi 008; sebelumnya biner dengan seluruh baris di-backfill ke ATM.';

-- 2. Samakan nama tiga cabang yang di daftar master tertulis lebih lengkap.
--    Rename, bukan hapus-lalu-insert: 4 vendor_packages dan 26 atm_vendor_packages
--    menggantung di ketiganya.
UPDATE public.vendor_branches SET branch_name = 'Advantage Jakarta Serang', updated_at = now() WHERE branch_code = 'ADVANTAGE_017';
UPDATE public.vendor_branches SET branch_name = 'TAG Jakarta Bekasi', updated_at = now() WHERE branch_code = 'TAG_004';
UPDATE public.vendor_branches SET branch_name = 'TAG Makassar', updated_at = now() WHERE branch_code = 'TAG_010';

-- 3. Vendor yang belum ada di master.
INSERT INTO public.vendors (code, name, request_prefix) VALUES
    ('BRINKS', 'Brinks', 'BRK'),
    ('KEJAR', 'Kejar', 'KJR'),
    ('PROSEGUR', 'Prosegur', 'PSG')
ON CONFLICT (code) DO NOTHING;

-- 4. 51 cabang yang belum ada.
INSERT INTO public.vendor_branches (vendor_id, branch_code, branch_name, is_active)
SELECT v.id, n.branch_code, n.branch_name, true
FROM (VALUES
    ('ABACUS_009', 'ABACUS', 'Abacus Makassar'),
    ('ADVANTAGE_019', 'ADVANTAGE', 'Advantage Bandung'),
    ('ADVANTAGE_020', 'ADVANTAGE', 'Advantage Denpasar'),
    ('ADVANTAGE_021', 'ADVANTAGE', 'Advantage Jakarta Karawang'),
    ('ADVANTAGE_022', 'ADVANTAGE', 'Advantage Jakarta Meruya'),
    ('ADVANTAGE_023', 'ADVANTAGE', 'Advantage Jakarta Rawamangun'),
    ('ADVANTAGE_024', 'ADVANTAGE', 'Advantage Jember'),
    ('ADVANTAGE_025', 'ADVANTAGE', 'Advantage Kediri'),
    ('ADVANTAGE_026', 'ADVANTAGE', 'Advantage Kudus'),
    ('ADVANTAGE_027', 'ADVANTAGE', 'Advantage Makassar'),
    ('ADVANTAGE_028', 'ADVANTAGE', 'Advantage Malang'),
    ('ADVANTAGE_029', 'ADVANTAGE', 'Advantage Solo'),
    ('ADVANTAGE_030', 'ADVANTAGE', 'Advantage Surabaya'),
    ('ADVANTAGE_031', 'ADVANTAGE', 'Advantage Tasikmalaya'),
    ('ADVANTAGE_032', 'ADVANTAGE', 'Advantage Tegal Slawi'),
    ('BRINKS_001', 'BRINKS', 'Brinks Balikpapan'),
    ('BRINKS_002', 'BRINKS', 'Brinks Bandung'),
    ('BRINKS_003', 'BRINKS', 'Brinks Batam'),
    ('BRINKS_004', 'BRINKS', 'Brinks Cirebon'),
    ('BRINKS_005', 'BRINKS', 'Brinks Denpasar'),
    ('BRINKS_006', 'BRINKS', 'Brinks Jakarta'),
    ('BRINKS_007', 'BRINKS', 'Brinks Klaten Solo'),
    ('BRINKS_008', 'BRINKS', 'Brinks Klaten Yogya'),
    ('BRINKS_009', 'BRINKS', 'Brinks Lampung'),
    ('BRINKS_010', 'BRINKS', 'Brinks Makassar'),
    ('BRINKS_011', 'BRINKS', 'Brinks Manado'),
    ('BRINKS_012', 'BRINKS', 'Brinks Medan'),
    ('BRINKS_013', 'BRINKS', 'Brinks Pekanbaru'),
    ('BRINKS_014', 'BRINKS', 'Brinks Purwokerto'),
    ('BRINKS_015', 'BRINKS', 'Brinks Semarang'),
    ('BRINKS_016', 'BRINKS', 'Brinks Surabaya'),
    ('KEJAR_001', 'KEJAR', 'Kejar Bandung'),
    ('KEJAR_002', 'KEJAR', 'Kejar Cirebon'),
    ('KEJAR_003', 'KEJAR', 'Kejar Jakarta'),
    ('KEJAR_004', 'KEJAR', 'Kejar Semarang'),
    ('KEJAR_005', 'KEJAR', 'Kejar Surabaya'),
    ('PROSEGUR_001', 'PROSEGUR', 'Prosegur Jakarta'),
    ('PROSEGUR_002', 'PROSEGUR', 'Prosegur Jember'),
    ('PROSEGUR_003', 'PROSEGUR', 'Prosegur Lampung'),
    ('PROSEGUR_004', 'PROSEGUR', 'Prosegur Madiun'),
    ('PROSEGUR_005', 'PROSEGUR', 'Prosegur Malang'),
    ('PROSEGUR_006', 'PROSEGUR', 'Prosegur Padang'),
    ('PROSEGUR_007', 'PROSEGUR', 'Prosegur Palembang'),
    ('PROSEGUR_008', 'PROSEGUR', 'Prosegur Solo'),
    ('PROSEGUR_009', 'PROSEGUR', 'Prosegur Surabaya'),
    ('SSI_005', 'SSI', 'SSI Dumai'),
    ('SSI_006', 'SSI', 'SSI Jayapura'),
    ('SSI_007', 'SSI', 'SSI Lhokseumawe'),
    ('TAG_018', 'TAG', 'TAG Jakarta Bintaro'),
    ('TAG_019', 'TAG', 'TAG Jakarta Lenteng Agung'),
    ('TAG_020', 'TAG', 'TAG Jakarta Tangerang')
) AS n(branch_code, vendor_code, branch_name)
JOIN public.vendors v ON v.code = n.vendor_code
ON CONFLICT (branch_code) DO NOTHING;

-- 5. Vault untuk 51 cabang baru (satu vault per cabang, seperti 348 baris existing).
--    type di-mirror dari category, mengikuti CreateVendorVaultAdmin di queries/vendor_vaults_admin.sql.
INSERT INTO public.vendor_vaults (vendor_branch_id, vault_code, type, category, currency_code, is_active)
SELECT vb.id, 'VLT_' || n.branch_code, n.category, n.category, 'IDR', true
FROM (VALUES
    ('ABACUS_009', 'CASH'),
    ('ADVANTAGE_019', 'CASH'),
    ('ADVANTAGE_020', 'CASH'),
    ('ADVANTAGE_021', 'CASH'),
    ('ADVANTAGE_022', 'ATM_CASH'),
    ('ADVANTAGE_023', 'CASH'),
    ('ADVANTAGE_024', 'CASH'),
    ('ADVANTAGE_025', 'CASH'),
    ('ADVANTAGE_026', 'ATM_CASH'),
    ('ADVANTAGE_027', 'CASH'),
    ('ADVANTAGE_028', 'CASH'),
    ('ADVANTAGE_029', 'CASH'),
    ('ADVANTAGE_030', 'CASH'),
    ('ADVANTAGE_031', 'CASH'),
    ('ADVANTAGE_032', 'CASH'),
    ('BRINKS_001', 'CASH'),
    ('BRINKS_002', 'CASH'),
    ('BRINKS_003', 'CASH'),
    ('BRINKS_004', 'CASH'),
    ('BRINKS_005', 'CASH'),
    ('BRINKS_006', 'CASH'),
    ('BRINKS_007', 'CASH'),
    ('BRINKS_008', 'CASH'),
    ('BRINKS_009', 'CASH'),
    ('BRINKS_010', 'CASH'),
    ('BRINKS_011', 'CASH'),
    ('BRINKS_012', 'CASH'),
    ('BRINKS_013', 'CASH'),
    ('BRINKS_014', 'CASH'),
    ('BRINKS_015', 'CASH'),
    ('BRINKS_016', 'CASH'),
    ('KEJAR_001', 'CASH'),
    ('KEJAR_002', 'CASH'),
    ('KEJAR_003', 'CASH'),
    ('KEJAR_004', 'CASH'),
    ('KEJAR_005', 'CASH'),
    ('PROSEGUR_001', 'CASH'),
    ('PROSEGUR_002', 'CASH'),
    ('PROSEGUR_003', 'CASH'),
    ('PROSEGUR_004', 'CASH'),
    ('PROSEGUR_005', 'CASH'),
    ('PROSEGUR_006', 'CASH'),
    ('PROSEGUR_007', 'CASH'),
    ('PROSEGUR_008', 'CASH'),
    ('PROSEGUR_009', 'CASH'),
    ('SSI_005', 'ATM'),
    ('SSI_006', 'ATM_CASH'),
    ('SSI_007', 'ATM'),
    ('TAG_018', 'ATM'),
    ('TAG_019', 'ATM_CASH'),
    ('TAG_020', 'ATM')
) AS n(branch_code, category)
JOIN public.vendor_branches vb ON vb.branch_code = n.branch_code
ON CONFLICT (vault_code) DO NOTHING;

-- 6. Isi category sebenarnya untuk 48 cabang yang sudah ada.
--    type ikut di-mirror, mengikuti UpdateVendorVaultAdmin.
UPDATE public.vendor_vaults vv
SET category = n.category, type = n.category, updated_at = now()
FROM (VALUES
    ('ABACUS_001', 'ATM_CASH'),
    ('ABACUS_002', 'ATM_CASH'),
    ('ABACUS_006', 'ATM'),
    ('ABACUS_003', 'ATM_CASH'),
    ('ABACUS_004', 'ATM_CASH'),
    ('ABACUS_005', 'ATM_CASH'),
    ('ABACUS_007', 'ATM'),
    ('ABACUS_008', 'ATM_CASH'),
    ('ADVANTAGE_001', 'ATM_CASH'),
    ('ADVANTAGE_002', 'ATM_CASH'),
    ('ADVANTAGE_003', 'ATM_CASH'),
    ('ADVANTAGE_017', 'ATM'),
    ('ADVANTAGE_004', 'ATM_CASH'),
    ('ADVANTAGE_016', 'ATM_CASH'),
    ('ADVANTAGE_005', 'ATM_CASH'),
    ('ADVANTAGE_006', 'ATM_CASH'),
    ('ADVANTAGE_007', 'ATM_CASH'),
    ('ADVANTAGE_008', 'ATM_CASH'),
    ('ADVANTAGE_009', 'ATM_CASH'),
    ('ADVANTAGE_010', 'ATM_CASH'),
    ('ADVANTAGE_011', 'ATM_CASH'),
    ('ADVANTAGE_012', 'ATM_CASH'),
    ('ADVANTAGE_013', 'ATM_CASH'),
    ('ADVANTAGE_014', 'ATM_CASH'),
    ('ADVANTAGE_015', 'ATM_CASH'),
    ('ADVANTAGE_018', 'ATM_CASH'),
    ('BIJAK_001', 'ATM_CASH'),
    ('SSI_001', 'ATM'),
    ('SSI_002', 'ATM'),
    ('SSI_003', 'ATM_CASH'),
    ('SSI_004', 'ATM_CASH'),
    ('TAG_001', 'ATM_CASH'),
    ('TAG_002', 'ATM'),
    ('TAG_003', 'ATM_CASH'),
    ('TAG_005', 'ATM_CASH'),
    ('TAG_004', 'ATM'),
    ('TAG_006', 'ATM_CASH'),
    ('TAG_007', 'ATM_CASH'),
    ('TAG_008', 'ATM_CASH'),
    ('TAG_009', 'ATM_CASH'),
    ('TAG_010', 'ATM'),
    ('TAG_011', 'ATM_CASH'),
    ('TAG_012', 'ATM_CASH'),
    ('TAG_013', 'ATM_CASH'),
    ('TAG_014', 'ATM_CASH'),
    ('TAG_015', 'ATM_CASH'),
    ('TAG_016', 'ATM_CASH'),
    ('TAG_017', 'ATM_CASH')
) AS n(branch_code, category)
JOIN public.vendor_branches vb ON vb.branch_code = n.branch_code
WHERE vv.vendor_branch_id = vb.id;

COMMIT;
