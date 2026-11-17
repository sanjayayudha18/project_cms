# Area 3 — ATM cash forecasting harian — ⏳ (pendukung sudah ada)

> Detail area dari [`../dev-plan.md`](../dev-plan.md). Dipindah dari `.kiro/steering/development-plan.md` (2026-11-12). Checklist dicek terhadap kode + git 2026-11-12.

Memproses data DMAA dan DSR vendor untuk penyesuaian nominal replenishment dan pembentukan final replenishment order (FSD v1.0 §3). Item fase: [2.3, 2.4a, 2.4b](../dev-phase/phase-2-operasional-atm.md). **2.4a = Fokus saat ini #1.**

## Checklist
- [x] Pendukung: upload DSR vendor (dry-run/commit), laporan DSR telat 2.3
- [ ] **2.4a** penyesuaian DSR (FSD §3 langkah 8) + order replenish final 12:30 — berikutnya, spec belum dibuat
- [ ] ⏸ **2.4b** pengecualian ATM bermasalah + upload complaint/project — ditahan, data ATM bermasalah belum ada

## Item FSD §3
Sumber review FSD: [`fsd-v1-alignment-2026-10-01.md` §7](../../docs/fsd-v1-alignment-2026-10-01.md) (2026-11-03). Item FSD di bawah belum disetujui/dibangun kecuali dicentang; tetap butuh prioritas user, pertanyaan AI-DLC (CLAUDE.md Sec 4a rule 0), dan persetujuan skema.

| Item FSD | Item pemilik | Catatan |
| --- | --- | --- |
| Timeline forecast harian (06:00 → order final 12:30), upload complain/project, ATM ID ganda dibuang + email info, ATM tidak ada di forecast → order dengan nominal kosong diisi ATM Monitoring, penyesuaian manual 11:45–12:00 | 2.4 | Tambahkan ke spec 2.4a/2.4b |

## Konflik PO
- ~~**#1** Rumus order langkah 8 vs keputusan 2026-10-08 (nominal DMAA final)~~ — **selesai 2026-11-12 (user):** langkah 8 `(Forecast Replenish + Plan Replenish) − (Saldo DSR + Forecast Refund)` berlaku.
