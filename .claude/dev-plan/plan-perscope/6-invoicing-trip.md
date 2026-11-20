# Area 6 — Invoicing trip ATM Replenish — ⏳

> Detail area dari [`../dev-plan.md`](../dev-plan.md). Dipindah dari `.kiro/steering/development-plan.md` (2026-11-12). Checklist dicek terhadap kode + git 2026-11-12.

Item fase: [Phase 3](../dev-phase/phase-3-finance.md). **Fokus saat ini #2**, setelah 2.4a. CROWN menghitung tagihan trip vendor + loop banding + approval; **tanpa eksekusi pembayaran** (SMART di luar scope).

## Checklist
- [ ] `internal/invoice` + tabel `invoice_*`; `features/invoice` CompanyPortal masih mock
- [ ] Mulai setelah 2.4a; konflik PO #4 (jadwal invoice) diputuskan dulu

## Item FSD §6
Sumber review FSD: [`fsd-v1-alignment-2026-10-01.md` §7](../../docs/fsd-v1-alignment-2026-10-01.md) (2026-11-03). Item FSD di bawah belum disetujui/dibangun kecuali dicentang; tetap butuh prioritas user, pertanyaan AI-DLC (CLAUDE.md Sec 4a rule 0), dan persetujuan skema.

| Item FSD | Catatan |
| --- | --- |
| Upload skema harga (replace, tanggal upload = efektif), trip maks per paket (3=30, 4=40, 5=50) + harga Additional Trip, prorata per hari untuk aktivasi/deaktivasi tengah bulan, hitungan penalti dari flag DSR-telat (2.3) + nominal manual, loop banding maks 3 putaran, final → task VOC. Master ATM dari file harian ESQ via MFT | **Mengubah scope Phase 3**: CROWN menghitung tagihan, bukan memvalidasi invoice yang diunggah vendor. Matematika uang → Sec 4 #7 |

## Konflik PO
- **#4** Jadwal invoice: "3 hari kerja terakhir minggu pertama bulan berikutnya" vs "3 hari kerja sebelum akhir bulan".
