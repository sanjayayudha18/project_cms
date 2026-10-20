# Intent: Kontrak OpenAPI untuk API backend (BE-10)

Author: user (product owner), ditulis bersama Claude. Status: **accepted 2026-10-19** (lihat "Jawaban PO").
Stage: 1 Plan selesai. Stage berikut: [`spec.md`](./spec.md).
Asal: backlog [`.claude/BE_debt.md`](../../BE_debt.md) BE-10, sebelumnya ditunda di [`be-tech-debt`](../be-tech-debt/intent.md) Q8 / FR-G3 (`b007b64`, 2026-10-19: *"ditunda; mulai dari modul baru berikutnya"*). User membuka kembali item ini 2026-10-19 dan memutuskan mengerjakannya sebagai AI-DLC sendiri.
Menyentuh Sec 4 #7: **tidak langsung** — ini dokumentasi kontrak, bukan perubahan perilaku. Endpoint auth (login/refresh/me) dan uang (nominal sebagai string desimal, QC F7) ikut terdokumentasi, jadi kontraknya harus persis sama dengan kode; tidak ada perubahan response shape atau endpoint.

## Problem
Kata user (BE_debt.md, review arsitektur 2026-10-12): *"Tanpa kontrak OpenAPI — 138+ endpoint; tipe FE ditulis manual → drift"*. Perbaikan yang dicatat: *"Spec OpenAPI untuk endpoint yang dipakai FE (mulai dari modul baru); generate tipe FE bila disetujui."*

Kondisi saat ini (dicek di kode 2026-10-19):
- ATM `backend/` mendaftarkan ±164 route (`r.Get/Post/Put/Patch/Delete`) di `cmd/api/main.go` + 32 file `internal/handler/*_handler.go`. Response = flat JSON (Sec 3/5), bukan envelope `pkg/response`. Tidak ada dokumen kontrak; satu-satunya sumber kebenaran adalah kode handler.
- Dua SPA menulis tipe request/response sendiri secara manual: CompanyPortal 17 file `features/*/types.ts`, VendorPortal 12 file + `src/lib/types.ts`. Client `src/lib/api/client.ts` di kedua portal mengembalikan `ApiResponse<T>` dengan `T` yang di-assert tangan — tidak ada yang mengecek bahwa `T` cocok dengan JSON yang sebenarnya dikirim Go.
- Akibatnya drift hanya ketahuan saat runtime / test manual. Contoh terbaru: bug prefix `/api/v1` ganda di EOD Monitoring + vendor parties (`486a950`) dan perubahan kontrak uang → string desimal (QC F7) yang harus dikirim bersamaan di BE dan kedua FE.
- `backend-cit` (calon `backend-exq`) belum punya endpoint bisnis; ia akan memakai envelope `pkg/response`.

## Proposed outcome
1. Ada kontrak API (OpenAPI) yang menjadi rujukan bersama BE ↔ FE untuk endpoint yang masuk scope.
2. Kontrak itu tidak bisa diam-diam berbeda dari kode: ada pengecekan otomatis yang gagal bila response handler tidak cocok dengan spec (drift ketahuan di test/CI, bukan di browser).
3. (Bila disetujui) tipe FE untuk endpoint dalam scope tidak lagi ditulis manual, atau setidaknya dicek terhadap spec.
4. Fitur/modul baru ke depan menambahkan endpoint-nya ke spec sebagai bagian dari Definition of Done.

## Affected users and systems
- **Users**: tidak ada perubahan tampilan atau alur.
- **Systems**: `backend/` (spec + test kontrak; mungkin route `/openapi.json`), `frontend/CompanyPortal-Vite` + `frontend/VendorPortal-Vite` (`lib/api`, `features/*/types.ts`) bila tipe di-generate, pipeline CI (`.github/workflows/ci.yml` hari ini; Jenkins direncanakan, `ci-jenkins/`), CLAUDE.md Sec 11 DoD bila "update spec" jadi wajib.

## Constraints
- Stack tetap (CLAUDE.md Sec 2). Library Go baru (generator/validator OpenAPI) dan tool dev FE baru (generator tipe) **butuh persetujuan** (Sec 4 #4). Stdlib / yang sudah terpasang diutamakan.
- Tidak ada perubahan response shape, status code, atau path. ATM `backend/` tetap flat JSON; `pkg/response` envelope hanya untuk `backend-cit`/`backend-exq` (Sec 3/5). Spec mendokumentasikan apa adanya; bila ditemukan inkonsistensi antar handler, dicatat, bukan diperbaiki diam-diam.
- Uang di API: spec mengikuti tipe yang benar-benar dikirim handler (string desimal untuk endpoint QC F7, `int64` rupiah penuh di endpoint pilot — lihat koreksi di "Jawaban PO"). Tidak boleh `number` float.
- Small diffs, satu concern per perubahan; dikerjakan per slice.
- Checkout dipakai bersama session lain; perubahan FE tidak boleh bentrok dengan pekerjaan FE yang sedang jalan (mis. FE-12 sisa).

## Open questions (untuk PO — belum dijawab)
1. **Scope**
   - (a) Satu modul dulu (mis. vendor-request / modul baru berikutnya) sebagai pilot, lalu bertahap. *Tradeoff*: cepat terbukti, pola teruji di skala kecil; sisa endpoint tetap tanpa kontrak untuk sementara.
   - (b) Semua ±164 route sekaligus. *Tradeoff*: drift tertutup menyeluruh; pekerjaan besar (berminggu-minggu), risiko spec salah tinggi tanpa pola yang teruji.
   - (c) Hanya endpoint yang benar-benar dipanggil FE (saran di BE_debt). *Tradeoff*: fokus pada sumber drift; perlu inventaris pemanggilan FE dulu.
2. **Cara membuat spec**
   - (a) Ditulis manual, YAML di repo. *Tradeoff*: tanpa dependency; risiko spec ketinggalan kode → harus dijaga oleh test kontrak (Q4).
   - (b) Di-generate dari kode Go (anotasi komentar mis. swag, atau library spec-from-code). *Tradeoff*: lebih dekat ke kode; **dependency baru**, anotasi menambah noise di handler, dan hasil generator sering tetap harus dikoreksi tangan.
3. **Tipe FE**
   - (a) Tetap manual. *Tradeoff*: nol tool baru; drift FE ↔ spec masih mungkin.
   - (b) Generate dari spec (mis. `openapi-typescript`, tool dev baru) untuk satu portal dulu. *Tradeoff*: drift FE hilang untuk scope itu; tambah langkah generate + file generated di repo.
   - (c) Generate untuk kedua portal. *Tradeoff*: konsisten; migrasi tipe lebih banyak dan bersinggungan dengan pekerjaan FE lain.
4. **Validasi drift**
   - (a) Test kontrak di Go: response handler (httptest) divalidasi terhadap spec. *Tradeoff*: drift BE ketahuan di `go test`; butuh validator schema (library baru, atau validator kecil buatan sendiri untuk subset OpenAPI yang dipakai).
   - (b) Cek di CI saja (lint spec + diff tipe generated). *Tradeoff*: ringan; tidak membuktikan handler cocok dengan spec.
   - (c) Keduanya.
5. **Lokasi, versioning, penyajian**
   - Lokasi: `backend/api/openapi.yaml` (dekat kode) vs `docs/` / root repo. *Tradeoff*: dekat kode lebih mudah di-embed + di-test; root lebih mudah dibaca kedua FE.
   - Versioning: satu file mengikuti `/api/v1` + versi di `info.version`, atau tanpa versi selain git. *Tradeoff*: versi eksplisit berguna bila ada konsumen luar; saat ini konsumen hanya dua SPA internal.
   - Penyajian: disajikan backend (mis. `GET /openapi.json`; dengan auth atau tidak?) vs hanya di repo. *Tradeoff*: disajikan memudahkan tooling/QA; tanpa auth membuka enumerasi endpoint.
6. **DoD ke depan**: apakah "endpoint baru wajib masuk spec + lulus test kontrak" ditambahkan ke CLAUDE.md Sec 11? *Tradeoff*: mencegah drift baru; menambah beban tiap fitur.
7. **`backend-cit` / `backend-exq`**: masuk scope sekarang (belum ada endpoint) atau menunggu rename + isi? *Tradeoff*: sekarang = pola envelope ikut dirancang; menunggu = tidak mendokumentasikan sesuatu yang belum ada.

## Saran Claude (2026-10-19, diminta PO; belum diputuskan)
1. **(a) pilot satu alur**: Vendor Request: endpoint `/vendor-requests*` (CompanyPortal) + `/vendor/replenish-orders*` (VendorPortal). Alur ini dipakai kedua portal, membawa uang string, dan paling sering berubah. Modul lain menyusul saat disentuh (lihat Q6).
2. **(a) YAML manual**. Tanpa dependency dan tanpa anotasi di handler. Risiko spec tertinggal ditutup oleh Q4.
3. **(b) generate untuk CompanyPortal dulu** dengan `openapi-typescript` (devDependency, hanya `.d.ts`, nol runtime). Tipe manual modul pilot diganti alias ke tipe generated. Dimulai setelah sisa FE-12 (`VendorRequestCreate*`) ter-commit agar tidak bentrok. VendorPortal menyusul bila pilot lancar.
4. **(a) test kontrak Go** memakai `getkin/kin-openapi` (dependency baru, hanya dipakai test). Validator JSON Schema buatan sendiri rawan salah. Test berjalan di `go test`, jadi CI ikut menjalankannya tanpa job baru.
5. File di `backend/api/openapi.yaml`, `info.version: v1`, riwayat lewat git. **Hanya di repo, tidak disajikan** (YAGNI, tanpa permukaan enumerasi endpoint). Endpoint `/openapi.json` ditambah bila nanti ada konsumen.
6. **Ya**: endpoint baru atau yang berubah wajib masuk spec + lulus test kontrak, ditambahkan ke Sec 11 DoD. Dengan begitu cakupan tumbuh sendiri tanpa proyek "spec semua endpoint".
7. **Tunggu** rename + isi `backend-exq`. Bila sudah ada, pola envelope ditambahkan di spec terpisah.

## Jawaban PO (2026-10-19)
User: *"Setuju semua saran Claude Q1–Q7 (termasuk dependency openapi-typescript dan kin-openapi), intent diterima, lanjut spec."* Artinya Q1–Q7 = saran di atas. Dependency yang disetujui: `github.com/getkin/kin-openapi` (Go, hanya test) dan `openapi-typescript` (devDependency CompanyPortal).

Koreksi constraint uang setelah dicek di kode (2026-10-19): endpoint pilot mengirim nominal sebagai `int64` rupiah penuh (JSON integer), bukan string desimal; QC F7 hanya mengubah endpoint ATM portal + DSR. Spec mendokumentasikannya apa adanya (`integer`, `format: int64`); tidak diubah di sini.

## Stages 2–6
- `spec.md` — stub, ditulis setelah intent diterima.
- `plan.md`, `tests.md`, `review.md`, `incidents.md` — stub.
