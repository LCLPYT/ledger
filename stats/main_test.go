package stats

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"ledger/config"
	"ledger/db"
	"ledger/db/migrations"

	"github.com/pressly/goose/v3"
	"github.com/stephenafamo/bob"
)

var testDB bob.DB

// devDatabasePort is the port of the development database. The tests truncate
// every table, so they must never run against it.
const devDatabasePort = 5432

func TestMain(m *testing.M) {
	code, err := runTests(m)

	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	os.Exit(code)
}

// useModuleRoot moves to the module root so the shared .env file is found the
// same way the server finds it.
func useModuleRoot() error {
	dir, err := os.Getwd()

	if err != nil {
		return err
	}

	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return os.Chdir(dir)
		}

		parent := filepath.Dir(dir)

		if parent == dir {
			return fmt.Errorf("could not find the module root")
		}

		dir = parent
	}
}

func runTests(m *testing.M) (int, error) {
	if err := useModuleRoot(); err != nil {
		return 0, err
	}

	v, err := config.CreateCommonConfig()

	if err != nil {
		return 0, err
	}

	v.SetDefault("database_port", devDatabasePort)

	if v.GetInt("database_port") == devDatabasePort {
		return 0, fmt.Errorf("refusing to run tests against port %d, set DATABASE_PORT=5433 or use `just test`", devDatabasePort)
	}

	sqldb, err := db.OpenSQL(v)

	if err != nil {
		return 0, err
	}

	defer sqldb.Close()

	provider, err := goose.NewProvider(goose.DialectPostgres, sqldb, migrations.FS)

	if err != nil {
		return 0, err
	}

	if _, err := provider.Up(context.Background()); err != nil {
		return 0, err
	}

	testDB = bob.NewDB(sqldb)

	return m.Run(), nil
}

const truncateSQL = `
TRUNCATE
    "stat_values", "stat_records", "player_game_stats", "player_minigame_stats",
    "round_players", "round_teams", "rounds",
    "match_player_sessions", "match_players", "matches",
    "definition_version_stats", "stats", "definition_versions", "definitions",
    "players"
RESTART IDENTITY CASCADE
`

// newService truncates the domain tables and returns a service with an empty
// schema cache.
func newService(t *testing.T) *Service {
	t.Helper()

	if _, err := testDB.ExecContext(t.Context(), truncateSQL); err != nil {
		t.Fatalf("truncate: %v", err)
	}

	return NewService(testDB)
}
