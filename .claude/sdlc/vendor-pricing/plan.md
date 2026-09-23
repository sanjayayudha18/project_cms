# Plan: Harga Paket per Vendor FLM (desain ulang tabel paket)

Status: **desain disepakati (2026-09-22) — belum ada kode, belum ada migration.**
Sumber requirement: `archives/Harga Paket per vendor FLM.docx` (7 screenshot tabel harga, 5 vendor).
Menggantikan: `vendor_packages.price` + `vendor_packages.priority_class` dari baseline `001_baseline_schema.sql`.
ERD usulan: `.claude/sdlc/vendor-pricing/schema.dbml` (paste ke dbdiagram.io).
Skema yang berlaku sekarang: `backend/migrations/CMS_DB_dbdiagramio.dbml` — **jangan disunting**
sampai migration ini benar-benar dijalankan.

Catatan teknis dari DBML skema sekarang: `atm_vendor_packages` **sudah punya** EXCLUDE no-overlap
`(atm_id, daterange) WHERE is_active` dari migration 003 — tidak perlu dibuat ulang. `service_packages`
mewarisi `is_active` + `deleted_at` (migration 004 soft-disable) lewat RENAME, tidak perlu ditambah.

## Latar: kenapa didesain ulang

`vendor_packages(vendor_branch_id, code, priority_class, price)` hanya muat satu angka harga.
Dokumen memperlihatkan harga bergantung pada **enam** dimensi sekaligus: paket (frekuensi CR & FLM),
kelompok mesin, kelas prioritas, tier jumlah kelolaan, periode berlaku, dan pengecualian per-ATM.

Kondisi data saat ini mengonfirmasi tabelnya memang belum pernah dipakai sebagai daftar harga:
560 baris, **seluruhnya `price = 0.00` dan `priority_class = 'ALL'`**. Tabel itu selama ini
berfungsi sebagai peta kelolaan, bukan harga.

## Temuan data (dari `002_baseline_seed.sql`)

| Vendor | Cabang | Baris paket | ATM | Kode paket di DB | Di dokumen |
|---|---|---|---|---|---|
| ROH | 300 | 485 | 692 | Paket 4, 5 | — (internal) |
| TAG | 17 | 30 | 369 | Paket 3, 4, 5 | Paket 3, 4, 5, **6** |
| Advantage | 18 | 25 | 83 | Paket 3, 4, 5 | 3x4, 4x4, 5x5 / 3x6, 4x6, 5x6 |
| Abacus | 8 | 13 | 121 | Paket 4, 5 | Paket **3**, 4, 5 (+tier) |
| Bijak | **1** | 3 | **588** | Paket 3, 4, 5 | Paket 3, 4, 5 |
| SSI | 4 | 4 | 9 | Paket 5 | Paket 5 ATM, Paket 4 CRM (+tier) |

- **ROH bukan vendor FLM.** Ia penanda ATM yang berlokasi di kantor cabang CIMB. 300 "cabang vendor"-nya
  berisi nama kantor cabang CIMB (`ROH_001 = "Aceh - Bireun"`), dan 674 dari 692 ATM-nya berstatus ONSITE
  (97%), sementara lima vendor asli hampir seluruhnya OFFSITE.
- **Tidak ada tabel kantor cabang CIMB di skema.** `locations` berisi 1.892 baris, semuanya `type='ATM_SITE'`
  (satu lokasi per ATM). Jadi 300 baris ROH adalah satu-satunya tempat atribusi kantor cabang tersimpan.
- `atms.machine_type` punya **tiga** nilai (ATM 1.246, CRM 628, CDM 30); dokumen hanya mengenal **dua**
  kelompok tarif. `atms.priority_class` punya tiga nilai (Non VIP 1.734, VIP 128, Industri 42);
  dokumen menggabungkan VIP & Industri jadi satu tarif.
- Anomali: 18 ATM ROH berstatus OFFSITE, dan 42 ATM tanpa baris kelolaan sama sekali (37 ONSITE, 5 OFFSITE).

## Keputusan (product owner, 2026-09-22)

| # | Topik | Keputusan |
|---|---|---|
| P1 | Kelolaan vs harga | **Dipisah jadi dua konsep.** Kelolaan berlaku untuk semua 1.862 ATM; harga hanya untuk ~1.170 ATM yang divendorkan. |
| P2 | Grain harga | **Dasar per PT (`vendor_id`) + override opsional per cabang.** Bukan per-cabang murni. |
| P3 | Mekanisme override | **Per field, nullable + COALESCE.** Override boleh mengisi `base_price` saja; `add_cr`/`add_flm` tetap ikut tingkat di atasnya. |
| P4 | Struktur fisik 3 tingkat | **Satu tabel**, `vendor_branch_id` & `atm_id` nullable. Tingkat ditentukan kolom mana yang terisi. |
| P5 | Presedensi | **Paling spesifik menang**: ATM > cabang > PT (per field). |
| P6 | Biaya Tambahan Operasional | **Bukan tingkat presedensi** — kolom aditif, ditambahkan di atas harga manapun yang berlaku. |
| P7 | Periode override | **Setiap baris punya periode sendiri.** Sesuai dokumen Harga Khusus yang siklusnya berbeda dari kontrak PT. |
| P8 | Versioning harga | **Riwayat, effective-dated.** Semua periode hidup berdampingan; lookup pakai tanggal invoice. |
| P9 | Identitas paket | **Simpan keduanya**: `code` (tampilan) + `cr_frequency`/`flm_frequency` (perhitungan). |
| P10 | Lokasi frekuensi | **Generalisasi tabel paket**: `vendor_packages` -> `service_packages`, `vendor_branch_id` jadi nullable (NULL = paket internal). Frekuensi satu sumber kebenaran untuk ATM vendor maupun internal. |
| P11 | Pemetaan tipe mesin & kelas | **Kolom grup via generated column di `atms`**: `price_machine_group` (ATM / CDM_CRM), `price_class` (REGULAR / VIP_INDUSTRI). Bukan duplikat baris, bukan normalisasi data `atms`. |
| P12 | Harga tidak lengkap | **Error keras, invoice ditolak** dengan pesan menyebut kombinasi yang hilang. Tidak ada fallback nol. |
| P13 | Maker-checker | **Ikut pola master data yang ada** (D1 master-data): staged ke `master_data_change_requests`, apply setelah approval, endpoint balas 202. |
| P14 | RBAC | Internal: **role finance/admin saja** yang boleh lihat & ubah. Vendor: **boleh lihat harganya sendiri**, scoped ketat. |
| P15 | Status ROH | **Tetap ada di `vendors`**, ditandai `kind = 'INTERNAL'`. Tidak dihapus, tidak di-soft-delete. |
| P16 | 300 cabang CIMB | **Pindah ke tabel baru `cimb_branches`.** ROH tinggal sebagai entitas penanda tanpa cabang. |
| P17 | Visibilitas ROH | Disembunyikan dari daftar vendor admin (ada filter untuk menampilkan) · **tetap muncul** di dropdown assignment ATM sebagai pilihan internal · **tidak pernah** muncul di tabel harga. |
| P18 | 60 baris anomali | **Migrasi jalan terus**, 60 baris jadi daftar pengecualian untuk dibereskan manual. Dilarang menebak otomatis dari coverage area. |
| P19 | Pengisian data | **Seed sebagian**: vendor yang lengkap di dokumen di-seed lewat migration; SSI & Abacus Paket 3 diinput manual lewat layar. |
| P20 | Cakupan | Migration + backend + **layar CompanyPortal**. Sisi VendorPortal menyusul di pekerjaan terpisah (backend tetap menyiapkan scoping-nya). |

## Penyimpangan dari P1–P20 saat implementasi (2026-09-22)

Tiga hal ditemukan saat menulis `009_vendor_package_prices.sql` dan mengubah bentuk yang disepakati.
Maksud tiap keputusan tetap utuh; yang berubah cara mencapainya.

| # | Disepakati | Dijalankan | Alasan |
|---|---|---|---|
| P10 | `vendor_packages` → `service_packages` | **Rename dibatalkan.** Nama tetap `vendor_packages`; `vendor_branch_id` nullable tetap jalan | Rename menyentuh ~166 baris di ~20 file (queries, sqlc-generated, service, handler, frontend) tanpa nilai fungsional, dan membalik keputusan D4 ("nama tabel ikuti DB yang ada"). `COMMENT ON TABLE` menjelaskan kenapa namanya menyebut vendor padahal bisa internal. |
| P16 | ROH jadi entitas penanda **tanpa cabang** | ROH tetap punya **8 hub regional** (`ROH_H01`–`ROH_H08`); hanya **300** cabang `ROH_0xx` yang pindah | Migrasi 007 menambahkan 8 hub regional ("ROH Jawa Barat", "ROH Sumatera") dengan 171 baris `branch_coverage_areas` menggantung padanya. Hub bukan kantor cabang CIMB. ROH sebagai unit internal dengan 8 wilayah justru koheren. |
| P9/P10 | `cr_frequency` + `flm_frequency` sebagai kolom di tabel paket | Pindah ke tabel referensi **`package_frequencies (package_code, machine_group)`**, 8 baris | FLM berbeda menurut tipe mesin: `PAKET 3` ATM = CR 3 & **FLM 4**, `PAKET 3` CDM/CRM = CR 3 & **FLM 6** (konsisten di dokumen Bijak, TAG, Advantage). Satu baris paket bisa dipakai ATM maupun CRM di cabang yang sama, jadi kolom di sana akan berbohong untuk salah satunya. |

Pemecahan migration: **009 = struktur saja**, **010 = perpindahan data** (300 cabang ROH →
`cimb_branches`, 485 paket ROH → paket internal, backfill `atms.cimb_branch_id`, buang
`vendor_packages.price` + `priority_class`). Bagian yang sulit dibalik dipisah supaya bisa direview sendiri.

Konsekuensi: ERD di `schema.dbml` masih menggambarkan `service_packages` dengan kolom frekuensi —
**perlu disesuaikan** ke bentuk yang benar-benar dimigrasikan.

## Skema yang dituju

```sql
-- 1. vendors: bedakan vendor FLM dari unit internal
ALTER TABLE vendors ADD COLUMN kind text NOT NULL DEFAULT 'FLM_VENDOR'
  CHECK (kind IN ('FLM_VENDOR','INTERNAL'));
UPDATE vendors SET kind = 'INTERNAL' WHERE code = 'ROH';

-- 2. kantor cabang CIMB dapat rumah sendiri
CREATE TABLE cimb_branches (
    id          bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    branch_code text NOT NULL UNIQUE,
    branch_name text NOT NULL,
    region_id   bigint REFERENCES regions(id),
    is_active   boolean NOT NULL DEFAULT true,
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now(),
    deleted_at  timestamptz
);
ALTER TABLE atms ADD COLUMN cimb_branch_id bigint REFERENCES cimb_branches(id);

-- 3. paket digeneralisasi: berlaku untuk vendor maupun internal
ALTER TABLE vendor_packages RENAME TO service_packages;
ALTER TABLE service_packages
    ALTER COLUMN vendor_branch_id DROP NOT NULL,   -- NULL = paket internal
    DROP COLUMN price,
    DROP COLUMN priority_class,
    ADD COLUMN cr_frequency  integer NOT NULL,
    ADD COLUMN flm_frequency integer NOT NULL;

-- 4. pemetaan tarif dijaga Postgres, bukan kode aplikasi
ALTER TABLE atms
  ADD COLUMN price_machine_group text GENERATED ALWAYS AS
    (CASE WHEN machine_type = 'ATM' THEN 'ATM' ELSE 'CDM_CRM' END) STORED,
  ADD COLUMN price_class text GENERATED ALWAYS AS
    (CASE WHEN priority_class = 'Non VIP' THEN 'REGULAR' ELSE 'VIP_INDUSTRI' END) STORED;

-- 5. satu tabel harga, tiga tingkat
CREATE TABLE vendor_package_prices (
    id                   bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    vendor_id            bigint NOT NULL REFERENCES vendors(id),
    package_code         text   NOT NULL,          -- 'PAKET 4'
    machine_group        text   NOT NULL CHECK (machine_group IN ('ATM','CDM_CRM')),
    price_class          text   NOT NULL CHECK (price_class  IN ('REGULAR','VIP_INDUSTRI')),
    tier_min             integer NOT NULL DEFAULT 1,
    tier_max             integer,                  -- NULL = tanpa batas atas
    -- nullable: tingkat override boleh mengisi sebagian saja (P3)
    base_price           numeric(20,2),
    add_cr_price         numeric(20,2),
    add_flm_price        numeric(20,2),
    extra_amount         numeric(20,2) NOT NULL DEFAULT 0,  -- Biaya Tambahan Operasional (P6)
    -- tingkat ditentukan kolom mana yang terisi (P4)
    vendor_branch_id     bigint REFERENCES vendor_branches(id),
    atm_id               bigint REFERENCES atms(id),
    sla_note             text,
    currency             char(3) NOT NULL DEFAULT 'IDR',
    effective_start_date date NOT NULL,
    effective_end_date   date,
    created_at           timestamptz NOT NULL DEFAULT now(),
    updated_at           timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT vpp_one_level  CHECK (atm_id IS NULL OR vendor_branch_id IS NULL),
    CONSTRAINT vpp_tier_chk   CHECK (tier_max IS NULL OR tier_max >= tier_min),
    CONSTRAINT vpp_period_chk CHECK (effective_end_date IS NULL
                                     OR effective_end_date >= effective_start_date),
    -- COALESCE wajib: EXCLUDE memperlakukan NULL sebagai tidak-bertabrakan,
    -- sehingga baris tingkat-PT (dua kolom NULL) tidak akan terjaga tanpa ini
    CONSTRAINT vpp_no_overlap EXCLUDE USING gist (
        vendor_id WITH =, package_code WITH =, machine_group WITH =, price_class WITH =,
        COALESCE(vendor_branch_id, 0) WITH =,
        COALESCE(atm_id, 0) WITH =,
        int4range(tier_min, COALESCE(tier_max, 2147483647), '[]') WITH &&,
        daterange(effective_start_date, effective_end_date, '[]') WITH &&
    )
);
```

`btree_gist` sudah terpasang di baseline — `EXCLUDE` tidak menambah dependency.

## Aturan lookup harga

Untuk satu ATM pada satu tanggal:

1. Ambil `package_code` dari baris kelolaan aktif ATM itu, `machine_group` + `price_class` dari
   generated column di `atms`.
2. Ambil semua baris `vendor_package_prices` yang cocok dan aktif pada tanggal tsb, di ketiga tingkat.
3. Resolusi **per field**: `base_price` = nilai dari baris paling spesifik yang tidak NULL
   (ATM -> cabang -> PT). Idem `add_cr_price`, `add_flm_price`.
4. `extra_amount` **dijumlahkan**, tidak di-override.
5. `base_price` tidak terdefinisi di tingkat manapun => **error, baris invoice ditolak** (P12).
6. ATM internal (`service_packages.vendor_branch_id IS NULL`) tidak pernah sampai ke langkah ini —
   tidak ada baris harga yang bisa cocok. Aturan "internal tidak ditagih" terbaca dari skema,
   bukan dari aturan yang harus diingat orang.

## Belum terjawab — memblokir kalkulator, bukan skema

**Semantik tier "Penambahan jumlah kelolaan" (Abacus & SSI).** Marginal seperti bracket pajak
(mesin ke-51..100 di tarif band 2) atau flat (seluruh armada di band tempat total jatuh)? Dan
dihitung per vendor nasional, per cabang, atau per tipe mesin?

Skemanya identik untuk semua kemungkinan, jadi migration boleh jalan duluan. Tapi **tidak boleh ada
satu baris kalkulator invoice ditulis sebelum ini dikonfirmasi ke procurement.** Catatan: Bijak punya
588 ATM di satu cabang, jadi jawaban "per cabang" vs "per vendor" berbeda nyata di sana.

## Langkah berikutnya (status per 2026-09-22, setelah 010 ditulis)

**Sudah ada di disk:** `009_vendor_package_prices.sql` (struktur) · `010_vendor_package_prices_data.sql`
(perpindahan data + seed harga) · plan ini · `schema.dbml` (STALE, lihat catatan di bawah).
**Belum dijalankan ke DB manapun, belum diverifikasi.**

### 010_vendor_package_prices_data.sql — ditulis, belum dijalankan

Catatan koreksi saat menulis: filter cabang ROH pakai regex `^ROH_[0-9]{3}$`, BUKAN
`LIKE 'ROH_0%'` seperti draf awal di bawah — kode cabang naik sampai `ROH_300`, jadi
`LIKE 'ROH_0%'` hanya menangkap `ROH_001`–`ROH_099` dan melewatkan 201 baris. Diverifikasi
ulang lewat `002_baseline_seed.sql`: memang 300 baris `ROH_001`–`ROH_300`.

Enam bagian, sesuai urutan di file:

1. Isi `cimb_branches` dari **300** cabang `vendor_branches` (regex di atas, bukan 8 hub
   `ROH_H01`–`ROH_H08` dari migrasi 007). `region_id` sengaja NULL — `vendor_branches` tidak
   punya FK region yang bisa dipercaya untuk baris ROH, tidak ditebak.
2. Backfill `atms.cimb_branch_id` lewat `atm_vendor_packages` (aktif) → `vendor_packages` →
   `vendor_branches` (ROH_0xx) → `cimb_branches`, dijalankan SEBELUM langkah 3 selagi
   `vendor_branch_id` ROH masih utuh.
3. Kolapskan paket ROH jadi paket internal — data-driven (ambil kode paket ROH yang benar-benar
   ada, bukan hardcode "PAKET 4"/"PAKET 5"), repoint `atm_vendor_packages`, lalu soft-disable
   baris lama.
4. `ALTER TABLE vendor_packages DROP COLUMN price, DROP COLUMN priority_class`.
5. Seed harga PT (P19), periode 2026-01-01 s.d. 2027-12-31 (satu-satunya periode yang berlaku
   hari ini): **Advantage, Bijak, TAG** penuh (flat, tidak bertier) dan **Abacus** penuh
   (bertier per jumlah kelolaan, tanpa Add CR/FLM terpisah) — total ~112 baris ditranskripsi
   langsung dari gambar Bagian IV dokumen. **SSI** hanya Paket 4 (CDM/CRM) dan Paket 5 (ATM) —
   dokumen tidak memuat Paket 3/6 SSI, tidak ditebak. **Belum di-seed sama sekali** (butuh
   atribusi vendor per-ATM yang tidak ada di dokumen): Harga Khusus luar service area (17 ATM)
   dan Biaya Tambahan Operasional (3 ATM) — masuk lewat layar admin P20 setelah `atm_id`
   terverifikasi manusia, bukan lewat migrasi.
6. Daftar pengecualian (P18): tidak diisi apa pun (bukan tabel baru) — query verifikasi
   ditinggal sebagai komentar di file untuk siapa pun yang menjalankan UAT. ~18 ATM ROH
   berstatus OFFSITE + ~42 ATM tanpa kelolaan aktif = ~60 baris tanpa harga, dilarang ditebak
   dari `branch_coverage_areas`.

### Status per 2026-09-23

009 dan 010 sudah **dijalankan ke DB dev** dan diverifikasi (300 `cimb_branches`, 692 ATM
di-backfill, 2 paket internal, 485 paket ROH lama dinonaktifkan, 120 baris harga, 60 baris
pengecualian persis sesuai perkiraan). Satu bug ditemukan & diperbaiki saat menjalankan 009:
sentinel `tier_max` di constraint `vpp_no_overlap` memakai `2147483647` (INT_MAX), yang overflow
saat Postgres menganonikalkan batas atas inklusif — diganti `999999999` di migrasi dan di DB.

Backend P20 (queries → repo → service maker-checker → handler → `Applier`) **sudah ditulis**:
`vendor_package_prices_admin.sql`, `VendorPackagePriceAdminRepository`,
`VendorPackagePriceAdminService`, `VendorPackagePriceApplier`, `AdminVendorPackagePriceHandler`,
mount di `/api/v1/admin/vendors/{vendorID}/package-prices`, `go build`/`go vet`/`go test ./...`
hijau semua. Tidak ada `Enable` — baris harga adalah riwayat effective-dated, bukan entitas
on/off; `Disable` menutup periode (`effective_end_date = kemarin`), periode baru = baris baru.

Efek samping yang ditemukan & diperbaiki saat menulis backend ini: `sqlc generate` gagal total
karena `vendor_packages_admin.sql` dan `atm_assignments_admin.sql` masih mereferensikan
`priority_class`/`price` yang sudah dihapus 010. Diperbaiki: `vendor_packages` sekarang murni
`vendor_branch_id` + `code` (keduanya immutable), jadi `Update` untuk entity itu dihapus
seluruhnya (tidak ada lagi field yang bisa diedit).

### Layar CompanyPortal — SUDAH ditulis (2026-09-23)

Tab baru "Harga Paket" di `VendorDetailPage.tsx` (vendor-scoped, bukan sub-tab cabang seperti
"Paket" -- `vendor_package_prices.vendor_id` adalah FK langsung, `vendor_branch_id`/`atm_id`
cuma override opsional per baris). File baru: `PackagePricesPanel.tsx` (tabel + create/edit/akhiri),
`VendorPackagePriceFormDialog.tsx` (form; field grain diimmutable-kan di mode edit, sama pola
dengan `VendorPackageFormDialog`'s "Kode Paket"), `lib/vendorPackagePriceFormSchema.ts`. Ditambah
ke `types.ts`/`api.ts`/`hooks.ts`/`master-data/pending.ts` (entity type baru). Tidak ada tombol
"Aktifkan" -- baris harga riwayat effective-dated, bukan entitas on/off; "Akhiri" menutup periode.
`tsc --noEmit`, `biome check`, `vitest run` (17/17 existing tests), dan `npm run build` semua hijau.
**Verifikasi manual di browser belum dilakukan** (Golden Rule #10 — tugas user).

### Ditemukan saat mengerjakan layar ini: dua frontend lain jadi stale akibat migrasi 010

`vendor_packages` API tidak lagi mengembalikan `priority_class`/`price` (dijelaskan di bawah), tapi
dua file frontend masih mengasumsikan field itu ada:
- `PackagesPanel.tsx` + `VendorPackageFormDialog.tsx` (tab "Paket" per cabang): kolom
  "Kelas Prioritas"/"Harga" akan tampil kosong, dan tombol "Ubah" akan gagal — endpoint
  `PUT .../packages/{id}` sudah dihapus dari backend (tidak ada lagi field yang bisa diedit).
- `ATMAssignmentsDialog.tsx` (dialog kelolaan ATM): label "Paket" di tabel riwayat dan opsi
  dropdown paket menampilkan `undefined` untuk bagian `priority_class` — kosmetik, tidak fatal.

Belum diperbaiki (di luar cakupan "layar CompanyPortal vendor_package_prices"); perlu keputusan
terpisah: hapus field itu dari kedua file, atau ganti tampilannya.

### Belum dikerjakan

1. **Kalkulator invoice** (lookup harga 3 tingkat + Additional CR/FLM) — belum dibangun, dan
   sengaja diblokir sampai procurement mengonfirmasi semantik tier (marginal vs flat; dihitung
   per vendor nasional / per cabang / per tipe mesin). Bijak dengan 588 ATM di satu cabang
   membuat jawabannya beda nyata secara materiil.
2. **Seed harga yang belum lengkap** (butuh verifikasi manusia, sengaja tidak ditebak): SSI
   Paket 3/6 (tidak ada di dokumen), Harga Khusus luar service area (17 ATM), Biaya Tambahan
   Operasional (3 ATM) — ketiganya sekarang bisa diinput lewat layar admin di atas (tinggal isi
   `atm_id`/`vendor_branch_id` setelah diverifikasi manusia), bukan migrasi lanjutan.
3. **`backend/internal/handler/integration_test.go:165`** hanya memuat migrasi `001`+`002` —
   `003`–`010` (termasuk seluruh skema harga ini) tidak pernah dijalankan di DB tes. Gap
   pre-existing, `backend/CLAUDE.md` melarang perbaikan oportunistik di luar cakupan; perlu
   diputuskan terpisah.
4. **`schema.dbml`** (ERD proposal awal) masih stale — masih menampilkan `service_packages`
   dengan kolom `cr_frequency`/`flm_frequency` yang sudah tidak dipakai (frekuensi sekarang di
   `package_frequencies`, bukan kolom). Dokumentasi saja, tidak memengaruhi kode.
5. ~~Dua frontend stale (`PackagesPanel`/`VendorPackageFormDialog`, `ATMAssignmentsDialog`)~~ —
   **SUDAH diperbaiki (2026-09-23)**: kolom/field `priority_class`/`price` dan mode edit
   `vendor_packages` dihapus end-to-end (types → api → hooks → dialog → test); `ATMAssignmentsDialog`
   berhenti menampilkan `priority_class` paket. `go build`/`go vet`, `tsc --noEmit`, `biome check`,
   dan test kedua fitur (88/88) hijau.

### Dampak ke CLAUDE.md — SUDAH dikerjakan

`.claude/CLAUDE.md` Sec 3 (Master data) dan Sec 12 (Resolved Decisions) sudah diperbarui
2026-09-22 mengikuti penerapan migrasi ini.
