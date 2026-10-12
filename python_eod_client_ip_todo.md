# TODO: IP client di service Python EOD (di luar scope QC F13)

Dicatat 2026-10-12 atas permintaan user saat menyusun F13 (`.claude/sdlc/client-ip-trust/intent.md`, Q5): *"di luar scope, tapi tolong di catat ke file .md baru, agar tidak lupa"*.

## Kondisi
- `backend_python/eod_retry_scheduler/routers/retry.py:19` dan `backend_python/service_dsr_etl/routers/retry.py:19` mencatat `ip = request.client.host` saat retry manual (`process_manual_retry(..., ip=ip)` → audit retry).
- Kedua service dijalankan lewat `uvicorn.run(...)` di `*/run.py` tanpa `proxy_headers` / `forwarded_allow_ips`.
- `eod_retry_scheduler` diakses browser lewat Nginx CompanyPortal (`/api/eod/*` → `:8091`), jadi `request.client.host` selalu **IP Nginx/gateway Docker**, bukan IP user. Nilainya tidak bisa dipalsukan, tapi tidak berguna untuk audit.
- `service_dsr_etl` (:8090) tidak diekspos ke browser (dipanggil Go backend), jadi IP-nya adalah IP backend Go. Perlu dicek apakah Go meneruskan IP user ke sana.

## Arah perbaikan (belum diputuskan)
- Jalankan uvicorn dengan `proxy_headers=True` dan `forwarded_allow_ips=<CIDR Nginx>` (CIDR yang sama dengan `TRUSTED_PROXIES` di F13), sehingga `request.client.host` = IP user hanya bila request datang dari Nginx tepercaya. Default kosong = tetap IP koneksi langsung.
- Tambah config ke `.env.example` service Python (`RETRY_*` / `DSR_ETL_*`) dan catat di `.claude/docs/deployment.md` bersama config F13.
- Test: header `X-Forwarded-For` palsu dari klien non-tepercaya tidak mengubah IP yang tercatat.

## Kapan
Setelah F13 (Go) selesai, supaya CIDR dan catatan deploy-nya sama. Tanyakan ke user: AI-DLC atau cukup `bugfixes.md` (CLAUDE.md Sec 4a rule 0).
