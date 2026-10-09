# Intent: Replenishment — realisasi pengisian ATM vs order (Phase 2.2c)

> **Re-sequenced 2026-10-08 (user):** Phase 2.2 dipecah. 2.2a = rencana CIT tim ACM (`../cit-acm-plan/`), 2.2b = kirim CIT approved ke vendor, **2.2c = intent ini**. Keputusan Q1–Q14 di bawah tetap berlaku; dikerjakan setelah 2.2a/2.2b.

Author: user (product owner), ditulis bersama Claude. Status: **draft 2026-10-08** — open questions dijawab user 2026-10-08 (tanya-jawab satu per satu); menunggu PO accept.
Stage: 1 Plan. Stage berikut: `spec.md`.
Menyentuh Sec 4 #7: **uang (nominal realisasi vs order)** + **skema baru** (tabel hasil klasifikasi, `holidays`, laporan realisasi vendor) + **alur approval** (laporan vendor) → wajib AI-DLC penuh.

## Problem
Kata user (2026-10-08): *"rapikan dokumen lain juga, lalu mulai intent 2.2"* — 2.2 = `internal/replenishment` di
`.kiro/steering/development-plan.md`, setelah Phase 2.1 ditutup (*"untuk saat angka DMAA sudah final"*).

Kebutuhan bisnis dari URS v0.3 / FSD v1.0 (`.claude/docs/requirements.md`, `feature-flows/06-replenishment-validation-reports`):
- *"Replenishment result is classified (holiday-adjusted): on-schedule · early (1–2 days) · late (1–2 days) · not done
  (>2 days off or skipped) · replenished without an order (realisasi vs order)."*
- Report wajib: amount pengisian ≠ order (+ detail ID), trip / realisasi per vendor per ATM ID, order vs transaksi, refund per ID.
- Dashboard FNC 003: nominal instruksi harian + terminal ID.

Kondisi sekarang (dicek di kode, 2026-10-08):
- **Order sudah ada** sebagai Vendor Request (`vendor_requests` + `vendor_request_items`): dibuat dari baris DMAA
  (`dmaa_atm_forecast`, nominal final) atau Manual_Request, maker-checker `draft → pending_approval → approved`,
  cegah order ganda (`is_requested`), tiket per ATM (`vendor_request_tickets`, mig 026), laporan selesai berhasil/gagal
  per ATM (`approved → completion_pending → completed`, mig 021) yang memotong kuota kunjungan.
- Status `processing` / `failed` ada di CHECK constraint `vendor_requests_status_chk` tapi tidak dipakai endpoint mana pun.
- Tabel `replenishment_instructions` disetujui di CLAUDE.md Sec 3 tapi **belum dimigrasi**.
- **Data realisasi pengisian dari mesin sudah masuk DB**: `itm_replenish` (ETL `backend_python/itm/replenish/`,
  per `terminal_id`, `replenish_date` + `replenish_time`, nominal `replenish_denom_10k/20k/50k/100k`, `refund_*`) —
  saat ini hanya dibaca ATM portal.
- Laporan selesai di Vendor Request hanya **berhasil/gagal**, tanpa nominal dan tanggal realisasi.
- Tidak ada kalender hari libur (`holidays` masih proposal FSD, belum disetujui).
- Frontend `features/replenishment/ReplenishmentScreen.tsx` dan dashboard `ReplenishmentSummary` masih **mock**
  (status `completed/in-transit/scheduled/delayed/pending-vendor`, data hardcoded).

## Proposed outcome
1. **Vendor Request yang `approved` = instruksi pengisian.** Tidak ada tabel `replenishment_instructions` dan tidak ada
   state machine kedua; 2.2 menambah **tabel hasil klasifikasi** di atas Vendor Request / tiket.
2. **Klasifikasi harian otomatis** per tiket (request, ATM): tanggal realisasi dari **`itm_replenish`** dibandingkan dengan
   **`replenish_date` tiket**, dalam **hari kerja** (lewati Sabtu/Minggu + tabel `holidays`):
   sesuai jadwal (0) · maju 1–2 hari · mundur 1–2 hari · tidak dilakukan (> 2 hari / tidak ada pengisian, final setelah
   lewat 2 hari kerja) · **pengisian tanpa order** (baris `itm_replenish` tanpa tiket yang cocok).
   Job berjalan setelah ETL ITM replenish (SLA 07:00) dan menghitung ulang yang masih terbuka.
3. **Nominal**: realisasi ITM per denom vs order per denom; **selisih persis > 0** (tanpa toleransi) → report "amount ≠ order".
4. **Vendor melapor via VendorPortal** (ganti mock): tanggal + nominal per denom per ATM. Laporan ini **menggantikan input
   ATM-USER** pada alur laporan selesai yang ada: vendor submit → **ATM-SPV / BRANCH-ATM-SPV** approve/tolak →
   kuota kunjungan terpotong seperti sekarang (alur `approved → completion_pending → completed` tetap, maker pindah ke vendor).
5. **ITM menang**: bila laporan vendor beda dengan ITM (tanggal/nominal), klasifikasi resmi tetap dari ITM; baris ditandai
   "beda dengan laporan vendor" dan muncul di report.
6. **Tabel `holidays`**: dikelola **ADMIN saja**, **langsung berlaku + `audit_logs` dalam tx yang sama** (deviasi GR#3
   terdokumentasi, pola Role Management — dicatat di CLAUDE.md Sec 12 saat diterapkan).
7. **Tampilan**: layar Replenishment internal (ganti mock) + dashboard `ReplenishmentSummary`; **vendor melihat
   klasifikasi + selisih miliknya sendiri** di VendorPortal (scoped ke assignment, Sec 5). Semua dibaca dari replica.
8. **Report 2.2**: "amount ≠ order" (+ detail ATM ID) dan "trip / realisasi per vendor per ATM ID (incl. tertinggi)",
   export **CSV** (stdlib).

## Affected users and systems
- Users: Vendor (lapor realisasi + lihat klasifikasi miliknya), ATM-SPV / BRANCH-ATM-SPV (approve laporan vendor),
  ATM-USER (monitoring), ADMIN (hari libur), manajemen (dashboard).
- Systems: `backend/` (migrasi, queries, service, handler, job klasifikasi — runtime Go vs Python EOD ditetapkan di spec),
  `frontend/CompanyPortal-Vite` (`features/replenishment`, `features/dashboard`, admin hari libur, approve laporan vendor),
  `frontend/VendorPortal-Vite` (form laporan + tampilan klasifikasi), `backend_python/itm/replenish` (sumber, read-only).
- Tabel: dibaca `vendor_requests`, `vendor_request_items`, `vendor_request_tickets`, `itm_replenish`, `atms`,
  `atm_vendor_packages`; baru: hasil klasifikasi, `holidays`, laporan realisasi vendor (nama + kolom ditetapkan di spec,
  dicatat di CLAUDE.md Sec 3 setelah diterapkan); diubah: alur laporan selesai `vendor_requests` (maker = vendor).

## Constraints
- Uang `numeric`, IDR eksplisit, tidak pernah float (Sec 6). Hasil klasifikasi reproducible: simpan input (order,
  realisasi ITM, laporan vendor, tanggal) + selisih (Sec 6).
- Report/dashboard/VendorPortal dari **replica**; tulis ke primary (Sec 6).
- Laporan vendor: maker (vendor) ≠ checker (SPV), RBAC di route **dan** service, `audit_logs` dalam tx yang sama (Sec 5).
- Vendor hanya bisa melapor / melihat ATM & order miliknya (Sec 5 "vendors scoped to own assignments").
- Kuota kunjungan tetap terpotong sekali per (request, ATM) — idempoten seperti sekarang.
- Job klasifikasi ringan + idempoten per tanggal (boleh dijalankan ulang); jangan bulk berat di 07:00–20:00 (Sec 14).
- Tidak ada hard delete; `itm_replenish` tidak diubah.
- Tidak ada dependency baru (export CSV stdlib).

## Resolved decisions (tanya-jawab satu per satu dengan user, 2026-10-08)
1. Q1 — Sumber realisasi resmi = **`itm_replenish`**; vendor **juga wajib melapor**.
2. Q2 — **Vendor Request `approved` = instruksi**; tidak ada `replenishment_instructions`.
3. Q3 — Tanggal jadwal = **`replenish_date` tiket**.
4. Q4 — **Tabel `holidays` sekarang**; Q4b — **langsung berlaku + audit** (deviasi GR#3); Q14 — dikelola **ADMIN saja**.
5. Q5 — Klasifikasi **harian otomatis** (setelah ETL ITM).
6. Q6 — Vendor lapor **tanggal + nominal via VendorPortal**; Q6b — **menggantikan input ATM-USER**, internal approve,
   kuota terpotong seperti sekarang; Q6c — **ITM menang**, selisih ditandai.
7. Q13 — Approver laporan vendor = **ATM-SPV / BRANCH-ATM-SPV**.
8. Q7 — Vendor **melihat klasifikasi miliknya sendiri**.
9. Q8 — Status `processing` / `failed` **dibiarkan tidak dipakai**.
10. Q9 — Report: **amount ≠ order** + **trip/realisasi per vendor per ATM**; Q12 — export **CSV**.
11. Q10 — Pemenuhan dana / CIT pickup **di luar 2.2**.
12. Q11 — Nominal tidak sesuai = **selisih persis > 0**, tanpa toleransi.

## Open questions untuk spec (detail desain, bukan keputusan bisnis)
- Pencocokan `itm_replenish` ↔ tiket: kunci `terminal_id` + jendela tanggal; bila satu ATM terisi dua kali dalam jendela, mana yang dipakai.
- Runtime job klasifikasi: Python EOD scheduler (sudah ada) atau goroutine di `cmd/api`.
- Batas waktu vendor melapor + perlakuan laporan yang tidak pernah masuk (kuota tidak terpotong?).
- Request lama yang sudah `completed` lewat laporan ATM-USER: tetap valid, tidak diklasifikasi ulang dari laporan vendor.

## Out of scope (usulan)
- Engine forecast (dibatalkan 2026-10-08, CROWN tidak forecasting).
- Pemenuhan dana / CIT (flow 04) — Phase 2.2a/2.2b.
- Report refund per ID, order vs transaksi, transaksi tarik/setor, user log.
- Export XLSX/PDF (Phase 0.5).
- Exclude ATM bermasalah + list input FSD → Phase 2.4.
- Laporan DSR telat → Phase 2.3.
- Invoice / rekonsiliasi escrow → Phase 3/4.
