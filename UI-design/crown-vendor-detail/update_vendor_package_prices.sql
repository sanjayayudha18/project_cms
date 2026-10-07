-- =====================================================================
-- Migrasi: public.vendor_package_prices
-- 1) Kolom baru `package` diisi dari nilai lama `package_code` (PAKET 3, dst.)
-- 2) `package_code` dibangun ulang: PKG<nomor paket>_<kode vendor>_<sequence 3 digit>
--    Sequence dihitung per vendor mengikuti urutan id, dijamin tidak sama.
-- 3) `package_code` dijadikan UNIQUE.
--
-- Catatan: kolom kode vendor mengikuti kolom code pada tabel vendors/master_vendors
-- yang terpasang. Sesuaikan blok di bawah (2 opsi) dengan tabel yang benar.
--
-- CARA AMAN:
--   Jalankan di lingkungan uji dulu, lalu verifikasi dengan query VALIDASI
--   di bagian bawah sebelum COMMIT.
-- =====================================================================

BEGIN;

-- ---------------------------------------------------------------------
-- LANGKAH 1: kolom baru `package`, isi dari nilai lama package_code
-- ---------------------------------------------------------------------
ALTER TABLE public.vendor_package_prices
  ADD COLUMN IF NOT EXISTS package text;

UPDATE public.vendor_package_prices
SET package = package_code;

-- ---------------------------------------------------------------------
-- LANGKAH 2: bangun ulang package_code
--   Format: PKG<nomor paket>_<CodeVendor>_<sequence>, sequence 3 digit per vendor
--
--   OPSI A (dipakai di file ini): kode vendor = v.id (vendor_id), cocok bila
--   belum ada kolom code teks. Contoh hasil: PKG3_3_001
--
--   OPSI B (lebih bagus bila tabel vendor punya kolom code teks, mis. ABA):
--   pakai v.code. Contoh hasil: PKG3_ABA_001
--   Hapus komentar salah satu blok CTE di bawah sesuai skema tabel vendor:
--     * vendors(code)         -> join ke public.vendors
--     * master_vendors(vendor_code) -> join ke public.master_vendors
-- ---------------------------------------------------------------------

WITH numbered AS (
  SELECT p.id,
         p.vendor_id,
         p.package,
         ROW_NUMBER() OVER (PARTITION BY p.vendor_id ORDER BY p.id) AS seq
  FROM public.vendor_package_prices p
),
coded AS (
  SELECT n.id,
         n.vendor_id,
         n.package,
         n.seq,
         -- OPSI A: kode vendor = vendor_id
         n.vendor_id::text AS vendor_code

         -- OPSI B: kode vendor dari tabel master. Aktifkan salah satu, hapus OPSI A.
         -- (SELECT v.code FROM public.vendors v WHERE v.id = n.vendor_id) AS vendor_code
         -- (SELECT mv.vendor_code FROM public.master_vendors mv WHERE mv.id = n.vendor_id) AS vendor_code
  FROM numbered n
)
UPDATE public.vendor_package_prices p
SET package_code = 'PKG' || COALESCE(NULLIF(regexp_replace(c.package, '\D', '', 'g'), ''), 'X')
                   || '_' || c.vendor_code
                   || '_' || lpad(c.seq::text, 3, '0')
FROM coded c
WHERE p.id = c.id;

-- ---------------------------------------------------------------------
-- LANGKAH 3: jadikan package_code wajib & unik
-- ---------------------------------------------------------------------
ALTER TABLE public.vendor_package_prices
  ALTER COLUMN package_code SET NOT NULL,
  ADD CONSTRAINT vendor_package_prices_package_code_key UNIQUE (package_code);

-- ---------------------------------------------------------------------
-- VALIDASI (jalankan sebelum COMMIT, semua harus 0 / sesuai harapan)
-- ---------------------------------------------------------------------
-- duplikat package_code (harus 0 baris):
-- SELECT package_code, COUNT(*) FROM public.vendor_package_prices GROUP BY 1 HAVING COUNT(*) > 1;
-- package kosong / NULL (harus 0):
-- SELECT COUNT(*) FROM public.vendor_package_prices WHERE package IS NULL;
-- sampel hasil:
-- SELECT id, vendor_id, package_code, package FROM public.vendor_package_prices ORDER BY id LIMIT 10;

-- Kalau validasi OK -> COMMIT;  kalau tidak -> ROLLBACK;
COMMIT;

/*
======================================================================
 REFERENSI NILAI (dari data ekspor 24 Sep 2026, 120 baris, id 3-122)
 Berikut pemetaan id -> package_code baru dan package, sebagai kontrol.
 Pola yang dipakai: OPSI A (kode vendor = vendor_id).
======================================================================
-- id   3 -> PKG3_3_001     | package = PAKET 3
-- id   4 -> PKG4_3_002     | package = PAKET 4
-- id   5 -> PKG5_3_003     | package = PAKET 5
-- id   6 -> PKG3_3_004     | package = PAKET 3
-- id   7 -> PKG4_3_005     | package = PAKET 4
-- id   8 -> PKG5_3_006     | package = PAKET 5
-- id   9 -> PKG3_3_007     | package = PAKET 3
-- id  10 -> PKG4_3_008     | package = PAKET 4
-- id  11 -> PKG5_3_009     | package = PAKET 5
-- id  12 -> PKG3_3_010     | package = PAKET 3
-- id  13 -> PKG4_3_011     | package = PAKET 4
-- id  14 -> PKG5_3_012     | package = PAKET 5
-- id  15 -> PKG3_4_001     | package = PAKET 3
-- id  16 -> PKG4_4_002     | package = PAKET 4
-- id  17 -> PKG5_4_003     | package = PAKET 5
-- id  18 -> PKG3_4_004     | package = PAKET 3
-- id  19 -> PKG4_4_005     | package = PAKET 4
-- id  20 -> PKG5_4_006     | package = PAKET 5
-- id  21 -> PKG3_4_007     | package = PAKET 3
-- id  22 -> PKG4_4_008     | package = PAKET 4
-- id  23 -> PKG5_4_009     | package = PAKET 5
-- id  24 -> PKG3_4_010     | package = PAKET 3
-- id  25 -> PKG4_4_011     | package = PAKET 4
-- id  26 -> PKG5_4_012     | package = PAKET 5
-- id  27 -> PKG3_7_001     | package = PAKET 3
-- id  28 -> PKG4_7_002     | package = PAKET 4
-- id  29 -> PKG5_7_003     | package = PAKET 5
-- id  30 -> PKG6_7_004     | package = PAKET 6
-- id  31 -> PKG3_7_005     | package = PAKET 3
-- id  32 -> PKG4_7_006     | package = PAKET 4
-- id  33 -> PKG5_7_007     | package = PAKET 5
-- id  34 -> PKG6_7_008     | package = PAKET 6
-- id  35 -> PKG3_7_009     | package = PAKET 3
-- id  36 -> PKG4_7_010     | package = PAKET 4
-- id  37 -> PKG5_7_011     | package = PAKET 5
-- id  38 -> PKG6_7_012     | package = PAKET 6
-- id  39 -> PKG3_7_013     | package = PAKET 3
-- id  40 -> PKG4_7_014     | package = PAKET 4
-- id  41 -> PKG5_7_015     | package = PAKET 5
-- id  42 -> PKG6_7_016     | package = PAKET 6
-- id  43 -> PKG3_2_001     | package = PAKET 3
-- id  44 -> PKG4_2_002     | package = PAKET 4
-- id  45 -> PKG5_2_003     | package = PAKET 5
-- id  46 -> PKG3_2_004     | package = PAKET 3
-- id  47 -> PKG4_2_005     | package = PAKET 4
-- id  48 -> PKG5_2_006     | package = PAKET 5
-- id  49 -> PKG3_2_007     | package = PAKET 3
-- id  50 -> PKG4_2_008     | package = PAKET 4
-- id  51 -> PKG5_2_009     | package = PAKET 5
-- id  52 -> PKG3_2_010     | package = PAKET 3
-- id  53 -> PKG4_2_011     | package = PAKET 4
-- id  54 -> PKG5_2_012     | package = PAKET 5
-- id  55 -> PKG3_2_013     | package = PAKET 3
-- id  56 -> PKG4_2_014     | package = PAKET 4
-- id  57 -> PKG5_2_015     | package = PAKET 5
-- id  58 -> PKG3_2_016     | package = PAKET 3
-- id  59 -> PKG4_2_017     | package = PAKET 4
-- id  60 -> PKG5_2_018     | package = PAKET 5
-- id  61 -> PKG3_2_019     | package = PAKET 3
-- id  62 -> PKG4_2_020     | package = PAKET 4
-- id  63 -> PKG5_2_021     | package = PAKET 5
-- id  64 -> PKG3_2_022     | package = PAKET 3
-- id  65 -> PKG4_2_023     | package = PAKET 4
-- id  66 -> PKG5_2_024     | package = PAKET 5
-- id  67 -> PKG3_2_025     | package = PAKET 3
-- id  68 -> PKG4_2_026     | package = PAKET 4
-- id  69 -> PKG5_2_027     | package = PAKET 5
-- id  70 -> PKG3_2_028     | package = PAKET 3
-- id  71 -> PKG4_2_029     | package = PAKET 4
-- id  72 -> PKG5_2_030     | package = PAKET 5
-- id  73 -> PKG3_2_031     | package = PAKET 3
-- id  74 -> PKG4_2_032     | package = PAKET 4
-- id  75 -> PKG5_2_033     | package = PAKET 5
-- id  76 -> PKG3_2_034     | package = PAKET 3
-- id  77 -> PKG4_2_035     | package = PAKET 4
-- id  78 -> PKG5_2_036     | package = PAKET 5
-- id  79 -> PKG3_2_037     | package = PAKET 3
-- id  80 -> PKG4_2_038     | package = PAKET 4
-- id  81 -> PKG5_2_039     | package = PAKET 5
-- id  82 -> PKG3_2_040     | package = PAKET 3
-- id  83 -> PKG4_2_041     | package = PAKET 4
-- id  84 -> PKG5_2_042     | package = PAKET 5
-- id  85 -> PKG3_2_043     | package = PAKET 3
-- id  86 -> PKG4_2_044     | package = PAKET 4
-- id  87 -> PKG5_2_045     | package = PAKET 5
-- id  88 -> PKG3_2_046     | package = PAKET 3
-- id  89 -> PKG4_2_047     | package = PAKET 4
-- id  90 -> PKG5_2_048     | package = PAKET 5
-- id  91 -> PKG3_2_049     | package = PAKET 3
-- id  92 -> PKG4_2_050     | package = PAKET 4
-- id  93 -> PKG5_2_051     | package = PAKET 5
-- id  94 -> PKG3_2_052     | package = PAKET 3
-- id  95 -> PKG4_2_053     | package = PAKET 4
-- id  96 -> PKG5_2_054     | package = PAKET 5
-- id  97 -> PKG3_2_055     | package = PAKET 3
-- id  98 -> PKG4_2_056     | package = PAKET 4
-- id  99 -> PKG5_2_057     | package = PAKET 5
-- id 100 -> PKG3_2_058     | package = PAKET 3
-- id 101 -> PKG4_2_059     | package = PAKET 4
-- id 102 -> PKG5_2_060     | package = PAKET 5
-- id 103 -> PKG5_1_001     | package = PAKET 5
-- id 104 -> PKG5_1_002     | package = PAKET 5
-- id 105 -> PKG5_1_003     | package = PAKET 5
-- id 106 -> PKG5_1_004     | package = PAKET 5
-- id 107 -> PKG5_1_005     | package = PAKET 5
-- id 108 -> PKG5_1_006     | package = PAKET 5
-- id 109 -> PKG5_1_007     | package = PAKET 5
-- id 110 -> PKG5_1_008     | package = PAKET 5
-- id 111 -> PKG5_1_009     | package = PAKET 5
-- id 112 -> PKG5_1_010     | package = PAKET 5
-- id 113 -> PKG4_1_011     | package = PAKET 4
-- id 114 -> PKG4_1_012     | package = PAKET 4
-- id 115 -> PKG4_1_013     | package = PAKET 4
-- id 116 -> PKG4_1_014     | package = PAKET 4
-- id 117 -> PKG4_1_015     | package = PAKET 4
-- id 118 -> PKG4_1_016     | package = PAKET 4
-- id 119 -> PKG4_1_017     | package = PAKET 4
-- id 120 -> PKG4_1_018     | package = PAKET 4
-- id 121 -> PKG4_1_019     | package = PAKET 4
-- id 122 -> PKG4_1_020     | package = PAKET 4
*/
