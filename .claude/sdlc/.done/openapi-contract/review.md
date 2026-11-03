# Review: Kontrak OpenAPI BE ↔ FE (BE-10)

Status: **2026-10-19** (Stage 5) — R1 **fixed**, tidak ada Important terbuka. Reviewer: Claude (pass bugs / security / compliance atas `d1e7082` + `062b0d9`, input `spec.md`, `plan.md`, `tests.md`). Code owner merge, bukan Claude.

## Important

| # | Area | Temuan | Usulan perbaikan |
|---|---|---|---|
| R1 | Bug (test) / DoD | T3 hanya memvalidasi kasus yang ada di `contractCases()`, tetapi tidak ada yang mengecek bahwa **setiap** operasi di spec punya minimal satu kasus. Operasi baru bisa lulus T2 (route == spec) tanpa bentuk response-nya pernah divalidasi. Akibatnya butir DoD Sec 11 ("lulus test kontrak") bisa terpenuhi padahal kosong. | Di akhir `TestOpenAPIContract_Responses`, kumpulkan operasi yang cocok lewat `router.FindRoute`, lalu gagal untuk setiap operasi spec yang tidak tersentuh. |

## Perbaikan (2026-10-19, belum di-commit)
| # | Perbaikan | Bukti |
|---|---|---|
| R1 | `openapi_contract_test.go`: map `covered` diisi dari `FindRoute` per kasus; operasi spec tanpa kasus → `no response case for <METHOD> <path>` | Hijau: 26/26 operasi sudah tercakup. RED: probe `GET /vendor-requests/zz-red` ditambahkan sementara ke spec → FAIL dengan pesan di atas; spec dipulihkan dari salinan (tanpa diff). `golangci-lint` handler 0 issue. |

## Nits
- N1: T5 (`schema.d.ts` basi) hanya jalan di `pnpm test` lokal karena `ci.yml` belum punya job frontend. Ini keputusan S4; pindahkan ke CI saat job FE/Jenkins ada (FE-06 / `ci-jenkins`).
- N2: Daftar status Vendor Request kini ada di empat tempat: CHECK migrasi 029, Go `vendor_request_actions.go`, enum spec `VendorRequestStatus`, dan `VENDOR_REQUEST_STATUSES` FE. Status baru yang lupa ditambah ke spec hanya tertangkap T3 bila ada fixture yang memakainya.
- N3: Temuan F1–F11 (`tests.md`) dicatat, bukan diperbaiki. Yang paling berdampak ke FE: F1 tanggal vendor orders berupa timestamp, F2 uang campur integer/string desimal, F3/F5 kode error tidak seragam. Kandidat item debt terpisah bila FE mulai memakai tipe generated untuk VendorPortal.
- N4: Komentar kepala `types.ts` masih merujuk ke "design.md API contract"; sumber kebenaran kini spec. Kosmetik.
- N5: `tsconfig.test.json` punya 122 error lama (tidak di-gate). Slice 2 tidak menambah satu pun (dibandingkan per file dengan HEAD).
- N6: Spec ditulis tangan, jadi operasi yang **tidak** di-mount di dua prefix pilot tidak dicek sama sekali. Ini sesuai out-of-scope spec; DoD Sec 11 mewajibkan modul baru mulai dengan spec.

## Security
- Tidak ada permukaan runtime baru: spec tidak disajikan (`/openapi.json` out of scope), `backend/api` hanya diimpor dari test, kin-openapi hanya di `_test.go` (tetap ikut `govulncheck`: 0 vuln terjangkau).
- `openapi-typescript` = devDependency, hanya di stage builder Docker; image nginx hanya berisi `dist/`.
- Tidak menyentuh auth, uang, rekonsiliasi, migrasi, delete (Sec 4 #7). Enum status di spec hanya mendokumentasikan CHECK yang ada.

## Definition of Done (CLAUDE.md Sec 11)
- [x] Sesuai requirement (FR1–FR8) + module map (tidak ada modul/tabel baru)
- [x] Auth path / RBAC tidak berubah; 401/403 middleware per prefix ikut divalidasi T3
- [x] Maker-checker + audit — N/A (tidak ada perubahan state)
- [x] Replica/primary — N/A
- [x] Uang tetap integer/string desimal seperti kode (F2 dicatat), tanpa float
- [x] Tes hijau: Go contract T1–T4 + R1, `go test ./...`, CP build/test 1386/lint, T5
- [x] Tanpa secret / config baru
- [x] Response shape tidak berubah (spec mengikuti flat JSON yang ada)
- [x] Endpoint di modul spec ada di spec + lulus test kontrak (butir DoD baru, R1 kini menegakkannya)
- [ ] Build Docker — belum dijalankan (perintah build CP yang sama, `tsc -b` + `vite build`, hijau lokal)

## Gates
- Important R1 ditutup 2026-10-19; tes ulang hijau.
- Manual browser check (CP Vendor Request + VP Orders): **outstanding (user)**. Runtime tidak berubah, hanya tipe.
- Commit hanya saat user bilang "commit"; merge oleh code owner.
