package mc

import (
	"database/sql"
	"errors"
	"ledger/models"
	"ledger/mojang"
	"time"
)

// FetchUsername is the function used to resolve a Minecraft UUID to a username.
// It can be replaced in tests to avoid real network calls.
var FetchUsername = mojang.FetchUsername

// FetchUUIDByName is the function used to resolve a Minecraft username to a UUID.
// It can be replaced in tests to avoid real network calls.
var FetchUUIDByName = mojang.FetchUUIDByName

const UsernameStaleDuration = 7 * 24 * time.Hour

// UpsertPlayer inserts a minecraft_players row if it doesn't exist and returns the player's DB id.
// A background goroutine fetches and caches the Minecraft username whenever the cache is missing
// or older than usernameStaleDuration.
// db is used only for that background fetch; tx is used for the insert/select.
func UpsertPlayer(db *sql.DB, tx *sql.Tx, uuid string) (int64, error) {
	var id int64
	err := tx.QueryRow(
		`INSERT INTO minecraft_players (uuid) 
			VALUES ($1) ON CONFLICT (uuid) DO NOTHING 
			RETURNING id`,
		uuid,
	).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		// Player already existed — fetch id and username cache timestamp.
		var fetchedAt *time.Time
		err = tx.QueryRow(
			`SELECT id, username_fetched_at FROM minecraft_players WHERE uuid = $1`,
			uuid,
		).Scan(&id, &fetchedAt)
		if err != nil {
			return 0, err
		}
		if fetchedAt == nil || time.Since(*fetchedAt) > UsernameStaleDuration {
			go RefreshUsernameCache(db, id, uuid)
		}
		return id, nil
	}
	if err != nil {
		return 0, err
	}
	// New player: fetch username in background after the transaction commits.
	// The Mojang API call takes long enough (network) that the commit will have happened.
	go RefreshUsernameCache(db, id, uuid)
	return id, nil
}

// GetPlayerID returns the DB id for a minecraft UUID, or 0 if not found.
func GetPlayerID(db *sql.DB, uuid string) (int64, error) {
	player, _, err := QueryPlayer(db, uuid)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}

	return player.ID, err
}

// RefreshUsernameCache fetches the current username from Mojang and updates the DB row.
// Intended to be called in a goroutine.
func RefreshUsernameCache(db *sql.DB, playerID int64, uuid string) {
	name, err := FetchUsername(uuid)
	if err != nil {
		return
	}
	_, _ = db.Exec(
		`UPDATE minecraft_players 
			SET username = NULLIF($1, ''), username_fetched_at = now() 
			WHERE id = $2`,
		name, playerID,
	)
}

// QueryPlayer fetches a player row by UUID.
// Returns sql.ErrNoRows if the player doesn't exist.
func QueryPlayer(db *sql.DB, uid string) (models.MinecraftPlayer, *time.Time, error) {
	var p models.MinecraftPlayer
	var fetchedAt *time.Time
	err := db.QueryRow(
		`SELECT id, uuid, username, created_at, username_fetched_at
		 FROM minecraft_players WHERE uuid = $1`,
		uid,
	).Scan(&p.ID, &p.UUID, &p.Username, &p.CreatedAt, &fetchedAt)
	return p, fetchedAt, err
}

// UpsertPlayerWithUsername inserts or updates a player row with a known username and
// returns the full player record. Used when the username is already fetched from Mojang.
func UpsertPlayerWithUsername(db *sql.DB, uuid, username string) (models.MinecraftPlayer, error) {
	_, err := db.Exec(
		`INSERT INTO minecraft_players (uuid, username, username_fetched_at)
		 VALUES ($1, $2, now())
		 ON CONFLICT (uuid) DO UPDATE
		   SET username = EXCLUDED.username, username_fetched_at = now()`,
		uuid, username,
	)
	if err != nil {
		return models.MinecraftPlayer{}, err
	}
	p, _, err := QueryPlayer(db, uuid)
	return p, err
}
