-- name: InsertSession :one
INSERT INTO sessions (user_id, expires_at) VALUES ($1, $2) RETURNING id;

-- name: SessionExists :one
SELECT EXISTS(
  SELECT 1 FROM sessions WHERE id = $1 AND user_id = $2 AND expires_at > now()
);

-- name: RefreshSession :execresult
UPDATE sessions SET expires_at = $1 WHERE id = $2 AND user_id = $3;

-- name: DeleteUserSessions :exec
DELETE FROM sessions WHERE user_id = $1;
