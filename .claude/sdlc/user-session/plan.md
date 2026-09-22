# Plan & Task List: User Session Lifetime (absolut 1 jam, parameterized)

Status: **draft (2026-09-21)** — belum mulai coding. Stage: 3 Build.
Catatan SDLC: `intent.md` & `spec.md` belum ada (feature ini masuk lewat permintaan langsung). Isi menyusul kalau perlu jejak audit penuh.

## Keputusan (2026-09-21)
| # | Topik | Keputusan |
|---|---|---|
| D1 | Cara hitung timeout | **Absolut dari login.** Sesi mati 1 jam setelah login, aktif atau tidak. Bukan idle timeout. |
| D2 | Ruang lingkup parameter | **Satu env var global**, berlaku untuk SEMUA user & kedua portal. Ubah 1 jam → 2 jam = ganti `SESSION_MAX_LIFETIME=2h` + redeploy, **tanpa ubah kode**. |
| D3 | Cara enforce | Pakai `exp` refresh token itu sendiri sebagai deadline sesi — **tanpa claim baru, tanpa tabel session, tanpa Redis tambahan**. Rotasi mewarisi `exp` lama (tidak memperpanjang). |
| D4 | `REFRESH_TOKEN_EXPIRY` lama (7 hari) | **Diganti** oleh `SESSION_MAX_LIFETIME`. Tidak ada dua knob yang saling tabrakan. |

## Kondisi awal (kode yang ada sekarang)
- `pkg/config/config.go` — `AccessTokenExpiry` (`ACCESS_TOKEN_EXPIRY`, default 15m) + `RefreshTokenExpiry` (`REFRESH_TOKEN_EXPIRY`, default **7 hari**). Sudah ada helper `parseDuration`.
- `pkg/auth/token_service.go` — `GenerateTokenPair` → access token `exp = now + 15m`, refresh token `exp = now + 7d` + JTI. `ValidateRefreshToken` sudah cek `exp` + blacklist Redis.
- `backend/internal/handler/auth_handler.go` `Refresh()` — validasi cookie → blacklist token lama (rotasi) → `GenerateTokenPair` **baru penuh 7 hari lagi** → sliding window praktis tak terbatas selama user aktif.
- `setRefreshCookie()` (L304) — `MaxAge: 604800` **hardcoded 7 hari**.
- FE `CompanyPortal-Vite/src/lib/api/client.ts` — 401 → single-flight refresh → gagal = `logout()`. `VendorPortal-Vite/src/features/auth/AuthContext.tsx` pola serupa.
- `backend-cit` hanya **memvalidasi** access token, tidak pernah menerbitkan → ikut otomatis, tanpa perubahan kode.

## Inti perubahan (satu paragraf)
Deadline sesi = `exp` refresh token pertama (`login_at + SESSION_MAX_LIFETIME`). Setiap rotasi di `/refresh` menerbitkan refresh token baru dengan `exp` **yang sama** (bukan `now + lifetime`), jadi `ValidateRefreshToken` yang sudah ada langsung jadi gerbang absolutnya — tidak ada kode gate baru. Access token di-cap ke `min(now + ACCESS_TOKEN_EXPIRY, deadline)` supaya tidak ada sisa akses lewat batas. Cookie `MaxAge` ikut sisa detik ke deadline.

---

## Task list
Legenda: `[ ]` todo · `[~]` in progress · `[x]` done · **Validate** = bukti task selesai.

### Fase 1 — Parameter
- [x] **T1.1** `pkg/config`: ganti field `RefreshTokenExpiry` → `SessionMaxLifetime`, baca `SESSION_MAX_LIFETIME` (default `1h`) lewat `parseDuration` yang sudah ada.
  - Validate: unit test — env kosong → 1h; `SESSION_MAX_LIFETIME=2h` → 2h; nilai ngawur → fallback 1h (bukan 0, jangan sampai sesi mati seketika).
- [x] **T1.2** Guard nilai minimum: `< 5m` dianggap salah ketik → fallback default + log warning. (Konstanta bernama, jangan magic number.)
  - Validate: unit test `SESSION_MAX_LIFETIME=10s` → 1h + satu baris log.
- [x] **T1.3** Update `.env.example`, `docker-compose.yml`, dan CLAUDE.md §9 (daftar env). Hapus `REFRESH_TOKEN_EXPIRY`.
  - Validate: `grep -r REFRESH_TOKEN_EXPIRY` nihil.

### Fase 2 — Token service (`pkg/auth`)
- [x] **T2.1** `TokenConfig`: `RefreshTokenExpiry` → `SessionMaxLifetime`. Sesuaikan komentar `// 7 days`.
- [x] **T2.2** `GenerateTokenPair(identity, deadline time.Time)` — deadline nol = sesi baru (`now + SessionMaxLifetime`); deadline terisi = rotasi (warisi apa adanya).
  - Validate: table-driven test — login → `exp` = now+1h; rotasi pada menit ke-30 → `exp` tetap sama, **tidak** mundur maupun maju.
- [x] **T2.3** Clamp token warisan: deadline efektif = `min(deadline, now + SessionMaxLifetime)`. Melindungi dari refresh token 7-hari yang masih beredar saat deploy, dan dari `SESSION_MAX_LIFETIME` yang dipendekkan.
  - Validate: test — deadline masuk +7d, lifetime 1h → `exp` hasil = now+1h.
- [x] **T2.4** Cap access token: `exp = min(now + AccessTokenExpiry, deadline)`.
  - Validate: test — sisa sesi 5 menit → access token `exp` 5 menit, bukan 15.
- [x] **T2.5** Perbarui pemanggil di `backend/internal/auth/service.go` (`Login`, deadline nol).

### Fase 3 — Handler
- [x] **T3.1** `Refresh()`: ambil `claims.ExpiresAt` dari refresh token lama → teruskan sebagai deadline ke `GenerateTokenPair`.
  - Validate: integration test — login, rotasi 3× dalam 1 jam → sukses; rotasi ke-4 setelah lewat 1 jam → **401**.
- [x] **T3.2** `setRefreshCookie(w, token, deadline)`: `MaxAge = int(time.Until(deadline).Seconds())`, minimal 0. Hapus `604800`.
  - Validate: test — `Set-Cookie` pada rotasi menit ke-30 punya `Max-Age` ≈ 1800.
- [x] **T3.3** Refresh gagal karena sesi habis → selain 401, **bersihkan cookie** (`clearRefreshCookie`) supaya browser tidak terus mengirim token mati. Pesan: `"Sesi berakhir, silakan login kembali"`.
  - Validate: test — response 401 membawa header `Set-Cookie` dengan `Max-Age=-1`.

### Fase 4 — Frontend (kedua portal)
- [x] **T4.1** `CompanyPortal-Vite/src/lib/auth/store.ts` + `VendorPortal-Vite/.../AuthContext.tsx`: saat `refreshToken()` gagal, `logout()` lalu redirect ke login dengan pesan sesi berakhir (bukan pesan error generik).
  - Validate: component test / manual — mock 401 dari `/refresh` → mendarat di halaman login dengan notice.
- [x] **T4.2** Pastikan notice sesi berakhir bukan sekadar warna (ikon + teks) — CLAUDE.md §13 aksesibilitas.

### Fase 5 — Test & tutup
- [x] **T5.1** Perbarui test yang mengasumsikan refresh 7 hari: `pkg/auth/token_service_test.go`, `token_property_test.go`, `session_property_test.go`, `pkg/config` test.
  - Validate: `go test ./...` hijau di tiga modul (`pkg`, `backend`, `backend-cit`).
- [x] **T5.2** Coverage `pkg/auth` tetap ≥ 80%.
- [x] **T5.3** Uji parameter sungguhan: jalankan dengan `SESSION_MAX_LIFETIME=2m`, login, tunggu lewat → verifikasi logout paksa. Bukti dicatat di `tests.md`.
- [x] **T5.4** Update `.claude/development-progress.md` + `graphify update .`.

---

## Urutan & dependensi
```
T1 → T2 → T3 → T4 → T5
```
Backend (T1–T3) dan frontend (T4) harus di-deploy bersamaan: setelah T3 semua sesi berumur maks 1 jam, FE lama akan menampilkan error generik alih-alih pesan sesi berakhir.

## Risiko
| Risiko | Level | Mitigasi |
|---|---|---|
| Semua user aktif ter-logout saat deploy (clamp T2.3) | SEDANG | Deploy di luar jam operasional (URS: 07:00–20:00) |
| Operator mengeluh login ulang tiap jam saat kerja penuh | SEDANG | Knob `SESSION_MAX_LIFETIME` siap pakai — naikkan tanpa ubah kode |
| Pekerjaan form panjang hilang saat sesi mati di tengah jalan | SEDANG | Di luar scope plan ini; kalau jadi keluhan nyata → intent baru (warning T-5 menit / draft lokal) |
| `SESSION_MAX_LIFETIME` salah ketik di prod → sesi mati seketika | RENDAH | Guard minimum T1.2 |
| Dua VM (rencana 2028) beda nilai env | RENDAH | Hanya backend ATM yang menerbitkan token; CIT tidak perlu var ini |

## Out of scope
Idle timeout (terpisah dari absolut) · warning/countdown sebelum sesi habis · "ingat saya" · timeout per-role atau per-user (butuh kolom/tabel baru — kalau diminta, bikin intent sendiri) · tabel sesi aktif & force-logout admin · audit event khusus sesi berakhir.

## Catatan implementasi
_(isi selama coding: penyimpangan dari plan, keputusan baru)_
