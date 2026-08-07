# 05-UI-BRAND.md

# [05-UI-BRAND.md](http://05-UI-BRAND.md) — LAW
> Status: **LOCKED**. Load only when working on frontend. Backend tasks must never load this file.  
> This file is the visual contract. If a mockup and this file disagree, this file wins.  
> Live reference specimen: **crown-design-system.html**
* * *
## 0\. RULE ZERO — `color-scheme`
**Every page must declare its colour scheme. This is not optional.**

```plain
<meta name="color-scheme" content="light">
:root { color-scheme: light; }
```

**Why this is Rule Zero:** without it, Safari on macOS with the OS in dark mode renders every _user-agent-controlled_ surface in dark styling while your page stays light. Form inputs go dark-on-dark. Select dropdowns get dark chrome. Scrollbars invert. Autofill turns fields muddy. Text selection becomes unreadable. The page looks broken even though every line of your CSS is correct.

One declaration fixes all of it. **This is the highest-value line of CSS in the project.**
### Phase 1 decision: LIGHT MODE ONLY
1. Screenshots are **audit evidence**. Berita acara, recon results, and approval trails get printed and filed. Two visual variants of the same evidence is a governance problem nobody wants to explain.
2. Every screen has a **printed counterpart**. Dark mode prints badly and forces a second stylesheet.
3. It **halves the QA surface** for a team this size. Every component, every state, twice.

Dark tokens are fully designed in §4.4 so that if dark mode is ever approved it is a token swap, not a rewrite. But it is **not built in Phase 1**, and nobody adds a theme toggle.

* * *
## 1\. Typeface

| Role | Family | Weights | Source |
| ---| ---| ---| --- |
| Interface | Plus Jakarta Sans | 400, 500, 600, 700 | Google Fonts (variable 200–800) |
| Numerals & codes | IBM Plex Mono | 400, 500 | Google Fonts |

```plain
<link rel="preconnect" href="https://fonts.googleapis.com">
<link rel="preconnect" href="https://fonts.gstatic.com" crossorigin>
<link href="https://fonts.googleapis.com/css2?family=Plus+Jakarta+Sans:wght@400;500;600;700&family=IBM+Plex+Mono:wght@400;500&display=swap" rel="stylesheet">
--font-sans: "Plus Jakarta Sans", ui-sans-serif, system-ui, -apple-system, "Segoe UI", sans-serif;
--font-mono: "IBM Plex Mono", ui-monospace, SFMono-Regular, monospace;
```

### Why these
**Plus Jakarta Sans** was commissioned by the Jakarta city government and drawn by Tokotype, an Indonesian foundry. It is institutional by origin, which is exactly the register a bank operations system needs: clean, slightly humanist, quietly confident. It carries none of the startup-deck baggage that Inter and Poppins now drag along, and it is an Indonesian typeface on an Indonesian bank's system.

**IBM Plex Mono** is not decoration, it is the _comparison instrument_. This system's core job is putting two numbers side by side so a human spots a discrepancy instantly. Monospaced digits make a mismatch in the seventh position jump out. Proportional digits hide it. Pairing a humanist sans with a mono is genuine structural contrast, not two similar sans-serifs fighting each other.
### Rules
*   **Never a third family.** Not for headings, not for personality.
*   Never set Plus Jakarta Sans below 12px. It goes soft.
*   Never set IBM Plex Mono as body text. Figures, IDs, and code only (§3).
*   If webfonts fail to load, the system stack takes over and **the layout must not shift**. Test with fonts blocked.

* * *
## 2\. Type Scale
Ratio **1.25 (major third)**, base 16px, body line-height 1.5. **The vertical rhythm unit is therefore 24px** and all vertical spacing is a multiple of it.

| Token | Size | Line-height | Weight | Tracking | Use |
| ---| ---| ---| ---| ---| --- |
| `display` | `clamp(2.1rem, 5vw, 3.1rem)` | 1.04 | 800 | \-0.038em | Page title, one per screen |
| `h1` | 1.75rem | 1.15 | 700 | \-0.028em | Section heading |
| `h2` | 1.375rem | 1.25 | 600 | \-0.02em | Subsection |
| `h3` | 1.0625rem | 1.35 | 600 | \-0.01em | Panel title |
| `body` | 1rem | 1.5 | 400 | 0 | Prose, form values |
| `sm` | 0.875rem | 1.45 | 400 | 0 | Table cells, secondary UI |
| `xs` | 0.75rem | 1.4 | 500 | 0 | Metadata, timestamps |
| `label` | 0.6875rem | 1.3 | 600 | 0.1em, uppercase | Column headers, eyebrows |

**Non-negotiables:**
*   Do not invent intermediate sizes. Eight steps is enough. **15px does not exist.**
*   All-caps labels always carry 0.08–0.12em tracking. Capitals sit too close by default.
*   Body prose capped at **68ch**. Tables exempt.
*   Headings sitting flush left get `margin-left: -0.035em` for optical alignment.

```css
h1, h2, h3, h4 { text-wrap: balance; }
p { text-wrap: pretty; max-width: 68ch; }
body { font-optical-sizing: auto; }
```

* * *
## 3\. Numerals — the most important rule in this file
CROWN is a cash system. Digits are the product.

| Context | Treatment |
| ---| --- |
| Any amount in a table or comparison | `--font-mono`, weight 500, right-aligned |
| Any amount in prose or a single field | `--font-sans` with `font-variant-numeric: tabular-nums` |
| IDs, reference codes, ATM IDs, hashes | `--font-mono`, weight 400, left-aligned |

```css
.amount {
  font-family: var(--font-mono);
  font-weight: 500;
  font-variant-numeric: tabular-nums;
  text-align: right;
  white-space: nowrap;
}
```

### Formatting rules
*   **Always show the currency code.** `IDR 1.240.500.000`, never a bare number. An amount without a unit is a defect.
*   Indonesian separators: period for thousands, comma for decimals. Use `Intl.NumberFormat('id-ID')`. **Never hand-roll it.**
*   Rupiah shows no decimals unless the value has them.
*   Negatives and variances always carry a **leading sign plus** the colour: `−IDR 12.500.000`. Use the true minus sign `−` (U+2212), not a hyphen. It shares digit width in mono.
*   **A calculated zero renders as** **`0`****\*\*\*\*. Blank or an em dash means not calculated.** Those are different facts: one vendor is compliant, the other is late. Never collapse them.
*   Never truncate or abbreviate in a reconciliation view. No `1,2 M`. Full precision or nothing.

* * *
## 4\. Colour System
### 4.1 Strategy
**Restrained.** Tinted neutrals carry the surface; brand red appears on at most 10% of any screen. Operators stare at these screens for eight hours and red is how the system says _look here_. Spend it everywhere and it stops meaning anything.

The **vendor portal** runs identical tokens with a **committed** treatment on chrome only: maroon top bar, full-red active navigation. Vendors pass through in minutes and need instant brand recognition. Content areas stay identical to the internal app.
### 4.2 Anchor
CIMB Niaga Red `#E4142A` → `oklch(56% 0.223 27)`

All colour authored in **OKLCH**. Build shades by holding chroma and hue constant and varying lightness. Drop chroma as lightness approaches either extreme or it goes garish.

**Never** **`#000`** **or** **`#fff`****\*\*\*\*.** Every neutral is tinted toward hue 30–40 at chroma 0.006–0.012: small enough that nobody consciously notices, large enough that the interface feels like one material.
### 4.3 Light tokens (shipped)

| Token | OKLCH | Hex fallback | Role |
| ---| ---| ---| --- |
| `--bg` | `oklch(98.6% 0.006 40)` | `#FBFAF9` | Page background |
| `--surface` | `oklch(99.6% 0.003 40)` | `#FEFDFD` | Panels, table body |
| `--surface-raised` | `oklch(99.8% 0.002 40)` | `#FFFEFE` | Popovers, dialogs |
| `--surface-sunken` | `oklch(96.4% 0.008 38)` | `#F4F1F0` | Table headers, wells |
| `--border` | `oklch(91% 0.008 35)` | `#E7E3E1` | Hairlines, table rules |
| `--border-strong` | `oklch(83% 0.012 34)` | `#CFC9C6` | Input borders |
| `--text` | `oklch(26% 0.02 30)` | `#2B2523` | Primary text |
| `--text-secondary` | `oklch(48% 0.016 30)` | `#6E6461` | Labels, supporting copy |
| `--text-muted` | `oklch(64% 0.012 32)` | `#9C9491` | Placeholders, disabled |
| `--primary` | `oklch(56% 0.223 27)` | `#E4142A` | Fills: buttons, active states |
| `--primary-hover` | `oklch(50% 0.20 27)` | `#C40F23` |  |
| `--primary-fg` | `oklch(99% 0.004 40)` | `#FFFCFB` | Text on `--primary` (5.3:1) |
| `--primary-text` | `oklch(47% 0.185 27)` | `#B00E1F` | Red as text: links, key figures (7:1) |
| `--primary-tint` | `oklch(95% 0.028 25)` | `#FCE9EA` | Subtle red background |

### 4.4 Dark tokens (designed, NOT shipped)
Dark mode is not inverted light mode. Depth comes from **surface lightness**, not shadow. Shadows are removed entirely.

| Token | Value |
| ---| --- |
| `--bg` | `oklch(16% 0.008 30)` |
| `--surface` | `oklch(19.5% 0.009 30)` |
| `--surface-raised` | `oklch(23.5% 0.010 30)` |
| `--surface-sunken` | `oklch(13.5% 0.007 30)` |
| `--border` | `oklch(28% 0.010 30)` |
| `--border-strong` | `oklch(36% 0.012 30)` |
| `--text` | `oklch(94% 0.006 40)` |
| `--text-secondary` | `oklch(76% 0.010 35)` |
| `--text-muted` | `oklch(60% 0.010 32)` |
| `--primary` (fill) | `oklch(53% 0.205 27)` — deeper than light mode |
| `--primary-fg` | `oklch(98% 0.004 40)` |
| `--primary-text` | `oklch(72% 0.145 27)` — lighter than light mode |
| `--primary-tint` | `oklch(28% 0.055 27)` |

**The non-obvious part:** the accent needs **two different values** in dark mode. As a _fill_ it must go darker so white text keeps contrast. As _text_ it must go lighter to read on a dark surface. Using one value for both is the single most common dark-mode failure.

Dark mode also needs typographic compensation (§12.3). It is not a colour-only change.
### 4.5 Semantic colours

| Role | Light | Tint | Dark | Dark tint |
| ---| ---| ---| ---| --- |
| Success | `oklch(49% 0.13 151)` | `oklch(95% 0.030 151)` | `oklch(68% 0.14 151)` | `oklch(26% 0.05 151)` |
| Warning | `oklch(58% 0.13 70)` | `oklch(95.5% 0.035 78)` | `oklch(74% 0.14 75)` | `oklch(27% 0.05 70)` |
| Info | `oklch(52% 0.11 250)` | `oklch(94.5% 0.025 250)` | `oklch(70% 0.12 250)` | `oklch(26% 0.05 250)` |
| Danger | `oklch(45% 0.19 22)` | `oklch(94% 0.030 22)` | `oklch(66% 0.17 22)` | `oklch(26% 0.06 22)` |

### 4.6 The red-on-red problem — solve it, don't ignore it
The brand is red. Destructive is red. Error is red. Without a rule, a user cannot tell _primary action_ from _this will delete something_.

**The rule:**
*   **Brand red** (`--primary`, hue 27) is only ever a **filled** button or an **active** state. A filled red button in CROWN always means _proceed_.
*   **Danger red** (hue 22, darker and cooler) is only ever an **outlined** button with a warning icon and an explicit verb. **Never filled.**
*   Destructive confirmations spell out the consequence: "Reject invoice INV-2027-03-0142, IDR 1.240.000.000, PT Advantage SCM". Never "Are you sure?".
*   Error and variance states carry **an icon and a word**, never colour alone.

* * *
## 5\. Spacing & Layout
**4pt base**, tuned to the 24px rhythm unit.

```plain
--space-2xs: 4px   --space-xs: 8px    --space-sm: 12px
--space-md:  16px  --space-lg: 24px   --space-xl: 32px
--space-2xl: 48px  --space-3xl: 72px  --space-4xl: 96px
```

*   Use `gap`, not margins, for sibling spacing.
*   **Vary the rhythm.** Section gaps large (48–72px), intra-group gaps tight (8–12px). Uniform padding everywhere is what makes an interface look machine-generated.
*   Radius: `4px` inputs and buttons, `8px` panels, `999px` pills. Nothing else.
*   Content max-width 1440px. Data tables may go full-bleed.
*   **Cards are not the default.** Whitespace and alignment group things perfectly well. Use a bordered panel only when content is genuinely separable and actionable. **Never nest a card in a card.**
*   Grids: `repeat(auto-fit, minmax(280px, 1fr))` before reaching for breakpoints.

**Elevation** (light mode only; dark uses surface lightness instead):

```css
--shadow-sm: 0 1px 2px oklch(30% 0.02 28 / 0.06);
--shadow-md: 0 4px 14px oklch(30% 0.02 28 / 0.08);
--shadow-lg: 0 12px 32px oklch(30% 0.02 28 / 0.10);
```

If you can clearly see the shadow, it is too strong.

**z-index scale** — semantic, never arbitrary:
`dropdown 100` · `sticky 200` · `overlay 300` · `dialog 400` · `toast 500` · `tooltip 600`

* * *
## 6\. Component Vocabulary
### Buttons

| Variant | Appearance | Use |
| ---| ---| --- |
| Primary | Filled `--primary` | The one affirmative action per view |
| Secondary | `--surface` fill, `--border-strong` border | Everything else |
| Ghost | Transparent, text only | Toolbar and row actions |
| Danger | Outlined hue-22 + warning icon | Reject, cancel, remove |

Height 40px standard, 32px compact, 44px touch. Tap target always ≥44px even when the visual is smaller.

**All eight states required on every interactive element:** default, hover, focus-visible, active, disabled, loading, error, success. A component without a disabled state is unfinished.
### Inputs
*   Label always visible above the field. **No placeholder-as-label, ever.** It vanishes on typing and destroys the form for screen readers.
*   Placeholder shows the format, not an instruction.
*   Error sits below the field with an icon and names **the fix**, not the failure: "Use the format PJPUR-YYYY-NNNN", not "Invalid input".
*   Required marked on the label, not by colour.
*   Money inputs: `inputmode="decimal"`, mono, right-aligned, **format on blur, never mid-typing**.
### Status pills
Small, uppercase, tracked, **icon plus label**, tinted background, no border.
`Matched` success · `Pending checker` info · `Awaiting DSR` warning · `Variance` warning · `Rejected` danger · `Not started` neutral
### Maker-checker approval bar
The most repeated component in the system. It always shows **who submitted**, **when**, and a **before/after diff**. A checker approving a change they cannot see is a control on paper only. Approve is primary; Reject is danger-outlined and opens a required justification field. Sticky at the bottom of the record so actions stay reachable through a long diff.

* * *
## 7\. Data Tables — the workhorse
Most of CROWN is a table. Get this right and most of the product is right.

**Structure:**
*   `border-collapse: separate; border-spacing: 0`. **Required** for sticky headers in Safari (§12.2). Borders go on cells.
*   Header: `--surface-sunken`, `label` styling, sticky.
*   Row height 44px standard, 36px compact. Ship the density toggle; operators want compact.
*   Hover tints the row. No transition on the background, it feels laggy at scale.
*   **No zebra striping.** It fights with status tinting and adds noise.

**Columns:**
*   Amounts right-aligned, mono, tabular. Always.
*   Dates in a fixed format `07 Aug 2026`. **Never relative time in a table** — "2 days ago" is unusable in an audit context.
*   Text truncates with ellipsis plus `title`. Never wrap in a dense table.
*   **First column is the identity** (ATM ID or reference) and stays sticky on horizontal scroll.

**Variance rows — the whole point of the system:**
*   **Tint the cell, not the row.** A tinted row reads as selected.
*   The variance figure gets an explicit sign, an icon, and semantic colour. Three signals, so colour is never load-bearing.
*   **Sort by absolute variance descending by default.** The biggest problem should be the first thing on screen.

**Every table needs:** column sort, result count, sticky header, keyset pagination (**never infinite scroll** in an audit context), CSV/XLSX export, and a real empty state.

* * *
## 8\. States

| State | Requirement |
| ---| --- |
| Empty | Explain what belongs here and how to put it there. "No DSR for 07 Aug 2026. Vendors upload through the portal before the 09:00 WIB cutoff. Four of six have submitted." Never "No data". |
| Loading | Skeleton matching the real content shape. Never a centred spinner on a data view. |
| Error | What broke, whether it is retryable, one action. Never a bare error code. |
| Partial | When EOD has not completed, say so and disable the affected views. Silently showing yesterday's numbers is the worst possible failure in this system. |
| Stale | Any figure from a `processing_date` older than today carries a visible timestamp. |

* * *
## 9\. Motion
Restraint. This is an operations tool, not a marketing site.

| Duration | Use |
| ---| --- |
| 120ms | Button press, checkbox, colour change |
| 200ms | Hover, tooltip, dropdown |
| 300ms | Dialog, drawer, accordion |

```css
--ease-out: cubic-bezier(0.16, 1, 0.3, 1);
--ease-in:  cubic-bezier(0.7, 0, 0.84, 0);
```

*   **Animate** **`transform`** **and** **`opacity`** **only.** Everything else forces layout recalculation.
*   Exit animations run at ~75% of enter duration.
*   **No bounce, no elastic.** Real objects decelerate, they do not spring.
*   **Never stagger table rows.** Stagger on 200 rows is nausea.
*   Honour `prefers-reduced-motion: reduce` globally.

* * *
## 10\. Accessibility
*   **Never encode status with colour alone.** Every semantic state carries an icon or a word. This is also why variances stay readable in a photocopied berita acara.
*   Text contrast ≥ 4.5:1, large text ≥ 3:1. Deep red on light is safe for **bold text and fills only**, never small or thin text.
*   Visible `:focus-visible` ring everywhere: 2px, 2px offset, ≥3:1 contrast. Never `outline: none` without a replacement.
*   Full keyboard operability. Tables navigable, dialogs trap focus, Escape closes.
*   Use native `<dialog>` with `showModal()` plus `inert` on background content. Use the `popover` attribute for dropdowns and tooltips: no z-index wars, light dismiss, accessible by default.
*   Every icon-only button needs an `aria-label`.
*   Minimum tap target 44×44px.

* * *
## 11\. Tailwind Setup (v4)
Tailwind v4, CSS-first config. **No** **`tailwind.config.js`****\*\*\*\*.** Paste as `src/styles/theme.css`.

```css
@import "tailwindcss";

/* ---- Rule Zero ---- */
:root { color-scheme: light; }

@layer base {
  /* Hex first. An unsupported colour function drops the whole
     declaration, which is how you get transparent backgrounds
     and invisible text on older engines. */
  :root {
    --bg: #FBFAF9;  --surface: #FEFDFD;  --surface-raised: #FFFEFE;
    --surface-sunken: #F4F1F0;
    --border-c: #E7E3E1; --border-strong: #CFC9C6;
    --text: #2B2523; --text-secondary: #6E6461; --text-muted: #9C9491;
    --primary: #E4142A; --primary-hover: #C40F23; --primary-fg: #FFFCFB;
    --primary-text: #B00E1F; --primary-tint: #FCE9EA;
    --success: #1B7A4A; --success-tint: #E6F4EC;
    --warning: #A66A00; --warning-tint: #FBF0DE;
    --info:    #2A5FAF; --info-tint:    #E7EEF9;
    --danger:  #A00D22; --danger-tint:  #FBE8EA;
  }

  @supports (color: oklch(0% 0 0)) {
    :root {
      --bg:             oklch(98.6% 0.006 40);
      --surface:        oklch(99.6% 0.003 40);
      --surface-raised: oklch(99.8% 0.002 40);
      --surface-sunken: oklch(96.4% 0.008 38);
      --border-c:       oklch(91% 0.008 35);
      --border-strong:  oklch(83% 0.012 34);
      --text:           oklch(26% 0.02 30);
      --text-secondary: oklch(48% 0.016 30);
      --text-muted:     oklch(64% 0.012 32);
      --primary:        oklch(56% 0.223 27);
      --primary-hover:  oklch(50% 0.20 27);
      --primary-fg:     oklch(99% 0.004 40);
      --primary-text:   oklch(47% 0.185 27);
      --primary-tint:   oklch(95% 0.028 25);
      --success:        oklch(49% 0.13 151);
      --success-tint:   oklch(95% 0.030 151);
      --warning:        oklch(58% 0.13 70);
      --warning-tint:   oklch(95.5% 0.035 78);
      --info:           oklch(52% 0.11 250);
      --info-tint:      oklch(94.5% 0.025 250);
      --danger:         oklch(45% 0.19 22);
      --danger-tint:    oklch(94% 0.030 22);
    }
  }

  body { background: var(--bg); color: var(--text); font-optical-sizing: auto; }
  input, select, textarea, button { font: inherit; color: inherit; }
  input[type="checkbox"], input[type="radio"] { accent-color: var(--primary); }
}

@theme inline {
  --font-sans: "Plus Jakarta Sans", ui-sans-serif, system-ui, sans-serif;
  --font-mono: "IBM Plex Mono", ui-monospace, SFMono-Regular, monospace;

  --color-bg:             var(--bg);
  --color-surface:        var(--surface);
  --color-surface-raised: var(--surface-raised);
  --color-surface-sunken: var(--surface-sunken);
  --color-border:         var(--border-c);
  --color-border-strong:  var(--border-strong);
  --color-ink:            var(--text);
  --color-ink-secondary:  var(--text-secondary);
  --color-ink-muted:      var(--text-muted);
  --color-primary:        var(--primary);
  --color-primary-hover:  var(--primary-hover);
  --color-primary-fg:     var(--primary-fg);
  --color-primary-text:   var(--primary-text);
  --color-primary-tint:   var(--primary-tint);
  --color-success:        var(--success);
  --color-success-tint:   var(--success-tint);
  --color-warning:        var(--warning);
  --color-warning-tint:   var(--warning-tint);
  --color-info:           var(--info);
  --color-info-tint:      var(--info-tint);
  --color-danger:         var(--danger);
  --color-danger-tint:    var(--danger-tint);

  --radius-sm: 4px;
  --radius-md: 8px;
  --ease-out: cubic-bezier(0.16, 1, 0.3, 1);
}
```

Usage: `bg-surface`, `text-ink-secondary`, `border-border`, `text-primary-text`, `font-mono`.

**Component library: shadcn/ui** — **LOCKED**, see `90-DECISIONS-LOG` ADR-0012.

Approved subset, nothing else without an ADR amendment:
**Button · Input · Select · Dialog · Popover · Table · Badge · Toast · Tabs · Form**
*   Components are **copied into** **`src/components/ui/`** **and committed**. They are project source, not a dependency.
*   **Restyle to CROWN tokens on arrival.** No default shadcn styling ships. A component still carrying stock styling has not been adopted, it has been pasted.
*   **Radix is the only new runtime dependency** and is the sanctioned exception to the no-new-libraries rule in `01-ARCHITECTURE` §1.
*   **Never strip the accessibility behaviour.** Removing a focus trap or an ARIA attribute to fix a styling problem is a defect, not a workaround.
*   Upgrades are manual and reviewed. No silent version drift.

* * *
## 12\. Safari & macOS Survival Guide
Every item below has produced a real visual defect. Apply all of it.
### 12.1 The dark-mode hijack — see §0
### 12.2 Sticky table headers
Safari refuses `position: sticky` on `<thead>` and breaks it entirely under `border-collapse: collapse`.

```css
table { border-collapse: separate; border-spacing: 0; }
thead th {
  position: sticky; top: 0; z-index: 200;
  background: var(--surface-sunken);
  box-shadow: inset 0 -1px 0 var(--border-c); /* NOT border-bottom */
}
```

Stick the `th`, not the `thead`. Use `box-shadow` for the rule: borders detach from sticky cells while scrolling.
### 12.3 Text rendering
macOS renders type heavier than Windows, and light-on-dark heavier still.

```css
/* Light mode: leave default rendering alone, it is correct. */
@media (prefers-color-scheme: dark) {
  body {
    -webkit-font-smoothing: antialiased;
    font-weight: 350;
    line-height: 1.58;
    letter-spacing: 0.008em;
  }
}
```

**Do not apply** **`-webkit-font-smoothing: antialiased`** **globally.** On light backgrounds it thins the type and makes the whole interface look weak and washed out. This is a common cause of "the mockup looks off on Mac".
### 12.4 Autofill
Safari overrides input backgrounds on autofill and cannot be unset, only covered.

```css
input:-webkit-autofill,
input:-webkit-autofill:hover,
input:-webkit-autofill:focus {
  -webkit-text-fill-color: var(--text);
  -webkit-box-shadow: 0 0 0 1000px var(--surface) inset;
  caret-color: var(--text);
  transition: background-color 9999s ease-out 0s;
}
```

### 12.5 Form controls

```css
select { -webkit-appearance: none; appearance: none; }
input[type="date"] { min-height: 40px; }
input[type="date"]::-webkit-date-and-time-value { text-align: left; }
```

Safari's native `<select>` and `<input type="date">` cannot be fully styled. On any screen where they matter visually, use the shadcn Select and Date Picker instead of native controls.
### 12.6 Scrollbars

```css
* { scrollbar-width: thin; scrollbar-color: var(--border-strong) transparent; }
::-webkit-scrollbar { width: 10px; height: 10px; }
::-webkit-scrollbar-track { background: transparent; }
::-webkit-scrollbar-thumb {
  background: var(--border-strong);
  border-radius: 999px;
  border: 3px solid var(--bg);
}
```

Safari below 18.2 ignores `scrollbar-color`, so both syntaxes are required.
### 12.7 Assorted traps

| Trap | Fix |
| ---| --- |
| `100vh` overflows on iOS Safari | Use `100dvh` |
| `backdrop-filter` does nothing | Add `-webkit-backdrop-filter` (and use it almost never) |
| `position: absolute` inside `overflow: hidden` clips dropdowns | Use the `popover` attribute or `position: fixed` |
| Flex children overflow instead of shrinking | `min-width: 0` on the flex child |
| Momentum scroll jitter in panels | `-webkit-overflow-scrolling: touch` |
| `text-wrap: balance` unsupported below Safari 17.5 | Progressive enhancement, degrades harmlessly |

### 12.8 Test matrix
Per the TSD, supported browsers are Edge, Firefox, and Chrome. **Safari is not a target browser for the product, but it is what development happens on**, so it must render correctly or every mockup review judges the wrong thing.

Test each screen in: Chrome light, Chrome dark, **Safari macOS light, Safari macOS dark**, Edge light, Firefox light, and at 320px width.

* * *
## 13\. Print (berita acara and audit evidence)
Cash count minutes, recon results, and approval trails get printed and filed. Print is a first-class output.

```css
@media print {
  :root { color-scheme: light; }
  body { print-color-adjust: exact; -webkit-print-color-adjust: exact; }
  nav, aside, .no-print, button { display: none !important; }
  thead { display: table-header-group; }  /* repeat header per page */
  tr { break-inside: avoid; }
  a[href]::after { content: " (" attr(href) ")"; font-size: 0.75em; }
}
```

Every printed page carries the document reference, `processing_date`, generation timestamp, and page N of M. An audit document without provenance is worthless.

* * *
## 14\. What AI Must Never Do Here
Hard bans. Each one produces output that looks machine-generated.
*   **Side-stripe borders.** A thick coloured `border-left` on a card, alert, or list item. Never intentional design. Use a full border, a background tint, or an icon.
*   **Gradient text.** `background-clip: text` on a gradient. Emphasis comes from weight and size.
*   **Glassmorphism.** Blurred translucent panels used decoratively.
*   **The hero-metric block.** Giant number, tiny label, gradient accent, three supporting stats.
*   **Identical card grids.** Repeated equal-sized cards with icon, heading, body text.
*   **Modal as the first idea.** Exhaust inline and progressive alternatives first.
*   **Emoji as interface icons.** Lucide only, one consistent stroke weight.
*   **Any gradient.** This palette has one hue.
*   **Placeholder text as a label.**
*   **A theme toggle.** Phase 1 is light only (§0).
*   **A third font family.**
*   **Colour as the only carrier of meaning.** Anywhere, ever.
### Prompt template for frontend work
> Read `.ai/00-AI-ENTRYPOINT.md` and `.ai/05-UI-BRAND.md`. Build `<component>` for the `<internal app | vendor portal>`. Use existing tokens and the shadcn subset only, no new dependencies. Light mode only, `color-scheme: light` declared. Include all eight interactive states plus empty, loading, and error. Money uses `--font-mono`, right-aligned, with the IDR code shown. No colour-only status. Plan the files and structure first, then wait for approval.