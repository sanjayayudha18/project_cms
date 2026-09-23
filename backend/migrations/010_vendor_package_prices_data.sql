-- 010_vendor_package_prices_data.sql
-- Perpindahan data untuk skema harga paket vendor FLM dari 009.
-- Sumber requirement: archives/Harga Paket per vendor FLM.docx (gambar Bagian IV per vendor)
-- Keputusan P1-P20 & urutan langkah: .claude/sdlc/vendor-pricing/plan.md ("Langkah berikutnya")
--
-- Kenapa dipisah dari 009: bagian ini memindah dan mengubah bentuk data yang sudah
-- ada (bukan sekadar menambah struktur), jadi sulit dibalik begitu jalan. 009 sengaja
-- berhenti sebelum titik ini - vendor_packages.price dan priority_class masih ada
-- setelah 009, migrasi ini yang membuangnya, setelah datanya dipindah ke tempat baru.
--
-- Urutan LANGKAH SENGAJA seperti ini, jangan diacak:
--   1-2 dulu (cimb_branches + backfill atms.cimb_branch_id) SELAGI vendor_packages.
--       vendor_branch_id ROH masih utuh - langkah 3 menghapus jejak itu.
--   3 baru mengubah vendor_packages (butuh hasil langkah 2 sudah aman di atms).
--   4 baru boleh DROP COLUMN, setelah price/priority_class tidak dipakai lagi.
--   5-6 independen, taruh di akhir.

BEGIN;

-- ---------------------------------------------------------------------
-- 1. cimb_branches dari 300 cabang ROH_0xx (kantor cabang, bukan hub).
--    Filter regex, BUKAN 'ROH_0%' - kode cabang naik sampai ROH_300, jadi LIKE
--    'ROH_0%' hanya akan menangkap ROH_001-ROH_099 dan melewatkan 201 baris.
--    8 hub ROH_H01-ROH_H08 (migrasi 007) otomatis tidak cocok regex ini.
--
--    region_id sengaja NULL: vendor_branches tidak punya FK ke regions (hanya
--    kolom teks bebas `region` dan `location_id` yang NULL untuk semua baris ROH
--    karena ROH bukan ATM_SITE). Tidak ditebak.
-- ---------------------------------------------------------------------

INSERT INTO public.cimb_branches (branch_code, branch_name, region_id, is_active, created_at, updated_at)
SELECT vb.branch_code, vb.branch_name, NULL, vb.is_active, now(), now()
FROM public.vendor_branches vb
JOIN public.vendors v ON v.id = vb.vendor_id
WHERE v.code = 'ROH'
  AND vb.branch_code ~ '^ROH_[0-9]{3}$'
ON CONFLICT (branch_code) DO NOTHING;

COMMENT ON COLUMN public.cimb_branches.region_id
    IS 'NULL untuk baris hasil migrasi 010: vendor_branches (sumber data) tidak punya atribusi region_id yang bisa dipercaya. Isi manual bila diperlukan, jangan ditebak dari region text bebas.';

-- ---------------------------------------------------------------------
-- 2. Backfill atms.cimb_branch_id via jalur kelolaan yang masih aktif:
--    atm_vendor_packages (aktif) -> vendor_packages -> vendor_branches (ROH_0xx)
--    -> cimb_branches (branch_code sama).
-- ---------------------------------------------------------------------

UPDATE public.atms a
SET cimb_branch_id = cb.id,
    updated_at = now()
FROM public.atm_vendor_packages avp
JOIN public.vendor_packages vp ON vp.id = avp.vendor_package_id
JOIN public.vendor_branches vb ON vb.id = vp.vendor_branch_id
JOIN public.vendors v ON v.id = vb.vendor_id
JOIN public.cimb_branches cb ON cb.branch_code = vb.branch_code
WHERE a.id = avp.atm_id
  AND avp.is_active = true
  AND v.code = 'ROH'
  AND vb.branch_code ~ '^ROH_[0-9]{3}$';

-- ---------------------------------------------------------------------
-- 3. Kolapskan paket ROH jadi paket internal (per kode paket, bukan per cabang).
--    Data-driven, bukan hardcode 'PAKET 4'/'PAKET 5': ambil kode apa pun yang
--    benar-benar dipakai ROH saat ini, supaya migrasi tidak diam-diam salah
--    kalau datanya berubah sebelum dijalankan.
-- ---------------------------------------------------------------------

INSERT INTO public.vendor_packages (vendor_branch_id, code, priority_class, price, created_at, updated_at)
SELECT NULL, roh_codes.code, 'ALL', 0, now(), now()
FROM (
    SELECT DISTINCT vp.code
    FROM public.vendor_packages vp
    JOIN public.vendor_branches vb ON vb.id = vp.vendor_branch_id
    JOIN public.vendors v ON v.id = vb.vendor_id
    WHERE v.code = 'ROH'
) AS roh_codes;

UPDATE public.atm_vendor_packages avp
SET vendor_package_id = internal_pkg.id,
    updated_at = now()
FROM public.vendor_packages old_vp
JOIN public.vendor_branches vb ON vb.id = old_vp.vendor_branch_id
JOIN public.vendors v ON v.id = vb.vendor_id
JOIN public.vendor_packages internal_pkg
    ON internal_pkg.code = old_vp.code
   AND internal_pkg.vendor_branch_id IS NULL
WHERE avp.vendor_package_id = old_vp.id
  AND v.code = 'ROH';

UPDATE public.vendor_packages vp
SET is_active = false, deleted_at = now(), updated_at = now()
FROM public.vendor_branches vb
JOIN public.vendors v ON v.id = vb.vendor_id
WHERE vp.vendor_branch_id = vb.id
  AND v.code = 'ROH';

-- ---------------------------------------------------------------------
-- 4. price & priority_class sudah tidak dipakai (harga pindah ke
--    vendor_package_prices; ATM internal tidak lagi butuh keduanya).
-- ---------------------------------------------------------------------

ALTER TABLE public.vendor_packages
    DROP COLUMN price,
    DROP COLUMN priority_class;

-- ---------------------------------------------------------------------
-- 5. Seed harga PT (P19) dari archives/Harga Paket per vendor FLM.docx,
--    periode 1 Jan 2026 - 31 Des 2027 (satu-satunya periode yang berlaku
--    per hari ini, 2026-09-22; periode 2025 sebelumnya sudah lewat).
--
--    Dimasukkan penuh: Advantage, Bijak, TAG (harga flat, tidak bertier),
--    Abacus (bertier per jumlah kelolaan, tanpa Add CR/Add FLM terpisah).
--
--    SSI hanya Paket 4 (CDM/CRM) dan Paket 5 (ATM) - dokumen tidak memuat
--    Paket 3/6 SSI. TIDAK ditebak dari pola vendor lain; isi manual nanti.
--
--    TIDAK di-seed sama sekali (butuh atribusi vendor per-ATM yang tidak ada
--    di dokumen, di luar cakupan migrasi data ini):
--      - Harga Khusus (Harga di Luar Service Area), 17 ATM.
--      - Biaya Tambahan Operasional Kelolaan Eksisting, 3 ATM.
--    Keduanya override per-ATM (vendor_package_prices.atm_id) untuk vendor
--    tertentu - masuk lewat layar admin setelah P20 selesai, bukan lewat
--    migrasi, supaya vendor_id-nya diverifikasi manusia, bukan ditebak.
-- ---------------------------------------------------------------------

-- Advantage: flat per paket/kelompok mesin/kelas, tidak bertier.
INSERT INTO public.vendor_package_prices
    (vendor_id, package_code, machine_group, price_class, tier_min, tier_max, base_price, add_cr_price, add_flm_price, effective_start_date, effective_end_date)
SELECT v.id, x.package_code, x.machine_group, x.price_class, 1, NULL, x.base_price, x.add_cr_price, x.add_flm_price, '2026-01-01', '2027-12-31'
FROM (SELECT id FROM public.vendors WHERE code = 'ADVANTAGE') v
CROSS JOIN (VALUES
    ('PAKET 3', 'ATM',     'REGULAR',      1865063::numeric, 266438::numeric, 159863::numeric),
    ('PAKET 4', 'ATM',     'REGULAR',      2184788, 266438, 159863),
    ('PAKET 5', 'ATM',     'REGULAR',      2283750, 266438, 159863),
    ('PAKET 3', 'ATM',     'VIP_INDUSTRI', 2078213, 266438, 159863),
    ('PAKET 4', 'ATM',     'VIP_INDUSTRI', 2397938, 266438, 159863),
    ('PAKET 5', 'ATM',     'VIP_INDUSTRI', 2717663, 266438, 159863),
    ('PAKET 3', 'CDM_CRM', 'REGULAR',      2283750, 304500, 152250),
    ('PAKET 4', 'CDM_CRM', 'REGULAR',      2740500, 304500, 152250),
    ('PAKET 5', 'CDM_CRM', 'REGULAR',      3045000, 304500, 152250),
    ('PAKET 3', 'CDM_CRM', 'VIP_INDUSTRI', 2486750, 304500, 152250),
    ('PAKET 4', 'CDM_CRM', 'VIP_INDUSTRI', 3146500, 304500, 152250),
    ('PAKET 5', 'CDM_CRM', 'VIP_INDUSTRI', 3298750, 304500, 152250)
) AS x(package_code, machine_group, price_class, base_price, add_cr_price, add_flm_price);

-- Bijak (PT Bintang Jasa Artha Kelola): flat, sama pola dengan Advantage.
INSERT INTO public.vendor_package_prices
    (vendor_id, package_code, machine_group, price_class, tier_min, tier_max, base_price, add_cr_price, add_flm_price, effective_start_date, effective_end_date)
SELECT v.id, x.package_code, x.machine_group, x.price_class, 1, NULL, x.base_price, x.add_cr_price, x.add_flm_price, '2026-01-01', '2027-12-31'
FROM (SELECT id FROM public.vendors WHERE code = 'BIJAK') v
CROSS JOIN (VALUES
    ('PAKET 3', 'ATM',     'REGULAR',      1831000::numeric, 263000::numeric, 131000::numeric),
    ('PAKET 4', 'ATM',     'REGULAR',      2040000, 263000, 131000),
    ('PAKET 5', 'ATM',     'REGULAR',      2192000, 263000, 131000),
    ('PAKET 3', 'ATM',     'VIP_INDUSTRI', 2009500, 263000, 131000),
    ('PAKET 4', 'ATM',     'VIP_INDUSTRI', 2351000, 263000, 131000),
    ('PAKET 5', 'ATM',     'VIP_INDUSTRI', 2462500, 263000, 131000),
    ('PAKET 3', 'CDM_CRM', 'REGULAR',      2173000, 263000, 131000),
    ('PAKET 4', 'CDM_CRM', 'REGULAR',      2690500, 263000, 131000),
    ('PAKET 5', 'CDM_CRM', 'REGULAR',      3126000, 263000, 131000),
    ('PAKET 3', 'CDM_CRM', 'VIP_INDUSTRI', 2382000, 263000, 131000),
    ('PAKET 4', 'CDM_CRM', 'VIP_INDUSTRI', 2894500, 263000, 131000),
    ('PAKET 5', 'CDM_CRM', 'VIP_INDUSTRI', 3323000, 263000, 131000)
) AS x(package_code, machine_group, price_class, base_price, add_cr_price, add_flm_price);

-- TAG (PT Tunas Artha Gardatama): flat, satu-satunya vendor dengan Paket 6.
INSERT INTO public.vendor_package_prices
    (vendor_id, package_code, machine_group, price_class, tier_min, tier_max, base_price, add_cr_price, add_flm_price, effective_start_date, effective_end_date)
SELECT v.id, x.package_code, x.machine_group, x.price_class, 1, NULL, x.base_price, x.add_cr_price, x.add_flm_price, '2026-01-01', '2027-12-31'
FROM (SELECT id FROM public.vendors WHERE code = 'TAG') v
CROSS JOIN (VALUES
    ('PAKET 3', 'ATM',     'REGULAR',      1806000::numeric, 315000::numeric, 157500::numeric),
    ('PAKET 4', 'ATM',     'REGULAR',      1973125, 315000, 157500),
    ('PAKET 5', 'ATM',     'REGULAR',      2152500, 315000, 157500),
    ('PAKET 6', 'ATM',     'REGULAR',      2391666, 315000, 157500),
    ('PAKET 3', 'ATM',     'VIP_INDUSTRI', 2076900, 315000, 157500),
    ('PAKET 4', 'ATM',     'VIP_INDUSTRI', 2269094, 315000, 157500),
    ('PAKET 5', 'ATM',     'VIP_INDUSTRI', 2475375, 315000, 157500),
    ('PAKET 6', 'ATM',     'VIP_INDUSTRI', 2750416, 315000, 157500),
    ('PAKET 3', 'CDM_CRM', 'REGULAR',      2637333, 315000, 157500),
    ('PAKET 4', 'CDM_CRM', 'REGULAR',      2677500, 315000, 157500),
    ('PAKET 5', 'CDM_CRM', 'REGULAR',      3109166, 315000, 157500),
    ('PAKET 6', 'CDM_CRM', 'REGULAR',      3348333, 315000, 157500),
    ('PAKET 3', 'CDM_CRM', 'VIP_INDUSTRI', 3032933, 315000, 157500),
    ('PAKET 4', 'CDM_CRM', 'VIP_INDUSTRI', 3079125, 315000, 157500),
    ('PAKET 5', 'CDM_CRM', 'VIP_INDUSTRI', 3575541, 315000, 157500),
    ('PAKET 6', 'CDM_CRM', 'VIP_INDUSTRI', 3850583, 315000, 157500)
) AS x(package_code, machine_group, price_class, base_price, add_cr_price, add_flm_price);

-- Abacus: bertier per jumlah kelolaan (1-50/51-100/101-150/151-200/201+),
-- tanpa Add CR/Add FLM terpisah - satu-satunya vendor dengan struktur ini.
INSERT INTO public.vendor_package_prices
    (vendor_id, package_code, machine_group, price_class, tier_min, tier_max, base_price, add_cr_price, add_flm_price, effective_start_date, effective_end_date)
SELECT v.id, x.package_code, x.machine_group, x.price_class, x.tier_min, x.tier_max, x.base_price, NULL, NULL, '2026-01-01', '2027-12-31'
FROM (SELECT id FROM public.vendors WHERE code = 'ABACUS') v
CROSS JOIN (VALUES
    ('PAKET 3', 'ATM', 'REGULAR', 1,   50,   2003100::numeric),
    ('PAKET 4', 'ATM', 'REGULAR', 1,   50,   2197000),
    ('PAKET 5', 'ATM', 'REGULAR', 1,   50,   2355400),
    ('PAKET 3', 'ATM', 'REGULAR', 51,  100,  1997200),
    ('PAKET 4', 'ATM', 'REGULAR', 51,  100,  2191000),
    ('PAKET 5', 'ATM', 'REGULAR', 51,  100,  2339500),
    ('PAKET 3', 'ATM', 'REGULAR', 101, 150,  1991200),
    ('PAKET 4', 'ATM', 'REGULAR', 101, 150,  2185100),
    ('PAKET 5', 'ATM', 'REGULAR', 101, 150,  2443600),
    ('PAKET 3', 'ATM', 'REGULAR', 151, 200,  1985300),
    ('PAKET 4', 'ATM', 'REGULAR', 151, 200,  2179200),
    ('PAKET 5', 'ATM', 'REGULAR', 151, 200,  2437600),
    ('PAKET 3', 'ATM', 'REGULAR', 201, NULL, 1944700),
    ('PAKET 4', 'ATM', 'REGULAR', 201, NULL, 2135100),
    ('PAKET 5', 'ATM', 'REGULAR', 201, NULL, 2389100),

    ('PAKET 3', 'ATM', 'VIP_INDUSTRI', 1,   50,   2197000),
    ('PAKET 4', 'ATM', 'VIP_INDUSTRI', 1,   50,   2584700),
    ('PAKET 5', 'ATM', 'VIP_INDUSTRI', 1,   50,   2713900),
    ('PAKET 3', 'ATM', 'VIP_INDUSTRI', 51,  100,  2191000),
    ('PAKET 4', 'ATM', 'VIP_INDUSTRI', 51,  100,  2578700),
    ('PAKET 5', 'ATM', 'VIP_INDUSTRI', 51,  100,  2708000),
    ('PAKET 3', 'ATM', 'VIP_INDUSTRI', 101, 150,  2185100),
    ('PAKET 4', 'ATM', 'VIP_INDUSTRI', 101, 150,  2572800),
    ('PAKET 5', 'ATM', 'VIP_INDUSTRI', 101, 150,  2702000),
    ('PAKET 3', 'ATM', 'VIP_INDUSTRI', 151, 200,  2179200),
    ('PAKET 4', 'ATM', 'VIP_INDUSTRI', 151, 200,  2566900),
    ('PAKET 5', 'ATM', 'VIP_INDUSTRI', 151, 200,  2696100),
    ('PAKET 3', 'ATM', 'VIP_INDUSTRI', 201, NULL, 2135100),
    ('PAKET 4', 'ATM', 'VIP_INDUSTRI', 201, NULL, 2516000),
    ('PAKET 5', 'ATM', 'VIP_INDUSTRI', 201, NULL, 2643000),

    ('PAKET 3', 'CDM_CRM', 'REGULAR', 1,   50,   2713000),
    ('PAKET 4', 'CDM_CRM', 'REGULAR', 1,   50,   2972300),
    ('PAKET 5', 'CDM_CRM', 'REGULAR', 1,   50,   3230800),
    ('PAKET 3', 'CDM_CRM', 'REGULAR', 51,  100,  2708000),
    ('PAKET 4', 'CDM_CRM', 'REGULAR', 51,  100,  2966400),
    ('PAKET 5', 'CDM_CRM', 'REGULAR', 51,  100,  3224900),
    ('PAKET 3', 'CDM_CRM', 'REGULAR', 101, 150,  2702000),
    ('PAKET 4', 'CDM_CRM', 'REGULAR', 101, 150,  2960500),
    ('PAKET 5', 'CDM_CRM', 'REGULAR', 101, 150,  3219000),
    ('PAKET 3', 'CDM_CRM', 'REGULAR', 151, 200,  2696100),
    ('PAKET 4', 'CDM_CRM', 'REGULAR', 151, 200,  2954600),
    ('PAKET 5', 'CDM_CRM', 'REGULAR', 151, 200,  3213000),
    ('PAKET 3', 'CDM_CRM', 'REGULAR', 201, NULL, 2643000),
    ('PAKET 4', 'CDM_CRM', 'REGULAR', 201, NULL, 2898900),
    ('PAKET 5', 'CDM_CRM', 'REGULAR', 201, NULL, 3150800),

    ('PAKET 3', 'CDM_CRM', 'VIP_INDUSTRI', 1,   50,   2972300),
    ('PAKET 4', 'CDM_CRM', 'VIP_INDUSTRI', 1,   50,   3230800),
    ('PAKET 5', 'CDM_CRM', 'VIP_INDUSTRI', 1,   50,   3489300),
    ('PAKET 3', 'CDM_CRM', 'VIP_INDUSTRI', 51,  100,  2966400),
    ('PAKET 4', 'CDM_CRM', 'VIP_INDUSTRI', 51,  100,  3224900),
    ('PAKET 5', 'CDM_CRM', 'VIP_INDUSTRI', 51,  100,  3483300),
    ('PAKET 3', 'CDM_CRM', 'VIP_INDUSTRI', 101, 150,  2960500),
    ('PAKET 4', 'CDM_CRM', 'VIP_INDUSTRI', 101, 150,  3219000),
    ('PAKET 5', 'CDM_CRM', 'VIP_INDUSTRI', 101, 150,  3477400),
    ('PAKET 3', 'CDM_CRM', 'VIP_INDUSTRI', 151, 200,  2954600),
    ('PAKET 4', 'CDM_CRM', 'VIP_INDUSTRI', 151, 200,  3213000),
    ('PAKET 5', 'CDM_CRM', 'VIP_INDUSTRI', 151, 200,  3471500),
    ('PAKET 3', 'CDM_CRM', 'VIP_INDUSTRI', 201, NULL, 2896900),
    ('PAKET 4', 'CDM_CRM', 'VIP_INDUSTRI', 201, NULL, 3150800),
    ('PAKET 5', 'CDM_CRM', 'VIP_INDUSTRI', 201, NULL, 3404800)
) AS x(package_code, machine_group, price_class, tier_min, tier_max, base_price);

-- SSI: hanya Paket 5 (ATM) dan Paket 4 (CDM/CRM) ada di dokumen, bertier per
-- jumlah mesin - batas tier BEDA antara ATM dan CDM/CRM (bukan salin-tempel).
INSERT INTO public.vendor_package_prices
    (vendor_id, package_code, machine_group, price_class, tier_min, tier_max, base_price, add_cr_price, add_flm_price, effective_start_date, effective_end_date)
SELECT v.id, x.package_code, x.machine_group, x.price_class, x.tier_min, x.tier_max, x.base_price, x.add_cr_price, x.add_flm_price, '2026-01-01', '2027-12-31'
FROM (SELECT id FROM public.vendors WHERE code = 'SSI') v
CROSS JOIN (VALUES
    -- Paket 5, ATM: 1-50 / 51-100 / 101-150 / 151-200 / 201-500
    ('PAKET 5', 'ATM', 'REGULAR',      1,   50,   5780000::numeric, 435000::numeric, 195000::numeric),
    ('PAKET 5', 'ATM', 'REGULAR',      51,  100,  3675000,          435000,          195000),
    ('PAKET 5', 'ATM', 'REGULAR',      101, 150,  3656625,          435000,          195000),
    ('PAKET 5', 'ATM', 'REGULAR',      151, 200,  3638342,          435000,          195000),
    ('PAKET 5', 'ATM', 'REGULAR',      201, 500,  3601958,          435000,          195000),
    ('PAKET 5', 'ATM', 'VIP_INDUSTRI', 1,   50,   6532000,          435000,          195000),
    ('PAKET 5', 'ATM', 'VIP_INDUSTRI', 51,  100,  4200000,          435000,          195000),
    ('PAKET 5', 'ATM', 'VIP_INDUSTRI', 101, 150,  4179000,          435000,          195000),
    ('PAKET 5', 'ATM', 'VIP_INDUSTRI', 151, 200,  4158105,          435000,          195000),
    ('PAKET 5', 'ATM', 'VIP_INDUSTRI', 201, 500,  4116524,          435000,          195000),

    -- Paket 4, CDM/CRM: <11 / 12-50 / 51-100 / 101-250 / 251-500
    ('PAKET 4', 'CDM_CRM', 'REGULAR',      1,   10,   7680000, 680000, 350000),
    ('PAKET 4', 'CDM_CRM', 'REGULAR',      12,  50,   4950000, 500000, 230000),
    ('PAKET 4', 'CDM_CRM', 'REGULAR',      51,  100,  4770000, 500000, 230000),
    ('PAKET 4', 'CDM_CRM', 'REGULAR',      101, 250,  3750000, 500000, 230000),
    ('PAKET 4', 'CDM_CRM', 'REGULAR',      251, 500,  3200000, 500000, 230000),
    ('PAKET 4', 'CDM_CRM', 'VIP_INDUSTRI', 1,   10,   8675000, 770000, 400000),
    ('PAKET 4', 'CDM_CRM', 'VIP_INDUSTRI', 12,  50,   5600000, 570000, 260000),
    ('PAKET 4', 'CDM_CRM', 'VIP_INDUSTRI', 51,  100,  5400000, 570000, 260000),
    ('PAKET 4', 'CDM_CRM', 'VIP_INDUSTRI', 101, 250,  4250000, 570000, 260000),
    ('PAKET 4', 'CDM_CRM', 'VIP_INDUSTRI', 251, 500,  3580000, 570000, 260000)
) AS x(package_code, machine_group, price_class, tier_min, tier_max, base_price, add_cr_price, add_flm_price);

-- ---------------------------------------------------------------------
-- 6. Daftar pengecualian (P18) - TIDAK diisi harga, sengaja dibiarkan kosong
--    dan terlihat lewat query ini (bukan tabel baru, bukan flag tersembunyi):
--
--    SELECT a.id, a.terminal_id, a.deployment_type, a.cimb_branch_id
--    FROM public.atms a
--    LEFT JOIN public.atm_vendor_packages avp ON avp.atm_id = a.id AND avp.is_active
--    WHERE (a.cimb_branch_id IS NOT NULL AND a.deployment_type = 'OFFSITE')
--       OR avp.id IS NULL;
--
--    ~18 ATM ROH berstatus OFFSITE (janggal - ROH semestinya ONSITE di kantor
--    cabang) + ~42 ATM tanpa kelolaan aktif sama sekali = ~60 baris. Dilarang
--    ditebak vendornya dari branch_coverage_areas; butuh verifikasi manusia.
-- ---------------------------------------------------------------------

COMMIT;
