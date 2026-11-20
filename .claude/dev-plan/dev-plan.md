# Rencana Pengembangan CROWN — Ringkasan

> Rencana hidup. Dipindah dari `.kiro/steering/development-plan.md` (2026-11-12, user). Perbarui saat sebuah item selesai, tabel disetujui, atau dependensi bergeser.
> Detail per area: [`plan-perscope/`](./plan-perscope/) · detail per fase: [`dev-phase/`](./dev-phase/) · status built/mock/gap: [`status-snapshot.md`](./status-snapshot.md) · changelog: [`../development-progress.md`](../development-progress.md).

---

## Scope CROWN (enam area)

> **Angka adalah nomor area, bukan urutan kerja.** Urutan ada di "Fokus saat ini".

Legenda: ✅ selesai (tersambung + teruji; cek browser manual oleh user tetap outstanding) · 🚧 dalam development · ⏳ belum mulai · ⏸ ditahan

| # | Area | Status | Item fase | Detail |
| --- | --- | --- | --- | --- |
| 1 | Management vendor dan master data | 🚧 sebagian besar selesai | master data, `vendor-pks`, `vendor-cis-limit`, X.7/X.10/X.16/X.17 | [1-vendor-master-data.md](./plan-perscope/1-vendor-master-data.md) |
| 2 | Menerima dan memproses forecast DMAA | ✅ | 2.1 | [2-forecast-dmaa.md](./plan-perscope/2-forecast-dmaa.md) |
| 3 | ATM cash forecasting harian (DMAA + DSR vendor → penyesuaian nominal → order replenish final) | ⏳ | 2.3 ✅, **2.4a**, 2.4b ⏸ | [3-forecasting-harian.md](./plan-perscope/3-forecasting-harian.md) |
| 4 | Pemenuhan Order ATM Replenish | ✅ | 2.2a–2.2d | [4-pemenuhan-order.md](./plan-perscope/4-pemenuhan-order.md) |
| 5 | Schedule dan execution Cash Count | ⏳ | Phase 5 | [5-cash-count.md](./plan-perscope/5-cash-count.md) |
| 6 | Invoicing trip ATM Replenish | ⏳ | Phase 3 | [6-invoicing-trip.md](./plan-perscope/6-invoicing-trip.md) |

Pembayaran di SMART di luar scope.

---

## ⛔ Scope guard — baca sebelum memulai task APA PUN

**Scope (user 2026-11-12): CROWN bukan lagi MVP.** Target = FSD v1.0 §A (`.document/DSR End To End Cash Management V01.md`), enam area di atas. Phase 4, 6, 7 tetap diparkir.

**Fokus saat ini (berurutan, user 2026-11-12):**
1. **2.4a** forecasting kas ATM harian: penyesuaian DSR (FSD §3 langkah 8) + order replenish final (area 3).
2. **Phase 3** invoicing trip (area 6), setelah 2.4a.
3. **Phase 5** cash count (area 5) + ingest saldo escrow yang dibutuhkannya.
- Berjalan bersamaan: `vendor-cis-limit` (P6), X.17 `ext-user-api` (gate + cek manual). Ditahan: **2.4b** (belum ada data ATM bermasalah).

Apa pun di luar enam area dan item lintas-fitur adalah **di luar scope**. Jika sebuah permintaan menyentuhnya, BERHENTI dan tanya.

Aturan:
1. **Satu task = satu item di rencana ini.** Bukan item di sini → tambahkan dulu (dengan persetujuan user), baru dibangun.
2. **Master data: hanya item FSD §1** (user 2026-11-12). Item FSD §1 (Service Type, escrow ATM/Cash, kolom Region di master vendor, notifikasi in-app PKS Amber/Red ke PIC, "Terminated" manual) boleh dibangun — masing-masing sebagai item, dengan spec dan pola maker-checker master data. Hal lain di master data butuh persetujuan user; selain itu hanya bugfix. Pengecualian: X.7 (user OK 2026-10-28), X.10 (2026-11-02), X.16 (2026-11-06).
3. **Phase 3 dan Phase 5 masuk scope** sesuai urutan di atas, masing-masing dengan spec Kiro lengkap (uang/rekonsiliasi, CLAUDE.md Sec 4 #7). Phase 5 mencakup **ingest file saldo escrow** untuk "Cek Escrow"; rekonsiliasi escrow penuh tetap di Phase 4. **Jangan sentuh Phase 4, 6, 7** kecuali user eksplisit mengubah prioritas.
4. **Jangan menyambungkan layar mock** ([status-snapshot.md](./status-snapshot.md)) secara ad hoc — hanya lewat item fase pemiliknya.
5. Tabel/kolom baru → catat di `.claude/CLAUDE.md` Sec 3 saat diterapkan. Migrasi lewat `go run ./cmd/migrate up` (jangan `psql -f`). **Nomor bebas berikutnya: `041`** (`040` vendor-vaults-export-revamp `dde5cfc`; `039` ext-user-api; `038` vendor-cis-limit; `022` + `037` vendor-pks; `024` dibuang).
6. Known gap yang diterima ([status-snapshot.md](./status-snapshot.md)) **tidak** boleh "diperbaiki sambil lalu".

---

## Fase

> **Penomoran ≠ urutan eksekusi.** Nomor dipertahankan stabil karena dokumen lain merujuknya (mis. "0.3", "2.1").

| Fase | Status | Detail |
| --- | --- | --- |
| 0 Fondasi infrastruktur | ✅ (0.4, 0.5 ditunda) | [phase-0-fondasi.md](./dev-phase/phase-0-fondasi.md) |
| 1 Tutup pekerjaan berjalan | ✅ | [phase-1-tutup-berjalan.md](./dev-phase/phase-1-tutup-berjalan.md) |
| 2 Operasional ATM | ← **SAAT INI** (2.4a berikutnya) | [phase-2-operasional-atm.md](./dev-phase/phase-2-operasional-atm.md) |
| 3 Finance (invoicing trip) | ⏳ masuk scope | [phase-3-finance.md](./dev-phase/phase-3-finance.md) |
| 4 Integrasi (rekonsiliasi escrow) | 🅿 diparkir | [phase-4-integrasi.md](./dev-phase/phase-4-integrasi.md) |
| 5 Cash count | ⏳ masuk scope | [phase-5-cash-count.md](./dev-phase/phase-5-cash-count.md) |
| 6 Sisa CIT | 🅿 diparkir | [phase-6-sisa-cit.md](./dev-phase/phase-6-sisa-cit.md) |
| 7 EOD | 🅿 diparkir | [phase-7-eod.md](./dev-phase/phase-7-eod.md) |
| X Lintas-fitur | campuran | [lintas-fitur.md](./dev-phase/lintas-fitur.md) |

### Graf dependensi

```
Phase 1 ─> Phase 0 ─> Phase 2 (2.1 ✅ ─> 2.2a ✅ ─> 2.2b ✅ ─> 2.2c ✅ ─> 2.2d ✅ ; 2.3 ✅ ; 2.4a ⏳ ─> 2.4b ⏸)
                         │
                         ├─> 2.4a ─> 3 Finance (invoicing) ─> 5 Cash count (+ ingest saldo escrow)
                         ├─> 4 Rekonsiliasi escrow (diparkir)
                         ├─> 7 EOD
                         └─> 6 Sisa CIT
```

---

## Strategi urutan

**Tutup yang sedang berjalan → infra → operasional ATM.** Infra bersama lebih dulu agar modul bisnis tidak masing-masing menemukan ulang job async, notifikasi, penyimpanan dokumen, dan read replica.

Aturan mengikat:
- State machine + orkestrasi berbasis DB untuk alur panjang (`.kiro/steering/how_to_handle_flowprocess.md`), jangan satu endpoint sinkron yang panjang.
- Maker-checker lewat `approval.Orchestrator` yang ada — jangan state machine kedua.
- Uang = numeric/integer minor units (string desimal di wire). Write → primary, laporan/dashboard → replica.
- Setiap perubahan state menulis `audit_logs`. Ingest file idempoten per hash file.
- Modul baru dimulai dengan spec OpenAPI-nya (`backend/api/openapi.yaml`).

## Keputusan yang sudah ditetapkan

1. **Runtime EOD** (2026-09-25): Python (`backend_python/`). Tanpa Go `cmd/batch`.
2. **Rumus Order ATM** (2026-11-12, user): CROWN menyesuaikan nominal DMAA dengan DSR sesuai FSD §3 langkah 8; menggantikan "DMAA final" 2026-10-08.
3. **Filter `branch_id` cabang + disable-with-children**: lihat Phase 1.2.
4. **Tabel cash count**: nama `cash_count_schedules` + `cash_count_evidences` disetujui; kolom diputuskan di spec Phase 5.
5. **Lokasi CIT** (2026-10-08): fungsi CIT di `backend/`; `backend-cit` di-rename `backend-exq` (2026-10-20).
6. **Penetapan vault** (2026-10-13): alokasi nominal per vault + area ACM per region (v3) menggantikan per-ATM (v1).
7. **Migrasi** (2026-10-15): runner kustom `backend/cmd/migrate`; golang-migrate tidak dipakai.
8. **Scope** (2026-11-12, user): bukan lagi MVP — enam area FSD v1.0 §A; invoicing = CROWN menghitung tagihan + loop banding, tanpa pembayaran; Phase 3 + 5 masuk scope, 4/6/7 diparkir.

## Konvensi kerja

1. Spec dulu untuk pekerjaan non-trivial: `.kiro/specs/<feature>/` (requirements → design → tasks) atau `.claude/sdlc/<feature>/` (legacy). Spec selesai dipindah ke `.kiro/specs/.done/` / `.claude/sdlc/.done/`.
2. Setiap task membawa **`Model:` + `Effort:`** (`.kiro/steering/model-recommendation.md`).
3. Test menyertai setiap fitur; coverage ≥80% di `internal/*`, ambang FE lewat `pnpm run test:coverage`. Verifikasi browser manual dilakukan user (Golden Rule #10).
4. Saat item selesai: perbarui status di file area + file fase terkait, tabel di file ini, `.claude/development-progress.md`, dan CLAUDE.md Sec 3/12 bila skema berubah.
5. Jaga "selesai" tetap jujur: tersambung + didukung DB + teruji.
