# Performance Test Results — 2026-10-13

> Live runtime baseline of the CMS local stack (CPU, memory, latency). Captured by Kiro on 2026-10-13.
> **Scope caveat:** these numbers prove the app is healthy **at idle and trivial load only**. They are NOT a production capacity test. For the real bottleneck analysis and a load-test plan, see `stress-test.txt` at the repo root.

---

## Environment

| Item | Value |
|------|-------|
| Host | Windows, 16 logical cores, 15.5 GiB RAM |
| Runtime | Docker 29.8.2 / Compose v5.5.1 |
| Git commit | `7eb4fb6` |
| Postgres | External (NOT dockerized) — reachable (backend healthy) |
| Test client | Single PowerShell client (sequential), local loopback |

### Containers under test

| Container | Image | Port | Status |
|-----------|-------|------|--------|
| cms-backend (Go + Chi) | `cms-backend-api` | 8080 | Up, healthy |
| cms-redis | `redis:7-alpine` | 6379 | Up, healthy |
| userportal-vite-fe (nginx) | `companyportal-vite-fe` | 3001 | Up, serves HTTP 200 (healthcheck false-negative, see Findings) |
| vendorportal-vite-fe (nginx) | `vendorportal-vite-fe` | 3002 | Up, serves HTTP 200 (healthcheck false-negative, see Findings) |

> `backend-cit` (port 8081) was not started — it is still a `/health`-only skeleton, not worth profiling.

---

## Results

### Idle resource usage (`docker stats`, no load)

| Container | CPU (idle) | Memory |
|-----------|------------|--------|
| cms-backend | ~0% | 4.7 – 7.5 MiB |
| cms-redis | 0.4 – 2.5% | 6 – 7 MiB |
| userportal-vite-fe | ~0% | 12 – 13 MiB |
| vendorportal-vite-fe | ~0% | 14 – 15 MiB |
| **Total** | **negligible** | **~40 MiB** |

### Latency — 300-request burst to `GET /health`

| Metric | Value |
|--------|-------|
| Throughput | 128 req/s (sequential, single client) |
| p50 | 7.4 ms |
| p95 | 9.3 ms |
| p99 | 11.4 ms |
| max | 26.4 ms |
| Backend CPU during burst | ~0% |

> `/health` does no DB work, no auth, no file parsing — this is a best-case endpoint. It measures framework + container overhead, not real workload cost.

---

## Findings

1. **Not lagging; CPU/memory are effectively nothing at idle and light load.** The Go backend barely registers (~5–7 MiB RAM, 0% CPU). Total stack footprint is ~40 MiB. Health-endpoint latency is single-digit milliseconds.

2. **Frontend "unhealthy" status is a false alarm (healthcheck config bug).** Both nginx containers report `unhealthy`, but both return HTTP 200 to real requests on :3001/:3002. The container healthcheck runs `wget localhost:80` which resolves to IPv6 `[::1]:80` and gets `Connection refused`, while the actual IPv4-mapped port works. Fix later: point the healthcheck at `127.0.0.1` explicitly, or have nginx listen on `[::]:80`. Not a runtime outage.

3. **This baseline does not answer production capacity.** It cannot, because:
   - Only `/health` was exercised — the expensive paths (DSR upload → synchronous Python ETL, dashboard queries, auth under Redis load) were not touched.
   - A single sequential client cannot generate 500 concurrent users or 100 parallel uploads.
   - Local Docker on a 16-core laptop ≠ the production Compute Engine VM + CloudSQL.

---

## Known architectural bottlenecks (from `stress-test.txt`, not re-measured here)

These are where performance will actually bend under load — not CPU/memory:

- **DSR upload is synchronous + external.** The Go handler calls the Python `service_dsr_etl` over blocking HTTP (up to ~25s per phase, two phases), sitting close to the server's 30s `WriteTimeout`. The Python ETL — not Go — is the upload ceiling.
- **pgx pool is untuned.** `pgxpool.New(DatabaseURL)` with no `pool_max_conns`, so default `max(4, NumCPU)`, single primary. Likely the first hard limit under 500 users.
- **Rate limiter fails closed.** If Redis is slow/down, all logins are rejected. DSR rate limit is 10/hr/vendor, so "100 concurrent uploads" must come from ≥10 distinct vendors or 90 correctly return HTTP 429.
- **Read replica not wired.** `DATABASE_REPLICA_URL` is read but unused — heavy dashboard/report reads hit the primary.

---

## Recommended next steps

To turn "feels fine" into "holds at 500 users / 100 uploads":

1. Install **k6** and write scripts for the two NFR scenarios: 500 mixed-profile users (~70% read / 20% light action / 10% upload) and 100 concurrent DSR uploads spread across ≥10 vendors. Pair with a `docker stats` sampler to capture CPU/mem under real load.
2. Profile real endpoints (login, a dashboard list, a DSR dry-run) — requires valid dev credentials.
3. Supply three inputs so pass/fail thresholds are meaningful (not guesses):
   - production VM spec (vCPU / RAM),
   - CloudSQL max-connections tier,
   - typical DSR file row count.

---

## How to reproduce

```powershell
# 1. Start the stack (backend owns the shared network + Redis — start FIRST)
cd backend;  docker compose up -d
cd ..\frontend; docker compose up -d

# 2. Confirm backend connected to external Postgres (healthy = DB reachable)
docker ps --format "{{.Names}} | {{.Status}}"

# 3. Idle resource snapshot
docker stats --no-stream --format "{{.Name}} | CPU {{.CPUPerc}} | MEM {{.MemUsage}} ({{.MemPerc}})"

# 4. Latency burst against /health (sequential; Start-Job output is unreliable here)
$lat = New-Object System.Collections.ArrayList
for ($i=0; $i -lt 300; $i++) {
  $t = Measure-Command { try { Invoke-WebRequest http://localhost:8080/health -UseBasicParsing -TimeoutSec 10 | Out-Null } catch {} }
  [void]$lat.Add($t.TotalMilliseconds)
}
$s = $lat | Sort-Object; $n = $s.Count
"p50=$([math]::Round($s[[int]($n*0.50)],1)) p95=$([math]::Round($s[[int]($n*0.95)],1)) p99=$([math]::Round($s[[int]($n*0.99)],1))"
```

> Note: `docker compose up` prints progress to stderr, so PowerShell may flag a successful start as a `NativeCommandError` with exit code 1 — verify real state with `docker ps`, not the exit code.

---

## Ringkasan ELI5 (Explain Like I'm 5)

Bayangkan aplikasi CMS itu seperti **sebuah toko**. Kita cek kesehatan tokonya.

### Yang kita cek
Kita nyalakan tokonya (semua container Docker), lalu ketuk pintunya 300 kali dengan cepat dan lihat: capek nggak? lambat nggak? makan banyak tenaga nggak?

### Hasilnya: tokonya sehat dan santai
- **Tenaga (CPU):** Hampir nggak keluar keringat. Seperti disuruh angkat sendok — enteng banget.
- **Memori (RAM):** Seluruh toko cuma pakai ~40 MB. Kecil sekali, seperti satu foto di HP.
- **Kecepatan:** Setiap ketukan pintu dijawab dalam ~7 milidetik. Lebih cepat dari kedipan mata. **Jadi: nggak lemot sama sekali.**

### Tapi ada "tapi"-nya yang penting
Kita cuma ngetuk **pintu depan yang paling gampang** (`/health`) — pintu yang nggak perlu buka brankas, nggak perlu cek KTP, nggak perlu angkat barang berat.

Pekerjaan berat yang sesungguhnya belum dites:
- 🏋️ **Upload DSR** — seperti titip barang ke tukang hitung di sebelah (service Python). Tokonya harus **nunggu diam** sampai tukangnya selesai (bisa ~25 detik). Kalau banyak orang titip sekaligus, antreannya numpuk di sini.
- 🚪 **Pintu masuk pegawai (login)** — kalau penjaga pintunya (Redis) sakit, **semua orang nggak bisa masuk**, bukan cuma sebagian.
- 🪣 **Pipa air (koneksi database)** — pipanya masih setelan pabrik, kecil. Kalau 500 orang buka keran bareng, airnya bisa seret.

### Hal lucu yang kita temukan
Papan "BUKA/TUTUP" di dua toko frontend nunjukkin **"TUTUP" (unhealthy)**, padahal pintunya **benar-benar terbuka dan melayani pelanggan**. Ternyata cuma alat pengeceknya yang salah lihat (ngecek alamat IPv6, padahal tokonya pakai IPv4). Bukan toko rusak, cuma papannya yang keliru.

### Kesimpulan satu kalimat
> Aplikasinya **ringan dan ngebut saat sepi**, tapi kita belum tahu kuat nggak kalau **ramai banget** — karena bagian beratnya (upload, login massal, database) belum dicoba dengan beban sungguhan.

### Langkah selanjutnya
Pakai alat namanya **k6** untuk mendatangkan "500 pelanggan palsu" sekaligus, lalu lihat tokonya goyang di mana. Untuk itu dibutuhkan 3 info: spesifikasi server produksi, batas koneksi CloudSQL, dan ukuran file DSR yang biasa dipakai.
