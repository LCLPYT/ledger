package mc

import (
	"context"
	"errors"
	dbsqlc "ledger/db/sqlc"
	"ledger/models"
	"ledger/mojang"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
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
// or older than UsernameStaleDuration.
// pool is used only for background username fetching; q must be a tx-backed or pool-backed Queries.
func UpsertPlayer(pool *pgxpool.Pool, q *dbsqlc.Queries, uuid string) (int64, error) {
	ctx := context.Background()

	id, err := q.InsertPlayerByUUID(ctx, uuid)
	if err != nil && errors.Is(err, pgx.ErrNoRows) {
		// Player already existed — fetch id and username cache timestamp.
		row, err := q.GetPlayerIDAndFetchTime(ctx, uuid)
		if err != nil {
			return 0, err
		}
		if !row.UsernameFetchedAt.Valid || time.Since(row.UsernameFetchedAt.Time) > UsernameStaleDuration {
			go RefreshUsernameCache(pool, row.ID, uuid)
		}
		return row.ID, nil
	}
	if err != nil {
		return 0, err
	}
	// New player: fetch username in background after the transaction commits.
	go RefreshUsernameCache(pool, id, uuid)
	return id, nil
}

// GetPlayerID returns the DB id for a minecraft UUID, or 0 if not found.
func GetPlayerID(pool *pgxpool.Pool, uuid string) (int64, error) {
	q := dbsqlc.New(pool)
	row, err := q.GetPlayerByUUID(context.Background(), uuid)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, nil
	}
	return row.ID, err
}

// RefreshUsernameCache fetches the current username from Mojang and updates the DB row.
// Intended to be called in a goroutine.
func RefreshUsernameCache(pool *pgxpool.Pool, playerID int64, uuid string) {
	name, err := FetchUsername(uuid)
	if err != nil {
		return
	}
	q := dbsqlc.New(pool)
	_ = q.UpdatePlayerUsername(context.Background(), dbsqlc.UpdatePlayerUsernameParams{
		Column1: name,
		ID:      playerID,
	})
}

// QueryPlayer fetches a player row by UUID.
// Returns pgx.ErrNoRows if the player doesn't exist.
func QueryPlayer(pool *pgxpool.Pool, uid string) (models.MinecraftPlayer, *time.Time, error) {
	q := dbsqlc.New(pool)
	row, err := q.GetPlayerByUUID(context.Background(), uid)
	if err != nil {
		return models.MinecraftPlayer{}, nil, err
	}

	p := rowToPlayer(row)

	var fetchedAt *time.Time
	if row.UsernameFetchedAt.Valid {
		fetchedAt = new(row.UsernameFetchedAt.Time)
	}
	return p, fetchedAt, nil
}

// UpsertPlayerWithUsername inserts or updates a player row with a known username and
// returns the full player record. Used when the username is already fetched from Mojang.
func UpsertPlayerWithUsername(pool *pgxpool.Pool, uuid, username string) (models.MinecraftPlayer, error) {
	q := dbsqlc.New(pool)
	err := q.UpsertPlayerWithUsername(context.Background(), dbsqlc.UpsertPlayerWithUsernameParams{
		Uuid:     uuid,
		Username: pgtype.Text{String: username, Valid: true},
	})
	if err != nil {
		return models.MinecraftPlayer{}, err
	}
	p, _, err := QueryPlayer(pool, uuid)
	return p, err
}

func rowToPlayer(row dbsqlc.GetPlayerByUUIDRow) models.MinecraftPlayer {
	p := models.MinecraftPlayer{
		ID:        row.ID,
		UUID:      row.Uuid,
		CreatedAt: row.CreatedAt.Time,
	}
	if row.Username.Valid {
		p.Username = new(row.Username.String)
	}
	return p
}
