---
paths:
  - "frontend/**"
---

# UI/UX & Brand (frontends only)

> Moved verbatim from `.claude/CLAUDE.md` Sec 13 on 2026-09-29. "Sec N" references point to CLAUDE.md. Loaded automatically when working under `frontend/`.

**Brand anchor**: CIMB Niaga Red — `#E4142A` → `oklch(56% 0.223 27)`. Use OKLCH for all colors; build shade scales by holding chroma+hue constant and varying lightness. Never `#000`/`#fff`; tint neutrals slightly toward the brand hue.

**Two themes, one per frontend:**
*   **Internal app** (`frontend/CompanyPortal-Vite`) → **Option A "Merah Sirih"**. Warm off-white neutrals, red as a ≤10% accent (primary buttons, active states, key figures). Optimized for data-dense screens operators stare at all day.
    *   Primary `oklch(56% 0.223 27)` · Primary Deep `oklch(47% 0.185 27)` · Red Tint `oklch(94% 0.03 25)` · Surface `oklch(98.6% 0.006 40)` · Text `oklch(26% 0.02 30)`
*   **Vendor portal** (`frontend/VendorPortal-Vite`) → **Option B "Merah Menyala"**. Bold, brand-forward: maroon-red top bar, full-red active sidebar. Strong CIMB identity from first load, especially on login.
    *   Primary `oklch(54% 0.233 27)` · Maroon Bar `oklch(40% 0.155 26)` · Maroon Deep `oklch(30% 0.11 25)` · Surface `oklch(99.5% 0.003 40)` · Text `oklch(25% 0.02 28)`

**Rules:**
*   Font: one family in multiple weights (hierarchy via scale + weight, not two competing fonts). Use `tabular-nums` for all money/metrics tables.
*   **Accessibility**: never encode status with color alone — always pair red/green with a label or icon (color-blind users). Deep red on white is safe for bold text/buttons only; do NOT use it for small thin text (insufficient contrast).
*   Red is an accent, not wallpaper (internal). Don't scatter it everywhere — it works because it's rare.
*   Money as `tabular-nums`, currency shown explicitly (IDR), amounts right-aligned in tables.
*   Palette reference / live mockups: see the CMS palette showcase artifact.
