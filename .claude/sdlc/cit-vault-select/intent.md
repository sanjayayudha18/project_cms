# Intent: Pemenuhan vault ACM — bagi nominal request ke vault + tiket pemenuhan vault (perubahan Phase 2.2a)

Author: user (product owner), ditulis bersama Claude.
Status: **v3 accepted — 2026-10-13** ("accept intent, lanjut spec"). (v1 accepted 2026-10-13: pilih branch → vault per ATM; v2: + tiket.
v3 mengganti model **per ATM** menjadi **pembagian nominal per request** — lihat "Riwayat".)
Stage: 1 Plan. Stage berikut: `spec.md` (v3).
Menyentuh Sec 4 #7: **skema baru** (alokasi vault, tiket, area per region), **perhitungan uang** (total alokasi vs order,
kapasitas vault), **alur approval** (satu rencana per request) → wajib AI-DLC penuh. Mengganti keputusan 2.2a **F10/F10b**
(area per cabang, request dipecah per area) dan **F13** (vault per ATM), serta isi pihak vendor 2.2b FR3.

## Problem — kata user (2026-10-13)
> *"menu ini seharusnya pilih branch vendor dulu. setelah pilih branch vendor, user akan pilih branch vault. contoh:
> branch vendor "ABACUS_001 · Abacus Bali" punya 2 vault yg bisa dipilih"*

> *"request rencana pemenuhan vault oleh ACM juga perlu tiketing, saat team ACM selesai assign vendor pemenuhan uang
> buatkan tiketnya per cabang."*

> *"pembuatan tiket "Tiket pemenuhan vault" user tidak perlu assign ATM satu per satu, cukup berdasarkan tiket "Tiket
> replenish" saja"* — tiket replenish yang dimaksud = **nomor Vendor Request per vendor + region**
> (`REP-<vendor>-<region>-<YYYYMMDD>-<NNN>`, mig 026), bukan tiket per ATM.

> *"ACM memilih satu atau lebih vault untuk request, lalu membagi nominal, bukan per ATM"*

> *"pemegang request adalah role ATM_user (role ini sudah terpusat …). yang tidak di urus oleh role ATM_user adalah ATM2
> yg on prem … di urus oleh role yg berbeda"* · ACM *"terpusat hanya untuk JABODETABEK, sisanya berbeda tiap region"*.

Kondisi sekarang (dicek di kode, 2026-10-13):
- ATM-SPV approve → request dipecah **per Area ACM** menurut cabang pengisi tiap ATM (`acm_area_branches`); tiap area
  memilih **satu branch vault per ATM** (`vendor_request_vault_assignments`, mig 027). Tidak ada pilihan vault.
- Saldo/kapasitas per **branch** (Σ vault aktif branch via `dsr_location_vault_maps`).
- Pihak vendor 2.2b: pihak vault per branch; cabang pengisi melihat branch vault per ATM.
- Tidak ada tiket untuk sisi vault; tiket yang ada = nomor request (vendor + region) + tiket replenish per ATM (026).
- S1 v1 sudah dibuat (uncommitted): migrasi 030 `vendor_request_vault_assignments.vendor_vault_id` (per ATM) — di-apply ke
  dev, 0 baris.

## Proposed outcome
1. Setelah ATM-SPV approve, request **tidak dipecah**: satu **rencana pemenuhan vault per request**, dipegang **satu Area
   ACM** yang ditentukan dari **region request** (ADMIN memetakan `region_code` → Area ACM; mis. "ACM JABODETABEK" memegang
   semua region JABODETABEK, area lain per region).
2. **ACM-USER** area itu memilih **satu atau lebih vault** (CASH/ATM_CASH, dengan branch vendornya) dan **membagi nominal
   per denom** ke vault-vault itu. Tidak ada pemetaan ATM → vault.
3. Total pembagian per denom **minimal = total order** request; **boleh lebih** (buffer) dengan **alasan wajib**; kurang →
   tidak bisa submit.
4. Tier per vault terhadap vendor + region request (tier 3 = region lain → urgent + alasan), seperti 2.2a.
5. Saldo + sisa kapasitas **per vault** = saldo DSR vault − alokasi yang sudah tersimpan; kurang → **peringatan saja**.
6. Saat **ACM-USER submit**, terbit **tiket pemenuhan vault per vault** `<kode vault>_<YYYYMMDD>_<NNN>`.
7. ACM-SPV approve → ATM-SPV review → kirim ke vendor (alur 2.2a/2.2b tetap, tapi satu rencana per request).
8. Vendor: **pihak vault** melihat tiket + nominal vault-nya; **cabang pengisi** melihat **daftar semua vault** request
   (tiket, nominal, alamat).

## Affected users and systems
- Users: ACM-USER / ACM-SPV (bagi nominal, approve), ATM-SPV / BRANCH-ATM-SPV (review), ADMIN (area ↔ region),
  vendor pihak vault + cabang pengisi (VendorPortal). ATM-USER (pemegang request, terpusat) tidak berubah.
- Systems: `backend/` (rencana per request, alokasi, tiket, area per region, kapasitas, pihak vendor),
  `frontend/CompanyPortal-Vite` (Penetapan Vault, Area ACM, detail Vendor Request), `frontend/VendorPortal-Vite` (Orders).

## Constraints
- Vault terpilih: aktif, kategori `CASH`/`ATM_CASH`, branch aktif ber-region; dicek saat simpan **dan** submit.
- Maker ≠ checker (ACM-USER submit ≠ ACM-SPV approve; ATM-SPV ≠ maker/approver ACM) tetap.
- Uang `bigint` IDR / `numeric`, tidak pernah float; saldo tidak diketahui = NULL.
- Audit dalam tx; tidak ada hard delete (selain replace-all alokasi saat `draft`, seperti sekarang).
- Respon flat JSON seperti handler ATM `backend`.

## Resolved decisions (user, 2026-10-13)
1. Q1 — AI-DLC baru `cit-vault-select/`.
2. Q2 — Vault yang boleh dipilih: **`CASH` / `ATM_CASH`** saja.
3. Q3 — Saldo + kapasitas **per vault**.
4. T1 — Banyak ATM/request boleh memakai vault yang sama selama uangnya cukup; kapasitas kurang = **peringatan saja**.
5. V1 — **Pembagian nominal per request** ke satu atau lebih vault; **tidak** per ATM (menggantikan F13 + v1/v2).
6. V2 — **Satu area per request** (request tidak dipecah per area).
7. V3 — Area ditentukan **region request**: mapping `region_code` → Area ACM oleh ADMIN (satu region tepat satu area; satu
   area boleh banyak region). Region tanpa area → request menunggu + peringatan ADMIN (seperti 2.2a FR7.4).
   Mapping cabang → area (`acm_area_branches`) tidak dipakai lagi untuk rencana baru.
8. V4 — Total per denom **≥ order**; lebih = buffer, **alasan wajib**; kurang → submit ditolak.
9. V5 — **Tier tetap, per vault** (tier 3 urgent + alasan).
10. V6 — Cabang pengisi melihat **daftar semua vault** request (tiket, nominal, alamat); koordinasi pengambilan di luar CROWN.
11. V7 — Tiket **per vault** (`<kode vault>_<YYYYMMDD>_<NNN>`, NNN global per (vault, tanggal replenish), tidak dipakai ulang).
12. V8 — Kapasitas vault = saldo − **alokasi tersimpan** saja; order request yang belum dibagi **tidak** dibebankan.
13. V9 — Tiket terbit saat **ACM-USER submit**; submit ulang: vault sama → **nomor tetap** (walau nominal berubah), vault
    dihapus → nonaktif, vault baru → nomor baru. Reject tidak mengubah tiket. Cancel request tidak mengubah tiket.
14. V10 — Tiket tampil di CompanyPortal + VendorPortal.
15. V11 — ATM **on-prem** (role lain) **di luar scope**.

16. V12 — Alokasi per denom **wajib kelipatan pecahannya** (100K kelipatan Rp100.000, dst.).
17. V13 — Alasan buffer **satu per rencana**, wajib 10–500 karakter bila ada denom dengan total alokasi > order.

## Catatan untuk spec
- Data lama per ATM (`vendor_request_vault_assignments`): di dev 0 baris → tidak dimigrasi; tampilan lama read-only bila ada.
- Migrasi 030 v1 (kolom `vendor_vault_id` per ATM) belum di-commit dan 0 baris di dev → diganti, bukan ditambal (lihat plan).

## Riwayat
- v1 (accepted): per ATM pilih branch → vault; U5–U8 (beban branch ke semua vault, sembunyikan branch tanpa vault, default
  vault kapasitas terbesar, kode + alamat vault ke vendor). U5 **gugur** oleh V8; U6–U8 dibawa ke v3 di spec.
- v2: tiket per branch vault, lalu per area (T2–T8). **Diganti** V7/V9 (per vault, satu area per request).

## Out of scope
- ATM on-prem dan role-nya (V11).
- `dsr_location_vault_maps`, ETL DSR; pindah buku / escrow per vault; alur cash pickup FSD (petugas, nopol, serah terima).
- Pemetaan ATM → vault / cabang pengisi → vault (V1, V6).
