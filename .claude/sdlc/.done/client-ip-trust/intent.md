# Intent: IP client tidak bisa dipalsukan lewat `X-Forwarded-For` (QC F13)

Author: user (product owner), ditulis bersama Claude. Status: **accepted 2026-10-12** (PO, Q1–Q5 dijawab di bawah).
Stage: 1 Plan. Stage berikut: `spec.md`.
Asal: Backend QC 2026-10-09, item F13 (`backend_QC_fixes.md`), ditemukan saat upgrade chi (F12).
Menyentuh Sec 4 #7: **auth** (rate limit login) + **audit** (kolom `ip` di `audit_logs`, Sec 5). Tidak ada migrasi skema.

## Problem
Kata user (2026-10-12): *"lanjut F13"*. Topologi produksi menurut user: **Nginx → backend** (1 proxy tepercaya, tanpa load balancer di depan).

Kondisi sekarang (dicek di kode, 2026-10-12):
- `extractClientIP` (`backend/internal/handler/auth_handler.go:337`) mengambil IP **paling kiri** dari `X-Forwarded-For` tanpa memeriksa apakah request datang dari proxy tepercaya. Bagian paling kiri adalah nilai yang dikirim browser/client sendiri, jadi bebas dipalsukan. Fungsi ini dipanggil di 60 tempat di `internal/handler`.
- `middleware.RealIP` chi dipasang di `backend/cmd/api/main.go:100` dan `backend-cit/cmd/api/main.go:63`. Ia menimpa `r.RemoteAddr` dengan isi header `True-Client-IP` / `X-Real-IP` / `X-Forwarded-For` dari siapa pun. Middleware ini sudah ditandai deprecated oleh chi (staticcheck SA1019, satu-satunya temuan lint non-gocyclo).
- Nginx kedua frontend (`frontend/*/nginx.conf`) sudah mengirim `X-Real-IP $remote_addr` dan `X-Forwarded-For $proxy_add_x_forwarded_for` ke backend (`host.docker.internal:8080`). Nilai `$remote_addr` dari Nginx itulah IP asli.

Dampak:
1. **Rate limit login per (email, IP)** (`backend/internal/auth/service.go:85,127`): attacker bisa memutar IP palsu di setiap percobaan untuk menghindari limit per IP. Lockout per akun lokal tetap berlaku, jadi brute force ke satu akun lokal tetap dibatasi, tapi password spraying ke banyak akun tidak.
2. **Kolom `ip` di `audit_logs`** untuk semua aksi admin/master data/vendor request bisa diisi IP apa saja. Ini melemahkan audit trail (Sec 5: audit wajib mencatat IP).

## Proposed outcome
1. Backend hanya memercayai header IP bila koneksi langsung (`RemoteAddr`) berasal dari **proxy tepercaya** (Nginx). Selain itu, IP = `RemoteAddr`, header diabaikan.
2. Dari proxy tepercaya, IP client diambil dari **kanan** rantai `X-Forwarded-For` (melewati hop tepercaya), bukan paling kiri. Untuk topologi 1 hop, hasilnya sama dengan `X-Real-IP` yang ditulis Nginx.
3. `middleware.RealIP` dihapus dari kedua backend; lint SA1019 hilang.
4. Satu tempat saja yang menentukan IP (`extractClientIP`), sehingga rate limit login dan `audit_logs.ip` otomatis ikut benar tanpa mengubah 60 pemanggilnya.
5. Daftar proxy tepercaya diatur lewat env (bukan hardcode), tercatat di `.env.example` dan `docs/deployment.md`.

## Affected users and systems
- Users: tidak ada perubahan tampilan. Semua user yang login dan semua aksi yang diaudit mendapat IP yang benar.
- Systems: `backend/internal/handler/auth_handler.go` (`extractClientIP`), `backend/cmd/api/main.go`, `backend-cit/cmd/api/main.go`, config env (`pkg/config` bila dipakai bersama), `backend/internal/audit/writer.go:3` (komentar), `.env.example`, `.claude/docs/deployment.md`. Nginx config hanya dibaca, kemungkinan tidak diubah.

## Constraints
- Stack tetap; tidak ada library baru (stdlib `net/netip` cukup).
- Response shape tidak berubah. Tidak ada migrasi skema.
- Default aman: bila env proxy tepercaya kosong, header IP **tidak** dipercaya (pakai `RemoteAddr`).
- Dev lokal (Vite proxy :5173/:5174 → :8080, Nginx container via `host.docker.internal`) harus tetap jalan; IP yang tercatat di dev boleh berupa IP proxy/gateway.
- Manual browser verification = tugas user (GR#10).

## Open questions (untuk PO / spec)
1. **Cloud Armor**: `docs/deployment.md` #11 mencantumkan Cloud Armor (WAF, terutama vendor portal). Cloud Armor hanya bisa dipasang di belakang GCP Load Balancer, yang berarti ada hop tambahan di depan Nginx. Apakah Cloud Armor/LB memang tidak dipakai (topologi tetap 1 hop), atau direncanakan nanti? Kalau nanti, cukup dengan menambah entri di env tanpa ubah kode.
2. **Bentuk config**: daftar CIDR proxy tepercaya (mis. `TRUSTED_PROXIES=172.17.0.0/16`, alamat Nginx/gateway Docker di VM) atau jumlah hop (`TRUSTED_PROXY_HOPS=1`)? Saran Claude: **CIDR**, karena jumlah hop saja tetap bisa ditipu bila port 8080 bisa diakses langsung tanpa lewat Nginx.
3. **Port 8080 di VM produksi**: apakah hanya bisa dijangkau dari Nginx (firewall/bind ke localhost), atau terbuka di jaringan internal? Menentukan apakah CIDR cukup sempit.
4. **`backend-cit`**: cukup hapus `RealIP` di sana (belum ada endpoint yang memakai IP), atau perlu helper yang sama sekarang? Saran Claude: hapus saja, helper menyusul saat `backend-exq` butuh IP.
5. **Python EOD service**: `eod_retry_scheduler` dan `service_dsr_etl` (`routers/retry.py:19`) mencatat `request.client.host` saat retry manual. Di belakang Nginx itu selalu IP Nginx (tidak bisa dipalsukan, tapi juga bukan IP user). Ikut diperbaiki di sini, atau di luar scope (Go saja)? Saran Claude: **di luar scope**, dicatat sebagai catatan.

## Jawaban PO (2026-10-12)
1. **Cloud Armor / LB**: direncanakan nanti. Sekarang topologi Nginx → backend (1 hop). Saat LB/Cloud Armor dipasang, cukup tambah range alamat LB ke config proxy tepercaya, tanpa ubah kode. Spec mencatat langkah itu.
2. **Bentuk config**: daftar **CIDR** proxy tepercaya (mis. `TRUSTED_PROXIES`). Header hanya dipercaya bila `RemoteAddr` ada di daftar; hop tepercaya di kanan `X-Forwarded-For` dilewati. Kosong = header diabaikan.
3. **Port 8080 produksi**: belum diketahui. Spec menganggap port bisa terbuka: CIDR dibuat sempit (hanya Nginx/gateway Docker) + catatan deploy di `docs/deployment.md` untuk menutup 8080 dari luar (defense in depth).
4. **`backend-cit`**: cukup hapus `middleware.RealIP`. Helper IP tepercaya dipakai di sana nanti saat `backend-exq` butuh IP.
5. **Python EOD**: di luar scope. Dicatat terpisah di `python_eod_client_ip_todo.md` (root repo) supaya tidak lupa.
