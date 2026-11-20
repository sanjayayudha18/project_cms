# Area 5 — Schedule dan execution Cash Count — ⏳

> Detail area dari [`../dev-plan.md`](../dev-plan.md). Dipindah dari `.kiro/steering/development-plan.md` (2026-11-12). Checklist dicek terhadap kode + git 2026-11-12.

Item fase: [Phase 5](../dev-phase/phase-5-cash-count.md). **Fokus saat ini #3**, setelah Phase 3.

## Checklist
- [ ] `internal/cashcount` + migrasi `cash_count_*` (nama tabel disetujui); layar jadwal/evidence VendorPortal masih mock
- [ ] Ingest saldo escrow untuk "Cek Escrow"
- [x] Konflik PO #2 (delta Rec 7) dan #5 (jendela kategori) diputuskan dulu — 2026-11-12, ScDD 5.0 K1 + K2

## Item FSD §4–§5
Sumber review FSD: [`fsd-v1-alignment-2026-10-01.md` §7](../../docs/fsd-v1-alignment-2026-10-01.md) (2026-11-03). Item FSD di bawah belum disetujui/dibangun kecuali dicentang; tetap butuh prioritas user, pertanyaan AI-DLC (CLAUDE.md Sec 4a rule 0), dan persetujuan skema.

| Item FSD | Catatan |
| --- | --- |
| Cash count vault: H > 15 bio (3×), M 5–15 bio (2×), L < 5 bio (1×) per 6 bulan; ambang selisih Rp10 juta (≤ hitung ulang, > eskalasi ke Direktur Ops & IT); approval D3; BA termasuk UTLE + Uang Rusak; kunjungan onsite atau virtual; rumus Selisih Cash Count / Cek Escrow | Rumus = uang/rekonsiliasi (Sec 4 #7) |
| Cash count selektif mesin: surat tugas multi-ATM, status Assigned → … → Ready for Execution, inspeksi Hari 1 + BA segel (poin 1–9), hitung per-kaset Hari 2 vs Remaining Bill Counter (poin 10–15), loop review CCR, approval D3 → D2, parameter Kategori ATM Anomaly + Checklist Berita Acara | Sebelumnya "kosong di FSD" |
| 11 laporan cash count (§5): Summary Plan Vault, Status, Progress ATM/Cash/Valas, Selisih, Penalty, Temuan Vault, Performance MtM + Rank, Rekap | Baca dari replica; ekspor lewat `backend-exq` |

Tabel usulan (belum disetujui): `cash_count_category_params`, `cash_count_pics`, `cash_count_reschedule_requests` — [`docs/data-map.md`](../../docs/data-map.md).

## Konflik PO
- **#2** Rekonsiliasi cash count: hasil Rec 7 diinput manual ke CROWN — tinjau ulang "CROWN tidak menghitung delta cash count". **Diputuskan 2026-11-12 (user, ScDD 5.0 K1):** CROWN menghitung Selisih Cash Count, Cek Escrow, dan Selisih Final (= Selisih CC − Uang Masuk + Uang Keluar, tanda + contoh angka menunggu PO); Uang Masuk/Keluar diinput dari Rec 7.
- **#5** Jendela kategori cash count: "3 Mths / 1 Mths" vs rata-rata 6 bulan. **Diputuskan 2026-11-12 (user, ScDD 5.0 K2):** rata-rata 6 bulan, bulanan naik-saja (L614 salah ketik).
- ~~Terbuka: sumber saldo escrow H-1 (file SIBS vs Corebanking).~~ Diputuskan 2026-11-12 (user, ScDD 5.0 K3): file harian Corebanking; contoh file masih ditunggu.
