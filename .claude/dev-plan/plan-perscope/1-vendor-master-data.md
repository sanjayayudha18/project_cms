# Area 1 — Management vendor dan master data — 🚧 sebagian besar selesai

> Detail area dari [`../dev-plan.md`](../dev-plan.md). Dipindah dari `.kiro/steering/development-plan.md` (2026-11-12). Checklist dicek terhadap kode + git 2026-11-12.

Item fase: master data maker-checker (Phase 1), [`vendor-pks`, `vendor-cis-limit`, X.7, X.9, X.10, X.16, X.17](../dev-phase/lintas-fitur.md). Aturan: scope guard rule 2 (hanya item FSD §1).

## Checklist
- [x] Master data maker-checker: vendor, cabang, vault, PIC, paket, harga, ATM, kelolaan ATM, peta lokasi DSR
- [x] Region, Area ACM, hari libur, admin user, role management
- [x] Impor CSV revamp (X.10), ekspor async + katalog/pratinjau (X.7, X.9, X.16)
- [x] Vendor PKS + Direktur Utama (mig `022`/`037`)
- [x] Revamp ekspor vendor_vaults (`.agents/specs/vendor-vaults-export-revamp`, mig `040` applied ke dev 2026-11-12, `dde5cfc`)
- [ ] `vendor-cis-limit`: P1–P5 selesai (mig `038`); P6 endpoint inquiry, P7 CompanyPortal, P8 dokumen belum
- [ ] X.17 `ext-user-api`: kode di-commit (`fd19cdc`); gate task 12 + cek manual task 13 outstanding
- [ ] Sisa FSD §1: Service Type, kolom Region di master vendor, notifikasi in-app PKS Amber/Red ke PIC, "Terminated" manual

## Item FSD §1
Sumber review FSD: [`fsd-v1-alignment-2026-10-01.md` §7](../../docs/fsd-v1-alignment-2026-10-01.md) (2026-11-03). Item FSD di bawah belum disetujui/dibangun kecuali dicentang; tetap butuh prioritas user, pertanyaan AI-DLC (CLAUDE.md Sec 4a rule 0), dan persetujuan skema.

| Item FSD | Item pemilik | Catatan |
| --- | --- | --- |
| Master vendor: Service Type, Escrow ATM/Cash, Limit CIS, kolom Region; notifikasi in-app PKS Amber/Red ke PIC berwenang; "Terminated" manual | `vendor-pks-cis-limit` (mig `022`) | Butuh pengecualian scope guard rule 2 saat dimulai |

## Konflik PO
- **#3** Ambang PKS: FSD < 8 / Amber < 6 / Red 3 / Green ≥ 9 bulan vs repo 9 bulan (A4); daftar nilai status berbeda.
