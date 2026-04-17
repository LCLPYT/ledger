-- name: ListPlayers :many
SELECT id, uuid, username, created_at
FROM minecraft_players
WHERE ($1 = ''
       OR LOWER(username) LIKE '%' || LOWER($1) || '%'
       OR LOWER(uuid::text) = LOWER($1))
ORDER BY
  CASE WHEN $1 = '' THEN 0 ELSE 1 END,
  CASE WHEN LOWER(uuid::text) = LOWER($1) THEN 0 ELSE 1 END,
  similarity(username, $1) DESC,
  created_at DESC
LIMIT $2 OFFSET $3;

-- name: InsertPlayerByUUID :one
INSERT INTO minecraft_players (uuid) VALUES ($1)
ON CONFLICT (uuid) DO NOTHING RETURNING id;

-- name: GetPlayerIDAndFetchTime :one
SELECT id, username_fetched_at FROM minecraft_players WHERE uuid = $1;

-- name: GetPlayerByUUID :one
SELECT id, uuid, username, created_at, username_fetched_at FROM minecraft_players WHERE uuid = $1;

-- name: UpsertPlayerWithUsername :exec
INSERT INTO minecraft_players (uuid, username, username_fetched_at)
VALUES ($1, $2, now())
ON CONFLICT (uuid) DO UPDATE SET username = EXCLUDED.username, username_fetched_at = now();

-- name: UpdatePlayerUsername :exec
UPDATE minecraft_players SET username = NULLIF($1, ''), username_fetched_at = now() WHERE id = $2;

-- name: GetPlayerUUIDByName :one
SELECT uuid FROM minecraft_players
WHERE LOWER(username) = LOWER($1) AND username_fetched_at > now() - INTERVAL '1 hour';

-- name: DeletePlayerByUUID :execresult
DELETE FROM minecraft_players WHERE uuid = $1;

-- name: GetPlayerData :one
SELECT data FROM minecraft_players WHERE uuid = $1;

-- name: GetPlayerDataAtPath :one
SELECT data #> $1::text[] FROM minecraft_players WHERE uuid = $2;

-- name: GetPlayerDataForUpdate :one
SELECT data FROM minecraft_players WHERE id = $1 FOR UPDATE;

-- name: UpdatePlayerData :exec
UPDATE minecraft_players SET data = $1::jsonb WHERE id = $2;

-- name: DeletePlayerDataAtPath :execresult
UPDATE minecraft_players SET data = data #- $1::text[] WHERE uuid = $2;
