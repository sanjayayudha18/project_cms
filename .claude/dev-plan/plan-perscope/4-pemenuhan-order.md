# Area 4 — Pemenuhan Order ATM Replenish — ✅

> Detail area dari [`../dev-plan.md`](../dev-plan.md). Dipindah dari `.kiro/steering/development-plan.md` (2026-11-12). Checklist dicek terhadap kode + git 2026-11-12.

Item fase: [2.2a–2.2d](../dev-phase/phase-2-operasional-atm.md). Alur ujung ke ujung: `.claude/feature-flows/10-order-atm-lifecycle/feature-flow.md`.

## Checklist
- [x] Vendor Request + state machine, tiket replenish per region, kuota kunjungan
- [x] 2.2a penetapan vault ACM (v3: alokasi per vault + tiket pemenuhan vault)
- [x] 2.2b kirim ke vendor, terima/tolak di VendorPortal
- [x] 2.2c realisasi vs order (`replenish_classify`)
- [x] 2.2d akun vendor per peran + cakupan cabang
- [ ] Sisa: cek browser manual (user), build Docker, merge `dev1` → `main` (X.6)
