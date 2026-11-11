-- ext-user-api: external API clients (migration 039). secret_hash is selected
-- only where verification needs it; it never goes to a response or a log.

-- name: GetApiClientByClientID :one
SELECT id, client_id, name, secret_hash, credential_version, scopes, assignable_roles,
       allowed_cidrs, service_user_id, is_active
FROM api_clients
WHERE client_id = $1;

-- name: ListApiClients :many
SELECT c.id, c.client_id, c.name, c.credential_version, c.scopes, c.assignable_roles,
       c.allowed_cidrs, c.service_user_id, u.username AS service_username,
       c.is_active, c.created_at, c.updated_at, c.secret_rotated_at, c.disabled_at
FROM api_clients c
JOIN users u ON u.id = c.service_user_id
ORDER BY c.id;

-- name: CreateApiClient :one
INSERT INTO api_clients (client_id, name, secret_hash, scopes, assignable_roles,
                         allowed_cidrs, service_user_id, created_by)
VALUES (sqlc.arg('client_id'), sqlc.arg('name'), sqlc.arg('secret_hash'), sqlc.arg('scopes'),
        sqlc.arg('assignable_roles'), sqlc.arg('allowed_cidrs'), sqlc.arg('service_user_id'),
        sqlc.arg('created_by'))
RETURNING id, client_id, name, credential_version, scopes, assignable_roles, allowed_cidrs,
          service_user_id, is_active, created_at;

-- name: RotateApiClientSecret :one
UPDATE api_clients
SET secret_hash = sqlc.arg('secret_hash'),
    credential_version = credential_version + 1,
    secret_rotated_at = now(),
    updated_at = now()
WHERE client_id = sqlc.arg('client_id')
RETURNING id, client_id, credential_version, service_user_id;

-- name: SetApiClientActive :one
UPDATE api_clients
SET is_active = sqlc.arg('is_active'),
    disabled_at = CASE WHEN sqlc.arg('is_active')::boolean THEN NULL ELSE now() END,
    updated_at = now()
WHERE client_id = sqlc.arg('client_id')
RETURNING id, client_id, is_active, service_user_id;

-- name: UpdateApiClientGrants :one
UPDATE api_clients
SET scopes = sqlc.arg('scopes'),
    assignable_roles = sqlc.arg('assignable_roles'),
    allowed_cidrs = sqlc.arg('allowed_cidrs'),
    updated_at = now()
WHERE client_id = sqlc.arg('client_id')
RETURNING id, client_id, scopes, assignable_roles, allowed_cidrs, service_user_id;

-- name: GetActiveAppAccessUser :one
-- For the CLI --actor flag: an active, non-deleted APPACCESS user.
SELECT u.id, u.username
FROM users u
JOIN roles r ON r.id = u.role_id
WHERE u.username = $1 AND u.is_active AND u.deleted_at IS NULL AND r.role = 'APPACCESS';

-- name: CreateServiceUser :one
-- Service account behind one api_client: inactive, no password, role EXT-CLIENT.
INSERT INTO users (role_id, username, full_name, email, is_karyawan, auth_source,
                   password_hash, is_active)
SELECT r.id, sqlc.arg('username'), sqlc.arg('full_name'), sqlc.arg('email'),
       false, 'service', NULL, false
FROM roles r
WHERE r.role = 'EXT-CLIENT'
RETURNING id;
