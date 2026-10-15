# Tests: Perbaikan tech debt backend (BE-01 – BE-12)

Status: **partial — Fase A (A1, A2, A3) green 2026-10-15** (Stage 4). Input: `spec.md` (FR-A1–A3), `plan.md` Fase A (accepted 2026-10-15). Belum di-commit. Fase B–G belum dikerjakan (plan masih draft untuk fase itu).

## Commands (run 2026-10-15)
| Command | Result |
|---|---|
| alpinelinux.org/releases | stable terbaru v3.24 (3.24.2, support s.d. 2028-06-01); 3.20 EOL 2026-04-01 |
| `FROM alpine:3.20` → `alpine:3.24` di `backend/Dockerfile` + `backend-cit/Dockerfile` | — |
| `docker build -f backend/Dockerfile .` / `-f backend-cit/Dockerfile .` | sukses keduanya; `/etc/alpine-release` = 3.24.2 |
| `docker run --env-file backend/.env` (+ Redis sementara dengan alias `cms-redis`, DB dev via `host.docker.internal`) | `docker inspect` → `running healthy`; `id -un` = `appuser`; `GET /health` 200 (T-A1) |
| `cd backend && go get golang.org/x/crypto@v0.56.0 && go mod tidy` | x/crypto v0.54.0 → v0.56.0; `go` directive 1.25.0 → 1.26.0 (dipaksa x/crypto v0.56.0); `go get` ikut menaikkan `go.work` 1.25.0 → 1.26.0 (workspace harus ≥ module); toolchain tetap go1.26.9 = builder `golang:1.26.9`. `backend-cit`/`pkg` tidak me-require x/crypto → tidak berubah |
| `cd backend && go build ./... && go vet ./... && go test ./...` | clean / all ok |
| `cd backend && golangci-lint run --build-tags integration ./...` | 0 issues |
| `go test ./...` di `backend-cit`, `pkg` | all ok |
| `govulncheck ./...` ×3 (T-A2) | backend: 0 terjangkau; sisa 1 di module = **GO-2026-5932** (`x/crypto/openpgp`, tanpa fix, tidak di-import → *accepted*); backend-cit + pkg: no vulnerabilities |
| `git rm -r backend_python/dmaa/bak` (2 file, tidak di-import siapa pun) | — |
| `cd backend_python && py -3 -m unittest discover` | 77 tests OK (55 skip integration tanpa `DATABASE_URL`) |

## Test → requirement
| Test | Covers |
|---|---|
| T-A1 docker build + health `healthy` + non-root | FR-A1 base image didukung, HEALTHCHECK + `USER appuser` tetap (backend) |
| T-A2 govulncheck ×3 + `go test` ×3 | FR-A2 x/crypto ≥ v0.56.0, tanpa regresi |
| unittest Python | FR-A3 `dmaa/bak` dihapus tanpa memutus import |

## Catatan
- `backend-cit/Dockerfile` hanya diganti base image sesuai A1. Image itu **belum** non-root dan **tanpa** HEALTHCHECK (sudah begitu sebelumnya; Sec 9). Di luar Fase A → follow-up (atau sekalian saat rename ke `backend-exq`).

## Outstanding
- Tidak ada manual browser check untuk Fase A.
- T-A1/T-A2 sebagai job CI menunggu fase B.
