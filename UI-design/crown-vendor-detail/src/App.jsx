// Subject: revamp halaman detail vendor CROWN (CIMB Niaga cash management) — "ABA · Abacus", fokus tab Harga Paket.
// Rencana desain: konsol ops bank bergaya lembar tarif — kertas hangat, merah CIMB sebagai aksen utama,
// Work Sans (display) + Source Sans 3 (UI). Tabel diberi struktur grup per paket/mesin, delta harga antar
// tingkat, status pill, aksi Ubah/Akhiri dengan modal dan konfirmasi. (Panel Tangga Harga & Cara baca
// dihilangkan atas permintaan: tabel harga kini memakai lebar penuh.)

import { useState } from "react";
import Topbar from "./components/Topbar";
import VendorHeader from "./components/VendorHeader";
import HargaPaketTab from "./components/HargaPaketTab";
import { CabangTab, InfoTab, PicTab } from "./components/SimpleTabs";
import { initialTiers, vendor } from "./data";

const TABS = ["Info", "Cabang", "PIC Vendor-wide", "Harga Paket"];

export default function App() {
  const [tab, setTab] = useState("Harga Paket");
  const [tiers, setTiers] = useState(initialTiers);

  return (
    <div className="min-h-screen bg-[#F7F4F0] font-['Source_Sans_3'] text-[#241B19] antialiased">
      <link
        rel="stylesheet"
        href="https://fonts.googleapis.com/css2?family=Work+Sans:wght@500;600;700&family=Source+Sans+3:wght@400;500;600;700&display=swap"
      />
      <Topbar />

      <main className="mx-auto max-w-[1280px] px-6 pb-20">
        <VendorHeader vendor={vendor} tiers={tiers} />

        <nav className="mt-8 flex gap-1 border-b border-[#E9E1DA]">
          {TABS.map((t) => {
            const active = tab === t;
            return (
              <button
                key={t}
                onClick={() => setTab(t)}
                className={`relative px-4 py-2.5 text-[14px] font-semibold transition-colors ${
                  active ? "text-[#C8102E]" : "text-[#7A6A63] hover:text-[#241B19]"
                }`}
              >
                {t}
                <span
                  className={`absolute inset-x-3 -bottom-px h-[2.5px] rounded-full bg-[#C8102E] transition-opacity ${
                    active ? "opacity-100" : "opacity-0"
                  }`}
                />
              </button>
            );
          })}
        </nav>

        <div className="mt-6">
          {tab === "Info" && <InfoTab vendor={vendor} />}
          {tab === "Cabang" && <CabangTab />}
          {tab === "PIC Vendor-wide" && <PicTab />}
          {tab === "Harga Paket" && <HargaPaketTab tiers={tiers} setTiers={setTiers} />}
        </div>
      </main>
    </div>
  );
}
