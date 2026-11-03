# Design Document: Vendor Detail Revamp

## Overview

This is a **presentation-layer revamp** of the existing vendor detail page (`frontend/CompanyPortal-Vite/src/features/admin-vendors/`). It re-skins `VendorDetailPage.tsx` and its three tab panels to match the `crown-vendor-detail` artifact, and upgrades the Harga Paket tab from a plain `DataTable` into a filterable price sheet with a live tier count and three-state status pills. It adds a summary strip to the header.

The single hard constraint: **behavior, data contracts, and maker-checker semantics do not change.** Every endpoint, hook, mutation, and authorization guard already exists and stays exactly as-is. We are changing markup, tokens, layout, the computed presentation helpers (status, range, formatting), and the Harga Paket filter/count UI. Nothing else.

### What exists today (verified in code)

| Piece | File | Status |
| --- | --- | --- |
| Page shell + tabs | `components/VendorDetailPage.tsx` | 4 tabs already: Info, Cabang, PIC Vendor-wide, Harga Paket |
| Cabang | `components/BranchesPanel.tsx` | `DataTable`, links to branch detail, create/edit/disable/enable (maker-checker) |
| PIC Vendor-wide | `components/PicsPanel.tsx` (`branchId={null}`) | `DataTable`, create/edit/disable/enable, warnings |
| Harga Paket | `components/PackagePricesPanel.tsx` | `DataTable`, create/edit/end, pending badge |
| Data hooks | `hooks.ts` | `useVendor`, `useVendorBranches`, `useVendorPics`, `useVendorPackagePrices`, all mutations |
| Types | `types.ts` | `AdminVendor`, `AdminVendorBranch`, `AdminVendorPic`, `AdminVendorPackagePrice` |
| Pending set / badge | `../master-data/pending`, `../master-data/PendingApprovalBadge` | `usePendingEntityIds("vendor_package_price")`, etc. |

### What the design mock gets wrong (and how we reconcile)

The mock (`UI-design/crown-vendor-detail/src/data.js`) is a design fiction. Reconciliation table:

| Mock concept | Real contract | Decision |
| --- | --- | --- |
| Flat `tier` with client `id`/`seq` | `AdminVendorPackagePrice.id` from server | Use server `id`. Drop `seq`. |
| `kodePaket()` synthesized `PKG3_ABA_009` | `package_code` field from server (resolved by `vendor-package-code-split`, migration 017: `package_code` is now a real, server-generated, globally-unique per-row column — it is no longer the label) | Render `package_code` verbatim; **do not** synthesize codes. The mock's separate "Paket" grouping concept maps to the sibling `package` field (the label, e.g. "PAKET 3") — group/filter by `package`, display the per-row code from `package_code`. |
| `harga: 2003100` (number) | `base_price: "2003100.00"` (decimal string) | Display the string; never `Number()` it. IDR range needs care — see Money handling. |
| `mesin: "ATM" \| "CRM"` | `machine_group: "ATM" \| "CDM_CRM"` | Map `CDM_CRM` → display "CDM/CRM". |
| `kelas: "Regular"` | `price_class: "REGULAR" \| "VIP_INDUSTRI"` | Map to "Regular" / "VIP/Industri". |
| `tipe: "Standar"` | no field | **Drop the Tipe column.** It has no backing data. |
| Edit applies instantly + toast "disimpan" | mutation → 202 staged | Keep existing "diajukan untuk persetujuan" toast + pending badge. |
| `min`/`max` unit tiers | `tier_min` / `tier_max` (nullable max) | Direct map; `null` max → `${min}+`. |
| No override level | `vendor_branch_id` / `atm_id` optional | Keep the existing "Tingkat Harga" (level) column: PT / Cabang / ATM. |

## Architecture

### Component tree (after revamp)

```
VendorDetailPage (vendorId)
├─ VendorHeader (new)                         ← Req 1, 2
│   ├─ back link → /settings/admin/vendors
│   ├─ code tile + title + subtitle + status Badge
│   └─ SummaryStrip (new)                      ← Req 2
│       ├─ Cabang aktif      (useVendorBranches)
│       ├─ PIC vendor-wide   (useVendorPics, vendor_wide_only)
│       ├─ Paket berlaku     (useVendorPackagePrices)
│       └─ Rentang harga aktif (useVendorPackagePrices)
├─ TabBar (existing pattern, restyled)         ← Req 3
└─ panel by tab
    ├─ InfoTab (new component, extracted)       ← Req 4
    ├─ BranchesPanel (restyle only)             ← Req 5
    ├─ PicsPanel branchId={null} (restyle only) ← Req 6
    └─ PackagePricesPanel (upgrade)             ← Req 7-10
        ├─ PriceFilterBar (new)                 ← Req 7
        ├─ DataTable (restyled columns)         ← Req 8, 9
        ├─ VendorPackagePriceFormDialog (existing)
        └─ ConfirmActionDialog (existing)
```

### Shared presentation helpers (new, single source — Req 9.5)

A small `packagePrice.ts` helper module under `admin-vendors/lib/` (create the `lib/` sibling that already exists in the feature folder) holds the derivations reused by the header, the filter, and the row:

```ts
export type PriceStatus = "Berlaku" | "Dijadwalkan" | "Berakhir";

// today: caller passes an ISO yyyy-mm-dd so the function is pure and testable.
export function priceStatus(p: AdminVendorPackagePrice, todayISO: string): PriceStatus;

export function machineLabel(g: "ATM" | "CDM_CRM"): string;      // "ATM" | "CDM/CRM"
export function classLabel(c: "REGULAR" | "VIP_INDUSTRI"): string;// "Regular" | "VIP/Industri"
export function levelLabel(p: AdminVendorPackagePrice): string;  // "PT (dasar)" | "Cabang #..." | "ATM #..."
export function tierLabel(p: AdminVendorPackagePrice): string;   // "1-50" | "251+"
export function periodLabel(p: AdminVendorPackagePrice): { start: string; end: string };
export function formatIDR(decimalString: string): string;        // grouping only, string-in/string-out
```

`priceStatus` is the **single** implementation. The existing inline `isOpenEnded` check in `PackagePricesPanel` (`effective_end_date === null || >= today`) is replaced by a call to `priceStatus`, and the header/filter use the same function. This is the one behavioral consolidation the revamp makes, and it must preserve today's meaning (see Correctness Properties).

> Note: the mock has a "Dijadwalkan" (scheduled/future-start) state the current panel does not distinguish — it only shows Berlaku vs Berakhir. Adding Dijadwalkan is a **presentation refinement**, not a data change: a future `effective_start_date` already exists in the contract; the current UI just lumps it into Berlaku. The revamp surfaces it. Actions on a Dijadwalkan row: Ubah allowed, Akhiri not (nothing in effect to end) — see Req 10.2/10.3.

### Money handling (Req 2.5, 8.2, 12.5) — the one careful bit

`base_price` is an exact decimal string (e.g. `"2003100.00"`). The rules:

- **Row display**: format with digit grouping only, operating on the string (split on `.`, group the integer part). Never `Number()` / `parseFloat`. Prefix `IDR`.
- **Summary price range** (min/max): min/max of a set of decimal strings requires a comparison. Use a **string-safe decimal compare** (normalize by left-padding the integer part to equal width, then lexical compare) — not float conversion. Only Berlaku rows with non-null `base_price` participate (Req 2.4, 2.8). Empty set → placeholder "—" (Req 2.7).
- The helper returns formatted strings; no arithmetic leaves the helper as a number.

This avoids the classic float-rounding bug on IDR amounts (Golden Rule: money is never float).

### Data flow — no new fetches beyond what the tabs already do

The `SummaryStrip` needs branch, PIC, and price counts. Rather than invent an aggregate endpoint (out of scope), it reuses the same three list hooks the tabs use, called at the header level with `page_size: 100, status: "all"` (matching the panels' existing calls). TanStack Query dedupes identical query keys, so the header and the Cabang/PIC/Harga tabs share one cached result each — no extra network cost. This is consistent with how the panels already over-fetch (`page_size: 100`).

- Active-branch count: `branches.filter(b => b.is_active).length` / `branches.length`.
- Vendor-wide PIC count: `useVendorPics(vendorId, { vendor_wide_only: true, ... }).total` (or list length).
- Paket berlaku / range: derived from `useVendorPackagePrices` via `priceStatus`.

Loading: while any source query is loading, that metric shows a neutral placeholder (Req 2.6), not `0`.

## Components and Interfaces

### VendorHeader (new)

Props: `{ vendor: AdminVendor | undefined; vendorId: number }`. Internally calls the three list hooks for the strip (or receives them as props from the page to keep fetching centralized — implementer's choice, but keep it one query each). Renders:

- Back link (`Link to="/settings/admin/vendors"`) styled as the design's red ghost link.
- Code tile (dark `--n-900` bg, tinted-red text) + eyebrow ("Manajemen Vendor") + `PageHeader`-style title `{code} · {name}` + subtitle (legal name).
- Status `Badge` (success/danger, icon + text) — Req 1.3.
- `SummaryStrip` as a 4-cell bordered grid using design-system tokens.

Uses the existing `PageHeader` where it fits, or composes equivalent markup with tokens; the eyebrow/title/description already map to `PageHeader`'s props.

### InfoTab (new, extracted from the inline block in VendorDetailPage)

Props: `{ vendor: AdminVendor }`. Two-column layout: a profile card (labelled fields, `—` for empty per Req 4.3) and an address card (`hq_address` + the standing note about legal/NPWP changes going through Vendor Management). Status rendered as a `Badge`. **No edit control** (Req 4.5).

### PriceFilterBar (new, inside PackagePricesPanel)

Props: `{ packageOptions: string[]; machineOptions: string[]; value: PriceFilters; onChange: ...; matchCount: number }`. Three `<select>` controls (Semua paket / Semua mesin / Semua status) + a "{n} tingkat" count. Options derived from the vendor's own prices (Req 7.5). All filtering is **client-side** over the already-fetched `package_prices` list (the panel already fetches all with `page_size: 100`).

### PackagePricesPanel (upgraded)

Keeps its hooks (`useVendorPackagePrices`, `useDisableVendorPackagePrice`, `useUpdate/Create`), `usePendingEntityIds("vendor_package_price")`, `VendorPackagePriceFormDialog`, `ConfirmActionDialog`. Changes:

- Add local filter state; compute `filtered` + `sorted` (by package_code, machine_group, price_class, tier_min) with `useMemo`.
- Replace inline `isOpenEnded` with `priceStatus(...)`; drive the status `Badge` with the three-state result.
- Columns: Kode Paket (`package_code`, mono/tabular), Paket, Mesin, Kelas, Tingkat (unit), Harga Dasar (right, tabular, string), Periode (start → end), Status (Badge + pending), Aksi. **Drop the mock's "Tipe" column** (no data).
- Action gating by status: Berlaku → Ubah + Akhiri; Dijadwalkan → Ubah only; Berakhir → "Diakhiri {date}" text, no actions (Req 10.2-10.4). Pending → both disabled (Req 10.6).
- Empty vs no-match: `emptyMessage` for zero prices; a separate "Tidak ada tingkat yang cocok dengan filter." when the list is non-empty but the filter matches nothing (Req 7.6).

### BranchesPanel / PicsPanel (restyle only)

No prop or behavior change. Restyle the container/table chrome to match the design (card border, uppercase muted headers, quiet red row hover, category pill). Keep `DataTable`, `Badge` (icon+text), `PendingApprovalBadge`, and every existing action/dialog. PicsPanel's design shows a **card grid** for PICs; adopting that is optional and can stay as the existing table if `DataTable` restyle is simpler — the requirements only mandate the fields, notification distinction, actions, empty state, and warnings (Req 6), not a specific table-vs-card form. Recommended: keep `DataTable` for consistency and lower risk; add the notification-recipient distinction as a badge/star.

## Design-system mapping (Req 11)

The mock hard-codes hex (`#C8102E`, `#F7F4F0`, `#241B19`, Work Sans / Source Sans 3). The CMS uses **Merah Sirih** tokens. Translate, do not copy hex:

| Mock hex | Token | Use |
| --- | --- | --- |
| `#C8102E` brand red | `--red-500` / `--red-600` | primary buttons, active tab underline, focus, links |
| `#F7F4F0` paper | `--n-50` | page background |
| `#FFFEFC` card | `--n-0` | cards, table surface |
| `#E9E1DA` hairline | `--n-200` | borders, dividers |
| `#241B19` ink | `--n-900` | headings |
| `#7A6A63` muted | `--n-500` / `--n-600` | captions, metadata |
| green status | `success` role tokens | Berlaku / Aktif |
| amber status | `warning` role tokens | Dijadwalkan |
| grey status | neutral tokens | Berakhir |
| red destructive | `danger` role | Akhiri / Nonaktifkan |

Fonts: use the CMS system stack (design steering §6), not Work Sans / Source Sans 3 — the CMS is deliberately system-font. Money and tiers use `tabular-nums` (Req 11.2). Status always `Badge` (icon + text, Req 11.3). Brand red stays ≤10% and off destructive actions (Req 11.4-11.5). Focus rings preserved (Req 11.6). No framer-motion dependency is required; the modal/toast animation in the mock is optional polish and should reuse whatever `ConfirmActionDialog` / the toast system already does rather than introducing a new animation lib.

## Correctness Properties

These are the invariants worth testing. They center on the two risk areas: status derivation (drives visibility, filters, actions, and the header) and money (must never go through float).

- **Property 1 — Price status is a total, mutually-exclusive classification.** For any `AdminVendorPackagePrice` and any `todayISO`, `priceStatus` returns exactly one of Berlaku / Dijadwalkan / Berakhir, following: future start ⇒ Dijadwalkan; else end ≤ today ⇒ Berakhir; else Berlaku. Boundary dates (start == today, end == today) resolve per Req 9.1-9.3 (start==today is not future ⇒ not Dijadwalkan; end==today ⇒ Berakhir). _Validates Req 9.1, 9.2, 9.3._

- **Property 2 — Status consolidation preserves today's Berlaku/Berakhir split.** For any price, the set {Berlaku, Dijadwalkan} under the new `priceStatus` maps onto exactly the rows the current panel treated as `isOpenEnded` (open-ended-or-future), and Berakhir maps onto the rows it treated as ended. No row that is actionable today becomes non-actionable except a future-start row moving from Ubud+Akhiri to Ubah-only, which is intended. _Validates Req 9.5, 10.2, 10.3, 10.4 (no regression)._

- **Property 3 — Money formatting is string-pure.** For any decimal string `base_price`, `formatIDR` produces a grouped string whose digits, in order, equal the input's digits (ignoring the inserted grouping separators and preserving the decimal part), and never invokes `Number`/`parseFloat`. _Validates Req 8.2, 12.5._

- **Property 4 — Active price range bounds are members of the set.** For any set of Berlaku prices with non-null `base_price`, the range min and max returned for the Summary_Strip are each equal to one of the input `base_price` strings (min ≤ every element, max ≥ every element under the string-safe decimal compare), and a set with fewer than one qualifying row yields the placeholder. _Validates Req 2.5, 2.7, 2.8._

- **Property 5 — Filtering is a subset that respects "all".** For any filter selection, the filtered rows are a subset of all rows, every filtered row matches each non-"all" filter, and when all three filters are "all" the filtered set equals the full set. The reported tier count equals the filtered row count. _Validates Req 7.2, 7.3, 7.4._

## Testing Strategy

Frontend-only, mirroring the repo's existing `ATMsPanel.test.tsx` (RTL + MSW) and any helper unit tests:

- **Unit (pure helpers)** — `packagePrice.ts`: table-driven tests for `priceStatus` (boundary dates), `formatIDR` (grouping, decimals, no float), the decimal min/max compare, and the label mappers. These carry Properties 1, 3, 4 and are cheap and high-value.
- **Component (RTL + MSW)** — `PackagePricesPanel`: filters narrow the list and update the count (Property 5); three-state status badges render with icon + text; action gating by status (Berlaku vs Dijadwalkan vs Berakhir); pending row disables actions; a 202 mutation shows the "diajukan untuk persetujuan" message not "disimpan"; error path surfaces an error. `VendorHeader`/`SummaryStrip`: metrics render, placeholders while loading, range placeholder when no Berlaku rows.
- **Component (light)** — `InfoTab` renders fields + `—` placeholder + status badge, no edit control. `BranchesPanel`/`PicsPanel` smoke tests only assert the restyle didn't drop the link, badges, actions, or empty state (behavior unchanged).

No backend, integration-DB, or migration tests — nothing below the presentation layer changes. Coverage target follows the frontend norm; the pure helpers should be near-fully covered since they hold the money and status logic.

## Out of Scope / Non-Goals

- Any backend, SQL, migration, endpoint, or response-shape change.
- Any change to maker-checker flow, approval routing, or authorization.
- A vendor-profile edit control on the Info tab (edit stays on the vendor list).
- An aggregate "vendor summary" endpoint (the strip reuses existing list queries).
- Per-branch detail page restyle (`VendorBranchDetailPage.tsx`) — the design artifact covers the vendor-level page only; branch-level pages are a separate follow-up if desired.
- Introducing framer-motion or new fonts; the CMS uses its system stack and existing animation primitives.
