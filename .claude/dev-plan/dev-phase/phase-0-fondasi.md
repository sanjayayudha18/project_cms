# Phase 0 — Fondasi infrastruktur ✅ SELESAI (0.4, 0.5 ditunda)

> Dipindah dari `.kiro/steering/development-plan.md` (2026-11-12). Ringkasan: [`../dev-plan.md`](../dev-plan.md).

| Item | Status |
| --- | --- |
| 0.1 Routing read-replica (Go + Python `db_read_pool`) | ✅ 2026-10-07. Sengaja tetap di primary: read-after-write upload DSR, pembacaan badge perubahan master data |
| 0.2 `import_jobs` + helper idempotensi | ✅ Helper Python + EOD (mig `019`/`020`). Helper Go ditunda sampai ingest file Go pertama (Phase 3 invoice / 4 escrow) |
| 0.3 `internal/notification` (in-app + SMTP) | ✅ `eb457cf`, review R1–R3 diperbaiki. Pengiriman SMTP nyata menunggu kredensial relay |
| 0.4 `internal/document` | ⏸ ditunda ke Phase 3/5 (intent diterima, `.claude/sdlc/document/`) |
| 0.5 `internal/export` (XLSX/PDF) | ⏸ ditunda; dependensi baru (excelize, tool PDF) butuh persetujuan (Golden Rule #1) |

> Worker async: dibuang sebagai item tersendiri. Putuskan per fitur (goroutine + tabel job DB vs antrean Redis vs pipeline Python).
