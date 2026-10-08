# Intent: Nomor tiket replenish per ATM

Author: user (product owner), ditulis bersama Claude. Status: **accepted 2026-10-08 (PO, via chat: "setuju lanjut")**.
Stage: 1 Plan. Stage berikut: `spec.md` (setelah intent diterima).
Menyentuh Sec 4 #7: **skema (kolom/tabel baru + migrasi)** → wajib AI-DLC penuh.

## Problem
Kata PO: "pada saat replenish ATM di request oleh user tambahkan juga no tiket replenish per ATM, jadi tiap ATM yang di-replenish no tiket yang unik dan tidak boleh sama."

Kondisi sekarang (dicek di kode 2026-10-07):
- Request replenish = `vendor_requests`, punya `request_number` unik (`vendor_requests_number_uq`, format `REP-<prefix>-<YYYYMMDD>-<seq>`, dari `vendor_request_number_seq`).
- Baris request = `vendor_request_items`, unik per `(vendor_request_id, terminal_id, periode_pred, denom)` → satu ATM bisa punya beberapa baris dalam satu request.
- Tidak ada nomor per ATM. Hasil per ATM baru ada saat laporan selesai (`vendor_request_atm_results`, PK `(vendor_request_id, terminal_id)`, migrasi 021).
- `UpdateItems` (`service/vendor_request_actions.go`) menghapus semua item lalu insert ulang → nomor yang disimpan di `vendor_request_items` akan hilang/berubah setiap edit.

## Proposed outcome
1. Setiap ATM dalam satu request replenish punya **satu nomor tiket** (semua denom/tanggal ATM itu dalam request yang sama berbagi tiket).
2. Nomor tiket **unik global** dan dijamin oleh constraint DB (`UNIQUE`), bukan hanya oleh kode.
3. Tiket dibuat **saat request dibuat** (sejak draft), dan tidak pernah dipakai ulang — termasuk bila request ditolak/dibatalkan.
4. Format: `<request_number>-<NNN>` (contoh `REP-ABC-20261007-001-001`), NNN urut per request.
5. Nomor tiket tampil di detail request (CompanyPortal) dan di sisi vendor (VendorPortal) serta di laporan selesai per ATM.

## Affected users and systems
- Users: Operator/maker request replenish, Manager (approver), Vendor (pelaksana), ATM-SPV (laporan selesai/kuota).
- Systems: `backend/` (migrasi baru, `queries/vendor_request.sql`, `service/vendor_request_*`, handler + response), CompanyPortal + VendorPortal (tampilan), mungkin export.

## Constraints
- Uniqueness ditegakkan DB (`UNIQUE` constraint), dibuat dalam tx yang sama dengan create request.
- Tidak ada dependency baru. Penulisan di primary.
- Audit: create/update request sudah diaudit; nomor tiket ikut tercatat di snapshot audit.
- Data lama: request yang sudah ada tidak punya tiket → perlu keputusan backfill (open question 3).

## Open questions (dijawab PO sebelum spec)
1. **Edit item (`UpdateItems`) saat draft/revisi**: ATM yang dihapus dari request → tiketnya dibuang (nomor bolong) atau disimpan non-aktif? ATM baru yang ditambah → dapat NNN berikutnya (tidak mengisi nomor bolong)?
2. **Request dibatalkan/ditolak lalu dibuat ulang**: request baru = `request_number` baru → tiket baru. Benar?
3. **Backfill**: request lama (sebelum migrasi) diberi tiket otomatis oleh migrasi, atau dibiarkan kosong?
4. **Batas NNN**: 3 digit (maks 999 ATM per request) cukup?
5. **Dipakai di mana lagi**: nomor tiket perlu ikut di export/CSV, email/notifikasi ke vendor, atau dicari (search) di daftar request?
6. **Revise** (`Revise` setelah reject): request yang sama dipakai lagi — tiket tetap sama?

## Resolved decisions (PO, 2026-10-07/08, via chat)
- AI-DLC penuh di `.claude/sdlc/replenish-ticket/`.
- Granularitas: per (request, ATM).
- Waktu dibuat: saat request dibuat.
- Format: `<request_number>-<NNN>`.
- Q1: ATM dihapus saat edit → tiket disimpan non-aktif, nomor tidak pernah dipakai ulang; ATM baru → NNN berikutnya.
- Q2: request baru (setelah batal/tolak) → tiket baru.
- Q3: request lama di-backfill oleh migrasi.
- Q4: 3 digit (maks 999 ATM per request) — diterima sebagai bagian "setuju".
- Q5: tidak ada usulan saat itu → spec mengambil default minimal (lihat spec "Asumsi"), PO konfirmasi saat accept spec.
- Q6: revisi setelah reject → tiket ATM yang tetap ada tidak berubah.

## Revisi format (PO, 2026-10-08, via chat — menggantikan "Format" dan "Granularitas" di atas)
- Format baru: `<terminal_id>_<kode denom>_<tanggal replenish YYYYMMDD>_<NNN>`, contoh `Y18X_100K_20261008_001`.
- Kode denom huruf besar: `50K`, `100K`.
- Tanggal = tanggal replenish (sama dengan segmen tanggal di `request_number`), bukan tanggal request dibuat.
- NNN = urutan per (ATM, denom, tanggal replenish), global lintas request: request kedua untuk ATM+denom+tanggal yang
  sama → `_002`.
- Granularitas: satu tiket per (request, ATM, denom) — ATM yang diisi 50K dan 100K dalam satu request punya 2 tiket.
- Q1/Q2/Q6 tetap berlaku (non-aktif saat dihapus, nomor tidak dipakai ulang, revise tidak mengubah tiket).

## Revisi granularitas (PO, 2026-10-08, via chat — menggantikan dua butir terakhir di atas)
- Kekhawatiran PO: "jika vendor mendapat 2 tiket akan dihitung jalan 2 kali, padahal itu merupakan 1 terminal ID yg sama".
- Keputusan: **satu tiket per (request, ATM)** = satu jalan. Kode denom: satu denom → `50K`/`100K`; lebih dari satu
  denom → `MIX` (contoh `Y18X_MIX_20261008_001`).
- NNN = urutan per (ATM, tanggal replenish), global lintas request, tidak bergantung denom → NNN = kunjungan ke-berapa
  ke ATM itu pada tanggal itu.
- Edit draft yang mengubah kode denom suatu ATM → tiket lama non-aktif, terbit tiket baru (NNN berikutnya).

## Revisi nomor request + relasi tiket (PO, 2026-10-08, via chat)
- Kata PO: "kita perlu update nomor requestnya juga — before: REP-BJK-20261008-001, after:
  REP-BJK-<REGION>-20261008-001; pastikan no request ini juga menjadi FK di table vendor_request_tickets".
- `<REGION>` = **kode region vendor cabang** — kolom baru `vendor_branches.region_code` (dikelola lewat master data
  maker-checker, seed awal dari teks `vendor_branches.region`). Bukan region lokasi ATM: dicek di dev, satu request
  Bijak Jakarta mencakup ATM di JKT_EAST + TANGERANG (sampai 6 region lokasi), sedangkan vendor cabangnya selalu satu.
- 1 request = 1 kode region; ATM dari region lain → ditolak.
- Counter 3 digit request = per (vendor, kode region, tanggal replenish).
- `vendor_request_tickets` berelasi lewat `request_number` (FK), tanpa `vendor_request_id`.
- Nomor request lama (`REP-BJK-20261008-001`, `VR-…`) tidak diubah (aturan Req 4.9 yang sudah ada).
- Menggantikan "Out of scope: Mengubah format `request_number`" di atas.

## Out of scope
- Integrasi nomor tiket ke sistem tiket eksternal.
- Mengubah format `request_number`.
