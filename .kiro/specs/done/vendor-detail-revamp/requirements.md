# Requirements Document

## Introduction

This feature is a **frontend-only visual and UX revamp** of the existing vendor detail page in the internal CompanyPortal (route `/settings/admin/vendors/:vendorId`), bringing it in line with the `crown-vendor-detail` design artifact (`UI-design/crown-vendor-detail`). The page already exists (`VendorDetailPage.tsx`) with the exact four-tab structure the design targets — Info, Cabang, PIC Vendor-wide, Harga Paket — and every backend endpoint it consumes is already built, wired, and maker-checker-guarded. **No backend change, no migration, no new endpoint, and no new data contract is in scope.** The work is limited to how the existing data and existing mutations are presented.

The revamp introduces: a vendor identity header with a four-metric summary strip and a status pill; a two-card Info layout (profile fields + registered address); a richer Cabang table and a card-based PIC Vendor-wide layout; and a substantially upgraded Harga Paket tab with inline filters (paket / mesin / status), a live tier count, three-state status pills (Berlaku / Dijadwalkan / Berakhir), and Ubah / Akhiri row actions backed by the existing add/edit dialog and end-of-validity confirmation.

Two facts constrain the design and must be honored, not papered over:

1. **The design mock is not the data contract.** The mock (`src/data.js`) uses a flat, mutable `tier` object with client-side `id`/`seq` and a synthetic `kodePaket`. The real screen is backed by `AdminVendorPackagePrice` (vendor-scoped, effective-dated, exact-decimal-string money) served flat-JSON from `/api/v1/admin/vendors/{vendorId}/package-prices`. Where the two disagree, the real contract wins and the visual treatment adapts to it.
2. **Every create / update / disable is maker-checker.** The design mock applies edits optimistically and shows a success toast ("Perubahan disimpan"). The real mutations return **202 Accepted (staged for approval)**, not an applied change. The revamp must preserve the existing "staged for approval" messaging and the pending-approval row badge — it must never present a staged change as if it were live.

This document is written so the revamp is a faithful **re-skin plus interaction upgrade**, never a regression in correctness, authorization, or approval semantics.

## Glossary

- **CMS**: The internal Cash Management System CompanyPortal (React + Vite, "Merah Sirih" internal theme).
- **Vendor_Detail_Page**: The existing page component at route `/settings/admin/vendors/:vendorId` (`VendorDetailPage.tsx`) that hosts the four tabs.
- **Vendor**: An `AdminVendor` record (`code`, `name`, `legal_name`, `npwp`, `contact_email`, `contact_phone`, `hq_address`, `is_active`), read from `GET /api/v1/admin/vendors/{vendorId}`.
- **Vendor_Header**: The identity block at the top of the Vendor_Detail_Page: back link, vendor code tile, title, subtitle, vendor status pill, and the Summary_Strip.
- **Summary_Strip**: The four-metric row in the Vendor_Header: active-branch count, vendor-wide PIC count, in-effect package count, and active price range.
- **Info_Tab**: The tab presenting Vendor profile fields and registered address.
- **Branches_Panel**: The Cabang tab listing the vendor's branches (`BranchesPanel.tsx`), each row linking to that branch's detail page.
- **Pics_Panel**: The PIC Vendor-wide tab listing vendor-wide PICs (`vendor_branch_id IS NULL`), rendered by the existing `PicsPanel` with `branchId = null`.
- **Package_Prices_Panel**: The Harga Paket tab listing the vendor's package prices (`PackagePricesPanel.tsx`).
- **Package_Price**: An `AdminVendorPackagePrice` row — vendor-scoped, effective-dated, with a `base_price` decimal string, optional `vendor_branch_id` / `atm_id` override level, `machine_group`, `price_class`, `tier_min` / `tier_max`, `effective_start_date`, and `effective_end_date`.
- **Price_Status**: The computed lifecycle state of a Package_Price, one of **Berlaku** (in effect today), **Dijadwalkan** (starts in the future), or **Berakhir** (ended on or before today).
- **Pending_Approval_State**: The condition where a Vendor or child entity has an in-flight maker-checker change request, surfaced by the existing `usePendingEntityIds` set and the `PendingApprovalBadge`.
- **Staged_Response**: The `202 Accepted` `ChangeRequestAccepted` result returned by every vendor create / update / disable mutation, indicating the change was submitted for approval and is not yet applied.
- **Admin_Operator**: An authenticated internal user holding the `ADMIN` or `ADMIN_PARAM` role, authorized to view and stage changes to admin vendor data.
- **Internal_Design_System**: The "Merah Sirih" token set and shared UI components (`PageHeader`, `Badge`, `Button`, `DataTable`, `ConfirmActionDialog`) documented in the `ui_design` steering doc.

## Requirements

### Requirement 1: Vendor identity header

**User Story:** As an Admin_Operator, I want the vendor detail page to lead with a clear identity header, so that I immediately know which vendor I am looking at and its overall status.

#### Acceptance Criteria

1. WHEN the Vendor_Detail_Page renders with a loaded Vendor, THE Vendor_Header SHALL display the Vendor code and name together as the page title and the Vendor legal name in the subtitle.
2. WHEN the Vendor_Detail_Page renders with a loaded Vendor, THE Vendor_Header SHALL display an eyebrow label identifying the section as vendor management.
3. WHEN the Vendor_Detail_Page renders with a loaded Vendor, THE Vendor_Header SHALL render the Vendor active status as a badge paired with both an icon and a text label, using the success variant when the Vendor is active and the danger variant when the Vendor is inactive.
4. WHEN the Vendor_Detail_Page renders, THE Vendor_Header SHALL provide a "Kembali ke daftar" navigation control that returns to the vendor list route.
5. WHILE the Vendor record is still loading, THE Vendor_Header SHALL render a non-error placeholder title rather than empty or broken content.
6. IF the Vendor record fails to load, THEN THE Vendor_Detail_Page SHALL display an error message indicating the vendor was not found.

### Requirement 2: Summary strip metrics

**User Story:** As an Admin_Operator, I want a quick-glance summary of the vendor's branches, PICs, packages, and price range, so that I can gauge the vendor's footprint without opening each tab.

#### Acceptance Criteria

1. WHEN the Summary_Strip renders, THE Summary_Strip SHALL display four metrics: active-branch count, vendor-wide PIC count, in-effect package-price count, and active price range.
2. WHEN computing the active-branch metric, THE Summary_Strip SHALL show the number of active branches out of the total branch count for the Vendor.
3. WHEN computing the vendor-wide PIC metric, THE Summary_Strip SHALL count only PICs whose branch scope is vendor-wide.
4. WHEN computing the in-effect package metric, THE Summary_Strip SHALL count Package_Prices whose Price_Status is Berlaku.
5. WHEN computing the active price range, THE Summary_Strip SHALL derive the minimum and maximum `base_price` among Package_Prices whose Price_Status is Berlaku, formatting each as an IDR amount.
6. WHERE a metric's underlying data has not yet loaded, THE Summary_Strip SHALL render a neutral placeholder for that metric rather than a misleading zero.
7. WHERE there are no in-effect Package_Prices, THE Summary_Strip SHALL render a neutral placeholder for the active price range rather than a computed range.
8. WHEN deriving the active price range, THE Summary_Strip SHALL only use Package_Prices whose `base_price` is present, ignoring rows with a null `base_price`.

### Requirement 3: Tab navigation

**User Story:** As an Admin_Operator, I want the four vendor tabs preserved and styled per the design, so that navigation is familiar and accessible.

#### Acceptance Criteria

1. WHEN the Vendor_Detail_Page renders, THE Vendor_Detail_Page SHALL present exactly four tabs in this order: Info, Cabang, PIC Vendor-wide, Harga Paket.
2. WHEN a tab is selected, THE Vendor_Detail_Page SHALL render only that tab's panel and mark that tab as the active tab.
3. WHEN the tab strip renders, THE Vendor_Detail_Page SHALL expose the tabs with accessible tab semantics including an indication of which tab is selected.
4. WHEN a tab is focused via the keyboard, THE Vendor_Detail_Page SHALL render a visible focus indicator on the focused tab.
5. WHEN the Vendor_Detail_Page first opens, THE Vendor_Detail_Page SHALL default to a defined initial tab.

### Requirement 4: Info tab presentation

**User Story:** As an Admin_Operator, I want the Info tab to show the vendor profile and registered address clearly, so that I can review the vendor's legal and contact data at a glance.

#### Acceptance Criteria

1. WHEN the Info_Tab renders with a loaded Vendor, THE Info_Tab SHALL display the Vendor code, name, legal name, NPWP, contact email, contact phone, and active status.
2. WHEN the Info_Tab renders the Vendor registered address, THE Info_Tab SHALL present the `hq_address` in a dedicated address area distinct from the profile fields.
3. WHERE a Vendor profile field is empty, THE Info_Tab SHALL render a placeholder marker instead of an empty value.
4. WHEN the Info_Tab renders the Vendor active status, THE Info_Tab SHALL render it as a badge paired with both an icon and a text label.
5. WHEN the Info_Tab renders, THE Info_Tab SHALL NOT expose any control that edits Vendor profile data directly on this tab, preserving the existing edit path via the vendor list.

### Requirement 5: Cabang tab presentation

**User Story:** As an Admin_Operator, I want the Cabang tab restyled per the design while keeping its existing behavior, so that branch navigation and management are unchanged but more readable.

#### Acceptance Criteria

1. WHEN the Branches_Panel renders, THE Branches_Panel SHALL list the vendor's branches with columns for branch code, branch name, region, category, and status.
2. WHEN the Branches_Panel renders a branch code, THE Branches_Panel SHALL render it as a link that navigates to that branch's detail page.
3. WHEN the Branches_Panel renders a branch status, THE Branches_Panel SHALL render it as a badge paired with both an icon and a text label.
4. WHEN a branch has a Pending_Approval_State, THE Branches_Panel SHALL render the pending-approval indicator alongside its status.
5. WHEN the Branches_Panel renders branch actions, THE Branches_Panel SHALL preserve the existing create, edit, disable, and enable controls with their existing maker-checker behavior.
6. WHERE the vendor has no branches, THE Branches_Panel SHALL display an empty-state message.

### Requirement 6: PIC Vendor-wide tab presentation

**User Story:** As an Admin_Operator, I want the PIC Vendor-wide tab restyled per the design while keeping its existing behavior, so that vendor-wide contacts are easy to scan and still manageable.

#### Acceptance Criteria

1. WHEN the Pics_Panel renders on the PIC Vendor-wide tab, THE Pics_Panel SHALL list only PICs whose branch scope is vendor-wide.
2. WHEN the Pics_Panel renders a PIC, THE Pics_Panel SHALL display the PIC name, role or position, email, and phone.
3. WHERE a PIC is a designated notification recipient, THE Pics_Panel SHALL visually distinguish that PIC.
4. WHEN the Pics_Panel renders PIC actions, THE Pics_Panel SHALL preserve the existing create, edit, disable, and enable controls with their existing maker-checker behavior.
5. WHERE there are no vendor-wide PICs, THE Pics_Panel SHALL display an empty-state message.
6. WHERE the PIC list carries a non-blocking warning, THE Pics_Panel SHALL surface that warning.

### Requirement 7: Harga Paket filtering and count

**User Story:** As an Admin_Operator, I want to filter the package-price list by package, machine, and status and see how many tiers match, so that I can focus on the relevant prices in a long list.

#### Acceptance Criteria

1. WHEN the Package_Prices_Panel renders, THE Package_Prices_Panel SHALL provide filter controls for package code, machine group, and Price_Status.
2. WHEN a filter value is selected, THE Package_Prices_Panel SHALL show only the Package_Prices matching all currently selected filters.
3. WHEN filters are set to their "all" values, THE Package_Prices_Panel SHALL show every Package_Price for the Vendor.
4. WHEN the filtered result changes, THE Package_Prices_Panel SHALL display the count of matching tiers.
5. WHEN the package-code and machine-group filter options are built, THE Package_Prices_Panel SHALL derive them from the Vendor's own Package_Prices so that only present values are offered.
6. WHERE no Package_Price matches the current filters, THE Package_Prices_Panel SHALL display a "no tiers match the filter" message distinct from the "vendor has no package prices" empty state.

### Requirement 8: Harga Paket row presentation

**User Story:** As an Admin_Operator, I want each package-price row to show its code, grain, tier, price, period, and status clearly, so that I can read the price sheet accurately.

#### Acceptance Criteria

1. WHEN the Package_Prices_Panel renders a Package_Price row, THE Package_Prices_Panel SHALL display the package code, machine group, price class, override level, tier range, base price, effective period, and Price_Status.
2. WHEN the Package_Prices_Panel renders the base price, THE Package_Prices_Panel SHALL render it right-aligned using tabular figures and SHALL display the money value as its exact decimal string without performing arithmetic on it in the client.
3. WHERE a Package_Price `base_price` is null, THE Package_Prices_Panel SHALL render a placeholder marker instead of a computed or empty amount.
4. WHEN the Package_Prices_Panel renders a tier range, THE Package_Prices_Panel SHALL render an open-ended upper bound distinctly from a closed range.
5. WHEN the Package_Prices_Panel renders the effective period, THE Package_Prices_Panel SHALL render the start date and the end date, rendering an open-ended end date distinctly from a fixed end date.
6. WHEN the Package_Prices_Panel renders the override level, THE Package_Prices_Panel SHALL distinguish a base (PT-level) row, a branch-scoped row, and an ATM-scoped row.

### Requirement 9: Price status derivation

**User Story:** As an Admin_Operator, I want each package price's status computed consistently, so that "Berlaku", "Dijadwalkan", and "Berakhir" always mean the same thing across the header, filters, and table.

#### Acceptance Criteria

1. WHEN a Package_Price has an `effective_start_date` after the current date, THE CMS SHALL classify its Price_Status as Dijadwalkan.
2. WHEN a Package_Price has an `effective_end_date` on or before the current date, THE CMS SHALL classify its Price_Status as Berakhir.
3. WHEN a Package_Price has an `effective_start_date` on or before the current date and either no `effective_end_date` or an `effective_end_date` after the current date, THE CMS SHALL classify its Price_Status as Berlaku.
4. WHEN the CMS renders a Price_Status, THE CMS SHALL render it as a badge paired with both an icon and a text label, never signalling status by color alone.
5. WHEN Price_Status is used by the Summary_Strip, the status filter, and the row badge, THE CMS SHALL derive it from a single shared computation.

### Requirement 10: Harga Paket actions and maker-checker

**User Story:** As an Admin_Operator, I want to add, edit, and end package prices from this tab, so that I can maintain the price sheet — while the system still enforces two-person approval.

#### Acceptance Criteria

1. WHEN the Package_Prices_Panel renders, THE Package_Prices_Panel SHALL provide an "Tambah Harga Paket" control that opens the existing package-price create form.
2. WHEN a Package_Price has Price_Status Berlaku or Dijadwalkan, THE Package_Prices_Panel SHALL provide an "Ubah" action opening the existing edit form for that row.
3. WHEN a Package_Price has Price_Status Berlaku, THE Package_Prices_Panel SHALL provide an "Akhiri" action that opens a confirmation before ending the price period.
4. WHEN a Package_Price has Price_Status Berakhir, THE Package_Prices_Panel SHALL NOT offer Ubah or Akhiri and SHALL indicate the row is ended.
5. WHEN a create, edit, or end mutation returns a Staged_Response, THE Package_Prices_Panel SHALL present the change as submitted for approval and SHALL NOT present it as already applied.
6. WHEN a Package_Price has a Pending_Approval_State, THE Package_Prices_Panel SHALL render the pending-approval indicator and SHALL disable that row's Ubah and Akhiri actions.
7. IF a create, edit, or end mutation fails, THEN THE Package_Prices_Panel SHALL surface the error to the Admin_Operator.
8. WHEN the Akhiri confirmation is shown, THE Package_Prices_Panel SHALL state that ending the period makes it history that cannot be re-activated and that a new row must be created for a later period.

### Requirement 11: Visual system conformance

**User Story:** As an Admin_Operator, I want the revamped page to follow the internal design system, so that it is consistent with the rest of the CMS and accessible.

#### Acceptance Criteria

1. WHEN the Vendor_Detail_Page and its panels render, THE CMS SHALL use the Internal_Design_System tokens for color, spacing, radius, and typography rather than hard-coded one-off values.
2. WHEN the page renders any monetary amount, tier count, or metric figure, THE CMS SHALL render it using tabular figures.
3. WHEN the page renders any status, THE CMS SHALL pair color with an icon and a text label so status is never conveyed by color alone.
4. WHEN the page uses the brand red, THE CMS SHALL reserve it for primary actions, active tab indication, and focus, keeping error and destructive signalling on the danger role.
5. WHEN a destructive action (Akhiri / Nonaktifkan) is offered, THE CMS SHALL render it using the danger role rather than the brand primary.
6. WHEN interactive controls receive keyboard focus, THE CMS SHALL render a visible focus indicator.

### Requirement 12: Behavior and authorization preserved

**User Story:** As the CMS, I want the revamp to change only presentation, so that existing data access, authorization, and approval guarantees are not weakened.

#### Acceptance Criteria

1. WHEN the revamped page reads or mutates vendor data, THE CMS SHALL use the existing admin vendor endpoints and existing hooks without adding new endpoints or changing request or response contracts.
2. WHEN the revamped page stages any change, THE CMS SHALL route it through the existing maker-checker mutation path unchanged.
3. WHEN the revamped page renders, THE CMS SHALL remain within the existing ADMIN / ADMIN_PARAM authorization scope of the route, adding no client-side capability that bypasses server authorization.
4. WHEN the revamp is complete, THE CMS SHALL preserve the existing per-branch detail navigation from the Cabang tab unchanged.
5. WHEN the revamp renders money values, THE CMS SHALL treat `base_price` strictly as an opaque decimal string for display, never converting it to a floating-point number.
