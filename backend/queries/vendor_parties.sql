-- Phase 2.2b (.claude/sdlc/cit-send-vendor): pihak vendor per Vendor Request + keputusan terima/tolak.

-- ---------------------------------------------------------------------------
-- Isi pihak (cit-send-vendor FR1/FR3, revised by cit-vault-select FR7).
-- Kolom di-whitelist: tidak ada saldo/kapasitas, tier, urgent, alasan buffer, harga (FR7.4).
-- ---------------------------------------------------------------------------

-- name: ListRequestPartyAtmRows :many
-- One row per (ATM, denom) with the replenish branch = active kelolaan on the replenish date.
SELECT i.terminal_id,
       COALESCE(max(i.lokasi_atm), '')::text AS lokasi_atm,
       t.ticket_number,
       i.denom,
       sum(i.amount_replenish)::bigint AS amount_replenish,
       rb.id AS replenish_branch_id, rb.branch_code AS replenish_branch_code, rb.branch_name AS replenish_branch_name,
       rv.id AS replenish_vendor_id, rv.name AS replenish_vendor_name
FROM vendor_requests vr
JOIN vendor_request_items i ON i.vendor_request_id = vr.id
JOIN atms a ON a.terminal_id = i.terminal_id
JOIN LATERAL (
    SELECT COALESCE(pk.vendor_branch_id, avp.vendor_branch_id) AS vendor_branch_id
    FROM atm_vendor_packages avp
    LEFT JOIN vendor_packages_branch pk ON pk.id = avp.vendor_package_id
    WHERE avp.atm_id = a.id AND avp.is_active
      AND avp.effective_start_date <= vr.replenish_date
      AND (avp.effective_end_date IS NULL OR avp.effective_end_date >= vr.replenish_date)
    ORDER BY avp.effective_start_date DESC, avp.id DESC
    LIMIT 1
) k ON true
JOIN vendor_branches rb ON rb.id = k.vendor_branch_id
JOIN vendors rv ON rv.id = rb.vendor_id
LEFT JOIN vendor_request_tickets t
       ON t.request_number = vr.request_number AND t.terminal_id = i.terminal_id AND t.is_active
WHERE vr.id = $1
GROUP BY i.terminal_id, t.ticket_number, i.denom, rb.id, rb.branch_code, rb.branch_name, rv.id, rv.name
ORDER BY i.terminal_id, i.denom;

-- name: ListRequestPartyVaultRows :many
-- One row per (allocated vault, denom) of the request's live plan, with the
-- active vault ticket and the vault address (vault location, else branch location; U8).
SELECT a.vendor_vault_id, vv.vault_code, t.ticket_number,
       a.vault_branch_id, vb.branch_code AS vault_branch_code, vb.branch_name AS vault_branch_name,
       v.id AS vault_vendor_id, v.name AS vault_vendor_name,
       concat_ws(', ', NULLIF(l.address_line1, ''), NULLIF(l.address_line2, ''), NULLIF(l.city_or_regency, ''),
                 NULLIF(l.province, ''))::text AS vault_address,
       m.denom, m.amount
FROM vendor_request_vault_allocations a
JOIN vendor_request_vault_plans p ON p.id = a.vault_plan_id AND p.status <> 'cancelled'
JOIN vendor_request_vault_allocation_amounts m ON m.allocation_id = a.id
JOIN vendor_vaults vv ON vv.id = a.vendor_vault_id
JOIN vendor_branches vb ON vb.id = a.vault_branch_id
JOIN vendors v ON v.id = vb.vendor_id
LEFT JOIN locations l ON l.id = COALESCE(vv.location_id, vb.location_id)
LEFT JOIN vendor_request_vault_tickets t
       ON t.vendor_request_id = a.vendor_request_id AND t.vendor_vault_id = a.vendor_vault_id AND t.is_active
WHERE a.vendor_request_id = $1
ORDER BY vb.branch_code, vv.vault_code, m.denom;

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
-- FR4.3 / cit-vault-select FR7.5: branch vault menolak -> plan request (yang memuat vault branch itu) kembali draft; alasan vendor
-- tercatat sebagai penolakan plan (rejected_by = user vendor, dipakai UI untuk label "Ditolak vendor").
UPDATE vendor_request_vault_plans p
SET status = 'draft', approved_by = NULL, approved_at = NULL,
    rejected_by = sqlc.arg('actor_id')::bigint, rejected_at = now(), rejection_reason = sqlc.arg('reason')::text
WHERE p.vendor_request_id = sqlc.arg('request_id')::bigint AND p.status <> 'cancelled'
  AND p.id IN (SELECT a.vault_plan_id FROM vendor_request_vault_allocations a
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
-- VendorPortal (FR5): cakupan = vendor dari claim + users.vendor_branch_id (NULL = semua branch vendor)
-- + peran pihak yang diizinkan role akun (vendor-party-scope Req 3.2, vendorrole.PartyRoles).
-- ---------------------------------------------------------------------------

-- name: GetUserVendorScope :one
-- FR5.2: dibaca dari primary per request, bukan dari JWT. vendor-party-scope Req 3.8/3.9: role ikut
-- dibaca di query yang sama, supaya token dengan role lama ditolak tanpa query tambahan.
SELECT u.vendor_id, u.vendor_branch_id, r.role FROM users u
JOIN roles r ON r.id = u.role_id
WHERE u.id = $1 AND u.is_active AND u.deleted_at IS NULL;

-- name: ListVendorOrders :many
SELECT p.id, p.role, p.status, p.content, p.sent_at, p.decided_at,
       p.vendor_branch_id, b.branch_code, b.branch_name,
       vr.request_number, vr.replenish_date, vr.status AS request_status, vr.is_canceled
FROM vendor_request_vendor_parties p
JOIN vendor_branches b ON b.id = p.vendor_branch_id
JOIN vendor_requests vr ON vr.id = p.vendor_request_id
WHERE b.vendor_id = sqlc.arg('vendor_id')::bigint
  AND (sqlc.narg('vendor_branch_id')::bigint IS NULL OR p.vendor_branch_id = sqlc.narg('vendor_branch_id')::bigint)
  AND p.role = ANY(sqlc.arg('party_roles')::text[])
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
  AND p.role = ANY(sqlc.arg('party_roles')::text[])
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
  AND (sqlc.narg('vendor_branch_id')::bigint IS NULL OR p.vendor_branch_id = sqlc.narg('vendor_branch_id')::bigint)
  AND p.role = ANY(sqlc.arg('party_roles')::text[]);
