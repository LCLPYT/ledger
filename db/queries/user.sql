-- name: AddUser :one
INSERT INTO users (username, email, password_hash, verified_at)
VALUES ($1, $2, $3, now())
RETURNING id;

-- name: GetUserByIdentifier :one
SELECT id, password_hash FROM users
WHERE (username = $1 OR email = $1) AND verified_at IS NOT NULL;

-- name: GetUserByID :one
SELECT id, username, email, created_at FROM users WHERE id = $1;

-- name: ListUsers :many
SELECT id, username, email, created_at FROM users ORDER BY created_at DESC;

-- name: CreateShellUser :one
INSERT INTO users (username, email) VALUES ($1, $2)
RETURNING id, username, email, created_at;

-- name: GetUserPassword :one
SELECT password_hash FROM users WHERE id = $1;

-- name: UpdateUserPassword :exec
UPDATE users SET password_hash = $1 WHERE id = $2;

-- name: UpdateUsername :exec
UPDATE users SET username = $1 WHERE id = $2;

-- name: UpdateEmail :exec
UPDATE users SET email = $1 WHERE id = $2;

-- name: DeleteUser :execresult
DELETE FROM users WHERE id = $1;
