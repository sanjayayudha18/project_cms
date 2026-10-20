# Intent: Penetapan branch vault (penyedia uang) per ATM oleh tim ACM (Phase 2.2a)

Author: user (product owner), ditulis bersama Claude.
Status: **accepted revisi 2 — 2026-10-08** (user: "Diterima, lanjut plan"). Versi "rencana CIT harian + transfer donor" **digantikan**.
Stage: 1 Plan. Stage berikut: `spec.md` (ditulis ulang mengikuti intent ini).
Menyentuh Sec 4 #7: **uang (saldo vault)**, **auth/RBAC (role baru ACM-USER/ACM-SPV, scoping area)**, **skema baru**,
**alur approval Vendor Request (status baru)** → wajib AI-DLC penuh.

## Problem — kata user (2026-10-08)
> *"team atm request replenish ATM -> team ACM menerima lalu, memberikan informasi ke team ATM bahwa ATM2 pada region
> tertertu aku diisi oleh branch vendor mana. diutamakan jika ATM dikelola oleh vendor ABA maka system bisa
> merekomendasikan vendor branch ABA dengan region yg sama sebagai prioritas. -> team ACM memberikan rekomendasi ke team
> ATM -> team ATM cek, jika sudah sesuai approved. fase ini berhenti sampai disini. setelah approved dan request replenish
> ke vendor akan dibuat di fase berikutnya"*
>
> *"ATM X akan diisi oleh vendor penyedia uang (sumber uang). vendor branch replenish akan mengambil uang untuk replenish
> ke vendor penyedia uang. kedepannya kita perlu membedakan antara vendor branch vault (penyedia uang) dan vendor branch
> replenish (pelaksana replenish ATM)"*

Dua peran branch vendor yang mulai dibedakan:
- **Branch replenish** (pelaksana) = cabang pengelola kelolaan ATM (`atm_vendor_packages`) — **tidak berubah** di fase ini.
- **Branch vault** (penyedia uang) = branch `CASH` / `ATM_CASH` yang vault-nya menyediakan uang untuk ATM itu;
  ditetapkan ACM per ATM per request. Branch replenish mengambil uang ke branch vault.

Kondisi sekarang (dicek di kode, 2026-10-08):
- Vendor Request: ATM-USER buat → ATM-SPV / BRANCH-ATM-SPV approve (`pending_approval → approved`); tiket per ATM dibuat
  saat create; laporan selesai dari `approved` (`completion_pending → completed`). Satu request = satu vendor + satu `region_code`.
- `vendor_branches.category` ∈ `ATM`/`CASH`/`ATM_CASH`; `vendor_branches.region_code` (mig 026); `vendor_vaults` per branch.
- DSR per vendor; saldo akhir per vault per denom **belum disimpan** ETL; tidak ada kaitan lokasi DSR ↔ `vendor_vaults`.
- Tidak ada role ACM; tidak ada konsep penyedia uang per ATM.

## Proposed outcome (alur fase ini)
1. **ATM-USER** membuat Vendor Request → **ATM-SPV** approve (seperti sekarang) → request masuk status baru
   **menunggu penetapan vault (ACM)**.
2. Request **dipecah per Area ACM** menurut branch replenish tiap ATM (contoh: 6 ATM area Budi, 4 ATM area Andi). Tiap area
   membuat **rekomendasi vault** untuk ATM miliknya.
3. **ACM-USER** area memilih **satu branch vault per ATM** (semua denom). Sistem merekomendasikan berurutan:
   **(1)** branch vault **vendor yang sama** dengan pengelola ATM **dan `region_code` sama** (termasuk branch replenish itu
   sendiri bila `ATM_CASH`, sejajar dengan branch lain di tier ini) → **(2)** branch vault **vendor lain, region sama** →
   **(3)** region lain hanya **urgent + alasan wajib**.
4. Sistem **menampilkan saldo DSR** branch vault (saldo akhir 00:00, DSR tanggal replenish) dan **sisa kapasitas**
   (saldo − kebutuhan ATM-nya sendiri − alokasi lain); bila kurang → **peringatan, tidak memblokir**.
5. **ACM-USER submit → ACM-SPV** area approve / reject (reject → kembali ke ACM-USER).
6. Setelah **semua bagian area** di-approve ACM-SPV → request ke **ATM-SPV / BRANCH-ATM-SPV** untuk cek rekomendasi:
   approve → request **siap** (selesai fase ini); reject + alasan → kembali ke ACM untuk revisi.
7. Laporan selesai replenish baru boleh setelah request **siap**. Request yang sudah `approved` sebelum rilis **melewati**
   langkah ACM (tetap bisa laporan selesai dari `approved`).
8. **Berhenti di sini.** Membuat/mengirim request replenish ke vendor (branch replenish + branch vault) = fase berikutnya (2.2b).

## Affected users and systems
- Users: ATM-USER (tidak berubah), ATM-SPV / BRANCH-ATM-SPV (approve request + approve rekomendasi vault), ACM-USER (menyusun
  rekomendasi), ACM-SPV (approve internal ACM), ADMIN (Area ACM, mapping DSR).
- Systems: `backend/` (status baru Vendor Request, service rekomendasi vault, Area ACM, mapping DSR), `backend_python/dsr/dsr_etl.py`
  (simpan saldo akhir per vault), `frontend/CompanyPortal-Vite` (layar ACM, review ATM-SPV di detail Vendor Request, Area ACM,
  mapping DSR). VendorPortal tidak berubah.
- Tabel baru (nama di spec): Area ACM (area, branch, anggota), mapping lokasi DSR → vault, rekomendasi vault per (request, area),
  penetapan vault per ATM. Diubah: `vendor_requests` (status baru), `dsr_daily_rows` (flow `saldo_akhir`).

## Constraints
- Uang `numeric`, IDR eksplisit; saldo/kapasitas yang ditampilkan disimpan sebagai snapshot pada penetapan (reproducible).
- Maker ≠ checker di tiap lapis (ACM-USER ≠ ACM-SPV; ATM-SPV ≠ ACM); RBAC di route **dan** service; `audit_logs` dalam tx.
- Branch replenish / kelolaan ATM **tidak** diubah. Tidak ada hard delete pada data operasional.
- Tidak ada pindah buku / escrow.

## Resolved decisions (user, 2026-10-08)
Masih berlaku dari versi sebelumnya:
1. Phase 2.2 dipecah 2.2a → 2.2b → 2.2c; fungsi CIT dibangun di `backend/` (`backend-cit` → `backend-exq` nanti, oleh user).
2. Saldo dari **DSR**, angka **saldo akhir 00:00 per vault**, DSR `report_date` = tanggal replenish (C3); ETL menyimpan baris
   SALDO AKHIR per denom (C2); DSR berisi satu blok per vault (C1); blok `SALDO HARIAN ATM` = total, diabaikan (C4).
3. Lokasi DSR dipetakan ke **`vendor_vaults`** (B2b), master data **maker-checker**; satu branch bisa punya beberapa vault (B2).
4. Tipe penyedia uang dari **`vendor_branches.category`** `CASH` / `ATM_CASH` (B2c); saldo "tidak diketahui" bila DSR/mapping tidak ada.
5. Role baru **ACM-USER + ACM-SPV**.
6. **Area ACM** (R2–R5, R8): ADMIN assign branch **satu-satu** + anggota; satu branch satu area; user boleh di beberapa area;
   dikelola ADMIN **langsung + audit** (deviasi GR#3); branch tanpa area → peringatan ke ADMIN. ACM-SPV hanya area miliknya (R7).
7. Pindah buku antar escrow **ditunda**.
8. ADMIN read-only di layar ACM (S2).

Baru (revisi flow, F1–F13):
9. F1 — "ATM diisi oleh branch vault" = **penyedia uang**; branch replenish mengambil uang ke sana.
10. F2 — Langkah ACM **setelah ATM-SPV approve** request.
11. F3 — Rekomendasi ACM di-approve **ATM-SPV / BRANCH-ATM-SPV**.
12. F4 — Rekomendasi **per Vendor Request**.
13. F5 — Saldo vault **ditampilkan + peringatan**, tidak memblokir.
14. F6 — Prioritas: **vendor sama + region sama → vendor lain region sama → urgent lintas region (alasan wajib)**.
15. F7 — Branch replenish `ATM_CASH` boleh jadi vault-nya sendiri, **sejajar** dengan branch tier 1 lain.
16. F8 — ATM-SPV reject + alasan → **kembali ke ACM**.
17. F9 — Lapis approval: **ACM-USER → ACM-SPV → ATM-SPV**.
18. F10 — Area ACM menentukan penerima **berdasarkan branch replenish**; F10b — request lintas area **dipecah per area**.
19. F11 — Status Vendor Request baru sampai rekomendasi approved; laporan selesai setelah **siap**.
20. F12 — Request `approved` sebelum rilis **melewati** langkah ACM.
21. F13 — Branch vault **per ATM** (semua denom dari satu branch).

Digantikan (tidak berlaku lagi): rencana CIT harian per (tanggal, area); transfer donor → penerima per denom; submit diblokir
bila kekurangan belum tertutup; multi-donor per branch; ACM membuka tanggal manual; versi rencana harian.

## Out of scope
- Membuat/mengirim request replenish ke vendor, VendorPortal (2.2b).
- Pindah buku / escrow / SOF / BDS.
- Realisasi vs order, `holidays` (2.2c).
- Mengubah kelolaan / branch replenish ATM.
- CSV import mapping; backfill saldo DSR lama; rename `backend-cit`.
