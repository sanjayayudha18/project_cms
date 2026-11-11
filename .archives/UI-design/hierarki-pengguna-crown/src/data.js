// Role model mirrors 03b-SYSTEM-TABLES.md §Role model (domain_scope x role_level).
// Phase 1 = 10 active roles; the 4 Branch roles are seeded inactive and therefore
// absent from this screen until Phase 2 flips is_active.

export const ROLES = {
  ADMIN: { code: "ADMIN", domain: "all", level: "admin" },
  APPSUPPORT: { code: "APPSUPPORT", domain: "support", level: "admin" },
  ATM_USER: { code: "ATM_USER", domain: "atm", level: "maker" },
  ATM_SPV: { code: "ATM_SPV", domain: "atm", level: "checker" },
  CAM_USER: { code: "CAM_USER", domain: "cam", level: "maker" },
  CAM_SPV: { code: "CAM_SPV", domain: "cam", level: "checker" },
  APPPARAM_USER: { code: "APPPARAM_USER", domain: "parameter", level: "maker" },
  APPPARAM_SPV: { code: "APPPARAM_SPV", domain: "parameter", level: "checker" },
  VENDOR_USER: { code: "VENDOR_USER", domain: "vendor", level: "maker" },
  VENDOR_SPV: { code: "VENDOR_SPV", domain: "vendor", level: "checker" },
};

export const DOMAINS = {
  atm: { label: "ATM", note: "Order & replenishment" },
  cam: { label: "CAM", note: "Cash adjustment & monitoring" },
  parameter: { label: "Parameter", note: "Master & parameter aplikasi" },
  vendor: { label: "Vendor", note: "Portal PJPUR" },
  all: { label: "Semua domain", note: "" },
  support: { label: "Support", note: "BOD monitor & rerun" },
};

export const LEVEL_LABEL = { admin: "Admin", maker: "Maker", checker: "Checker" };

// Approval tier proposal: a checker's ceiling per approval, in IDR.
// Tier 2 has no ceiling and is required for emergency / additional replenishment.
export const TIERS = {
  1: { label: "Level 1", ceiling: 500000000, note: "Order reguler" },
  2: { label: "Level 2", ceiling: null, note: "Termasuk emergency & additional" },
};

export const VENDORS = {
  ADV: "PT Advantage SCM",
  BRG: "PT Bringin Gigantara",
  SSI: "PT Swadharma Sarana Informatika",
};

export const USERS = [
  // ── ATM ───────────────────────────────────────────────────────────────────
  { id: 4, name: "Dewi Lestari", username: "dewi.lestari@cimbniaga.co.id", role: "ATM_SPV", auth: "entra_id", supervisorId: null, tier: 1 },
  { id: 11, name: "Sanjaya Rangga", username: "sanjaya.rangga@cimbniaga.co.id", role: "ATM_SPV", auth: "entra_id", supervisorId: null, tier: 2 },
  { id: 3, name: "Budi Santoso", username: "budi.santoso@cimbniaga.co.id", role: "ATM_USER", auth: "entra_id", supervisorId: 4, tier: null },
  { id: 12, name: "Jon Son", username: "jon.son@cimbniaga.co.id", role: "ATM_USER", auth: "entra_id", supervisorId: 4, tier: null },
  { id: 13, name: "Rizky Pratama", username: "rizky.pratama@cimbniaga.co.id", role: "ATM_USER", auth: "entra_id", supervisorId: 11, tier: null },
  { id: 14, name: "Fajar Nugroho", username: "fajar.nugroho@cimbniaga.co.id", role: "ATM_USER", auth: "entra_id", supervisorId: 11, tier: null },
  { id: 15, name: "Anindita Wulandari", username: "anindita.w@cimbniaga.co.id", role: "ATM_USER", auth: "entra_id", supervisorId: null, tier: null },

  // ── CAM ───────────────────────────────────────────────────────────────────
  { id: 16, name: "Ratna Kusumaningrum", username: "ratna.kusuma@cimbniaga.co.id", role: "CAM_SPV", auth: "entra_id", supervisorId: null, tier: 2 },
  { id: 17, name: "Hendra Wijaya", username: "hendra.wijaya@cimbniaga.co.id", role: "CAM_USER", auth: "entra_id", supervisorId: 16, tier: null },
  { id: 18, name: "Siti Maimunah", username: "siti.maimunah@cimbniaga.co.id", role: "CAM_USER", auth: "entra_id", supervisorId: null, tier: null },

  // ── Parameter ─────────────────────────────────────────────────────────────
  { id: 19, name: "Yoga Prasetyo", username: "yoga.prasetyo@cimbniaga.co.id", role: "APPPARAM_SPV", auth: "entra_id", supervisorId: null, tier: 1 },
  { id: 20, name: "Nadia Safitri", username: "nadia.safitri@cimbniaga.co.id", role: "APPPARAM_USER", auth: "entra_id", supervisorId: 19, tier: null },

  // ── Vendor (auth lokal, di-scope ke satu PJPUR) ────────────────────────────
  { id: 21, name: "Agus Setiawan", username: "adv.spv01", role: "VENDOR_SPV", auth: "local", vendor: "ADV", supervisorId: null, tier: 1 },
  { id: 9, name: "Tri Handoko", username: "adv.user01", role: "VENDOR_USER", auth: "local", vendor: "ADV", supervisorId: 21, tier: null },
  { id: 22, name: "Dimas Ardiansyah", username: "adv.user02", role: "VENDOR_USER", auth: "local", vendor: "ADV", supervisorId: 21, tier: null },
  { id: 10, name: "Lina Marlina", username: "brg.user01", role: "VENDOR_USER", auth: "local", vendor: "BRG", supervisorId: null, tier: null },
  { id: 23, name: "Wayan Sudiarta", username: "ssi.spv01", role: "VENDOR_SPV", auth: "local", vendor: "SSI", supervisorId: null, tier: 2 },
  { id: 24, name: "Putri Handayani", username: "ssi.user01", role: "VENDOR_USER", auth: "local", vendor: "SSI", supervisorId: 23, tier: null },

  // ── Admin / support (di luar rantai maker-checker) ─────────────────────────
  { id: 1, name: "System Administrator", username: "svc.crown.admin", role: "ADMIN", auth: "local", supervisorId: null, tier: null },
  { id: 2, name: "Eko M Juniarto", username: "eko.juniarto@cimbniaga.co.id", role: "APPSUPPORT", auth: "entra_id", supervisorId: null, tier: null },
];

export const idr = (n) =>
  n === null || n === undefined ? "Tanpa batas" : `IDR ${new Intl.NumberFormat("id-ID").format(n)}`;

export const roleOf = (u) => ROLES[u.role];
export const levelOf = (u) => ROLES[u.role].level;
export const domainOf = (u) => ROLES[u.role].domain;

export const roleLabel = (code) => code.replace(/_/g, "-");

// Same domain_scope, maker under checker, and vendors stay inside their own PJPUR.
export function eligibility(maker, spv) {
  if (maker.id === spv.id) return { ok: false, reason: "Tidak bisa menjadi atasan dirinya sendiri" };
  if (levelOf(maker) !== "maker")
    return { ok: false, reason: `${roleLabel(maker.role)} bukan maker` };
  if (domainOf(maker) !== domainOf(spv))
    return { ok: false, reason: `Domain ${DOMAINS[domainOf(maker)].label} ≠ ${DOMAINS[domainOf(spv)].label}` };
  if (domainOf(spv) === "vendor" && maker.vendor !== spv.vendor)
    return { ok: false, reason: `Ter-scope ke ${VENDORS[maker.vendor]}` };
  return { ok: true, reason: null };
}
