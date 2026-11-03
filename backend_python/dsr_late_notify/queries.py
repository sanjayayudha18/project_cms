"""SQL for the 09:00 DSR-late notification (.claude/sdlc/dsr-late-report spec FR5, plan D2/D3).

UNSENT_BRANCHES copies the counted_vaults / obliged / hits / first_hit CTEs of
backend/queries/dsr_late_report.sql verbatim, with `days` narrowed to the one report date
($1) and as_of = $2. Change both sides together and keep the shared fixture
backend/internal/repository/testdata/dsr_late_cases.json green on both
(Go: dsr_late_report_repository_integration_test.go, Python: test_job_integration.py).
"""

UNSENT_BRANCHES = """
WITH days AS (
    SELECT $1::date AS report_date
),
counted_vaults AS (
    SELECT dy.report_date, v.id AS vault_id, v.vendor_branch_id, b.vendor_id
    FROM days dy
    JOIN vendor_vaults v ON v.is_active
                        AND (v.created_at AT TIME ZONE 'Asia/Jakarta')::date <= dy.report_date
    JOIN vendor_branches b ON b.id = v.vendor_branch_id
    JOIN vendors vd ON vd.id = b.vendor_id AND vd.kind = 'FLM_VENDOR'
    WHERE EXISTS (SELECT 1 FROM dsr_location_vault_maps m
                  WHERE m.is_active AND m.vendor_id = b.vendor_id AND m.vendor_vault_id = v.id)
),
obliged AS (
    SELECT DISTINCT cv.report_date, cv.vendor_branch_id
    FROM counted_vaults cv
    JOIN vendor_branches b ON b.id = cv.vendor_branch_id
    WHERE (b.created_at AT TIME ZONE 'Asia/Jakarta')::date <= cv.report_date
      AND (b.is_active OR (b.deleted_at AT TIME ZONE 'Asia/Jakarta')::date > cv.report_date)
),
hits AS (
    SELECT cv.report_date, cv.vendor_branch_id, r.id AS receipt_id, r.received_at, r.checksum, l.label
    FROM dsr_receipts r
    CROSS JOIN LATERAL unnest(r.locations) AS l(label)
    JOIN dsr_location_vault_maps m ON m.is_active AND m.vendor_id = r.vendor_id
                                  AND lower(m.dsr_location) = lower(l.label)
    JOIN counted_vaults cv ON cv.vault_id = m.vendor_vault_id
                          AND cv.vendor_id = r.vendor_id
                          AND cv.report_date = r.report_date
    WHERE r.received_at <= $2::timestamptz
),
first_hit AS (
    SELECT DISTINCT ON (report_date, vendor_branch_id) report_date, vendor_branch_id, receipt_id, received_at, checksum
    FROM hits
    ORDER BY report_date, vendor_branch_id, received_at, receipt_id
)
SELECT vd.id AS vendor_id, vd.name AS vendor_name,
       b.id AS vendor_branch_id, b.branch_code, b.branch_name
FROM obliged o
LEFT JOIN first_hit f ON f.report_date = o.report_date AND f.vendor_branch_id = o.vendor_branch_id
JOIN vendor_branches b ON b.id = o.vendor_branch_id
JOIN vendors vd ON vd.id = b.vendor_id
WHERE f.receipt_id IS NULL
ORDER BY vd.name, b.branch_code, b.id
"""

# K3: same rules as backend/queries/notifications.sql ListActiveUserRecipientsByVendorBranches
# (users of the branch or vendor-wide, active) for every vendor role (spec FR5.3).
VENDOR_USERS = """
SELECT b.id AS vendor_branch_id, u.id AS user_id, u.email
FROM vendor_branches b
JOIN users u ON u.vendor_id = b.vendor_id
JOIN roles r ON r.id = u.role_id
WHERE b.id = ANY($1::bigint[])
  AND r.role = ANY($2::text[])
  AND (u.vendor_branch_id IS NULL OR u.vendor_branch_id = b.id)
  AND u.is_active = true AND u.deleted_at IS NULL
"""

# K3: same rules as ListVendorBranchNotificationPicEmails, with pic_role replenish OR vault.
VENDOR_PIC_EMAILS = """
SELECT b.id AS vendor_branch_id, p.email::text AS email
FROM vendor_branches b
JOIN vendor_pics p ON p.vendor_id = b.vendor_id
WHERE b.id = ANY($1::bigint[])
  AND p.roles && $2::text[]
  AND (p.vendor_branch_id IS NULL OR p.vendor_branch_id = b.id)
  AND p.is_notification_recipient = true
  AND p.is_active = true AND p.deleted_at IS NULL
  AND COALESCE(btrim(p.email), '') <> ''
"""

# Same rows as ListActiveUserRecipientsByRoles (spec FR5.4).
ROLE_USERS = """
SELECT u.id AS user_id, u.email
FROM users u JOIN roles r ON r.id = u.role_id
WHERE r.role = ANY($1::text[]) AND u.is_active = true AND u.deleted_at IS NULL
ORDER BY u.id
"""
