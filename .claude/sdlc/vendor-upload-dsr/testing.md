# Testing flow: vendor upload DSR (local dev, Windows + Docker Desktop + Git Bash)

Urutan lengkap dari start service sampai file DSR berhasil diupload & confirm.
Ditulis dari hasil debug sesi ini — semua gotcha di bawah adalah masalah nyata yang
ditemukan, bukan langkah preventif teoretis.

## 0. Prasyarat sekali-jalan

- Postgres jalan di `localhost:5432`, db `cms` sudah dimigrasi (`backend/migrations/*`).
- Docker network `cms-net` sudah dibuat: `docker network create cms-net` (kalau belum ada).
- Image `cms-backend-api` sudah dibuild dari `backend/Dockerfile` (context repo root, untuk resolve `pkg/` via `go.work`).
- Python 3 + pip tersedia untuk `service_dsr_etl` / `eod_retry_scheduler` dan `backend_python/dsr` (tidak di-docker-kan, jalan langsung di host).

`retry_scheduler` sudah **dipecah jadi dua service independen** (kode generik yang dipakai
bersama dipindah ke `backend_python/lib/`, mengikuti pola `backend/`+`backend-cit/` berbagi `pkg/`
di repo ini):
- **`backend_python/service_dsr_etl/`** — hanya `dsr`, port **8090** (yang dipanggil Go via
  `DSR_RETRY_SCHEDULER_BASE_URL`). Ini yang wajib jalan untuk testing upload DSR.
- **`backend_python/eod_retry_backend_python/`** — `dmaa`, `itm_cashpos`, `itm_replenish`, port **8091**.
  Tidak terlibat di alur upload DSR, tapi jalankan juga kalau mau test EOD retry scan untuk
  3 file type lain.

## 1. Start `service_dsr_etl` (host, port 8090)

HARUS dijalankan dari **repo root**, bukan dari dalam foldernya sendiri — semua path relatif
di `config.py` (`FTP_DATA`, dst) dan di `backend_python/dsr/dsr_etl.py`
(`ROOT_DIR = Path(__file__).resolve().parents[2]`) diasumsikan repo-root-relative. Jalankan
cwd di tempat lain → path ETL script/​FTP_DATA salah resolve (pernah kejadian: subprocess
`dsr_etl.py` gagal `NotADirectoryError` karena path ganda-nested).

Install deps sekali:
```bash
python -m pip install -r backend_python/service_dsr_etl/requirements.txt -r backend_python/dsr/requirements.txt
```

Jalankan dari repo root, env langsung di-passthrough (bukan lewat `.env` file — `.env` file
discovery pydantic-settings itu relatif ke CWD, gampang salah kalau CWD berubah). `cd
/path/to/CMS2` di bawah ini adalah placeholder — ganti dengan path repo asli, atau skip kalau
terminal kamu sudah berada di root repo.

**Git Bash:**
```bash
cd /path/to/CMS2
DSR_ETL_DATABASE_URL="postgresql://postgres:1818@localhost:5432/cms?sslmode=disable" \
DSR_ETL_AUTH_SECRET="change-me" \
DSR_ETL_AUTH_MODE="api_key" \
PYTHONPATH="backend_python" \
python -m uvicorn service_dsr_etl.main:app --host 0.0.0.0 --port 8090
```

**PowerShell** (syntax env var beda dari Bash — `VAR=val \` tidak valid di sini, pakai
`$env:VAR = "val"` satu baris per statement, TANPA line-continuation backslash):
```powershell
$env:DSR_ETL_DATABASE_URL = "postgresql://postgres:1818@localhost:5432/cms?sslmode=disable"
$env:DSR_ETL_AUTH_SECRET = "change-me"
$env:DSR_ETL_AUTH_MODE = "api_key"
$env:PYTHONPATH = "backend_python"
python -m uvicorn service_dsr_etl.main:app --host 0.0.0.0 --port 8090
```

(Kalau juga mau jalankan `eod_retry_scheduler` untuk dmaa/itm_cashpos/itm_replenish, sama
persis polanya tapi prefix env `RETRY_`, module `eod_retry_scheduler.main:app`, port 8091.)

**Kenapa `?sslmode=disable` wajib**: `asyncpg` (dipakai kedua service) tanpa `sslmode`
eksplisit gagal handshake ke Postgres lokal ini dengan error yang menyesatkan
(`ConnectionDoesNotExistError: connection was closed in the middle of operation`, atau di
Windows muncul sebagai `WinError 64`/`WinError 10054`). `psycopg` (dipakai `backend_python/dsr`)
tidak kena karena default-nya `sslmode=prefer`. Root cause-nya di server Postgres lokal,
bukan di kode — fix-nya di connection string, bukan di event loop (jangan buang waktu
utak-atik `asyncio` event loop policy, itu bukan penyebabnya meski traceback-nya kelihatan
seperti masalah Proactor loop).

**`DSR_ETL_AUTH_SECRET`** harus sama persis dengan isi `DSR_RETRY_SCHEDULER_AUTH` di
`backend/.env` (minus prefix `Bearer `).

Cek sehat:
```bash
curl http://localhost:8090/health
# {"status":"success","data":{...,"database":"connected","filesystem":"accessible"}}
```
Kalau `filesystem: not accessible` → folder `FTP_DATA/DSR` belum ada di repo root.
`mkdir -p FTP_DATA/DSR`.

## 2. Start backend (`cms-backend`, Docker, port 8080)

Backend sekarang punya `backend/docker-compose.yaml` (memiliki service `backend` + `redis`,
owns network `cms-net`). Volume mount `FTP_DATA` sudah didefinisikan di compose file ini
(`../FTP_DATA:/app/FTP_DATA`) supaya file yang ditulis backend (di dalam container) bisa
dibaca `service_dsr_etl` (di host) — **tidak perlu lagi `docker run` manual atau
`MSYS_NO_PATHCONV`**, compose tidak kena masalah path-mangling Git Bash yang dulu ada di
`docker run`.

```bash
cd backend
docker compose up -d --build
```

Kalau ada container `cms-backend` lama sisa dari `docker run` manual sebelumnya, hapus dulu
sebelum `docker compose up` (nama container bentrok):
```bash
docker rm -f cms-backend
```

Verifikasi mount (opsional, sudah otomatis benar via compose):
```bash
docker inspect cms-backend --format '{{json .Mounts}}'
# Source harus <repo>/FTP_DATA, Destination harus persis "/app/FTP_DATA"
```

Cek sehat:
```bash
curl http://localhost:8080/health
```

Env wajib di `backend/.env` (lihat `backend/.env.example` untuk template):
```
DSR_UPLOAD_DIR=FTP_DATA/DSR
DSR_RETRY_SCHEDULER_BASE_URL=http://host.docker.internal:8090
DSR_RETRY_SCHEDULER_AUTH=Bearer change-me
```

## 3. Start frontend (Vendor Portal, Docker, port 3002)

Sudah jalan lewat `frontend/docker-compose.yml`:
```bash
cd frontend && docker compose up -d vendorportal
```
Nginx di container ini sudah proxy `/api/` → `http://host.docker.internal:8080` (lihat
`frontend/VendorPortal-Vite/nginx.conf`) — tidak perlu setup tambahan.

## 4. Login & upload — manual lewat browser

1. Buka `http://localhost:3002`.
2. Login pakai akun vendor seed: username `vendor.bijak`, password `password123`
   (bcrypt hash-nya ada di `backend/migrations/015_seed_vendor_bijak_user.sql`; vendor
   `BIJAK`, branch "Bijak Jakarta").
3. Ke DSR Menu → **Add DSR** → pilih file `.xls`/`.xlsx` (contoh ada di `DSR_DATA/`).
4. Tunggu preview dry-run muncul (tidak ada yang masuk DB di titik ini) → review →
   **Konfirmasi & Simpan**.
5. Sukses = badge hijau "Data DSR berhasil disimpan".

Kalau sesi browser sudah lama terbuka sebelum service di atas di-restart, **logout lalu
login ulang dulu** — access token yang lama tidak otomatis invalid tapi kalau backend
sempat direstart dengan `.env`/wiring baru, token lama masih valid secara JWT tapi
percuma kalau service downstream-nya (`service_dsr_etl`) baru saja hidup.

## 5. Login & upload — scripted (tanpa buka browser, buat verifikasi cepat)

```python
import requests

s = requests.Session()
r = s.post("http://localhost:8080/api/v1/auth/login",
           json={"username": "vendor.bijak", "password": "password123"},
           headers={"X-Portal-Type": "vendor"})
token = r.json()["access_token"]
headers = {"Authorization": f"Bearer {token}"}

path = "DSR_DATA/Laporan Saldo Harian DSR CIMB NIAGA BIJAK JAKARTA Tanggal 15 Juli 2026.xlsx"
with open(path, "rb") as f:
    files = {"file": (path.split("/")[-1], f,
              "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")}
    dry = s.post("http://localhost:8080/api/v1/dsr/uploads", files=files, headers=headers).json()

commit = s.post("http://localhost:8080/api/v1/dsr/uploads/confirm",
                 json={"staged_filename": dry["staged_filename"], "checksum": dry["checksum"]},
                 headers=headers).json()
print(commit)  # daily.status / rencana_isi.status harus "completed"
```

## Troubleshooting cepat (masalah yang sudah pernah kejadian)

| Gejala | Penyebab | Cek |
|---|---|---|
| FE: modal upload langsung "Unauthorized" | Access token expired & refresh gagal (biasa kalau sesi browser dibuka lama sebelum service direstart) | Logout → login ulang. Kalau masih 401 langsung setelah login baru, baru curigai `RequireAuth`/`JWT_SECRET`. |
| `dry_run_failed: connection refused` ke `host.docker.internal:8090` | `service_dsr_etl` belum jalan | `curl http://localhost:8090/health` |
| `dry_run_failed: retry_scheduler returned status 500` | `dsr_etl.py` subprocess crash (bukan auth/network) — cek stdout/stderr di `service_dsr_etl`'s uvicorn log | Log biasanya nunjuk file/parse error yang jelas |
| `dry_run_failed: File not found: .../FTP_DATA/DSR/<nama>` | Backend container dan `service_dsr_etl` host TIDAK share filesystem yang sama (mount belum ada / rusak) | `docker inspect cms-backend --format '{{json .Mounts}}'` — pastikan Source/Destination benar. Kalau pakai Git Bash, ingat `MSYS_NO_PATHCONV=1`. |
| `service_dsr_etl` gagal start: `asyncpg...ConnectionDoesNotExistError` / `WinError 64` / `WinError 10054` | Postgres lokal reject asyncpg's default (no-SSL-negotiation) handshake | Tambah `?sslmode=disable` di `DSR_ETL_DATABASE_URL` |
| `service_dsr_etl` health 503 "Filesystem access failure" | `FTP_DATA/DSR` belum ada di repo root | `mkdir -p FTP_DATA/DSR` |
| Port 8090/8091 "address already in use" tapi curl ke situ balikin data lama/aneh | Proses lama belum benar-benar mati (`pkill` pattern-nya tidak match di Windows) | `netstat -ano \| grep <port>` untuk cari PID asli, lalu `powershell -Command "Stop-Process -Id <pid> -Force"` |
