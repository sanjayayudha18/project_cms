# Phase 4 — Integrasi (rekonsiliasi escrow) — 🅿 diparkir

> Dipindah dari `.kiro/steering/development-plan.md` (2026-11-12). Ringkasan: [`../dev-plan.md`](../dev-plan.md).

**Jangan mulai tanpa prioritas ulang eksplisit dari user.**

| Item | Gate |
| --- | --- |
| Ingest batch escrow `internal/corebanking` → `escrow_batch_files` / `escrow_batch_rows` → rekonsiliasi vs posisi kas CMS | 0.2, Phase 3 |

- Ingest **saldo escrow** untuk "Cek Escrow" cash count bukan bagian fase ini — ada di Phase 5 (user 2026-11-12).
- Masih terbuka dari FSD: sumber saldo escrow H-1 (file SIBS vs Corebanking).
