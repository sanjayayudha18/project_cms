# Spec: IP client tepercaya / anti-spoof `X-Forwarded-For` (QC F13)

Status: **accepted 2026-10-12** (user: "terima spec, commit, lanjut plan"; S1–S4 dikonfirmasi). Stage: 2 Design — selesai. Stage berikut: `plan.md`.
Sumber: `intent.md` (accepted 2026-10-12, Q1–Q5).

## ⚠ Flagged (Sec 4 #7)
- **Auth**: IP yang dipakai rate limit login per (email, IP) berubah sumbernya. Salah konfigurasi = semua user terlihat dari satu IP (IP Nginx) → limit per IP (`RATE_LIMIT_IP`, default 20 per 15 menit) bisa memblokir login semua orang sekaligus. Ini risiko utama; lihat NFR3 dan catatan deploy.
- **Audit**: nilai kolom `audit_logs.ip` berubah (dari nilai header yang bisa dipalsukan menjadi IP asli). Baris lama tidak diubah.
- Tidak ada migrasi skema, tidak ada endpoint baru, response shape tidak berubah.

## Alur

```
Browser ──► Nginx (frontend container) ──► backend Go :8080
            $remote_addr = IP browser       RemoteAddr = IP Nginx / gateway Docker
            XFF = "<header klien>, <IP browser>"

Middleware TrustedRealIP (pengganti chi RealIP):
  RemoteAddr ∉ TRUSTED_PROXIES  → IP = RemoteAddr (semua header IP diabaikan)
  RemoteAddr ∈ TRUSTED_PROXIES  → baca X-Forwarded-For dari KANAN,
                                  lewati entri yang juga ∈ TRUSTED_PROXIES,
                                  entri pertama yang bukan proxy tepercaya = IP client
                                  (XFF kosong/semua tepercaya/entri rusak → IP = RemoteAddr)
  → r.RemoteAddr ditimpa dengan IP client hasil di atas
extractClientIP(r) → host dari r.RemoteAddr (tidak lagi membaca header)
```

Nanti saat GCP LB / Cloud Armor dipasang (intent Q1): range alamat LB ditambahkan ke `TRUSTED_PROXIES`; algoritma yang sama melewati hop LB tanpa ubah kode.

## Functional requirements

### FR1 — Config proxy tepercaya
- Env baru `TRUSTED_PROXIES`: daftar dipisah koma berisi CIDR (`172.17.0.1/32`, `10.0.0.0/8`) atau IP tunggal (dianggap `/32` / `/128`). Spasi di sekitar entri diabaikan.
- Dibaca di `pkg/config` (`Config.TrustedProxies []netip.Prefix`), sehingga kedua backend memakai parser yang sama.
- Kosong / tidak diset = **tidak ada proxy tepercaya** (default aman): header IP selalu diabaikan.
- Entri tidak valid → `config.Load` mengembalikan error, service gagal start (fail fast, sama seperti `JWT_SECRET` tidak valid).

### FR2 — Middleware `TrustedRealIP` (backend ATM)
- Di `pkg/middleware` (shared, supaya `backend-exq` bisa memakainya nanti): `TrustedRealIP(trusted []netip.Prefix) func(http.Handler) http.Handler`.
- Algoritma persis seperti diagram Alur. Hanya `X-Forwarded-For` yang dibaca; `X-Real-IP` dan `True-Client-IP` **diabaikan** (S3).
- `r.RemoteAddr` ditimpa dengan IP client (tanpa port), sama seperti perilaku chi `RealIP` dulu, supaya `middleware.Logger` dan kode lain yang membaca `RemoteAddr` ikut benar (S1).
- Dipasang di `backend/cmd/api/main.go` di posisi `middleware.RealIP` sekarang (sebelum `Logger`), dengan `cfg.TrustedProxies`.

### FR3 — `extractClientIP`
- `backend/internal/handler/auth_handler.go:337`: tidak lagi membaca `X-Forwarded-For`; mengembalikan host dari `r.RemoteAddr` (pakai `net.SplitHostPort`, fallback nilai apa adanya bila tanpa port). IPv6 tanpa kurung siku.
- 60 pemanggilnya tidak diubah. Rate limit login (`auth.Service.Login`) dan `audit_logs.ip` otomatis memakai IP hasil FR2.

### FR4 — `backend-cit`
- Hapus `r.Use(middleware.RealIP)` di `backend-cit/cmd/api/main.go:63` (intent Q4). Tidak memasang `TrustedRealIP` sekarang.

### FR5 — Dokumentasi & config
- `backend/.env.example`: `TRUSTED_PROXIES=` (kosong) + komentar cara mengisi. `backend-cit/.env.example`: baris yang sama dengan komentar "belum dipakai".
- `.claude/docs/deployment.md`: bagian "IP client & proxy tepercaya":
  - nilai produksi = IP yang dilihat backend saat Nginx menyambung (gateway jaringan Docker di VM, cek `docker network inspect` / log `RemoteAddr`), sesempit mungkin (`/32`);
  - port 8080 sebaiknya ditutup dari luar VM (firewall / tidak di-publish), sehingga semua request lewat Nginx (intent Q3);
  - langkah saat LB/Cloud Armor dipasang: tambahkan range IP LB GCP ke `TRUSTED_PROXIES`;
  - peringatan: kalau `TRUSTED_PROXIES` kosong di prod, semua user tercatat dengan IP Nginx → rate limit per IP berlaku bersama untuk semua user.
- Komentar `backend/internal/audit/writer.go:3` diperbarui (bukan lagi "chi's RealIP").
- CLAUDE.md Sec 9 (blok env): tambah `TRUSTED_PROXIES=`.

## Non-functional
- **NFR1**: tanpa library baru; stdlib `net/netip` + `net`.
- **NFR2**: overhead per request diabaikan (parse string pendek, daftar prefix kecil, tanpa alokasi besar).
- **NFR3 (keamanan)**: header dari klien yang tidak tepercaya tidak pernah mengubah IP yang dipakai (rate limit + audit). Dibuktikan test.
- **NFR4**: lint `staticcheck SA1019` (`middleware.RealIP`) hilang di kedua backend.

## Tests (dibuat di stage 4, dilacak di `tests.md`)
| ID | Uji | Req |
| --- | --- | --- |
| T1 | Parser `TRUSTED_PROXIES`: kosong, satu IP, CIDR, campuran + spasi, IPv6, entri rusak → error | FR1 |
| T2 | Middleware: RemoteAddr tidak tepercaya + XFF/X-Real-IP palsu → IP = RemoteAddr | FR2, NFR3 |
| T3 | Middleware: dari Nginx tepercaya, XFF `"1.2.3.4, 203.0.113.9"` → `203.0.113.9` (entri kiri palsu diabaikan) | FR2 |
| T4 | Middleware: 2 hop tepercaya (simulasi LB nanti) XFF `"203.0.113.9, 10.1.2.3"` dengan `10.0.0.0/8` tepercaya → `203.0.113.9` | FR2, intent Q1 |
| T5 | Middleware: XFF kosong / semua tepercaya / entri rusak → RemoteAddr; IPv6 `[::1]:port` | FR2 |
| T6 | `extractClientIP`: IPv4:port, `[IPv6]:port`, tanpa port; header XFF diabaikan | FR3 |
| T7 | Handler login: request dengan XFF palsu dari klien tidak tepercaya → `LoginRequest.IP` = RemoteAddr | FR3, NFR3 |
| T8 | `golangci-lint` kedua backend: tidak ada SA1019 | NFR4 |

## Keputusan spec (dikonfirmasi PO 2026-10-12, S1–S4 semua sesuai usulan)
- **S1** ✅: middleware **menimpa `r.RemoteAddr`** (seperti chi `RealIP`, tapi hanya dari proxy tepercaya), bukan menyimpan IP di context. Alasan: log request (`middleware.Logger`) ikut menampilkan IP asli, dan cukup satu sumber kebenaran.
- **S2** ✅: entri `TRUSTED_PROXIES` yang rusak = service gagal start (bukan diabaikan diam-diam).
- **S3** ✅: hanya `X-Forwarded-For` yang dipakai; `X-Real-IP` diabaikan. Nginx kita mengisi keduanya dengan nilai yang sama untuk 1 hop, dan XFF tetap benar saat LB ditambah nanti.
- **S4** ✅: nama env `TRUSTED_PROXIES` (tanpa prefix), dibaca lewat `pkg/config`.

## Out of scope
- Service Python EOD (`python_eod_client_ip_todo.md`, intent Q5).
- Mengubah Nginx config (sudah mengirim XFF dengan benar).
- Memperbaiki nilai `audit_logs.ip` lama.
- Memasang `TrustedRealIP` di `backend-cit` (intent Q4).
- Setup LB / Cloud Armor / firewall VM (tugas infra; hanya didokumentasikan).
