# CMS design.md (Kiro Steering Doc)

# Design System
> Visual language for the End-to-End Cash Management System (CMS) at CIMB Niaga.  
> Place in `.kiro/steering/design.md`. Kiro must follow these tokens and rules for every screen it generates. Brand color is **CIMB Red**. Companion to the living style guide (HTML reference).
* * *
## Status
Design system v0.1 defined. **Light mode only** (no dark mode, no theme toggle). Color space is **OKLCH** throughout. Frontend is React + TypeScript on Vite; these tokens become CSS custom properties.

* * *
## 1\. Color strategy
**Restrained.** CIMB Red is the brand signal, so it carries identity and "commit" actions but not volume, roughly 10% of any surface. CMS is an all-day internal ops tool: calm neutral surfaces, red only where it means "act here." A screen with three red buttons has none.

60 / 30 / 10 by visual weight: neutral surfaces (60), text/borders/inactive (30), red accents + CTAs + focus (10).

* * *
## 2\. Brand — CIMB Red (hue 29)
Warm crimson. Full 10-shade scale so hover/pressed/tint variants exist instead of one flat red.

| Token | OKLCH | Use |
| ---| ---| --- |
| `--red-50` | `oklch(0.965 0.018 29)` | Tint backgrounds, row hover |
| `--red-100` | `oklch(0.925 0.045 29)` | Focus halo, subtle fills |
| `--red-200` | `oklch(0.855 0.090 29)` | Borders on tinted areas |
| `--red-300` | `oklch(0.745 0.140 29)` |  |
| `--red-400` | `oklch(0.640 0.185 29)` |  |
| `--red-500` | `oklch(0.552 0.205 29)` | Primary. CIMB Red. CTAs, masthead, identity |
| `--red-600` | `oklch(0.485 0.193 29)` | Primary hover, eyebrow text |
| `--red-700` | `oklch(0.410 0.162 29)` |  |
| `--red-800` | `oklch(0.325 0.120 29)` |  |
| `--red-900` | `oklch(0.250 0.082 29)` | Text on red tints |

* * *
## 3\. Neutrals — tinted toward red (hue 29)
No dead gray. Every neutral carries a whisper of brand chroma (0.003–0.009) so surfaces feel cohesive without reading as "tinted." Never use `#000` or `#fff`.

| Token | OKLCH | Use |
| ---| ---| --- |
| `--n-0` | `oklch(0.992 0.003 29)` | Cards, inputs, elevated surfaces |
| `--n-50` | `oklch(0.975 0.004 29)` | Page background |
| `--n-100` | `oklch(0.952 0.005 29)` | Table row dividers, subtle fills |
| `--n-200` | `oklch(0.908 0.006 29)` | Borders, table header base |
| `--n-300` | `oklch(0.845 0.007 29)` | Input borders |
| `--n-400` | `oklch(0.700 0.008 29)` | Disabled text, placeholders |
| `--n-500` | `oklch(0.560 0.009 29)` | Metadata, captions |
| `--n-600` | `oklch(0.448 0.008 29)` | Secondary text |
| `--n-700` | `oklch(0.352 0.007 29)` | Body-strong, labels |
| `--n-800` | `oklch(0.258 0.006 29)` | Body text |
| `--n-900` | `oklch(0.178 0.005 29)` | Headings |

* * *
## 4\. Semantic colors — kept far from brand in hue space
Four roles, each with a background tint (badges/rows) and a foreground (text/icons). Success/warning/info sit far from red so status never reads as branding.

| Role | BG tint | Foreground | Solid | Use |
| ---| ---| ---| ---| --- |
| Success (155) | `oklch(0.955 0.03 155)` | `oklch(0.480 0.115 155)` | `oklch(0.560 0.130 155)` | Reconciled, approved, on-schedule |
| Warning (78) | `oklch(0.960 0.055 78)` | `oklch(0.520 0.115 78)` | `oklch(0.760 0.150 78)` | Pending, early/late fill, review |
| Danger (12) | `oklch(0.955 0.035 12)` | `oklch(0.500 0.195 12)` | `oklch(0.545 0.205 12)` | Variance, failed, not done, destructive |
| Info (245) | `oklch(0.955 0.03 245)` | `oklch(0.480 0.110 245)` | `oklch(0.580 0.120 245)` | Auto-generated, system note, virtual |

* * *
## 5\. THE hard rule: brand red vs. error red
In a red-branded banking app these two **will** collide. If a delete button and a primary CTA are the same red, users cannot tell danger from action.
*   **Brand red = hue 29** (warm). Reserved for primary actions, identity, focus. Use `--red-500`.
*   **Error red = hue 12** (rose). Reserved for destructive/error only. Use `--danger-500`.
*   **Error is NEVER signalled by color alone.** Always pair with an icon + text. (Accessibility + the two reds are close enough that colorblind/tired users need the redundancy.)

* * *
## 6\. Typography
**System font stack.** A data-dense banking tool needs no personality font; the system stack loads instantly and reads crisply small. One family, hierarchy through scale + weight.

`-apple-system, BlinkMacSystemFont, "Segoe UI", system-ui, sans-serif`

| Role | Size / weight | Use |
| ---| ---| --- |
| Display | 44 / 700 | Big figures, page hero |
| XL | 28 / 700 | Page titles |
| LG | 21 / 600 | Section / card headings |
| Base | 16 / 400 | Body (min 16px) |
| SM | 14 / 400 | Secondary text, metadata |
| XS / label | 12 / 500, `letter-spacing: 0.1em`, uppercase | Eyebrows, table headers |

Rules:
*   **`tabular-nums`** **on every amount, metric, and table.** Digits must align in columns.
*   Money uses monospace for cell values; align right.
*   `text-wrap: balance` on headings, `text-wrap: pretty` on prose.
*   Body measure 65–75ch max.

* * *
## 7\. Spacing, radius, elevation
*   **4pt base scale:** 4, 8, 12, 16, 24, 32, 48, 72px. Named tokens (`--space-sm`…), use `gap` not margins.
*   **Radius:** sm 4px (inputs/badges), md 6px (buttons), lg 10px (cards/tables).
*   **Shadows subtle**, tinted with brand hue: `0 1px 2px oklch(0.25 0.02 29 / 0.06)` (sm), `0 4px 12px … / 0.08` (md). If you can clearly see the shadow, it's too strong.
*   Vary spacing for rhythm. Same padding everywhere is monotony.

* * *
## 8\. Components
**Buttons** — one primary per view.
*   Primary: `--red-500` bg, hover `--red-600`, white text.
*   Secondary: `--n-0` bg, `--n-300` border.
*   Ghost: transparent, `--red-600` text, `--red-50` hover.
*   Danger: `--danger-500` (rose), for destructive only.
*   Disabled: `--n-200` bg, `--n-400` text.

**Tables** — the primary interface (orders, DSR, invoices, recaps).
*   Uppercase `--n-500` headers on `--n-50`, `--n-100` row dividers.
*   Amounts right-aligned, tabular, monospace.
*   Row hover: quiet `--red-50`. Status carried by badges, never row background color.

**Badges** — pill, icon + label. Map domain vocabularies once:
*   Validation: Sesuai jadwal (success) · Maju / Mundur (warning) · Tidak dilakukan (danger).
*   Reconciliation: Open (danger) · Close (success).
*   Balance tier: High (danger) · Medium (warning) · Low (neutral).
*   Order origin: Auto-generate (info) · Manual (neutral).
*   None use brand red.

**Forms** — focus ring is a soft red halo (`box-shadow: 0 0 0 3px var(--red-100)`, border `--red-400`). The one everyday place brand red touches routine input. Errors: `--danger-500` border + icon + hint text. Never `outline: none` without a `:focus-visible` replacement.

* * *
## 9\. Motion
*   Durations: 100–150ms feedback, 200–300ms state, 300–500ms layout. Exits ~75% of enter.
*   Easing: ease-out `cubic-bezier(0.22, 1, 0.36, 1)`. No bounce, no elastic.
*   Animate **transform and opacity only.** Never animate layout properties.

* * *
## 10\. Bans (match-and-refuse)
*   No side-stripe accent borders (colored `border-left`/`border-right` > 1px).
*   No gradient text (`background-clip: text`).
*   No glassmorphism as default.
*   No hero-metric template (big number + gradient accent).
*   No identical repeated card grids.
*   No em dashes in UI copy. No pure black/white.
*   No signalling error/status by color alone, always icon + text.

* * *
## 11\. Two Themes — One Per Frontend

**Brand anchor**: CIMB Niaga Red — `#E4142A` → `oklch(56% 0.223 27)`. Use OKLCH for all colors; build shade scales by holding chroma+hue constant and varying lightness. Never `#000`/`#fff`; tint neutrals slightly toward the brand hue.

### Internal App — "Merah Sirih" (`frontend/CompanyPortal-Vite`)

Warm off-white neutrals, red as a ≤10% accent (primary buttons, active states, key figures). Optimized for data-dense screens operators stare at all day. This is the theme described in Sections 1-10 above.

| Token | OKLCH | Role |
| ---| ---| --- |
| Primary | `oklch(56% 0.223 27)` | CTAs, active states |
| Primary Deep | `oklch(47% 0.185 27)` | Hover, pressed |
| Red Tint | `oklch(94% 0.03 25)` | Row hover, tinted backgrounds |
| Surface | `oklch(98.6% 0.006 40)` | Page background |
| Text | `oklch(26% 0.02 30)` | Body text, headings |

### Vendor Portal — "Merah Menyala" (`frontend/VendorPortal-Vite`)

Bold, brand-forward: maroon-red top bar, full-red active sidebar. Strong CIMB identity from first load, especially on login. External-facing presence.

| Token | OKLCH | Role |
| ---| ---| --- |
| Primary | `oklch(54% 0.233 27)` | CTAs, active states |
| Maroon Bar | `oklch(40% 0.155 26)` | Top navigation bar |
| Maroon Deep | `oklch(30% 0.11 25)` | Sidebar active, pressed states |
| Surface | `oklch(99.5% 0.003 40)` | Page background |
| Text | `oklch(25% 0.02 28)` | Body text, headings |

### Theme Rules

*   **Accessibility**: never encode status with color alone — always pair red/green with a label or icon (color-blind users). Deep red on white is safe for bold text/buttons only; do NOT use it for small thin text (insufficient contrast).
*   Red is an accent, not wallpaper (internal). Don't scatter it everywhere — it works because it's rare.
*   Money as `tabular-nums`, currency shown explicitly (IDR), amounts right-aligned in tables.
*   Font: one family in multiple weights (hierarchy via scale + weight, not two competing fonts).
*   Both themes share the same semantic color tokens (success/warning/danger/info from Section 4) and the same component patterns (Section 8). Only brand intensity and navigation chrome differ.

* * *
## Conventions
*   Update this file when a token, component pattern, or rule changes.
*   Tokens live as CSS custom properties; the HTML style guide is the visual source of truth. Keep them in sync.
*   When generating a new screen, reuse existing tokens and component patterns; do not invent new colors or one-off spacing.
*   When building for the vendor portal, apply "Merah Menyala" palette. For internal app, apply "Merah Sirih" (default, described in Sections 1-10).