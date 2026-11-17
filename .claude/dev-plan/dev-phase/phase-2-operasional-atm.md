# Phase 2 — Operasional ATM (inti transaksional harian) ← SAAT INI

> Dipindah dari `.kiro/steering/development-plan.md` (2026-11-12). Ringkasan: [`../dev-plan.md`](../dev-plan.md).

## 2.1 Forecast / Order ATM — ✅ DITUTUP 2026-10-08 (tanpa engine)
- DMAA mengirim `Order_All_*.xlsx`; `backend_python/dmaa/dmaa_etl.py` memuat `dmaa_files` + `dmaa_atm_forecast` (idempoten per SHA-256). `amount_replenish` DMAA diperlakukan sebagai nominal order final sampai 2026-11-12; kini CROWN menyesuaikannya dengan DSR (→ 2.4).
- Dibangun di atasnya: viewer DMAA, Forecast Browser + ringkasan vendor × region, Vendor Request (guard order ganda lewat `is_requested`), tiket replenish, kuota kunjungan.
- Dibuang: `internal/forecast`, `forecast_runs` / `forecast_results`.

## 2.2 Replenishment (fungsi CIT di `backend/`)
Vendor Request `approved` = instruksi replenish. Alur ujung ke ujung: `.claude/feature-flows/10-order-atm-lifecycle/feature-flow.md`.

### 2.2a Penetapan vault oleh ACM — ✅ dibangun (v1 `ef6dbfe`, v3 `94c4189`)
- v1 (`cit-acm-plan/`, mig `027`/`028`): role ACM-USER/ACM-SPV, pemetaan lokasi DSR → vault (maker-checker), ETL menyimpan SALDO AKHIR per vault.
- **v3 (`cit-vault-select/`, mig `030`) menggantikan penetapan per-ATM:** ACM membagi nominal request ke beberapa vault (CASH / ATM_CASH), setiap alokasi mendapat tiket pemenuhan vault, area ACM per **region** (bukan per cabang). Seed dev diperbaiki di `e79e4f0`.
- Sisa: cek browser manual (user), build Docker.
- **Model:** Opus, Effort: High.

### 2.2b Kirim CIT yang disetujui ke vendor — ✅ dibangun `42e7b08` (mig `029`)
- `cit-send-vendor/`: `ready` → `sent_to_vendor`; cabang replenish + cabang vault terima/tolak di Orders VendorPortal; semua terima → `vendor_accepted`. Alur pengambilan di luar scope.
- Sisa: cek browser manual (user, `vendor.aba` pada `REP-ABA-MDN-20261014-001`).

### 2.2c Realisasi vs order — ✅ dibangun `de5f3af` / `9a5af6d` (mig `031`)
- `replenishment-realisasi/`: job Python `replenish_classify` (setelah `itm_replenish`, cron 09:15 WIB, run manual) mengklasifikasikan tepat waktu / lebih awal / terlambat / tidak dilakukan / tanpa order dalam hari kerja (`holidays`); vendor melaporkan tanggal + nominal; ITM menang; laporan CSV. Review R1–R5 diperbaiki.
- Sisa: cek browser manual + test ITM (user), build Docker.
- **Model:** Opus, Effort: High.

### 2.2d Pisah akun vendor per peran + cakupan cabang — ✅ dibangun, task 1–14 di-commit (`950ffe1` … `aad7de4`, mig `032`)
- `.kiro/specs/vendor-party-scope/`: role baru `VENDOR-ATM-USER` / `VENDOR-VAULT-USER` (`VENDOR-USER` tetap memegang kedua pihak), cakupan per cabang pada order/realisasi/notifikasi, peran PIC (langsung + audit, deviasi GR#3), konteks vendor `/auth/me`.
- Checkpoint otomatis 2026-10-21 hijau (Go 2067 pass / 0 skip, CP 1425, VP 250).
- Sisa: cek browser manual (user, akun seed `vendor.abacus.*`).
- **Model:** Opus, Effort: High.

## 2.3 Laporan DSR telat/tidak kirim — ✅ dibangun `67a2d72` (mig `033`, `.claude/sdlc/dsr-late-report/`)
- Laporan bulanan DSR telat/tidak kirim per cabang vendor (batas 09:00 WIB, OK / TELAT / TIDAK KIRIM, dasar penalti FLM) + tampilan daily/summary/warnings, ekspor CSV beraudit, dan notifikasi telat 09:00 (Python `dsr_late_notify`, dijadwalkan di `eod_retry_scheduler`).
- `dsr_etl.py` menulis satu baris `dsr_receipts` append-only per commit; vendor di-resolve lewat kode (review R1). Review R1–R3 diperbaiki 2026-10-27; nit n1–n10 dibiarkan (keputusan user).
- Gate 2026-10-27: Go 2136 / 0 skip, Python 113 / 0 skip, CP 1627, kontrak OpenAPI + `gen:api` bersih.
- Sisa (user): build Docker `backend` + `backend_python`, cek browser manual, upload vendor nyata sekitar 09:00 WIB + run notifikasi 09:00 nyata, ADMIN mengisi `dsr_location_vault_maps` (laporan dan notifikasi tetap kosong sampai itu).
- **Model:** Sonnet, Effort: Medium.

## 2.4 Forecasting kas ATM harian (scope area 3, FSD v1.0 §3) — dipecah 2026-11-12 (user)

### 2.4a Penyesuaian DSR + order replenish final — ⏳ BERIKUTNYA (belum dimulai)
- CROWN menyesuaikan nominal DMAA dengan DSR vendor — langkah 8 `(Forecast Replenish + Plan Replenish) − (Saldo DSR + Forecast Refund)` pukul 09:30–10:00, Plan Replenish = file forecast H-1 — penyesuaian manual 11:45–12:00, order replenish final ditampilkan 12:30 (ringkasan per area vault vendor per denom + detail per ATM) yang mengisi Vendor Request.
- Spec Kiro baru (nama dipilih di Stage 1); uang → rantai penuh (CLAUDE.md Sec 4 #7). Kemungkinan perubahan skema. Refund tetap `0` sampai DMAA mengirimnya.
- **Bergantung pada:** 2.1 ✅, upload DSR ✅, 0.2 ✅.

### 2.4b Pengecualian ATM bermasalah + daftar input FSD — ⏸ DITAHAN (user 2026-11-02: data ATM bermasalah belum diberikan)
- CROWN (bukan DMAA) mengecualikan/menandai ATM "bermasalah" dan menangani daftar input FSD v1.0 (upload complaint/project 10:00–11:30, buang ATM ID ganda + email info, order bernominal kosong diisi ATM Monitoring) sebelum order final.
- Draf spec `.kiro/specs/problem-atm-exclusion/requirements.md` (D1–D19 menunggu PO; di-scope 2026-10-08). Kemungkinan perubahan skema.
- **Bergantung pada:** 2.4a.

## Keputusan Phase 2 (user, 2026-10-08)
1. ~~`amount_replenish` DMAA final — tanpa rumus di dalam CROWN.~~ **Digantikan 2026-11-12 (user):** CROWN menyesuaikannya dengan DSR sesuai FSD §3 langkah 8 (→ 2.4).
2. Refund: DMAA mengirimnya **nanti**; `dmaa_etl.py` menulis `amount_refund = 0` sampai file memiliki kolom refund.
3. Pengecualian ATM bermasalah + daftar input FSD tetap di CROWN → 2.4.
4. Fungsi CIT dibangun di `backend/`, bukan di backend CIT terpisah.
