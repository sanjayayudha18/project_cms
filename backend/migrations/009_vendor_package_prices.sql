-- 009_vendor_package_prices.sql
-- Harga paket vendor FLM: struktur saja, TANPA perpindahan data.
-- Sumber requirement: archives/Harga Paket per vendor FLM.docx
-- Keputusan P1-P20: .claude/sdlc/vendor-pricing/plan.md · ERD: schema.dbml
--
-- Kenapa: vendor_packages.price hanya muat satu angka, padahal harga bergantung
-- pada enam dimensi (paket, kelompok mesin, kelas prioritas, tier jumlah kelolaan,
-- periode berlaku, pengecualian per-ATM). Bukti tabel itu belum pernah dipakai
-- sebagai daftar harga: 560 baris, SELURUHNYA price=0.00 dan priority_class='ALL'.
--
-- Perpindahan data (300 cabang ROH -> cimb_branches, 485 paket ROH -> paket
-- internal, backfill atms.cimb_branch_id) sengaja DIPISAH ke 010 supaya bagian
-- yang sulit dibalik bisa direview sendiri. Setelah 009 jalan, kolom price dan
-- priority_class di vendor_packages masih ada dan belum dibuang - 010 yang
-- membuangnya, setelah datanya dipindah.
--
-- CATATAN NAMA: tabel tetap bernama vendor_packages, TIDAK di-rename jadi
-- service_packages seperti draf P10. Rename menyentuh ~166 baris di ~20 file
-- (queries, sqlc-generated, service, handler, frontend) tanpa memberi nilai
-- fungsional apa pun, dan membalik keputusan D4 ("nama tabel ikuti DB yang ada").
-- Substansi P10 - vendor_branch_id nullable - tetap dijalankan di bawah.

BEGIN;

-- ---------------------------------------------------------------------
-- 1. Bedakan vendor FLM dari unit internal (P15).
--    ROH bukan vendor FLM: ia penanda ATM yang berlokasi di kantor cabang CIMB.
--    Tetap ada sebagai entitas yang bisa dirujuk, tidak dihapus, tidak di-soft-delete.
-- ---------------------------------------------------------------------

ALTER TABLE public.vendors
    ADD COLUMN IF NOT EXISTS kind text NOT NULL DEFAULT 'FLM_VENDOR';

ALTER TABLE public.vendors
    DROP CONSTRAINT IF EXISTS vendors_kind_chk;

ALTER TABLE public.vendors
    ADD CONSTRAINT vendors_kind_chk CHECK (kind IN ('FLM_VENDOR', 'INTERNAL'));

UPDATE public.vendors SET kind = 'INTERNAL', updated_at = now() WHERE code = 'ROH';

COMMENT ON COLUMN public.vendors.kind
    IS 'FLM_VENDOR | INTERNAL. ROH = INTERNAL (ATM di kantor cabang CIMB, diisi petugas cabang sendiri, tidak ditagih). Unit INTERNAL tidak pernah punya baris di vendor_package_prices. Migrasi 009.';

-- ---------------------------------------------------------------------
-- 2. Kantor cabang CIMB dapat rumah sendiri (P16).
--    Sebelum ini tidak ada tabel kantor cabang di skema: locations seluruhnya
--    bertipe ATM_SITE (satu lokasi per ATM), sehingga 300 baris ROH_0xx di
--    vendor_branches adalah satu-satunya tempat atribusi kantor cabang tersimpan.
--
--    CATATAN: hanya 300 cabang ROH_0xx (per-lokasi) yang pindah ke sini.
--    8 hub regional ROH_H01-ROH_H08 dari migrasi 007 BUKAN kantor cabang -
--    mereka pengelompokan wilayah dengan 171 baris branch_coverage_areas
--    menggantung padanya, dan tetap tinggal sebagai cabang ROH.
-- ---------------------------------------------------------------------

CREATE TABLE IF NOT EXISTS public.cimb_branches (
    id          bigint      GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    branch_code text        NOT NULL,
    branch_name text        NOT NULL,
    region_id   bigint      REFERENCES public.regions(id),
    is_active   boolean     NOT NULL DEFAULT true,
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now(),
    deleted_at  timestamptz,
    CONSTRAINT uq_cimb_branches_code UNIQUE (branch_code)
);

COMMENT ON TABLE public.cimb_branches
    IS 'Kantor cabang CIMB tempat ATM internal berdiri. Diisi dari 300 baris ROH_0xx di vendor_branches (migrasi 010). Bukan cabang vendor. Migrasi 009.';

ALTER TABLE public.atms
    ADD COLUMN IF NOT EXISTS cimb_branch_id bigint REFERENCES public.cimb_branches(id);

CREATE INDEX IF NOT EXISTS atms_cimb_branch_id_idx
    ON public.atms (cimb_branch_id) WHERE cimb_branch_id IS NOT NULL;

COMMENT ON COLUMN public.atms.cimb_branch_id
    IS 'Terisi untuk ATM internal (dikelola petugas cabang CIMB, tidak ditagih); NULL untuk ATM yang divendorkan. Di-backfill di migrasi 010.';

-- ---------------------------------------------------------------------
-- 3. Jembatan tarif: 3 nilai machine_type dan 3 nilai priority_class di DB
--    vs 2x2 kelompok tarif di dokumen (P11).
--
--    Generated column, bukan duplikat baris harga: kalau tarif CRM dan CDM
--    disimpan sebagai dua baris kembar, mengubah satu dan lupa yang lain tidak
--    akan tertangkap constraint apa pun - untuk data finansial itu mode
--    kegagalan yang paling mahal.
--
--    Nilai yang tidak dikenal sengaja dipetakan ke NULL, bukan ke salah satu
--    grup: tipe mesin baru akan gagal keras saat lookup harga (P12) alih-alih
--    diam-diam ditagih dengan tarif yang salah.
-- ---------------------------------------------------------------------

ALTER TABLE public.atms
    ADD COLUMN IF NOT EXISTS price_machine_group text
        GENERATED ALWAYS AS (
            CASE machine_type
                WHEN 'ATM' THEN 'ATM'
                WHEN 'CRM' THEN 'CDM_CRM'
                WHEN 'CDM' THEN 'CDM_CRM'
                ELSE NULL
            END
        ) STORED;

ALTER TABLE public.atms
    ADD COLUMN IF NOT EXISTS price_class text
        GENERATED ALWAYS AS (
            CASE priority_class
                WHEN 'Non VIP'  THEN 'REGULAR'
                WHEN 'VIP'      THEN 'VIP_INDUSTRI'
                WHEN 'Industri' THEN 'VIP_INDUSTRI'
                ELSE NULL
            END
        ) STORED;

COMMENT ON COLUMN public.atms.price_machine_group
    IS 'Turunan machine_type untuk lookup harga: ATM -> ATM; CRM/CDM -> CDM_CRM; lainnya -> NULL (gagal keras). CEILING: mengunci asumsi tidak ada vendor yang menghargai CDM beda dari CRM - benar untuk kelima vendor di dokumen harga. Migrasi 009.';

COMMENT ON COLUMN public.atms.price_class
    IS 'Turunan priority_class untuk lookup harga: Non VIP -> REGULAR; VIP/Industri -> VIP_INDUSTRI; lainnya/NULL -> NULL (gagal keras). Dokumen harga menyatukan VIP & Industri dalam satu tarif. Migrasi 009.';

-- ---------------------------------------------------------------------
-- 4. Paket berlaku untuk ATM vendor MAUPUN internal (P10).
--    vendor_branch_id jadi nullable: NULL = paket internal, tanpa vendor dan
--    tanpa harga. Ini yang membuat aturan "internal tidak ditagih" terbaca dari
--    skema - tidak ada baris harga yang bisa cocok dengan paket tanpa vendor -
--    bukan jadi aturan yang harus diingat orang.
--
--    price dan priority_class BELUM dibuang di sini; lihat migrasi 010.
-- ---------------------------------------------------------------------

ALTER TABLE public.vendor_packages
    ALTER COLUMN vendor_branch_id DROP NOT NULL;

COMMENT ON TABLE public.vendor_packages
    IS 'Paket layanan (frekuensi kontrak) yang dipetakan ke ATM lewat atm_vendor_packages. Namanya menyebut "vendor" karena alasan historis (keputusan D4: nama tabel ikuti DB yang ada), tetapi sejak migrasi 009 baris dengan vendor_branch_id NULL adalah paket INTERNAL - dipakai ATM di kantor cabang CIMB, tanpa vendor dan tanpa harga. Harga ada di vendor_package_prices, frekuensi di package_frequencies.';

COMMENT ON COLUMN public.vendor_packages.vendor_branch_id
    IS 'NULL = paket internal (ATM kantor cabang CIMB, tidak ditagih). Terisi = paket kontrak vendor FLM. Migrasi 009.';

-- ---------------------------------------------------------------------
-- 5. Frekuensi CR & FLM per (kode paket, kelompok mesin).
--
--    KENAPA TABEL SENDIRI, bukan kolom di vendor_packages: frekuensi FLM
--    berbeda menurut tipe mesin. Dokumen Bijak, TAG, dan Advantage sepakat -
--    PAKET 3 untuk ATM = CR 3 & FLM 4, tapi PAKET 3 untuk CDM/CRM = CR 3 & FLM 6.
--    Satu baris vendor_packages bisa dipakai ATM maupun CRM di cabang yang sama,
--    jadi kolom frekuensi di sana akan berbohong untuk salah satunya.
--
--    Berlaku untuk paket vendor maupun internal: machine_group diambil dari
--    atms.price_machine_group, yang ada untuk semua ATM.
--
--    Dipakai untuk menghitung tagihan Additional CR / Additional FLM
--    (realisasi trip dikurangi frekuensi kontrak).
-- ---------------------------------------------------------------------

CREATE TABLE IF NOT EXISTS public.package_frequencies (
    package_code  text        NOT NULL,
    machine_group text        NOT NULL,
    cr_frequency  integer     NOT NULL,
    flm_frequency integer     NOT NULL,
    created_at    timestamptz NOT NULL DEFAULT now(),
    updated_at    timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT pk_package_frequencies PRIMARY KEY (package_code, machine_group),
    CONSTRAINT package_frequencies_machine_group_chk
        CHECK (machine_group IN ('ATM', 'CDM_CRM')),
    CONSTRAINT package_frequencies_cr_chk  CHECK (cr_frequency  > 0),
    CONSTRAINT package_frequencies_flm_chk CHECK (flm_frequency > 0)
);

COMMENT ON TABLE public.package_frequencies
    IS 'Frekuensi CR & FLM per bulan untuk tiap (kode paket, kelompok mesin). Grainnya (kode, kelompok mesin) - bukan kolom di vendor_packages - karena FLM berbeda menurut tipe mesin: PAKET 3 ATM = FLM 4, PAKET 3 CDM/CRM = FLM 6. Nilai dari archives/Harga Paket per vendor FLM.docx. Migrasi 009.';

INSERT INTO public.package_frequencies (package_code, machine_group, cr_frequency, flm_frequency) VALUES
    ('PAKET 3', 'ATM',     3, 4),
    ('PAKET 4', 'ATM',     4, 4),
    ('PAKET 5', 'ATM',     5, 5),
    ('PAKET 6', 'ATM',     6, 6),
    ('PAKET 3', 'CDM_CRM', 3, 6),
    ('PAKET 4', 'CDM_CRM', 4, 6),
    ('PAKET 5', 'CDM_CRM', 5, 6),
    ('PAKET 6', 'CDM_CRM', 6, 6)
ON CONFLICT (package_code, machine_group) DO NOTHING;

-- ---------------------------------------------------------------------
-- 6. Harga: satu tabel, tiga tingkat (P2, P3, P4, P5, P6, P7, P8).
--
--    Tingkat ditentukan kolom mana yang terisi:
--      vendor_branch_id NULL & atm_id NULL  -> tingkat PT      (dasar)
--      vendor_branch_id terisi              -> override cabang
--      atm_id terisi                        -> override ATM (Harga Khusus
--                                              di luar service area)
--
--    Kolom harga NULLABLE karena override berlaku PER FIELD: override boleh
--    mengisi base_price saja, sementara add_cr/add_flm tetap ikut tingkat di
--    atasnya. Konsekuensinya kelengkapan harga TIDAK bisa dijamin constraint -
--    itu urusan runtime, dan P12 mewajibkan gagal keras, bukan fallback nol.
-- ---------------------------------------------------------------------

CREATE TABLE IF NOT EXISTS public.vendor_package_prices (
    id                   bigint        GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    vendor_id            bigint        NOT NULL REFERENCES public.vendors(id),
    package_code         text          NOT NULL,
    machine_group        text          NOT NULL,
    price_class          text          NOT NULL,
    tier_min             integer       NOT NULL DEFAULT 1,
    tier_max             integer,
    base_price           numeric(20,2),
    add_cr_price         numeric(20,2),
    add_flm_price        numeric(20,2),
    extra_amount         numeric(20,2) NOT NULL DEFAULT 0,
    vendor_branch_id     bigint        REFERENCES public.vendor_branches(id),
    atm_id               bigint        REFERENCES public.atms(id),
    sla_note             text,
    currency             char(3)       NOT NULL DEFAULT 'IDR',
    effective_start_date date          NOT NULL,
    effective_end_date   date,
    created_at           timestamptz   NOT NULL DEFAULT now(),
    updated_at           timestamptz   NOT NULL DEFAULT now(),

    CONSTRAINT vpp_machine_group_chk CHECK (machine_group IN ('ATM', 'CDM_CRM')),
    CONSTRAINT vpp_price_class_chk   CHECK (price_class  IN ('REGULAR', 'VIP_INDUSTRI')),

    -- satu baris tidak boleh jadi override cabang dan override ATM sekaligus
    CONSTRAINT vpp_one_level_chk CHECK (atm_id IS NULL OR vendor_branch_id IS NULL),

    CONSTRAINT vpp_tier_chk   CHECK (tier_min >= 1 AND (tier_max IS NULL OR tier_max >= tier_min)),
    CONSTRAINT vpp_period_chk CHECK (effective_end_date IS NULL
                                     OR effective_end_date >= effective_start_date),

    -- uang tidak boleh negatif; NULL tetap boleh (artinya "ikut tingkat di atas")
    CONSTRAINT vpp_amount_chk CHECK (
        (base_price    IS NULL OR base_price    >= 0) AND
        (add_cr_price  IS NULL OR add_cr_price  >= 0) AND
        (add_flm_price IS NULL OR add_flm_price >= 0) AND
        extra_amount >= 0
    )
);

-- Cegah dua harga cocok untuk kombinasi + tanggal yang sama, per tingkat.
--
-- COALESCE WAJIB: EXCLUDE memperlakukan NULL sebagai tidak-bertabrakan (sama
-- seperti UNIQUE), sehingga tanpa COALESCE baris tingkat-PT - yang justru punya
-- dua kolom NULL - tidak akan terjaga sama sekali. Nilai 0 aman sebagai sentinel
-- karena kedua kolom ber-FK ke identity PK yang mulai dari 1.
--
-- Sentinel tier_max sengaja 999999999, BUKAN 2147483647 (INT_MAX): int4range
-- dengan batas atas inklusif '[]' dikanonikalisasi Postgres dengan +1 pada batas
-- atas - kalau sentinel-nya INT_MAX, +1 itu overflow ("integer out of range").
-- 999999999 jauh di atas jumlah kelolaan realistis mana pun, dan +1 masih aman.
--
-- btree_gist sudah terpasang sejak baseline 001, jadi tidak ada dependency baru.
ALTER TABLE public.vendor_package_prices
    DROP CONSTRAINT IF EXISTS vpp_no_overlap;

ALTER TABLE public.vendor_package_prices
    ADD CONSTRAINT vpp_no_overlap EXCLUDE USING gist (
        vendor_id                     WITH =,
        package_code                  WITH =,
        machine_group                 WITH =,
        price_class                   WITH =,
        COALESCE(vendor_branch_id, 0) WITH =,
        COALESCE(atm_id, 0)           WITH =,
        int4range(tier_min, COALESCE(tier_max, 999999999), '[]') WITH &&,
        daterange(effective_start_date, effective_end_date, '[]') WITH &&
    );

CREATE INDEX IF NOT EXISTS vpp_lookup_idx
    ON public.vendor_package_prices (vendor_id, package_code, machine_group, price_class);

CREATE INDEX IF NOT EXISTS vpp_branch_idx
    ON public.vendor_package_prices (vendor_branch_id) WHERE vendor_branch_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS vpp_atm_idx
    ON public.vendor_package_prices (atm_id) WHERE atm_id IS NOT NULL;

COMMENT ON TABLE public.vendor_package_prices
    IS 'Harga kontrak vendor FLM, tiga tingkat dalam satu tabel (PT / cabang / ATM), effective-dated dengan riwayat. Unit INTERNAL tidak pernah punya baris di sini. Perubahan lewat maker-checker (master_data_change_requests). Akses: role finance/admin internal, dan vendor hanya harganya sendiri. Migrasi 009.';

COMMENT ON COLUMN public.vendor_package_prices.extra_amount
    IS 'Biaya Tambahan Operasional: ADITIF - dijumlahkan di atas harga tingkat manapun yang berlaku, tidak pernah menggantikannya.';

COMMENT ON COLUMN public.vendor_package_prices.atm_id
    IS 'Terisi = Harga Khusus di luar service area untuk ATM ini. Punya periode berlaku sendiri, tidak mengikuti periode kontrak PT.';

COMMENT ON COLUMN public.vendor_package_prices.base_price
    IS 'NULLABLE: override tingkat cabang/ATM boleh mengisi sebagian field saja; yang NULL diambil dari tingkat di atasnya (ATM -> cabang -> PT). Tidak terdefinisi di tingkat manapun = error, baris invoice ditolak.';

COMMIT;
