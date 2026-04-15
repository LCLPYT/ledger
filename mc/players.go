package mc

import (
	"database/sql"
	"errors"
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
			go fetchAndCacheUsername(db, id, uuid)
		}
		return id, nil
	}
	if err != nil {
		return 0, err
	}
	// New player: fetch username in background after the transaction commits.
	// The Mojang API call takes long enough (network) that the commit will have happened.
	go fetchAndCacheUsername(db, id, uuid)
	return id, nil
}

// GetPlayerID returns the DB id for a minecraft UUID, or 0 if not found.
func GetPlayerID(db *sql.DB, uuid string) (int64, error) {
	var id int64
	err := db.QueryRow(
		`SELECT id FROM minecraft_players WHERE uuid = $1`,
		uuid,
	).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	return id, err
}

func fetchAndCacheUsername(db *sql.DB, playerID int64, uuid string) {
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
