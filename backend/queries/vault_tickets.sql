-- Tiket pemenuhan vault (cit-vault-select spec v3 FR6): one ticket per
-- (request, vault), <vault_code>_<YYYYMMDD>_<NNN>, NNN global per (vault,
-- replenish date), never reused. Same pattern as replenish tickets (026).

-- name: ListVaultTicketsForRequest :many
SELECT t.id, t.vendor_vault_id, vv.vault_code, t.ticket_number, t.is_active, t.created_at, t.deactivated_at
FROM vendor_request_vault_tickets t
JOIN vendor_vaults vv ON vv.id = t.vendor_vault_id
WHERE t.vendor_request_id = $1
ORDER BY t.is_active DESC, vv.vault_code, t.id;

-- name: LockVaultTicketSeq :exec
-- Serializes NNN issue per (vault, replenish_date) until tx end. Callers issue
-- in vendor_vault_id order, so two txs never wait on each other in opposite order.
SELECT pg_advisory_xact_lock(hashtextextended(
    'vault_ticket|' || sqlc.arg('vendor_vault_id')::bigint::text || '|' || sqlc.arg('replenish_date')::date::text, 0));

-- name: IssueVaultTicket :one
-- Next NNN for (vault, replenish_date) across all requests, including inactive
-- tickets -- numbers are never reused. Seq past 999 fails
-- vendor_request_vault_tickets_seq_chk. Call LockVaultTicketSeq first.
INSERT INTO vendor_request_vault_tickets (vendor_request_id, vendor_vault_id, replenish_date, seq, ticket_number)
SELECT sqlc.arg('vendor_request_id')::bigint, vv.id, sqlc.arg('replenish_date')::date, n.seq,
       vv.vault_code || '_' || to_char(sqlc.arg('replenish_date')::date, 'YYYYMMDD') || '_' || lpad(n.seq::text, 3, '0')
FROM vendor_vaults vv,
     (SELECT (COALESCE(max(seq), 0) + 1)::smallint AS seq
      FROM vendor_request_vault_tickets
      WHERE vendor_vault_id = sqlc.arg('vendor_vault_id')::bigint
        AND replenish_date = sqlc.arg('replenish_date')::date) n
WHERE vv.id = sqlc.arg('vendor_vault_id')::bigint
RETURNING ticket_number;

-- name: SetVaultTicketActive :one
-- Deactivate (vault no longer allocated) or reactivate (same vault back in the
-- same request -> same number).
UPDATE vendor_request_vault_tickets
SET is_active = sqlc.arg('is_active')::boolean,
    deactivated_at = CASE WHEN sqlc.arg('is_active')::boolean THEN NULL ELSE now() END
WHERE id = sqlc.arg('id')::bigint
RETURNING ticket_number;
