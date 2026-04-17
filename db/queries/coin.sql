-- name: InsertCoinTransaction :exec
INSERT INTO coin_transactions (player_id, amount, source, description, actor_user_id, actor_token_id)
VALUES ($1, $2, $3, $4, $5, $6);

-- name: GetLockedBalance :one
SELECT balance FROM coin_balances WHERE player_id = $1 FOR UPDATE;

-- name: UpsertBalance :one
INSERT INTO coin_balances (player_id, balance, updated_at) VALUES ($1, $2, now())
ON CONFLICT (player_id) DO UPDATE
  SET balance = coin_balances.balance + $2, updated_at = now()
RETURNING balance;

-- name: GetPlayerBalance :one
SELECT cb.balance FROM coin_balances cb
JOIN minecraft_players mp ON mp.id = cb.player_id WHERE mp.uuid = $1;

-- name: ListPlayerTransactions :many
SELECT id, player_id, amount, source, description, created_at, actor_user_id, actor_token_id
FROM coin_transactions WHERE player_id = $1
ORDER BY created_at DESC LIMIT $2 OFFSET $3;
