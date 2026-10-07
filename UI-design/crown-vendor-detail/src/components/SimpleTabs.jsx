import { Building2, Mail, MapPin, Phone, Star, User } from "lucide-react";
import { branches, pics } from "../data";

const card = "rounded-xl border border-[#E9E1DA] bg-[#FFFEFC]";

export function InfoTab({ vendor }) {
  const fields = [
    { label: "Kode vendor", value: vendor.code },
    { label: "Nama legal", value: vendor.legalName },
    { label: "NPWP", value: vendor.npwp },
    { label: "No. kontrak", value: vendor.contractNo },
    { label: "Email", value: vendor.email },
    { label: "Telepon", value: vendor.phone },
    { label: "Vendor sejak", value: vendor.since },
    { label: "Status", value: vendor.status },
  ];
  return (
    <div className="grid gap-6 lg:grid-cols-[minmax(0,1fr)_360px]">
      <section className={`${card} p-6`}>
        <h3 className="font-['Work_Sans'] text-[16px] font-bold text-[#241B19]">Profil vendor</h3>
        <dl className="mt-4 grid grid-cols-1 gap-x-8 gap-y-5 sm:grid-cols-2">
          {fields.map((f) => (
            <div key={f.label}>
              <dt className="text-[11px] font-bold uppercase tracking-[0.14em] text-[#7C6C64]">{f.label}</dt>
              <dd className="mt-1 text-[14.5px] font-medium text-[#241B19]">{f.value}</dd>
            </div>
          ))}
        </dl>
      </section>
      <section className={`${card} p-6`}>
        <h3 className="flex items-center gap-2 font-['Work_Sans'] text-[16px] font-bold text-[#241B19]">
          <MapPin size={16} className="text-[#C8102E]" /> Alamat terdaftar
        </h3>
        <p className="mt-3 text-[14px] leading-relaxed text-[#4A3B35]">{vendor.address}</p>
        <div className="mt-5 rounded-lg bg-[#F7F4F0] p-4 text-[13px] leading-relaxed text-[#7A6A63]">
          Perubahan data legal dan NPWP wajib melalui tiket Vendor Management dan persetujuan Procurement.
        </div>
      </section>
    </div>
  );
}

export function CabangTab() {
  return (
    <section className={`${card} overflow-hidden`}>
      <div className="flex items-center justify-between border-b border-[#E9E1DA] px-5 py-3.5">
        <h3 className="flex items-center gap-2 font-['Work_Sans'] text-[16px] font-bold text-[#241B19]">
          <Building2 size={16} className="text-[#C8102E]" /> Cabang vendor
        </h3>
        <span className="text-[12.5px] text-[#7C6C64]">{branches.filter((b) => b.is_active).length} aktif · {branches.length} total</span>
      </div>
      <div className="overflow-x-auto">
        <table className="w-full min-w-[720px] text-left">
          <thead>
            <tr className="border-b border-[#E9E1DA] text-[11px] font-bold uppercase tracking-[0.12em] text-[#7C6C64]">
              <th className="px-5 py-3">Kode cabang</th>
              <th className="px-4 py-3">Nama cabang</th>
              <th className="px-4 py-3">Lokasi</th>
              <th className="px-4 py-3">Region</th>
              <th className="px-4 py-3">Kategori</th>
              <th className="px-5 py-3">Status</th>
            </tr>
          </thead>
          <tbody>
            {branches.map((b) => (
              <tr key={b.id} className="border-b border-[#F1ECE6] last:border-0 hover:bg-[#FBF1F2]">
                <td className="px-5 py-3 text-[13.5px] font-semibold tabular-nums text-[#241B19]">{b.branch_code}</td>
                <td className="px-4 py-3 text-[14px] text-[#4A3B35]">{b.branch_name}</td>
                <td className="px-4 py-3 text-[13px] tabular-nums text-[#7A6A63]">{b.location_id}</td>
                <td className="px-4 py-3 text-[14px] text-[#4A3B35]">{b.region}</td>
                <td className="px-4 py-3">
                  <span className="rounded-full bg-[#F1ECE6] px-2.5 py-1 text-[12px] font-semibold text-[#7A6A63]">{b.category}</span>
                </td>
                <td className="px-5 py-3">
                  {b.is_active ? (
                    <span className="inline-flex items-center gap-1.5 text-[13px] font-semibold text-[#0E7C4A]">
                      <span className="h-1.5 w-1.5 rounded-full bg-[#0E7C4A]" /> Aktif
                    </span>
                  ) : (
                    <span className="inline-flex items-center gap-1.5 text-[13px] font-semibold text-[#7C6C64]">
                      <span className="h-1.5 w-1.5 rounded-full bg-[#B4A79E]" /> Nonaktif
                    </span>
                  )}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </section>
  );
}

export function PicTab() {
  return (
    <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
      {pics.map((p) => (
        <section key={p.id} className={`${card} p-5`}>
          <div className="flex items-start justify-between">
            <span className="grid h-11 w-11 place-items-center rounded-full bg-[#F3D9DD] font-['Work_Sans'] text-[14px] font-bold text-[#8E0C22]">
              {p.name.split(" ").map((w) => w[0]).slice(0, 2).join("")}
            </span>
            {p.primary && (
              <span className="inline-flex items-center gap-1 rounded-full bg-[#FBF3E2] px-2.5 py-1 text-[11.5px] font-bold text-[#9A6B12]">
                <Star size={11} fill="currentColor" /> PIC utama
              </span>
            )}
          </div>
          <h3 className="mt-3 font-['Work_Sans'] text-[15.5px] font-bold text-[#241B19]">{p.name}</h3>
          <p className="flex items-center gap-1.5 text-[13px] text-[#7A6A63]">
            <User size={12} /> {p.role}
          </p>
          <div className="mt-4 space-y-1.5 border-t border-[#F1ECE6] pt-3.5 text-[13px] text-[#4A3B35]">
            <p className="flex items-center gap-2 truncate">
              <Mail size={13} className="shrink-0 text-[#7C6C64]" /> {p.email}
            </p>
            <p className="flex items-center gap-2 tabular-nums">
              <Phone size={13} className="shrink-0 text-[#7C6C64]" /> {p.phone}
            </p>
          </div>
        </section>
      ))}
    </div>
  );
}
