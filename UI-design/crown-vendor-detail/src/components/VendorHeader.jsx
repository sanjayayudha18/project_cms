import { ArrowLeft, Building2, FileText, MapPin, ShieldCheck } from "lucide-react";
import { branches, pics, fmtIDR, tierStatus } from "../data";

export default function VendorHeader({ vendor, tiers }) {
  const activeTiers = tiers.filter((t) => tierStatus(t) === "Berlaku");
  const activePaket = new Set(activeTiers.map((t) => t.paket)).size;
  const prices = activeTiers.map((t) => t.harga);
  const range = prices.length ? `${fmtIDR(Math.max(...prices))} – ${fmtIDR(Math.min(...prices))}` : "—";
  const activeBranches = branches.filter((b) => b.is_active).length;

  const stats = [
    { icon: Building2, label: "Cabang aktif", value: `${activeBranches} dari ${branches.length}` },
    { icon: ShieldCheck, label: "PIC vendor-wide", value: `${pics.length} kontak` },
    { icon: FileText, label: "Paket berlaku", value: `${activePaket} paket` },
    { icon: MapPin, label: "Rentang harga aktif", value: range },
  ];

  return (
    <section className="pt-8">
      <a
        href="#"
        onClick={(e) => e.preventDefault()}
        className="inline-flex items-center gap-1.5 text-[13px] font-semibold text-[#C8102E] hover:text-[#8E0C22]"
      >
        <ArrowLeft size={15} /> Kembali ke daftar
      </a>

      <div className="mt-4 flex flex-wrap items-start justify-between gap-6">
        <div className="flex items-start gap-5">
          <span className="grid h-16 w-16 shrink-0 place-items-center rounded-2xl bg-[#241B19] font-['Work_Sans'] text-[20px] font-bold tracking-tight text-[#F3D9DD]">
            {vendor.code}
          </span>
          <div>
            <p className="text-[11px] font-bold uppercase tracking-[0.22em] text-[#C8102E]">
              Manajemen Vendor
            </p>
            <h1 className="mt-1 font-['Work_Sans'] text-[34px] font-bold leading-none tracking-[-0.03em] text-[#241B19]">
              {vendor.code} <span className="text-[#C9BBB2]">·</span> {vendor.name}
            </h1>
            <p className="mt-2 text-[14px] text-[#7A6A63]">
              {vendor.legalName} — data vendor beserta cabang, vault, PIC, dan paketnya.
            </p>
          </div>
        </div>
        <span className="inline-flex items-center gap-2 rounded-full border border-[#BBE3CD] bg-[#E5F4EC] px-3.5 py-1.5 text-[13px] font-semibold text-[#0E7C4A]">
          <span className="h-1.5 w-1.5 rounded-full bg-[#0E7C4A]" />
          Vendor {vendor.status}
        </span>
      </div>

      <dl className="mt-7 grid grid-cols-2 gap-px overflow-hidden rounded-xl border border-[#E9E1DA] bg-[#E9E1DA] lg:grid-cols-4">
        {stats.map(({ icon: Icon, label, value }) => (
          <div key={label} className="bg-[#FFFEFC] px-5 py-4">
            <dt className="flex items-center gap-1.5 text-[11px] font-bold uppercase tracking-[0.14em] text-[#7C6C64]">
              <Icon size={13} /> {label}
            </dt>
            <dd className="mt-1.5 truncate font-['Work_Sans'] text-[15px] font-semibold text-[#241B19]" title={value}>
              {value}
            </dd>
          </div>
        ))}
      </dl>
    </section>
  );
}
