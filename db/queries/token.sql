-- name: InsertToken :one
INSERT INTO access_tokens (user_id, name, expires_at, scopes)
VALUES ($1, $2, $3, $4) RETURNING id;

-- name: GetTokenAuth :one
SELECT scopes, revoked FROM access_tokens
WHERE id = $1 AND user_id = $2 AND expires_at > now();

-- name: ListTokens :many
SELECT id, name, created_at, expires_at, revoked, scopes
FROM access_tokens WHERE user_id = $1 ORDER BY created_at DESC;

-- name: RevokeToken :execresult
UPDATE access_tokens SET revoked = TRUE
WHERE id = $1 AND user_id = $2 AND revoked = FALSE;
