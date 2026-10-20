# Intent: laporan DSR telat / tidak kirim (dasar penalti FLM)

Author: TODO (product owner). Status: draft — menunggu penerimaan product owner.
Stage: 1 Plan. Trigger ke stage berikut: product owner menerima file ini.
Roadmap: Phase 2.3 (`.kiro/steering/development-plan.md`). Sumber: URS v0.3 Phase 1 (FNC 003 Dashboard, aturan DSR 09:00), CLAUDE.md Sec 3a.

## Problem
Vendor FLM wajib mengirim DSR harian paling lambat **09:00**. Upload DSR sudah
berjalan (`dsr_uploads`, `/api/v1/dsr`), tapi belum ada cara melihat vendor mana
yang telat atau tidak mengirim sama sekali. Rekap bulanan keterlambatan — dasar
penalti FLM — masih harus disusun manual, dan tidak ada peringatan saat DSR telat.

## Proposed outcome
- Operator/Manager membuka laporan bulanan DSR per vendor + area vault, kolom:
  **tanggal laporan** (bukan tanggal kirim), vendor + area vault, waktu diterima,
  status **OK / TELAT** (+ tidak kirim). Bisa difilter dan diekspor.
- Saat DSR telat/tidak masuk, pihak terkait mendapat notifikasi (in-app / email).
- Angka rekap bisa dipakai langsung sebagai dasar penalti FLM tanpa hitung ulang.

## Affected users and systems
- Users: Operator, Manager (pembaca laporan), Vendor (penerima notifikasi telat), Admin.
- Systems: `backend/` (`internal/dsr` atau modul laporan), `frontend/CompanyPortal-Vite`,
  tabel `dsr_uploads` (sudah ada: `report_date`, `vendor`, `created_at`, `processed_at`),
  `internal/notification` (**belum dibangun** — Phase 0.3), `internal/export`
  (CSV ada; XLSX/PDF butuh Phase 0.5).

## Constraints
- Laporan dibaca dari **read replica** (Sec 6), tidak menulis ulang data DSR.
- Status dihitung dari data yang tersimpan, reproducible (bisa dijelaskan kenapa TELAT).
- Waktu disimpan UTC, deadline & tampilan dalam Asia/Jakarta.
- Status tidak boleh hanya warna — wajib label/ikon (Sec 13).
- Bergantung pada 0.3 notification; tanpa itu hanya bagian laporan yang bisa jalan.

## Resolved decisions (tanya-jawab dengan user, 2026-09-28)
1. **Deadline = hari yang sama, 09:00 WIB.** DSR dengan `report_date` X wajib masuk tanggal X pukul 09:00 Asia/Jakarta (report_date = saldo 00:00 hari itu). Berlaku setiap hari, tanpa pengecualian libur.
2. **Waktu diterima = upload pertama yang berhasil.** Revisi setelah itu tidak mengubah status OK/TELAT.
3. **Wajib kirim = per cabang vendor** (`vendor_branches` aktif) per hari. Cabang yang tidak punya DSR sukses untuk tanggal X = TIDAK KIRIM.
4. **Upload gagal proses (`daily_status='failed'`) = tidak kirim.** Hanya upload yang berhasil diproses dihitung; bila vendor upload ulang dan berhasil, waktu upload itu yang dipakai.
5. **Notifikasi ke PIC vendor + Operator internal, lewat in-app + email (SMTP), dicek sekali pukul 09:00 WIB** untuk cabang yang belum punya DSR sukses.
6. **Ekspor CSV dulu** (`encoding/csv`, tanpa dependency baru). XLSX/PDF menyusul Phase 0.5.
7. **Laporan hidup, tanpa kunci/approval.** Selalu dihitung ulang dari data; keputusan penalti di luar sistem.

## Implikasi yang harus diselesaikan di `spec.md` (bukan keputusan baru)
- **Keputusan #3 vs skema saat ini:** `dsr_uploads` unik per `(report_date, vendor)` dan tidak menyimpan cabang. Untuk "per cabang vendor", upload harus bisa dikaitkan ke `vendor_branches` — perlu perubahan skema/alur upload DSR (migrasi → Sec 4 #7, STOP & flag). Perlu dicek juga bagaimana workbook DSR multi-lokasi memetakan ke cabang.
- **Keputusan #2 vs skema saat ini:** revisi DSR me-*replace* baris `dsr_uploads`, sehingga waktu upload pertama hilang. Perlu kolom/penyimpanan waktu upload sukses pertama (misal `first_received_at`) yang tidak ikut ter-replace.
- **Keputusan #5** bergantung pada Phase 0.3 `internal/notification` (belum dibangun) dan scheduler jam 09:00 (cron di Go atau di `eod_retry_scheduler` Python — pilih di spec).

## Out of scope
- Perhitungan nominal penalti FLM (hanya menyediakan data dasarnya).
- Perubahan alur upload DSR itu sendiri.
