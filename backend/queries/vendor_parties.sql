-- Phase 2.2b (.claude/sdlc/cit-send-vendor): pihak vendor per Vendor Request + keputusan terima/tolak.

-- ---------------------------------------------------------------------------
-- Isi pihak (FR1, FR3). Satu baris per (ATM, denom) dari penetapan vault 2.2a.
-- Kolom di-whitelist: tidak ada saldo/kapasitas, tier, urgent, harga (FR3.3).
-- ---------------------------------------------------------------------------

-- name: ListRequestPartyRows :many
SELECT asg.terminal_id,
       COALESCE(max(i.lokasi_atm), '')::text AS lokasi_atm,
       t.ticket_number,
       i.denom,
       sum(i.amount_replenish)::bigint AS amount_replenish,
       asg.replenish_branch_id, rb.branch_code AS replenish_branch_code, rb.branch_name AS replenish_branch_name,
       rv.id AS replenish_vendor_id, rv.name AS replenish_vendor_name,
       asg.vault_branch_id, vb.branch_code AS vault_branch_code, vb.branch_name AS vault_branch_name,
       vv.id AS vault_vendor_id, vv.name AS vault_vendor_name,
       concat_ws(', ', NULLIF(l.address_line1, ''), NULLIF(l.address_line2, ''), NULLIF(l.city_or_regency, ''),
                 NULLIF(l.province, ''))::text AS vault_address
FROM vendor_request_vault_assignments asg
JOIN vendor_requests vr ON vr.id = asg.vendor_request_id
JOIN vendor_request_items i ON i.vendor_request_id = asg.vendor_request_id AND i.terminal_id = asg.terminal_id
JOIN vendor_branches rb ON rb.id = asg.replenish_branch_id
JOIN vendors rv ON rv.id = rb.vendor_id
JOIN vendor_branches vb ON vb.id = asg.vault_branch_id
JOIN vendors vv ON vv.id = vb.vendor_id
LEFT JOIN locations l ON l.id = vb.location_id
LEFT JOIN vendor_request_tickets t
       ON t.request_number = vr.request_number AND t.terminal_id = asg.terminal_id AND t.is_active
WHERE asg.vendor_request_id = $1
GROUP BY asg.terminal_id, t.ticket_number, i.denom, asg.replenish_branch_id, rb.branch_code, rb.branch_name, rv.id, rv.name,
         asg.vault_branch_id, vb.branch_code, vb.branch_name, vv.id, vv.name, l.address_line1, l.address_line2,
         l.city_or_regency, l.province
ORDER BY asg.terminal_id, i.denom;

-- ---------------------------------------------------------------------------
-- Pihak: tulis (primary, dalam tx pemicu)
-- ---------------------------------------------------------------------------

-- name: LockVendorPartiesForRequest :many
-- Dipanggil setelah GetVendorRequestForUpdate (urutan kunci: request lalu pihak, NFR3).
SELECT * FROM vendor_request_vendor_parties
WHERE vendor_request_id = $1
ORDER BY id
FOR UPDATE;

-- name: LockVendorParty :one
SELECT * FROM vendor_request_vendor_parties WHERE id = $1 FOR UPDATE;

-- name: InsertVendorParty :one
INSERT INTO vendor_request_vendor_parties (vendor_request_id, vendor_branch_id, role, content)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: ResendVendorParty :one
-- Kirim ulang (FR2.2/FR2.3): kembali pending dengan isi baru; keputusan lama tetap di events.
UPDATE vendor_request_vendor_parties
SET status = 'pending', content = sqlc.arg('content')::jsonb, sent_at = now(),
    decided_by = NULL, decided_at = NULL, rejection_reason = NULL
WHERE id = sqlc.arg('id')::bigint
RETURNING *;

-- name: WithdrawVendorParty :one
UPDATE vendor_request_vendor_parties
SET status = 'withdrawn', decided_by = NULL, decided_at = NULL, rejection_reason = NULL
WHERE id = $1
RETURNING *;

-- name: DecideVendorParty :one
-- FR4: pending -> accepted | rejected. Guard status di WHERE; tidak ada baris = sudah diputuskan.
UPDATE vendor_request_vendor_parties
SET status = sqlc.arg('status')::text, decided_by = sqlc.arg('decided_by')::bigint, decided_at = now(),
    rejection_reason = sqlc.narg('rejection_reason')::text
WHERE id = sqlc.arg('id')::bigint AND status = 'pending'
RETURNING *;

-- name: ReturnVaultPlansForVaultBranch :many
-- FR4.3: branch vault menolak -> hanya plan area yang memuat ATM dengan vault itu kembali draft; alasan vendor
-- tercatat sebagai penolakan plan (rejected_by = user vendor, dipakai UI untuk label "Ditolak vendor").
UPDATE vendor_request_vault_plans p
SET status = 'draft', approved_by = NULL, approved_at = NULL,
    rejected_by = sqlc.arg('actor_id')::bigint, rejected_at = now(), rejection_reason = sqlc.arg('reason')::text
WHERE p.vendor_request_id = sqlc.arg('request_id')::bigint AND p.status <> 'cancelled'
  AND p.id IN (SELECT a.vault_plan_id FROM vendor_request_vault_assignments a
               WHERE a.vendor_request_id = sqlc.arg('request_id')::bigint AND a.vault_branch_id = sqlc.arg('vault_branch_id')::bigint)
RETURNING p.id, p.acm_area_id;

-- name: InsertVendorPartyEvent :exec
INSERT INTO vendor_request_vendor_party_events (party_id, event, actor_id, reason, content)
VALUES ($1, $2, $3, $4, $5);

-- ---------------------------------------------------------------------------
-- Internal (CompanyPortal, FR6.6)
-- ---------------------------------------------------------------------------

-- name: ListVendorPartiesForRequest :many
SELECT p.id, p.vendor_branch_id, p.role, p.status, p.sent_at, p.decided_at, p.rejection_reason,
       b.branch_code, b.branch_name, v.id AS vendor_id, v.name AS vendor_name, u.full_name AS decided_by_name
FROM vendor_request_vendor_parties p
JOIN vendor_branches b ON b.id = p.vendor_branch_id
JOIN vendors v ON v.id = b.vendor_id
LEFT JOIN users u ON u.id = p.decided_by
WHERE p.vendor_request_id = $1
ORDER BY p.role, b.branch_name, p.id; -- replenish before vault

-- name: ListVendorPartyEventsForRequest :many
SELECT e.id, e.party_id, e.event, e.reason, e.created_at, u.full_name AS actor_name
FROM vendor_request_vendor_party_events e
JOIN vendor_request_vendor_parties p ON p.id = e.party_id
JOIN users u ON u.id = e.actor_id
WHERE p.vendor_request_id = $1
ORDER BY e.created_at, e.id;

-- ---------------------------------------------------------------------------
-- VendorPortal (FR5): cakupan = vendor dari claim + users.vendor_branch_id (NULL = semua branch vendor).
-- ---------------------------------------------------------------------------

-- name: GetUserVendorScope :one
-- FR5.2: dibaca dari primary per request, bukan dari JWT.
SELECT vendor_id, vendor_branch_id FROM users
WHERE id = $1 AND is_active AND deleted_at IS NULL;

-- name: ListVendorOrders :many
SELECT p.id, p.role, p.status, p.content, p.sent_at, p.decided_at,
       p.vendor_branch_id, b.branch_code, b.branch_name,
       vr.request_number, vr.replenish_date, vr.status AS request_status, vr.is_canceled
FROM vendor_request_vendor_parties p
JOIN vendor_branches b ON b.id = p.vendor_branch_id
JOIN vendor_requests vr ON vr.id = p.vendor_request_id
WHERE b.vendor_id = sqlc.arg('vendor_id')::bigint
  AND (sqlc.narg('vendor_branch_id')::bigint IS NULL OR p.vendor_branch_id = sqlc.narg('vendor_branch_id')::bigint)
  AND (sqlc.arg('party_status')::text = '' OR p.status = sqlc.arg('party_status')::text)
  AND (sqlc.arg('request_status')::text = '' OR vr.status = sqlc.arg('request_status')::text)
  AND (sqlc.narg('from_date')::date IS NULL OR vr.replenish_date >= sqlc.narg('from_date')::date)
  AND (sqlc.narg('to_date')::date IS NULL OR vr.replenish_date <= sqlc.narg('to_date')::date)
ORDER BY vr.replenish_date DESC NULLS LAST, p.id DESC
LIMIT sqlc.arg('page_limit')::int OFFSET sqlc.arg('page_offset')::int;

-- name: CountVendorOrders :one
SELECT count(*)
FROM vendor_request_vendor_parties p
JOIN vendor_branches b ON b.id = p.vendor_branch_id
JOIN vendor_requests vr ON vr.id = p.vendor_request_id
WHERE b.vendor_id = sqlc.arg('vendor_id')::bigint
  AND (sqlc.narg('vendor_branch_id')::bigint IS NULL OR p.vendor_branch_id = sqlc.narg('vendor_branch_id')::bigint)
  AND (sqlc.arg('party_status')::text = '' OR p.status = sqlc.arg('party_status')::text)
  AND (sqlc.arg('request_status')::text = '' OR vr.status = sqlc.arg('request_status')::text)
  AND (sqlc.narg('from_date')::date IS NULL OR vr.replenish_date >= sqlc.narg('from_date')::date)
  AND (sqlc.narg('to_date')::date IS NULL OR vr.replenish_date <= sqlc.narg('to_date')::date);

-- name: GetVendorOrder :one
-- Satu pihak dalam cakupan; di luar cakupan = no rows -> 404 (FR5.3).
SELECT p.id, p.vendor_request_id, p.role, p.status, p.content, p.sent_at, p.decided_at, p.rejection_reason,
       p.vendor_branch_id, b.branch_code, b.branch_name,
       vr.request_number, vr.replenish_date, vr.status AS request_status, vr.is_canceled
FROM vendor_request_vendor_parties p
JOIN vendor_branches b ON b.id = p.vendor_branch_id
JOIN vendor_requests vr ON vr.id = p.vendor_request_id
WHERE p.id = sqlc.arg('id')::bigint
  AND b.vendor_id = sqlc.arg('vendor_id')::bigint
  AND (sqlc.narg('vendor_branch_id')::bigint IS NULL OR p.vendor_branch_id = sqlc.narg('vendor_branch_id')::bigint);
