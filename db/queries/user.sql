-- name: AddUser :one
INSERT INTO users (username, email, password_hash, verified_at)
VALUES ($1, $2, $3, now())
RETURNING id;