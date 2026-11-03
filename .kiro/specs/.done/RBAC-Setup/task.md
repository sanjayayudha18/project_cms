# Task — RBAC Hierarki & Multi-Layer Approval (Internal)

> Spec acuan implementasi. Scope: **internal CIMB dulu** (vendor menyusul).
> Basis: plan yang sudah disetujui + temuan riset codebase. Semua migrasi baru mulai `021`.
> Aturan proyek: tabel/kolom baru diusulkan di `.kiro/steering/project-context.md` Sec 2 dulu; sentuhan auth/maker-checker = STOP-and-confirm.

---

## Problem Statement

CMS punya RBAC role-string yang berjalan (`RequireAuth`/`RequireRoles`, 9 role internal), tapi belum bisa memodelkan struktur organisasi: satu supervisor punya beberapa anak buah, hierarki berlapis, dan approval maker-checker yang naik bertingkat sesuai nilai/jenis transaksi. Modul `approval`, tabel `approval_requests`, dan `audit_logs` belum ada.

## Requirements (dari klarifikasi user)

- Hierarki dipakai untuk **approval routing** maker-checker.
- Berbasis **level approval eksplisit** yang naik bertingkat.
- Role = kapabilitas, hierarki = siapa-lapor-ke-siapa (**dua konsep terpisah**).
- **Level** = kolom `approval_level` pada user, independen dari role.
- **Berapa tinggi naik** ditentukan **threshold jenis/nilai transaksi** (D-3 hierarchy).
- **Approver** = atasan langsung dalam garis hierarki (`supervisor_id`), naik bertingkat.
- **Fallback block leave** = delegasi eksplisit (`approval_delegations`).
- **Ketidaktersediaan** atasan dari status cuti eksplisit (`user_leaves`).
- Threshold di tabel konfigurasi **`approval_policies`** yang bisa diubah admin.

## Background (temuan riset — WAJIB dipatuhi)

- Backend ATM pakai layout **technical-layer**: `internal/{handler,service,repository,db}` + **sqlc** (`queries/*.sql` -> `internal/db`). Ikuti layout AKTUAL, bukan feature-folder di steering.
- `users` sudah punya `role_id, vendor_id, vendor_branch_id, is_karyawan, auth_source, is_active`. **Belum ada** `supervisor_id` & `approval_level`.
- RBAC keys ke `roles.role` (string). `AuthContext` (`pkg/middleware/rbac.go`) bawa `UserID, Username, Role, IsKaryawan, VendorID` — **belum** bawa level/supervisor.
- **Greenfield**: tidak ada `approval_requests`, `audit_logs`, `internal/audit`. Satu-satunya audit existing = `retry_audit_logs` (domain-specific, `012`).
- Nama `audit_logs` & `approval_requests` **sudah tercantum** sebagai canonical Core tables di project-context Sec 2 (pre-approved). Yang perlu diusulkan: `supervisor_id`, `approval_level`, `approval_policies`, `approval_steps`, `approval_delegations`, `user_leaves`.
- Migrasi additive acuan: `016_users_vendor_branch_id.sql` (BEGIN; ADD COLUMN IF NOT EXISTS; COMMENT; ADD CONSTRAINT FK; CREATE INDEX IF NOT EXISTS; COMMIT; + header WHY/SAFETY). Migrasi tertinggi saat ini `020`.
- Read/replica belum ter-wire (single `dbPool` + TODO `ponytail`). Pakai `dbPool` untuk read & write, tinggalkan TODO.
- sqlc config: `backend/sqlc.yaml` (`sql_package: pgx/v5`, `emit_pointers_for_null_types`, `emit_empty_slices`, `emit_json_tags`). Pola: SQL di `queries/*.sql` -> `sqlc generate` -> panggil `*db.Queries`; repository map row -> domain struct (lihat `auth_repository.go`).
- Route wiring di `cmd/api/main.go`: `r.With(RequireAuth, RequireRoles(...)).Mount("/api/v1/<x>", handler.Routes())`. Global mw: RequestID, RealIP, Logger, Recoverer.
- Error helpers ada di `internal/handler/error_response.go` (`writeJSON/writeError/writeForbidden/...`). `ValidationError` di `internal/service/atm_portal.go`.

## Proposed Solution (3 lapisan)

1. **Struktur hierarki** — `users.supervisor_id` (self-FK, pohon murni) + `users.approval_level int`. Chain naik dibaca rekursif via `supervisor_id`.
2. **Kebijakan approval** — `approval_policies` memetakan `(document_type, rentang nilai) -> required_level`.
3. **Runtime approval** — `approval_requests` (satu per dokumen) + `approval_steps` (jejak per tingkat) + `approval_delegations` + `user_leaves`. Semua transisi tulis `audit_logs`.

**Prinsip non-negosiable:** maker != checker (approver step != maker); tiap transisi tulis `audit_logs` (who/what/before/after/ip); write -> primary; idempotent per `(document_type, document_id)`.

**Alur:** Maker submit -> lookup `approval_policies` (amount+type -> required_level) -> buat request `pending` + generate steps L2..required_level dari chain supervisor -> tiap step: jika approver cuti cari delegate aktif -> approve naik ke step berikut / reject stop -> step terakhir approved => efek dokumen berlaku.

---

## Task Breakdown

### [ ] Task 1 — Usulkan skema di project-context.md + migrasi hierarki user
- Update `.kiro/steering/project-context.md` Sec 2: tandai kolom/tabel baru sebagai usulan.
- `backend/migrations/021_users_hierarchy.sql`: tambah `users.supervisor_id bigint` (self-FK, nullable) + `users.approval_level int` (nullable), gaya `016`. CHECK `supervisor_id <> id`. Index pada `supervisor_id`.
- **Test:** migrasi up/down bersih; insert menolak `supervisor_id = id`; FK & index ada.
- **Demo:** `\d users` menampilkan 2 kolom + FK; query `WITH RECURSIVE` menampilkan chain atasan.
- **Model:** Opus — migrasi additive pada tabel `users` (auth) + self-FK hierarki; sentuhan auth = danger zone, salah constraint mahal di-rollback.

### [ ] Task 2 — Migrasi `approval_policies` + seed contoh
- `022_approval_policies.sql`: `id, document_type, min_amount numeric(20,2), max_amount numeric(20,2), required_level int, is_active bool, timestamps`. Unik/anti-overlap per `(document_type, rentang)`; constraint `min_amount < max_amount`.
- Seed contoh: invoice `< 100jt -> L2`, `>= 100jt -> L3`.
- **Test:** up/down; lookup `(document_type, amount)` deterministik satu baris; overlap ditolak.
- **Demo:** query dua nilai berbeda -> required_level berbeda.
- **Model:** Opus — policy nilai uang -> required_level dengan anti-overlap; kesalahan di sini salah-rutekan approval maker-checker.

### [ ] Task 3 — Migrasi `audit_logs` + `internal/audit` writer
- `023_audit_logs.sql` (append-only): `id, actor_id, action, entity_type, entity_id, before jsonb, after jsonb, ip text, created_at`.
- `queries/audit.sql` + repository/service `internal/audit` `Write(ctx, entry)`; actor & ip dari `middleware.GetAuthContext` + RealIP.
- **Test:** map field benar; before/after JSON tersimpan; tak ada path update/delete.
- **Demo:** panggil writer dari harness, tampilkan baris audit lengkap.
- **Model:** Sonnet — tabel append-only + writer service mengikuti pola sqlc/repository yang ada; terkontrol dan ter-test.

### [ ] Task 4 — Migrasi runtime approval
- `024_approval_runtime.sql`:
  - `approval_requests` (`maker_id, document_type, document_id, amount numeric(20,2), required_level, status` enum `draft|pending|approved|rejected`, unik `(document_type, document_id)`).
  - `approval_steps` (`request_id, step_level, assigned_approver_id, acted_by_id, status, acted_at`).
  - `approval_delegations` (`from_user_id, to_user_id, start_at, end_at, reason`).
  - `user_leaves` (`user_id, start_at, end_at, reason`).
- **Test:** up/down; FK & unik idempotency; enum status tervalidasi.
- **Demo:** insert manual request + steps, tampilkan struktur runtime.
- **Model:** Opus — skema inti runtime maker-checker (requests/steps/delegations/leaves) dengan uniqueness idempotency; fondasi money-approval, mahal bila salah.

### [ ] Task 5 — Chain resolver + effective-approver resolver (service, no HTTP)
- `queries/approval.sql` + `internal/service`: (a) resolver chain atasan bertingkat dari `supervisor_id`/`approval_level` sampai `required_level`; (b) resolver "approver efektif" — approver cuti (`user_leaves` overlap now) -> delegate aktif (`approval_delegations` overlap now); guard maker != checker.
- **Test (TDD table-driven):** chain berhenti di required_level; approver cuti -> delegate; tak ada delegate -> error jelas; delegate = maker -> ditolak.
- **Demo:** unit test skenario normal, cuti+delegasi, tanpa delegasi.
- **Model:** Opus — resolver hierarki bertingkat + effective-approver (cuti/delegasi) + guard maker != checker; logika inti yang salahnya melanggar four-eyes.

### [ ] Task 6 — Approval orchestrator (submit/approve/reject) + idempotency + audit
- Service `SubmitForApproval`, `Approve`, `Reject`. Submit: lookup policy -> required_level -> request + steps via resolver -> idempotent per `(type, id)`. Approve: validasi approver step aktif = actor (atau delegate sah), maker != checker, majukan step / approved di step terakhir. Reject: stop. Tiap transisi tulis `audit_logs`. Write ke primary.
- **Test (TDD):** submit ganda tak duplikat; approve berjenjang sampai approved; maker approve ditolak; reject stop; audit tiap transisi.
- **Demo:** unit test end-to-end submit invoice 150jt -> 2 tingkat -> approved + trace audit.
- **Model:** Opus — orchestrator submit/approve/reject + idempotency + audit tiap transisi; jantung maker-checker, correctness uang/approval kritikal.

### [ ] Task 7 — HTTP handler + wiring RBAC
- `internal/handler/approval_handler.go` `Routes()`: `POST /submit`, `POST /{id}/approve`, `POST /{id}/reject`, `GET /{id}`, `GET /inbox`. Mount di `cmd/api/main.go` via `r.With(RequireAuth, RequireRoles(...supervisor roles...))`. Actor/ip dari `GetAuthContext` + RealIP.
- **Test (httptest):** happy path submit/approve/reject/inbox; 401 tanpa token; 403 role tak berhak; 409 double-submit.
- **Demo:** submit -> approver approve via inbox -> approved; non-approver 403.
- **Model:** Sonnet — HTTP handler + RBAC wiring di atas orchestrator yang sudah ada; pola handler standar, ter-cover httptest.

### [ ] Task 8 — Admin endpoints (hierarki/delegasi/cuti) + threading level ke JWT/AuthContext
- Endpoint admin set `supervisor_id`/`approval_level`, buat/cabut `approval_delegations`, catat `user_leaves` (tulis audit, `RequireRoles ADMIN/ADMIN_PARAM`).
- Tambah `approval_level` (+ opsional `supervisor_id`) ke JWT claims & `AuthContext`; update token service + tests 9 role.
- **Test:** admin-only enforced; delegasi rentang tumpang-tindih ditolak; klaim JWT bawa level; regression token existing hijau.
- **Demo:** admin set hierarki 3 tingkat + delegasi, jalankan alur approval Task 6/7 yang mengikuti delegasi saat atasan cuti.
- **Model:** Opus — mengubah JWT claims + AuthContext dan endpoint admin hierarki/delegasi; sentuhan auth, regresi token berisiko luas.

### [ ] Task 9 — Quality gate + dokumentasi
- `sqlc generate`, `go test ./...`, `golangci-lint run`, `go build`.
- Update project-context.md (tandai proposed -> ada), `.env.example` bila perlu, catatan pola approval untuk modul lain (invoice/dsr) memanggil orchestrator.
- **Test:** seluruh suite hijau; coverage `internal/*` yang disentuh >= 80%.
- **Demo:** ringkasan cara modul lain memakai `SubmitForApproval`.
- **Model:** Sonnet — quality gate (sqlc/test/lint/build) + dokumentasi; butuh judgment triase, bukan reasoning tingkat Opus.

---

## Tradeoffs & Constraints

- Model hierarki = **pohon murni** (`supervisor_id`); matriks multi-atasan = perubahan tabel terpisah bila nanti dibutuhkan.
- `approval_policies` fokus jenis+nilai; dimensi lain (region/vendor) bisa ditambah kolom kondisi kemudian.
- **Vendor-side hierarchy di luar scope** sekarang (internal dulu).
- Sentuhan auth/maker-checker = STOP-and-confirm; usulkan tabel/kolom di project-context Sec 2 sebelum migrasi.
- Read/replica belum ter-wire: pakai `dbPool` untuk read & write, tinggalkan TODO gaya `ponytail`.
- Ikuti layout technical-layer aktual (handler/service/repository + sqlc). Migrasi mulai `021`.

## Definition of Done (Sec 11 project-context)

- [ ] Sesuai module/table map (Sec 2) — kolom/tabel diusulkan dulu
- [ ] Auth path & RBAC ter-scope benar
- [ ] Maker-checker + audit_log ter-wire (maker != checker)
- [ ] Read replica untuk read, primary untuk write (TODO wiring didokumentasikan)
- [ ] Money numeric, timestamps timestamptz
- [ ] Tests hijau incl. auth/RBAC/maker-checker cases
- [ ] Tidak ada secret hardcoded; `.env.example` diperbarui bila perlu
- [ ] ATM backend tetap flat JSON shape
- [ ] Build bersih (`go build`, `sqlc generate`)
