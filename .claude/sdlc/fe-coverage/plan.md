# Plan: Coverage frontend naik ke 80% (FE-13)

Status: **accepted 2026-10-15** (engineer/PO: "Terima kedua plan.md. P1: ikut rename ke formatDecimalThousands."). Stage: 3 Build. Stage berikut: `tests.md` (diisi per slice).
Sumber: `spec.md` (accepted 2026-10-15; Q1 layar prototipe tetap dihitung, Q2 exclude non-kode saja, Q3 threshold pindah dari `fe-tech-debt` E2 ke C0, Q4 target 80 % keempat metrik).

## Pola test (dipakai ulang, tanpa dependency baru)
Referensi: `CompanyPortal-Vite/src/features/admin-atms/`.
- **`api.ts`** → `api.test.ts`: `vi.mock("@/lib/api/client", …)` lalu assert path, query string, method, dan body per fungsi (contoh `admin-atms/api.test.ts`).
- **`hooks.ts`** → `__tests__/hooksBehavior.test.tsx`: hook asli + `QueryClient` asli, `../api` di-mock lewat `vi.hoisted`; cek data, error, invalidasi setelah mutasi (contoh `admin-atms/__tests__/hooksBehavior.test.tsx`).
- **Halaman/dialog** → Testing Library: render, isi form, submit, error server (422 field, 409), status maker-checker (202 → toast "menunggu persetujuan"), tombol yang disembunyikan per role. Router pakai helper yang sudah ada (`src/test/` di CP, `src/test/routerTestUtils.tsx` di VP).
- VP auth/client: `fetch` di-mock per test (pola `features/auth/__tests__/AuthContext.test.tsx`).

Angka "belum tercover" di bawah = statement, dari baseline 2026-10-15 (`include: src/**`).

## Slice C0 — Config + threshold baseline
- `frontend/CompanyPortal-Vite/vite.config.ts` dan `frontend/VendorPortal-Vite/vite.config.ts`: tambah `test.coverage`:
  - `provider: "v8"`, `include: ["src/**"]`
  - `exclude: ["src/**/*.{test,spec}.{ts,tsx}", "src/**/__tests__/**", "src/test/**", "src/test-setup.ts", "src/main.tsx", "src/**/*.d.ts", "src/data/**"]`
  - `thresholds` = floor dari hasil ukur setelah `exclude` (perkiraan CP ~61/62/54/61, VP ~64/55/59/64).
- `frontend/VendorPortal-Vite/package.json` + `pnpm-lock.yaml`: `@vitest/coverage-v8 ^4.1.10` (sudah terpasang 2026-10-15, ikut commit C0).
- `.claude/sdlc/fe-tech-debt/plan.md` E2: catat bahwa bagian `vite.config.ts` pindah ke `fe-coverage` C0. E2 tinggal menjalankan `--coverage` di CI.
- Bukti: `pnpm test -- --coverage` lulus di kedua portal. Tes negatif sekali (lokal, tidak di-commit): naikkan satu threshold +5 → run gagal.

## Slice C1 — Auth + API client
| File sumber | Belum | Test |
|---|---|---|
| VP `features/auth/AuthContext.tsx` | 46 | perluas `features/auth/__tests__/AuthContext.test.tsx`: login 401/403 (`account_inactive`, `portal_mismatch`, lainnya)/422/429 (`Retry-After`)/503/default/network; refresh sukses, non-`VENDOR-USER`, gagal, network; logout clears + `POST /logout` gagal diabaikan; init dengan role non-vendor |
| VP `features/auth/LoginPage.tsx` | 22 | `features/auth/__tests__/LoginPage.test.tsx` (baru): pesan error, countdown rate-limit, submit disabled |
| VP `features/auth/useAuth.ts` | 10 | `__tests__/useAuth.test.tsx` (baru): throw di luar provider |
| VP `lib/api/client.ts` | 23 | `lib/api/__tests__/client.test.ts` (baru): Bearer, `skipAuth`, FormData tanpa Content-Type, 401 → refresh → retry sekali, 401 paralel = satu refresh, refresh gagal → `onAuthFailure` + throw, tanpa `configureApiAuth` → `onAuthFailure`, 204, error body `message`/`details`, body non-JSON |
| CP `lib/auth/store.ts` | 17 | perluas `lib/auth/store.test.ts`: cabang yang belum (403 lainnya, 422 tanpa detail, default, refresh catch, init catch) |
| CP `lib/api/client.ts` | 3 | perluas `lib/api/client.test.ts`: cabang `errors` di `parseApiError` |

Test hanya membaca perilaku yang ada. Catatan: test StrictMode `/refresh` 1× **bukan** di sini, karena itu RED test `fe-auth-unify` FR-1. Kalau `fe-auth-unify` dikerjakan lebih dulu, C1 tinggal menambah sisanya.

## Slice C2 — Maker-checker + admin (CP)
| File | Belum | Test |
|---|---|---|
| `features/approvals/{api,hooks}.ts`, `components/ApprovalInboxPage.tsx` | 31 | `approvals/api.test.ts`, `__tests__/hooksBehavior.test.tsx`, `__tests__/ApprovalInboxPage.test.tsx` (list, approve/reject, maker = checker tidak bisa approve, error 409) |
| `features/role-management/{api.ts, components/RoleManagementPage.tsx, CreateRoleDialog.tsx, PermissionEditor.tsx, hooks/useRoleQueries.ts}` | 40 | `api.test.ts`, `__tests__/RoleManagementPage.test.tsx`, perluas dialog/editor (cabang validasi) |
| `features/admin-users/{api,hooks}.ts`, `UserFormDialog`, `AdminUsersPage`, `UserFilterBar`, `UsersTable` | 85 | `api.test.ts`, `__tests__/hooksBehavior.test.tsx`, perluas page/dialog: LDAP vs local (temp password), disable/enable |
| `features/admin-holidays/{api.ts, AdminHolidaysPage.tsx}` | 34 | `api.test.ts`, perluas page (tanggal duplikat 409) |
| `features/admin-regions/*` (page, url state, api, hooks, dialog, filter bar, table) | 111 | `api.test.ts`, `__tests__/hooksBehavior.test.tsx`, `__tests__/AdminRegionsPage.test.tsx`, `useAdminRegionsUrlState.test.ts`; disable diblokir saat region dipakai (409) |
| `features/admin-acm-areas/{api.ts, AcmAreaDetailPanel, AdminAcmAreasPage}` | 36 | `api.test.ts`, perluas panel/page |

## Slice C3 — Form master-data (CP)
| File | Belum | Test |
|---|---|---|
| `features/admin-vendors/api.ts` | 105 | `api.test.ts` (semua endpoint vendor, branch, vault, PIC, package, price, DSR map) |
| `features/admin-vendors/hooks.ts` | 87 | `__tests__/hooksBehavior.test.tsx` |
| `PackagesPanel`, `VendorPackageFormDialog`, `VendorPackagePriceFormDialog`, `VendorBranchEditPage` (0–3 %) | 138 | test komponen baru per file: submit → 202 menunggu persetujuan, 422 per field, harga decimal-string tidak diubah jadi float |
| `PicsPanel`, `AdminVendorsPage`, `VendorVaultFormDialog`, `VendorPicFormDialog`, `VaultsPanel`, `VendorFormDialog`, `BranchesPanel`, `VendorBranchFormDialog`, `DsrLocationMapsPanel`, `PackagePricesPanel` | 177 | perluas test yang ada: disable/enable, kategori vault, `region_code`, validasi |
| `dsrLocationMaps.ts`, `lib/*FormSchema.ts` | 26 | test unit schema (Zod): valid/invalid per field |
| `features/master-data-io/{api.ts, MasterDataIOPage.tsx}` | 38 | `api.test.ts` (FormData upload, blob download), perluas page |
| `features/admin-atms/*` sisa | 42 | perluas test yang ada (cabang assignment, tabel) |

## Slice C4 — Uang / kuota / order
| File | Belum | Test |
|---|---|---|
| CP `features/vendor-request/{api,hooks,vendorSide}.ts` | 90 | `api.test.ts`, `__tests__/hooksBehavior.test.tsx`, `vendorSide.test.ts` |
| CP `VendorRequestDetail`, `VendorRequestCreate`, `VendorRequestList`, `ForecastBrowser`, `ForecastSummary`, url state, `CompletionPanels` | 158 | perluas test yang ada: transisi status (approve/reject/cancel/vendor-return/complete), jumlah IDR `tabular-nums`, total per denom |
| CP `features/atm-portal/*` (`AtmProfileScreen`, `CashposProfileTable`, `TabNavigation`, `ReplenishTable`, `AtmHeader`, data hooks, url state, `visitQuota.ts`, `lib/formatters.ts`, `FilterBar`, `VisitQuotaCard`) | 249 | test baru untuk 7 file 0 %, `visitQuota.test.ts` (sisa negatif = over quota), perluas url state |
| CP `features/replenishment/api.ts`, `ReplenishmentScreen` | 35 | `api.test.ts`, perluas screen |
| CP `features/vault-plan/{api.ts, VaultPlanDetail, VaultPlanList, RequestVaultPanel, allocation.ts}` | 48 | `api.test.ts`, perluas (kelipatan denom, buffer reason) |
| VP `features/dsr/*` (`DsrDetailDialog` 0 %, `DsrPage`, `DsrUploadDialog`, `dsrUploadApi`) | 46 | test baru `DsrDetailDialog`, perluas page/dialog/api |
| VP `features/orders/*`, `features/realizations/RealizationsPage`, `components/ui/{FileUpload,DatePicker}`, `features/notifications/NotificationsPage` | 61 | perluas test yang ada; `NotificationsPage` baru (mark read) |

## Slice C5 — Sisa sampai ≥ 80 %
- CP: `eod-monitoring` (145, 11 file 0 %), `routes` (159: guard per route, redirect `/login`, 403), `dashboard`, `components/layout`, `lib/utils/deriveStatus.ts`, `components/ui/ProgressBar`, `components/pages/Forbidden`, layar prototipe (`dmaa-forecast`, `forecast`, `dsr`, `cit` data hooks).
- VP: `schedule`, `evidence`, `invoices` (prototipe), `App.tsx`, `lib/utils/cn.ts`, `components/layout`.
- Ukur ulang dulu sebelum C5 dimulai, lalu ambil file dengan statement belum tercover terbanyak sampai keempat metrik ≥ 80.

## Langkah per slice (sama setiap slice)
1. Tulis test sesuai tabel. Bila test menemukan bug: berhenti, laporkan ke PO, catat via `cms2-bugfix` (jangan diperbaiki di slice ini).
2. Kedua portal: `pnpm test -- --coverage`, `pnpm lint`, `pnpm build` hijau.
3. Naikkan `thresholds` ke floor(angka terukur). Tidak boleh turun.
4. `tests.md`: angka sebelum/sesudah per metrik + daftar test baru → FR.
5. `FE_debt.md` FE-13 status `in-progress` + angka; baris tech-debt frontend di `development-progress.md`; `graphify update .`.
6. Satu commit per slice, hanya bila user bilang "commit".

## Risiko
- **Durasi test.** C3/C4 menambah banyak test komponen. Ukur `vitest run` per slice; bila > +30 %, catat dan pertimbangkan `pool`/isolasi (keputusan engineer).
- **Functions CP** (Q4) paling lambat naik. Ukur setelah C4; bila jauh di bawah 80, bawa data ke PO.
- **Test rapuh.** Assert teks/role yang terlihat user, bukan class CSS atau struktur DOM.
- **Overlap dengan `fe-auth-unify`.** C1 menyentuh file test yang sama (`AuthContext.test.tsx`). Kerjakan berurutan, jangan paralel.

## Di luar plan
Coverage di Jenkins (FE-06), E2E, refactor demi testability (FE-12), perubahan kode produksi.
