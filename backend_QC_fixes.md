# Backend QC — Daftar Perbaikan

Sumber: [backend_QC_result.md](./backend_QC_result.md) (QC 2026-10-09, commit `42e7b08`).
Cara pakai: kerjakan **satu item per sesi/commit**, urut dari atas. Sebelum mulai setiap item, AI tetap bertanya (CLAUDE.md Sec 4a rule 0): *AI-DLC baru atau cukup dicatat di `bugfixes.md`?* Kolom "Dok." di bawah hanya saran.

Status: `[ ]` belum · `[~]` sedang dikerjakan · `[x]` selesai (isi tanggal + commit)

## ⏸ Ditunda — item kompleks (keputusan user 2026-10-09: selesaikan yang simple dulu)

Jangan lupa: item di bawah **belum selesai** dan tetap wajib dikerjakan. Ambil lagi saat ada waktu / keputusan yang dibutuhkan sudah ada.

| ID | Item | Kenapa ditunda | Yang dibutuhkan untuk mulai | Titik lanjut |
| --- | --- | --- | --- | --- |
| F7 | Uang `float64` → string desimal (3 endpoint + `computeReplenishmentStatus` + 2 frontend) | Money + kontrak API 2 frontend → AI-DLC penuh | PO accept `intent.md` + jawab Q1–Q6 | `.claude/sdlc/money-decimal-string/intent.md` (draft) → `spec.md` |
| F13 | IP client bisa dipalsukan via `X-Forwarded-For` (rate limit login + IP audit) | Auth (Sec 4 #7) | Topologi proxy prod: Nginx / GCP LB / keduanya, berapa hop tepercaya | Bagian F13 di file ini |
| F11 | Setup CI (build, vet, lint, govulncheck, test `-tags integration` + Postgres) | Perlu keputusan platform | GitHub Actions atau Cloud Build? Postgres untuk test integrasi? | Bagian F11 |
| F8 | Coverage `repository` / `handler` / `service` ke ≥ 80% | Besar, bertahap per batch | — (bisa mulai kapan saja, batch kecil) | **Batch 1 selesai 2026-10-09** (applier harga vendor-wide, service 73.0→74.3%). Batch berikut: kandidat di bagian F8 |
| F17 | **Bug**: harga (vendor-wide `vendor_package_prices` + paket cabang `vendor_packages_branch`) yang mulai hari ini/masa depan **tidak bisa di-disable** — `effective_end_date = CURRENT_DATE - 1` melanggar `*_period_chk` (`end >= start`) → apply gagal saat approve | Money/master data + semantik produk (Sec 4 #7) | Keputusan PO: disable harga yang belum berlaku = apa? (mis. `end = start` → berlaku 1 hari; atau tolak di submit dengan pesan jelas; atau status/batal terpisah) | `queries/vendor_package_prices_admin.sql:104`, `queries/vendor_packages_admin.sql:88`; test di `masterdata_applier_price_integration_test.go` |
| F10 | Turunkan kompleksitas fungsi gocyclo > 20 | Refactor logika; butuh F8 dulu agar perilaku terkunci test | F8 untuk fungsi terkait | Bagian F10 (`masterdata_import.go` ikut flow FSD replace-all) |
| F9 | Pecah `vendor_request_actions.go` (1210 baris) | Hindari bentrok dengan pekerjaan Vendor Request yang sedang jalan | Fitur berikutnya yang menyentuh Vendor Request | Bagian F9 (gabung dengan F10 `UpdateItems`/`transition`) |

## Urutan eksekusi

| # | ID | Item | Prio | Ukuran | Dok. (saran) | Status |
| --- | --- | --- | --- | --- | --- | --- |
| 1 | F1 | `gofmt -w` 12 file | LOW | XS | tidak perlu (format saja) | [x] 2026-10-09 |
| 2 | F2 | Tambah `.golangci.yml` + pasang `govulncheck` | LOW | S | `bugfixes.md` | [x] 2026-10-09 |
| 2a | F12 | **Upgrade dependency + toolchain yang kena CVE** (hasil govulncheck F2) | **HIGH** | S–M | `bugfixes.md` | [x] 2026-10-09 |
| 2b | F13 | **IP client bisa dipalsukan via `X-Forwarded-For`** (rate limit login + IP audit) | **HIGH** | S–M | AI-DLC atau bugfix (auth, Sec 4 #7) — butuh info topologi proxy | [ ] |
| 2c | F14 | `go.work` di-gitignore tapi di-`COPY` Dockerfile → build Docker dari clone bersih gagal | MEDIUM | XS | `bugfixes.md` | [x] 2026-10-09 |
| 2d | F15 | Tidak ada `.dockerignore` di root (context build = root repo) + CLAUDE.md menyebut root `docker-compose.yml` yang tidak ada | LOW | XS | `bugfixes.md` | [x] 2026-10-09 |
| 3 | F3 | Bereskan errcheck di kode produksi | LOW | S | `bugfixes.md` | [x] 2026-10-09 |
| 4 | F4 | Guard konversi int → int32 (gosec G115) | LOW | XS | `bugfixes.md` | [x] 2026-10-09 |
| 4a | F16 | `page` tanpa batas atas → `int32(Page)` wrap → offset negatif (500) di ±8 list endpoint | LOW | S | `bugfixes.md` | [x] 2026-10-09 |
| 5 | F5 | Permission file upload DSR 0755/0644 → **diterima** (lihat log) | MEDIUM | XS | `bugfixes.md` | [x] 2026-10-09 |
| 6 | F6 | Repo admin baca dari replica + hapus TODO basi | MEDIUM | M | `bugfixes.md` | [x] 2026-10-09 |
| 7 | F7 | Uang `float64` → string desimal (API + 2 frontend) | HIGH | M–L | **AI-DLC** (money, Sec 4 #7) | [~] intent draft |
| 8 | F8 | Naikkan coverage `repository` / `handler` / `service` ke ≥ 80% | HIGH | L (bertahap) | `bugfixes.md` per batch | [ ] |
| 9 | F9 | Pecah `vendor_request_actions.go` (1210 baris) | MEDIUM | M | ikut fitur berikutnya | [ ] |
| 10 | F10 | Turunkan kompleksitas fungsi gocyclo > 20 | MEDIUM | M | ikut fitur yang menyentuh | [ ] |
| — | F11 | Setup CI (build, vet, lint, test `-tags integration`) | — | M | butuh keputusan user | [ ] |

Tidak dimasukkan (sengaja): duplikasi handler admin (M4) dan `main()` panjang (M5). Keduanya diterima apa adanya; tangani hanya kalau ada perubahan yang harus diterapkan ke semua handler sekaligus. ST1005 di `pkg/auth/errors.go` juga disengaja (pesan ditampilkan ke user); di-exclude lewat F2.

---

## F1 — gofmt
- **File**: `backend/cmd/hashpw/main.go`, `backend/tools.go`, `backend/internal/auth/service.go`, `backend/internal/auth/service_test.go`, `backend/internal/handler/admin_region_handler.go`, `backend/internal/repository/auth_repository.go`, `backend/internal/rolemgmt/repository.go`, `backend/internal/service/atm_visit_quota_integration_test.go`, `backend/internal/service/vendor_request_actions.go`, `backend/internal/service/vendor_request_ticket.go`, `pkg/auth/repository.go`, `pkg/auth/token_service_test.go`.
- **Langkah**: `gofmt -w` pada file di atas, tanpa menyentuh `internal/db/`.
- **Selesai bila**: `gofmt -l .` kosong di `backend/` & `pkg/` (kecuali `internal/db`), `go build` + `go test` hijau. Commit terpisah: `chore: gofmt backend and pkg`.

## F2 — Konfigurasi lint + govulncheck
- **Masalah**: belum ada `.golangci.yml`, jadi tiap developer memakai default yang berbeda. `govulncheck` belum terpasang.
- **Langkah**:
  1. Tambah `.golangci.yml` di root (dipakai ketiga modul). Linter yang diaktifkan: default (errcheck, govet, staticcheck, unused, ineffassign) + `gosec` + `gocyclo` (min 30, sebagai pagar, bukan target). Exclude `internal/db/` dan exclude errcheck di `_test.go` untuk `Close`/`Rollback`. Exclude ST1005 untuk `pkg/auth/errors.go`.
  2. `go install golang.org/x/vuln/cmd/govulncheck@latest` (tool dev, bukan dependency proyek), lalu jalankan di 3 modul dan catat hasilnya.
  3. Tambah perintah lint + govulncheck ke CLAUDE.md Sec 2a (Commands).
- **Selesai bila**: `golangci-lint run ./...` di 3 modul hanya menampilkan isu yang memang tercatat di F3–F4; hasil govulncheck tercatat (CVE ditemukan → item baru di file ini).
- **Catatan**: menambah tool dev, bukan library aplikasi. Tetap minta OK user (Golden Rule #1).

## F12 — CVE dari govulncheck  ⚠️ security
Hasil `govulncheck ./...` (2026-10-09). Hanya yang **dipanggil** kode kita (reachable):

| Sumber | Versi sekarang | Perbaiki ke | Advisory | Modul terdampak |
| --- | --- | --- | --- | --- |
| `github.com/go-chi/chi/v5` | v5.2.1 | ≥ v5.3.0 | GO-2025-3770 (host header → open redirect di `RedirectSlashes`), GO-2026-5777 | backend, backend-cit |
| `github.com/jackc/pgx/v5` | v5.7.4 | ≥ v5.9.2 | GO-2026-5004 | backend |
| `golang.org/x/text` | v0.40.0 | ≥ v0.41.0 | GO-2026-6629 | backend, backend-cit |
| Go stdlib (`crypto/tls`, `crypto/x509`, `encoding/asn1`, `encoding/xml`, `net/http`, `net/textproto`, `net/url`) | go1.26.3 (lokal) | ≥ go1.26.9 | GO-2026-5856, -6090, -6607, -5037, -5972, -6088, -6089, -6617, -5039, -6608, -6218 | semua |

- **Langkah**:
  1. Upgrade modul di `backend/` dan `backend-cit/`: `go get github.com/go-chi/chi/v5@v5.3.0 github.com/jackc/pgx/v5@v5.9.2 golang.org/x/text@v0.41.0 && go mod tidy` (pkg/ juga kalau memakai chi/pgx). Ini upgrade library yang sudah ada, bukan library baru, tetapi tetap minta OK user (Golden Rule #1). Baca changelog chi v5.3 dan pgx v5.8–5.9 untuk breaking change.
  2. Toolchain: dev lokal ke Go ≥ 1.26.9. Dockerfile `backend/` & `backend-cit/` memakai `golang:1.25-alpine` (tag mengambang, versi patch tidak terkunci); pin ke versi patch yang sudah bebas CVE di atas (cek govulncheck dengan toolchain itu), dan samakan `go` directive di `go.work`/`go.mod` (sekarang `go 1.25.0`).
  3. Jalankan ulang build + test `-tags integration` + govulncheck; target: tidak ada temuan reachable.
- **Selesai bila**: govulncheck "No vulnerabilities found" (reachable) di 3 modul; test hijau.

## F13 — Spoofing IP client  ⚠️ auth (Sec 4 #7)
- **Ditemukan**: saat F12, chi v5.3.0 menandai `middleware.RealIP` deprecated (GHSA-3fxj-6jh8-hvhx dkk.): ia menimpa `r.RemoteAddr` dengan nilai header `X-Forwarded-For`/`X-Real-IP`/`True-Client-IP` apa pun dari client.
- **Masalah lebih luas** (sudah ada sebelum upgrade): `extractClientIP` (`backend/internal/handler/auth_handler.go:337`) mengambil IP **paling kiri** dari `X-Forwarded-For` tanpa memeriksa proxy tepercaya. Dipakai untuk:
  - rate limit login per (email, IP) — `backend/internal/auth/service.go:85,127` → attacker bisa memutar IP palsu untuk menghindari limit per-IP (lockout per-akun lokal tetap berlaku, jadi brute force satu akun tetap dibatasi);
  - `ip` di `audit_logs` untuk semua aksi master-data/admin → IP audit bisa dipalsukan (Sec 5: audit wajib mencatat IP).
- **Lokasi**: `backend/cmd/api/main.go:100` & `backend-cit/cmd/api/main.go:63` (`r.Use(middleware.RealIP)`), `extractClientIP` + ±40 pemanggilnya (lewat helper, jadi cukup diperbaiki di satu tempat), komentar `backend/internal/audit/writer.go:3`.
- **Keputusan yang dibutuhkan dari user**: topologi proxy produksi: Nginx frontend → backend? GCP Load Balancer di depan (Sec 10)? Berapa hop proxy tepercaya? (LB GCP menambahkan `<client>,<lb>` di akhir XFF.)
- **Arah fix**: hapus `middleware.RealIP`; `extractClientIP` ambil IP dari **kanan** `X-Forwarded-For` dikurangi N hop proxy tepercaya (N dari config env, default 0 = pakai `RemoteAddr`), hanya bila `RemoteAddr` berasal dari proxy tepercaya. Test: header palsu tidak mengubah IP; rate limit tetap per IP asli.
- **Selesai bila**: lint SA1019 hilang, test spoofing lulus, `.env.example` + docs/deployment.md mencatat config proxy.

## F14 — `go.work` di-gitignore vs Dockerfile
- **Masalah**: `.gitignore:18-19` mengabaikan `go.work`/`go.work.sum`, tetapi `backend/Dockerfile` & `backend-cit/Dockerfile` menjalankan `COPY go.work go.work.sum ./`. Build di mesin dev berhasil (file ada lokal), tetapi dari clone bersih (CI/Cloud Build) gagal. Pin `toolchain` di `go.work` (F12) juga tidak ikut ter-commit, makanya pin ada di `go.mod`.
- **Pilihan**: (a) commit `go.work` + `go.work.sum` (umum untuk monorepo yang sengaja memakai workspace, seperti di sini); atau (b) Dockerfile tidak memakai workspace (`GOWORK=off`, cukup `replace ../pkg` yang sudah ada di go.mod). Saran: (a), lebih sederhana dan konsisten dengan CLAUDE.md Sec 3 (`go.work` bagian dari layout).
- **Selesai bila**: `docker build` dari clone bersih berhasil untuk kedua backend.

## F15 — `.dockerignore` root + dokumen compose basi
- **Masalah**:
  - Compose `backend/docker-compose.yaml` & `backend-cit/docker-compose.yaml` memakai `context: ..` (root repo). Docker hanya membaca `.dockerignore` di root context (atau `<Dockerfile>.dockerignore`), jadi `backend/.dockerignore` **tidak pernah dipakai**. Akibatnya seluruh repo (frontend `node_modules`, `FTP_DATA`, dokumen .docx, `graphify-out`, `.git`) ikut dikirim sebagai context, dan `COPY backend/` membawa `backend/.env` ke stage builder (tidak sampai ke image final, karena stage final hanya menyalin binary + migrations — tapi tetap ada di cache layer builder).
  - CLAUDE.md Sec 9 menyebut "root `docker-compose.yml`: backend + backend-cit + redis", padahal tidak ada; yang ada compose per backend.
- **Langkah**: tambah `backend/Dockerfile.dockerignore` + `backend-cit/Dockerfile.dockerignore` (atau satu `.dockerignore` root yang mengecualikan semuanya kecuali `go.work*`, `pkg/`, `backend/`, `backend-cit/`) dengan `**/.env`, `**/node_modules`, `.git`. Perbaiki teks CLAUDE.md Sec 9.
- **Selesai bila**: docker build kedua backend tetap berhasil, context build jauh lebih kecil, `.env` tidak masuk context.

## F3 — errcheck di kode produksi
- **Lokasi**:
  - `backend/internal/handler/admin_master_data_import_handler.go:59,106` `defer file.Close()`
  - `backend/internal/notification/smtp.go:51,56,59` `conn.Close()` / `defer c.Close()`
  - `pkg/middleware/rbac.go:113` `json.NewEncoder(w).Encode(...)`
  - `pkg/response/response.go:42` `json.NewEncoder(w).Encode(v)`
  - `backend-cit/cmd/api/main.go:51` `defer redisClient.Close()`, `:70` `w.Write(...)`
- **Langkah**: close pada file read-only/koneksi yang sudah gagal → `_ = x.Close()` (eksplisit diabaikan). `Encode` gagal setelah header terkirim → log via `slog.Warn`, tidak mengubah response. Jangan mengubah perilaku.
- **Selesai bila**: errcheck bersih di kode non-test, test hijau.

## F4 — Guard konversi int → int32
- **Lokasi**:
  - `backend/internal/repository/masterdata_import_batch_repository.go:92` `RowCount: int32(len(rows))`. Cek bahwa importer sudah membatasi jumlah baris (`MasterDataImportMaxBytes`); kalau batas baris tidak eksplisit, tambah guard.
  - `backend/internal/service/masterdata_applier_vendor_package.go:57`, `masterdata_applier_vendor_package_price.go:45,142` `int32(p.TierMin)` / `tierMaxInt32`. Pastikan validasi submit membatasi tier ke rentang kecil; tambah guard di validasi (bukan di applier) bila belum.
  - `backend/internal/service/atm_portal.go:213-214` dan `atm_portal_cashpos.go:124-125`: sudah aman (PageSize 1–100). Cukup beri komentar/nolint. (Catatan: gosec G115 tidak deterministik antar-run, kadang hanya salah satu yang muncul.)
- **Selesai bila**: tiap konversi punya batas yang terbukti (validasi + test), gosec G115 bersih atau di-nolint dengan alasan.

## F16 — `page` tanpa batas atas
- **Ditemukan**: saat F4. Semua parser `page` hanya mengecek `≥ 1`; service lalu `int32(params.Page)` (dan `vendor_order.go:139` menghitung `(Page-1)*PageSize` di int lalu `int32`). `page=3000000000` → wrap negatif → Postgres menolak OFFSET negatif → 500. Tidak ada korupsi data; murni input absurd → 500 bukan 400.
- **Lokasi parser**: `handler/admin_user_handler.go:408` `parsePageParams` (dipakai 12 handler admin), `handler/atm_portal_handler.go` `parseIntParam` (4×), `admin_region_handler.go:63` (inline), `vendor_order_handler.go:84` (`Atoi`, error diabaikan), handler DSR/DMAA/vendor request (cek). Konversi: `atm_portal.go:213`, `atm_portal_cashpos.go:124`, `atm_portal_profile.go:287,332`, `dmaa_forecast.go:122`, `dsr_upload.go:509`, `vendor_order.go:139`, `vendor_request_actions.go:82,245`.
- **Arah fix**: satu konstanta `maxPage` (mis. 100000 — dengan page_size 100 = 10 juta baris) di parser; tolak 400 di atasnya. Contoh yang sudah benar: `notification_handler.go` `parseBoundedInt(..., 1, 1<<20)`. Lalu `//nolint:gosec` dengan alasan di konversi.
- **Selesai bila**: `page=3000000000` → 400 di semua list endpoint (test handler), gosec G115 bersih.

## F5 — Permission file upload DSR
- **Lokasi**: `backend/internal/service/dsr_upload.go:191` (`MkdirAll 0o755`), `:195` (`WriteFile 0o644`).
- **Langkah**: ubah ke `0o750` / `0o640`. Cek bahwa proses Python `service_dsr_etl` membaca folder yang sama dengan user/grup yang sama (docker-compose / deploy doc). Kalau beda user, catat syarat grup bersama di `docs/deployment.md`.
- **Catatan**: di Windows dev, permission ini praktis tidak berpengaruh; efeknya di VM Linux.
- **Selesai bila**: test DSR upload hijau, catatan deploy diperbarui bila perlu.

## F6 — Repo admin pakai replica untuk read
- **Masalah**: `main.go` sudah membuat `dbReadPool` (`backend/cmd/api/main.go:60-80`), tetapi repo berikut masih hanya menerima `dbPool` dan komentarnya masih "TODO saat DATABASE_REPLICA_URL di-wire":
  - `NewUserAdminRepository` (`main.go:152`, `user_admin_repository.go:18`)
  - `NewVendorVaultAdminRepository` (`:309`, `vendor_vault_admin_repository.go:17`)
  - `NewVendorPicAdminRepository` (`:321`, `vendor_pic_admin_repository.go:17`)
  - `NewVendorPackageAdminRepository` (`:328`, `vendor_package_admin_repository.go:17`)
  - `NewVendorPackagePriceAdminRepository` (`:338`, `vendor_package_price_admin_repository.go:18`)
  - `NewVendorBranchAdminRepository` (`:348`, `vendor_branch_admin_repository.go:17`)
  - ATM assignment admin repo (`atm_assignment_admin_repository.go:20`)
- **Langkah**: ikuti pola yang sudah ada di `NewVendorAdminRepository(dbPool, dbReadPool)`. List/get layar → `dbRead`; lookup di dalam flow submit/apply (read-after-write, validasi stale) tetap ke primary. Hapus komentar TODO.
- **Risiko**: replica lag. Layar yang langsung me-refresh setelah submit bisa menampilkan data lama. Karena master data maker-checker (row baru berubah setelah approve), dampaknya kecil. `users` (apply langsung, bukan maker-checker) perlu dicek khusus: setelah create/update, response sebaiknya dibaca dari primary.
- **Selesai bila**: semua repo di atas menerima `(db, dbRead)`, test integrasi hijau, `grep "DATABASE_REPLICA_URL wiring lands"` kosong.

## F7 — Uang `float64` → string desimal  ⚠️ Sec 4 #7 (money)
- **Masalah**: nilai IDR dikonversi `pgtype.Numeric` → `float64` untuk response (melanggar CLAUDE.md Sec 6). Read-only, tetapi kontrak API-nya salah.
- **Lokasi backend**:
  - `backend/internal/service/atm_portal.go:131-137`, `:286-302`, helper `numericToFloat64Ptr` `:327` (threshold, refund_total, replenish_total, escrow)
  - `backend/internal/handler/atm_portal_handler.go:295-301`
  - `backend/internal/service/dsr_upload.go:338-357`, `:568-...` (denom_*, line_total_idr, fill_*_idr, splank_balance_0800_idr)
  - `backend/internal/handler/dsr_upload_handler.go:326-344`
- **Reuse**: helper `numericToDecimalStringPtr` sudah ada (`backend/internal/service/atm_portal_profile.go:421`). Setelah migrasi, hapus `numericToFloat64Ptr`.
- **Dampak frontend** (kontrak berubah number → string):
  - CompanyPortal: `src/features/atm-portal/types.ts`, `components/AtmTable.tsx`, `AtmHeader.tsx`, `ReplenishTable.tsx` + test terkait
  - VendorPortal: `src/features/dsr/dsrUploadApi.ts`, `DsrDetailDialog.tsx`, `DsrUploadDialog.tsx` + `__tests__/DsrUploadDialog.test.tsx`
  - Format tampilan tetap IDR, `tabular-nums`, rata kanan (Sec 13). Jangan `parseFloat` lalu menghitung; untuk format, pakai util format IDR yang sudah ada di masing-masing app.
- **Dok.**: AI-DLC (`.claude/sdlc/money-decimal-string/`) karena menyentuh money + kontrak API dua frontend.
- **Selesai bila**: `grep float64` di `internal/service` & `internal/handler` (non-test) tidak mengenai field uang; test backend + `pnpm test` + `build` kedua frontend hijau; verifikasi browser dicatat *outstanding* (tugas user).

## F8 — Coverage ≥ 80%
- **Kondisi** (dengan `-tags integration`): `repository` 33.5%, `handler` 68.0%, `service` 72.5%, `pkg/response` 0%.
- **Langkah** (batch kecil, satu commit per batch, prioritas Sec 4 #7 dulu):
  1. `service`: `vendor_request_actions.go`, `vault_plan_actions.go`, `vendor_party.go`, master-data applier. Fokus cabang gagal/RBAC/maker≠checker.
  2. `repository`: integration test repo admin read-only (list/filter/pagination), sekalian memverifikasi routing replica F6.
  3. `handler`: jalur 4xx (parse/validasi/forbidden) yang belum tersentuh.
  4. `pkg/response`: 1 test kecil.
- **Alat**: `go test -tags integration -coverprofile=c.out ./internal/... && go tool cover -func=c.out | sort -k3 -n` untuk mencari fungsi 0%.
- **Selesai bila**: tiap package `internal/*` ≥ 80% dengan tag integration.
- **Progres**:
  - Batch 1 (2026-10-09): `VendorPackagePriceApplier` + `mapPackageDBError` — service 73.0% → 74.3%. Menemukan bug F17.
  - Kandidat batch 2 (0% / rendah, dari coverprofile): `CurrentState` di 6 applier (atm_assignment, dsr_location_map, vendor_branch, vendor_package, vendor_pic, vendor_vault); `VendorRequestService.ListVendorOptions` + `AuditLog` (0%); `VaultPlanService.Reject` (62.5%), `SaveAssignments` (68.2%); `UpdateItems` (64.8%); `vendor_party.cancelVendorParties` (72.7%).

## F9 — Pecah `vendor_request_actions.go`
- **Kondisi**: 1210 baris (batas 800). Isinya create/edit, transition, completion, dll. dalam satu file.
- **Langkah**: pindah fungsi ke file per kelompok (mis. `vendor_request_edit.go`, `vendor_request_transition.go`, `vendor_request_completion.go`) dalam package yang sama. Murni pindah, tanpa ubah logika.
- **Kapan**: saat fitur berikutnya menyentuh Vendor Request, supaya diff tidak bentrok dengan pekerjaan yang sedang jalan.
- **Selesai bila**: tidak ada file > 800 baris, diff hanya pemindahan, test hijau.

## F10 — Fungsi kompleks (gocyclo > 20)
| Fungsi | Lokasi | Cyclo | Rencana |
| --- | --- | --- | --- |
| `importEnv.validateFields` / `validateRow` / `buildEnv` / `readImportCSV` | `backend/internal/service/masterdata_import.go:669/490/338/127` | 47/40/21/22 | **Jangan refactor terpisah**: dikerjakan saat import diganti flow FSD replace-all (Sec 12) |
| `UserAdminService.Create` / `Update` | `backend/internal/auth/user_admin.go:154/294` | 34/34 | Ekstrak validasi input ke satu fungsi bersama (auth → STOP & flag, Sec 4 #7) |
| `VaultPlanService.ReviewRequest` | `backend/internal/service/vault_plan_actions.go:283` | 25 | Ekstrak cabang approve/reject |
| `syncVendorParties` | `backend/internal/service/vendor_party.go:132` | 24 | Ekstrak diff party lama vs baru |
| `VendorRequestService.UpdateItems` / `transition` | `backend/internal/service/vendor_request_actions.go:734/958` | 23/23 | Gabung dengan F9 |
| `ATMAssignmentApplier.Apply` | `backend/internal/service/masterdata_applier_atm_assignment.go:88` | 21 | Ekstrak validasi sumber paket |

- **Syarat**: refactor hanya setelah coverage fungsi tersebut cukup (F8), supaya perilaku terkunci test.

## F11 — CI (perlu keputusan user)
- **Kondisi**: tidak ada `.github/workflows/` (atau pipeline lain) di repo. Test integrasi butuh `-tags integration` + Postgres; tanpa itu, coverage `service` hanya 37.8%.
- **Pertanyaan**: CI di mana? (GitHub Actions / Cloud Build sesuai `docs/deployment.md`.) Postgres service container untuk test integrasi?
- **Isi minimal**: build + vet + golangci-lint + `go test -tags integration` (3 modul) + govulncheck; frontend `pnpm test/lint/build` kedua app.

---

## Log pengerjaan
| Tanggal | ID | Commit | Catatan |
| --- | --- | --- | --- |
| 2026-10-09 | F1 | `5feca65` | `gofmt -w` 12 file. 8 file berubah di git (59+/59−, whitespace + 1 baris `//` doc comment); 4 sisanya hanya beda line ending (dinormalisasi git). `gofmt -l` bersih, build/vet/test backend + pkg hijau. |
| 2026-10-09 | F2 | `72e8610` | `.golangci.yml` di root (standard + gosec + gocyclo≥30; exclude `internal/db`, errcheck/gosec/gocyclo/QF di test, ST1005 `pkg/auth/errors.go`). `//nolint:gosec` G120 di `dsr_upload_handler.go:93`. Perintah lint + govulncheck ditambah ke CLAUDE.md Sec 2a. Sisa temuan lint = F3/F4/F5/F10 saja (backend 17, backend-cit 2, pkg 2). govulncheck menemukan CVE → item baru F12. |
| 2026-10-09 | F12 | `9f44cf0` | chi v5.3.0, pgx v5.9.2, x/text v0.41.0; `toolchain go1.26.9` di go.mod 3 modul; Dockerfile `golang:1.26.9-alpine`. govulncheck bersih di 3 modul; build/vet/test + integration hijau. Temuan baru: F13 (RealIP/XFF spoofing), F14 (go.work gitignored). Detail di `.claude/bugfixes.md`. |
| 2026-10-09 | F14 | `21f8cff` | `go.work` + `go.work.sum` di-commit, `.gitignore` diperbarui. Docker build kedua backend dari export index (setara clone bersih) berhasil. Temuan baru: F15 (`.dockerignore` root tidak ada, `.env` ikut context; CLAUDE.md compose basi). |
| 2026-10-09 | F15 | `ce9fa0f` | `.dockerignore` root (allowlist) — context 8.1MB, tanpa `.env`/`.md`; `backend/.dockerignore` mati dihapus; CLAUDE.md Sec 9 diperbaiki. Docker build kedua backend berhasil. |
| 2026-10-09 | F3 | `e2de53d` | 9 call site di 5 file → `_ =` sesuai konvensi repo (bukan `slog.Warn` seperti rencana awal: neighbour pattern menang, perilaku sama). errcheck bersih; semua test + integration hijau. |
| 2026-10-09 | F4 | `ebcd87a` | Tier ternyata **bug nyata** (tanpa batas atas → wrap int32 diam-diam saat approve): `maxTier = 999_999_999` di kedua validator + test 7 kasus. `RowCount` aman (≤ 2000) → nolint. Konversi page **bukan** false positive → dipisah ke F16. |
| 2026-10-09 | F5 | `e69e4ef` | Keputusan user: **terima 0644/0755** — Python ETL (user host lain, tidak di-container-kan) harus baca + pindahkan file; 0640 akan memutus DSR di server. Komentar + nolint di `dsr_upload.go`, catatan di `docs/deployment.md`. Perketat saat Python di-container-kan. |
| 2026-10-09 | F6 | `4298546` | 6 repo admin master-data: List/Count → replica (GetByID/pre-check tetap primary). `UserAdminRepository` **sengaja tetap primary** (apply langsung + re-list setelah write). Test topologi 18 kasus. TODO basi = 0. |
| 2026-10-09 | F7 | — | AI-DLC dimulai: `.claude/sdlc/money-decimal-string/intent.md` (draft). Temuan tambahan: `computeReplenishmentStatus` membandingkan uang dalam float (logika, bukan hanya tampilan). Menunggu PO accept + jawaban Q1–Q6. |
| 2026-10-09 | F16 | `255c6a8` | `maxPage = 1_000_000` + `parsePageParam` di semua parser page (9 + admin + region + vendor order). Offset SQL dihitung int4, jadi ini juga mencegah overflow di SQL. gosec 0; sisa lint hanya F10 (gocyclo) + F13 (RealIP), keduanya ditunda. |
| 2026-10-09 | F8 b1 | (belum di-commit) | Test applier harga vendor-wide (unit + integration). service 73.0→74.3%, `VendorPackagePriceApplier.Apply` 0→83.3%. Temuan bug **F17** (disable harga yang belum/baru berlaku gagal) → ditunda, butuh keputusan PO. |
