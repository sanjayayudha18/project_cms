// Data awal halaman detail vendor (sesuai screenshot: ABA · Abacus, tab Harga Paket)

export const TODAY = new Date("2026-09-24T00:00:00");

export const vendor = {
  code: "ABA",
  name: "Abacus",
  legalName: "PT Abacus Solusi Nusantara",
  npwp: "01.234.567.8-063.000",
  address: "Menara Sudirman Lt. 18, Jl. Jend. Sudirman Kav. 60, Jakarta Selatan 12190",
  email: "vendormgmt@abacus.co.id",
  phone: "+62 21 515 0880",
  since: "2019",
  contractNo: "CTR/ABA/2025/0117",
  status: "Aktif",
};

export const branches = [
  { id: 1, branch_code: "ABA-JKT-01", branch_name: "KCP Sudirman", location_id: "LOC-0042", region: "Jakarta Selatan", is_active: 1, category: "ATM" },
  { id: 2, branch_code: "ABA-JKT-02", branch_name: "KCP Thamrin", location_id: "LOC-0057", region: "Jakarta Pusat", is_active: 1, category: "ATM" },
  { id: 3, branch_code: "ABA-BDG-01", branch_name: "KCP Asia Afrika", location_id: "LOC-0113", region: "Bandung", is_active: 1, category: "ATM" },
  { id: 4, branch_code: "ABA-SBY-01", branch_name: "KCP Tunjungan", location_id: "LOC-0148", region: "Surabaya", is_active: 1, category: "CRM" },
  { id: 5, branch_code: "ABA-SMG-01", branch_name: "KCP Pemuda", location_id: "LOC-0172", region: "Semarang", is_active: 1, category: "ATM" },
  { id: 6, branch_code: "ABA-DPS-01", branch_name: "KCP Sunset Road", location_id: "LOC-0204", region: "Denpasar", is_active: 1, category: "CRM" },
  { id: 7, branch_code: "ABA-MKS-01", branch_name: "KCP Losari", location_id: "LOC-0231", region: "Makassar", is_active: 0, category: "ATM" },
  { id: 8, branch_code: "ABA-MDN-01", branch_name: "KCP Gatot Subroto", location_id: "LOC-0260", region: "Medan", is_active: 1, category: "ATM" },
];

export const pics = [
  { id: 1, name: "Dewi Anggraini", role: "Key Account Manager", email: "dewi.anggraini@abacus.co.id", phone: "+62 811 880 2141", primary: true },
  { id: 2, name: "Rangga Pratama", role: "Operation Lead", email: "rangga.pratama@abacus.co.id", phone: "+62 812 901 5570", primary: false },
  { id: 3, name: "Sinta Maharani", role: "Finance & Billing", email: "sinta.maharani@abacus.co.id", phone: "+62 813 774 3092", primary: false },
];

// status: "active" | "ended" — seq unik per baris, dipakai untuk Kode Paket
export const initialTiers = [
  { id: 101, seq: 9, paket: "PAKET 3", mesin: "ATM", kelas: "Regular", tipe: "Standar", min: 1, max: 50, harga: 2003100, mulai: "2026-01-01", berakhir: "2027-12-31", status: "active" },
  { id: 102, seq: 10, paket: "PAKET 3", mesin: "ATM", kelas: "Regular", tipe: "Standar", min: 51, max: 100, harga: 1997200, mulai: "2026-01-01", berakhir: "2027-12-31", status: "active" },
  { id: 103, seq: 11, paket: "PAKET 3", mesin: "ATM", kelas: "Regular", tipe: "Standar", min: 101, max: 150, harga: 1991200, mulai: "2026-01-01", berakhir: "2027-12-31", status: "active" },
  { id: 104, seq: 12, paket: "PAKET 3", mesin: "ATM", kelas: "Regular", tipe: "Standar", min: 151, max: 200, harga: 1985300, mulai: "2026-01-01", berakhir: "2027-12-31", status: "active" },
  { id: 105, seq: 13, paket: "PAKET 3", mesin: "ATM", kelas: "Regular", tipe: "Standar", min: 201, max: 250, harga: 1979800, mulai: "2026-01-01", berakhir: "2027-12-31", status: "active" },
  { id: 106, seq: 14, paket: "PAKET 3", mesin: "ATM", kelas: "Regular", tipe: "Standar", min: 251, max: null, harga: 1974500, mulai: "2026-01-01", berakhir: "2027-12-31", status: "active" },
  { id: 201, seq: 5, paket: "PAKET 2", mesin: "CRM", kelas: "Regular", tipe: "Standar", min: 1, max: 50, harga: 2450000, mulai: "2026-01-01", berakhir: "2026-12-31", status: "active" },
  { id: 202, seq: 6, paket: "PAKET 2", mesin: "CRM", kelas: "Regular", tipe: "Standar", min: 51, max: 100, harga: 2385000, mulai: "2026-01-01", berakhir: "2026-12-31", status: "active" },
  { id: 203, seq: 7, paket: "PAKET 2", mesin: "CRM", kelas: "Regular", tipe: "Standar", min: 101, max: 150, harga: 2330000, mulai: "2026-01-01", berakhir: "2026-12-31", status: "active" },
  { id: 204, seq: 8, paket: "PAKET 2", mesin: "CRM", kelas: "Regular", tipe: "Standar", min: 151, max: null, harga: 2275000, mulai: "2026-01-01", berakhir: "2026-12-31", status: "active" },
  { id: 301, seq: 1, paket: "PAKET 1", mesin: "ATM", kelas: "Regular", tipe: "Standar", min: 1, max: 50, harga: 1950000, mulai: "2024-01-01", berakhir: "2025-12-31", status: "ended" },
  { id: 302, seq: 2, paket: "PAKET 1", mesin: "ATM", kelas: "Regular", tipe: "Standar", min: 51, max: 100, harga: 1890000, mulai: "2024-01-01", berakhir: "2025-12-31", status: "ended" },
  { id: 303, seq: 3, paket: "PAKET 1", mesin: "ATM", kelas: "Regular", tipe: "Standar", min: 101, max: 150, harga: 1840000, mulai: "2024-01-01", berakhir: "2025-12-31", status: "ended" },
  { id: 304, seq: 4, paket: "PAKET 1", mesin: "ATM", kelas: "Regular", tipe: "Standar", min: 151, max: null, harga: 1790000, mulai: "2024-01-01", berakhir: "2025-12-31", status: "ended" },
];

const idr = new Intl.NumberFormat("id-ID", { maximumFractionDigits: 0 });
export const fmtIDR = (n) => `IDR ${idr.format(n)}`;
export const fmtNum = (n) => idr.format(n);

const MONTHS = ["Jan", "Feb", "Mar", "Apr", "Mei", "Jun", "Jul", "Agu", "Sep", "Okt", "Nov", "Des"];
export const fmtDate = (iso) => {
  const d = new Date(`${iso}T00:00:00`);
  return `${d.getDate()} ${MONTHS[d.getMonth()]} ${d.getFullYear()}`;
};

export const tierLabel = (t) => (t.max == null ? `${t.min}+` : `${t.min}\u2013${t.max}`);

export const tierStatus = (t) => {
  if (t.status === "ended") return "Berakhir";
  if (new Date(`${t.mulai}T00:00:00`) > TODAY) return "Dijadwalkan";
  return "Berlaku";
};

export const groupKey = (t) => `${t.paket}||${t.mesin}||${t.kelas}`;

export const kodePaket = (t) => `PKG${t.paket.replace(/\D/g, "") || "0"}_${vendor.code}_${String(t.seq ?? 0).padStart(3, "0")}`;

export const nextSeq = (tiers) => Math.max(0, ...tiers.map((t) => t.seq ?? 0)) + 1;
