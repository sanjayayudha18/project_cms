# Tests / feedback loop: IP client tepercaya (QC F13)

Status: **green 2026-10-12**. Stage: 4 Test. Reads: `plan.md` (accepted 2026-10-12), `spec.md`.

## Commands (dijalankan 2026-10-12, semua exit 0)
- `gofmt -l` pada file yang diubah: kosong.
- `go vet ./...` + `go test ./...` di `pkg`, `backend-cit`, `backend`: hijau.
- `backend`: `DATABASE_URL=<dev, localhost> go test -tags integration -count=1 -p 1 ./...`: hijau (8 paket ok).
- `golangci-lint run --build-tags integration ./...`: `pkg` 0, `backend-cit` 0, `backend` 2 (gocyclo `masterdata_import` validateRow/validateFields, ditunda di F10). **SA1019 `middleware.RealIP` hilang.**
- Coverage: `pkg/middleware` 87.4%, `pkg/config` 100%.

## Test cases (trace ke spec)
| ID | Req | Test | Hasil |
| --- | --- | --- | --- |
| T1 | FR1 | `pkg/config/config_test.go` `TestLoad_TrustedProxies`: unset, entri kosong, IP tunggal → /32, CIDR di-mask, campuran + spasi + IPv6 `::1` → /128, CIDR IPv6, entri sampah → error, prefix /33 → error | ✅ |
| T2 | FR2, NFR3 | `pkg/middleware/client_ip_test.go` `TestTrustedRealIP`: peer tidak tepercaya + XFF/X-Real-IP palsu → RemoteAddr utuh; daftar kosong = no-op | ✅ |
| T3 | FR2 | idem: dari Nginx tepercaya, XFF `1.2.3.4, 203.0.113.9` → `203.0.113.9`; `X-Real-IP` saja diabaikan | ✅ |
| T4 | FR2, intent Q1 | idem: 2 hop tepercaya (`10.0.0.0/8` sebagai LB) → `203.0.113.9`; XFF terpecah di 2 baris header digabung | ✅ |
| T5 | FR2 | idem: tanpa XFF, semua hop tepercaya, entri paling kanan rusak → RemoteAddr utuh; peer `[::1]:port`; IPv4-mapped `::ffff:` di-unmap | ✅ |
| T6 | FR3 | `backend/internal/handler/client_ip_test.go` `TestExtractClientIP`: IPv4:port, `[IPv6]:port`, tanpa port (IPv4/IPv6); XFF diabaikan | ✅ |
| T7 | FR3, NFR3 | idem `TestExtractClientIP_BehindTrustedRealIP`: `TrustedRealIP` → `extractClientIP` (nilai yang `Login` kirim sebagai `LoginRequest.IP`): klien langsung dengan XFF palsu → IP koneksi; lewat Nginx → IP klien asli | ✅ |
| T8 | NFR4 | lint kedua backend tanpa SA1019 | ✅ |

**Deviasi dari plan (T7)**: plan menyebut fake auth service yang merekam `LoginRequest.IP`. `AuthHandler.authService` bertipe konkret `*auth.Service`, jadi fake butuh refactor handler di luar scope. T7 menguji rantai middleware → `extractClientIP`, yaitu persis nilai yang dipakai `Login` (`auth_handler.go:89`).

## Manual (tugas user, GR#10) — outstanding
- Login lewat CompanyPortal (Nginx container) dengan `TRUSTED_PROXIES` = gateway Docker → `audit_logs.ip` / log request menampilkan IP browser, bukan gateway.
- `curl -H 'X-Forwarded-For: 1.2.3.4'` langsung ke :8080 → IP tercatat tetap IP pemanggil.
- Start log menampilkan `trusted proxies configured count=N` sesuai env.
