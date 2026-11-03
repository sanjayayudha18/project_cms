# Spec: Nomor tiket replenish per ATM + nomor request per region

Status: **accepted 2026-10-08 (PO, via chat: "setuju, lanjut plan")** — nomor request
`REP-<vendor>-<REGION>-<YYYYMMDD>-<NNN>`, tiket FK ke `request_number`, kode region vendor cabang. Bagian tiket (format, granularitas, FR9 gabung hasil laporan) sudah
accepted 2026-10-08 dan tidak berubah isinya — hanya kuncinya pindah dari `vendor_request_id` ke `request_number`.
Input: `intent.md` (Q1–Q6, "Revisi format", "Revisi granularitas", "Revisi nomor request + relasi tiket").
Sec 4 #7: **skema + migrasi data (backfill, seed master data, DROP TABLE)**. Bukan money/recon/auth.
Menyentuh master data `vendor_branches` → maker-checker (Sec 12 D1–D4).

## Definisi
- **Nomor request** (baru) = `REP-<prefix vendor>-<kode region>-<YYYYMMDD>-<NNN>`, contoh `REP-BJK-JKT-20261008-001`.
  - prefix vendor: tidak berubah (`vendors.request_prefix`, fallback dari `vendors.code`).
  - kode region: `vendor_branches.region_code` dari vendor cabang yang mengelola ATM di request (FR11).
  - `YYYYMMDD` = `replenish_date`; `NNN` = 001–999 per **(vendor, kode region, tanggal replenish)**.
- **Tiket** = satu tiket aktif per **(request, ATM)** = satu jalan vendor. Nomor tiket
  `<terminal_id>_<kode denom>_<YYYYMMDD>_<NNN>`:
  - kode denom: satu denom → `denom / 1000` + `K` (`50K`, `100K`); lebih dari satu denom → `MIX`.
  - `YYYYMMDD` = `replenish_date` request; `NNN` = 001–999 per **(terminal_id, tanggal replenish)**, global lintas
    request (kunjungan ke-berapa ke ATM itu pada tanggal itu, termasuk request batal/ditolak).
- Contoh (ATM `Y18X`, 8 Okt 2026, vendor Bijak cabang Jakarta kode `JKT`):

  | Situasi | Nomor request | No. Tiket |
  |---|---|---|
  | request A, hanya 100rb | `REP-BJK-JKT-20261008-001` | `Y18X_100K_20261008_001` |
  | request B (emergency), 50rb + 100rb | `REP-BJK-JKT-20261008-002` | `Y18X_MIX_20261008_002` |
  | B dibatalkan, request C hanya 50rb | `REP-BJK-JKT-20261008-003` | `Y18X_50K_20261008_003` |
- Tiket **tidak menggantikan** rincian per denom: nominal tetap per baris `vendor_request_items` (ATM × periode ×
  denom). Laporan selesai per tiket = per ATM (FR9) — PO: "tidak bisa kalau gagal gagal semua".
- **Tiket aktif** = tiket (request, ATM) yang berlaku sekarang. **Non-aktif** = ATM dihapus dari request atau kode
  denomnya berubah; baris dan nomornya tetap disimpan dan **tidak pernah dipakai ulang** (Q1).

## Functional requirements

### FR10 Nomor request per region (`createWithRetryingNumber`)
- FR10.1 Saat create: tentukan vendor cabang tiap ATM (kelolaan aktif per `periode_pred`, query yang sama dengan
  `validateItemsSingleVendor`), ambil `region_code`-nya. Semua ATM harus menghasilkan **satu** kode region.
- FR10.2 Lebih dari satu kode → 422 `ValidationError{Field:"items", Message:"ATM dalam satu request harus dari region
  vendor yang sama: <kode>: <terminal,…>; <kode>: <…>"}`. Cabang tanpa `region_code` → 422 `"region vendor cabang
  <branch_code> belum punya kode region"`.
- FR10.3 Kode region disimpan di `vendor_requests.region_code` (snapshot; tidak berubah walau master data diedit).
- FR10.4 `UpdateItems`: ATM baru harus berkode region sama dengan `vendor_requests.region_code` → selain itu 422
  seperti FR10.2. Nomor request tidak pernah berubah. Request lama (`region_code` NULL) → **tanpa cek region**, hanya
  cek vendor seperti sekarang (PO 2026-10-08).
- FR10.5 Counter `vendor_request_number_seq` dikunci per `(vendor_id, region_code, seq_date)`; atomik, batas 999
  (`ErrNumberExhausted`) — perilaku sama dengan sekarang, hanya kuncinya bertambah region.
- FR10.6 Request lama (`REP-<vendor>-<YYYYMMDD>-<NNN>`, `VR-…`, `REPABA…`) tidak ditulis ulang; `region_code`-nya NULL.
- FR10.7 Berlaku untuk semua jalur create (manual + Forecast Browser, planned/emergency/additional).

### FR11 Kode region vendor cabang (master data)
- FR11.1 Kolom `vendor_branches.region_code` (huruf besar/angka, 2–10 karakter), nullable.
- FR11.2 Diubah lewat alur master data vendor cabang yang sudah ada (create/update → `Submit` → approval → `Applier`,
  audit), form CompanyPortal + CSV import/export vendor branch ikut kolom ini (file lama tanpa kolom tetap valid).
- FR11.3 Seed awal di migrasi dari teks `vendor_branches.region` — tabel di bawah (**PO setuju apa adanya,
  2026-10-08**). Cabang dengan region kosong (51 di dev) atau tidak ada di tabel tetap NULL → request untuk ATM-nya
  **ditolak** (FR10.2) sampai kode diisi lewat master data (PO 2026-10-08; tidak ada kode cadangan).
- FR11.4 Teks `vendor_branches.region` tidak diubah (Forecast Browser tetap memakainya).

| Teks `region` (dev) | Usulan `region_code` |
|---|---|
| Jakarta | JKT |
| Jakarta Barat / Pusat / Selatan / Timur / Utara (+ `Jakarta UtaraÂ`) | JKTBAR / JKTPST / JKTSEL / JKTTIM / JKTUTR |
| Jawa Barat, Jawa barat | JABAR |
| Jawa Tengah | JATENG |
| Jawa Timur | JATIM |
| Bali | BALI |
| Bali Nusra | BALNUS |
| Indonesia Timur dan Kalimantan | INDTIM |
| Sumatera | SUMATERA |
| Medan | MDN |
| Aceh, Ambon, Jambi, Jember, Kupang, Lampung, Madiun | ACEH, AMBON, JAMBI, JEMBER, KUPANG, LAMPUNG, MADIUN |
| Balikpapan, Banjarmasin, Kendari, Makasar, Manado, Mataram | BPN, BJM, KDI, MKS, MDO, MTR |
| Palangkaraya, Palembang, Pangkal Pinang, Pekanbaru, Pontianak, Samarinda, Tanjung Pinang | PKY, PLG, PGK, PKU, PNK, SMD, TNJ |
| Integration Test Region | (NULL — data tes) |

### FR1 Pembuatan tiket saat create (`VendorRequestService.Create`)
- FR1.1 Dalam tx yang sama dengan insert header + item: untuk setiap `terminal_id` unik di item, insert satu tiket
  dengan kode denom dari himpunan denom ATM itu.
- FR1.2 NNN = `max(seq)` untuk `(terminal_id, replenish_date)` di seluruh tabel (termasuk non-aktif dan milik request
  batal/ditolak) + 1. ATM diproses urut `terminal_id` ascending.
- FR1.3 NNN akan melewati 999 → 422 `"nomor tiket ATM <terminal_id> tanggal <YYYYMMDD> sudah mencapai 999"`, rollback.
- FR1.4 Semua jalur create.
- FR1.5 Dua request bersamaan untuk ATM+tanggal yang sama tidak boleh dapat NNN sama dan tidak boleh gagal karena
  balapan → kunci per `(terminal_id, replenish_date)` dalam tx.

### FR2 Edit item (`UpdateItems`, status `draft` — termasuk draft hasil `Revise`)
- FR2.1 ATM yang masih ada dengan kode denom sama → tiket **tidak berubah** (Q6).
- FR2.2 ATM yang hilang → tiket aktifnya non-aktif (`deactivated_at = now()`).
- FR2.3 ATM yang kode denomnya berubah → tiket lama non-aktif, lalu tiket kode baru (aktif kembali bila ada, FR2.5;
  selain itu tiket baru).
- FR2.4 ATM baru → aktif kembali bila ada, selain itu tiket baru (setelah lolos FR10.4).
- FR2.5 Aktif kembali = tiket non-aktif request yang sama dengan `(terminal_id, kode denom)` sama → nomor sama.
- FR2.6 `replenish_date` tidak bisa diubah setelah create.

### FR3 Transisi lain
- FR3.1 Submit / approve / reject / revise / cancel tidak mengubah tiket maupun nomor request (Q2).

### FR4 Keunikan (DB)
- FR4.1 Tiket: `UNIQUE (ticket_number)`; `UNIQUE (terminal_id, replenish_date, seq)`;
  `UNIQUE (request_number, terminal_id, denom_code)`; partial unique `(request_number, terminal_id) WHERE is_active`.
- FR4.2 Request: `vendor_requests_number_uq` tetap; counter unik per `(vendor_id, region_code, seq_date)`.

### FR5 API (bentuk flat, menambah field — tidak ada endpoint baru)
- FR5.1 Detail request: `items[].ticket_number`, `atms[].ticket_number`, dan `region_code` di header.
- FR5.2 Tiket non-aktif tidak tampil. FR5.3 Pool baca `Get` tidak berubah.
- FR5.4 Vendor branch admin (list/detail/submit) membawa `region_code`.

### FR6 Audit (Sec 5)
- FR6.1 Audit `create`: `after.region_code`, `after.tickets: [{terminal_id, ticket_number}]`.
- FR6.2 Audit `update_items`: `after.tickets_added/deactivated/reactivated` (kosong dihilangkan).
- FR6.3 Perubahan `region_code` vendor cabang diaudit oleh alur master data yang sudah ada.

### FR7 Migrasi + backfill (`026`)
- FR7.1 Satu file `026_replenish_ticket_region.sql` (`022` dicadangkan, `025` terakhir), satu tx.
- FR7.2 Tiket backfill: tiap `(request_number, terminal_id)` di item → satu tiket aktif, kode denom dari himpunan
  denom, NNN per `(terminal_id, tanggal)` urut `created_at, id`. Idempotent; > 999 → gagal.
- FR7.3 Request tanpa `replenish_date` (`VR-…`) memakai `created_at` Asia/Jakarta (diterima PO).
- FR7.4 Salin `vendor_request_atm_results` → `result` tiket aktif; jumlah tidak cocok → `RAISE EXCEPTION`; lalu
  `DROP TABLE vendor_request_atm_results` (disetujui PO).
- FR7.5 Seed `vendor_branches.region_code` sesuai tabel FR11.3 (UPDATE by teks region, hanya baris yang masih NULL).
- FR7.6 `vendor_request_number_seq`: tambah `region_code`, ganti PK `(vendor_id, seq_date)` dengan
  `UNIQUE NULLS NOT DISTINCT (vendor_id, region_code, seq_date)` — baris lama (format lama) tetap dengan
  `region_code` NULL. Postgres dev 18.3 (fitur ada sejak 15).

### FR8 Frontend (CompanyPortal)
- FR8.1 Detail request: kolom **"No. Tiket"** di tabel item; nomor tiket di status per ATM.
- FR8.2 Form + tabel vendor cabang (master data): field "Kode Region".
- FR8.3 Pesan 422 FR10.2 tampil apa adanya di form buat request / Forecast Browser.

### FR9 Hasil laporan selesai di tiket (menggantikan `vendor_request_atm_results`)
- FR9.1 `result` (`success`/`failed`, NULL = belum) + `result_updated_at` di tiket.
- FR9.2 Ajukan laporan selesai menulis `result` di tiket aktif tiap ATM; diajukan ulang menimpa.
- FR9.3 Approve / hitung kunjungan / kuota / `atms[]` membaca dari tiket aktif — perilaku fitur kuota tidak berubah.
- FR9.4 Tiket non-aktif selalu `result` NULL.

## Data model
```sql
-- master data
ALTER TABLE vendor_branches ADD COLUMN region_code text CHECK (region_code ~ '^[A-Z0-9]{2,10}$');

-- request
ALTER TABLE vendor_requests ADD COLUMN region_code text;   -- snapshot, NULL untuk request lama
ALTER TABLE vendor_request_number_seq ADD COLUMN region_code text;
ALTER TABLE vendor_request_number_seq DROP CONSTRAINT vendor_request_number_seq_pkey;
ALTER TABLE vendor_request_number_seq
    ADD CONSTRAINT vendor_request_number_seq_uq UNIQUE NULLS NOT DISTINCT (vendor_id, region_code, seq_date);

-- tiket
CREATE TABLE public.vendor_request_tickets (
    id bigint GENERATED BY DEFAULT AS IDENTITY PRIMARY KEY,
    request_number text NOT NULL REFERENCES public.vendor_requests(request_number) ON DELETE RESTRICT,
    terminal_id text NOT NULL,
    denom_code text NOT NULL CHECK (denom_code ~ '^([1-9][0-9]*K|MIX)$'),
    replenish_date date NOT NULL,
    seq smallint NOT NULL CHECK (seq BETWEEN 1 AND 999),
    ticket_number text NOT NULL,
    is_active boolean NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL DEFAULT now(),
    deactivated_at timestamptz,
    result text CHECK (result IN ('success', 'failed')),
    result_updated_at timestamptz,
    CONSTRAINT vendor_request_tickets_number_uq UNIQUE (ticket_number),
    CONSTRAINT vendor_request_tickets_seq_uq UNIQUE (terminal_id, replenish_date, seq),
    CONSTRAINT vendor_request_tickets_code_uq UNIQUE (request_number, terminal_id, denom_code)
);
CREATE UNIQUE INDEX vendor_request_tickets_active_uq
    ON public.vendor_request_tickets (request_number, terminal_id) WHERE is_active;

DROP TABLE vendor_request_atm_results;   -- setelah data dipindah (FR7.4)
```
- FK ke `request_number` (PO): sah karena `vendor_requests_number_uq`; aman karena nomor request tidak pernah
  ditulis ulang (Req 4.9). Tidak ada `ON UPDATE CASCADE` — nomor memang immutable.
- `region_code` di request = snapshot; teks `vendor_branches.region` tidak disentuh.

## NFR
- Create/edit tetap satu tx; tambahan per ATM: 1 lock + 1 select max + 1 insert; resolusi region memakai lookup
  kelolaan yang sudah dipanggil `validateItemsSingleVendor` (tidak menambah query per ATM). Tidak ada dependency baru.

## Asumsi
- Q5 (diterima): nomor tiket hanya tampil di detail request CompanyPortal; tidak di list/search/export/notifikasi/
  VendorPortal.
- Tabel kode region FR11.3 — dikonfirmasi PO 2026-10-08.

## Tests (diturunkan ke `tests.md`)
- Unit: `denomCode`, `planTickets`, resolusi satu kode region (satu / campur / kosong), format nomor request baru.
- Integration (Postgres, skip bila `DATABASE_URL` kosong): nomor `REP-BJK-JKT-20261008-001`; counter terpisah per
  region; ATM lintas region → 422; cabang tanpa kode → 422; edit tambah ATM region lain → 422; tiket (MIX, `_002`,
  `_003`, aktif kembali, paralel, 999); FK tiket → request; laporan selesai di tiket (regresi kuota); seed migrasi.
- Handler: `ticket_number`, `region_code`. Frontend: kolom "No. Tiket", field "Kode Region" master data.
- Manual browser check: **outstanding** (Golden Rule #10).

## Out of scope
- Mengubah nomor request lama. Membersihkan teks `vendor_branches.region`. Region lokasi ATM (`regions`) di nomor.
- Sistem tiket eksternal. Lihat juga Asumsi Q5.
