# Plan: IP client tepercaya / anti-spoof `X-Forwarded-For` (QC F13)

Status: **accepted 2026-10-12** (user: "terima plan, commit, lanjut coding S1-S5"). Stage: 3 Build.
Sumber: `spec.md` (accepted 2026-10-12, S1–S4), `intent.md` (accepted 2026-10-12).

## Titik sentuh yang sudah dicek di kode (2026-10-12)
- `pkg/config/config.go` — `Config` + `Load(defaultPort)`, dipakai `backend/cmd/api/main.go:39` dan `backend-cit/cmd/api/main.go:28`. Test di `pkg/config/config_test.go` (pola `TestLoad_*` dengan `t.Setenv`).
- `pkg/middleware/` — berisi `rate_limiter.go`, `rbac.go` (+ test). Diimpor kedua backend sebagai `custommw`.
- `backend/cmd/api/main.go:100` dan `backend-cit/cmd/api/main.go:63` — `r.Use(middleware.RealIP)` (chi), di antara `RequestID` dan `Logger`.
- `backend/internal/handler/auth_handler.go:337` — `extractClientIP`; dipakai login (`:89` → `LoginRequest.IP` → rate limit `auth/service.go:85,127`) dan 59 pemanggil lain (IP audit). Belum ada test yang mengunci perilakunya (sengaja ditunda di F8 batch 8).
- `backend/internal/audit/writer.go:3` — komentar menyebut "chi's RealIP-normalized r.RemoteAddr".
- `backend/.env.example`, `backend-cit/.env.example`; `.claude/docs/deployment.md`; CLAUDE.md Sec 9 blok env.

## Slices (urutan kerja, satu commit untuk semua karena kecil dan saling terkait)

### S1 — Config `TRUSTED_PROXIES` (FR1, S2, S4)
- `pkg/config/config.go`: field `TrustedProxies []netip.Prefix`; helper `parseTrustedProxies(string) ([]netip.Prefix, error)` — split koma, `TrimSpace`, lewati entri kosong, `netip.ParsePrefix` atau `netip.ParseAddr` → `netip.PrefixFrom(addr, addr.BitLen())`, `Masked()`; entri rusak → error yang menyebut entri. `Load` mengembalikan error itu.
- Test T1 di `pkg/config/config_test.go`.

### S2 — Middleware `TrustedRealIP` (FR2, S1, S3)
- File baru `pkg/middleware/client_ip.go`: `TrustedRealIP(trusted []netip.Prefix) func(http.Handler) http.Handler`.
  - `remote` = `netip.ParseAddrPort(r.RemoteAddr)` (fallback `netip.ParseAddr`), di-`Unmap()`. Tidak valid atau tidak tepercaya → handler berikutnya tanpa perubahan.
  - Tepercaya: gabungkan semua nilai header `X-Forwarded-For` (bisa lebih dari satu baris), split koma, iterasi dari kanan: entri di-`TrimSpace` + `ParseAddr` (+`Unmap`); rusak → berhenti tanpa mengubah; tepercaya → lanjut ke kiri; bukan tepercaya → `r.RemoteAddr = ip.String()`, berhenti.
  - Header `X-Real-IP` / `True-Client-IP` tidak dibaca.
  - Daftar `trusted` kosong → middleware langsung `return next` (tanpa biaya).
- Test T2–T5 di `pkg/middleware/client_ip_test.go` (table-driven, `httptest`).

### S3 — `extractClientIP` (FR3)
- `auth_handler.go:337`: hapus cabang `X-Forwarded-For`; `host, _, err := net.SplitHostPort(r.RemoteAddr)`; err → kembalikan `r.RemoteAddr` apa adanya. Komentar diperbarui (IP sudah dinormalisasi `TrustedRealIP`).
- Test T6 + T7 di `backend/internal/handler/client_ip_test.go` (file baru): T6 unit `extractClientIP`; T7 login lewat router dengan `TrustedRealIP` terpasang + fake auth service yang merekam `LoginRequest.IP` (pakai pola fake di `auth_error_mapping_test.go` / handler test yang ada).

### S4 — Wiring (FR2, FR4)
- `backend/cmd/api/main.go`: ganti `r.Use(middleware.RealIP)` dengan `r.Use(custommw.TrustedRealIP(cfg.TrustedProxies))`; log `slog.Info` jumlah proxy tepercaya saat start (bukan isinya per request).
- `backend-cit/cmd/api/main.go`: hapus `r.Use(middleware.RealIP)` saja.
- T8: `golangci-lint run --build-tags integration ./...` di `backend`, `backend-cit`, `pkg` — SA1019 hilang.

### S5 — Dokumen (FR5)
- `backend/.env.example`: `TRUSTED_PROXIES=` + komentar (format, contoh gateway Docker, kosong = header diabaikan). `backend-cit/.env.example`: baris sama + "belum dipakai".
- `.claude/docs/deployment.md`: bagian "IP client & proxy tepercaya" (nilai produksi, tutup port 8080, langkah saat LB/Cloud Armor, peringatan rate limit bila kosong).
- `backend/internal/audit/writer.go:3`: komentar → "`TrustedRealIP`-normalized r.RemoteAddr".
- CLAUDE.md Sec 9: `TRUSTED_PROXIES=` di blok env; Sec 12: satu baris keputusan F13.
- `tests.md` (stage 4) dibuat setelah test hijau; `backend_QC_fixes.md` F13 + log; `development-progress.md` bila ada barisnya.

## Bukti selesai
- `go test ./...` di `pkg`, `backend-cit`, `backend` hijau; `backend` juga `-tags integration`.
- T1–T8 lulus; coverage `pkg/middleware` tetap ≥ 80%.
- `golangci-lint` hanya menyisakan 2 temuan gocyclo `masterdata_import` (ditunda), tanpa SA1019.
- Manual check (tugas user, GR#10): login lewat CompanyPortal (Nginx container) dengan `TRUSTED_PROXIES` diisi gateway Docker → `audit_logs.ip` / log request menampilkan IP browser, bukan gateway; header `X-Forwarded-For` palsu dari curl langsung ke :8080 tidak mengubah IP. Outstanding.

## Risiko
- **R1 (utama)**: `TRUSTED_PROXIES` kosong/salah di prod → semua user tercatat sebagai IP Nginx → rate limit per IP (20/15 menit) dibagi semua user → login massal terblokir. Mitigasi: log start menyebut jumlah proxy tepercaya; catatan deploy + `.env.example`; fail-fast untuk entri rusak (S2).
- **R2**: dev lokal lewat Vite proxy → IP tercatat `127.0.0.1` / `::1`. Diterima (spec Constraints). Bila perlu, dev boleh isi `TRUSTED_PROXIES=127.0.0.1,::1` (Vite `xfwd` belum aktif, jadi tetap IP loopback).
- **R3**: IPv4-mapped IPv6 (`::ffff:a.b.c.d`) — ditangani dengan `Unmap()` di kedua sisi.
- **R4**: perubahan nilai `audit_logs.ip` mulai rilis ini (lebih benar); baris lama tidak diubah.

## Di luar plan
- Python EOD (`python_eod_client_ip_todo.md`), Nginx config, LB/Cloud Armor/firewall, data audit lama, `TrustedRealIP` di `backend-cit`.
