# Requirements Document

## Introduction

This feature adds a read-only view of the ATMs managed by a single vendor branch, surfaced as a new sub-tab on the existing vendor branch detail page in the internal CompanyPortal (route `/settings/admin/vendors/:vendorId/branches/:branchId`). Operators managing vendor master data can already see a branch's Vault, PIC, Paket, and Harga Paket in sub-tabs; this feature adds an "ATM" sub-tab listing every ATM assigned to that branch, with its identifying and status attributes, so operators can confirm which machines fall under a vendor branch's scope without leaving the page.

The relationship between a vendor branch and the ATMs it manages is not a direct foreign key. It runs through the branch's packages and the per-ATM package assignment:

```
vendor_branches (id)
  └─ vendor_packages (vendor_branch_id)
       └─ atm_vendor_packages (vendor_package_id, atm_id, is_active, effective_start_date, effective_end_date)
            └─ atms (id, terminal_id, location_id, priority_class, is_active, ...)
                 └─ locations (id, name, city_or_regency, province, ...)
```

An ATM is "managed by" a vendor branch when an `atm_vendor_packages` row links that ATM to a `vendor_packages` row belonging to the branch. This is a display-only feature: no create, update, delete, or maker-checker approval is involved. All reads route to the read replica per DB topology rules. The scope, columns, and whether to include historically-ended assignments are open decisions flagged below for confirmation.

## Glossary

- **CMS**: The Cash Management System (internal CompanyPortal application).
- **Vendor_Branch**: A `vendor_branches` row — a regional sub-unit of a vendor, identified by `branch_code` and `branch_name`, scoped to one `vendor_id`.
- **Managed_ATM**: An `atms` row linked to a Vendor_Branch through an `atm_vendor_packages` assignment whose `vendor_package_id` belongs to a `vendor_packages` row of that Vendor_Branch.
- **ATM_Assignment**: An `atm_vendor_packages` row linking one ATM to one vendor package, carrying `is_active`, `effective_start_date`, and `effective_end_date`.
- **Branch_ATM_List_Endpoint**: The backend HTTP endpoint that returns the paginated list of Managed_ATMs for a given Vendor_Branch, under `/api/v1/admin/vendors/{vendorId}/branches/{branchId}/atms`.
- **Branch_ATM_Service**: The backend service layer that resolves Managed_ATMs for a Vendor_Branch and enforces authorization.
- **ATM_Panel**: The frontend sub-tab component on the vendor branch detail page that renders the Managed_ATM list.
- **Admin_Operator**: An authenticated internal user holding the `ADMIN` or `ADMIN_PARAM` role, authorized to view admin vendor data.
- **Priority_Class**: An ATM's priority tier (`VIP`, `Non VIP`, `Industri`) stored on `atms.priority_class`.
- **Active_Status**: Whether an entity is currently active (`is_active = true` and, where applicable, `deleted_at IS NULL`).

## Requirements

### Requirement 1: Retrieve ATMs managed by a vendor branch

**User Story:** As an Admin_Operator, I want to retrieve the list of ATMs managed by a specific vendor branch, so that I can see which machines fall under that branch's scope.

#### Acceptance Criteria

1. WHEN a request is received for the Managed_ATMs of a Vendor_Branch, THE Branch_ATM_Service SHALL return the set of Managed_ATMs whose ATM_Assignment references a vendor package belonging to that Vendor_Branch.
2. WHEN resolving Managed_ATMs for a Vendor_Branch, THE Branch_ATM_Service SHALL include each ATM at most once even WHERE the ATM has more than one ATM_Assignment to packages of the same Vendor_Branch.
3. WHEN the Branch_ATM_Service reads Managed_ATM data, THE Branch_ATM_Service SHALL execute the read against the read replica connection.
4. IF the specified Vendor_Branch does not exist, THEN THE Branch_ATM_List_Endpoint SHALL return an HTTP 404 status with a descriptive error message.
5. IF the specified Vendor_Branch exists but belongs to a vendor other than the one named in the request path, THEN THE Branch_ATM_List_Endpoint SHALL return an HTTP 404 status with a descriptive error message.
6. IF the specified Vendor_Branch has no Managed_ATMs, THEN THE Branch_ATM_List_Endpoint SHALL return an HTTP 200 status with an empty ATM list and a total count of zero.

### Requirement 2: Managed ATM attributes returned

**User Story:** As an Admin_Operator, I want each managed ATM to include its identifying and status attributes, so that I can recognize and assess each machine at a glance.

#### Acceptance Criteria

1. WHEN the Branch_ATM_List_Endpoint returns a Managed_ATM, THE Branch_ATM_List_Endpoint SHALL include the ATM identifier and the ATM terminal identifier.
2. WHEN the Branch_ATM_List_Endpoint returns a Managed_ATM, THE Branch_ATM_List_Endpoint SHALL include the ATM location name and the location city or regency.
3. WHEN the Branch_ATM_List_Endpoint returns a Managed_ATM, THE Branch_ATM_List_Endpoint SHALL include the ATM Priority_Class.
4. WHEN the Branch_ATM_List_Endpoint returns a Managed_ATM, THE Branch_ATM_List_Endpoint SHALL include the ATM Active_Status.
5. WHEN the Branch_ATM_List_Endpoint returns a Managed_ATM, THE Branch_ATM_List_Endpoint SHALL include the code of the vendor package through which the ATM is assigned to the Vendor_Branch.
6. WHERE a Managed_ATM has no associated location record, THE Branch_ATM_List_Endpoint SHALL return a null location name and a null location city or regency for that ATM.

### Requirement 3: Authorization

**User Story:** As the CMS, I want the managed-ATM list restricted to authorized admin roles, so that vendor master data is only visible to permitted users.

#### Acceptance Criteria

1. IF a request to the Branch_ATM_List_Endpoint carries no valid authentication token, THEN THE Branch_ATM_List_Endpoint SHALL return an HTTP 401 status.
2. IF a request to the Branch_ATM_List_Endpoint carries a valid authentication token whose role is neither `ADMIN` nor `ADMIN_PARAM`, THEN THE Branch_ATM_List_Endpoint SHALL return an HTTP 403 status.
3. WHEN a request to the Branch_ATM_List_Endpoint carries a valid authentication token whose role is `ADMIN` or `ADMIN_PARAM`, THE Branch_ATM_List_Endpoint SHALL process the request.

### Requirement 4: Pagination and result ordering

**User Story:** As an Admin_Operator, I want the managed-ATM list paginated and consistently ordered, so that branches with many ATMs remain responsive and readable.

#### Acceptance Criteria

1. WHEN the Branch_ATM_List_Endpoint returns Managed_ATMs, THE Branch_ATM_List_Endpoint SHALL return the results ordered by ATM terminal identifier in ascending order.
2. WHEN a request specifies a page number and page size, THE Branch_ATM_List_Endpoint SHALL return only the Managed_ATMs belonging to the requested page.
3. WHEN the Branch_ATM_List_Endpoint returns a page of Managed_ATMs, THE Branch_ATM_List_Endpoint SHALL include the total count of Managed_ATMs for the Vendor_Branch across all pages.
4. IF a request omits the page number, THEN THE Branch_ATM_List_Endpoint SHALL default the page number to 1.
5. IF a request omits the page size, THEN THE Branch_ATM_List_Endpoint SHALL default the page size to a fixed value defined by the endpoint.
6. IF a request specifies a page size greater than the endpoint maximum, THEN THE Branch_ATM_List_Endpoint SHALL clamp the page size to the endpoint maximum.

### Requirement 5: ATM sub-tab on the branch detail page

**User Story:** As an Admin_Operator, I want an "ATM" sub-tab on the vendor branch detail page, so that I can view the branch's managed ATMs alongside its other data.

#### Acceptance Criteria

1. WHEN the vendor branch detail page is displayed, THE ATM_Panel SHALL be reachable as a sub-tab labeled "ATM" alongside the existing Vault, PIC, Paket, and Harga Paket sub-tabs.
2. WHEN the ATM sub-tab is selected, THE ATM_Panel SHALL request the Managed_ATMs for the current Vendor_Branch from the Branch_ATM_List_Endpoint.
3. WHILE the Managed_ATM request is in progress, THE ATM_Panel SHALL display a loading indicator.
4. WHEN the Managed_ATM request succeeds with one or more ATMs, THE ATM_Panel SHALL display the ATMs in a table with columns for terminal identifier, location, Priority_Class, package code, and Active_Status.
5. WHEN the Managed_ATM request succeeds with zero ATMs, THE ATM_Panel SHALL display an empty-state message indicating the branch has no managed ATMs.
6. IF the Managed_ATM request fails, THEN THE ATM_Panel SHALL display an error message.

### Requirement 6: ATM sub-tab presentation

**User Story:** As an Admin_Operator, I want the managed-ATM table to follow the internal design system, so that it is readable and consistent with other master-data screens.

#### Acceptance Criteria

1. WHEN the ATM_Panel renders the Managed_ATM table, THE ATM_Panel SHALL render each ATM's Active_Status as a badge paired with both an icon and a text label.
2. WHEN the ATM_Panel renders a Managed_ATM's Active_Status, THE ATM_Panel SHALL render an active ATM with the success badge variant and an inactive ATM with the danger badge variant.
3. WHEN the ATM_Panel renders the terminal identifier column, THE ATM_Panel SHALL render the terminal identifier values using tabular figures.
4. WHERE a Managed_ATM has a null location name, THE ATM_Panel SHALL render a placeholder marker in the location column instead of an empty cell.
