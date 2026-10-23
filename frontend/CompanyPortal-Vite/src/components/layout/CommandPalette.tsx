import { CornerDownLeft, Search } from "lucide-react";
import { type KeyboardEvent, useEffect, useMemo, useRef, useState } from "react";

import { GROUP_LABELS, type NavItem, filterPaletteItems } from "@/lib/config/navigation";

interface CommandPaletteProps {
  items: NavItem[];
  /** Item menu yang sedang aktif; kotak ikonnya ditandai merah (Req 12.5). */
  activeId?: string;
  onSelect: (item: NavItem) => void;
  onClose: () => void;
}

const LIST_ID = "palette-list";
const optionId = (id: string) => `palette-opt-${id}`;

/** Indeks berikutnya secara melingkar; `n <= 0` → 0 (Req 12.6). */
export function nextIndex(i: number, delta: number, n: number): number {
  if (n <= 0) return 0;
  return (((i + delta) % n) + n) % n;
}

/** Command palette (Ctrl/⌘K): cari menu yang terlihat role user (Req 12). Di-unmount saat tutup. */
export default function CommandPalette({
  items,
  activeId,
  onSelect,
  onClose,
}: CommandPaletteProps) {
  const [query, setQuery] = useState("");
  const [index, setIndex] = useState(0);
  const activeRowRef = useRef<HTMLDivElement>(null);

  const results = useMemo(() => filterPaletteItems(items, query), [items, query]);
  const selected = results[index];

  // biome-ignore lint/correctness/useExhaustiveDependencies: scroll setiap indeks/hasil berubah
  useEffect(() => {
    activeRowRef.current?.scrollIntoView({ block: "nearest" });
  }, [index, results]);

  const handleKeyDown = (e: KeyboardEvent<HTMLInputElement>) => {
    switch (e.key) {
      case "ArrowDown":
      case "ArrowUp":
        e.preventDefault();
        setIndex((i) => nextIndex(i, e.key === "ArrowDown" ? 1 : -1, results.length));
        break;
      case "Enter":
        e.preventDefault();
        if (selected) onSelect(selected);
        break;
      case "Escape":
        e.preventDefault();
        onClose();
        break;
      case "Tab":
        // Focus trap: input adalah satu-satunya elemen yang bisa difokus.
        e.preventDefault();
        break;
    }
  };

  return (
    <div
      className="fixed inset-0 z-50 flex items-start justify-center bg-[var(--shell-overlay)] px-4 pt-[12vh] backdrop-blur-sm"
      onPointerDown={(e) => {
        if (e.target === e.currentTarget) onClose();
      }}
    >
      <div
        // biome-ignore lint/a11y/useSemanticElements: overlay + focus trap sendiri, bukan <dialog> native
        role="dialog"
        aria-modal="true"
        aria-label="Cari menu"
        className="shell-pop w-[min(560px,calc(100vw-32px))] self-start overflow-hidden rounded-2xl border border-[var(--shell-topbar-line)] bg-[var(--shell-topbar-bg)] shadow-[var(--shell-shadow-pop)] [transform-origin:top_center]"
      >
        <div className="flex items-center gap-3 border-b border-[var(--shell-topbar-line)] px-4">
          <Search aria-hidden="true" className="size-5 shrink-0 text-[var(--shell-muted)]" />
          <input
            // biome-ignore lint/a11y/noAutofocus: palette dialog harus langsung memfokus input (Req 12.1)
            autoFocus
            type="text"
            role="combobox"
            aria-expanded="true"
            aria-controls={LIST_ID}
            aria-autocomplete="list"
            aria-activedescendant={selected ? optionId(selected.id) : undefined}
            aria-label="Cari menu"
            placeholder="Ke menu mana? Ketik nama fitur…"
            value={query}
            onChange={(e) => {
              setQuery(e.target.value);
              setIndex(0);
            }}
            onKeyDown={handleKeyDown}
            className="h-14 min-w-0 flex-1 bg-transparent text-base text-[var(--shell-ink)] outline-none focus-visible:shadow-[inset_0_-2px_0_var(--shell-focus)] placeholder:text-[var(--shell-muted)]"
          />
          <kbd className="shrink-0 rounded-md border border-[var(--shell-input-line)] px-1.5 py-0.5 font-sans text-xs font-semibold text-[var(--shell-muted)]">
            Esc
          </kbd>
        </div>

        {results.length === 0 ? (
          // biome-ignore lint/a11y/useSemanticElements: <output> tidak cocok untuk pesan kosong
          <p role="status" className="px-4 py-8 text-center text-sm text-[var(--shell-muted)]">
            Tidak ada menu "{query.trim()}"
          </p>
        ) : (
          // biome-ignore lint/a11y/useFocusableInteractive: fokus tetap di input combobox (aria-activedescendant)
          <div
            // biome-ignore lint/a11y/useSemanticElements: listbox kustom per pola combobox WAI-ARIA
            role="listbox"
            id={LIST_ID}
            aria-label="Hasil menu"
            className="shell-scroll max-h-[min(360px,50vh)] overflow-y-auto p-2"
          >
            {results.map((item, i) => {
              const isSelected = i === index;
              const Icon = item.icon;
              const isRed = isSelected || item.id === activeId;
              return (
                // biome-ignore lint/a11y/useKeyWithClickEvents: keyboard ditangani combobox input (aria-activedescendant)
                // biome-ignore lint/a11y/useFocusableInteractive: fokus tetap di input combobox (aria-activedescendant)
                <div
                  key={item.id}
                  ref={isSelected ? activeRowRef : undefined}
                  id={optionId(item.id)}
                  // biome-ignore lint/a11y/useSemanticElements: option kustom per pola combobox WAI-ARIA
                  role="option"
                  aria-selected={isSelected}
                  onPointerMove={() => setIndex(i)}
                  onClick={() => onSelect(item)}
                  className={`flex h-12 cursor-pointer items-center gap-3 rounded-xl px-2 ${
                    isSelected ? "bg-[var(--shell-accent-tint)]" : ""
                  }`}
                >
                  <span
                    className={`flex size-8 shrink-0 items-center justify-center rounded-lg ${
                      isRed
                        ? "bg-[var(--shell-accent)] text-[var(--shell-on-accent)]"
                        : "bg-[var(--shell-chip-bg)] text-[var(--shell-maroon)]"
                    }`}
                  >
                    <Icon aria-hidden="true" className="size-4" />
                  </span>
                  <span className="min-w-0 flex-1 truncate text-sm font-semibold text-[var(--shell-ink)]">
                    {item.label}
                  </span>
                  <span className="shrink-0 text-xs text-[var(--shell-muted)]">
                    {GROUP_LABELS[item.group]}
                  </span>
                  {isSelected ? (
                    <CornerDownLeft
                      aria-hidden="true"
                      className="size-4 shrink-0 text-[var(--shell-muted)]"
                    />
                  ) : null}
                </div>
              );
            })}
          </div>
        )}
      </div>
    </div>
  );
}
