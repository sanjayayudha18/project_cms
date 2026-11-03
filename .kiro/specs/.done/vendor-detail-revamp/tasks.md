# Implementation Plan: Vendor Detail Revamp

## Overview

Frontend-only revamp of `frontend/CompanyPortal-Vite/src/features/admin-vendors/`. No backend, SQL, migration, endpoint, or contract change. The plan builds the shared presentation helpers first (they carry the money and status logic), then the new header/strip, then upgrades the Harga Paket panel, then restyles the two simple panels, then wires it all into the page.

Tasks are ordered so pure helpers land before the components that consume them. All money stays a decimal string; every status derivation routes through the single `priceStatus` helper; every mutation keeps its existing maker-checker (202-staged) path and messaging.

All new code mirrors existing feature patterns: `Badge`/`Button`/`DataTable`/`ConfirmActionDialog` from `@/components/ui`, `usePendingEntityIds` + `PendingApprovalBadge` from `../master-data`, hooks from `./hooks`, tests like `ATMsPanel.test.tsx` (RTL + MSW).

## Tasks

- [ ] 1. Create the shared package-price presentation helpers
  - Create `frontend/CompanyPortal-Vite/src/features/admin-vendors/lib/packagePrice.ts` with: `PriceStatus` type; `priceStatus(p, todayISO): PriceStatus` (future start → Dijadwalkan; end ≤ today → Berakhir; else Berlaku); `machineLabel`, `classLabel`, `levelLabel`, `tierLabel`, `periodLabel`; `formatIDR(decimalString)` (grouping only, string-in/string-out, never `Number`/`parseFloat`); and `compareDecimalStrings(a, b)` + `activePriceRange(prices, todayISO)` returning `{ minLabel, maxLabel } | null` over Berlaku, non-null `base_price` rows.
  - Keep every function pure (today passed in as an ISO string) so they are directly unit-testable.
  - _Requirements: 2.5, 2.7, 2.8, 8.2, 8.4, 8.5, 8.6, 9.1, 9.2, 9.3, 9.5, 12.5_
  - _Model: Opus, Effort: High — this module holds the money (string-pure IDR formatting + decimal min/max) and the status classification that drives visibility, filters, and actions; a float slip or a boundary-date error propagates everywhere and is the one real correctness risk in a frontend-only change._

- [ ]* 1.1 Unit-test the helpers
  - Table-driven tests for `priceStatus` (start==today, end==today, future start, open end, past end); `formatIDR` (grouping, decimal part preserved, digits unchanged, no float — Property 3); `compareDecimalStrings`/`activePriceRange` (bounds are members, empty set → null — Property 4); label mappers (`CDM_CRM`→"CDM/CRM", `VIP_INDUSTRI`→"VIP/Industri", null tier_max → `${min}+`, override levels).
  - **Validates: Properties 1, 3, 4; Requirements 8.2, 9.1, 9.2, 9.3, 2.7, 2.8**
  - _Model: Sonnet, Effort: Medium — thorough table-driven coverage of pure functions; the reasoning already lives in task 1, this pins the boundaries._

- [ ] 2. Extract the InfoTab component
  - Create `components/InfoTab.tsx` (props `{ vendor: AdminVendor }`) as a two-card layout: profile fields (code, name, legal_name, npwp, contact_email, contact_phone) with `—` for empty values, plus a status `Badge` (success/danger, icon + text); and an address card rendering `hq_address` with the standing legal/NPWP-change note. No edit control.
  - Use Merah Sirih tokens and `tabular-nums` where a figure appears; no hard-coded hex.
  - _Requirements: 4.1, 4.2, 4.3, 4.4, 4.5, 11.1, 11.3_
  - _Model: Sonnet, Effort: Medium — straightforward presentational component, but must honor the placeholder, no-edit, and badge rules exactly._

- [ ] 3. Build the SummaryStrip
  - Create `components/SummaryStrip.tsx` rendering four metric cells: Cabang aktif (`${active}/${total}`), PIC vendor-wide (count), Paket berlaku (count of Berlaku via `priceStatus`), Rentang harga aktif (`activePriceRange`, placeholder "—" when null).
  - Source data from the existing list hooks (`useVendorBranches`, `useVendorPics` with `vendor_wide_only`, `useVendorPackagePrices`, each `page_size: 100, status: "all"`); show a neutral placeholder per metric while its query loads (never a misleading 0).
  - _Requirements: 2.1, 2.2, 2.3, 2.4, 2.5, 2.6, 2.7, 2.8, 11.1, 11.2_
  - _Model: Sonnet, Effort: Medium — real work reconciling three async sources with placeholder-vs-zero rules and the shared range helper; a close mirror of existing hook usage._

- [ ] 4. Build the VendorHeader
  - Create `components/VendorHeader.tsx` (props `{ vendor, vendorId }`): back link to `/settings/admin/vendors`, code tile, eyebrow, `{code} · {name}` title, legal-name subtitle, status `Badge` (icon + text), and `SummaryStrip`. Render a non-error placeholder title while the vendor is loading.
  - Reuse `PageHeader` for the eyebrow/title/description where it fits; tokens only, no hex.
  - _Requirements: 1.1, 1.2, 1.3, 1.4, 1.5, 11.1, 11.4_
  - _Model: Sonnet, Effort: Medium — composition of the strip + identity block with a loading placeholder; established pattern._

- [ ] 5. Add the PriceFilterBar
  - Create `components/PriceFilterBar.tsx` (props `{ packageOptions, machineOptions, value, onChange, matchCount }`): three selects (Semua paket / Semua mesin / Semua status with Berlaku/Dijadwalkan/Berakhir) and a "{matchCount} tingkat" label. Options derived from the vendor's own prices. `Tambah Harga Paket` primary button lives here or in the panel header per the design.
  - _Requirements: 7.1, 7.5, 10.1, 11.1, 11.4_
  - _Model: Sonnet, Effort: Medium — a controlled filter bar; option derivation and the count label are the only logic._

- [ ] 6. Upgrade the PackagePricesPanel
  - In `components/PackagePricesPanel.tsx`: add local filter state; `useMemo` `filtered` (match all non-"all" filters over the fetched list) and `sorted` (`package`, machine_group, price_class, tier_min -- sort by the label `package`, not the per-row `package_code`, per `vendor-package-code-split` migration 017); pass `matchCount` to `PriceFilterBar`.
  - Replace the inline `isOpenEnded` check with `priceStatus(...)`; render the status `Badge` in three states (Berlaku success, Dijadwalkan warning, Berakhir neutral), each icon + text, plus `PendingApprovalBadge` when pending.
  - Columns: Kode Paket (`package_code`, tabular), Paket (`package`), Mesin (`machineLabel`), Kelas (`classLabel`), Tingkat (`tierLabel`), Harga Dasar (right, `tabular-nums`, `formatIDR`, "—" when null), Periode (`periodLabel`), Status, Aksi. Do not add a "Tipe" column.
  - Action gating: Berlaku → Ubah + Akhiri; Dijadwalkan → Ubah only; Berakhir → "Diakhiri {end}" text, no actions; pending → Ubah/Akhiri disabled. Keep `VendorPackagePriceFormDialog` and `ConfirmActionDialog`; keep the 202-staged toast (`pendingApprovalMessage`) and error toast unchanged.
  - Empty state (`emptyMessage`) for zero prices; a distinct "Tidak ada tingkat yang cocok dengan filter." when filtered-to-empty over a non-empty list.
  - _Requirements: 7.2, 7.3, 7.4, 7.6, 8.1, 8.2, 8.3, 8.4, 8.5, 8.6, 9.4, 9.5, 10.2, 10.3, 10.4, 10.5, 10.6, 10.7, 10.8, 11.2, 11.3, 11.5, 12.1, 12.2, 12.5_
  - _Model: Opus, Effort: High — the panel is where status, money display, action gating, filtering, and maker-checker messaging all meet; regressing the 202-staged semantics or the pending-disable gate is a correctness/authorization risk, and the Dijadwalkan branch is new behavior to get right._

- [ ]* 6.1 Component-test the upgraded PackagePricesPanel (RTL + MSW)
  - Filters narrow the list and update the "{n} tingkat" count (Property 5); three-state status badges each render icon + text; action gating per status (Berlaku vs Dijadwalkan vs Berakhir); pending row disables actions; a 202 create/edit/end shows the "diajukan untuk persetujuan" message, not "disimpan"; mutation failure surfaces an error; filtered-to-empty shows the no-match message distinct from the empty state.
  - **Validates: Requirements 7.2, 7.4, 7.6, 9.4, 10.2, 10.3, 10.4, 10.5, 10.6, 10.7**
  - _Model: Sonnet, Effort: Medium — broad but conventional RTL+MSW suite over the states; mirrors ATMsPanel.test.tsx._

- [ ] 7. Restyle BranchesPanel and PicsPanel (behavior unchanged)
  - Apply the design chrome (card border, uppercase muted headers, quiet red row hover, category pill; notification-recipient distinction on PICs) to `components/BranchesPanel.tsx` and `components/PicsPanel.tsx` using Merah Sirih tokens. Keep `DataTable`, every column's data, `Badge` (icon + text), `PendingApprovalBadge`, the branch-code link, all create/edit/disable/enable actions and dialogs, empty states, and PIC warnings exactly as-is.
  - _Requirements: 5.1, 5.2, 5.3, 5.4, 5.5, 5.6, 6.1, 6.2, 6.3, 6.4, 6.5, 6.6, 11.1, 11.3, 12.4_
  - _Model: Sonnet, Effort: Medium — restyle without touching behavior; the risk is accidentally dropping a link, action, badge, or warning, so it needs care but not deep reasoning._

- [ ] 8. Wire the revamped pieces into VendorDetailPage
  - In `components/VendorDetailPage.tsx`: replace the inline `PageHeader` + back link with `<VendorHeader vendor={vendor} vendorId={vendorId} />`; replace the inline Info `<dl>` block with `<InfoTab vendor={vendor} />`; keep the restyled `TabBar`, the four tabs in order, the default tab, accessible tab semantics, and the vendor-not-found error branch. Cabang/PIC/Harga tabs keep rendering their (restyled/upgraded) panels.
  - _Requirements: 1.6, 3.1, 3.2, 3.3, 3.4, 3.5, 12.3_
  - _Model: Sonnet, Effort: Medium — assembly of the new components into the existing shell while preserving tab semantics and the error branch._

- [ ]* 8.1 Light component tests for the page shell, VendorHeader/SummaryStrip, InfoTab
  - VendorHeader: title/subtitle/status render; loading placeholder title. SummaryStrip: four metrics render; per-metric placeholder while loading; range placeholder when no Berlaku rows. InfoTab: fields + `—` placeholder + status badge, no edit control. Page: four tabs in order, selecting a tab shows only its panel, not-found error branch renders.
  - **Validates: Requirements 1.1, 1.3, 1.5, 2.1, 2.6, 2.7, 3.1, 3.2, 4.1, 4.3, 4.5**
  - _Model: Sonnet, Effort: Medium — focused RTL assertions across the new presentational pieces._

- [ ] 9. Checkpoint — build, lint, and run frontend tests
  - From `frontend/CompanyPortal-Vite/`: `pnpm lint`, `pnpm test --run`, `pnpm build`. Fix any type or lint errors introduced by the revamp. Confirm no new dependency (no framer-motion, no new font) was added.
  - _Requirements: 11.1, 11.6, 12.1_
  - _Model: Sonnet, Effort: Medium — triage build/lint/test failures across the changed feature; judgment but not deep reasoning._

## Notes

- Tasks marked `*` are tests and can be deferred for a faster visual MVP, but the helper unit tests (1.1) guard the money and status logic and should not be skipped — they are the real safeguard in a change that otherwise only moves markup.
- `priceStatus` is the single status source (Req 9.5). Do not reintroduce an inline `isOpenEnded` check in any component.
- `base_price` stays an opaque decimal string end to end (Req 12.5). `formatIDR`/`compareDecimalStrings` must never call `Number`/`parseFloat`.
- The mock's "Tipe" column, `kodePaket()` synthesis, `seq`, and optimistic-apply toast are design fictions — none are implemented (see design.md reconciliation table).
- No backend, migration, endpoint, or contract change in this spec. If a reviewer thinks one is needed, stop and flag it — it would be out of scope.

## Task Dependency Graph

```json
{
  "waves": [
    { "id": 0, "tasks": ["1", "2"] },
    { "id": 1, "tasks": ["1.1", "3", "5", "7"] },
    { "id": 2, "tasks": ["4", "6"] },
    { "id": 3, "tasks": ["6.1", "8"] },
    { "id": 4, "tasks": ["8.1"] },
    { "id": 5, "tasks": ["9"] }
  ]
}
```
