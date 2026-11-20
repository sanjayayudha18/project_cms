# Bug Fix Log (Gemini / Antigravity)

Catatan perbaikan bug skala kecil yang diputuskan user tidak memerlukan alur spec fitur penuh (`.agents/specs/<feature>/`). Urutan terbaru di paling atas. Satu entri per perbaikan.

Template:

```markdown
## YYYY-MM-DD — <judul singkat>
- **Symptom**: gejala/error yang terlihat oleh user
- **Root cause**: akar penyebab masalah
- **Fix**: perubahan yang dilakukan (daftar file)
- **Tests**: automated test yang ditambahkan/dijalankan; manual browser verification: outstanding
- **Commit**: <sha>
```

---

## 2026-11-20 — Fix CI Failures on PR #5 (Go Toolchain Vulnerability & Test Statement Coverage)
- **Symptom**: CI pipeline failed on GitHub PR #5 with 8 failing checks:
  1. `CI / go (backend)`, `CI / go (backend-exq)`, `CI / go (pkg)` failed during `govulncheck` reporting vulnerability `GO-2026-6859` in `net/http@go1.26.10`.
  2. `CI / go-integration` failed because statement coverage in `internal/audit` (81.2% < 86%) and `internal/repository` (63.6% < 86%) fell below the required thresholds defined in `backend/coverage-thresholds.txt`.
- **Root cause**:
  1. `go.work` and `go.mod` files pinned `toolchain go1.26.10`, which contained an active standard library vulnerability (`GO-2026-6859`, fixed in `go1.26.11`).
  2. `internal/audit`: The transaction writer method `WriteTx` had 0% coverage.
  3. `internal/repository`: Multiple new repository implementations and methods added in PR #5 (e.g. `PksListRepository`, `PksTypeRepository`, `VendorVaultAdminRepository`, `VendorPicAdminRepository`, `CashCountPicAdminRepository`, etc.) lacked integration test coverage.
- **Fix**:
  1. Bumped toolchain from `go1.26.10` to `go1.26.11` in `go.work`, `backend/go.mod`, `backend-exq/go.mod`, `backend-extapi/go.mod`, and `pkg/go.mod`.
  2. Added `TestWriter_WriteTx` to `backend/internal/audit/writer_test.go` (coverage raised from 81.2% to 87.5% >= 86%).
  3. Added comprehensive repository integration test coverage:
     - `backend/internal/repository/vendor_pks_admin_repository_test.go`
     - `backend/internal/repository/vendor_vault_admin_repository_test.go`
     - `backend/internal/repository/vendor_pic_admin_repository_test.go`
     - `backend/internal/repository/cash_count_admin_repository_test.go`
     - Expanded tests in `vendor_admin_repository_test.go`, `vendor_branch_admin_repository_test.go`, `user_admin_repository_test.go`, and `region_admin_repository_test.go`.
     Coverage raised from 63.6% to 87.4% (>= 86%).
- **Tests**:
  - `govulncheck ./...` in `backend`, `backend-exq`, `pkg` (0 vulnerabilities found).
  - `golangci-lint run --build-tags integration ./...` (0 issues across all modules).
  - `go test -tags integration -count=1 -cover -p 1 ./...` and `scripts/check-coverage.sh` passed across `backend`, `backend-exq`, and `pkg`.
  - Manual browser verification: outstanding (user).
- **Commit**: pending user approval

