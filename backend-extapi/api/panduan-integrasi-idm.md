# Panduan Integrasi IDM — CROWN External User API

> Status: draft
> Untuk: tim IDM CIMB. Dari: tim CROWN.
> Kontrak mesin-baca: [`openapi.yaml`](./openapi.yaml) (versi `v1`). Bila panduan ini berbeda dengan `openapi.yaml`, kontrak yang berlaku; beri tahu tim CROWN.
>
> **Semua nilai di dokumen ini palsu** (client_id, secret, token, host, IP, nama). Nilai asli diserahkan terpisah oleh tim CROWN lewat kanal yang disepakati.

## 1. Ringkasan

IDM memakai API ini untuk **membuat**, **mengubah**, **menonaktifkan**, dan **mengaktifkan kembali** user CROWN. Otentikasi memakai OAuth2 *client credentials*.

- Hanya user yang dibuat lewat integrasi ini (user internal berotentikasi LDAP, tanpa password lokal) yang terlihat dan bisa diubah. User lain dijawab `404 not_found`.
- Role istimewa (`ADMIN`, `APPACCESS`, `ADMIN_PARAM`, dan akun layanan) tidak pernah bisa diberikan atau disentuh.
- Setiap client hanya boleh memberikan role pada daftar yang didaftarkan CROWN untuk client itu.
- Semua perubahan tercatat di log audit CROWN atas nama akun layanan client IDM.

## 2. Base URL

Setiap lingkungan punya host sendiri. Host asli diserahkan terpisah; di sini hanya placeholder.

| Lingkungan | Base URL |
| --- | --- |
| Dev | `https://crown-ext.example.invalid/api/ext/v1` |
| UAT | `https://crown-ext-uat.example.invalid/api/ext/v1` |
| Produksi | `https://crown-ext-prod.example.invalid/api/ext/v1` |

Contoh di bawah memakai `$BASE` untuk base URL tersebut:

```bash
BASE=https://crown-ext.example.invalid/api/ext/v1
```

Versi ada di path (`/v1`). Perubahan yang merusak (field dihapus/diganti nama, tipe berubah, otentikasi berubah) terbit di path baru (`/v2`); `v1` tetap jalan sampai dihentikan dengan pemberitahuan. Penambahan field opsional atau endpoint baru **bukan** perubahan yang merusak, jadi **abaikan field respons yang tidak dikenal**.

Hanya IP sumber yang didaftarkan untuk client yang diterima (lihat bagian 8).

## 3. Mengambil token

`POST $BASE/oauth/token` dengan HTTP Basic (`client_id` sebagai username, `client_secret` sebagai password) dan body `application/x-www-form-urlencoded`.

- `client_secret` **hanya** boleh di header Basic. Mengirimnya di body atau URL ditolak `400 invalid_request`.
- Sebelum di-encode Basic, `client_id` dan `client_secret` di-*form-urlencode* (RFC 6749 §2.3.1). Contoh `curl -u` di bawah memakai nilai yang tidak butuh encode; bila secret memuat karakter khusus, encode dulu.
- `scope` opsional, berisi subset scope client dipisah spasi. Bila dikosongkan, token memuat semua scope yang dimiliki client.

| Scope | Boleh |
| --- | --- |
| `users:read` | `GET` user |
| `users:write` | buat, ubah, nonaktifkan (`DELETE`), aktifkan (`/enable`) |

```bash
curl -sS -X POST "$BASE/oauth/token" \
  -u 'crown_aaaaaaaaaaaaaaaaaaaaaaaaaa:contoh-secret-palsu' \
  -d 'grant_type=client_credentials' \
  -d 'scope=users:read users:write'
```

Respons `200`:

```json
{
  "access_token": "eyJhbGciOiJIUzI1NiJ9.fake.fake",
  "token_type": "Bearer",
  "expires_in": 900,
  "scope": "users:read users:write"
}
```

Respons token selalu membawa `Cache-Control: no-store` dan `Pragma: no-cache`. Jangan menyimpan token ke log.

### Umur token dan pembaruan

- Token berlaku **15 menit** (`expires_in: 900`, bawaan).
- Tidak ada *refresh token*. Ambil token baru dengan `POST /oauth/token` yang sama sebelum token habis, atau saat menerima `401 invalid_token`.
- Token otomatis tidak berlaku bila secret client diputar (*rotate*) oleh CROWN atau client dinonaktifkan; ambil token baru dengan secret yang baru.

## 4. Memanggil endpoint user

Semua endpoint user memakai header `Authorization: Bearer <access_token>`. Body request memakai `Content-Type: application/json`; tipe lain dijawab `415`.

```bash
TOKEN=eyJhbGciOiJIUzI1NiJ9.fake.fake
```

Body diperiksa ketat: field yang tidak dikenal, tipe salah, atau lebih dari satu objek JSON ditolak `400 bad_request`.

### 4.1 Buat user — `POST /users` (scope `users:write`)

Membuat user LDAP tanpa password.

| Field | Wajib | Keterangan |
| --- | --- | --- |
| `username` | ya | Tidak bisa diubah sesudahnya. |
| `full_name` | ya | |
| `email` | ya | Format email valid; harus unik. |
| `role` | ya | Harus ada di daftar role client dan bukan role istimewa. |
| `employee_id` | tidak | Unik bila diisi. |
| `supervisor_id` | tidak | `id` user terkelola yang aktif; boleh `null`. |
| `approval_level` | tidak | Bilangan bulat; boleh `null`. |

```bash
curl -sS -X POST "$BASE/users" \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{
        "username": "jdoe",
        "full_name": "Jane Doe",
        "email": "jane.doe@example.invalid",
        "role": "ATM-USER",
        "employee_id": "E0001",
        "supervisor_id": 1000,
        "approval_level": 1
      }'
```

Respons `201 Created`, header `Location: /api/ext/v1/users/1001`, body:

```json
{
  "id": 1001,
  "username": "jdoe",
  "full_name": "Jane Doe",
  "email": "jane.doe@example.invalid",
  "role": "ATM-USER",
  "employee_id": "E0001",
  "supervisor_id": 1000,
  "approval_level": 1,
  "is_active": true,
  "created_at": "2026-01-15T03:04:05Z",
  "updated_at": "2026-01-15T03:04:05Z"
}
```

Bentuk objek user ini sama untuk semua endpoint yang mengembalikan user. `employee_id`, `supervisor_id`, `approval_level` bernilai `null` bila kosong.

### 4.2 Cari berdasarkan username — `GET /users?username=` (scope `users:read`)

Pencocokan persis; `username` wajib, 1–100 karakter. User nonaktif ikut dikembalikan (cek `is_active`).

```bash
curl -sS "$BASE/users?username=jdoe" -H "Authorization: Bearer $TOKEN"
```

`200` berisi objek user; tidak ditemukan → `404 not_found`.

### 4.3 Ambil berdasarkan id — `GET /users/{id}` (scope `users:read`)

```bash
curl -sS "$BASE/users/1001" -H "Authorization: Bearer $TOKEN"
```

`200` berisi objek user. `{id}` harus bilangan bulat positif, kalau tidak `400 bad_request`.

### 4.4 Ubah user — `PUT /users/{id}` (scope `users:write`)

**Penggantian penuh**: `full_name`, `email`, `role` wajib; field opsional yang **tidak dikirim akan dikosongkan**. `username` tidak bisa diubah (mengirimkannya ditolak sebagai field tak dikenal). User yang nonaktif dijawab `404 not_found` — aktifkan dulu (4.6).

```bash
curl -sS -X PUT "$BASE/users/1001" \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{
        "full_name": "Jane A. Doe",
        "email": "jane.doe@example.invalid",
        "role": "ATM-USER",
        "employee_id": "E0001",
        "supervisor_id": 1000,
        "approval_level": 1
      }'
```

`200` berisi objek user terbaru.

### 4.5 Nonaktifkan user — `DELETE /users/{id}` (scope `users:write`)

**DELETE = soft-delete.** User dinonaktifkan (tidak bisa login lagi); **tidak ada data yang dihapus** dan baris tetap ada untuk audit. Idempoten: menghapus user yang sudah nonaktif tetap `204` tanpa perubahan. Untuk mengaktifkan lagi gunakan 4.6.

```bash
curl -sS -X DELETE "$BASE/users/1001" -H "Authorization: Bearer $TOKEN" -i
```

`204 No Content`, tanpa body.

### 4.6 Aktifkan kembali — `POST /users/{id}/enable` (scope `users:write`)

Tanpa body. Idempoten: user yang sudah aktif dikembalikan apa adanya.

```bash
curl -sS -X POST "$BASE/users/1001/enable" -H "Authorization: Bearer $TOKEN"
```

`200` berisi objek user dengan `is_active: true`.

## 5. Format error dan tabel kode

### Endpoint token (`/oauth/token`) — format RFC 6749

```json
{ "error": "invalid_client", "error_description": "client authentication failed" }
```

| HTTP | `error` | Arti | Tindakan IDM |
| --- | --- | --- | --- |
| 400 | `invalid_request` | Bukan form-urlencoded, body rusak, `client_secret` di body, atau parameter dobel. | Perbaiki request; jangan diulang apa adanya. |
| 400 | `unsupported_grant_type` | `grant_type` bukan `client_credentials`. | Perbaiki request. |
| 400 | `invalid_scope` | Scope tidak dikenal atau tidak dimiliki client. | Minta scope yang dimiliki, atau kosongkan `scope`. |
| 401 | `invalid_client` | Client tidak dikenal, secret salah, client nonaktif, **atau IP sumber tidak terdaftar**. Isi respons sengaja sama untuk semua sebab. Header `WWW-Authenticate: Basic realm="crown-ext"`. | Periksa client_id/secret dan IP sumber; bila perlu hubungi tim CROWN. **Jangan mengulang otomatis** (lihat batas gagal di bagian 6). |
| 429 | `rate_limited` | Terlalu banyak permintaan token gagal. Header `Retry-After`. | Tunggu sesuai `Retry-After`, lalu coba lagi. |
| 500 | `server_error` | Kesalahan internal. | Coba lagi dengan jeda; bila berlanjut hubungi tim CROWN. |
| 503 | `service_unavailable` | Penyimpanan pembatas laju tidak tersedia; request ditolak. | Coba lagi dengan jeda. |

### Endpoint user — format `{error, message}`

```json
{ "error": "not_found", "message": "user not found" }
```

Untuk `validation_error`, ada field tambahan `field` yang menyebut field bermasalah:

```json
{ "error": "validation_error", "message": "wajib diisi", "field": "email" }
```

| HTTP | `error` | Arti | Tindakan IDM |
| --- | --- | --- | --- |
| 400 | `bad_request` | Id path tidak valid, JSON rusak, field tak dikenal/tipe salah, query `username` kosong atau lebih dari 100 karakter, atau user menjadi atasan dirinya sendiri. | Perbaiki request; jangan diulang apa adanya. |
| 400 | `invalid_reference` | Role atau `supervisor_id` tidak ada / tidak valid. | Periksa nilai referensi. |
| 401 | `invalid_token` | Token tidak ada, rusak, kedaluwarsa, atau dicabut (termasuk setelah secret diputar). Header `WWW-Authenticate: Bearer error="invalid_token"`. | Ambil token baru (bagian 3), ulangi request sekali. Bila tetap 401, hubungi tim CROWN. |
| 403 | `insufficient_scope` | Token tidak memuat scope yang dibutuhkan. | Minta token dengan scope yang benar; bila client tidak punya scope itu, minta ke tim CROWN. |
| 403 | `ip_not_allowed` | IP sumber tidak terdaftar untuk client. | Kirim dari IP terdaftar, atau minta tim CROWN menambah IP/CIDR. |
| 403 | `role_not_allowed` | Role tidak ada di daftar role client, atau user target memegang role istimewa. | Pakai role yang diizinkan; bila perlu minta tambahan role ke tim CROWN. |
| 404 | `not_found` | User tidak ada, bukan user yang boleh dilihat integrasi ini, atau (untuk `PUT`) user nonaktif. Juga `route not found` untuk path yang salah. | Periksa id/username/path; untuk user nonaktif jalankan `enable` dulu. |
| 405 | `method_not_allowed` | Metode HTTP tidak didukung untuk path itu. | Perbaiki metode. |
| 409 | `conflict` | `username`, `email`, atau `employee_id` sudah dipakai; `message` menyebut field-nya. | Cek dengan `GET /users?username=`; jangan buat ulang. |
| 415 | `unsupported_media_type` | `Content-Type` bukan `application/json`. | Kirim `application/json`. |
| 422 | `validation_error` | Field wajib kosong atau formatnya salah; `field` menyebut nama field. | Perbaiki field yang disebut. |
| 429 | `rate_limited` | Batas laju per client terlampaui. Header `Retry-After`. | Tunggu sesuai `Retry-After`, lalu coba lagi. |
| 500 | `internal_error` | Kesalahan internal; tanpa detail. | Coba lagi dengan jeda. Untuk `POST /users` ikuti aturan di bagian 7. |
| 503 | `service_unavailable` | Penyimpanan pembatas laju tidak tersedia; request ditolak. | Coba lagi dengan jeda. |

## 6. Batas laju

| Jenis | Batas bawaan | Kunci |
| --- | --- | --- |
| Panggilan API terotentikasi | **60 per menit** | per client |
| Permintaan token **gagal** | **10 per 15 menit** | per client **dan** per IP sumber |

Saat terlampaui, API menjawab **429** dengan header `Retry-After` (detik). Tunggu sedikitnya selama itu sebelum mencoba lagi. Permintaan token yang berhasil tidak dihitung sebagai gagal; hindari *loop retry* otomatis pada `401 invalid_client`, karena itu menghabiskan batas gagal dan mengunci client sementara.

Bila penyimpanan pembatas laju sedang bermasalah, request ditolak dengan `503 service_unavailable` (bukan dibiarkan lolos); coba lagi nanti.

## 7. Aturan retry setelah timeout

`POST /users` **tidak idempoten**. Bila request timeout atau koneksi putus sebelum respons diterima, user mungkin sudah dibuat. **Jangan langsung mengirim ulang.** Lakukan:

1. `GET /users?username=<username>`.
2. `200` → user sudah ada; pakai hasilnya, jangan buat lagi.
3. `404 not_found` → belum dibuat; aman mengirim `POST /users` lagi.

`PUT`, `DELETE`, dan `enable` aman diulang (hasil akhirnya sama).

## 8. Yang dibutuhkan CROWN dari IDM sebelum go-live

Mohon kirim ke tim CROWN:

1. **IP sumber atau CIDR** server IDM yang akan memanggil API (per lingkungan). Hanya IP ini yang diizinkan; CIDR `/0` tidak diterima.
2. **Kontak serah-terima secret**: satu orang/kanal yang menerima `client_id` dan `client_secret`. Secret hanya ditampilkan sekali saat dibuat atau diputar; simpan di penyimpan rahasia IDM, jangan di repositori atau log.
3. **Daftar role** yang perlu diberikan IDM (mis. `ATM-USER`). Role istimewa tidak dapat didaftarkan.
4. Scope yang dibutuhkan (`users:read`, `users:write`, atau keduanya).

Rotasi secret dilakukan tim CROWN; token lama langsung tidak berlaku, jadi IDM perlu mengambil token baru dengan secret baru.

## 9. Daftar periksa uji sambungan

- [ ] Ambil token dari IP terdaftar → `200`.
- [ ] Buat user → `201`; `GET` dengan username dan id → `200`.
- [ ] Ubah user → `200`; `DELETE` → `204`; `GET` menunjukkan `is_active: false`; `enable` → `200`.
- [ ] Ulangi dari IP tak terdaftar → `401 invalid_client` (token) / `403 ip_not_allowed` (API).
- [ ] Token kedaluwarsa atau salah → `401 invalid_token`; pemulihan dengan token baru berjalan.
