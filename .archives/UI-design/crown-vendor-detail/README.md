# crown-vendor-detail — sumber artifact

Kode sumber (React + Tailwind) dari artifact ClickUp "crown-vendor-detail":
revamp halaman detail vendor CROWN (ABA · Abacus), tab Harga Paket.

## Isi
- src/App.jsx — kerangka halaman: topbar, header vendor + stat strip, navigasi tab
- src/components/Topbar.jsx — bar atas: pencarian, notifikasi, profil admin
- src/components/VendorHeader.jsx — identitas vendor dan statistik ringkas
- src/components/HargaPaketTab.jsx — tabel harga paket: filter, modal Tambah/Ubah, konfirmasi Akhiri
- src/components/SimpleTabs.jsx — tab Info, Cabang, PIC Vendor-wide
- src/data.js — data contoh (vendor, cabang, PIC, tingkat harga) + helper format IDR/tanggal

## Cara menjalankan
Ini proyek React (Vite) dengan Tailwind. Di lingkungan yang punya npm:
1. Buat proyek Vite React baru, lalu timpa folder src/ dengan yang ada di sini.
2. Dependensi yang dipakai: framer-motion, lucide-react, tailwindcss.
3. Jalankan seperti biasa: npm run dev

Catatan: data di src/data.js hanya contoh statis (mock) untuk kebutuhan desain,
bukan koneksi ke database produksi.
