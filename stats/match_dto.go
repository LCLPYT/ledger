package stats

import (
	"time"

	"github.com/gofrs/uuid/v5"
)

type MatchInput struct {
	ID               uuid.UUID          `json:"id"`
	Status           string             `json:"status"`
	Game             Ref                `json:"game"`
	MinecraftVersion string             `json:"minecraft_version"`
	ServerName       string             `json:"server_name"`
	StartedAt        time.Time          `json:"started_at"`
	EndedAt          time.Time          `json:"ended_at"`
	Players          []PlayerInput      `json:"players"`
	Presence         []PresenceInput    `json:"presence"`
	Results          []MatchResultInput `json:"results"`
	Stats            map[string]any     `json:"stats"`
	Rounds           []RoundInput       `json:"rounds"`
}

type PlayerInput struct {
	UUID uuid.UUID `json:"uuid"`
	Name string    `json:"name"`
}

type PresenceInput struct {
	Player   uuid.UUID  `json:"player"`
	JoinedAt time.Time  `json:"joined_at"`
	LeftAt   *time.Time `json:"left_at"`
}

type MatchResultInput struct {
	Player    uuid.UUID `json:"player"`
	Score     int32     `json:"score"`
	Placement *int32    `json:"placement"`
	Winner    bool      `json:"winner"`
}

type RoundInput struct {
	Index     int32              `json:"index"`
	Minigame  Ref                `json:"minigame"`
	Status    string             `json:"status"`
	StartedAt time.Time          `json:"started_at"`
	EndedAt   time.Time          `json:"ended_at"`
	Map       *MapInput          `json:"map"`
	StatsRef  *uuid.UUID         `json:"stats_ref"`
	Teams     []RoundTeamInput   `json:"teams"`
	Players   []RoundPlayerInput `json:"players"`
}

type MapInput struct {
	ID   string `json:"id"`
	Seed *int64 `json:"seed"`
}

type RoundTeamInput struct {
	Key   string         `json:"key"`
	Rank  *int32         `json:"rank"`
	Stats map[string]any `json:"stats"`
}

type RoundPlayerInput struct {
	Player uuid.UUID      `json:"player"`
	Team   string         `json:"team"`
	Rank   *int32         `json:"rank"`
	Points int32          `json:"points"`
	Stats  map[string]any `json:"stats"`
}

type IngestResult struct {
	MatchID  int64     `json:"match_id"`
	PublicID uuid.UUID `json:"public_id"`
	Created  bool      `json:"created"`
}
