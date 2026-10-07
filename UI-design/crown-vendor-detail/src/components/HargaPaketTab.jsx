import { useMemo, useState } from "react";
import { AnimatePresence, motion } from "framer-motion";
import { CircleCheck, Pencil, Plus, X } from "lucide-react";
import { TODAY, fmtDate, fmtIDR, kodePaket, nextSeq, tierLabel, tierStatus } from "../data";

function StatusPill({ status }) {
  const map = {
    Berlaku: "border-[#BBE3CD] bg-[#E5F4EC] text-[#0E7C4A]",
    Berakhir: "border-[#E4DAD2] bg-[#F1ECE6] text-[#6B5B53]",
    Dijadwalkan: "border-[#F2DFB8] bg-[#FBF3E2] text-[#9A6B12]",
  };
  const dot = { Berlaku: "bg-[#0E7C4A]", Berakhir: "bg-[#B4A79E]", Dijadwalkan: "bg-[#C98A1B]" };
  return (
    <span className={`inline-flex items-center gap-1.5 rounded-full border px-2.5 py-1 text-[12px] font-semibold ${map[status]}`}>
      <span className={`h-1.5 w-1.5 rounded-full ${dot[status]}`} />
      {status}
    </span>
  );
}

const inputCls =
  "w-full rounded-lg border border-[#E4DAD2] bg-[#FFFEFC] px-3 py-2 text-[14px] text-[#241B19] outline-none focus:border-[#C8102E] focus:ring-2 focus:ring-[#C8102E]/15";

function TierModal({ mode, initial, paketOptions, mesinOptions, kelasOptions, onClose, onSave }) {
  const [f, setF] = useState(
    initial || {
      paket: paketOptions[0],
      mesin: mesinOptions[0],
      kelas: kelasOptions[0],
      tipe: "Standar",
      min: 1,
      max: 50,
      harga: 2000000,
      mulai: "2026-09-24",
      berakhir: "2027-12-31",
    }
  );
  const set = (k) => (e) => setF({ ...f, [k]: e.target.value });

  return (
    <motion.div
      className="fixed inset-0 z-50 grid place-items-center bg-[#241B19]/45 p-4"
      initial={{ opacity: 0 }}
      animate={{ opacity: 1 }}
      exit={{ opacity: 0 }}
      onClick={onClose}
    >
      <motion.div
        className="w-full max-w-lg rounded-2xl bg-[#FFFEFC] p-6 shadow-2xl"
        initial={{ y: 16, scale: 0.97, opacity: 0 }}
        animate={{ y: 0, scale: 1, opacity: 1 }}
        exit={{ y: 10, scale: 0.98, opacity: 0 }}
        transition={{ type: "spring", stiffness: 380, damping: 32 }}
        onClick={(e) => e.stopPropagation()}
      >
        <div className="flex items-start justify-between">
          <div>
            <h3 className="font-['Work_Sans'] text-[18px] font-bold text-[#241B19]">
              {mode === "edit" ? "Ubah Harga Paket" : "Tambah Harga Paket"}
            </h3>
            <p className="mt-0.5 text-[13px] text-[#7A6A63]">Harga dasar per unit untuk satu tingkat.</p>
          </div>
          <button onClick={onClose} className="rounded-full p-1.5 text-[#7C6C64] hover:bg-[#F1ECE6]" aria-label="Tutup">
            <X size={17} />
          </button>
        </div>

        <div className="mt-5 grid grid-cols-2 gap-4">
          <label className="block">
            <span className="mb-1 block text-[12px] font-bold uppercase tracking-wide text-[#7C6C64]">Paket</span>
            <select className={inputCls} value={f.paket} onChange={set("paket")}>
              {paketOptions.map((p) => (
                <option key={p}>{p}</option>
              ))}
            </select>
          </label>
          <label className="block">
            <span className="mb-1 block text-[12px] font-bold uppercase tracking-wide text-[#7C6C64]">Mesin</span>
            <select className={inputCls} value={f.mesin} onChange={set("mesin")}>
              {mesinOptions.map((m) => (
                <option key={m}>{m}</option>
              ))}
            </select>
          </label>
          <label className="block">
            <span className="mb-1 block text-[12px] font-bold uppercase tracking-wide text-[#7C6C64]">Kelas</span>
            <select className={inputCls} value={f.kelas} onChange={set("kelas")}>
              {kelasOptions.map((k) => (
                <option key={k}>{k}</option>
              ))}
            </select>
          </label>
          <label className="block">
            <span className="mb-1 block text-[12px] font-bold uppercase tracking-wide text-[#7C6C64]">Tingkat dari</span>
            <input type="number" min="1" className={inputCls} value={f.min} onChange={set("min")} />
          </label>
          <label className="block">
            <span className="mb-1 block text-[12px] font-bold uppercase tracking-wide text-[#7C6C64]">Sampai (kosong = terbuka)</span>
            <input type="number" min="1" className={inputCls} value={f.max ?? ""} onChange={(e) => setF({ ...f, max: e.target.value === "" ? null : e.target.value })} />
          </label>
          <label className="block">
            <span className="mb-1 block text-[12px] font-bold uppercase tracking-wide text-[#7C6C64]">Tingkat harga</span>
            <input className={inputCls} value={f.tipe} onChange={set("tipe")} />
          </label>
          <label className="block">
            <span className="mb-1 block text-[12px] font-bold uppercase tracking-wide text-[#7C6C64]">Harga dasar (IDR)</span>
            <input type="number" min="0" step="100" className={inputCls} value={f.harga} onChange={set("harga")} />
          </label>
          <label className="block">
            <span className="mb-1 block text-[12px] font-bold uppercase tracking-wide text-[#7C6C64]">Mulai berlaku</span>
            <input type="date" className={inputCls} value={f.mulai} onChange={set("mulai")} />
          </label>
          <label className="block">
            <span className="mb-1 block text-[12px] font-bold uppercase tracking-wide text-[#7C6C64]">Berakhir</span>
            <input type="date" className={inputCls} value={f.berakhir} onChange={set("berakhir")} />
          </label>
        </div>

        <div className="mt-6 flex justify-end gap-2.5">
          <button onClick={onClose} className="rounded-lg border border-[#E4DAD2] px-4 py-2 text-[14px] font-semibold text-[#7A6A63] hover:bg-[#F7F4F0]">
            Batal
          </button>
          <button
            onClick={() => onSave({ ...f, min: Number(f.min), max: f.max == null ? null : Number(f.max), harga: Number(f.harga) })}
            className="rounded-lg bg-[#C8102E] px-4 py-2 text-[14px] font-semibold text-white hover:bg-[#A80D27]"
          >
            {mode === "edit" ? "Simpan perubahan" : "Tambah tingkat"}
          </button>
        </div>
      </motion.div>
    </motion.div>
  );
}

function ConfirmEnd({ tier, onClose, onConfirm }) {
  return (
    <motion.div className="fixed inset-0 z-50 grid place-items-center bg-[#241B19]/45 p-4" initial={{ opacity: 0 }} animate={{ opacity: 1 }} exit={{ opacity: 0 }} onClick={onClose}>
      <motion.div
        className="w-full max-w-sm rounded-2xl bg-[#FFFEFC] p-6 shadow-2xl"
        initial={{ y: 14, scale: 0.97, opacity: 0 }}
        animate={{ y: 0, scale: 1, opacity: 1 }}
        exit={{ y: 8, scale: 0.98, opacity: 0 }}
        transition={{ type: "spring", stiffness: 380, damping: 32 }}
        onClick={(e) => e.stopPropagation()}
      >
        <h3 className="font-['Work_Sans'] text-[17px] font-bold text-[#241B19]">Akhiri harga paket ini?</h3>
        <p className="mt-2 text-[13.5px] leading-relaxed text-[#7A6A63]">
          {tier.paket} · {tier.mesin}, tingkat {tierLabel(tier)} unit seharga <span className="font-semibold text-[#241B19]">{fmtIDR(tier.harga)}</span> akan
          diakhiri per 24 Sep 2026. Riwayatnya tetap tersimpan.
        </p>
        <div className="mt-5 flex justify-end gap-2.5">
          <button onClick={onClose} className="rounded-lg border border-[#E4DAD2] px-4 py-2 text-[14px] font-semibold text-[#7A6A63] hover:bg-[#F7F4F0]">
            Batal
          </button>
          <button onClick={onConfirm} className="rounded-lg bg-[#C8102E] px-4 py-2 text-[14px] font-semibold text-white hover:bg-[#A80D27]">
            Ya, akhiri
          </button>
        </div>
      </motion.div>
    </motion.div>
  );
}

export default function HargaPaketTab({ tiers, setTiers }) {
  const [paket, setPaket] = useState("Semua paket");
  const [mesin, setMesin] = useState("Semua mesin");
  const [status, setStatus] = useState("Semua status");
  const [hoverId, setHoverId] = useState(null);
  const [modal, setModal] = useState(null); // {mode, tier?}
  const [ending, setEnding] = useState(null);
  const [flash, setFlash] = useState(null);

  const paketOptions = useMemo(() => [...new Set(tiers.map((t) => t.paket))], [tiers]);
  const mesinOptions = useMemo(() => [...new Set(tiers.map((t) => t.mesin))], [tiers]);
  const kelasOptions = useMemo(() => [...new Set(tiers.map((t) => t.kelas))], [tiers]);

  const filtered = tiers.filter(
    (t) =>
      (paket === "Semua paket" || t.paket === paket) &&
      (mesin === "Semua mesin" || t.mesin === mesin) &&
      (status === "Semua status" || tierStatus(t) === status)
  );

  const sorted = useMemo(
    () =>
      [...filtered].sort(
        (a, b) =>
          a.paket.localeCompare(b.paket) ||
          a.mesin.localeCompare(b.mesin) ||
          (a.kelas || "").localeCompare(b.kelas || "") ||
          a.min - b.min
      ),
    [filtered]
  );

  const selectCls =
    "rounded-lg border border-[#E4DAD2] bg-[#FFFEFC] px-3 py-2 text-[13px] font-medium text-[#241B19] outline-none focus:border-[#C8102E]";

  const saveTier = (data) => {
    if (modal.mode === "edit") {
      setTiers(tiers.map((t) => (t.id === modal.tier.id ? { ...t, ...data } : t)));
      setFlash("Perubahan harga paket disimpan.");
    } else {
      const id = Math.max(...tiers.map((t) => t.id)) + 1;
      const row = { id, seq: nextSeq(tiers), status: "active", ...data };
      setTiers([...tiers, row]);
      setFlash(`${data.paket} tingkat ${data.min}${data.max ? `\u2013${data.max}` : "+"} ditambahkan.`);
    }
    setModal(null);
    setTimeout(() => setFlash(null), 3500);
  };

  const confirmEnd = () => {
    setTiers(tiers.map((t) => (t.id === ending.id ? { ...t, status: "ended", berakhir: "2026-09-24" } : t)));
    setEnding(null);
    setFlash("Harga paket diakhiri per 24 Sep 2026.");
    setTimeout(() => setFlash(null), 3500);
  };

  return (
    <div>
      <section className="overflow-hidden rounded-xl border border-[#E9E1DA] bg-[#FFFEFC]">
        <div className="flex flex-wrap items-center gap-2.5 border-b border-[#E9E1DA] px-5 py-3.5">
          <select className={selectCls} value={paket} onChange={(e) => setPaket(e.target.value)}>
            <option>Semua paket</option>
            {paketOptions.map((p) => (
              <option key={p}>{p}</option>
            ))}
          </select>
          <select className={selectCls} value={mesin} onChange={(e) => setMesin(e.target.value)}>
            <option>Semua mesin</option>
            {mesinOptions.map((m) => (
              <option key={m}>{m}</option>
            ))}
          </select>
          <select className={selectCls} value={status} onChange={(e) => setStatus(e.target.value)}>
            <option>Semua status</option>
            <option>Berlaku</option>
            <option>Dijadwalkan</option>
            <option>Berakhir</option>
          </select>
          <span className="ml-1 text-[12.5px] text-[#7C6C64]">{filtered.length} tingkat</span>
          <button
            onClick={() => setModal({ mode: "add" })}
            className="ml-auto inline-flex items-center gap-1.5 rounded-lg bg-[#C8102E] px-4 py-2 text-[13.5px] font-semibold text-white shadow-sm hover:bg-[#A80D27]"
          >
            <Plus size={15} strokeWidth={2.6} /> Tambah Harga Paket
          </button>
        </div>

        <div className="overflow-x-auto">
          <table className="w-full min-w-[1080px] text-left">
            <thead>
              <tr className="border-b border-[#E9E1DA] text-[12px] font-bold uppercase tracking-[0.12em] text-[#7C6C64]">
                <th className="px-5 py-3">Kode Paket</th>
                <th className="px-4 py-3">Paket</th>
                <th className="px-4 py-3">Mesin</th>
                <th className="px-4 py-3">Tipe</th>
                <th className="px-4 py-3">Kelas</th>
                <th className="px-4 py-3">Tingkat (unit)</th>
                <th className="px-4 py-3 text-right">Harga dasar</th>
                <th className="px-4 py-3">Periode</th>
                <th className="px-4 py-3">Status</th>
                <th className="px-5 py-3 text-right">Aksi</th>
              </tr>
            </thead>
            <tbody>
              {sorted.map((t) => (
                <TierRow
                  key={t.id}
                  t={t}
                  hoverId={hoverId}
                  setHoverId={setHoverId}
                  onEdit={(tier) => setModal({ mode: "edit", tier })}
                  onEnd={(tier) => setEnding(tier)}
                />
              ))}
              {sorted.length === 0 && (
                <tr>
                  <td colSpan={10} className="px-5 py-12 text-center text-[14px] text-[#7C6C64]">
                    Tidak ada tingkat yang cocok dengan filter.
                  </td>
                </tr>
              )}
            </tbody>
          </table>
        </div>
      </section>

      <AnimatePresence>
        {flash && (
          <motion.div
            initial={{ y: 24, opacity: 0 }}
            animate={{ y: 0, opacity: 1 }}
            exit={{ y: 24, opacity: 0 }}
            className="fixed inset-x-0 bottom-6 z-50 mx-auto flex w-fit items-center gap-2 rounded-full bg-[#241B19] px-4 py-2.5 text-[13.5px] font-medium text-white shadow-xl"
          >
            <CircleCheck size={16} className="text-[#6FD3A0]" /> {flash}
          </motion.div>
        )}
        {modal && (
          <TierModal
            mode={modal.mode}
            initial={modal.tier}
            paketOptions={paketOptions}
            mesinOptions={mesinOptions}
            kelasOptions={kelasOptions}
            onClose={() => setModal(null)}
            onSave={saveTier}
          />
        )}
        {ending && <ConfirmEnd tier={ending} onClose={() => setEnding(null)} onConfirm={confirmEnd} />}
      </AnimatePresence>
    </div>
  );
}

function TierRow({ t, hoverId, setHoverId, onEdit, onEnd }) {
  const st = tierStatus(t);
  const ended = st === "Berakhir";
  return (
          <tr
            onMouseEnter={() => setHoverId(t.id)}
            onMouseLeave={() => setHoverId(null)}
            className={`border-b border-[#F1ECE6] transition-colors ${hoverId === t.id ? "bg-[#FBF1F2]" : "bg-[#FFFEFC]"} ${ended ? "opacity-70" : ""}`}
          >
            <td className="whitespace-nowrap px-5 py-3.5">
              <span className="font-mono text-[13px] font-semibold tabular-nums text-[#8E0C22]">{kodePaket(t)}</span>
            </td>
            <td className="px-4 py-3.5">
              <span className="block whitespace-nowrap text-[14px] font-semibold text-[#241B19]">{t.paket}</span>
            </td>
            <td className="whitespace-nowrap px-4 py-3.5 text-[14px] text-[#4A3B35]">{t.mesin}</td>
            <td className="whitespace-nowrap px-4 py-3.5 text-[14px] text-[#4A3B35]">{t.tipe}</td>
            <td className="whitespace-nowrap px-4 py-3.5 text-[14px] text-[#4A3B35]">{t.kelas}</td>
            <td className="px-4 py-3.5">
              <span className="block text-[14px] font-semibold tabular-nums text-[#241B19]">{tierLabel(t)}</span>
            </td>
            <td className="px-4 py-3.5 text-right">
              <span className="block whitespace-nowrap text-[14px] font-semibold tabular-nums text-[#241B19]">{fmtIDR(t.harga)}</span>
            </td>
            <td className="whitespace-nowrap px-4 py-3.5 text-[13px] tabular-nums text-[#4A3B35]">
              {fmtDate(t.mulai)} <span className="text-[#C9BBB2]">→</span> {fmtDate(t.berakhir)}
            </td>
            <td className="px-4 py-3.5">
              <StatusPill status={st} />
            </td>
            <td className="px-5 py-3.5">
              <div className="flex justify-end gap-2">
                {ended ? (
                  <span className="text-[12.5px] italic text-[#8A7A70]">Diakhiri {fmtDate(t.berakhir)}</span>
                ) : (
                  <>
                    <button
                      onClick={() => onEdit(t)}
                      className="inline-flex items-center gap-1 rounded-lg border border-[#E4DAD2] px-3 py-1.5 text-[12.5px] font-semibold text-[#4A3B35] hover:border-[#C8102E] hover:text-[#C8102E]"
                    >
                      <Pencil size={12} /> Ubah
                    </button>
                    <button
                      onClick={() => onEnd(t)}
                      className="rounded-lg bg-[#C8102E] px-3 py-1.5 text-[12.5px] font-semibold text-white hover:bg-[#A80D27]"
                    >
                      Akhiri
                    </button>
                  </>
                )}
              </div>
            </td>
          </tr>
  );
}
