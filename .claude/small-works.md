# Small Work Log

Small changes that are **not bugs** and don't need Kiro, ScDD or UI Revamp (CLAUDE.md Sec 4a, Small Work Development). Newest first. One entry per change.

**Fit check** (all must pass, else Kiro / ScDD / UI Revamp / bugfix):
- One concern, a few files (rule of thumb ≤ 5), one portal or one backend module
- No schema/migration, no new or changed endpoint (`openapi.yaml` unchanged)
- No new business rule, nothing in CLAUDE.md Sec 4 #7 (auth, money, recon, migration, delete)
- Not a bug (→ [bugfixes.md](./bugfixes.md)), not a screen redesign (→ UI Revamp)

Examples: label/copy change, add/hide/reorder a UI element, small tweak to existing behaviour, a default value, docs/tooling/config.

**Flow**: AI writes the entry (Request + Fit check + AC) → **user OKs the AC** (the one gate) → code + tests → fill Change/Tests → review → commit on the user's "commit".

Template:

```markdown
## YYYY-MM-DD — <short title>
- **Request**: what the user asked, in their words
- **Fit check**: pass (or which point fails → other method)
- **AC**: 1. … 2. …  (accepted YYYY-MM-DD by user)
- **Change**: files touched
- **Tests**: automated check added/run; manual browser verification: outstanding (user)
- **Commit**: <sha>
```

---

## 2026-11-20 — Tambah Pengguna jadi halaman sendiri
- **Request**: "saya ingin tambah pengguna punya form halaman sendiri tidak digabung di manajemen pengguna" — hanya Tambah; Edit tetap dialog. Skill UI: `ui-ux-pro-max`.
- **Fit check**: pass — CompanyPortal saja, endpoint sama (`POST /api/v1/admin/users`, `openapi.yaml` tidak berubah), tanpa schema, tanpa aturan bisnis baru, bukan Sec 4 #7 (guard `APPACCESS` tetap sama), bukan redesign layar.
- **AC**: (accepted 2026-11-20 by user)
  1. Route baru `/settings/admin/users/new`, guard sama dengan daftar (`APPACCESS`; tanpa role → Forbidden).
  2. Tombol "Tambah Pengguna" di Manajemen Pengguna dan kartu "Tambah Pengguna" di Settings hub membuka halaman itu (bukan dialog). Parameter `?create=true` dihapus.
  3. Halaman memuat field, validasi dan pesan error server yang sama dengan dialog sekarang (form dipakai bersama, bukan disalin); ada BackLink "Kembali ke Manajemen Pengguna" + tombol Batal.
  4. Simpan sukses → toast "Pengguna berhasil dibuat" lalu kembali ke `/settings/admin/users`; gagal → tetap di halaman dengan error per field.
  5. Edit pengguna tetap dialog, perilakunya tidak berubah.
- **Change**: CompanyPortal `features/admin-users/` — `UserForm.tsx` baru (isi form + submit, dipindah dari dialog; `Field` kini `<label>` yang membungkus input, `autoComplete` untuk email/password sementara), `UserFormDialog.tsx` jadi pembungkus tipis (dipakai untuk edit), `CreateUserPage.tsx` baru, `AdminUsersPage.tsx` (tombol → navigate, state `?create` dihapus), `useAdminUsersUrlState.ts` (param `create` dihapus), `index.ts`; `routes/settings/admin/users-new.tsx` baru + `routeTree.ts`; `rbac-settings/components/SettingsHubPage.tsx` (href kartu → `/settings/admin/users/new`).
- **Tests**: `CreateUserPage.test.tsx` baru (render halaman + back link, simpan → toast + kembali ke daftar, error field server → tetap di halaman, Batal); `AdminUsersPage.test.tsx` (tombol → navigate ke `/new`, tes `?create` dihapus); `SettingsHubPage.adminUsersCard.test.tsx` (href baru). `vitest` admin-users + rbac-settings + routes + lib/config: 38 file / 327 test hijau; `biome check` bersih; `tsc --noEmit` bersih; `pnpm build` sukses. Guard route `/new` memakai `requireRoles(["APPACCESS"])` yang sama dengan daftar (tes guard yang ada sudah mencakup daftar role itu). Manual browser verification: outstanding (user)
- **Commit**: —

---
