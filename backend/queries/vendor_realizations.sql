-- Laporan realisasi vendor per tiket replenish (replenishment-realisasi FR2, migration 031).
-- Ditulis vendor (pihak replenish) dari VendorPortal; menggantikan input ATM-USER pada laporan selesai.

-- name: ListRequestTicketOrders :many
-- Per tiket aktif request: jadwal + order per denom (input validasi FR2.2). Satu baris per (ATM, denom).
SELECT t.id AS ticket_id, t.terminal_id, t.ticket_number, t.replenish_date,
       i.denom, SUM(i.amount_replenish)::bigint AS order_amount
FROM vendor_request_tickets t
JOIN vendor_requests vr ON vr.request_number = t.request_number
JOIN vendor_request_items i ON i.vendor_request_id = vr.id AND i.terminal_id = t.terminal_id
WHERE vr.id = sqlc.arg('vendor_request_id')::bigint AND t.is_active
GROUP BY t.id, t.terminal_id, t.ticket_number, t.replenish_date, i.denom
ORDER BY t.terminal_id, i.denom DESC;

-- name: UpsertVendorRealization :one
-- Satu laporan per tiket; resubmit setelah reject menimpa (FR2.3). realized_date NULL = failed.
INSERT INTO vendor_request_realizations (ticket_id, vendor_request_id, realized_date, submitted_by)
VALUES (sqlc.arg('ticket_id')::bigint, sqlc.arg('vendor_request_id')::bigint,
        sqlc.narg('realized_date')::date, sqlc.arg('submitted_by')::bigint)
ON CONFLICT (ticket_id) DO UPDATE
SET realized_date = EXCLUDED.realized_date,
    submitted_by  = EXCLUDED.submitted_by,
    submitted_at  = now()
RETURNING id;

-- name: DeleteVendorRealizationAmounts :exec
-- Replace-all nominal anak saat resubmit (hard delete baris anak terdokumentasi di migrasi 031).
DELETE FROM vendor_request_realization_amounts WHERE realization_id = $1;

-- name: InsertVendorRealizationAmount :exec
INSERT INTO vendor_request_realization_amounts (realization_id, denom, amount)
VALUES (sqlc.arg('realization_id')::bigint, sqlc.arg('denom')::int, sqlc.arg('amount')::bigint);

-- name: ListVendorRealizationsForRequest :many
-- Laporan per ATM untuk detail request (internal + VendorPortal). Satu baris per (ATM, denom);
-- denom/amount NULL untuk ATM failed. Tanpa laporan -> tidak ada baris untuk ATM itu.
SELECT t.terminal_id, t.ticket_number, t.result, r.realized_date, r.submitted_at,
       r.submitted_by, u.full_name AS submitted_by_name,
       a.denom, a.amount
FROM vendor_request_realizations r
JOIN vendor_request_tickets t ON t.id = r.ticket_id AND t.is_active
JOIN users u ON u.id = r.submitted_by
LEFT JOIN vendor_request_realization_amounts a ON a.realization_id = r.id
WHERE r.vendor_request_id = sqlc.arg('vendor_request_id')::bigint
ORDER BY t.terminal_id, a.denom DESC NULLS LAST;
