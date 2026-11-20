# 5 Cash Count — Roadmap ScDD
> Status: accepted 2026-11-12 (user)

Area: [`../../5-cash-count.md`](../../5-cash-count.md) (checklist area + konflik PO #2, #5, sumber saldo escrow). Sumber: [`.document/DSR End To End Cash Management V01.md`](../../../../../.document/DSR%20End%20To%20End%20Cash%20Management%20V01.md) §A.5 "Schedule dan execution Cash Count" (L485–L1228) + §5 Reporting Requirement bagian cash count (L1459–L1513). Pendukung: [`fsd-v1-alignment-2026-10-01.md` §7](../../../../docs/fsd-v1-alignment-2026-10-01.md), [`data-map.md` → Proposed FSD v1.0](../../../../docs/data-map.md) (tabel usulan, belum disetujui).

Pilot pertama ScDD. Fase 5.0 berisi semua poin daftar periksa yang gagal: [5.0-keputusan.md](5.0-keputusan.md).

## Pembagian fase vs draft awal

Draft awal (7 fase: parameter, pre-cash count, eksekusi vault, eksekusi selektif mesin, rekonsiliasi, laporan) disesuaikan setelah membaca sumber penuh:

- **Saldo escrow harian dipisah jadi fase sendiri (5.2).** Kategori vault H/M/L dihitung dari rata-rata saldo escrow harian 6 bulan (L601–L602, L611) dan "Cek Escrow" butuh saldo escrow (L901–L903). Data ini belum ada di CROWN; user memutuskan ingest-nya bagian Phase 5 (2026-11-12). Fase ini bisa jalan paralel dengan 5.1.
- **Pre-cash count dipecah dua (5.3, 5.4).** Sumber punya dua aktor dengan alur berbeda: rencana kunjungan (kategorisasi + tanggal acak + review Tim CWM + review/submit PIC Coordinator, L595–L615, L660–L710) dan penugasan PIC Executor (task, konfirmasi/reschedule, memo audit, kirim data petugas ke vendor, L711–L728).
- **Selektif mesin dipecah dua (5.6, 5.7).** Penjadwalannya terpisah dari vault, oleh Cash Control & Resolution dengan surat tugas multi-ATM dan state machine sendiri (L922–L974); eksekusinya dua hari (Day 1 inspeksi + segel L978–L1065, Day 2 hitung per kaset L1069–L1121).
- **Rekonsiliasi + approval jadi satu fase bersama (5.8).** Langkah setelah laporan petugas (task ke ATM Digital & Reconciliation, input hasil Rec 7, remark CCR, finalisasi, approval D3 / D3→D2, eskalasi, kirim ke D1) sama untuk vault (L821–L855) dan mesin (L1122–L1150) dan dijelaskan lagi di §4 (L1207–L1228). Fase eksekusi (5.5, 5.7) berhenti di "submit laporan petugas".
- **Parameter (5.1) hanya master data**: kategori H/M/L, kategori ATM Anomaly, checklist Berita Acara, mapping PIC. Algoritme kategorisasi ("Metode Kunjungan", L595–L615) dan frekuensi tanggal acak (L660–L685) pindah ke 5.3 karena menghasilkan rencana, bukan parameter.

## Daftar Periksa

Kode keputusan K1–K17 ada di [5.0-keputusan.md](5.0-keputusan.md). "Tabel": semua nama tabel disetujui (K8, 2026-11-12); ✅ di kolom ini = nama disetujui, kolom tetap diputuskan di gerbang 1 fase.

| Fase | Sumber | Bisa ditunjuk | Konflik PO | Tabel | Catatan |
|---|---|---|---|---|---|
| 5.1 | ✅ L489–L658 | ✅ isi checklist = parameter tanpa seed (K15 cadangan) | ✅ K4, K6, K7, K12; STCC dipindah ke 5.3 | ✅ K8; + `cash_count_areas`, `cash_count_area_regions` (disetujui gerbang 1, 2026-11-12) | gerbang 1 accepted 2026-11-12 |
| 5.2 | ❌ sumber diputuskan Corebanking (K3), contoh file belum ada | ❌ kolom file belum diketahui | ✅ K3 | ✅ nama `escrow_batch_files`/`escrow_batch_rows` (CLAUDE.md §3) | menunggu contoh file Corebanking |
| 5.3 | ✅ L595–L615, L660–L710, L732–L774 | ✅ algoritme diputuskan (K5), L614 vs L615 diputuskan (K2) | ✅ STCC = kolom tampilan saja (5.3 D1, 2026-11-16; arti tetap ditunggu dari PO) | ✅ K8; + `cash_count_plans`, `cash_count_plan_areas` (disetujui gerbang 1, 2026-11-16) | gerbang 1 accepted 2026-11-16; build menunggu 5.2 (tabel saldo) |
| 5.4 | ✅ L711–L728 | ❌ format memo "Audit Cash Count Vault" tidak ada di sumber (L725–L726) | ❌ template + library PDF (K10) | ✅ K8 | menunggu 5.0 + 5.3 |
| 5.5 | ✅ L784–L820, L863–L914 | ✅ rumus Selisih Cash Count (L892) + ambang Rp10 juta (L810–L811) tertulis | ✅ K1, K11, K14 | ✅ K8 | menunggu 5.0 + 5.4; §4 #7 (uang) |
| 5.6 | ✅ L922–L974 | ✅ status Assigned → Requested Reschedule → Reschedule → Accepted → Ready for Execution tertulis | ❌ template + library PDF (K10) | ✅ K8 | menunggu 5.0 + 5.1 |
| 5.7 | ✅ L978–L1121, L1160–L1205 | ❌ isi checklist poin 1–15 di lampiran gambar (L585, L589); sumber Remaining Bill Counter tidak disebut (L1080, L1092); arti "update pembatalan pada jadwal bulan berjalan" (L1063) | ❌ template BA Day 2 + library PDF (K10); lampiran checklist (K15) | ✅ K8 | menunggu 5.0 + 5.6; §4 #7 (uang) |
| 5.8 | ✅ L821–L857, L1122–L1150, L1207–L1228 | ❌ rumus Selisih Final dipilih (K1) tetapi tanda + contoh angka dari PO belum ada | ✅ K1 (tanda + contoh angka menunggu PO), K11 | ✅ K8 | menunggu 5.0 + 5.5 (+ 5.7 untuk mesin); §4 #7 (rekonsiliasi) |
| 5.9 | ✅ L1459–L1507 (deskripsi 11 laporan) | ❌ lampiran "Cash Count Report Upd after meeting 22" (L1513) tidak ada di repo: kolom, rumus Penalty, SLA selisih, persen Performance Rank, kategori temuan 1–20 | ❌ lampiran laporan (K15); Valas di luar scope (K14) | ✅ hanya baca (replica); kind ekspor baru = migrasi kecil `export_jobs` saat gerbang 1 | menunggu 5.0 + 5.8 |

## Fase
| Fase | Nama | File | Status | Ukuran |
|---|---|---|---|---|
| 5.0 | Keputusan (tanpa kode) | [5.0-keputusan.md](5.0-keputusan.md) | draft | S |
| 5.1 | Parameter cash count + mapping PIC (kategori H/M/L, kategori ATM Anomaly, checklist BA, area, PIC Coordinator/Executor, peran `CC-*`) | [5.1-parameter.md](5.1-parameter.md) | in-build (gerbang 2 diterima user 2026-11-16; review berjalan) | M |
| 5.2 | Saldo escrow harian (ingest file → `escrow_batch_rows`) | [5.2-saldo-escrow.md](5.2-saldo-escrow.md) | blocked (draft 2026-11-16; contoh file Corebanking OPEN-1…OPEN-7) | M |
| 5.3 | Kategorisasi vault + rencana kunjungan 6 bulan (rata-rata saldo, aturan naik-saja, onsite/virtual, tanggal acak, review Tim CWM + PIC Coordinator) | [5.3-rencana-kunjungan.md](5.3-rencana-kunjungan.md) | accepted 2026-11-16 (gerbang 1; build setelah 5.2) | L |
| 5.4 | Penugasan PIC Executor (task, jadwal 1 bulan, reschedule, konfirmasi, memo audit, data petugas ke vendor) | belum ditulis | — | M |
| 5.5 | Eksekusi cash count vault (hitung per denom, checklist, BA ATM/MDM + Cash, DSR H-1 otomatis, Selisih Cash Count, hitung ulang/eskalasi Rp10 juta, submit) | belum ditulis | — | L |
| 5.6 | Selektif mesin — penjadwalan (surat tugas multi-ATM, PIC 1/2, accept/reschedule, ID card, task ke vendor) | belum ditulis | — | M |
| 5.7 | Selektif mesin — eksekusi Day 1 (inspeksi, BA segel, poin 1–9, masalah mesin ≤/> 1 bulan) + Day 2 (hitung per kaset vs Remaining Bill Counter, poin 10–15, investigasi selisih, review kelengkapan CCR) | belum ditulis | — | L |
| 5.8 | Rekonsiliasi + approval (task ATM Digital & Reconciliation, input hasil Rec 7, Cek Escrow, remark CCR, D3 / D3→D2, eskalasi, kirim D1, arsip rekap) | belum ditulis | — | M |
| 5.9 | Laporan cash count (11 laporan/dashboard, replica, ekspor via `backend-exq`) | belum ditulis | — | L |

Status: `draft` / `accepted` / `in-build` / `blocked` / `done <sha>`.

## Ketergantungan
- 5.0 dulu; tiap fase hanya menunggu keputusan yang tercantum di kolom Konflik PO-nya.
- 5.1 ∥ 5.2 (tidak saling butuh).
- Jalur vault: 5.3 setelah 5.1 + 5.2 → 5.4 → 5.5.
- Jalur mesin: 5.6 setelah 5.1 → 5.7. Jalur mesin ∥ jalur vault.
- 5.8 setelah 5.5 (bagian vault); bagian mesin setelah 5.7. Bagian Cek Escrow butuh 5.2.
- 5.9 setelah 5.8.
- Prasyarat di luar scope: `internal/document` (rantai `.claude/sdlc/document/`, K9) harus selesai sebelum 5.4–5.7 di-build (gerbang 2); rencana fasenya boleh ditulis lebih dulu.

## Keputusan Scope
- 2026-09-25 (user): nama tabel `cash_count_schedules` dan `cash_count_evidences` disetujui; kolom ditentukan di fase.
- 2026-10-05 (user, A2): Area Vault Vendor = `vendor_branches`. Sejak migrasi `040` (2026-11-12) nomor escrow ada di `vendor_vaults.escrow_cash` / `escrow_atm` (12 digit, tidak unik — satu escrow bisa dikelola beberapa vendor, dipakai untuk cek multi-vendor L707–L708).
- 2026-11-12 (user): ingest saldo escrow untuk "Cek Escrow" adalah bagian Phase 5, bukan Phase 4.
- 2026-11-12 (user): scope 5 memakai ScDD (pilot).
- 2026-11-12 (user, K8): nama tabel `cash_count_category_params`, `cash_count_pics`, `cash_count_reschedule_requests`, `cash_count_anomaly_categories`, `cash_count_checklist_items`, `cash_count_machine_assignments`, `cash_count_machine_assignment_atms`, `cash_count_vault_results`, `cash_count_machine_cassettes`, `cash_count_checklist_answers`, `cash_count_reconciliations` disetujui; kolom diputuskan di gerbang 1 tiap fase.
- 2026-11-12 (user): roadmap diterima (gerbang R).
- 2026-11-12 (user, K7): peran baru `CC-CWM`, `CC-COORDINATOR`, `CC-EXECUTOR`, `CC-CCR`, `CC-RECON`, `CC-D3`, `CC-D2` (CCR = Cash Count & Resolution = Cash Control & Resolution); rencana kunjungan = state machine sendiri + audit dalam tx (deviasi terdokumentasi seperti vault plan ACM 2.2a).
- 2026-11-12 (user, K3): saldo escrow harian = file harian Corebanking via MFT → Python EOD → `escrow_batch_rows`; contoh file masih ditunggu (syarat gerbang 1 fase 5.2).
- 2026-11-12 (user, K1 — konflik PO #2): CROWN menghitung Selisih Cash Count, Selisih Cek Escrow, dan Selisih Final Rekonsiliasi (= Selisih Cash Count − Uang Masuk + Uang Keluar); Uang Masuk/Keluar diinput dari Rec 7. Tanda rumus final + contoh angka masih ditunggu dari PO (gerbang 1 fase 5.8).
- 2026-11-12 (user, K2 — konflik PO #5): kategori awal = rata-rata saldo harian 6 bulan; bulanan hanya kenaikan yang diterapkan (L615 benar, L614 salah ketik), berlaku bulan berikutnya.
- 2026-11-12 (user, K4): Low < Rp5 M ≤ Med ≤ Rp15 M < High; basis = total escrow ATM + Cash per area vault vendor (`vendor_branches`); parameter lewat maker-checker.
- 2026-11-12 (user, K5): rencana 6 bulan, pola onsite/virtual diacak per vault, kunjungan tiap bulan, tanggal kalender tahun lalu dikecualikan, hari kerja saja, seed acak disimpan.
- 2026-11-12 (user, K6): PIC Coordinator/Executor = user internal (LDAP); area executor = lookup baru kecil; STCC = atribut vault (arti masih ditunggu dari PO); 2 executor per kunjungan vault.
- 2026-11-12 (user, K9): `internal/document` dibangun sekarang sebagai prasyarat scope 5, di rantai legacy `.claude/sdlc/document/` (CLAUDE.md §4a rule 7), bukan fase ScDD; pembagian fase tidak berubah.
- 2026-11-12 (user, K10): memo, surat tugas, BA = PDF dari template PO + e-sign "persetujuan klik + jejak audit" (user, waktu, IP, hash PDF di `audit_logs`), tanpa layanan luar. Template PO + persetujuan library PDF (dependency baru) masih ditunggu.
- 2026-11-12 (user, K11): ambang Rp10 juta pada |selisih| per BA, tepat Rp10 juta = hitung ulang, ambang = parameter; email eskalasi + D1 otomatis via `notification_emails` ke daftar penerima yang dikelola ADMIN.
- 2026-11-12 (user, K12): mesin selektif dipilih manual oleh CCR dengan kategori ATM Anomaly wajib sebagai alasan per ATM; Day 1 = H+1, Day 2 = H+2 hari kerja dari tanggal jadwal.
- 2026-11-12 (user, K13–K17 — "setuju semua rekomendasi, saya akan review nanti"): K13 vendor dihubungi lewat email ke PIC `cashcount` (VendorPortal ditunda); K14 Cash Valas di luar scope rilis pertama; K15 lampiran asli diminta dari PO, cadangan checklist = parameter tanpa seed; K16 Remaining Bill Counter diinput petugas + foto struk; K17 Cancelled = jadwal bulan berjalan ditandai batal, jadwal ulang manual oleh CCR.
- 2026-11-12 (user, gerbang 1 fase 5.1): nama tabel `cash_count_areas` + `cash_count_area_regions` disetujui; parameter kategori = satu baris per set; peran maker per entity type di engine maker-checker (`CC-CWM` / `CC-CCR` untuk entitas `cash_count_*`).
- 2026-11-16 (user, gerbang 1 fase 5.3): nama tabel `cash_count_plans` + `cash_count_plan_areas` dan kolom `vendor_branches.is_cash_count_stcc` disetujui; rencana kunjungan = state machine sendiri + audit dalam tx (deviasi GR#3 seperti vault plan ACM); aksi job Go tanpa `audit_logs` (structured log + jejak di tabel).
- Berlaku untuk semua fase (dari CLAUDE.md, bukan keputusan baru): fase dengan migrasi, uang, atau rekonsiliasi (minimal 5.2, 5.3, 5.5, 5.7, 5.8) wajib mengisi `## Gerbang Sec 4 #7`; master data lewat maker-checker (`/cms2-master-data`) kecuali deviasi GR#3 yang diputuskan; laporan dibaca dari replica dan diekspor lewat `backend-exq`; hari kerja dari tabel `holidays` (migrasi `031`).
- Data yang sudah ada dan relevan: `vendor_pics.roles` sudah punya nilai `cashcount` (migrasi `032`) — kandidat penerima task vendor (K13); halaman `CompanyPortal-Vite/src/routes/cash-count/` masih kartu `disabled`.
