# Development Progress — CMS ATM & CIT

> Last updated: 2026-09-18 · branch `dev1` · last commit `6a8e3f6 CRUD RBAC v1 done`
> Update file ini setiap kali spec selesai / fitur naik status. Status diambil dari **kode yang benar-benar ter-mount**, bukan dari folder `specs/done/`.

**Legenda status**
- ✅ **Live** — backend endpoint ter-mount + frontend memanggil API sungguhan
- 🟡 **Partial** — sebagian jalan (mis. backend ada, UI belum lengkap, atau task spec masih terbuka)
- 🎨 **UI prototype** — layar ada, data masih mock / belum ada endpoint
- ⬜ **Not started** — belum ada kode

---

## 1. Ringkasan per requirement URS (lihat `.claude/feature-flows/`)

| # | Fitur URS | Status | Yang sudah ada | Gap utama |
|---|---|---|---|---|
| 01 | Master data (vendor, vault, PIC, ATM, kelolaan) | ✅ (scope D3: vendor → ATM) | API admin vendor/cabang/vault/PIC/paket/ATM/kelolaan ATM, semua lewat maker-checker (202 → approval → applier); ekspor/template/impor CSV (dry-run, 1 batch = 1 approval, apply all-or-nothing); FE: detail vendor (tab), dialog kelolaan ATM, inbox persetujuan, badge menunggu approval, halaman ekspor/impor | Kelolaan branch/customer ditunda (D3); XLSX/PDF ditunda (D2); `vendor_packages` belum bisa diimpor; form tambah/ubah anak vendor di UI belum ada (lewat CSV/API); inbox belum menampilkan nama maker |
| 02 | Upload DSR harian & rekap keterlambatan | 🟡 | Vendor upload DSR (upload → confirm), list upload, detail daily & rencana isi | Status OK/TELAT (deadline 09:00), notifikasi telat, rekap bulanan penalty |
| 03 | ATM Cash Forecasting (Order ATM) | 🟡 | DMAA forecast viewer; Request Replenish ke vendor (draft → submit → approve/reject/revise/cancel, nomor request, audit log) | Upload data tambahan (complaint, project, problem ATM, adjustment), formula Order ATM penuh, approval berjenjang |
| 04 | Pemenuhan & pengambilan dana, surat tugas, serah terima | ⬜ | — | Semua |
| 05 | Rekonsiliasi invoice vendor | 🎨 | Layar invoice (internal + vendor portal) mock | Backend `internal/invoice` belum ada |
| 06 | Validasi realisasi replenishment & report | 🎨 | ATM portal: cashpos & riwayat replenish per ATM (read-only) | Klasifikasi 5 kategori, 8 report, export |
| 07 | Dashboard | 🎨 | Layar dashboard (mock, memanggil `/api/v1/dashboard/*` yang belum ada) | Endpoint dashboard, data dari replica |
| 08 | Cash count vault | ⬜ | — | Semua (tabel masih proposal) |
| 09 | Cash count selektif mesin | ⬜ | — | Semua |

---

## 2. Detail fitur yang sudah dibangun

### Platform / Core
| Fitur | Status | Backend | Frontend | Spec |
|---|---|---|---|---|
| Login internal & vendor (JWT, refresh, logout, login pakai email) | ✅ | `/api/v1/auth` | Login kedua portal | `done/user-login`, `done/Auth-Local-Lifecycle`, `done/revamp-vendor-loginpage` |
| RBAC + approval engine (hierarki user, delegasi, cuti, policy) | ✅ | `/api/v1/approvals`, `/api/v1/admin/approval/*` | `rbac-settings` | `done/RBAC-Setup`, `done/rbac-settings-menu` |
| Admin CRUD users | ✅ | `/api/v1/admin/users` | `admin-users` | `done/admin-user-vendor-management` |
| Admin CRUD vendors | ✅ | `/api/v1/admin/vendors` | `admin-vendors` | `done/admin-user-vendor-management` |
| Admin CRUD ATM | ✅ | `/api/v1/admin/atms` | `admin-atms` (Settings hub) | `admin-atm-management` (47/47 task) |
| Role management (menu/feature permission) | 🟡 | `/api/v1/admin/roles` (list, catalog, create, set permissions) | `role-management` | `role-management` — 47/52; sisa: task 0 (approval tabel di project-context), beberapa test opsional |
| Audit log viewer | 🟡 | `/api/v1/audit-logs` (list, detail) | `audit-log` (sebagian) | `audit-log-viewer` — 18/39; sisa: FilterBar, Table, DetailDrawer, BeforeAfterDiff, menu sidebar, verifikasi migration |
| Migration baseline | ✅ | `001_baseline_schema.sql` + `002_baseline_seed.sql` | — | CLAUDE.md §12 |

### ATM Operations
| Fitur | Status | Backend | Frontend | Spec |
|---|---|---|---|---|
| ATM portal (list ATM, cashpos, profil ATM, riwayat replenish & cashpos per ATM) | ✅ | `/api/v1/atm-portal` | `atm-portal` | `done/atm-portal`, `done/atm-profile` |
| DMAA forecast viewer | ✅ | `/api/v1/dmaa-forecast` | `dmaa-forecast` | `done/dmaa-forecast-viewer`, `done/update-cit-forecast-browser` |
| DSR upload (vendor portal) | ✅ | `/api/v1/dsr/uploads*` | Vendor `dsr` | `done/vendor-dsr-home` |
| Request replenish ke vendor (maker-checker, revise, cancel, audit) | ✅ | `/api/v1/vendor-requests` | `vendor-request` | `done/request-replenish-to-vendor`, `done/replenishment-request-enhancements`, `done/cit-vendor-request-enhancements` |
| Cash flow monitoring | 🟡 | (belum ada endpoint khusus) | `cash-flow` | `done/cash-flow-monitoring` |
| EOD monitoring | 🎨 | Tidak ada `cmd/batch`; tabel retry scheduler (lama `012`) **tidak** ada di baseline | `eod-monitoring` | `done/eod-monitoring-frontend`, `done/eod-retry-scheduler` |

### CIT
| Fitur | Status | Catatan |
|---|---|---|
| Split `backend-cit` (Go workspace, `pkg/` bersama) | ✅ | `done/backend-cit-split`. Port 8081, baru `/health` + grup route ber-auth |
| Modul CIT (`cit`, `journal`, `dsr`, `reconciliation`, `integration`) | 🎨 | Folder ada di `backend-cit/internal`, belum ada endpoint. Frontend `cit` masih mock |

### Layar UI prototype (mock, belum ada backend)
Internal: `dashboard`, `forecast`, `forecasting`, `dsr` (internal view), `invoice`, `reconciliation`, `replenishment`, `cit`.
Vendor portal: `orders`, `schedule`, `invoices`, `evidence`, `notifications`.

---

## 3. Belum ada sama sekali (modul di CLAUDE.md §3)
`internal/document` · `internal/notification` (email/in-app) · `internal/export` (CSV/XLSX/PDF) · `internal/vault` · `internal/vendorpic` · `internal/assignment` · `internal/forecast` (formula Order ATM) · `internal/cashcount` · `internal/invoice` · `internal/reconciliation` (ATM) · `internal/corebanking` (escrow batch) · `cmd/batch` (EOD).

---

## 4. Catatan teknis / utang
- Backend ATM masih pakai response JSON flat, belum `pkg/response` (disengaja, lihat CLAUDE.md §3).
- Beberapa folder `specs/done/` checkbox-nya belum dicentang (mis. `atm-profile`, `vendor-dsr-home`, `rbac-settings-menu` 0/x) padahal fiturnya live — tasks.md belum diupdate, bukan fitur belum jadi.
- Frontend `dashboard` memanggil `/api/v1/dashboard/metrics` & `/activity` yang belum ada di backend.
- Role management: create/update langsung tanpa maker-checker — deviasi yang disengaja (CLAUDE.md §12).

---

## 5. Log perubahan
| Tanggal | Perubahan |
|---|---|
| 2026-09-18 | Dokumen dibuat dari audit kode (route `backend/cmd/api/main.go`, fitur frontend, `tasks.md` tiap spec). |
