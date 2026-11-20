# Review menu Pengaturan — catatan user (2026-11-17)
> Status: draft — catatan mentah, belum ada spec/SDLC. Metode (Kiro / ScDD / bugfix) dipilih saat SDLC dibuat (CLAUDE.md §4a rule 0).

| # | Temuan / permintaan | Halaman | Catatan awal |
| --- | --- | --- | --- |
| 1 | Banyak menu tidak punya tombol **Back** | contoh `/settings/admin/users` (CompanyPortal, :3001) | Perlu inventaris halaman `/settings/*` mana saja yang belum punya Back. |
| 2a | Buat **data dummy** vendor | `/settings/admin/vendors` | Seed data master. |
| 2b | Tambah **filter khusus ROH** | `/settings/admin/vendors` | ROH = `vendors.kind='INTERNAL'` (CLAUDE.md §3). |
| 2c | Hapus tautan **"Lihat Ekspor Saya"** (`/exports`) | `/settings/admin/vendors` | Hanya di halaman ini, atau di semua halaman master data? |
| 3 | Seed **PIC vendor**: 2 PIC vendor-wide + 2 PIC cabang, per vendor | data master `vendor_pics` | PIC cabang = `vendor_pics` dengan cabang; vendor-wide = tanpa cabang. |

## Pertanyaan terbuka (dijawab saat SDLC)
- #1: Back kembali ke halaman sebelumnya (history) atau ke induk menu `/settings`?
- #2a: Dummy apa saja (vendor saja, atau ikut cabang/vault/ATM)? Berapa banyak? Hanya dev, jangan ikut prod?
- #2c: Hapus juga di halaman master data lain yang punya tautan sama?
- #3: Berlaku untuk semua vendor yang ada (termasuk ROH)? Peran PIC (`roles`) default ketiganya? Ada `is_director`?
- #2a + #3: seed = data migration / file seed dev → Sec 4 #7 (migrasi data) → butuh gate penuh, bukan bugfix.
