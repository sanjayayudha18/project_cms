# Intent: pengecualian ATM bermasalah + list input FSD (review order DMAA)

Author: user (product owner), ditulis bersama Claude. Status: **draft 2026-11-02**, menunggu product owner.
Stage: 1 Plan → stage berikut `spec.md` setelah intent diterima.
Roadmap: Phase 2.4 (`.kiro/steering/development-plan.md`). Sumber: URS v0.3 Phase 1 bagian "ATM Cash Forecasting Harian" butir 1–2, FSD v1.0 (`.claude/docs/fsd-v1-alignment-2026-10-01.md` baris 78), `.claude/docs/requirements.md`, `.claude/feature-flows/03-atm-cash-forecasting/`, keputusan user 2026-10-08.

## Problem
Order harian sekarang murni dari file DMAA: `dmaa_etl.py` memuat `dmaa_atm_forecast`, lalu
pengguna memilih barisnya di Forecast Browser untuk dijadikan Vendor Request. CROWN belum
punya cara meninjau rekomendasi itu. Akibatnya:

- ATM yang tidak bisa bertransaksi (URS: pending part, vandalism, dll.) tetap bisa dipesan.
- List dari tim complaint handling/rekonsiliasi dan project ATM dari unit bisnis tidak punya
  jalur masuk ke order.
- Nominal DMAA tidak bisa diganti untuk area yang perlu adjustment.
- Aturan URS "tidak ada order tambahan bila ATM sudah punya order emergency/adhoc sampai H-1
  atau aktif di H0" belum tentu terpenuhi. Yang ada sekarang hanya penanda `is_requested`
  per (terminal, periode, denom), yang menandai baris DMAA yang sudah jadi item request.

Keputusan 2026-10-08 menetapkan pekerjaan ini tugas CROWN, bukan DMAA.

## Proposed outcome
Empat list input URS, sebagai bagian dari review rekomendasi H0:

| List | Asal (URS) | Efek (URS) |
|---|---|---|
| Complaint handling / rekonsiliasi | Tim complaint handling / rekonsiliasi | Sudah ada rekomendasi DMAA emergency/planned kemarin → tidak dibuat order; belum ada → order emergency |
| Project ATM (replace, new, relokasi, dll.) | Unit bisnis internal | Ditambahkan sebagai order emergency/planned |
| ATM bermasalah | Belum disebut URS | Dikeluarkan dari rekomendasi |
| Adjustment order | Belum disebut URS | Mengganti nominal DMAA untuk ID yang terdaftar |

Hasil gabungannya tampil sebagai **draft order ATM** (rekomendasi DMAA + order manual +
adjustment), lalu melalui approval berjenjang sebelum terbit sebagai instruksi. Pengguna bisa
melihat kenapa sebuah ATM dikeluarkan, ditambahkan, atau nominalnya diganti.

## Affected users and systems
- Users: ATM Forecasting Unit (FSD; pemetaan ke role CROWN belum ada), ATM-USER/ATM-SPV, ADMIN.
- Systems: `backend/` (Vendor Request, `queries/vendor_request.sql`), `frontend/CompanyPortal-Vite`
  (`features/vendor-request`, Forecast Browser), `internal/notification`, `backend/api/openapi.yaml`.
  Bisa juga `backend_python/` bila list diproses lewat pipeline file.
- Data: tabel baru kemungkinan besar perlu (migrasi berikutnya `036`). Belum ada query untuk status
  "problem" ATM di `backend/queries`.

## Constraints
- DMAA `amount_replenish` tetap final dan **tidak dihitung ulang**. Adjustment mengganti nominal
  lewat list eksplisit, dan nilai asal DMAA tidak boleh hilang.
- Maker-checker + `audit_logs` untuk setiap perubahan yang memengaruhi order (GR#3).
- Upload file idempoten per hash (`import_jobs`, Sec 5). Helper Go `internal/importjob` belum
  ada, jadi ini bisa menjadi konsumen Go pertamanya.
- Uang numeric/minor unit, IDR eksplisit; tampilan Asia/Jakarta.
- Fitur ini menyentuh **uang dan migrasi** (Sec 4 #7), jadi wajib melewati seluruh tahap AI-DLC.
- Order yang sudah berjalan tidak boleh rusak; flow Vendor Request saat ini menjadi dasar.

## Out of scope (usulan, perlu konfirmasi PO)
- Forecast engine atau rumus order apa pun di CROWN.
- Refund DMAA (menunggu DMAA mengirim datanya).
- Pemenuhan dana / cash pickup (FSD, fase berikutnya).

## Open questions (belum dijawab, untuk PO)

**Cakupan**
1. Keempat list dibangun sekaligus, atau ATM bermasalah dulu lalu sisanya menyusul?

**ATM bermasalah**
2. Bentuknya list upload harian, atau status ATM yang berlaku sampai dicabut? Kalau status, siapa
   yang mencabut dan apakah ada tanggal berakhir?
3. Siapa yang menetapkan, dan apakah perlu maker-checker sebelum berlaku?
4. ATM bermasalah disembunyikan dari Forecast Browser, atau tetap tampil dengan penanda dan tidak
   bisa dipilih?
5. Bagaimana kalau ATM ditandai bermasalah saat sudah ada di Vendor Request yang sedang berjalan?
6. Apakah ATM bermasalah juga diblok saat input manual, atau hanya pada rekomendasi DMAA?

**Complaint dan project**
7. FSD menyebut order dari list ini dibuat dengan nominal **kosong**. Siapa yang mengisi nominalnya,
   dan kapan?
8. Project ATM bisa berisi ATM baru: apakah wajib sudah ada di master `atms`? Apa yang terjadi bila
   belum ada?
9. Apa beda "emergency" dan "planned" secara proses, dan dari data mana CROWN menentukannya?

**Adjustment**
10. URS menyebut "area tertentu" tapi juga "list". Unit adjustment per ATM, per area, atau per denom?
11. Apakah ada batas nominal, atau keterangan alasan wajib?

**Aturan order ganda**
12. Apa definisi "aktif di H0" untuk order emergency/adhoc, dalam istilah status Vendor Request?
13. Apakah pengecualian ini otomatis, atau tampil sebagai peringatan yang bisa di-override?

**Draft order dan approval**
14. Draft order ini menggantikan cara pilih baris di Forecast Browser, atau menjadi langkah sebelum
    Forecast Browser?
15. "Approval berjenjang" berapa level, dan siapa? Apakah sama dengan approval Vendor Request yang
    sudah ada, supaya tidak ada dua kali approval?
16. Jendela waktu FSD (upload 10:00–11:30, adjustment 11:45–12:00, merge 11:35–12:15, final order
    12:30) ditegakkan sistem, atau hanya panduan?

**Format dan peran**
17. Format file tiap list (CSV/XLSX, kolom), dan apakah ada contoh file nyata?
18. "ATM Forecasting Unit" dipetakan ke role yang ada, atau perlu role baru?
19. Siapa yang menerima notifikasi saat draft siap atau ada list yang gagal diproses?
