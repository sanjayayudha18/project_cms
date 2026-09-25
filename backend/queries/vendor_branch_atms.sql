-- Read-only "ATM" sub-tab on the vendor branch detail page (.kiro/specs/
-- vendor-branch-atms). An ATM is "managed by" a branch when it has an
-- active atm_vendor_packages assignment to a vendor_packages_branch row
-- owned by that branch. No mutations here -- display-only, no
-- maker-checker. GetVendorBranchVendorID already exists in
-- vendor_vaults_admin.sql and is reused for the vendor-scope check.

-- name: ListBranchATMs :many
-- One row per ATM managed by branch $1 (active assignments only, i.e. the
-- assignment window covers today). DISTINCT ON (a.id) dedupes an ATM that
-- has >1 active assignment to this branch's packages (Req 1.2); the inner
-- ORDER BY picks a deterministic package code (latest-starting assignment).
-- Postgres requires the DISTINCT ON expression to lead its own ORDER BY, so
-- the dedup runs in a subquery and the outer query re-sorts by terminal_id
-- (Req 4.1) before paginating.
SELECT atm_id, terminal_id, priority_class, is_active,
       location_name, location_city_or_regency, package_code
FROM (
    SELECT DISTINCT ON (a.id)
        a.id AS atm_id,
        a.terminal_id AS terminal_id,
        a.priority_class AS priority_class,
        a.is_active AS is_active,
        l.name AS location_name,
        l.city_or_regency AS location_city_or_regency,
        vp.package_code AS package_code
    FROM atm_vendor_packages avp
    JOIN vendor_packages_branch vp ON vp.id = avp.vendor_package_id
    JOIN atms a                   ON a.id = avp.atm_id
    LEFT JOIN locations l         ON l.id = a.location_id
    WHERE vp.vendor_branch_id = sqlc.arg('vendor_branch_id')
      AND avp.is_active = true
      AND avp.effective_start_date <= CURRENT_DATE
      AND (avp.effective_end_date IS NULL OR avp.effective_end_date >= CURRENT_DATE)
    ORDER BY a.id, avp.effective_start_date DESC, avp.id DESC
) dedup
ORDER BY terminal_id ASC
LIMIT sqlc.arg('page_limit')::bigint OFFSET sqlc.arg('page_offset')::bigint;

-- name: CountBranchATMs :one
-- Distinct ATM count for the same branch + active-assignment filter -- the
-- pagination total (Req 4.3). COUNT(DISTINCT a.id) matches ListBranchATMs's
-- dedup so total never exceeds the number of returned rows across all pages.
SELECT COUNT(DISTINCT a.id)
FROM atm_vendor_packages avp
JOIN vendor_packages_branch vp ON vp.id = avp.vendor_package_id
JOIN atms a                    ON a.id = avp.atm_id
WHERE vp.vendor_branch_id = sqlc.arg('vendor_branch_id')
  AND avp.is_active = true
  AND avp.effective_start_date <= CURRENT_DATE
  AND (avp.effective_end_date IS NULL OR avp.effective_end_date >= CURRENT_DATE);
