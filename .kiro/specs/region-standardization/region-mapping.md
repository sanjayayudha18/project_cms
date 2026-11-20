# Draf Pemetaan Region

> Status: accepted 2026-11-18 (PO, "Setuju, lanjut") — disusun tim dari DB dev 2026-11-18; M1–M4 diputuskan PO 2026-11-18.

Sumber: DB dev 2026-11-18 (`regions`, `locations`, `atms` aktif, `vendor_branches`). Format kode = kode cabang vendor (Q2), nama = ESQ yang dirapikan (Q3). Baris bertanda **M1–M4** butuh keputusan PO; sisanya usulan yang tinggal disetujui.

## A. Region baku (13 baris `regions`, rename sekali — Q6)

| `regions.id` | Kode lama | Kode baku | Nama baku | ATM aktif | Catatan |
|---|---|---|---|---|---|
| 1 | BALI_NUSRA | BALNUS | Bali Nusra | 153 | |
| 2 | INDTIM | INDTIM | Indonesia Timur | 63 | M1: Kalimantan terpisah, jadi nama tanpa "dan Kalimantan" |
| 8 | JABAR | JABAR | Jawa Barat | 158 | |
| 9 | JATENG | JATENG | Jawa Tengah | 237 | |
| 10 | JATIM | JATIM | Jawa Timur | 191 | |
| 3 | JKT_CENTRAL | JKTPST | Jakarta Pusat | 71 | |
| 4 | JKT_EAST | JKTTIM | Jakarta Timur | 342 | |
| 5 | JKT_NORTH | JKTUTR | Jakarta Utara | 48 | |
| 6 | JKT_SOUTH | JKTSEL | Jakarta Selatan | 158 | |
| 7 | JKT_WEST | JKTBAR | Jakarta Barat | 85 | |
| 11 | KALIMANTAN | KAL | Kalimantan | 50 | M1: tetap terpisah; kode M2 |
| 12 | SUMATERA | SUMATERA | Sumatera | 229 | |
| 13 | TANGERANG | TGR | Tangerang | 119 | M2 (singkatan dari nama lokasi ATM "TGR.…") |

ATM tidak berpindah region: hanya kode dan nama baris yang berubah. Total 1.904 ATM aktif di 13 region (16 ATM sisanya milik region uji `ITESTNUM`, di luar spec ini).

## B. Kode cabang lama → region baku + kota (34 kode, 356 cabang)

Kode per kota menjadi isi **tabel kota** (Q1) di bawah region bakunya.

| Kode cabang lama | Cabang | → Region baku | → Kota | Catatan |
|---|---|---|---|---|
| JKTBAR / JKTPST / JKTSEL / JKTTIM / JKTUTR | 21 / 18 / 18 / 30 / 15 | sama | — | sudah baku |
| JABAR / JATENG / JATIM / SUMATERA | 33 / 45 / 49 / 59 | sama | — | sudah baku |
| BALNUS | 8 | BALNUS | — | |
| INDTIM | 31 | 15 → INDTIM; 16 → KAL (M1) | — | 16 cabang kota Kalimantan (Balikpapan, Banjarmasin, Palangkaraya, Pontianak, Samarinda, Singkawang, Tarakan, Kas Mobil Samarinda) pindah ke KALIMANTAN, dipilih per `branch_name` |
| BALI | 2 | BALNUS | BALI Bali | |
| MTR | 1 | BALNUS | MTR Mataram | ATM NTB ada di BALI_NUSRA |
| KUPANG | 1 | BALNUS | KUPANG Kupang | ATM NTT ada di BALI_NUSRA |
| ACEH, JAMBI, LAMPUNG, MDN, PGK, PKU, PLG, TNJ | 1, 1, 1, 3, 1, 1, 1, 1 | SUMATERA | ACEH Aceh, JAMBI Jambi, LAMPUNG Lampung, MDN Medan, PGK Pangkal Pinang, PKU Pekanbaru, PLG Palembang, TNJ Tanjung Pinang | |
| AMBON, KDI, MDO, MKS | 1, 1, 1, 1 | INDTIM | AMBON Ambon, KDI Kendari, MDO Manado, MKS Makassar | |
| BJM, BPN, PKY, PNK, SMD | 2, 1, 1, 1, 1 | KAL (M1) | BJM Banjarmasin, BPN Balikpapan, PKY Palangkaraya, PNK Pontianak, SMD Samarinda | |
| JEMBER, MADIUN | 1, 1 | JATIM | JEMBER Jember, MADIUN Madiun | |
| JKT | 3 | per cabang (tabel D) | — | **M3** |

## C. 51 cabang tanpa kode (Q8) — dari kota di `branch_name`

| Region baku | Cabang |
|---|---|
| JABAR | ADVANTAGE_019 Bandung, ADVANTAGE_031 Tasikmalaya, BRINKS_002 Bandung, BRINKS_004 Cirebon, KEJAR_001 Bandung, KEJAR_002 Cirebon |
| JATENG | ADVANTAGE_026 Kudus, ADVANTAGE_029 Solo, ADVANTAGE_032 Tegal Slawi, BRINKS_007 Klaten Solo, BRINKS_008 Klaten Yogya, BRINKS_014 Purwokerto, BRINKS_015 Semarang, KEJAR_004 Semarang, PROSEGUR_008 Solo |
| JATIM | ADVANTAGE_024 Jember, ADVANTAGE_025 Kediri, ADVANTAGE_028 Malang, ADVANTAGE_030 Surabaya, BRINKS_016 Surabaya, KEJAR_005 Surabaya, PROSEGUR_002 Jember, PROSEGUR_004 Madiun, PROSEGUR_005 Malang, PROSEGUR_009 Surabaya |
| BALNUS | ADVANTAGE_020 Denpasar, BRINKS_005 Denpasar |
| SUMATERA | BRINKS_003 Batam, BRINKS_009 Lampung, BRINKS_012 Medan, BRINKS_013 Pekanbaru, PROSEGUR_003 Lampung, PROSEGUR_006 Padang, PROSEGUR_007 Palembang, SSI_005 Dumai, SSI_007 Lhokseumawe |
| INDTIM | ABACUS_009 Makassar, ADVANTAGE_027 Makassar, BRINKS_010 Makassar, BRINKS_011 Manado, SSI_006 Jayapura |
| KAL (M1) | BRINKS_001 Balikpapan |
| JKTBAR | ADVANTAGE_022 Jakarta Meruya |
| JKTTIM | ADVANTAGE_023 Jakarta Rawamangun |
| JKTSEL | TAG_018 Jakarta Bintaro, TAG_019 Jakarta Lenteng Agung |
| TGR | TAG_020 Jakarta Tangerang |
| JABAR (M4) | ADVANTAGE_021 Jakarta Karawang |
| JKTPST (M3) | BRINKS_006 Jakarta, KEJAR_003 Jakarta, PROSEGUR_001 Jakarta |

## D. Cabang kode `JKT` (3)

| Cabang | Usulan | Catatan |
|---|---|---|
| ADVANTAGE_017 Jakarta Serang | TGR | Serang = Banten; ATM Banten ada di TANGERANG/JKT_WEST/JKT_SOUTH |
| TAG_004 Jakarta Bekasi | JKTTIM | ATM Jawa Barat (Bekasi) ada di JKT_EAST |
| BIJAK_001 Bijak Jakarta | JKTPST (M3) | nama tidak menyebut wilayah |

## Keputusan PO yang dibutuhkan

- **M1 — Kalimantan** ✅ DIPUTUSKAN PO 2026-11-18: **(a) KALIMANTAN terpisah**. 23 cabang Kalimantan (16 dari INDTIM menurut kota di `branch_name`, 6 berkode BJM/BPN/PKY/PNK/SMD — BJM 2 cabang, BRINKS_001 Balikpapan; dihitung ulang saat dry-run 042, 2026-11-18) → `KALIMANTAN`; kota BJM/BPN/PKY/PNK/SMD di bawah KALIMANTAN; region INDTIM bernama "Indonesia Timur"; 50 ATM tetap di region 11. Latar: sisi ATM memisahkan `KALIMANTAN` (50 ATM), sisi cabang menggabung Kalimantan ke `INDTIM` ("Indonesia Timur dan Kalimantan", 16 cabang + BJM/BPN/PKY/PNK/SMD + BRINKS_001). Pilihan: (a) `KALIMANTAN` terpisah — 22 cabang pindah ke KALIMANTAN; (b) gabung ke `INDTIM` — 50 ATM pindah dari region 11 ke INDTIM (region 11 di-soft-disable).
- **M2 — kode Tangerang dan Kalimantan** ✅ DIPUTUSKAN PO 2026-11-18: **dipendekkan** → `TGR` (Tangerang, singkatan yang sudah dipakai di nama lokasi ATM) dan `KAL` (Kalimantan). Nama tetap "Tangerang" dan "Kalimantan". Di bagian M1 di atas, `KALIMANTAN` sebagai tujuan = kode `KAL`.
- **M3 — cabang "Jakarta" tanpa wilayah** (BRINKS_006, KEJAR_003, PROSEGUR_001, BIJAK_001) ✅ DIPUTUSKAN PO 2026-11-18: **semua ke `JKTPST`**. Bila lokasi sebenarnya berbeda, dikoreksi kemudian lewat maker-checker cabang vendor.
- **M4 — ADVANTAGE_021 Jakarta Karawang** ✅ DIPUTUSKAN PO 2026-11-18: **`JABAR`** (Karawang = Jawa Barat; ATM kota Karawang, mis. lokasi "KRW.KCP Karawang Surya Cipta CRM 1", juga di region JABAR).

## Turunan otomatis (bukan keputusan baru)

- `acm_area_regions` (34 baris dev, semua "Area ACM Dev") dipetakan ulang ke kode baku; duplikat setelah penggabungan dilebur menjadi satu baris per region.
- `cash_count_area_regions`, `cash_count_pics.region_code`: 0 baris dev, tidak ada yang dipetakan.
- `vendor_requests` + `vendor_request_number_seq`: 10 request belum selesai di dev → di-clear dulu (Q7, dengan OK user saat dijalankan).
