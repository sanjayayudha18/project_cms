# UR New Template v0.3 - E2E Cash Management System v.4 - Phase 1.docx

| Initiative Name | E2E Cash Management System |
| ---| --- |
| JIRA ID |  |

# Approval Page:

| Prepared by :<br>Signed by the parties reviewing this document |
| --- |
| Name | : | Robby Hartono L. |  |
| Title | : | Vendor Operation & Control Manager |  |
| Date | : |  | Signature |
|  |  |  |  |
| Name | : | Agus Triyono |  |
| Title | : | ATM Support & Monitoring Head |  |
| Date | : |  | Signature |
|  |  |  |  |
| Name | : | Eka Christina Doloksaribu |  |
| Title | : | Cash & Wealth Management Operations, Control, Improvement Head |  |
| Date | : |  | Signature |
|  |  |  |  |

|  |  |
| ---| --- |
| Approved by :<br>Signed by the parties reviewing this document |
| Name | : | Nicky Cahyadi Herman |  |
| Title | : | Channel Operations &<br>Settlement Head |  |
| Date | : |  | Signature |
|  |  |  |  |
| Name | : | Ketut Meliana Saputra |  |
| Title | : | Transaction Operational & Digital Improvement Head |  |
| Date | : |  | Signature |
|  |  |  |  |
| Name | : | Tajindra Pal Singh |  |
| Title | : | Head of Central Operations |  |
| Date | : |  | Signature |
|  |  |  |  |
|  |  |
|  |  |

| Acknowledge by :<br>Signed by the parties reviewing this document |
| --- |
| Name | : | Febrina Afivah |  |
| Title | : | IT Business Partner |  |
| Date | : |  | Signature |
|  |  |  |  |
| Name | : | Atikah Rahmawati |  |
| Title | : | IT Business Partner |  |
| Date | : |  | Signature |
|  |  |  |  |
| Name | : |  |  |
| Title | : |  |  |
| Date | : |  | Signature |
|  |  |  |  |

# Revision History

| Date | Version | Description | Author |
| ---| ---| ---| --- |
|  |  |  |  |
|  |  |  |  |

Table of Contents
[A. Approval Page: i](http://#_Toc190957874)
[B. Revision History ii](http://#_Toc190957875)
[A. BUSINESS REQUIREMENT iv](http://#_Toc190957876)
[1\. Business Problem / Opportunity Statement iv](http://#_Toc190957877)
[2\. Business Objective & Success Criteria iv](http://#_Toc190957878)
[3\. Business Process Overview iv](http://#_Toc190957879)
[a. As-is Business Process iv](http://#_Toc190957880)
[b. To-be Business Process viii](http://#_Toc190957881)
[B. Functional RequiremenT xi](http://#_Toc190957882)
[C. NON-FUNCTIONAL REQUIREMENT xii](http://#_Toc190957883)
[1\. Volume Projection (if applicable) xii](http://#_Toc190957884)
[2\. Reporting / Reconciliation Requirement (if applicable) xii](http://#_Toc190957885)
[3\. Performance Requirement (if applicable) xii](http://#_Toc190957886)
[4\. Data Governance Requirement (DGCC) xiv](http://#_Toc190957887)
[D. Appendix xvi](http://#_Toc190957889)
[E. HIGH LEVEL ESTIMATION TIMELINE xvii](http://#_Toc190957890)
# BUSINESS REQUIREMENT
## Business Problem / Opportunity Statement
Saat ini Bank belum memiliki system yang dapat mengotomasi proses Cash Management secara End to End, Dokumen ini akan mendeskripsikan kebutuhan pengguna (User Requirements Specification/URS) secara komprehensif untuk pembangunan sistem End-to-End (E2E) Cash Management System (CMS). Sistem ini mencakup proses pengelolaan kas untuk layanan ATM , termasuk proses Realisasi dan Proyeksi secara harian dan pengelolaan Dokumen Vendor untuk kebutuhan Vendor Management. Dokumen ini disusun dengan tingkat detail naratif agar dapat menjadi rujukan tunggal bagi tim bisnis, IT, vendor dan pihak lain yang membutuhkan.
## Business Objective & Success Criteria
Tujuan utama implementasi E2E Cash Management System adalah menyediakan satu sistem terintegrasi yang mampu mengatur, memonitor, dan mengendalikan seluruh aktivitas pengelolaan kas secara end-to-end dengan kontrol yang kuat.
Manfaat yang diharapkan antara lain:
*   Peningkatan visibilitas melalui dashboard operasional dan finansial yang real-time.
*   Standardisasi proses dan dokumentasi (instruksi, pelaksanaan, DSR, BA, rekonsiliasi).
*   Penurunan risiko mismatch, keterlambatan, dan potensi fraud melalui rekonsiliasi data lintas sumber.
## Business Process Overview
### As-is Business Process
1. **Proses as – is ATM Cash forecasting harian**
*   **Proses penerimaan DSR dari vendor PJPUR**
Penerimaan data DSR masih dilakukan melalui email oleh vendor ke tim ATM support
![](https://t90181331789.p.clickup-attachments.com/t90181331789/5df3d500-e6ef-44bb-8c92-5240a9e9ac72/c256989aaf050354d58c37bc2aae1ecc.png)
*   **Proses Realisasi**
Aktivitas penerimaan data DSR dari vendor, penarikan data “orders” untuk data rekomendasi pengisian ATM serta penarikan data “horizons” untuk data prediktif refund dari Opticash masih dilakukan secara manual. Selain itu, staff ATM support harus melakukan pengolahan data rekomendasi isi ATM, prediktif refund ATM dan rekap DSR di Ms. Excel untuk persiapan instruksi pengambilan/penyetoran uang.
![](https://t90181331789.p.clickup-attachments.com/t90181331789/d8091996-117c-454d-99a6-27bbb1a1f75f/43bb2534c6359bee559e44143ec81895.png)
*   **Proses Perhitungan Proyeksi Harian**
Aktivitas penarikan data “horizon” report dari H+0 sampai H+2 dilakukan secara manual. Selanjutnya staff, ATM Support membuat kertas kerja baru berdasarkan data report, melakukan pengolahan data pivot, pemeriksaan nilai CIT yang minus dan nilai di nol kan
![](https://t90181331789.p.clickup-attachments.com/t90181331789/2ddeeda0-9e73-4920-943f-21bdce21940e/4a48930d80c00190f9078ab6d99ebf61.png)
1. **Proses as – is Cash count vault vendor**
Aktivitas pengisian berita acara dan rekonsiliasi hasil cash count dengan saldo escrow masih dilakukan secara manual
![](https://t90181331789.p.clickup-attachments.com/t90181331789/31b0dbe0-a499-476d-bc03-464b006d44c1/3474d9bce34981067e0ba340cd52f641.png)
### To-be Business Process
Sistem menyediakan fasilitas end-to-end untuk proses perencanaan kebutuhan kas ATM, instruksi pengisian ATM, pengelolaan kebutuhan dana, pemenuhan uang oleh Cash Management, hingga pengambilan ke vendor FLM.
1. **Manajemen Vendor & Master Data**
Sistem menyediakan modul master data yang memungkinkan bank mengelola informasi vendor dan infrastruktur pengelolaan kas (Cash Management). Data vendor meliputi identitas legal, status aktif/non-aktif, NPWP, dan kontak notifikasi (PIC). Data vault vendor memuat alamat lengkap, koordinat (opsional), jam operasional, kapasitas, serta kategori vault (ATM/Cash). Sistem juga menyimpan PIC vendor beserta jabatan, nomor kontak, dan email untuk memastikan notifikasi sampai kepada pihak yang tepat. Kelolaan vendor terhadap mesin ATM dan/atau cabang/nasabah harus dicatat dengan akurat, termasuk lokasi, kapasitas.
![](https://t90181331789.p.clickup-attachments.com/t90181331789/2286eebc-3995-4850-8517-fd54387f6e47/4a21363e717db1eb5a427a3de1c7693d.x-emf)
Perubahan terhadap master data tunduk pada kontrol maker–checker: satu pihak sebagai pembuat (maker) dan pihak lain sebagai penyetuju (checker). Setiap perubahan tercatat dalam audit trail lengkap yang memuat siapa, kapan, dan apa yang berubah. Sistem menyediakan fasilitas impor dan ekspor data (CSV/XLSX) dengan validasi struktur dan konten untuk memudahkan onboarding vendor dan pemeliharaan data massal.
Detail naratif kebutuhan master data vendor, vault, PIC, kelolaan, maker-checker.
![](https://t90181331789.p.clickup-attachments.com/t90181331789/2286eebc-3995-4850-8517-fd54387f6e47/4a21363e717db1eb5a427a3de1c7693d.x-emf)
Fitur yang tersedia:
*   Upload data/dokumen
*   Download/Unduh dokumen
*   Edit/Update data
*   Hapus (dengan kontrol maker-checker bila diperlukan)
*   Pencarian & filter data
*   Export ke CSV/XLSX/PDF
*   Notifikasi email/in-app
*   Audit trail otomatis
### ATM Cash Forecasting Harian
![](https://t90181331789.p.clickup-attachments.com/t90181331789/a5267c14-4f8a-4d49-bf3b-c3e93b3dc439/02a1beca980896a128e5bc3a6a5ecabd.x-emf)
1. Persiapan Data Forecast
    *   Sistem menggunakan data rekomendasi ATM (Data Science) yang diterima pada hari itu (H0) sebagai dasar perhitungan kebutuhan kas.
    *   Sistem dapat mengakomodir upload data dan mengelola data tambahan sebagai bagian dari review rekomenasi, diantaranya :
        *   List data Complaint Handling/rekonsiliasi
List yang diterima dari team complain handling / rekonsiliasi akan dilakukan pengecekan dengan data rekomendasi DMAA, jika dari list team complain handling / rekonsiliasi sudah ada rekomendasi dari DMAA baik yang emergency atau planned yang dikirimkan kemarin dari system DMAA maka ID atau Lokasi tersebut tidak dibuatkan sebagai order Kembali, begitu sebaliknya jika belum ada rekomendasi maka dimasukkan sebagai order emergency.
*       *       *   Project ATM (replace machine , new ATM, relokasi, dll)
List Project diterima dari pihak internal (Bisnis Unit) maka id atau Lokasi project ditambahkan sebagai order / rekomendasi dan dimasukkan sebagai order emergency/planned.
*       *       *   ATM yang bermasalah (untuk diexclude dalam recommendation)
Lokasi mesin yang sedang problem akan ditake out dari rekomendasi karena Lokasi yang ini tidak bisa untuk transaksi yang disebabkan beberapa problem diantaranya pending part, vandalism, dll.
*       *       *   Adjustment Order
Untuk area tertentu yang perlu dilakukan adjustment terkait dengan nominal rekomendasi dan dari list yang akan dilakukan adjustment dapat dilakukan upload untuk mereplace nominal yang telah dibentuk oleh DMAA.
1. Validasi dan Konsolidasi Order
*   Sistem melakukan pengecekan terhadap order emergency/adhoc ATM yang sudah pernah diterbitkan sebelumnya (max H-1)
*   ATM yang sudah memiliki order emergency/adhoc H-1 atau yang aktif pada H0 tidak akan dibuatkan order tambahan (exclude order pada file forecasting dari data science).
*   Sistem menggabungkan dan menampilkan rekomendasi dari data science, order manual, dan adjustment order menjadi draft order ATM.
2. Pengelolaan Data Pendukung
*   Vendor FLM melakukan upload Daily Summary Report (DSR).
Laporan DSR ini Adalah laporan saldo fisik yang ada di vaulting/kantor FLM yang dilaporkan setiap hari sebelum jam 9:00. Pada akhir bulan system dapat mengakomodir untuk mengeluarkan report selama 1 bulan untuk laporan DSR yang tidak mengirimkan/terlambat dalam melakukan upload untuk selanjutnya menjadi acuan pengenaan penalty ke FLM.
![](https://t90181331789.p.clickup-attachments.com/t90181331789/bfa8f0e7-8197-4f60-b3b3-843a7c353e0e/c9ba5aead939e6e4c5dc9428fa4535a0.png)
Bentuk Report Laporan DSR yang mengalami keterlambatan/tidak dikirimkan :

| No | Tanggal Laporan<br>(Bukan Tanggal pengiriman DSR) | Nama Vendor dan Area Vault | Tanggal Terima | Status |
| ---| ---| ---| ---| --- |
| 1 | 01-Aug-2025 (Friday) | Abacus Bali | (Mon) 04/Aug/2025 13:03 | TELAT |
| 2 | 01-Aug-2025 (Friday) | Abacus Bandung | (Sat) 02/Aug/2025 08:28 | OK |

*   Sistem menerima data proyeksi refund berdasarkan rencana replenishment ATM.
Data Proyeksi refund diproses dari data DMAA berdasarkan prediksi pengisian beberapa hari kedepan.
![](https://t90181331789.p.clickup-attachments.com/t90181331789/402a8af0-911f-4ccd-87f6-5976bcaccc1f/0dad93923b0f9452a38fb8bdac3cda86.x-emf)
_\*)contoh hari ini adalah tanggal 14 september saat proses kebutuhan uang, maka untuk perhitungan refund contoh id 1023 ada proyeksi dari Data Science untuk pengisian tanggal 15 maka akan ada perkiraan refund sebesar 73 juta dengan perhitungan :_
_Opening balance (tanggal 14 September – transaksi tanggal 14 & 15 september=perkiraan refund)_
*   Sistem juga menjadikan order hari sebelumnya yang akan diisi pada hari H (Hari Proses Order) sebagai pengurang saldo fisik yang ada di vendor / FLM
*   Data dapat dikelompokkan berdasarkan vendor, vault, dan denominasi untuk bisa diproses lebih lanjut berapa kebutuhan uang per vendor-nya.
![](https://t90181331789.p.clickup-attachments.com/t90181331789/88ff9efb-87b4-4908-9ba0-d44e94533f30/6afc7c61f9c647ef5e5184449172be56.png)
1. Perhitungan Kebutuhan ATM
*   Sistem menghitung kebutuhan kas ATM secara otomatis berdasarkan formula:
Order ATM = (Saldo DSR+ Proyeksi Refund)-(Rekomendasi DMAA + Rencana Isi Hari-H) apabila terdapat rekomendasi dari DMAA namun DSR tidak tersedia, maka system akan menghitung berdasarkan rekomendasi DMAA 🡪 apakah rumus ini masih valid?
1.     1. **Approval dan Publikasi Order**
*   Hasil perhitungan kebutuhan ATM harus melalui proses approval berjenjang.
*   Setelah disetujui, sistem menerbitkan instruksi pengisian ATM dan notifikasi kepada pihak terkait.
    1. **Pemenuhan Dana**
*   Cabang atau Cash Management menentukan lokasi sumber dana dan nominal pemenuhan per denominasi per vault.
*   Lokasi Pengambilan Uang
*   Nominal Per Denominasi
*   Tanggal Pengambilan
*   Jam Pengambilan
*   Vault Penyedia Dana
*   Vendor FLM menerima informasi lokasi dan jadwal pengambilan dana melalui aplikasi.
    1. **Pengambilan Dana**
*   Vendor FLM menginput data petugas dan kendaraan yang akan digunakan untuk pengambilan dana.
*   Nama Petugas
*   Nomor KTP
*   Nomor NIP
*   Nama Perusahaan
*   Keperluan Pengambilan
*   Nominal Pengambilan
*   Nominal Per Denominasi
*   Jumlah Lembar
*   Nomor Kendaraan
*   Tanggal Pengambilan
*   Data pengambilan dana yang diinput oleh FLM harus mendapatkan persetujuan Maker dan Checker.
    1. **Surat Tugas dan Serah Terima**
*   Sistem menghasilkan surat tugas pengambilan dana yang dapat didownload oleh pihak terkait.
*   Sistem mendukung proses verifikasi dan serah terima dana antara Cash Management dan Vendor FLM.
2. Pembayaran invoice vendor FLM
    *       *   Vendor dapat melakukan upload invoice ATM dan data pendukung kedalam system
        *   Team internal CIMB Niaga dapat melakukan upload ke dalam system yang berisi informasi : Data ATM aktif, data terminasi ATM, Harga, Paket trip, category ATM (VIP, Industri & Regular)
        *   System dapat melakukan rekonsiliasi secara otomatis
        *   Team internal CIMB Niaga dapat melakukan adjust manual terhadap sanggahan vendor
Detail proses instruksi, DSR, forecast, rekomendasi kas, pembukuan, revisi DSR.
Fitur yang tersedia:
*   Upload data/dokumen
*   Download/Unduh dokumen
*   Edit/Update data
*   Hapus (dengan kontrol maker-checker bila diperlukan)
*   Pencarian & filter data
*   Export ke CSV/XLSX/PDF
*   Notifikasi email/in-app
*   Audit trail otomatis
**Business Rules**
*   ATM yang berstatus problem tidak dapat direkomendasikan untuk replenishment.
*   ATM yang sudah memiliki order aktif tidak dapat dibuatkan order baru pada periode yang sama.
*   Seluruh proses approval menerapkan prinsip maker-checker.
*   Seluruh aktivitas upload, perubahan data, approval, dan distribusi order tercatat dalam audit trail.
*   Vendor/cabang hanya dapat mengakses data yang menjadi tanggung jawabnya.
*   System dapat memvalidasi rekomendasi yang diinstruksikan dijalankan/tidak.
    1.     1.     1. Pengisian sesuai jadwal (replenish sesuai jadwal)
            2. Pengisian maju (replenish dilakukan 1 hari atau maksimum 2 hari lebih awal dari jadwal) – disesuaikan dengan kondisi hari libur.
            3. Pengisian mundur (replenish dilakukan 1 hari atau maksimum 2 hari lebih lama dari jadwal yang ditentukan) – disesuaikan dengan kondisi hari libur.
            4. Pengisian tidak dilakukan (replenish tidak dilakukan sesuai jadwal atau maksimum lebih dari dan/atau kurang dari 2 hari)
            5. Pengisian tanpa ada order (realisasi replenish dibandingkan dengan Order)
            6. Report refund per id dan bisa difilter per vendor per denom
            7. Report amount pengisian tidak sesuai dengan order berserta detail ID
            8. Report transaksi (Tarik dan setor),
            9. Report Profile (id atm, Lokasi, denom, dll)
            10. Report Oder vs Transaksi
            11. Report User
            12. Log User (last Login date)
            13. Report trip/Realisasi pengisian dan dapat menampilkan tertinggi, per vendor secara per id ATM.
![](https://t90181331789.p.clickup-attachments.com/t90181331789/16edd7a6-fc4e-4275-bf57-7b7a07e83dc5/d083961845f78c9200fbaae992f24ac9.x-emf)
1. **Dashboard & Pelaporan**
CMS menyediakan dashboard Utama untuk kelolaan ATM yang menyajikan metrik status, aging, SLA, tren historis, dan indikator exception; dilengkapi filter, drill‑down, dan ekspor ke CSV/XLSX/PDF. Sistem juga menyediakan notifikasi email/in‑app untuk keterlambatan DSR vendor.
1. **Cash count Vault vendor**
Setiap bulan, sistem melakukan analisa saldo kelolaan vault vendor menggunakan sumber escrow dari SIBS/MIS untuk menentukan kategori risiko dan menyusun jadwal visit cash count yang bersifat acak (non‑berpola). Penjadwalan mempertimbangkan ketersediaan PIC wilayah, dan dapat memilih PIC yang akan bertugas, dengan memberikan notifikasi via email, PIC yang ditunjuk/bertugas akan menerima emai notifikasi/instruksi dan dapat memilih accept or reject, jika accept maka system dapat menerbitkan surat tugas yang dapat diunduh oleh PIC, jika reject, maka terdapat history dan dilakukan penjadwalan ulang baik reschedule jadwal atau Ganti PIC.
Pada saat pelaksanaan, PIC Cash Count mengisi Berita Acara (BA) yang berisi hasil perhitungan fisik dan checklist kondisi vault secara digital langsung di system ini. Kolom DSR pada BA akan terisi otomatis dari DSR yang diunggah vendor, sehingga perbandingan dan selisih dapat dihitung secara otomatis. Dokumentasi pendukung (foto/file) diunggah, dan BA ditandatangani secara digital oleh pihak vendor dan PIC bank. Sistem kemudian menghasilkan dokumen final (BA, checklist, foto, dan dokumen pendukung) yang dapat diunduh/ dicetak.
Untuk kebutuhan manajemen, sistem menyediakan rekapitulasi yang merangkum hasil cash count, nilai DSR, saldo escrow H‑1 (otomatis dari MIS/SIBS), hasil proofing (diinput manual oleh user, beserta hasil rekonsiliasi tiga arah (cash count vs escrow vs proofing). Output rekap mencakup ringkasan per vault (ATM & Cash), ringkasan checklist, serta evaluasi performa vendor.
Berita Acara Cash Count Vault ATM
![](https://t90181331789.p.clickup-attachments.com/t90181331789/df22ceb8-f6be-4049-b0b4-e38ff60208bd/c448b8abf6240773423f04ecc35cb804.png)
Berita Acara Cash Count Vault Cash
![](https://t90181331789.p.clickup-attachments.com/t90181331789/669e7c18-ea2c-438a-86ad-1b790d790d88/989666a8e915a621cbcc3a255f439e5b.png)
Checklist Cash count Selektif Mesin
![](https://t90181331789.p.clickup-attachments.com/t90181331789/cebcc259-8356-4bc6-97c5-93c4159fee97/929e13d914b2f9c171e1896eda587715.png)
![](https://t90181331789.p.clickup-attachments.com/t90181331789/753557f2-65f7-4d8a-905c-3e39dac8b875/33b1681df1e40dfdc03a55f2e9c9acdb.png)
Checklist Cash Count Vault Vendor PJPUR
![](https://t90181331789.p.clickup-attachments.com/t90181331789/0623fba7-f3d2-4906-888d-6774ead71d22/6be05dff7c7ea0523df66b6929ad5fb2.png)
![](https://t90181331789.p.clickup-attachments.com/t90181331789/f8a76821-b651-4d82-bdb3-98b0844e0b7b/b0880e92ceb05a0a5de66e4579d87e18.png)
Berita Acara Cash count Valas
![](https://t90181331789.p.clickup-attachments.com/t90181331789/b7300687-390a-4216-aaf6-1e1f0ed2a1ed/c04552ec3fec137f870268cb033541e6.png)
Parameter Checklist
![](https://t90181331789.p.clickup-attachments.com/t90181331789/0fc74c51-29ed-4d3d-a415-7e1e5cf85b36/2b785679980a5eaa6cdefd3fb044b750.png)
![](https://t90181331789.p.clickup-attachments.com/t90181331789/7eb0e1ab-d9b2-453b-b8b4-9555993c8fc4/2722dec9cb87fc42686b5ab7df1895f6.png)
Secara alur proses :
1. Sistem menganalisa saldo kelolaan dan histori untuk menentukan kategori risiko serta menyusun jadwal kunjungan secara acak (non‑berpola).
2. PIC yang bertugas akan mendapat notifikasi via email, dan dapat memilih Accept or Reject,
3. (If Reject) Jadwal dapat dilakukan re-schedule otomatis atau adjust maupun ganti PIC.
4. (If Accept) Surat tugas diterbitkan otomatis dan dapat diunduh oleh PIC .
5. Pelaksanaan: PIC mengisi BA (autofill kolom DSR), melengkapi checklist, dan mengunggah dokumentasi foto.
6. E‑sign dilakukan oleh vendor dan PIC bank. Sistem menghasilkan dokumen final yang siap diunduh/cetak.
7. Rekapitulasi menyajikan hasil cash count, nilai DSR, saldo escrow H‑1, serta hasil proofing; sistem melakukan rekonsiliasi dan menandai selisih untuk ditindaklanjuti.
8. Dashboard menampilkan progres cash count, temuan, dan evaluasi performa vendor per periode.

| No | No Escrow | Nama Escrow | Cash count status | Findings |
| ---| ---| ---| ---| --- |
| 1 | 80017854xxxxx | ATM Abacus Bali | Complete | \- |
| 2 | 8001784xxxxx | CASH Abacus Bali | Complete | Selisih Kurang |
| 3 | 8001830xxxxx | ATM Abacus Sukabumi | Not Complete |  |

_Status :_
_Complete : sudah dilaksanakan cash count_
_Not Complete : belum dilaksanakan cash count_
_On-progress : sedang dilaksanakan cash count_
Detail penjadwalan random tidak berpola setiap bulannya, ignore tanggal dan hari libur, Berita Acara , checklist, foto, e-sign, rekonsiliasi, rekapitulasi report.
Fitur yang tersedia:
*   Upload data/dokumen
*   Download/Unduh dokumen
*   Edit/Update data
*   Hapus (dengan kontrol maker-checker bila diperlukan)
*   Pencarian & filter data
*   Export ke CSV/XLSX/PDF
*   Notifikasi email/in-app
*   Audit trail otomatis
1. **Cash count selektif Mesin**
Selain cash count di level vault, sistem juga mendukung kegiatan cash count selektif pada mesin‑mesin tertentu sesuai instruksi pengawasan. Prosesnya sejalan dengan mekanisme di level vault: penerbitan surat tugas, pengisian BA (hasil hitung fisik), checklist, dokumentasi foto, penandatanganan digital, dan pembuatan dokumen final. Hasil kegiatan ini direkap untuk evaluasi performa vendor pada level mesin.
Detail instruksi, BA, checklist, dokumen, e-sign, rekap.
Fitur yang tersedia:
*   Upload data/dokumen
*   Download/Unduh dokumen
*   Edit/Update data
*   Hapus (dengan kontrol maker-checker bila diperlukan)
*   Pencarian & filter data
*   Export ke CSV/XLSX/PDF
*   Notifikasi email/in-app
*   Audit trail otomatis
# Functional RequiremenT

| Function ID | Function Name | Description | Impacted Stakeholder | Priority |
| ---| ---| ---| ---| --- |
| FNC 001 | ATM Cash Forecasting | Sistem dapat mengeluarkan penerimaan laporan DSR dari vendor FLM, instruksi pengisian mesin ATM dan perhitungan proyeksi kebutuhan ATM | ATM Support and Monitoring | High |
| FNC 002 | Cash count | Sistem dapat melakukan scheduling, rekonsiliasi, report progress, dan hasil cash count.<br>Cash count mencakup : | Cash count Vault (ATM dan Cash)<br>Cash count Selektif Mesin ATM<br>Vendor Operations & Control | High |
| FNC 003 | Dashboard | ATM Forecasting :<br>Daily instruction (Amount and Term ID)<br>Rekap Keterlambatan Laporan DSR<br>Cash Count | Daily Progress pelaksanaan Cash count<br>Monthly Report pelaksanaan cash count<br>ATM Support and Monitoring<br>Dan<br>Vendor Operations & Control | High |

# NON-FUNCTIONAL REQUIREMENT
Kebutuhan non‑fungsional berikut merupakan standar minimum yang harus dipenuhi:
*   Availability up to 100% (24×7) di luar jadwal pemeliharaan terencana.
*   Performa: waktu muat dashboard ≤ 3 detik (p95); unggah DSR ≤ 30 detik/dokumen (ukuran moderat); respons awal posting jurnal ≤ 5 detik dengan konfirmasi status asinkron ≤ 2 menit.
*   Skalabilitas: mendukung peningkatan jumlah vendor dan volume DSR/BA tanpa degradasi performa yang signifikan (skalabilitas horizontal untuk portal vendor).
*   Keamanan: sesuai ketentuan Bank.
*   Kepatuhan: mematuhi kebijakan internal bank dan regulasi yang relevan (privasi, keamanan, audit), pemanfaatan e‑sign tersertifikasi (opsional).
*   Observabilitas: logging terstruktur, telemetry, metrik kinerja, dan alert operasional.
*   Usability & Aksesibilitas: UI konsisten, dapat digunakan di perangkat bergerak (responsive), serta memenuhi standar aksesibilitas dasar.
## Volume Projection (if applicable)
<< can indicate, number of users / number of transaction / bandwith consumption / etc >>

| No. | Volume | Year 1 | Year 2 | Year 3 | Year 3 | Year 5 | Growth Rate (%) |
| ---| ---| ---| ---| ---| ---| ---| --- |
| 1 |  |  |  |  |  |  |  |
| 2 |  |  |  |  |  |  |  |
| 3 |  |  |  |  |  |  |  |

## Reporting / Reconciliation Requirement (if applicable)

| Requirement ID | Requirement Name | Description | Frequency | Output |
| ---| ---| ---| ---| --- |
| REP.001 | Delivery Status Reporting | Provide real-time and summary reports on the status of RFP transactions (e.g., Received, Settled). | Real-time & Daily | Dashboard / CSV / Excel |

## Performance Requirement (if applicable)

| Category | Requirement | Value |
| ---| ---| --- |
| Performance | Response time<br>Processing time<br>Peak Transaction<br>Example: |
| 1~2 seconds average service response time per transaction<br>BOD (Beginning of Day) Process: Less than 2 hours, using 8 million accounts & 200 parameter rules<br>During EOM period (Week 4) processing account up to 12 million (less than 3 hours processing)<br>Availability | Hours of operation (e.g. 7am to 11pm)<br>Uptime (eg. % or h x w)<br>Location of operation<br>Example: |
| 07.00 AM – 08.00 PM<br>24 Hours x 7 Days<br>Data Center Bintaro & NTT<br>ScalabilityThroughput / concurrent access<br>300 concurrent active user<br>Data Governance Requirement (DGCC)<br>merupakan bentuk kepatuhan terhadap UU Pelindungan Data Pribadi dan POJK 22/2023 (perlindungan konsumen) |

| # | Requirement | Y | N | If Yes, must read and fill in below column<br>If No, than N/A | DGCC Requirement | Wajib diisi |
| ---| ---| ---| ---| ---| ---| --- |
| 1 | Sistem yang dikembangkan terdapat aktivitas pengumpulan data pribadi atas nama Bank baik melalui aplikasi internal maupun pihak ketiga (misalnya : pembukaan rekening dari nasabah di channel Octo Mobile / Octo Clicks atau pembukaan OCTOPay/OCTO Savers melalui app e-commerce), |  |  | (dasar : UU PDP Pasal 21 dan POJK 22/2023 pasal 39)<br>Pastikan di requirement ini pada flow pengumpulan data pribadi tersebut sudah menambahkan dan menyimpan tickmark klausul:<br>Pernyataan Pemberitahuan Privasi\*<br>Permintaan persetujuan untuk pemrosesan Direct Marketing, 3rd Party Data Transfer, ex-Customer\*\*<br>(Bank wajib meminta persetujuan bila ada pembukaan produk Bank yang membentuk CIF baru / fasilitas untuk ETB)<br>Note: \* dan \*\*lihat appendix | Pilihan pengisian :<br>DGCC requirement sudah diterapkan<br>atau |
| DGCC requirement akan diterapkan pada UR ini.<br>2 | Sistem yang dikembangkan merupakan platform customer / public-facing baru |  |  | (dasar : UU PDP Pasal 21)<br>Pastikan di requirement ini, platform Anda harus menyampaikan:<br>1\. Pemberitahuan Privasi Bank (misalnya memasang footer link atau tab baru) yang merujuk ke halaman [https://www.cimbniaga.co.id/id/tentang-kami/pemberitahuan-privasi](https://www.cimbniaga.co.id/id/tentang-kami/pemberitahuan-privasi)<br>2\. Permintaan preferensi Cookies (untuk web-based platform) |  |
| 3 | Sistem yang Anda kembangkan melibatkan proses pengumpulan data pribadi di pihak ketiga dan pengiriman data pribadi dari pihak ketiga ke Bank |  |  | (dasar : POJK 22/2023 pasal 23 ayat 3) | Terdapat requirement pada pihak ketiga tersebut untuk memperoleh persetujuan dari subjek data (nasabah) untuk memberikan datanya kepada pihak Bank dan/atau<br>Terdapat pernyataan tertulis dari PIhak Ketiga bahwa pihak tersebut telah memperoleh persetujuan dari subyek data (nasabah) |
| 4 | Sistem yang Anda kembangkan melibatkan pihak ketiga baik di dalam proses pengembangan ataupun saat live dan / atau terdapat pengiriman data pribadi dari Bank ke pihak ketiga |  |  | (dasar :UU PDP Pasal 21)<br>Pastikan terdapat requirement untuk melakukan/membuat: | Third-Party Risk Assessment (TPRA) triage<br>Perjanjian tertulis antara Bank dan pihak ketiga tersebut dalam pemrosesan Data Pribadi (contoh: Data Processing Agreement (DPA) / Joint-Controller Processing Agreement (JCA) |

Penjelasan DGCC Requirement
_\*merujuk ke Memo No. 08/MEMO/DM&DPO/VI/2024 – Implementasi Penyampaian Pemberitahuan Privasi CIMB Niaga_
_\*\*merujuk ke Memo No. 56/MEMO/DM&DPO/VIII/2024 - Implementasi Manajemen Persetujuan terhadap Pemrosesan Data Pribadi Nasabah/Calon Nasabah di CIMB Niaga._
1. 
# Appendix
This section to add information for related Document / memo, format report screen lay out, etc
# HIGH LEVEL ESTIMATION TIMELINE

| Items | HLE Drawdown Date | HLE Project Start | HLE Project End | Remarks |
| ---| ---| ---| ---| --- |
| Timeline | Please fill target drawdown date | Please fill start date | Please fill end date | Please fill any remarks if required,<br>Example:<br>FUNC.01 – FUNC03: need to be delivered by March next year due to regulatory compliance |