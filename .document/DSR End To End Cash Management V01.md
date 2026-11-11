

### Project Overview 

|**Initiative Name**|_End To End Cash Management (Replace Opti Cash)_|
|---|---|
|**JIRA ID**|_CEO/ID/202609/091_|





<!-- Start of picture text -->
Prepared by :<br>Signed by the parties who creates this document.<br>Name :  Febrina Afivah<br>Title  :  IT Business Partner Head<br>Date  :  Signature<br>Reviewed by :<br>Name :  Budi Dwi Septianto<br>Title  :  IT Business Partner Enabler Head<br>Date  :  Signature<br>Name :  Eka Christina Dolok Pasaribu<br>Title  :  Cash & Wealth Management<br>Operation<br>Date  :  Signature<br>Name :  Robby Hartono Lamtoro<br>Title  :  Vendor Operation & Control Manager<br>Date  :  Signature<br>Name :  Agus Triyono<br>Title  :  ATM Monitoring & Support<br>Date  :  Signature<br>Approved by :<br>Name :  Eric Juna<br>Title  :  Cyber Security Advisory Head<br>Date  :  Signature<br><!-- End of picture text -->





<!-- Start of picture text -->
Name :  Nicky Cahyadi Herman<br>Title  :  Channel Operations & Settlement<br>Head<br>Date  :  Signature<br>Name :  Ketut Meliana Saputra<br>Title  :  Transaction Operational & Digital<br>Improvement Head<br>Date  :  Signature<br>Name :  Afandi<br>Title  :  IT Development Head<br>Date  :  Signature<br>Name :  Vera<br>Title  :  IT Testing Management Head<br>Date  :  Signature<br>Name :  Endrik<br>Title  : IT Infrastructur Architecture Head<br>Date : Signature<br>Name :  Ronald Alexander<br>Title : Strategy & Application Architect Head<br>Date  :  Signature<br>Name :  Yunita Kusuma Dewi<br>Title  :  IT Solution Architecture Head<br>Date  :  Signature<br><!-- End of picture text -->



#### Revision History 

|**Date**|**Version**|**Description**|**Author**|
|---|---|---|---|
|1-Sept-2026|1.0|First draft|Febrina Afivah|
|||-<br>Vendor Management||
|||-<br>ATM Replenishment||
|||-<br>Cash Count parameter||
|||-<br>Cash count Planning||
|||-<br>Cash Count Execution||





## Table of Contents 

|**_A._**|**_SOLUTION OVERVIEW .................................................................................................... 4_**|
|---|---|
|**1.**|<br>**Reference .................................................................................................................. 5**|
|**2.**|<br>**Functional Scope ........................................................................................................ 5**|
|**3.**|<br>**Integration Requirement .......................................................................................... 18**|
|**4.**|<br>**Infrastructure Requirement (if applicable) ................................................................ 18**|
|**5.**|<br>**Reporting Requirement (if applicable) ...................................................................... 18**|
|**6.**|<br>**Reconciliation Requirement (if applicable) ................................................................ 18**|
|**7.**|<br>**UI Mock Up Screen (if applicable) ............................................................................. 18**|
|**8.**|<br>**Data Governance (DGCC) requirement...................................................................... 18**|
|**9.**|<br>**Cyber Security Requirement ..................................................................................... 18**|
|**_B._**|**_Resource & Effort ......................................................................................................... 18_**|





# **A. SOLUTION OVERVIEW** 

Membangun system CROWN untuk menggantikan aplikasi OPTI Cash yang akan berperan sebagai orchestration layer dan operational control untuk proses pemenuhan uang fisik untuk kebutuhan ATM Replenish, memproses invoice dan melakukan proses cash count. 

System ini menjadi titik central pengolaan data order, workflow approval , monitoring serta integrasi dengan system internal untuk dapat meng-support flow proses dibawah ini: 



- Management vendor dan master data 

- ATM forecasting harian 

- Pemenuhan Order ATM Replenish 

- Schedule dan execution Cash Count 

- Invoicing trip ATM Replenish 

- Untuk proses pembayaran di SMART akan dilakukan di luar system (out of scope) 

### **1. Reference** 



UR New Template v0.3 - E2E Cash Manag 



### **2. Functional Scope** 

Cakupan solusi pada fase ini meliputi: 

1. Management vendor dan master data 

Modul ini digunakan untuk melakukan pemeliharaan data vendor meliputi master vendor, vault, PIC utama, kelolaan ATM dan monitoring PKS vendor. Adapun proses pemeliharaan ini akan dilakukan menggunakan proses checker dan maker melalui flow process dibawah ini: 

- Upload Master Vendor 

Menu ini digunakan untuk melakukan upload data master vendor: 

- Terdapat fitur upload dan download  untuk data master vendor. 

- System akan me-replace data master vendor yang lama dan mengganti dengan data vendor yang baru berdasarkan file yang diupload. 

- System akan mencatat perubahan yang terjadi atas mekanisme upload ini dalam audit trail (user id (checker dan maker) yang melakukan update, field yang diupdate (before-after)) 

Berikut adalah flow processnya 



|**No.**|**Tahapan Proses**|**Pelaksana**||**Deskripsi Singkat**|
|---|---|---|---|---|
|1|Upload<br>Master<br>Company<br>Vendor<br>File|<br> <br>Cash<br>&<br>Management<br>Maker|Wealth<br>Operation|<br> <br>Mengunggah file Master Company Vendor<br>melalui CROWN|
|||Cash<br>&|Wealth||
|2|Review Master Data|<br>Management<br>Maker|Operation|<br>Memeriksa master data yang telah diunggah|
|3|Check<br>Rejected<br>Data|<br>Cash<br>&<br>Management<br>Maker|Wealth<br>Operation|<br> <br>Memeriksa apakah terdapat data atau record<br>yang ditolak|





|**No.**|**Tahapan Proses**|**Pelaksana**||**Deskripsi Singkat**|
|---|---|---|---|---|
|4|Correct<br>Rejected<br>Records|<br>Cash<br>&<br>Management<br>Maker|Wealth<br>Operation|<br> <br>Memperbaiki record yang ditolak apabila<br>terdapat kesalahan data|
|5|Re-upload<br>Corrected Records|Cash<br>&<br>Management<br>Maker|Wealth<br>Operation|<br> <br>Mengunggah ulang record yang telah diperbaiki<br>melalui CROWN|
|6|Re-review<br>Master<br>Data|<br>Cash<br>&<br>Management<br>Maker|Wealth<br>Operation|<br> <br>Memeriksa kembali master data yang telah<br>diperbaiki dan diunggah ulang|
|7|Proceed<br>with<br>Approval|<br>Cash<br>&<br>Management<br>Checker|Wealth<br>Operation|<br> <br>Melakukan review terhadap master data yang<br>tidak memiliki rejected record untuk proses<br>approval|
|8|Determine<br>Approval Status|Cash<br>&<br>Management<br>Checker|Wealth<br>Operation|<br> <br>Menentukan apakah master data disetujui atau<br>ditolak|
|9|Return<br>Rejected<br>Master Data|<br>CROWN||Mengembalikan master data yang ditolak oleh<br>Checker kepada Maker untuk diperbaiki dan<br>diunggah ulang|
|10|Store Master Data|CROWN||Menyimpan master data yang telah disetujui<br>pada CROWN|
|11|<sup>Complete</sup><br>Master<br>Data Process|<br>CROWN||Mengakhiri proses setelah master data berhasil<br>disimpan|



`o` Master Data Vendor Inquiry 

Menu ini digunakan untuk melakukan inquiry data vendor untuk dapat memonitoring masa aktif PKS vendor: 

- System akan melakukan perhitungan untuk durasi expired PKS setiap kali menu ini di akses. 

- Terdapat perhitungan untuk membandingkan expiry date vendor untuk menentukan flagging “Green” (expiry date => 9 bulan ) atau “Amber” (expiry date =6 bulan dan red = 3 bulan sebelum overdue). 

- Terdapat fitur untuk dapat meng-update field status PKS secara manual untuk vendor yang sudah di “Terminate”. Proses update ini akan dilakukan secara manual oleh tim OPS berdasarkan “Nama Perusahaan” 



- System dapat menampilkan in-app notification yang menampilkan list PKS vendor yang sudah amber maupun red. In -app notification ini hanya dapat di akses oleh PIC yang authorize dengan menu ini. 

Berikut adalah flow process untuk menu ini: 



|**No.**|**Tahapan Proses**|**Pelaksana**|**Deskripsi Singkat**|
|---|---|---|---|
|1|Calculate PKS Expiry<br>Period|<br>CROWN|Menghitung sisa periode PKS berdasarkan<br>perbandingan tanggal berakhir PKS dengan<br>tanggal berjalan|
|2|Display<br>Remaining<br>Months|<br>CROWN|Menampilkan jumlah bulan yang tersisa<br>sebelum tanggal berakhir PKS|
|3|Check<br>PKS<br>Expiry<br>Period|<br>CROWN|Memeriksa apakah sisa periode PKS kurang dari<br>8 bulan sebelum tanggal berakhir|
|4|Set PKS Status to<br>Amber|<br>CROWN|Menampilkan sisa periode PKS dan menetapkan<br>status Amber apabila sisa periode PKS kurang<br>dari 6 bulan|
|5|Set PKS Status to<br>Green|<br>CROWN|Menampilkan sisa periode PKS dan menetapkan<br>status Green apabila sisa periode PKS tidak<br>kurangdari =>9 bulan|
|6|Set PKS Status to Red|CROWN|Menampilkan sisa periode PKS dan menetapkan<br>status Amber apabila sisa periode PKS = 6 bulan<br>dan Red apabila sisa periode PKS =3 bulan|
|6|Check<br>Termination<br>Status|<br>CROWN|Memeriksa apakah PKS perlu diterminasi|







<!-- Start of picture text -->
No. Tahapan Proses  Pelaksana  Deskripsi Singkat<br>Search  Company  Cash  &  Wealth  Melakukan pencarian data berdasarkan Nama<br>7<br>Information  Management Operation  Perusahaan apabila PKS akan diterminasi<br>Update PKS Status to  Cash  &  Wealth  Memperbarui status PKS menjadi Terminated<br>8<br>Terminated  Management Operation  pada CROWN<br>Complete  PKS  Mengakhiri proses setelah status PKS berhasil<br>9  CROWN<br>Monitoring Process  diperbarui<br><!-- End of picture text -->

#### Berikut adalah informasi yang perlu ditampilkan pada Menu Master Vendor: 



<!-- Start of picture text -->
No. Nama Perusahaan Alamat Kantor Pusat Nama Direktur Utama Informasi Contact Direktur Utama PKS Efektif Tanggal Efektif PKS Tanggal Expired PKS Masa Berlaku PKS Status PKS Monitoring PKS Flag<br>number text - company name text - address text - CEO name number and email text - PKS name date date number in year Active / Expired / Terminated will calculate remaining days to due date (Eg.  8 months before expired") green<br>Office�8,�Lantai�18- Perubahan dan Pernyataan<br>A,�Jalan�Jenderal�S Kembali Perjanjian Induk<br>1 PT. Abacus Cash Solution udirman�Kavling�52-53, Kota�Adm.  Djunaidi Arifien Kerjasama Jasa Layanan Cash Replenishment, Cash Collection,  1-Jan-26 31-Dec-27 1 Active 15 amber<br>Jakarta�Selatan,�Pro dan First Line Maintenance ATM,<br>vinsi�DKI�Jakarta, CDM, TST<br>JL.CIDENG�BARAT�N<br>O.49�RT�001�RW�0 Perubahan dan Pernyataan<br>02,�Desa/Kelurahan Kembali Perjanjian Induk<br>2 PT. ADVANTAGE SCM �Cideng,�Kec. Gambir,�Kota�Adm.� Wondo Julianto Hartana Kerjasama Jasa Layanan Cash Replenishment, Cash Collection,  1-Jan-26 31-Dec-27 1 Active 15<br>Jakarta�Pusat,�Provi dan First Line Maintenance ATM,<br>nsi�DKI�Jakarta,  CDM, TST<br>Kode�Pos:�10150<br>3 PT. BINTANG JASA ARTHA KELOLA Jalan Roa Malaka Utara No. 21, Desa/Kelurahan Roa Malaka,�Kec. Tambora, Kota Adm. Jakarta Barat, Provinsi DKI�Jakarta Tjatur Batoro Perubahan dan Pernyataan Kembali Perjanjian Induk Kerjasama Jasa Layanan Cash Replenishment, Cash Collection, dan First Line Maintenance ATM, CDM, TST 1-Jan-26 31-Dec-27 1 Active 15<br>Jl. Ciputat�Raya No.<br>4 PT. BRINKS SOLUTIONS INDONESIA 18, Desa/Kelurahan Pondok�Pinang, Kec. Kebayoran Lama, Kota Adm. Jakarta�Selatan,  Davidkerim Maksud Perubahan dan Pernyataan Kembali Perjanjian Induk Kerjasama tentang Jasa Layanan Uang Tunai dan Barang Berharga 1-Jan-26 31-Dec-27 1 Active 15<br>Provinsi DKI�Jakarta,<br><!-- End of picture text -->

Deskripsi dari Field Menu Master Master Vendor di bawah ini: 

|**Field**|**Deskripsi**|
|---|---|
|**Nama Perusahaan**|Nama resmi perusahaan atau mitra yang terikat dalam Perjanjian<br>Kerja Sama. Penulisan sebaiknya sesuai dengan dokumen legal<br>perusahaan.|
|**Alamat Kantor Pusat**|Alamat lengkap kantor pusat perusahaan, meliputi nama gedung<br>atau jalan, kota, provinsi, dan kode pos.|
||Nama lengkap Direktur Utama atau pimpinan tertinggi|
|**Nama Direktur Utama**|perusahaan sesuai dengan dokumen legal perusahaan yang<br>masih berlaku.|
||Informasi kontak resmi Direktur Utama yang dapat berupa nomor|
|**Informasi Kontak**|telepon, alamat email, atau informasi kontak lain yang telah|
|**Direktur Utama**|terdaftar. Karena termasuk data pribadi, akses terhadap informasi<br>ini sebaiknya dibatasi sesuai kebutuhan dan kewenangan.|
||Informasi yang menunjukkan apakah Perjanjian Kerja Sama telah|
|**PKS Efektif**|berlaku secara efektif. Field ini dapat menggunakan nilai seperti<br>**Ya/Tidak**atau**Efektif/Belum Efektif**.|





|**Tanggal Efektif PKS**|Tanggal mulai berlakunya Perjanjian Kerja Sama sesuai dengan<br>ketentuan yang tercantum dalam dokumen PKS.|
|---|---|
|**Tanggal Expired PKS**|Tanggal berakhirnya masa berlaku Perjanjian Kerja Sama. Istilah<br>yang lebih formal dapat menggunakan**Tanggal Berakhir PKS**.|
|**Masa Berlaku PKS**|Jangka waktu berlakunya Perjanjian Kerja Sama, yang dihitung<br>sejak tanggal efektif sampai dengan tanggal berakhir PKS.<br>Contohnya**1 tahun**,**2 tahun**, atau**1 Januari 2026 sampai dengan**<br>**31 Desember 2026**.|
||Kondisi terkini Perjanjian Kerja Sama, misalnya**Draft**,**Dalam**|
|**Status PKS**|**Proses Persetujuan**,**Aktif**,**Akan Berakhir**,**Berakhir**,<br>**Diperpanjang**, atau**Dihentikan**.|
|**Monitoring PKS**|Informasi untuk memantau masa berlaku dan tindak lanjut PKS,<br>termasuk sisa masa berlaku, kebutuhan perpanjangan, status<br>proses perpanjangan, dan pihak yang bertanggung jawab<br>melakukan tindak lanjut.|



`o` Master Detail kelolaan ATM vendor 

Menu ini berisi mapping nama Perusahaan dengan area vault vendor, escrow ,tipe escrow (cash/ATM) serta detail informasi ATM id yang di Kelola oleh masing-masing Nama Perusahaan yang **sudah** terdaftar pada menu “Master Vendor” diatas: 

- Apabila terdapat nama Perusahaan yang belum terdaftar pada menu “Master Vendor”, maka detail informasi Kelolaan ATM tidak dapat di simpan pada system. 

- Ops team perlu mendaftarkan nama Perusahaan yang belum terdaftar tersebut, kemudian melakukan upload ulang pada menu master vendor. 





|**No.**|**Tahapan Proses**|**Pelaksana**|**Deskripsi Singkat**|
|---|---|---|---|
|1|Upload<br>Master<br>Kelolaan ATM|Cash<br>&<br>Wealth<br>Management Operation<br>Maker|Mengunggah file Master Kelolaan ATM melalui<br>CROWN|
|2|Review Master Data|Cash<br>&<br>Wealth<br>Management Operation<br>Maker|Memeriksa master data yang telah diunggah|
|3|Check Rejected Data|Cash<br>&<br>Wealth<br>Management Operation<br>Maker|Memeriksa apakah terdapat data yang ditolak|
|4|Check<br>Company<br>Registration|Cash<br>&<br>Wealth<br>Management Operation<br>Maker|Memeriksa apakah nama perusahaan sudah<br>terdaftar|
|5|Register Company in<br>Master Vendor|Cash<br>&<br>Wealth<br>Management Operation<br>Maker|Mendaftarkan nama perusahaan dan mengunggah<br>ulang file melalui menu Master Vendor apabila<br>perusahaan belum terdaftar|
|6|Update<br>Corrected<br>Value|Cash<br>&<br>Wealth<br>Management Operation<br>Maker|Memperbaiki data yang ditolak dan memperbarui<br>file dengan nilai yang benar|





|**No.**|**Tahapan Proses**|**Pelaksana**|**Deskripsi Singkat**|
|---|---|---|---|
|7|Re-upload<br>Master<br>ATM Location Detail|Cash<br>&<br>Wealth<br>Management Operation<br>Maker|Mengunggah ulang file Master Detail Lokasi ATM<br>yang telah diperbaiki|
|8|Review and Approve<br>Master Data|Cash<br>&<br>Wealth<br>Management Operation<br>Checker|Memeriksa master data dan melakukan proses<br>approval|
|9|Determine Approval<br>Status|Cash<br>&<br>Wealth<br>Management Operation<br>Checker|Menentukan apakah master data disetujui atau<br>ditolak|
|10|<sup>Return</sup><br>Rejected<br>Data|CROWN|Mengembalikan data yang ditolak kepada Maker<br>untuk diperbaiki dan diunggah ulang|
|11|Store Master Data|CROWN|Menyimpan master data yang telah disetujui di<br>CROWN|



- Ops team dapat melakukan input dan maintain limit CIS pada masing-masing vendor  (field Limit CIS “editable) 





<!-- Start of picture text -->
No. Tahapan Proses  Pelaksana  Deskripsi Singkat<br>Melakukan pencarian berdasarkan  Nama<br>Inquiry Company and  Cash  &  Wealth<br>1  Perusahaan  dan  Area Vault Vendor  melalui<br>Vendor Vault Area  Management Operation<br>CROWN<br>Display Company and  Menampilkan nama perusahaan dan area vault<br>2  System<br>Vendor Vault Area  vendor yang sesuai dengan kriteria pencarian<br>&  Wealth  Menginput nominal Limit CIS untuk perusahaan<br>3  Input CIS Limit Amount  Cash<br>Management Operation  dan area vault vendor yang dipilih<br><!-- End of picture text -->



|**No.**|**Tahapan Proses**|**Pelaksana**|**Deskripsi Singkat**|
|---|---|---|---|
|4|Store<br>CIS<br>Limit<br>Information|<br>System|Menyimpan informasi Limit CIS yang telah<br>diinputpada CROWN|
|5|Complete<br>CIS<br>Limit<br>Setup|<br>System|Mengakhiri proses setelah informasi Limit CIS<br>berhasil disimpan|



#### Berikut adalah informasi yang perlu ditampilkan pada Menu Master Kelolaan ATM: 



<!-- Start of picture text -->
No. Nama Perusahaan Area Vault Vendor Alamat Vault Region Nama PIC Phone Contact PIC Vendor Email Address Service Type ATM Escrow Cash Escrow Limit CIS<br>number text - company name text - company and area  text - address region name text - CEO name phone number email text (eg. ATM ; Cash ; ATM and Cash count number (link to SIBcount number (link to SIB amount number<br>1 PT. Abacus Cash Solution Abacus Bandung Jl. Melati … Jawa Barat Suhartono 08… suhartono@abacus.co.id ATM 807838…. NA 500,000,000<br>2 PT. Abacus Cash Solution Abacus Subang Jawa Barat ATM 8001785xxxxx Tidak Ada CASH<br>3 PT. Abacus Cash Solution Abacus Ciamis Jawa Barat ATM & CASH 8001830xxxxx 8001829xxxxx<br>4 PT. Abacus Cash Solution Abacus Jakarta Bogor Jabodetabek ATM 8001785xxxxx Tidak Ada CASH<br>5 PT. Abacus Cash Solution Abacus Jakarta Rawamangun Jabodetabek ATM 8001785xxxxx Tidak Ada CASH<br>6 PT. Abacus Cash Solution Abacus Makassar Sulawesi CASH Tidak Ada ATM 800178xxxxx<br>7 PT. Abacus Cash Solution Abacus Medan Sumatera ATM & CASH 8001812xxxxx 80017882xxxxx<br>8 PT. Abacus Cash Solution Abacus Solo Jawa Tengah ATM & CASH 8001785xxxxx 8001784xxxxx<br>9 PT. Abacus Cash Solution Abacus Sukabumi Jawa Barat ATM 8001830xxxxx Tidak Ada CASH<br>10 PT. Abacus Cash Solution Abacus Yogyakarta Jawa Tengah ATM & CASH 8001785xxxxx 8001784xxxxx<br>11 PT. ADVANTAGE SCM Advantage Balikpapan Kalimantan ATM & CASH 8000348xxxxx 8000768xxxxx<br>12 PT. ADVANTAGE SCM Advantage Bandung Jawa Barat CASH Tidak Ada ATM 8000648xxxxx<br>13 PT. ADVANTAGE SCM Advantage Banjarmasin Kalimantan ATM & CASH 8000350xxxxx 8001022xxxxx<br>14 PT. ADVANTAGE SCM Advantage Batam Sumatera ATM & CASH 8001519xxxxx 8001693xxxxx<br>15 PT. ADVANTAGE SCM Advantage Denpasar Bali Nusra CASH Tidak Ada ATM 8000785xxxxx<br>16 PT. ADVANTAGE SCM Advantage Jakarta Meruya Jabodetabek ATM & CASH 8000349xxxxx 8000338xxxxx<br>17 PT. ADVANTAGE SCM Advantage Jakarta Rawamangun Jabodetabek CASH Tidak Ada ATM 8000338xxxxx<br>18 PT. ADVANTAGE SCM Advantage Jakarta Serang Jabodetabek ATM 8000349xxxxx Tidak Ada CASH<br>19 PT. ADVANTAGE SCM Advantage Jakarta Karawang Jabodetabek CASH Tidak Ada ATM 8000338xxxxx<br>20 PT. ADVANTAGE SCM Advantage Jambi Sumatera ATM & CASH 800035xxxxx 8001581xxxxx<br>21 PT. ADVANTAGE SCM Advantage Jember Jawa Timur CASH Tidak Ada ATM 8001261xxxxx<br>22 PT. ADVANTAGE SCM Advantage Kediri Jawa Timur CASH Tidak Ada ATM 8001693xxxxx<br>23 PT. ADVANTAGE SCM Advantage Kudus Jawa Tengah ATM & CASH 8000349xxxxx 8001467xxxxx<br>24 PT. ADVANTAGE SCM Advantage Semarang Jawa Tengah ATM & CASH 8000349xxxxx 8000675xxxxx<br>25 PT. ADVANTAGE SCM Advantage Kupang Bali Nusra ATM & CASH 8001876xxxxx 8001616xxxxx<br><!-- End of picture text -->

#### Deskripsi dari Field Menu Master Kelolaan ATM di bawah ini: 

|**Field**|**Deskripsi**|
|---|---|
|Nama Perusahaan|Nama badan usaha atau vendor yang menyediakan layanan<br>pengelolaan uang dan/atau operasional vault.|
|Area Vault Vendor|Area operasional yang mencakup atau membawahi lokasi vault vendor.|
|Alamat Vault|Alamat lengkap lokasi fisik vault vendor.|
|Region|Wilayah regional yang membawahi area operasional vault vendor.|
|Nama PIC|Nama petugas vendor yang menjadi penanggung jawab atau kontak<br>utama untuk lokasi vault tersebut.|
|Phone Contact PIC|Nomor telepon PIC vendor yang dapat dihubungi untuk koordinasi|
|Vendor|operasional.|
|Email Address|Alamat email PIC atau vendor yang digunakan untuk komunikasi,<br>notifikasi, dan pengiriman dokumen.|
|Service Type|Jenis layanan yang disediakan vendor pada lokasi vault, misalnya<br>pengelolaan kas, replenishment, atau Cash in Safe.|
|ATM Escrow|Nomor atau identitas akun escrow yang digunakan untuk pencatatan<br>dan rekonsiliasi dana terkait pengelolaan ATM.|
|Cash Escrow|Nomor atau identitas akun escrow yang digunakan untuk pencatatan<br>dan rekonsiliasi dana tunai pada Cash Vault.|





Limit CIS 

Batas maksimum nilai uang yang dapat disimpan atau dikelola dalam layanan **Cash in Safe (CIS)** sesuai ketentuan yang berlaku. 

`o` Monitoring Limit CIS 

Menu ini digunakan untuk memonitoring coverage Limit CIS dari masing-masing average balance escrow per area vault vendor: 

   - Menampilkan min balance escrow (Cash +ATM) 

   - Menampilkan max balance escrow (cash +ATM) 

   - Menampilkan average daily balance escrow (cash +ATM) 

   - Fitur untuk membandingkan value average daily balance dengan amount limit CIS, apabila average daily balance< limit CIS = covered,  average daily balance > limit CIS = not covered 

- Tidak ada proses upload pada menu ini, dan data yang ditampilkan berdasarkan hasil upload Master data vendor dan Master Detail kelolaan ATM vendor 



|No.|Tahapan Proses|Pelaksana|Deskripsi Si|ngkat|
|---|---|---|---|---|
|1|Access Monitoring Coverage Limit CIS|User|Mengakses<br>monitoring<br>Limit CIS|<br>menu<br> <br>coverage|
||||Memilih|perusahaan|
|2|Select Company and Period|User|dan<br>monitoring|periode|





|3|Get Vendor Escrow Balance|System|Mengambil<br>saldo<br>escrow ATM dan Cash|
|---|---|---|---|
|4|Calculate Total Escrow Balance|System|Menjumlahkan<br>saldo<br>escrow ATM dan Cash|
|5|Calculate Minimum Balance|System|Menghitung<br>saldo<br>escrow minimum|
|6|Calculate Maximum Balance|System|Menghitung<br>saldo<br>escrow maksimum|
|7|Calculate Average Daily Balance|System|Menghitung rata-rata<br>saldo escrow harian|
|8|Get CIS Limit|System|Mengambil Limit CIS<br>per area dan vendor|
|9|Determine CIS Limit Coverage|System|Membandingkan<br>average daily balance<br>dengan Limit CIS|
|10|Set Coverage Status|System|Menetapkan<br>status<br>Covered<br>atau<br>Not<br>Covered<br>apabila<br>average daily balance<<br>limit CIS = covered,<br>average daily balance ><br>limit CIS = not covered|
|11|Display Monitoring Result|System|Menampilkan<br>hasil<br>monitoring|



#### Berikut adalah informasi yang perlu ditampilkan pada Menu monitoring limit CIS : 



<!-- Start of picture text -->
Nama bulan  account number (link  account number  Min balance (saldo terendah dalam  Max balance (saldo tertinggi  Posisi Saldo(ambil dari SIBS  avg. data 1  J-I, if I<J,Covered, if I>J<br>number text - company name (Jan 2026) text - company and area  to SIBS) (link to SIBS) periode bulan itu) dalam periode bulan itu) bulan terakhir) - link to SIBS amount number Undercovered<br>1 PT. Abacus Cash Solution Jan-26 Abacus Bandung 807838…. 8078345 100,000,000 600,000,000 345,350,000 500,000,000 Covered<br>2 PT. Abacus Cash Solution Jan-26 Abacus Bandung 807838…. NA<br>3 PT. Abacus Cash Solution Jan-26 Abacus Subang 8001785xxxxx Tidak Ada CASH<br>4 PT. Abacus Cash Solution Jan-26 Abacus Ciamis 8001830xxxxx 8001829xxxxx<br>5 PT. Abacus Cash Solution Jan-26 Abacus Jakarta Bogor 8001785xxxxx Tidak Ada CASH<br>6 PT. Abacus Cash Solution Jan-26 Abacus Jakarta Rawamangu8001785xxxxx Tidak Ada CASH<br>7 PT. Abacus Cash Solution Jan-26 Abacus Makassar Tidak Ada ATM 800178xxxxx<br>8 PT. Abacus Cash Solution Jan-26 Abacus Medan 8001812xxxxx 80017882xxxxx<br>9 PT. Abacus Cash Solution Jan-26 Abacus Solo 8001785xxxxx 8001784xxxxx<br>10 PT. Abacus Cash Solution Jan-26 Abacus Sukabumi 8001830xxxxx Tidak Ada CASH<br>11 PT. Abacus Cash Solution Jan-26 Abacus Yogyakarta 8001785xxxxx 8001784xxxxx<br>12 PT. ADVANTAGE SCM Jan-26 Advantage Balikpapan 8000348xxxxx 8000768xxxxx<br>13 PT. ADVANTAGE SCM Jan-26 Advantage Bandung Tidak Ada ATM 8000648xxxxx<br>14 PT. ADVANTAGE SCM Jan-26 Advantage Banjarmasin 8000350xxxxx 8001022xxxxx<br>15 PT. ADVANTAGE SCM Jan-26 Advantage Batam 8001519xxxxx 8001693xxxxx<br>16 PT. ADVANTAGE SCM Jan-26 Advantage Denpasar Tidak Ada ATM 8000785xxxxx<br>17 PT. ADVANTAGE SCM Jan-26 Advantage Jakarta Meruya 8000349xxxxx 8000338xxxxx<br>18 PT. ADVANTAGE SCM Jan-26 Advantage Jakarta Rawama Tidak Ada ATM 8000338xxxxx<br>19 PT. ADVANTAGE SCM Jan-26 Advantage Jakarta Serang 8000349xxxxx Tidak Ada CASH<br>20 PT. ADVANTAGE SCM Jan-26 Advantage Jakarta Karawan Tidak Ada ATM 8000338xxxxx<br>21 PT. ADVANTAGE SCM Jan-26 Advantage Jambi 800035xxxxx 8001581xxxxx<br>22 PT. ADVANTAGE SCM Jan-26 Advantage Jember Tidak Ada ATM 8001261xxxxx<br>23 PT. ADVANTAGE SCM Jan-26 Advantage Kediri Tidak Ada ATM 8001693xxxxx<br>24 PT. ADVANTAGE SCM Jan-26 Advantage Kudus 8000349xxxxx 8001467xxxxx<br>25 PT. ADVANTAGE SCM Jan-26 Advantage Semarang 8000349xxxxx 8000675xxxxx<br>26 PT. ADVANTAGE SCM Jan-26 Advantage Kupang 8001876xxxxx 8001616xxxxx<br>Deskripsi dari Field Menu Master Monitoring Limit CIS di bawah ini:<br>Field Deskripsi<br><!-- End of picture text -->



|Nama Perusahaan|Nama perusahaan atau vendor yang mengelola ATM Vault dan<br>Cash Vault.|
|---|---|
|Periode Saldo|Periode tanggal atau bulan yang menjadi dasar pengambilan dan<br>perhitungan data saldo.|
|Area Vault Vendor|Area operasional atau lokasi vault vendor yang menjadi objek<br>pemantauan saldo.|
|ATM Escrow|Saldo pada akun escrow yang berkaitan dengan pengelolaan kas<br>untuk kebutuhan ATM pada periode yang ditentukan.|
|Cash Escrow|Saldo pada akun escrow yang berkaitan dengan pengelolaan uang<br>tunai di Cash Vault pada periode yang ditentukan.|
|Min Balance (ATM +<br>CASH)|Nilai saldo gabungan ATM Escrow dan Cash Escrow terendah<br>selama periode pelaporan.|
|Max Balance (ATM +<br>CASH)|Nilai saldo gabungan ATM Escrow dan Cash Escrow tertinggi<br>selama periode pelaporan.|
|Avg. Daily Balance (ATM<br>+ CASH)|Rata-rata saldo harian gabungan ATM Escrow dan Cash Escrow<br>selama periode pelaporan.|
||Batas maksimum nilai pertanggungan atau pengelolaan dana|
|Limit CIS|dalam layanan**Cash in Safe (CIS)**sesuai ketentuan yang<br>ditetapkan.|
|CIS Coverage|Tingkat atau persentase penggunaan Limit CIS berdasarkan saldo<br>yang menjadi dasar perhitungan.|



2. Forecasting DMAA Pada tahapan ini, DMAA mengolah data history transaksi, saldo, pola kebutuhan kas, dan data pendukung lainnya untuk menghasilkan proyeksi kebutuhan replenishment pada periode berikutnya. 

Illustration Framework: Integration with CROWN (I/O EDP) 





#### Data Source 



#### 3. ATM cash forecasting harian 

Modul ATM Forecasting & Replenishment digunakan untuk menghasilkan rekomendasi pengisian uang ATM berdasarkan hasil forecasting dan saldo aktual DSR. Modul ini mengintegrasikan data forecast dengan data vendor, permintaan emergency/adhoc, complain, dan project, serta melakukan validasi duplikasi ATM ID, penyesuaian nominal replenishment, dan pembentukan final replenishment order. Berikut adalah flow process nya : 







|**No.**|**Tahapan Proses**|**Pelaksana**|**Deskripsi Singkat**|
|---|---|---|---|
|1|Calculate and generate<br>ATM forecast|EDP|EDP menghitung dan menghasilkan data<br>forecast kebutuhan pengisian uang ATM.|
|2|Get ATM forecast result<br>(06:00)|CROWN|CROWN mengambil hasil ATM forecast pada<br>pukul 06.00.|
|3|Mapping ATM ID with<br>depot and location|CROWN|CROWN memetakan setiap ATM ID dengan<br>depot dan lokasi yang terkait.|
|4|Upload Daily Summary<br>Report (06:30–09:00)|Vendor ATM|Vendor ATM mengunggah Daily Summary<br>Report melalui CROWN pada pukul 06.30<br>sampai 09.00.|
|5|≤ 09:00 or > 09:00?|CROWN|Sistem memeriksa apakah Daily Summary<br>Report diunggah paling lambat pukul 09.00 atau<br>setelah pukul 09.00.|
|6|Upload Daily Summary<br>Report|Vendor ATM|Jika waktu telah melewati pukul 09.00, Vendor<br>ATM tetap mengunggah Daily Summary Report<br>melalui CROWN.|
|7|Log activities and flag<br>transactions with delayed<br>DSR uploads|<br>Vendor ATM /<br>CROWN|CROWN mencatat aktivitas dan memberikan<br>penanda pada transaksi yang mengalami<br>keterlambatan pengunggahan Daily Summary<br>Report.|
|8|Calculate = (Forecast<br>Replenish + Plan<br>Replenish) − (Saldo DSR +<br>Forecast Refund) (09:30–<br>10:00)|<br>CROWN|CROWN menghitung kebutuhan replenish<br>berdasarkan jumlah Forecast Replenish dan<br>Plan Replenish, dikurangi Saldo DSR dan<br>Forecast Refund, pada pukul 09.30 sampai<br>10.00.<br>*Plan replenish membaca file forecast H-1<br>(proses yang dibentuk hari sebelumnya)|
|9|Upload Complain file<br>(10:00–11:30)|ATM<br>Complain/Recon|Tim ATM Complain/Recon mengunggah file<br>complain melalui CROWN pada pukul 10.00<br>sampai 11.30.|





|10|<sup>Duplicate ATM ID with</sup><br>forecast data?|CROWN|Sistem memeriksa apakah ATM ID dalam file<br>complain juga terdapat pada data forecast.|
|---|---|---|---|
|11|<sup>Drop request from Data</sup><br>complain|CROWN|Jika ATM ID ditemukan pada data forecast,<br>CROWN mengeluarkan request ATM ID tersebut<br>dari data complain.|
|12|<sup>Process data from</sup><br>forecast data|CROWN|Setelah request dikeluarkan dari data complain,<br>CROWN memproses ATM ID menggunakan data<br>forecast.|
|13|<sup>Send Complain Email</sup><br>Notification|CROWN|CROWN mengirimkan email notifikasi satu arah<br>kepada tim ATM Complain mengenai ATM ID yang<br>teridentifikasi duplikat. Email tersebut bersifat<br>informatif dan tidak memerlukan respons atau tindak<br>lanjut melalui sistem.|
|14|<sup>Create Order with blank</sup><br>value|CROWN|Jika ATM ID tidak ditemukan pada data forecast,<br>CROWN membuat order dengan nilai replenish<br>yang masih kosong.|
|15|Update/input replenish<br>amount value for each<br>ATM ID|ATM Monitoring<br>& Support|ATM Monitoring & Support memperbarui atau<br>memasukkan nilai replenish untuk setiap ATM<br>ID yang memiliki nilai kosong.|
|16|<sup>Upload Project file</sup><br>(10:00–11:30)|ATM Business|ATM Business mengunggah file project melalui<br>CROWN pada pukul 10.00 sampai 11.30.|
|17|<sup>Duplicate ATM ID with</sup><br>forecast data?|CROWN|Sistem memeriksa apakah ATM ID dalam file<br>project juga terdapat pada data forecast.|
|18|<sup>Drop request from Data</sup><br>project|CROWN|Jika ATM ID ditemukan pada data forecast,<br>CROWN mengeluarkan request ATM ID tersebut<br>dari data project.|
|19|<sup>Process data from</sup><br>forecast data|CROWN|Setelah request dikeluarkan dari data project,<br>CROWN memproses ATM ID menggunakan data<br>forecast.|
|20|<sup>Send Complain Email</sup><br>Notification|CROWN|CROWN mengirimkan email notifikasi satu arah<br>kepada tim ATM Business mengenai ATM ID yang<br>teridentifikasi duplikat. Email tersebut bersifat|





||||informatif dan tidak memerlukan respons atau tindak<br>lanjut melalui sistem.|
|---|---|---|---|
|21|<sup>Create Order with blank</sup><br>value|CROWN|Jika ATM ID tidak ditemukan pada data forecast,<br>CROWN membuat order dengan nilai replenish<br>yang masih kosong.|
|22|Update/input replenish<br>amount value for each<br>ATM ID|ATM Monitoring<br>& Support|ATM Monitoring & Support memperbarui atau<br>memasukkan nilai replenish untuk setiap ATM<br>ID yang berasal dari data project.|
|23|<sup>Update order data</sup><br>(11:35–12:15)|CROWN|CROWN memperbarui data order berdasarkan<br>hasil forecast, complain, project, serta nilai<br>replenish yang telah dilengkapi.|
|24|Any adjustment?|CROWN|Sistem memeriksa apakah masih terdapat<br>penyesuaian terhadap nilai final replenish.|
|25|Manual adjust final<br>replenish amount (11:45–<br>12:00)|<br>ATM Monitoring<br>& Support|Jika terdapat penyesuaian, ATM Monitoring &<br>Support mengubah nilai final replenish secara<br>manual pada pukul 11.45 sampai 12.00.|
|26|<sup>Update order data</sup><br>(11:35–12:15)|CROWN|CROWN memperbarui kembali data order<br>berdasarkan hasil penyesuaian manual.|
|27|<sup>Display final Replenish</sup><br>Order (12:30)|CROWN|CROWN menampilkan final Replenish Order<br>pada pukul 12.30 setelah seluruh proses<br>perhitungan dan penyesuaian selesai.|
|28|End Forecasting Process|CROWN|Mengakhiri proses forecasting dan<br>pembentukan Final Replenishment Order.|



Final replenish Order, system akan menampilkan informasi summary final replenish per vendor vault area per denominasi dan detail per ATM ID yang dikelola oleh vendor vault area tersebut 





#### Terdapat fungsi untuk : 

   - Melakukan upload untuk laporan DSR oleh pihak ke 3 ( vendor ATM) sesuai dengan jam yang di tentukan diatas 

   - Melakukan upload untuk data complain oleh tim ATM complain dan data project tim ATM Business. 

   - System mencatat dan memberikan flagging apabila terdapat keterlambatan upload DSR oleh pihak ke 3 (vendor ATM) 

4. Pemenuhan Order ATM Replenish: 

- Setelah CROWN menampilkan final replenish amount yang akan diberikan kepada masingmasing vendor area (requestor), system akan membuatkan workflow approval dan CIT ATM replenishment  dengan flow dibawah ini : 





|**No.**|**Tahapan Proses**|**Pelaksana**|**Deskripsi Singkat**|
|---|---|---|---|
|1|Display Final Replenishment<br>Order|CROWN|Menampilkan<br>final<br>ATM<br>replenishment order|
|2|Review Final Replenishment<br>Order|ATM Monitoring Support|Memeriksa<br>final<br>replenishment<br>order sebelum diajukan|
|3|Submit Replenishment Order<br>for Approval|ATM Monitoring Support|Mengajukan<br>final<br>replenishment<br>order untuk approval|
|4|Create Approval Task|CROWN|Membuat approval task untuk ATM<br>Monitoring& Support Head|
|5|Access Approval Task List|ATM Monitoring & Support<br>Head|Membuka task list dan memeriksa<br>detail replenishment order|
|6|Approve<br>or<br>Reject<br>Replenishment Order|<br>ATM Monitoring & Support<br>Head|Menyetujui<br>atau<br>menolak<br>replenishment order|
|7|Revise Replenishment Order|ATM Forecasting Unit|Memperbaiki replenishment order<br>yangditolak|
|8|Create<br>ACM<br>Branch<br>Coordinator Task|<br>CROWN|Membuat task untuk ACM Branch<br>Coordinatorjika order disetujui|





|**No.**|**Tahapan Proses**|**Pelaksana**|**Deskripsi Singkat**|
|---|---|---|---|
|9|Review ATM Replenishment<br>CIT Request|<br>ACM Branch Coordinator|Memeriksa detail CIT request untuk<br>ATM replenishment|
|10|<sup>Check Vendor Vault Escrow</sup><br>Balance H-1|<br>ACM Branch Coordinator|Membuka<br>dan<br>memeriksa<br>ketersediaan saldo escrow vendor<br>pada H-1|
|11|Assign Cash Pickup Location|ACM Branch Coordinator|Menentukan<br>lokasi<br>pengambilan<br>uangfisik oleh vendor|
|12|Arrange Fund Transfer|ACM Branch Coordinator|Mengatur proses pindah buku untuk<br>menyiapkan<br>uang<br>fisik<br>,<br>memasukkan SOF dan destination<br>escrow number**(proses pemindah**<br>**bukuan dilakukan manual melalui**<br>**aplikasi BDS)**|
|13|Submit Cash Pickup Request|ACM Branch Coordinator|Mengajukan permintaan cash pickup<br>kepada vendor|
|14|<sup>Create Vendor Cash Pickup</sup><br>Tasks|<br>CROWN|Membuat task cash pickup untuk<br>Vendor ATM Provider dan Receiver<br>(pembuatan<br>task<br>berdasarkan<br>nomor escrow yang di input oleh<br>ACM/branch coordinator)|
|15|<sup>Complete Pickup Personnel</sup><br>Information|<br>Vendor ATM Receiver|Mengisi employee ID petugas dan<br>nomorpolisi kendaraan|
|16|Receive Cash Pickup Details|Vendor ATM Provider|Menerima detail jadwal, petugas,<br>dan kendaraan untuk cashpickup|
|17|Pick Up Physical Cash|Vendor ATM Receiver|Datang ke lokasi yang ditentukan<br>untuk mengambil uangfisik|
|18|Validate Pickup Personnel|Vendor ATM Provider|Memvalidasi employee ID petugas<br>dan nomorpolisi kendaraan|
|19|<sup>Determine</sup><br>Pickup<br>Data<br>Validity|<br>Vendor ATM Provider|Menentukan apakah data petugas<br>dan kendaraan sesuai dengan task|
|20|Reject Cash Pickup Request|Vendor ATM Provider|Menolak<br>permintaan<br>jika<br>data<br>petugas atau kendaraan tidak valid|





|**No. Tahapan Proses**|**Pelaksana**|**Deskripsi Singkat**|
|---|---|---|
|21 Hand Over Physical Cash|Vendor ATM Provider &<br>Receiver|Melakukan serah terima uang fisik<br>jika data telah valid|
|22 Close Vendor Task|Vendor<br>ATM<br>Receiver/Provider|Menutup task setelah serah terima<br>uangfisik selesai|
|23<sup>Close</sup><br>Related<br>ATM<br>Replenishment CIT Tasks|CROWN|Menutup seluruh CIT task yang<br>terkait dengan ATM replenishment|
|24 End|User/System|Proses<br>cash<br>pickup<br>dan<br>ATM<br>replenishment CIT selesai|



#### 5. Schedule dan execution Cash Count 

Cash Count adalah proses pengelolaan dan verifikasi fisik kas pada ATM atau vault yang mencakup pengaturan parameter kunjungan, perencanaan jadwal, pelaksanaan cash count, pemeriksaan pada mesin yang dipilih berdasarkan kriteria tertentu, serta rekonsiliasi hasil perhitungan fisik dengan data sistem untuk memastikan akurasi saldo dan kepatuhan operasional 

#### 1. Cash Count Parameter 

Sistem mengelola parameter cash count, seperti kategori vault, frekuensi kunjungan, metode kunjungan (online/offline), serta kriteria pemilihan ATM atau vault yang akan dilakukan cash count. 

- Category Vault: 



|**No.**|**Tahapan Proses**|**Pelaksana**|**Deskripsi Singkat**|
|---|---|---|---|
|1|Access Visit Category|<br>Tim Cash & Wealth|<br>Mengakses menu parameter kategori kunjungan|
||Parameter|Management|pada CROWN|





|**No.**|**Tahapan Proses**|**Pelaksana**|**Deskripsi Singkat**|
|---|---|---|---|
|2|Define Amount Range<br>byCategory|Tim Cash & Wealth<br>Management|Menentukan rentang nominal untuk masing-<br>masingkategori High,Medium,dan Low|
|3|Define<br>Online<br>Visit<br>Period by Category|Tim Cash & Wealth<br>Management|Menentukan periode atau frekuensi kunjungan<br>online<br>untuk<br>masing-masing<br>kategori<br>High,<br>Medium, dan Low<br>High : periode onsite 3 kali dalam 6 bulan<br>Med : periode onsite 2 kali dalam 6 bulan<br>Low: periode onsite 1 kali dalam 6 bulan<br>System akan mengatur munculnya periode onsite<br>secara random selama 6 bulan.|
|4|Complete<br>Visit<br>Category<br>Parameter<br>Setup|Tim Cash & Wealth<br>Management|Mengakhiri proses setelah parameter rentang<br>nominal dan periode kunjungan online ditentukan|



Parameter rentang nominal dan periode kunjungan: 

||**Range  Amount**|Periode Kunjungan Onsite/6 bulan|
|---|---|---|
|High|>15 bio|3x|
|Med|>5 bio<15bio|2x|
|Low|<5bio|1x|



System akan menentukan secara random periode kunjungan onsite/virtual. Informasi ini dapat di lihat oleh user pada aplikasi CROWN seperti contoh dibawah ini (tampilan dibawah ini akan berubah setiap 6 bulan secara random by system): 

|**Month**|**Jan-26**|**Feb-26**|**Mar-26**|**Apr-26**|**May-26**|**Jun-26**|
|---|---|---|---|---|---|---|
|High|Onsite|Onsite|Onsite|Virtual|Virtual|Virtual|
|Medium|Onsite|Onsite|Virtual|Virtual|Virtual|Virtual|
|Low|Virtual|Onsite|Virtual|Virtual|Virtual|Virtual|



System akan mencatat perubahan yang terjadi atas mekanisme upload ini dalam audit trail (user id (checker dan maker)yang melakukan update, field yang diupdate (before-after)) 

#### • <mark>Category Selektif Mesin :</mark> 

Menu ini digunakan untuk mendaftarkan jenis error abnormal yang akan digunakan pada proses scheduling. 







Berikut penjelasan tahapan flow process Category ATM Anomally sebagai berikut: 

|**No**|**Tahapan**|**Pelaksana**|**Deskripsi Singkat**|
|---|---|---|---|
|1|Access Parameter<br>Category Selective<br>Machine|Cash Count & Resolution|Membuka menu parameter kategori selektif<br>mesin pada CROWN.|
||Input Category||Menginput kategori transaksi ATM Anomaly|
|2|ATM Anomaly|Cash Count & Resolution|yang menjadi dasar pemilihan mesin untuk|
||Transaction||Cash Count Selektif.|
|3|Save Category<br>Parameter|CROWN|Menyimpan parameter kategori ATM<br>Anomaly yang telah diinput.|
|4|Display Category|CROWN|Menampilkan daftar kategori ATM Anomaly|
||ATM Anomaly||yang telah tersimpan dan aktif digunakan.|
|5|End Process|CROWN|Proses pengelolaan kategori ATM Anomaly<br>selesai.|



Menu ini digunakan untuk mendaftarkan informasi checklist yang akan di tampilkan pada form checklist selektif atau non selektif mesin. 





Berikut penjelasan tahapan flow process Category Checklist Form Berita Acra sebagai berikut: 

|**No**|**Tahapan**|**Pelaksana**|**Deskripsi Singkat**|
|---|---|---|---|
|1|Access Menu<br>Parameter Checklist<br>Cash Count|Cash Count &<br>Resolution|Mengakses menu pengelolaan parameter checklist<br>Cash Count.|
|2|Input Checklist Form|Cash Count &|Menginput item checklist yang akan digunakan pada|
||Berita Acara|Resolution|Form Berita Acara Cash Count.|
|3|Slt Ct|Cash Count &|Menentukan kategori checklist yaitu Selected|
||eec aegory|Resolution|Machine atau Selected Non-Machine.|
||Assin Checklist to Da|Cash Count &|Menentukan penggunaan Selected Machine|
|3|g   y||checklist untuk Cash Count Da 1 (Mesin) atau Da 2|
||1 Mesin or Day 2 Vault|Resolution|y    y<br>(Vault).|
|4|Save Parameter|CROWN|Menyimpan parameter checklist yang telah diinput<br>dan ditetapkan.|
|5|Display Checklist ATM<br>Anomaly|CROWN|Menampilkan daftar checklist yang aktif dan siap<br>digunakan pada proses Cash Count.|
|6|End Process|CROWN|Proses pengelolaan parameter checklist selesai.|



Lampiran Checklist Input H+1: 



Lampiran Checklist Input H+2: 





- Metode Kunjungan: 



|**No.**|**Tahapan Proses**|**Pelaksana**|**Deskripsi Singkat**|
|---|---|---|---|
|1|Calculate<br>6-Month<br>Average Daily Escrow<br>Balance|<br> <br>CROWN|Menghitung rata-rata saldo escrow harian berdasarkan data 6<br>bulan sebelumnya, setiap 3 hari kerja sebelum akhir bulan ke-<br>6|
|2|Categorize<br>Vendor<br>Vault|<br>CROWN|Menetapkan kategori setiap vault vendor menjadi High,<br>Medium, atau Low berdasarkan rata-rata saldo harian bulan<br>ke-6|





|**No.**|**Tahapan Proses**|**Pelaksana**|**Deskripsi Singkat**|
|---|---|---|---|
|3|Generate Initial Cash<br>Count Visit Plan|<br>CROWN|Membuat kategori awal vault vendor serta rencana<br>kunjungan cash count online atau offline untuk 6 bulan<br>berikutnya berdasarkanparameter cash count|
|4|Recalculate<br>Current<br>Month Average Daily<br>Balance|<br> <br>CROWN|Menghitung kembali rata-rata saldo harian berdasarkan data<br>bulan berjalan|
|5|Compare Current and<br>Previous<br>Vault<br>Categories|<br> <br>CROWN|Membandingkan kategori High, Medium, atau Low bulan<br>berjalan<br>dengan<br>kategori<br>pada<br>rencana<br>kunjungan<br>sebelumnya|
|6|Check Vault Category<br>Increase|<br>CROWN|Memeriksa apakah kategori vault vendor mengalami<br>peningkatan|
|7|Update Next Month’s<br>Vault Category|<br>CROWN|Memperbarui kategori vault untuk bulan berikutnya apabila<br>kategori vault mengalami peningkatan<br>Low to Med : Update to Med<br>Med to Low : Update to Low|
|8|Maintain Current Vault<br>Category|<br>CROWN|Mempertahankan kategori vault existing dan mengabaikan<br>penurunan kategori apabila kategori bulan berjalan lebih<br>rendah dari kategori bulan sebelumnya<br>Med to Low: maintain Med<br>High to Med: maintain Med|





- Master data PIC kordinator dan eksekutor: 



|**No.**|**Tahapan Proses**|**Pelaksana**|**Deskripsi Singkat**|
|---|---|---|---|
|1|Upload<br>PIC<br>Coordinator<br>Mapping|<br>Tim Cash & Wealth<br>Management|Upload data mapping PIC Coordinator yang sudah final<br>berdasarkan region pada aplikasi CROWN|
|2|Upload<br>PIC<br>Executor Mapping|<br>Tim Cash & Wealth<br>Management|Upload data mapping PIC Executor yang sudah final<br>berdasarkan areapada aplikasi CROWN|
|3|Review<br>PIC<br>MappingData|<br>Tim Cash & Wealth<br>Management|Memeriksa data mapping PIC Coordinator dan PIC<br>Executoryangtelah diupload|
|4|Check PIC Mapping<br>Changes|<br>Tim Cash & Wealth<br>Management|Memeriksa apakah terdapat penambahan, perubahan,<br>ataupenghapusan data mappingPIC|
|5|Maintain<br>PIC<br>Mapping|<br>Tim Cash & Wealth<br>Management|Menambah, mengubah, atau menghapus mapping PIC<br>Coordinator per region dan PIC Executor per area<br>apabila diperlukan|
|6|Proceed<br>with<br>Checker Workflow|<br><sup>CROWN</sup>|Meneruskan data mapping PIC ke workflow Checker<br>apabila proses review atau perubahan data telah<br>selesai|





|**No.**|**Tahapan Proses**|**Pelaksana**|**Deskripsi Singkat**|
|---|---|---|---|
|7|Review<br>PIC<br>Mapping<br>for<br>Approval|Checker|Memeriksa data mapping PIC yang diajukan dan<br>menentukan hasil approval|
|8|Return PIC Mapping<br>for Update|CROWN|Mengirimkan kembali task kepada Maker untuk<br>diperbarui apabila data mapping PIC ditolak oleh<br>Checker|
|9|Update<br>PIC<br>Mapping Data|Tim Cash & Wealth<br>Management|Memperbarui data mapping PIC sesuai hasil review<br>Checker, kemudian meneruskan kembali ke workflow<br>Checker|
|10|<sup>Save PIC Mapping</sup><br>Changes|CROWN|Menyimpan perubahan data mapping PIC apabila telah<br>disetujui oleh Checker|
|11|<sup>Complete</sup><br>PIC<br>MappingProcess|CROWN|Mengakhiri proses setelah perubahan data mapping<br>PIC berhasil disimpan|



- Mapping antara region, area, city harus sama dengan master data kelolaan ATM dan Master data ATM ID yang ada pada aplikasi ESQ 

- Informasi mapping PIC executor dan coordinator merupakan informasi yang final di luar CROWN yang telah didapatkan dari RCU nya atau atasan L2 di cabang. 

- Proses maintenance dilakukan untuk kebutuhan maintain PIC yang pindah /resign 

- Terdapat proses upload/download pada menu ini. Untuk data yang di update melalui mekanisme upload, maka akan me-replace keseluruhan data mapping yang ada di system dengan data yang baru 

- System akan mencatat perubahan yang terjadi atas mekanisme upload ini dalam audit trail (user id (checker dan maker)yang melakukan update, field yang diupdate (before-after)) 



#### • Frekuensi Kunjungan (random date): 



|**No.**|**Tahapan Proses**|**Pelaksana**|**Deskripsi Singkat**|
|---|---|---|---|
|1|Access<br>Visit<br>Date<br>Parameter|Tim Cash & Wealth<br>Management|Mengakses menu parameter tanggal kunjungan<br>melalui CROWN|
|2|Set<br>Annual<br>Scheduling<br>Parameters|<br>Tim Cash & Wealth<br>Management|Menentukan parameter yang digunakan untuk<br>penyusunan jadwal kunjungan cash count tahunan|
|3|Retrieve<br>Active<br>Vendor List|CROWN|Mengambil daftar vendor aktif beserta vault office<br>yangakan dimasukkan dalamperencanaan kunjungan|
|4|Retrieve<br>Historical<br>Visit Dates|CROWN|Mengambil informasi tanggal kunjungan masing-<br>masingvendorpada tahun-tahun sebelumnya|
|5|Retrieve<br>Annual<br>Working<br>Day<br>Calendar|CROWN|Mengambil kalender hari kerja tahunan sebagai dasar<br>penentuan tanggal kunjungan|





|**No.**|**Tahapan Proses**|**Pelaksana**|**Deskripsi Singkat**|
|---|---|---|---|
|6|Check Previous Year<br>Assigned Date|CROWN|Memeriksa apakah tanggal kunjungan yang akan<br>ditetapkan sama dengan tanggal yang pernah<br>diberikanpada tahun sebelumnya|
|7|Exclude<br>Duplicate<br>Visit Date|CROWN|Mengecualikan tanggal yang sama dengan tanggal<br>kunjunganyangdiberikanpada tahun sebelumnya|
|8|Generate<br>Eligible<br>Visit Dates|CROWN|Menghasilkan tanggal kunjungan yang memenuhi<br>parameter untuk setiapvendor atau vault office|
|9|Review Cash Count<br>Schedule|Tim Cash & Wealth<br>Management|<br>Memeriksa hasil perencanaan jadwal kunjungan cash<br>countyangdibuat oleh sistem|
|10|<sup>Determine Schedule</sup><br>Approval|Tim Cash & Wealth<br>Management|<br>Menentukan apakah jadwal kunjungan cash count<br>sudah sesuai atau masih memerlukanperubahan|
|11|<sup>Update Vendor Visit</sup><br>Date Manually|Tim Cash & Wealth<br>Management|<br>Mengubah tanggal kunjungan secara manual untuk<br>vendor atau vault office tertentu apabila jadwal belum<br>sesuai|
|12|<sup>Re-review Updated</sup><br>Schedule|Tim Cash & Wealth<br>Management|<br>Memeriksa kembali jadwal setelah tanggal kunjungan<br>vendor diperbarui secara manual|
|13|<sup>Submit Final Cash</sup><br>Count Planning|Tim Cash & Wealth<br>Management|<br>Mengirimkan perencanaan jadwal cash count final<br>setelah seluruh tanggal kunjungan dinyatakan sesuai|



#### 2. Pre-Cash Count (scheduling) 

Sistem menghasilkan dan menampilkan rencana kunjungan cash count, termasuk penjadwalan tanggal kunjungan dan penugasan PIC berdasarkan parameter dan aturan yang telah ditentukan. 







|**No.**|**Tahapan Proses**|**Pelaksana**|**Deskripsi Singkat**|
|---|---|---|---|
|1|Retrieve<br>PIC<br>MappingData|<br>CROWN|Mengambil data mapping PIC Coordinator dan PIC Executor dari<br>parameter PIC Maintenance|
|2|Retrieve<br>Vendor<br>and Vault Details|<br>CROWN|Mengambil nama vendor, vault vendor area, escrow area, tipe<br>vault,area,region,dan kota dari Vendor Management|
|3|Display Historical<br>6-Month Average<br>DailyBalance|<br> <br>CROWN|Menampilkan hasil perhitungan rata-rata saldo harian<br>berdasarkan data historis 6 bulan sebelumnya|
|4|Display<br>Current<br>Month<br>Average<br>DailyBalance|<br> <br>CROWN|Menampilkan hasil perhitungan rata-rata saldo harian<br>berdasarkan data bulan berjalan|
|5|Display<br>Vault<br>Category and Visit<br>Method|<br> <br>CROWN|Menampilkan kategori High, Medium, atau Low serta metode<br>kunjungan onsite atau online untuk setiap vault area<br>berdasarkanparameter H,M,dan L|
|6|Generate Random<br>Visit Dates|<br>CROWN|Menentukan tanggal kunjungan secara acak untuk periode 6<br>bulan berikutnya|
|7|Check<br>Multi-<br>Vendor<br>Escrow<br>Cash or ATM IDs|<br>CROWN|Memeriksa apakah terdapat nomor Escrow Cash atau ATM yang<br>sama dan dikelola oleh beberapa vendor|
|8|Maintain<br>Visit<br>Dates<br>and<br>PIC<br>Assignments|<br> <br>PIC<br>Coordinator|Apabila terdapat nomor escrow baik cash maupun ATM yang<br>sama dan di Kelola oleh beberapa vendor, PIC kordinator perlu<br>Menentukan atau memperbarui tanggal kunjungan dan<br>assignment PIC Executor untuk nomor Escrow Cash atau ATM<br>yangdikelola oleh beberapa vendor tersebut|
|9|Review<br>System-<br>Generated Plan|PIC<br>Coordinator|Memeriksa tanggal kunjungan dan assignment PIC Executor<br>yangdibuat oleh sistem|
|10|<sup>Submit Final Visit</sup><br>Plan|<br>PIC<br>Coordinator|Mengirimkan final planning kunjungan setelah tanggal dan<br>assignment PIC Executor dinyatakan sesuai|
|11|<sup>Generate Task for</sup><br>PIC Executor|<br>CROWN|Membuat dan mengirimkan task kunjungan kepada PIC<br>Executor berdasarkan finalplanning|
|12|<sup>Display</sup><br>Next<br>Month’s Schedule|<br><sup>CROWN</sup>|Membatasi PIC Executor agar hanya dapat melihat jadwal<br>kunjungan untuk 1 bulan berikutnya|
|13|<sup>Check</sup><br>Visit<br>Availability|<br>PIC Executor|<sup>Memastikan</sup><br>apakah<br>PIC<br>Executor<br>dapat<br>menjalankan<br>kunjungan sesuaijadwal|





|**No.**|**Tahapan Proses**|**Pelaksana**|**Deskripsi Singkat**|
|---|---|---|---|
|14|<sup>Submit Reschedule</sup><br>Request|PIC Executor|<sup>Mengajukan permintaan perubahan jadwal apabila kunjungan</sup><br>tidak dapat dilaksanakan|
|15|<sup>Log</sup><br>Reschedule<br>Request History|CROWN|Mencatat histori permintaan reschedule selama 1 bulan|
|16|Review<br>Reschedule<br>Request|PIC<br>Coordinator|Memeriksa permintaan reschedule dan memperbarui tanggal<br>kunjungan atau PIC Executor jika diperlukan|
|17|<sup>Confirm</sup><br>Visit<br>Schedule|PIC Executor|<sup>Mengonfirmasi jadwal kunjungan apabila kunjungan dapat</sup><br>dilaksanakan|
|18|<sup>Generate</sup><br>Cash<br>Count Audit Memo|CROWN|Menghasilkan memo dengan perihal**Audit Cash Count Vault**<br>serta mencantumkan nama dan IDpetugas|
|19|Auto-Populate<br>Officer Details into<br>Memo|CROWN|Mengisi otomatis nama dan ID petugas ke dalam memo<br>berdasarkan data PIC Executor|
|20|Send<br>Officer<br>Details to Vendor<br>ATM|CROWN|Mengirimkan nama dan ID petugas kepada Vendor ATM|
|21|<sup>Complete</sup><br>Visit<br>SchedulingProcess|<sup>CROWN</sup>|Mengakhiri<br>proses<br>setelah<br>informasi<br>petugas<br>berhasil<br>dikirimkan kepada Vendor ATM|



System dapat menampilkan overall hasil perhitungan berdasarkan parameter yang sudah di set up sebelumnya secara detail seperti dibawah ini: 



<!-- Start of picture text -->
No Vendor Vault Status Koordinatormor Escrow Amor Escrow Type Vault ATM Vault CASH Vault ATM Daily Balance CIT Daily Balance Total Balance Category 6 Mths Category 1 Mths Area Region City Total Vault PIC CC Method Jan Date Feb Date Mar Date Apr Date May Date Jun Date Jul Date<br>1 Abacus Abacus Bali Fredy 8E+11 8E+11 ATM & CASH 1 1 6,802,603,291 11,797,617,526 18,600,220,817 High Outregion Jatim Bali NDenpasar 2 Non STCC Onsite 22 25 3 7 19 12 2<br>2 Abacus Abacus Bandung Fenny 8E+11 8E+11 ATM & CASH 1 1 5,577,168,894 19,981,677,507 25,558,846,401 High Outregion Jabar Bandung 2 Non STCC Onsite 14 4 26 7 11 17 7<br>3 Abacus Abacus Subang Eka 8E+11 dak Ada CAS ATM 1 0 -     -     -    High JabodetabeJabodetabeJabodetabe 1 STCC Onsite 14 4 26 7 12 17 7<br>4 Abacus Abacus Ciamis Fenny 8E+11 8E+11 ATM & CASH 1 1 2,073,580,608 3,040,246,142 5,113,826,750 High Outregion Jabar Ciamis 2 Non STCC Onsite 7 4 5 13 20 4 24<br>5 Abacus Abacus Makassar Fredy dak Ada AT 8E+11 CASH 0 1 -    17,911,431,796 17,911,431,796 High Outregion KalimantanMakassar 1 Non STCC Onsite 7 12 3 20 5 15 3<br>6 Abacus Abacus Medan Fenny 8E+11 8E+11 ATM & CASH 1 1 13,988,075,099 83,834,543,969 97,822,619,068 High Outregion Sumatera Medan 2 Non STCC Onsite 20 9 2 14 8 12 24<br>7 Abacus Abacus Solo Fenny 8E+11 8E+11 ATM & CASH 1 1 1,568,764,199 45,745,544,462 47,314,308,662 High Outregion Jateng Solo 2 Non STCC Onsite 15 12 13 6 18 5 21<br>8 Abacus Abacus Sukabumi Fenny 8E+11 dak Ada CAS ATM 1 0 1,716,024,483  -    1,716,024,483 Medium Outregion Jabar Sukabumi 1 STCC Virtual 14 4 5 20 6 11 16<br>9 Abacus Abacus Yogyakarta Fenny 8E+11 8E+11 ATM & CASH 1 1 3,060,118,621 29,058,509,187 32,118,627,808 High Outregion Jateng Yogyakarta 2 Non STCC Onsite 7 12 12 1 12 15 3<br>10 Advantage Advantage Balikpapan Fredy 8E+11 8E+11 ATM & CASH 1 1 440,164,908 2,106,228,638 2,546,393,547 Medium Outregion KalimantanBalikpapan 2 STCC Virtual 6 9 2 20 6 8 16<br>11 Advantage Advantage Bandung Fenny dak Ada AT 8E+11 CASH 0 1 -    15,793,560,141 15,793,560,141 High Outregion Jabar Bandung 1 Non STCC Onsite 14 3 2 7 13 5 17<br>12 Advantage Advantage Banjarmasi Fredy 8E+11 8E+11 ATM & CASH 1 1 281,823,596 8,382,545,915 8,664,369,510 High Outregion KalimantanBanjarmasi 2 Non STCC Onsite 7 4 4 20 5 12 22<br>13 Advantage Advantage Batam Fenny 8E+11 8E+11 ATM & CASH 1 1 4,823,775,721 2,185,736,549 7,009,512,270 High Outregion Sumatera Batam 2 Non STCC Onsite 12 9 10 6 20 4 10<br>Berikut deskripsi dari Overall hasil perhitungan di bawah ini:<br>Field Deskripsi<br>Nama perusahaan vendor pengelola vault atau pelaksana layanan pengelolaan<br>Vendor<br>kas.<br>Vault  Nama atau identitas lokasi vault vendor yang menjadi objek Cash Count.<br><!-- End of picture text -->



|Status|Status keaktifan vault atau status pengelolaan data vault. Sebaiknya diperjelas<br>menjadi**Status Vault**apabila kolom menunjukkan aktif atau tidak aktif.|
|---|---|
|Coordinator|Nama koordinator yang bertanggung jawab atas pelaksanaan dan koordinasi<br>Cash Count pada vault terkait.|
|ATM Escrow|Nomor rekening atau akun escrow yang digunakan untuk pencatatan dan<br>rekonsiliasi dana ATM Vault.|
|CIT/Cash Escrow|Nomor rekening atau akun escrow yang digunakan untuk pencatatan dan<br>rekonsiliasi dana CIT atau Cash Vault.|
|Type Vault|Jenis vault berdasarkan layanan yang dikelola, misalnya**ATM**,**Cash**, atau**ATM**<br>**& Cash**.|
|ATM Vault|Penanda ketersediaan layanan ATM Vault. Nilai**1**menunjukkan tersedia,<br>sedangkan**0**menunjukkan tidak tersedia.|
|Cash Vault|Penanda ketersediaan layanan Cash Vault. Nilai**1**menunjukkan tersedia,<br>sedangkan**0**menunjukkan tidak tersedia.|
|ATM Daily|Nilai saldo harian dana ATM yang tercatat pada vault atau rekening escrow|
|Balance|ATM terkait.|
|CIT Daily|Nilai saldo harian dana CIT atau Cash Vault yang tercatat pada vault atau|
|Balance|rekening escrow terkait.|
|Total Balance|Total saldo gabungan ATM Daily Balance dan CIT Daily Balance.|
|Category 3 Mths|Kategori vault berdasarkan evaluasi saldo selama tiga bulan, misalnya**High**,<br>**Medium**, atau**Low**.|
|Category 1 Mths|Kategori vault berdasarkan evaluasi saldo dalam satu bulan atau periode<br>terbaru.|
|Area|Area operasional yang mencakup lokasi vault vendor, misalnya Outregion atau<br>area lain sesuai klasifikasi perusahaan.|
|Region|Wilayah regional yang membawahi lokasi vault vendor.|
|City|Kota tempat vault vendor berlokasi.|
|Total Vault|Jumlah jenis vault yang tersedia pada lokasi terkait. Contohnya, nilai**2**dapat<br>menunjukkan tersedianya ATM Vault dan Cash Vault.|
|PIC CC|Kategori atau penugasan PIC yang bertanggung jawab melaksanakan Cash<br>Count.|
|Method|Metode pelaksanaan Cash Count, seperti**Onsite**atau**Virtual**.|
|Jan Date|Tanggal pelaksanaan atau rencana Cash Count pada bulan Januari.|
|Feb Date|Tanggal pelaksanaan atau rencana Cash Count pada bulan Februari.|





|Mar Date|Tanggal pelaksanaan atau rencana Cash Count pada bulan Maret.|
|---|---|
|Apr Date|Tanggal pelaksanaan atau rencana Cash Count pada bulan April.|
|May Date|Tanggal pelaksanaan atau rencana Cash Count pada bulan Mei.|
|Jun Date|Tanggal pelaksanaan atau rencana Cash Count pada bulan Juni.|
|Jul Date|Tanggal pelaksanaan atau rencana Cash Count pada bulan Juli.|



#### 3. Cash Count Execution 

PIC Executor melaksanakan kunjungan cash count sesuai jadwal, melakukan verifikasi fisik kas, dan mendokumentasikan hasil pemeriksaan melalui checklist serta berita acara yang tersedia di sistem. 

Proses Cash Count pada sistem terdiri atas 2 bagian yaitu: 

- A. Cash Count Vault 

Proses ini bertujuan untuk memastikan bahwa jumlah uang yang disiapkan oleh vendor telah sesuai dengan kebutuhan pengisian ATM/CRM dan tercatat dengan benar. Cash Count Vault dapat dilakukan secara **onsite** (langsung datang mesin ATM/CRM) maupun **virtual** (dapat Via Whatsapp, Webex atau Teams) sesuai dengan prosedur yang berlaku. Adapun alur proses Cash Count Vault terdiri atas tahapan berikut. 



<!-- Start of picture text -->
No. Tahapan Pelaksana Deskripsi Singkat<br><!-- End of picture text -->



|1|Bring ID Card|Petugas Cash Count|Membawa kartu identitas untuk proses<br>verifikasi di lokasi vendor.|
|---|---|---|---|
|2|<sup>Validate ID</sup><br>Card|Vendor ATM|Memvalidasi identitas Petugas Cash Count<br>sebelum pelaksanaan Cash Count.|
|3|Perform Cash<br>Count by<br>Denomination|Petugas Cash Count|Menghitung uang berdasarkan denominasi<br>di dalam dan di luar vault vendor.|
|4|<sup>Input Cash</sup><br>Count Report|Petugas Cash Count|Menginput hasil perhitungan uang ke dalam<br>laporan Cash Count.|
|5|Complete Vault<br>Vendor<br>Checklist|Petugas Cash Count|Melengkapi checklist pemeriksaan vault<br>Vendor PJPUR.|
||Complete|||
|6|ATM/MDM<br>Cash Count|Petugas Cash Count|Melengkapi Berita Acara Cash Count Vault<br>ATM/MDM.|
||Minutes|||
|7|<sup>Complete Vault</sup><br>Cash Minutes|Petugas Cash Count|Melengkapi Berita Acara Cash Count Vault<br>Cash.|
|8|<sup>Retrieve DSR</sup><br>Data H-1|CROWN|Mengambil data DSR H-1 sebelum<br>pelaksanaan kunjungan.|
|9|<sup>Auto-Fill DSR</sup><br>Data|CROWN|Mengisi data DSR secara otomatis ke dalam<br>Berita Acara.|
|10|Compare DSR<br>Data with Cash<br>Count Result|Petugas Cash Count|Membandingkan data DSR H-1 dengan hasil<br>Cash Count.|
|11|Check Variance|Petugas Cash Count|Menentukan apakah terdapat selisih antara<br>data DSR dan hasil Cash Count.|
||||Melakukan perhitungan ulang apabila|
|11A|Recount Cash|Petugas Cash Count|terdapat selisih kurang dari atau sama<br>dengan Rp10 juta.|
|11B|<sup>Escalate</sup><br>Variance|Petugas Cash Count|Melakukan eskalasi melalui email apabila<br>selisih melebihi Rp10 juta.|





||Continue||Melanjutkan penyelesaian laporan apabila|
|---|---|---|---|
|11C|Report<br>Completion|Petugas Cash Count|<br>tidak terdapat selisih.|
|12|<sup>Complete and</sup><br>Submit Report|Petugas Cash Count|Melengkapi laporan dan mengirimkannya<br>untuk proses berikutnya.|
|13|Input ATM and<br>Cash Vault<br>Recap|CROWN|Menginput rekapitulasi hasil Cash Count<br>Vault ATM dan Cash.|
||Create and|||
|14|Send|CROWN|Membuat dan meneruskan task rekonsiliasi|
||Reconciliation||kepada ATM Digital & Reconciliation.|
||Task|||
||Receive and|||
|15|Review|ATM Digital &|Menerima dan memeriksa task beserta data|
||Reconciliation|Reconciliation|pendukung rekonsiliasi.|
||Task|||
|16|Input<br>Riliti|ATM Digital &|Menginput hasil rekonsiliasi berdasarkan|
||econcaon<br>Result|Reconciliation|data uang masuk dan uang keluar.|
|17|Submit<br>Reconciliation|ATM Digital &|Mengirim hasil rekonsiliasi yang telah|
||<br>Result|Reconciliation|dilengkapi.|
||Send||Mk k hil kilii k|
|18|Reconciliation<br>Result Task|CROWN|enerusan tas as reonsas epada<br>Cash Count & Resolution.|
||Input<br>|Cash Count &|Menginput keterangan berdasarkan hasil|
|19|Reconciliation<br>Remarks|Resolution|rekonsiliasi yang diterima.|
||<sup>Finalize Cash</sup>|Cash Count &|Menyelesaikan laporan Cash Count beserta|
|20|Count Report|Resolution|hasil dan keterangan rekonsiliasi.|
||<sup>Request</sup>|Cash Count &|Mengajukan laporan final untuk proses|
|21|Approval|Resolution|persetujuan D3.|
|22|<sup>Review and</sup>|D3|Membuka task, meninjau laporan, dan|
||Approve Report||memberikan keputusan persetujuan.|





|23|<sup>Approval</sup><br>Decision|D3|Menentukan tindak lanjut berdasarkan nilai<br>selisih pada laporan.|
|---|---|---|---|
|23A|<sup>Archive Recap</sup><br>Report|CROWN|Mengarsipkan laporan rekap apabila selisih<br>kurang dari atau sama dengan Rp10 juta.|
||Download and|Cash Count &|Mengunduh laporan final dan melakukan|
|23B|Escalate Final<br>Recap|<br>Resolution|eskalasi kepada Ops & IT Director apabila<br>selisih melebihi Rp10 juta.|
|24|<sup>Archive Final</sup><br>Recap Report|CROWN|Mengarsipkan laporan final setelah proses<br>persetujuan atau eskalasi selesai.|
|25|End Process|CROWN|Mengakhiri proses Cash Count Non-Selektif.|
|Terlam|pir Rekapitulasi Cash|Count Vendor sebagai|output utama proses Cash Count Non-Selektif pada ATM|
|Vault d<br>rekonsi|an Cash Vault. Rek<br>liasi yang digunakan|apitulasi tersebut mem<br>untuk kebutuhan pela|uat hasil perhitungan kas, identifikasi selisih, dan hasil<br>poran serta analisis performa vendor.|





Deskripsi dari Field Rekaptulasi Cash Count ATM Vault di bawah ini: 

|**Field**|**Deskripsi**|
|---|---|
|PIC Rekonsiliasi|Petugas dari tim ATM Digital & Reconciliation yang bertanggung jawab<br>melakukan proses rekonsiliasi.|
|Vault|Nama atau identitas lokasi vault yang menjadi objek Cash Count. Data<br>ini di dapat saat proses schedulling cash count.|
|ATM Vault<br>Category|Kategori vault ATM berdasarkan klasifikasi yang ditetapkan, misalnya<br>High, Medium, atau Low. Data ini di dapat saat proses schedulling cash<br>count.|
|Method|Metode pelaksanaan Cash Count, seperti onsite atau metode lain yang<br>ditetapkan. Data ini di dapat saat proses schedulling cash count.|





|Area|Area operasional tempat vault berada. Data ini di dapat saat proses<br>schedulling cash count.|
|---|---|
|Region|Wilayah regional yang membawahi lokasi vault. Data ini di dapat saat<br>proses schedulling cash count.|
|PIC 1|Petugas pertama yang melaksanakan atau mendampingi Cash Count.<br>Data ini di dapat saat proses schedulling cash count.|
|PIC 2|Petugas kedua yang melaksanakan atau mendampingi Cash Count.<br>Data ini di dapat saat proses schedulling cash count.|
|Actual Date|Tanggal aktual pelaksanaan Cash Count. Data ini di dapat saat proses<br>schedulling cash count.|
|DSR Date|Tanggal data DSR yang digunakan sebagai acuan rekonsiliasi.|
||Total uang fisik yang ditemukan dan dihitung pada saat pelaksanaan|
|Uang Fisik|Cash Count. Data ini di dapat saat proses pengisin checklist form Berita<br>Acara.|
||Total uang yang tercatat sedang berada dalam sirkulasi atau proses|
|Uang Beredar|operasional pada saat Cash Count. Data ini di dapat saat proses<br>pengisin checklist form Berita Acara.|
|Uang Masuk Geser<br>Vault H+0|Total uang yang masuk karena perpindahan antar-vault pada hari<br>pelaksanaan Cash Count. Data ini di dapat saat proses pengisin<br>checklist form Berita Acara.|
|Uang Bongkaran<br>H+0|Total uang hasil pembongkaran kaset ATM/CRM yang diterima pada<br>hari pelaksanaan Cash Count. Data ini di dapat saat proses pengisin<br>checklist form Berita Acara.|
||Saldo berdasarkan data DSR satu hari sebelum tanggal pelaksanaan|
|Saldo DSR H-1|Cash Count. Data ini di dapat saat proses pengisin checklist form Berita<br>Acara.|
||Perbedaan antara saldo berdasarkan hasil Cash Count dan saldo<br>pembanding sebelum proses rekonsiliasi.Data diperoleh dari pengisian|
|Selisih Cash Count|checklist pada Form Berita Acara dan dihitung menggunakan formula:<br>[Uang Fisik + Uang Beredar − Uang Masuk Geser Vault (H+0) − Uang<br>Bongkaran (H+0) − Saldo DSR H-1].|
|Remark Cash|Keterangan hasil Cash Count, seperti Tidak Selisih, Selisih Kurang, atau|
|Count|Selisih Lebih.|
|No. Escrow|Nomor referensi escrow yang digunakan untuk mengidentifikasi<br>transaksi atau saldo terkait.|





|Saldo Escrow|Saldo yang tercatat pada akun atau data escrow terkait.|
|---|---|
|Selisih Cek Escrow|Perbedaan antara saldo hasil Cash Count atau saldo rekonsiliasi dengan<br>saldo escrow. Data ini nantinya di ambil dari perhitungan [Uang Fisik +<br>Uang Beredar – Uang Masuk Geser Vault (H+0) – Uang Bongkaran (H+0)<br>- Saldo Escrow]|
|Uang Masuk/Uang<br>Keluar|Total mutasi uang masuk atau uang keluar yang digunakan sebagai<br>komponen penyesuaian dalam proses rekonsiliasi. Data ini nantinya<br>akan di lakukan input oleh team ATM Digital & Reconciliation by sistem.|
|Selisih Final|Nilai selisih akhir setelah seluruh data Cash Count, DSR, escrow, serta|
|Rekonsiliasi|uang masuk dan keluar diperhitungkan.|
|Remark Final|Keterangan atas hasil akhir rekonsiliasi, seperti Tidak Selisih, Selisih|
|Rekonsiliasi|Kurang, atau Selisih Lebih.|
|Status Rekonsiliasi|Status perkembangan proses rekonsiliasi, misalnya Open, On Progress,<br>Completed, atau status lain sesuai ketentuan.|
|Keterangan<br>Rekonsiliasi|Catatan hasil analisis, klarifikasi, penyebab selisih, atau tindak lanjut<br>atas proses rekonsiliasi. Data ini nantinya kan di lakukan input oleh<br>team Cash Count & Resolution by sistem.|
||Nilai uang yang tidak layak edar dan telah dipisahkan dari uang layak|
|UTLE|edar berdasarkan hasil pemeriksaan.Data diperoleh dari pengisian<br>checklist pada Form Berita Acara.|
||Nilai uang yang mengalami kerusakan fisik dan dicatat secara terpisah|
|Uang Rusak|dalam hasil Cash Count.Data diperoleh dari pengisian checklist pada<br>Form Berita Acara.|



B. Cash Count for selected machine 

Proses ini bertujuan untuk memastikan kondisi mesin, jumlah uang dalam kaset, serta kesesuaian hasil perhitungan uang fisik dengan data atau laporan sebagai acuan pelaksanaan cash count. Proses ini dapat dilakukan secara onsite, virtual, atau kombinasi keduanya, dengan penjadwalan terpisah yaitu H+1 untuk pemeriksaan kondisi mesin dan H+2 untuk perhitungan uang fisik pada kaset. Adapun alur proses Cash Count for Selected Machine terdiri atas tahapan berikut: 

- Schedulling Pelaksanaan Cash Count Selektif Mesin 







<!-- Start of picture text -->
No. Tahapan Proses Pelaksana Deskripsi Singkat<br>Membuka dan menampilkan daftar<br>Display Jadwal  Cash Control &<br>1  jadwal cash count selektif mesin yang<br>Scheduling Cash Count  Resolution<br>tersedia pada CROWN.<br>Membuat  jadwal  baru  untuk<br>pelaksanaan cash count selektif mesin.<br>Create Scheduling Cash  Cash Control &  Pada proses ini dalam pembuatan satu<br>2<br>Count  Resolution  surat tugas bisa digunakan untuk<br>melakukan cash count selektif mesin<br>lebih dari 1 ATM ID<br>Memilih atau memasukkan ID mesin.<br>Cash Control &  Informasi lokasi, film, SLM, vendor, dan<br>3  Input ID Mesin<br>Resolution  area terisi otomatis berdasarkan master<br>data.<br>Memasukkan  tanggal  pelaksanaan,<br>Input Tanggal, PIC, dan  Cash Control &<br>4  menentukan PIC, dan memilih metode<br>Metode Visit  Resolution<br>kunjungan Cash Count.<br><!-- End of picture text -->



|5|Submit Assign PIC 1 dan<br>2 Eksekutor|Cash Control &<br>Resolution|Mengirim penugasan kepada PIC 1 dan<br>PIC 2 yang akan bertindak sebagai<br>Petugas Cash Count.|
|---|---|---|---|
|6|Create Task Assignment|CROWN|CROWN membuat task assignment<br>dengan status awal**Assigned**.|
||Review Cash Count|Petugas Cash|Petugas Cash Count memeriksa detail|
|7|<br>Assignment|<br>Count|penugasan, termasuk mesin, lokasi,<br>jadwal, dan peran masing-masing PIC.|
||||Petugas<br>Cash<br>Count<br>menentukan<br>apakah penugasan dapat diterima atau|
|8|Accept or Reject|Petugas Cash|perlu dijadwalkan ulang. Petugas Cash|
||Assignment|Count|Count<br>menentukan<br>penerimaan<br>penugasan melalui status_Approved_<br>atau_Reschedule_.|
|9|Request Reschedule|Petugas Cash<br>Count|Mengajukan<br>permintaan<br>perubahan<br>jadwal pelaksanaan Cash Count.|
||||Menginput usulan tanggal baru dan|
|9A|Input Proposed New Date<br>and Reason|Petugas Cash<br>Count|alasan<br>reschedule.<br>Sistem<br>memperbarui<br>status<br>task<br>menjadi<br>**Requested Reschedule.**|
|9B|Review Request<br>Reschedule|Cash Control<br>Resolution|&<br>Meninjau dan memutuskan permintaan<br>reschedule.|
||Approve or Reject|Cash Control &|Menentukan<br>apakah<br>permintaan|
|10|Reschedule|Resolution|reschedule disetujui atau ditolak.|
||||Jika permintaan disetujui, jadwal Cash|
||||Count diperbarui berdasarkan tanggal|
|10A|Update Schedule|Cash Control &<br>Resolution|yang disepakati. Sistem nantinya kan<br>mengupdate<br>status<br>task<br>menjadi<br>**Reschedule.**|
||||Task penugasan diterbitkan kembali|
|10B|Return to Task<br>Assignment|CROWN|kepada Petugas Cash Count untuk<br>diperiksa dan diterima.|
||||Jika permintaan belum disetujui, Cash|
|10C|Input New Date|Cash Control &<br>Resolution|Control & Resolution memasukkan<br>alternatif tanggal baru.|





|10D|Reprocess Assignment<br>Schedule|Cash Control &<br>Resolution /<br>CROWN|Jadwal yang diperbarui diproses kembali<br>dan diteruskan ke tahapan penugasan<br>PIC.|
|---|---|---|---|
||||Jika<br>penugasan<br>disetujui,<br>CROWN|
|11|Update Status: Accepted|CROWN|memperbarui<br>status<br>penugasan|
||||menjadi**Accepted**.|
||Gnrt nd Dnld|Pt Ch|Petugas Cash Count membuat dan|
|12|eeae a owoa<br>Assignment Letter|eugas as<br>Count / CROWN|mengunduh<br>surat<br>tugas<br>melalui<br>CROWN.|
|13|Update Status: Ready for<br>Execution|CROWN|CROWN<br>memperbarui<br>status<br>task<br>menjadi**Ready for Execution**setelah<br>surat tugas tersedia.|
|14|Create ID Task Cash<br>Count Mesin|CROWN|CROWN membuat ID task unik untuk<br>proses Cash Count Mesin.|
|||Petuas Cash|Petugas Cash Count mengunggah ID|
|15|Upload ID Card|g<br>Count|Card yang akan digunakan dalam<br>pelaksanaan kunjungan.|
||Create Task for ATM||CROWN membuat task kepada ATM|
|16|<br>Vendor|CROWN|Vendor serta melampirkan surat tugas<br>dan ID Card Petugas Cash Count.|
|17|End Scheduling Process|CROWN|Proses penjadwalan selesai dan task<br>siap dilaksanakan sesuai jadwal.|



- Execution Cash Count Mesin (H+1) 





|**No.**|**Tahapan Proses**|**Pelaksana**|**Deskripsi Singkat**|
|---|---|---|---|
||||Petugas Cash Count tiba di lokasi|
|1|Arrived at Location|Petugas Cash Count|mesin sesuai dengan jadwal dan<br>surat tugas.|
||||ATM Vendor memvalidasi ID Card<br>yang dibawa oleh petugas cash|
|2|Validasi ID Card|ATM Vendor|count dengan dokumen penugasan|
||||Petugas Cash Count yang ada pada<br>aplikasi CROWN.|





||Perform ATM||Melakukan pemeriksaan fisik mesin|
|---|---|---|---|
|3|<br>Physical Inspection|Petugas Cash Count|ATM, kondisi perangkat, cassette,<br>seal, dan area sekitar mesin.|
||||Memasukkan<br>data<br>kunjungan,|
|4|Input Data|Petugas Cash Count|meliputi<br>nama<br>vendor,<br>waktu|
||Kunjungan||menghubungi PJPUR, dan waktu<br>pengecekan mesin.|
||Input Form BA||Mengisi Berita Acara pemeriksaan|
|5|Pengecekan|Petugas Cash Count|seal atau segel berdasarkan kondisi|
||Seal/Segel||aktual.|
||Input Checklist||Mengisi<br>checklist<br>pemeriksaan|
|6|Selektif Mesin Point<br>1–9|Petugas Cash Count|selektif mesin untuk poin 1 sampai<br>dengan poin 9.|
||||Melengkapi<br>checklist<br>mesin|
|7|Complete Machine|Petugas Cash Count /|berdasarkan<br>hasil<br>pemeriksaan|
||Checklist|ATM Vendor|Petugas Cash Count dan ATM<br>Vendor.|
||Uload Insection||Mengunggah<br>Inspection<br>Report,|
|8|p p<br>Report/Berita Acara|Petugas Cash Count|Berita Acara, foto, dan dokumen<br>pendukung ke dalam CROWN.|
||||Menentukan<br>apakah<br>ditemukan|
|9|Check Machine|Petugas Cash Count /|permasalahan<br>pada<br>mesin|
||Problem|CROWN|berdasarkan hasil pemeriksaan Day|
||||<br>1.|
||||**Kondisi:**<br>**Tidak**<br>**ditemukan**|
||||**permasalahan**<br>**(No).**<br>Petugas<br>Cash|
|9A|Task Closed Cash<br>Count Day 1|Petugas Cash Count|Count menutup aktivitas pemeriksaan<br>Cash Count Day 1. Sistemupdate<br>status task: Completed|
||||**Kondisi:**<br>**Ditemukan**<br>**permasalahan**<br>**(Yes).**Apabila ditemukan permasalahan|
||Estimate Resolution|Petugas Cash Count / ATM|pada<br>mesin,<br>Petugas<br>Cash<br>Count|
|10|Duration|Vendor|menginput estimasi durasi penyelesaian|
||||dan<br>alasan<br>investigasi.<br>Jika<br>durasi|
||||penyelesaian ≤ 1 bulan, status diperbarui|





||||menjadi_On Progress_; jika >|1 bulan,|
|---|---|---|---|---|
||||jadwal Cash Count Day 1 dan Da<br>dibatalkan oleh cash count re<br>berdasarkan feedback dari tim<br> <br> <br> <br>|y 2 akan<br>solution<br>petugas<br> <br>|
||||cash<br>count<br>dan<br>sistem|<br>akan|
||||menampilkan status_Cancelled_.<br>**Kondisi: Resolution ≤ 1 bulan**||
||||Memasukkan<br>hasil<br>inv|estigasi|
||Input Investigation|Petugas Cash Count/|vendor, penyebab masalah, r|encana|
|10A|<br>|<br>|penanganan,<br>dan|hasil|
||Reason|CROWN|penyelesaian.<br>Sistem|akan|
||||menampilkan<br>update|status|
||||_Onprogress._||
||Sd Tk  Ch||CROWN meneruskan task|untuk|
|10B|en as to as<br>Count & Resolution|CROWN|proses penjadwalan Cash<br>Day 2.|Count|
||Scheduling Cash|Cash Control &|Menentukan dan membuat|jadwal|
|10C|<br>Count Day 2|Resolution|pelaksanaan Cash Count Me<br>2.|sin Day|
||||Setelah pelaksanaan Cash Coun|t Day 1|
|10D|End Day 1 Process|CROWN|selesai,<br>Cash<br>Count<br>&<br>Re<br>melanjutkan proses dengan me<br>penjadwalan Cash Count Day 2.|solution<br>lakukan|
||||**Kondisi: Resolution > 1 bulan**||
||||Memasukkan<br>hasil<br>inv|estigasi|
||Estimate Resolution|Petugas Cash Count /|vendor, penyebab masalah, r|encana|
|11|<br>|<br>|penanganan,<br>dan|hasil|
||Duration|ATM Vendor|penyelesaian.<br>Tim<br>cash|count|
||||resolution  akan mengupdate<br>_Cancelled._|status|
||||Memasukkan<br>hasil<br>inv|estigasi|
|11A|Input Investigation|Petugas Cash Count|vendor, penyebab masalah, r|encana|
||Reason||penanganan,<br>dan|hasil|
||||penyelesaian.||





|11B|Task Closed Cash<br>Count Day 1|Petugas Cash Count|Menutup task Cash Count Day 1<br>karena<br>masalah<br>membutuhkan<br>penyelesaian dalam jangka panjang.|
|---|---|---|---|
|11C|Send Task to Cash<br>Count & Resolution|CROWN|CROWN meneruskan task untuk proses<br>penjadwalan Cash Count Day 2. Namun,<br>apabila<br>status<br>diperbarui<br>menjadi<br>_Cancelled_berdasarkan informasi yang<br>diterima dari Petugas Cash Count dan<br>ditindaklanjuti<br>oleh<br>Cash<br>Count<br>Resolution Team, maka sistem akan<br>memperbarui<br>informasi<br>pembatalan<br>tersebut pada jadwal Cash Count di<br>bulan berjalan.|
|11D|Stop Scheduling<br>Cash Count Day 2|Cash Control &<br>Resolution|Cash Count Day 2 tidak dilanjutkan<br>karena masalah mesin belum dapat<br>diselesaikan.|
|12|End Day 1 Process|CROWN|Proses dihentikan sampai mesin<br>siap dijadwalkan kembali.|



• Execution Cash Count Vault Area (H+2) 





|**No.**|**Tahapan**|**Pelaksana**|**Deskripsi Singkat**|
|---|---|---|---|
|1|Arrive at Vendor Vault<br>Area|Petugas Cash Count|Tiba di area vault vendor untuk<br>melaksanakan Cash Count Day 2.|
|2|Validate ID Card|ATM Vendor|Memvalidasi identitas Petugas Cash<br>Count sebelum proses dimulai.|
|3|Perform Initial ATM<br>Condition Verification|Petugas Cash Count|Memeriksa kondisi awal ATM sebelum<br>kaset dibuka.|
|4|Obtain Initial<br>Replenishment Bill<br>Counter Data|Petugas Cash Count|Mengambil data awal_replenishment_<br>_bill counter_sebagai data pembanding.|





|5|Verify Cassette Seal<br>Against Day 1 BA|Petugas Cash Count|Memastikan kondisi dan nomor segel<br>kaset sesuai dengan Berita Acara Day<br>1.|
|---|---|---|---|
|6|Perform Physical Cash<br>Count per Cassette|Petugas Cash Count|Membuka kaset dan menghitung uang<br>fisik pada setiap kaset.|
|7|Input BA Day 2 per<br>Cassette|Petugas Cash Count|Menginput hasil Cash Count setiap<br>kaset ke dalam Berita Acara Day 2.|
|8|Input Selective<br>Machine Checklist|Petugas Cash Count|Mengisi checklist selektif mesin poin<br>10–15 pada CROWN.|
||Compare Physical||Membandingkan hasil perhitungan|
|9|Cash and Remaining<br>Bill Counter|Petugas Cash Count|uang fisik dengan data_remaining bill_<br>_counter_.|
|10|Check Cash Count<br>Discrepancy|Petugas Cash Count|Menentukan apakah terdapat selisih<br>antara uang fisik dan data bill counter.|
||Complete Cash Count||Melengkapi laporan dan dokumen|
|10A|Report and Supporting<br>Documents|Petugas Cash Count|pendukung apabila tidak terdapat<br>selisih.|
|10B|Prepare Discrepancy<br>Investigation Report|ATM Vendor|Menyusun laporan investigasi<br>berdasarkan hasil pemeriksaan<br>apabila terdapat selisih.|
|10C|Input Discrepancy<br>Amount into BA Day 2|Petugas Cash Count|Menginput nominal selisih ke dalam<br>Form Berita Acara Day 2.|
|10D|Download and Sign BA<br>Form|Petugas Cash Count|Mengunduh dan menandatangani<br>Form Berita Acara Day 2.|
||Sign and Stamp||Menandatangani dan membubuhkan|
|10E|Discrepancy<br>Investigation Report|ATM Vendor|stempel vendor pada laporan<br>investigasi selisih.|
|11|Finalize Cash Count<br>Report Day 2|Petugas Cash Count|Menyelesaikan laporan Cash Count<br>Day 2 beserta dokumen pendukung.|
||Receive Cash Count|Cash Count &|Menerima laporan Cash Count Day 2|
|12|Report|Resolution|dari Petugas Cash Count.|
||Ri d Vlid|Ch C &|Meninjau hasil Cash Count serta|
|13|evew an aate<br>Cash Count Results|as ount<br>Resolution|memvalidasi kelengkapan dan<br>kesesuaian dokumen.|





||Check Document|Cash Count &|Menentukan apakah laporan dan|
|---|---|---|---|
|14|Completeness and<br>Validity|<br>Resolution|dokumen pendukung telah lengkap<br>dan valid.|
||Return Report for|Cash Count &|Mengembalikan laporan apabila<br>|
|14A|<br>Completion|Resolution|terdapat dokumen yang belum<br>lengkap atau tidak valid.|
|14B|Complete and Re-<br>upload Documents|Petugas Cash Count|Melengkapi dan mengunggah kembali<br>dokumen sesuai hasil review CCR.|
|14C|Return to CCR Review|Petugas Cash Count|Mengirimkan kembali laporan kepada<br>CCR untuk dilakukan review ulang.|
||Confirm Documents|Cash Count &|Mengonfirmasi bahwa laporan dan|
|14D|<br>Complete and Valid|<br>Resolution|seluruh dokumen pendukung telah<br>lengkap dan valid.|
||Cl Ch Ct D||Menutup task pelaksanaan Cash|
|15|ose as oun ay<br>2 Task|CROWN|Count Day 2 untuk Petugas Cash<br>Count.|
||Create and Send||Membuat dan mengirim task|
|16|<br>Reconciliation Task|CROWN|rekonsiliasi kepada ATM Digital &<br>Reconciliation.|
||Receive, Review, and|ATM Diitl &|Menerima, meninjau, dan|
|17|Confirm Cash Count<br>Results|ga<br>Reconciliation|mengonfirmasi hasil Cash Count<br>untuk proses rekonsiliasi.|
||Input Reconciliation|ATM Digital &|Menginput hasil rekonsiliasi|
|18|<br>Result|<br>Reconciliation|berdasarkan data Cash Count dan<br>data pendukung.|
||Submit Reconciliation|ATM Digital &|Mengirim hasil rekonsiliasi yang telah|
|19|Result|Reconciliation|dilengkapi.|
|20|Send Reconciliation<br>Result Task to CCR|CROWN|Meneruskan task hasil rekonsiliasi<br>kepada Cash Count & Resolution.|
||Input Reconciliation|Cash Count &|Menginput keterangan berdasarkan|
|21|Remarks|Resolution|hasil rekonsiliasi yang diterima.|
||Fili Ch C|Ch C &|Menyelesaikan laporan Cash Count|
|22|naze as ount<br>Report|as ount<br>Resolution|beserta hasil dan keterangan<br>rekonsiliasi.|





|23|Distribute Selective<br>Machine Cash Count<br>Results|Cash Count &<br>Resolution|Mendistribusikan hasil Cash Count<br>kepada pihak terkait melalui email.|
|---|---|---|---|
|24|Request Approval|Cash Count &|Mengajukan laporan untuk proses|
|||Resolution|persetujuan.|
||Ri d A||Membuka task, meninjau checklist|
|25|evew an pprove<br>Report Checklist|D3|laporan, dan memberikan<br>persetujuan.|
|26|Review and Approve|D2|Melakukan review lanjutan dan|
||Report Checklist||memberikan persetujuan akhir.|
|27|Archive Recap Report|Cash Count &|Mengarsipkan laporan rekap dan|
||and Approval|Resolution|dokumen persetujuan pada CROWN.|
|28|End Process|CROWN|Mengakhiri proses setelah laporan<br>dan persetujuan selesai diarsipkan.|



Output utama proses Cash Count adalah rekapitulasi hasil Cash Count non selektif mesin danSelektif Mesin yang mencakup hasil pemeriksaan mesin dan perhitungan uang fisik vault atau kaset. Laporan rekapitulasi akan tersedia setelah seluruh aktivitas Cash Count selesai dilaksanakan oleh Petugas Cash Count serta telah direview dan disetujui oleh Cash Count Resolution. Informasi tersebut digunakan sebagai dasar rekapitulasi vendor, rekonsiliasi hasil Cash Count, serta analisis performa vendor untuk mendukung kegiatan monitoring dan evaluasi operasional ATM. 

Terlampir Rekapitulasi Cash Count Vendor sebagai output utama proses Cash Count untuk kebutuhan rekonsiliasi, pelaporan vendor, dan analisis performa vendor. 



Deskripsi dari Field Rekaptulasi Cash Count Mesin di bawah ini: 

|**Field**|**Deskripsi**|
|---|---|
|Seal Date|Tanggal pelaksanaan pengecekan dan<br>pemasangan/verifikasi seal kaset ATM.|
|Cash Count Date|Tanggal pelaksanaan Cash Count.|





|Metode Cek Seal|Metode pelaksanaan pengecekan seal (Onsite atau<br>Virtual).|
|---|---|
|Cash Count Method|Metode pelaksanaan Cash Count (Onsite atau Virtual).|
|ATM ID|Identitas unik ATM/CRM yang dilakukan Cash Count.|
|Machine (SLM)|Jenis atau nama mesin ATM/CRM.|
|PIC Cek Seal|Petugas yang bertanggung jawab melakukan pengecekan<br>seal.|
|PIC Cash Count|Petugas yang bertanggung jawab melakukan Cash Count.|
|Vendor (FLM)|Nama vendor pengelola ATM atau PJPUR.|
|Vault Vendor|Nama vault atau lokasi penyimpanan kas vendor.<br>Perwakilan vendor yang mendampingi proses Cash Count.|
|PIC Vendor|Data ini di ambil PIC vendor yang terdaftar dalam sistem<br>yang menerima task.|
|Location|Lokasi ATM/CRM yang dilakukan Cash Count.|
|Denom|Nominal pecahan uang yang dihitung.|
|Reject|Jumlah lembar uang yang teridentifikasi sebagai reject oleh<br>mesin penghitung uang.|
|C1 - C4|Jumlah uang fisik pada Cassette 1 sampai Cassette 4|
|Sisa Fisik|Total uang fisik yang ditemukan pada seluruh kaset saat<br>Cash Count. Data ini di dapat dari Sisa Fisik * Denom|
|Remaining BC|Saldo sisa uang berdasarkan data Remaining Bill Counter.|
|Selisih|Perbedaan antara Sisa Fisik dan Remaining Bill Counter.|
|Sisa Fisik Rekon|Nilai sisa fisik setelah dilakukan proses rekonsiliasi.|
|K / L|Status hasil rekonsiliasi, yaitu Kurang (K) atau Lebih (L).|
|Catatan|Informasi tambahan, temuan, atau keterangan selama<br>Cash Count.|
|Point 1-9|Hasil checklist selektif mesin Day 1|
|Point 10-15|Hasil checklist selektif mesin Day 2|
||Total nilai temuan atau total skor checklist sesuai|
|Total|kebutuhan pelaporan. Data ini di dapatkan dari Jumlah<br>Temuan Checklist point 1-15.|
|Source|Sumber informasi atau asal data temuan.|
|Finding Category|Kategori temuan hasil Cash Count atau pemeriksaan<br>mesin.|







<!-- Start of picture text -->
Waktu terakhir terjadinya kasus/problem yang tercatat<br>Jam Terakhir Problem di<br>pada sistem ESQ. Data di ambil hasil inputan berita acara<br>ESQ<br>waktu kunjungan.<br>Waktu pertama kali vendor PJPUR dihubungi terkait<br>Jam Contact Vendor<br>penanganan kasus. Data di ambil hasil inputan berita acara<br>PJPUR<br>waktu kunjungan.<br>Status konfirmasi atau validasi yang diberikan oleh tim RPS atas<br>Konfirmasi Hasil proofing  hasil Cash Count atau temuan yang teridentifikasi. Informasi<br>RPS  konfirmasi ini diinput secara manual oleh pengguna dari tim Cash<br>Count & Resolution.<br><!-- End of picture text -->

#### 4. Cash Count Reconciliation 

Sistem membandingkan hasil perhitungan fisik kas dengan saldo yang tercatat pada sistem dan laporan operasional, kemudian mengidentifikasi serta mencatat setiap selisih untuk proses tindak lanjut dan penyelesaian. Hasil rekapitulasi ini akan dilanjutkan ke tim ATM digital & Reconciliation untuk dapat dilengkapi dengan hasil proses recon yang sudah dilakukan pada aplikasi REC7 



|**No.**|<br>**Tahapan Proses**|**Pelaksana**|**Deskripsi Singkat**|
|---|---|---|---|
|1|Display Recap BA|Sistem CROWN|Menampilkan rekap Berita Acara berdasarkan<br>saldo escrow danjenis escrow.|





|2|Review and Submit for<br>Reconciliation Process|Cash<br>Count<br>Reconciliation|&<br>Melakukan review terhadap rekap Berita Acara,<br>kemudian<br>mengajukannya<br>untuk<br>proses<br>rekonsiliasi melalui CROWN.|
|---|---|---|---|
|3|Open Task and Input<br>Reconciliation Result|ATM<br>Digital<br>Reconciliation|&<br>Membuka task rekonsiliasi dan menginput hasil<br>rekonsiliasi ke dalam CROWN.|
|4|Submit and Send Task<br>to Cash Control &<br>Resolution|ATM<br>Digital<br>Reconciliation|&<br>Melakukan submit dan mengirimkan task<br>rekonsiliasi kepada Cash Control & Resolution<br>melalui CROWN.|
|5|Finalize Cash Count<br>RecapReport|Cash<br>Control<br>Resolution|&<br>Melakukan finalisasi cash count recap report<br>berdasarkan hasil rekonsiliasi melalui CROWN.|
|6|Submit<br>Approval<br>Request|Cash<br>Control<br>Resolution|&<br>Mengajukan permintaan persetujuan atas cash<br>count recapreport melalui CROWN.|
|7|Review and Approve<br>Final Report|Cash<br>Control<br>Resolution D3|&<br>Melakukan review dan memberikan persetujuan<br>atas final report melalui CROWN.|
|8|Send Report to D1<br>Officers|Cash<br>Control<br>Resolution D3|&<br>Mendownload final report BA dan mengirimkan<br>final report kepada pejabat D1 terkait, yaitu CBO,<br>ORM, OIT Risk, CB Risk Control, Digital Banking,<br>NSO,dan SPAPM melalui email.|



#### 6. Invoicing trip ATM Replenish 

#### 1. Parameter Invoice 

Menu ini digunakan untuk mengupload skema harga sebagai dasar perhitungan invoice. Data yang diunggah akan menggantikan skema harga sebelumnya, dan tanggal unggah terbaru akan dicatat sebagai tanggal efektif skema harga 

Berikut dibawah ini adalah flow process parameter invoice: 





|**No.**|**Tahapan Proses**|**Pelaksana**|**Deskripsi Singkat**|
|---|---|---|---|
|1|Finalize the pricing<br>scheme for all ATM<br>vendors|ATM Monitoring|ATM Monitoring melakukan finalisasi skema<br>harga yang akan digunakan untuk seluruh<br>vendor ATM.|
|2|<sup>Upload the pricing</sup><br>scheme|ATM Monitoring|ATM Monitoring mengunggah skema harga<br>yang telah difinalisasi ke dalam sistem<br>CROWN.|
|3|Upload successful?|CROWN|CROWN memvalidasi keberhasilan proses<br>upload skema harga. Jika upload gagal,<br>proses dilanjutkan dengan perbaikan dan<br>upload ulang.|
|4|Correct the pricing<br>scheme and upload it<br>again|ATM Monitoring|Jika upload gagal, ATM Monitoring<br>memperbaiki skema harga dan melakukan<br>upload ulangke CROWN.|
|5|<sup>The system saves the</sup><br>pricingscheme|CROWN|Jika upload berhasil, CROWN menyimpan<br>skema harga terbaru ke dalam sistem.|
|6|The system records the<br>effective date based on<br>the latest upload date|CROWN|CROWN mencatat tanggal efektif skema<br>harga berdasarkan tanggal upload terakhir.|





|7<sup>The pricing scheme is</sup><br>active|CROWN|CROWN mengaktifkan skema harga terbaru<br>untuk digunakan dalam proses perhitungan<br>invoice.|
|---|---|---|



Berikut dibawah ini adalah informasi yang akan di upload kedalam parameter invoice (see header) dengan contoh isiannya : 

|**Vendor**<br>**Name**|**Paket**<br>**(1,2,3,dst) **|**Jenis Mesin**<br>**ATM/CDM**|**Kategori**<br>**(VIP, Reguler)**|**Price**|**Additional**<br>**Trip Price**|
|---|---|---|---|---|---|
|Abacus|1|ATM|VIP|1000|100|
|Abacus|2|ATM|VIP|1100|110|
|Abacus|4|CDM|VIP|1200|120|
|Abacus|5|ATM|Special rate|1500|150|
|Abacus|5|CDM|Non VIP|1300|130|



#### 2. Maintenace Master data ATM 

Master data ATM akan menggunakan informasi yang ada pada aplikasi ESQ. Perlu diperhatikan **penulisan format** informasi **Area, Region, City ,Provinsi, FLM vendor, FLM vendor Region, FLM vendor sub region, Paket untuk dapat dibuat standard dan sama dengan informasi yang ada pada Master Vendor** 

Berikut dibawah ini adalah flow process untuk melakukan maintenance Master data ATM apabila terdapat proses aktivasi baru, proses deactivate, pendaftaran paket dan update paket untuk ATM id terkait : 





|**No.**|**Tahapan Proses**|**Pelaksana**|**Deskripsi Singkat**|
|---|---|---|---|
|1|<sup>Open the ESQ</sup><br>application|ATM Monitoring|ATM Monitoring mengakses aplikasi ESQ untuk<br>melakukanproses maintenance ATM.|
|2|Process type?|ATM Monitoring|ATM Monitoring memilih jenis proses yang<br>akan dilakukan, yaitu**Registration**atau<br>**Deactivation**.|
|3|Register the ATM and<br>packages linked to its<br>ATM ID|ATM Monitoring|Jika memilih**Registration**, ATM Monitoring<br>mendaftarkan ATM beserta paket yang melekat<br>pada ATM ID melalui ESQ.|
|4|Deactivate the ATM<br>and packages linked<br>to its ATM ID|ATM Monitoring|Jika memilih**Deactivation**, ATM Monitoring<br>menonaktifkan ATM beserta paket yang<br>melekatpada ATM ID melalui ESQ.|
|5|Complete the maker-<br>checker process in<br>ESQ|Maker/Checker|Maker dan Checker menjalankan proses<br>pengajuan, pemeriksaan, dan persetujuan<br>sesuai alurpada ESQ.|
|6|<sup>Daily EOD (end of</sup><br>day)|ESQ|Sistem ESQ menjalankan proses akhir hari<br>untuk memproses data maintenance ATM yang<br>telah disetujui.|
|7|Generate the ATM<br>Master file at the end<br>of each day|ESQ|ESQ menghasilkan file ATM Master setiap akhir<br>hari berdasarkan data maintenance terbaru.|
|8|<sup>Place the ATM Master</sup><br>file in MFT|MFT|File ATM Master yang dihasilkan ESQ<br>ditempatkan di MFT untuk diteruskan ke<br>CROWN.|





CROWN mengambil file dari MFT dan 9<sup>Get file from MFT and</sup> CROWN memperbarui ATM Master berdasarkan update ATM master informasi terbaru. 

3. validasi terhadap trip,Harga/ Trip, dan Penalty 

CROWN akan menghitung jumlah trip yang tercatat pada system. Proses perhitungan dijadwalkan untuk dilaksanakan pada 3 (tiga) hari kerja terakhir dalam minggu pertama bulan berikutnya. 





|**No.**|**Tahapan Proses**|**Pelaksana**|**Deskripsi Singkat**|
|---|---|---|---|
|1|<sup>Retrieve the ATM master file from</sup><br>ESQ|CROWN /<br>ESQ|CROWN mengambil file ATM Master<br>dari ESQ sebagai sumber data ATM<br>untukprosesperhitungan invoice.|
|2|Run the scheduler to retrieve<br>assigned package information and<br>the total number of ATM IDs for<br>each registered ATM vendor|CROWN|Sistem menjalankan scheduler untuk<br>mengambil informasi paket yang di<br>assigned pada masing-masing ATM ID<br>dan jumlah ATM ID pada setiap<br>vendor ATM yang terdaftar (ATM<br>kelolaan).<br>System akan melakukan perhitungan<br>untuk mendapatkan maksimum trip<br>yang didapatkan berdasarkan paket<br>yang di assigned pada masing-masing<br>ATM ID, contoh:<br>10 ID ATM , Paket 3 , max trip 30/bln<br>10 ID ATM , paket 4, max trip : 40/bln<br>10 ID ATM,  paket 5 max trip : 50/bln<br>Harga per paket mencerminkan harga<br>per bulan. Untuk jumlah trip yang<br>lebih dari max trip akan<br>menggunakan harga additional<br>trip/bulan.|
|3|Perform Scheduled Activity on the<br>Last Three Business Days of the<br>First Week of the Following Month|CROWN|Tiga hari kerja sebelum akhir bulan,<br>sistem menghitung total perjalanan<br>vendor untuk setiap ATM ID pada<br>kategori ATM dan CDM.<br>Sistem menjalankan aktivitas<br>terjadwal pada tiga hari kerja terakhir<br>dalam minggu pertama bulan<br>berikutnya untuk memulai proses<br>perhitungan invoice.|
|4|<sup>Any ATM ID activated or</sup><br>deactivated this month?|CROWN|Sistem memeriksa apakah terdapat<br>ATM ID yang diaktifkan atau<br>dinonaktifkan pada periode<br>perhitungan.|
|5|<sup>Record activation / deactivation</sup><br>dates. Calculateprorated charges|CROWN|Jika terdapat aktivasi atau<br>penonaktifan,sistem mencatat|





||by package type using the active||tanggalnya dan menghitung biaya|
|---|---|---|---|
||period before deactivation or from<br>activation until month-end||secara prorata berdasarkan masa<br>aktif ATM ID.<br>Perhitungan prorate yang dilakukan:<br>-<br>**Harga perhari = harga**<br>**paket/jumlah hari pada bulan**<br>**berjalan**<br>-<br>**Durasi deactivate : tanggal mesin**<br>**diaktifkan pada bulan berjalan –**<br>**tanggal deactivate  dibulan berjalan**<br>**+1**<br>-<br>**Rumus prorate deactivate :(durasi***<br>**harga perhari)**<br>-<br>**Durasi New Activate: jumlah hari**<br>**pada bulan berjalan -tanggal aktif**<br>**pada bulan bejalan +1**<br>-<br>**Rumus prorate new activate :**<br>**Durasi* harga perhari**|
|6|Retrieve charges from invoice<br>parameters. Compare total trips<br>against the minimum trip<br>allowance per package, then apply<br>thepriceper trip|CROWN|Sistem mengambil tarif dari<br>parameter invoice, membandingkan<br>total trip dengan minimum trip setiap<br>paket, kemudian menerapkan harga<br>per trip.|
|7|<sup>Check late DSR submission flags for</sup><br>each vendor area|CROWN|Sistem memeriksa penanda<br>keterlambatan pengiriman DSR pada<br>setiaparea vendor.|
|8|Display detailed trips and charges<br>per ATM ID, including IDs activated<br>or deactivated in the current<br>month|CROWN|Sistem menampilkan detail trip dan<br>biaya per ATM ID, termasuk ATM ID<br>yang diaktifkan atau dinonaktifkan<br>dalamperiode berjalan.|
|9|Is there a penalty?|CROWN|Sistem memeriksa apakah terdapat<br>penalti yang perlu dibebankan<br>kepada vendor ATM.|
|10|<sup>Display the number of penalties</sup><br>chargeable to the ATM vendor|CROWN|Jika terdapat penalti, sistem<br>menampilkan jumlah penalti yang<br>dapat dibebankan kepada vendor<br>ATM.|
|11|<sup>Enter the penalty amount to be</sup><br>charged|ATM<br>Monitoring|ATM Monitoring memasukkan<br>nominal penalti yang akan<br>dibebankan kepada vendor ATM.|
|12|Submit billing details to the ATM<br>vendor for review. Initialize the<br>appeal round counter to 0|ATM<br>Monitoring|ATM Monitoring mengirimkan rincian<br>tagihan kepada vendor untuk<br>diperiksa. Sistem menginisialisasi|





||||penghitung putaran keberatan<br>menjadi nol.|
|---|---|---|---|
|13|<sup>Review trip charges, prorated</sup><br>charges and applicable penalties|ATM Vendor|Vendor ATM memeriksa biaya trip,<br>biaya prorata, dan penalti yang<br>dikenakan.|
|14|Any vendor adjustment or appeal?|ATM Vendor|Vendor menentukan apakah terdapat<br>penyesuaian atau keberatan atas<br>rincian tagihan.|
|15|Over than 3 appeal rounds used?|CROWN|Sistem memeriksa apakah jumlah<br>putaran pengajuan keberatan telah<br>melewati batas maksimal tiga<br>putaran.<br>untuk proses pengajuan keberatan<br>maksimum 3 kali|
|16|Reject and enter adjustment /<br>appeal details. Upload the<br>mandatory evidence file. Increase<br>the appeal round counter by 1|ATM Vendor|Vendor memasukkan detail<br>penolakan, penyesuaian, atau<br>keberatan, mengunggah bukti wajib,<br>dan menambahkan satu pada<br>penghitung putaran keberatan.|
|17|Send the vendor response and<br>evidence back to ATM Monitoring<br>for review|CROWN|CROWN mengirimkan tanggapan dan<br>bukti pendukung dari vendor kepada<br>ATM Monitoringuntuk diperiksa.|
|18|<sup>Review the vendor adjustment /</sup><br>appeal and supporting evidence|ATM<br>Monitoring|ATM Monitoring memeriksa<br>pengajuan penyesuaian atau<br>keberatan vendor beserta bukti<br>pendukungnya.|
|19|<sup>Any adjustment or appeal from</sup><br>ATM Monitoring?|ATM<br>Monitoring|ATM Monitoring menentukan apakah<br>terdapat penyesuaian atau<br>tanggapan keberatan atas pengajuan<br>vendor.|
|20|Return reviewed / adjusted details<br>to the vendor for review and<br>confirmation|ATM<br>Monitoring|Jika tidak terdapat keberatan<br>lanjutan dari ATM Monitoring, rincian<br>yang telah diperiksa atau disesuaikan<br>dikembalikan kepada vendor untuk<br>ditinjau dan dikonfirmasi.|
|21|Reject and enter adjustment /<br>appeal details. Upload the<br>mandatory evidence file. CROWN<br>sends the response back to the<br>ATM vendor|ATM<br>Monitoring|Jika terdapat keberatan, ATM<br>Monitoring memasukkan detail<br>penolakan atau penyesuaian dan<br>mengunggah bukti wajib. CROWN<br>kemudian mengirimkan tanggapan<br>kepada vendor ATM.|





|22|Confirm the billing details when<br>there is no further adjustment /<br>appeal|ATM Vendor|Vendor mengonfirmasi rincian<br>tagihan apabila sudah tidak terdapat<br>penyesuaian atau keberatan lebih<br>lanjut.|
|---|---|---|---|
|23|<sup>Display the final billing details and</sup><br>create task to the VOC team|CROWN|Sistem menampilkan rincian tagihan<br>final dan membuat tugas untuk<br>ditindaklanjuti oleh tim VOC.|
|24|<sup>Download the invoice from</sup><br>CROWN|VOC|Tim VOC mengunduh invoice yang<br>telah difinalisasi dari CROWN.|
|25|<sup>Complete the invoice package with</sup><br>the penalty memo|VOC|Tim VOC melengkapi paket dokumen<br>invoice dengan memo penalti apabila<br>terdapatpenalti.|
|26|Send the invoice and penalty<br>memo to the ATM Business team<br>via email|VOC|Tim VOC mengirimkan invoice dan<br>memo penalti kepada tim ATM<br>Business melalui email.|
|27|Process payment through SMART|ATM<br>Business|Tim ATM Business memproses<br>pembayaran invoice melalui SMART.|





### **3. Integration Requirement** 

Berikut ini adalah Integration diagram untuk aplikasi CROWN: 





### 4. **Infrastructure Requirement** (if applicable) 

Aplikasi ini akan dibangun di GCP dengan gambaran infrastructure dibawah ini: 

























### 5. **Reporting Requirement** (if applicable) 

- Berikut dibawah ini adalah format report harian untuk aktifitas ATM cash management: 

   1. Pengisian sesuai jadwal (replenish sesuai jadwal) 



2. Pengisian maju (replenish dilakukan 1 hari atau maksimum 2 hari lebih awal dari jadwal) – disesuaikan dengan kondisi hari libur. 





3. Pengisian mundur (replenish dilakukan 1 hari atau maksimum 2 hari lebih lama dari jadwal yang ditentukan) – disesuaikan dengan kondisi hari libur. 



4. Pengisian tidak dilakukan (replenish tidak dilakukan sesuai jadwal atau maksimum lebih dari dan/atau kurang dari 2 hari) 





5. Pengisian tanpa ada order (realisasi replenish dibandingkan dengan Order) setelah H+2 



6. Report refund per id dan bisa difilter per vendor per denom 





7. Report amount pengisian tidak sesuai dengan order berserta detail ID 



8. Report transaksi (Tarik dan setor cash position transaction per ATM ID) -> (nama menu di opti cash history transaction) report akan mengambil data  H+1.report akan disimpan selama 1 bulan kemudian akan di replace baru. 





9. Report Profile (id atm, Lokasi, denom, dll) 





<!-- Start of picture text -->
10. Report Oder vs Transaksi<br>11. Report User<br>12. Log User (last Login date)<br><!-- End of picture text -->



13. Report trip/Realisasi pengisian dan dapat menampilkan tertinggi, per vendor secara per id ATM. 



- Berikut dibawah ini adalah format report harian untuk aktifitas Cash Count cash management. Report final ini akan di hasilkan secara bulanan, namun informasi dapat dilihat setiap kali task proses cash count selesai dilakukan: 

   - Dashboard Summary Plan Vault: 

      - Report ini merupakan dashboard perencanaan dan pemantauan pelaksanaan cash count vault. Dashboard menampilkan jumlah vault berdasarkan kategori saldo, jenis vault, lokasi/region, penanggung jawab cash count, serta perbandingan antara rencana dan realisasi harian 

   - Report Status Cash Count Vault: 

      - Report ini merupakan data detail master dan jadwal pelaksanaan cash count untuk setiap vault vendor. Data digunakan untuk menentukan cakupan pemeriksaan, prioritas berdasarkan saldo, PIC pelaksana, metode pemeriksaan, serta jadwal cash count. 

   - DashboardSummary ATM Vault: 

Report Progress Cash Count ATM merupakan laporan bulanan untuk memantau penyelesaian cash count dan hasil rekonsiliasi ATM Vault, termasuk identifikasi selisih berupa overage dan shortage. 



- DashboardSummary CASH Vault: 

Report Progress Cash Count – CASH merupakan laporan untuk memantau penyelesaian cash count dan hasil rekonsiliasi Cash Vault, termasuk selisih overage dan shortage beserta status penyelesaiannya 

- DashboardSummary CASH Vault: 

Report Progress Cash Count – Cash Valas digunakan untuk memantau penyelesaian cash count dan hasil rekonsiliasi Cash Vault Valas, termasuk potensi selisih overage dan shortage 

- Report Selisih Cash Count Vault: 

Report ini merupakan monitoring selisih hasil cash count vault per vendor dan lokasi. Laporan mencatat jenis selisih, nilai selisih, tanggal cash count, tanggal submission, lama penyelesaian, pencapaian SLA, serta penyebab selisih 

- Report Penalty: 

Report ini merupakan monitoring temuan Cash Control & Resolution (CCR) yang berpotensi dikenakan denda kepada vendor. Laporan memuat jenis pelanggaran, objek pelanggaran, nominal denda, serta lokasi/vendor yang menjadi temuan 

- Report Temuan Vault 

   - Report ini merupakan monitoring jumlah temuan atau kejadian hasil cash count per area vault dan per periode, yang dirinci berdasarkan nomor/kategori temuan 1 sampai 20. 

- Performance Month to Month: 

Report ini merupakan dashboard perbandingan stabilitas kinerja vendor berdasarkan jumlah temuan yang dikenakan. Semakin rendah jumlah temuannya , semakin konsisten atau stabil kinerja vendor selama periode pengukuran. 

- Performance Rank-All Vendor: 

Report ini merupakan peringkat kinerja vendor per bulan berdasarkan persentase temuan cash count. Penilaian dihitung berdasarkan total area vault dan tiga kategori temuan Human Error,Workstation Support,Infrastructure 

Semakin rendah total persentase temuan, semakin baik peringkat vendor. 

- Report rekap ATM Vault, Cash Vault, Cash Valas: 

Report ini merupakan detail hasil rekonsiliasi cash count per vault yang digunakan untuk mencocokkan hasil pemeriksaan fisik dengan saldo rekening escrow, mutasi uang masuk/keluar, dan hasil rekonsiliasi akhir. 

#### Berikut adalah lempiran report cash count yang dimaksud: 



Cash Count Report Upd after meeting 22 



### 6. **Reconciliation Requirement** (if applicable) 

Bagian ini menjelaskan mengenai segala sesuatu mengenai rekonsiliasi baik secara process maupun teknis pelaksanaanya (antara lain metode rekon antar sumber data, key matching. dsb) 

7. **UI Mock Up Screen** (if applicable) 

Bagian ini menyajikan rancangan antarmuka pengguna ( _User Interface Mockup_ ) sebagai referensi visual dalam pengembangan aplikasi CROWN. 

Mockup aplikasi dapat diakses melalui tautan berikut : Mockup Aplikasi CROWN 





**8. Data Governance (DGCC) requirement** 





**9. Cyber Security Requirement** 









# **B. RESOURCE & EFFORT** 

Menjelaskan PIC Name dan rentang waktu yang di alokasi untuk para PIC tersebut 

|**No**|**Roles**|**PIC/Names**|**Effort (Weeks)**|
|---|---|---|---|
|**1**|**Business Partner**|_Please fill name of_<br>_resources_|_Please fill the date resource/s required_|
|**2**|**TPM**|_Please fill name of_<br>_resources_|_Please fill the date resource/s required_|
|**3**|**Developer …(n)**|_Please fill name of_<br>_resources_|_Please fill the date resource/s required_|
|**4**|**Testing Specialist …(n)**|_Please fill name of_<br>_resources_|_Please fill the date resource/s required_|
|**5**|**IT Cyber Security…(n)**|_Please fill name of_<br>_resources_|_Please fill the date resource/s required_|
|**6**|**IT Infrastructure …(n)**|_Please fill name of_<br>_resources_|_Please fill the date resource/s required_|
|**7**<br>**8**|**IT SME (Subject Matter**<br>**Expert)**<br>**<if applicable> …(n)**<br>**…**|_Please fill name of_<br>_resources_|_Please fill the date resource/s required_|



