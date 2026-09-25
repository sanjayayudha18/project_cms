import { Bell, ChevronDown, Search } from "lucide-react";

export default function Topbar() {
  return (
    <header className="sticky top-0 z-40 border-b border-[#E9E1DA] bg-[#FFFEFC]/90 backdrop-blur">
      <div className="mx-auto flex h-14 max-w-[1280px] items-center gap-4 px-6">
        <div className="flex items-center gap-3">
          <span className="grid h-8 w-8 place-items-center rounded-md bg-[#C8102E] font-['Work_Sans'] text-[13px] font-bold text-white">
            CR
          </span>
          <span className="font-['Work_Sans'] text-[15px] font-bold tracking-tight text-[#241B19]">
            CROWN
          </span>
          <span className="hidden text-[13px] text-[#B4A79E] sm:inline">/</span>
          <span className="hidden text-[13px] font-medium text-[#7A6A63] sm:inline">
            Manajemen Vendor
          </span>
        </div>

        <div className="mx-auto w-full max-w-md">
          <label className="flex items-center gap-2 rounded-lg border border-[#E9E1DA] bg-[#F7F4F0] px-3 py-1.5 text-[13px] text-[#7A6A63]">
            <Search size={15} strokeWidth={2.2} />
            <input
              className="w-full bg-transparent outline-none placeholder:text-[#B4A79E]"
              placeholder="Cari menu, fitur..."
            />
          </label>
        </div>

        <div className="flex items-center gap-4">
          <button className="relative rounded-full p-1.5 text-[#7A6A63] hover:bg-[#F1ECE6]" aria-label="Notifikasi">
            <Bell size={18} />
            <span className="absolute right-1 top-1 h-2 w-2 rounded-full bg-[#C8102E] ring-2 ring-white" />
          </button>
          <div className="h-6 w-px bg-[#E9E1DA]" />
          <button className="flex items-center gap-2.5">
            <span className="grid h-8 w-8 place-items-center rounded-full bg-[#F3D9DD] text-[12px] font-bold text-[#8E0C22]">
              SA
            </span>
            <span className="hidden text-left leading-tight sm:block">
              <span className="block text-[13px] font-semibold text-[#241B19]">System Administrator</span>
              <span className="block text-[11px] font-medium tracking-wide text-[#7C6C64]">ADMIN</span>
            </span>
            <ChevronDown size={15} className="text-[#7C6C64]" />
          </button>
        </div>
      </div>
    </header>
  );
}
