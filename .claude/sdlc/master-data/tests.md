# Tests — master data

## Backend (`cd backend`)
- `go vet ./...` dan `go test ./...` — unit (service, handler, repository dengan fake).
- Integrasi (DB nyata): `DATABASE_URL=postgres://postgres:1818@localhost:5432/cms?sslmode=disable go test -tags=integration ./internal/service/... ./internal/repository/...`
- Cakupan kunci: RBAC (ADMIN/ADMIN_PARAM + service), maker≠checker, staleness, apply-on-approve, overlap kelolaan, ekspor/template, dry-run (header/BOM/`;`/aturan per entitas/limit/cap error), confirm (1 batch = 1 approval, idempoten per hash, rollback atomik → semua baris `stale`, reject batch, `Submit` gagal melepas slot pending), endpoint detail approval (maker/approver saja).

## Frontend (`cd frontend/CompanyPortal-Vite`)
- `npx tsc --noEmit -p tsconfig.app.json`, `npx vitest run`, `npx biome check <file>`.
- Baru: inbox persetujuan + dialog review (diff & batch), halaman ekspor/impor (error blokir submit; submit batch), detail vendor (tab, legal name/NPWP), dialog kelolaan ATM, badge "Menunggu approval" + kunci aksi, penanganan 202 di layar vendor/ATM.
- Gagal yang sudah ada sebelumnya (bukan bagian pekerjaan ini): 4 tes `SettingsHubPage`; format biome `SettingsHubPage`/`RbacUsersPage`; gofmt `auth_*`, `error_response.go`, `cmd/hashpw`.

## Belum diuji
Verifikasi visual di browser; uji beban impor 10.000 baris; entitas selain vendor pada integrasi batch apply (memakai jalur `applyOne` yang sama).
