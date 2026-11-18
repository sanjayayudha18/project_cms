# Review: Perbaikan menu Pengaturan

> Status: draft

Tahap 5 Deploy. Reviewer: Claude (self-review, 2026-11-17; **bukan pengganti gerbang code owner** — merge tetap keputusan manusia).
Input: diff kerja di `dev1` (belum di-commit) terhadap `requirements.md`, `design.md`, `tasks.md` (semua accepted 2026-11-17).
**Cek browser manual: OUTSTANDING — tugas user** (Golden Rule #10, task 9).

## Ringkasan

Tidak ada temuan **Important** yang tersisa. Celah test R1 (integration test `kind` skip) ditutup: test dijalankan ke DB dev dan lulus. Sisanya nit dan catatan untuk code owner, plus cek manual user (R7).

## Review Passes

**Bug / korektness**
- `GET /api/v1/admin/vendors`: `kind` divalidasi allowlist (`FLM_VENDOR`/`INTERNAL`) sebelum service dipanggil; kosong → `nil` → filter mati. Filter yang sama dipasang di `ListVendorsAdmin` **dan** `CountVendorsAdmin`, jadi `total`/paging konsisten (Property 2).
- Frontend: `kind` di URL divalidasi `z.enum`; nilai tidak dikenal dibuang di `parseParams`. Mengubah `kind` me-reset `page` ke 1 (`setParams`), sama dengan `pks_status`.
- Ekspor PKS (`AdminVendorsPage.requestPksExport`) tidak membawa `kind` — sesuai Q9. Konsekuensi: dengan filter ROH aktif, isi ekspor ≠ isi layar (lihat R4).
- `BackLink` memakai `<Link to>` (bukan `history.back`), jadi aman saat halaman dibuka langsung dari URL. Test 2.3 memastikan tepat satu tautan di 12 halaman dan tidak ada di `SettingsHubPage`.
- `ReportExportNotice`: prop `showMyExportsLink` default `true`, jadi DSR telat/Replenishment tidak berubah; pesan error tetap tampil saat `false`.

**Keamanan**
- Query berparameter (sqlc `narg`); input `kind` tidak pernah masuk SQL sebagai teks bebas.
- Route group, `RequireRoles`, dan RBAC service tidak berubah; tidak ada endpoint baru, tidak ada write baru dari layar.
- Seed dev ditulis langsung tanpa `audit_logs` (data dev, bukan aksi user — design). Email PIC `@pic.dev.local`, `is_notification_recipient = false`, jadi tidak ada email keluar. Lihat R2 untuk risiko salah target DB.

**Kepatuhan spec / CLAUDE.md**
- Tanpa migrasi, tanpa perubahan `backend-exq`, tanpa dependency baru.
- Respons datar sama dengan handler tetangga; error 400 memakai `writeError(..., "bad_request", ...)` yang sudah ada.
- `/admin/vendors` belum ada di `backend/api/openapi.yaml`, jadi contract test tidak berlaku (design DoD).
- Trace: Property 1 + 2 → 3.3; Property 3 → 2.3; Property 4 → 6.3 (`tasks.md` → Notes).

## Findings

| # | Tingkat | Temuan | Status |
|---|---|---|---|
| R1 | Important (cakupan test) | Kasus `kind` di `TestVendorAdminListPks_Property_Integration` (`backend/internal/service/vendor_admin_list_pks_integration_test.go`) belum pernah jalan (skip, `DATABASE_URL` kosong). Filter SQL hanya terbukti lewat unit test handler (fake service) + cek predikat SQL manual di 6.3. | **Ditutup 2026-11-17** — `go test -tags integration ./internal/service -run TestVendorAdminListPks_Property_Integration -v -count=1` ke DB dev (`localhost`, satu tx yang di-rollback, tanpa data tersisa): PASS, rapid 100 kasus, termasuk partisi `kind` FLM_VENDOR/INTERNAL pada List + Count. |
| R2 | Nit (operasional) | `seed_settings_review.sql` tidak punya guard terhadap DB non-dev. Bila salah dijalankan di DB lain, Bagian 2 menambah PIC seed ke **semua** vendor eligible (di dev: 868 baris). Header sudah "DEV ONLY", sama dengan `seed_vendor_users.sql`. | Dibiarkan; ikut pola seed dev yang ada. |
| R3 | Nit (duplikasi) | `VendorKind` didefinisikan dua kali: `types.ts` dan `useAdminVendorsUrlState.ts` (dari `VENDOR_KINDS`). Nilainya sama hari ini; bila satu berubah, keduanya bisa menyimpang. | Dibiarkan; bisa diganti `import type { VendorKind } from "./types"` di commit berikut. |
| R4 | Catatan perilaku | Ekspor PKS mengabaikan filter Jenis vendor (Q9, diputuskan PO). User yang memfilter ROH lalu ekspor akan mendapat semua jenis. | Didokumentasikan; sesuai keputusan. |
| R5 | Nit (UI) | Kelas `focus-visible:border-[var(--red-400)]` di `BackLink` tidak berefek karena tidak ada `border` width; fokus tetap terlihat lewat `ring-2`. | Dibiarkan. |
| R6 | Catatan (diff) | `graphify-out/*` berubah besar (±46k baris `graph.json`) dari `graphify update .` di task 8. | Saran: commit terpisah dari commit fitur agar diff fitur mudah direview. |
| R7 | Outstanding (user) | Cek API `kind=INTERNAL` lewat backend yang berjalan dan seluruh cek browser task 9. | **Terbuka** — tugas user. |

Diperbaiki selama gate (task 7, sudah hijau): import relatif lintas-feature di `backLinkPlacement.test.tsx` → alias `@/features/...`; 11 error format Biome.

## Definition of Done (CLAUDE.md §11)

- [x] Sesuai requirement + peta modul/tabel (tidak ada tabel/kolom baru)
- [x] Jalur auth benar, RBAC tidak berubah (route + service)
- [x] Maker-checker + audit: tidak ada write baru dari layar; seed dev tanpa audit sesuai design
- [x] Baca di replica / tulis di primary: jalur `List`/`Count` yang ada tidak diubah
- [x] Uang numeric / timestamp timestamptz: tidak relevan (tidak ada kolom baru)
- [x] Test lulus incl. auth/RBAC: unit + frontend hijau (2007 test); integration `kind` lulus (R1, 2026-11-17)
- [x] Tidak ada secret/config hardcoded; tidak ada env baru
- [x] Bentuk respons datar sama dengan tetangga
- [x] OpenAPI: tidak berlaku (`/admin/vendors` belum di spec)
- [ ] Build di Docker: tidak dijalankan dalam review ini (Go build + `pnpm build` hijau di task 7)
