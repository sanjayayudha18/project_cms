# Hierarki Pengguna — CROWN (Redesign)

Prototipe UI halaman **Pengaturan → Hierarki pengguna** untuk CROWN (Cash Management System).
Versi ini memisahkan **Supervisor (SPV)** dan **Pengguna biasa** ke dua tabel terpisah.

## Cara paling cepat melihatnya
Buka `hierarki-pengguna-crown.html` langsung di browser. File ini sudah bundled (React + CSS
di dalamnya), tidak butuh install atau server.

## Isi source
- `src/App.jsx` — entry point, meneruskan ke `SimpleApp`
- `src/SimpleApp.jsx` — layar utama: toolbar pencarian/filter domain + tabel SPV, Pengguna biasa, Admin & support
- `src/simple.css` — token warna & tipografi (Plus Jakarta Sans, IBM Plex Mono, CIMB red #D31327)
- `src/data.js` — role model (domain_scope x role_level), daftar pengguna contoh, aturan eligibility atasan
- `src/Rail.jsx`, `src/TeamPanel.jsx`, `src/dialogs.jsx`, `src/ui.jsx`, `src/crown.css` — komponen desain versi sebelumnya, disimpan sebagai referensi
- `kits/shadcn-ui` — komponen UI (Button, Input, Select, Dialog, Badge, Field)
- `tests/artifact.spec.js` — smoke test

## Catatan
Semua data di dalamnya adalah data contoh. Perubahan atasan dan level persetujuan hanya
simulasi di dalam prototipe, tidak terhubung ke CROWN.
