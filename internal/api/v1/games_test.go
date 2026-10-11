package v1_test

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/aarondl/opt/omit"
	"github.com/aarondl/opt/omitnull"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/quadrubo/golib/app"
	"github.com/quadrubo/golib/testkit"
	. "github.com/quadrubo/golib/testkit/matchers"
	"github.com/samber/do/v2"
	"github.com/stephenafamo/bob"
	"github.com/stephenafamo/bob/types"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/testing/protocmp"
	"google.golang.org/protobuf/types/known/fieldmaskpb"

	ledgerv1 "github.com/lclpyt/ledger/gen/go/apis/ledger/v1"
	"github.com/lclpyt/ledger/gen/go/bob/enums"
	"github.com/lclpyt/ledger/gen/go/bob/model"
	"github.com/lclpyt/ledger/internal/wiring"
)

const (
	// The postgres_test service of docker-compose.yml. The suite recreates testDatabaseName on it every run.
	testServerURL    = "postgres://db:db@127.0.0.1:5433/db?sslmode=disable"
	testDatabaseURL  = "postgres://db:db@127.0.0.1:5433/" + testDatabaseName + "?sslmode=disable"
	testDatabaseName = "ledger_api_v1_test"

	errorDomain = "ledger.test"
)

var (
	suite                *testkit.Suite
	db                   bob.DB
	client               ledgerv1.GamesServiceClient
	playerSettingsClient ledgerv1.PlayerSettingsServiceClient
)

func TestV1(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "API v1 Suite")
}

type testDatabase struct {
	db bob.DB
}

var (
	_ testkit.Dependency       = (*testDatabase)(nil)
	_ testkit.SettingsProvider = (*testDatabase)(nil)
)

func (d *testDatabase) Start(ctx context.Context, injector do.Injector) error {
	if err := recreateDatabase(ctx); err != nil {
		return err
	}

	conn, err := bob.Open("pgx", testDatabaseURL)

	if err != nil {
		return fmt.Errorf("test database: failed to open: %w", err)
	}

	d.db = conn
	do.ProvideValue(injector, conn)

	return nil
}

func recreateDatabase(ctx context.Context) error {
	server, err := bob.Open("pgx", testServerURL)

	if err != nil {
		return fmt.Errorf("test database: failed to open the server connection: %w", err)
	}

	defer func() { _ = server.Close() }()

	for _, stmt := range []string{
		"DROP DATABASE IF EXISTS " + testDatabaseName + " WITH (FORCE)",
		"CREATE DATABASE " + testDatabaseName,
	} {
		if _, err := server.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("test database: failed to recreate, is postgres_test up? %w", err)
		}
	}

	return nil
}

func (d *testDatabase) Stop(context.Context) error {
	return d.db.Close()
}

func (d *testDatabase) Settings() map[string]any {
	return map[string]any{
		"modules": map[string]any{
			"postgres": map[string]any{"url": testDatabaseURL},
		},
	}
}

var _ = BeforeSuite(func(ctx SpecContext) {
	var err error

	suite, err = testkit.Boot(ctx, testkit.Options{
		Modules: func(conf app.Module) []app.Module {
			return slices.Concat(
				wiring.Auth(conf),
				wiring.GRPC(),
			)
		},
		Dependencies: []testkit.Dependency{&testDatabase{}},
		Settings: map[string]any{
			"modules": map[string]any{
				"grpcerr": map[string]any{"domain": errorDomain},
				"logging": map[string]any{"level": "error"},
			},
		},
	})

	Expect(err).ToNot(HaveOccurred())
	DeferCleanup(func() { Expect(suite.Stop()).To(Succeed()) })

	db = do.MustInvoke[bob.DB](suite.Injector())

	conn := authedConn(createAPIKey(ctx, "*"))
	client = ledgerv1.NewGamesServiceClient(conn)
	playerSettingsClient = ledgerv1.NewPlayerSettingsServiceClient(conn)
})

func newGame() *ledgerv1.Game {
	return &ledgerv1.Game{
		Type:             ledgerv1.Game_TYPE_MINIGAME,
		NameTranslations: map[string]string{"en": "Spleef", "de": "Spleef"},
	}
}

func createGame(ctx context.Context) *ledgerv1.Game {
	GinkgoHelper()

	game, err := client.CreateGame(ctx, &ledgerv1.CreateGameRequest{
		GameId: testkit.ResourceID("game"),
		Game:   newGame(),
	})

	Expect(err).ToNot(HaveOccurred())

	return game
}

func seedDeletedGame(ctx context.Context) string {
	GinkgoHelper()

	id := testkit.ResourceID("deleted")
	now := time.Now()

	_, err := model.Games.Insert(&model.GameSetter{
		ID:               omit.From(id),
		Type:             omit.From(enums.GameTypeGame),
		NameTranslations: omit.From(types.NewJSON(json.RawMessage(`{"en":"Deleted"}`))),
		DeleteTime:       omitnull.From(now),
		PurgeTime:        omitnull.From(now.Add(30 * 24 * time.Hour)),
	}).One(ctx, db)

	Expect(err).ToNot(HaveOccurred())

	return "games/" + id
}

func listAll(ctx context.Context, req *ledgerv1.ListGamesRequest) []string {
	GinkgoHelper()

	var names []string

	for {
		resp, err := client.ListGames(ctx, req)
		Expect(err).ToNot(HaveOccurred())

		if req.GetPageSize() > 0 {
			Expect(len(resp.GetGames())).To(BeNumerically("<=", req.GetPageSize()))
		}

		for _, game := range resp.GetGames() {
			names = append(names, game.GetName())
		}

		if resp.GetNextPageToken() == "" {
			return names
		}

		req.PageToken = resp.GetNextPageToken()
	}
}

func expectStatus(err error, code codes.Code) {
	GinkgoHelper()

	Expect(err).To(HaveOccurred())
	Expect(status.Code(err)).To(Equal(code), "unexpected status: %v", err)
	Expect(err).To(HaveErrorDomain(errorDomain))
}

var _ = Describe("GamesService", func() {
	Describe("CreateGame", func() {
		It("stores the game under the id the call supplies", func(ctx SpecContext) {
			id := testkit.ResourceID("game")

			game, err := client.CreateGame(ctx, &ledgerv1.CreateGameRequest{GameId: id, Game: newGame()})

			Expect(err).ToNot(HaveOccurred())
			Expect(game.GetName()).To(Equal("games/" + id))
			Expect(game.GetType()).To(Equal(ledgerv1.Game_TYPE_MINIGAME))
			Expect(game.GetNameTranslations()).To(Equal(map[string]string{"en": "Spleef", "de": "Spleef"}))
			Expect(game.GetCreateTime().AsTime()).To(BeTemporally("~", time.Now(), time.Minute))
			Expect(game.GetUpdateTime().AsTime()).To(BeTemporally("~", time.Now(), time.Minute))
			Expect(game.GetDeleteTime()).To(BeNil())
			Expect(game.GetPurgeTime()).To(BeNil())
		})

		It("ignores the name the game carries", func(ctx SpecContext) {
			id := testkit.ResourceID("game")
			game := newGame()
			game.Name = "games/" + testkit.ResourceID("carried")

			created, err := client.CreateGame(ctx, &ledgerv1.CreateGameRequest{GameId: id, Game: game})

			Expect(err).ToNot(HaveOccurred())
			Expect(created.GetName()).To(Equal("games/" + id))
		})

		It("returns an AlreadyExists for an id a game already carries", func(ctx SpecContext) {
			existing := createGame(ctx)

			_, err := client.CreateGame(ctx, &ledgerv1.CreateGameRequest{
				GameId: existing.GetName()[len("games/"):],
				Game:   newGame(),
			})

			expectStatus(err, codes.AlreadyExists)
			Expect(err).To(HaveReason("GAME_ALREADY_EXISTS"))
			Expect(err).To(HaveErrorMetadata("name", existing.GetName()))
		})

		It("returns an AlreadyExists for an id a deleted game carries", func(ctx SpecContext) {
			deleted := seedDeletedGame(ctx)

			_, err := client.CreateGame(ctx, &ledgerv1.CreateGameRequest{
				GameId: deleted[len("games/"):],
				Game:   newGame(),
			})

			expectStatus(err, codes.AlreadyExists)
		})

		DescribeTable("rejects an invalid request",
			func(ctx SpecContext, field string, mutate func(*ledgerv1.CreateGameRequest)) {
				req := &ledgerv1.CreateGameRequest{GameId: testkit.ResourceID("game"), Game: newGame()}
				mutate(req)

				_, err := client.CreateGame(ctx, req)

				expectStatus(err, codes.InvalidArgument)
				Expect(err).To(HaveViolatedField(field))
			},
			Entry("without a game", "game", func(req *ledgerv1.CreateGameRequest) {
				req.Game = nil
			}),
			Entry("without a game_id", "game_id", func(req *ledgerv1.CreateGameRequest) {
				req.GameId = ""
			}),
			Entry("with a game_id outside the id pattern", "game_id", func(req *ledgerv1.CreateGameRequest) {
				req.GameId = "Not_An-Id"
			}),
			Entry("with a game_id ending in a hyphen", "game_id", func(req *ledgerv1.CreateGameRequest) {
				req.GameId = "game-"
			}),
			Entry("without a type", "game.type", func(req *ledgerv1.CreateGameRequest) {
				req.Game.Type = ledgerv1.Game_TYPE_UNSPECIFIED
			}),
			Entry("with an undefined type", "game.type", func(req *ledgerv1.CreateGameRequest) {
				req.Game.Type = 99
			}),
			Entry("without an english name", "game.name_translations", func(req *ledgerv1.CreateGameRequest) {
				req.Game.NameTranslations = map[string]string{"de": "Spleef"}
			}),
			Entry("with an empty english name", "game.name_translations", func(req *ledgerv1.CreateGameRequest) {
				req.Game.NameTranslations = map[string]string{"en": ""}
			}),
			Entry("with a malformed language tag", `game.name_translations["EN"]`,
				func(req *ledgerv1.CreateGameRequest) {
					req.Game.NameTranslations = map[string]string{"en": "Spleef", "EN": "Spleef"}
				}),
		)
	})

	Describe("GetGame", func() {
		It("returns the game the create returned", func(ctx SpecContext) {
			created := createGame(ctx)

			found, err := client.GetGame(ctx, &ledgerv1.GetGameRequest{Name: created.GetName()})

			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeComparableTo(created, protocmp.Transform()))
		})

		It("returns a deleted game", func(ctx SpecContext) {
			name := seedDeletedGame(ctx)

			found, err := client.GetGame(ctx, &ledgerv1.GetGameRequest{Name: name})

			Expect(err).ToNot(HaveOccurred())
			Expect(found.GetName()).To(Equal(name))
			Expect(found.GetType()).To(Equal(ledgerv1.Game_TYPE_GAME))
			Expect(found.GetDeleteTime()).ToNot(BeNil())
			Expect(found.GetPurgeTime()).ToNot(BeNil())
		})

		It("returns a NotFound for a game that does not exist", func(ctx SpecContext) {
			name := "games/" + testkit.ResourceID("absent")

			_, err := client.GetGame(ctx, &ledgerv1.GetGameRequest{Name: name})

			expectStatus(err, codes.NotFound)
			Expect(err).To(HaveReason("GAME_NOT_FOUND"))
			Expect(err).To(HaveErrorMetadata("name", name))
		})

		DescribeTable("names name on an InvalidArgument for a malformed name",
			func(ctx SpecContext, name string) {
				_, err := client.GetGame(ctx, &ledgerv1.GetGameRequest{Name: name})

				expectStatus(err, codes.InvalidArgument)
				Expect(err).To(HaveViolatedField("name"))
			},
			Entry("empty", ""),
			Entry("of another collection", "users/spleef"),
			Entry("with an id outside the id pattern", "games/Spleef"),
			Entry("with a trailing segment", "games/spleef/rounds/1"),
		)
	})

	Describe("ListGames", func() {
		It("returns the games in the order they were created", func(ctx SpecContext) {
			first, second, third := createGame(ctx), createGame(ctx), createGame(ctx)

			names := listAll(ctx, &ledgerv1.ListGamesRequest{})

			var positions []int
			for _, name := range []string{first.GetName(), second.GetName(), third.GetName()} {
				for i, listed := range names {
					if listed == name {
						positions = append(positions, i)
					}
				}
			}
			Expect(positions).To(HaveLen(3))
			Expect(positions[0]).To(BeNumerically("<", positions[1]))
			Expect(positions[1]).To(BeNumerically("<", positions[2]))
		})

		It("returns the games the get returns", func(ctx SpecContext) {
			created := createGame(ctx)

			resp, err := client.ListGames(ctx, &ledgerv1.ListGamesRequest{PageSize: 1000})
			Expect(err).ToNot(HaveOccurred())

			Expect(resp.GetGames()).To(ContainElement(BeComparableTo(created, protocmp.Transform())))
		})

		It("walks the collection by keyset without repeating a game", func(ctx SpecContext) {
			created := []string{createGame(ctx).GetName(), createGame(ctx).GetName(), createGame(ctx).GetName()}

			paged := listAll(ctx, &ledgerv1.ListGamesRequest{PageSize: 1})
			unpaged := listAll(ctx, &ledgerv1.ListGamesRequest{PageSize: 1000})

			Expect(paged).To(ContainElements(created))
			Expect(paged).To(Equal(unpaged))
		})

		It("takes a token under another page_size", func(ctx SpecContext) {
			createGame(ctx)
			createGame(ctx)

			first, err := client.ListGames(ctx, &ledgerv1.ListGamesRequest{PageSize: 1})
			Expect(err).ToNot(HaveOccurred())
			Expect(first.GetNextPageToken()).ToNot(BeEmpty())

			rest := listAll(ctx, &ledgerv1.ListGamesRequest{PageSize: 1000, PageToken: first.GetNextPageToken()})
			all := listAll(ctx, &ledgerv1.ListGamesRequest{PageSize: 1000})

			Expect(append([]string{first.GetGames()[0].GetName()}, rest...)).To(Equal(all))
		})

		It("leaves out deleted games", func(ctx SpecContext) {
			deleted := seedDeletedGame(ctx)
			active := createGame(ctx)

			names := listAll(ctx, &ledgerv1.ListGamesRequest{})

			Expect(names).To(ContainElement(active.GetName()))
			Expect(names).ToNot(ContainElement(deleted))
		})

		It("includes deleted games under show_deleted", func(ctx SpecContext) {
			deleted := seedDeletedGame(ctx)
			active := createGame(ctx)

			names := listAll(ctx, &ledgerv1.ListGamesRequest{ShowDeleted: true})

			Expect(names).To(ContainElements(active.GetName(), deleted))
		})

		It("names page_token on an InvalidArgument for a token that does not decode", func(ctx SpecContext) {
			_, err := client.ListGames(ctx, &ledgerv1.ListGamesRequest{PageToken: "not a token"})

			expectStatus(err, codes.InvalidArgument)
			Expect(err).To(HaveViolatedField("page_token"))
			Expect(err).To(HaveErrorMetadata("page_token", "not a token"))
		})

		It("names page_token on an InvalidArgument for a token issued under another show_deleted",
			func(ctx SpecContext) {
				createGame(ctx)
				createGame(ctx)

				first, err := client.ListGames(ctx, &ledgerv1.ListGamesRequest{PageSize: 1})
				Expect(err).ToNot(HaveOccurred())
				Expect(first.GetNextPageToken()).ToNot(BeEmpty())

				_, err = client.ListGames(ctx, &ledgerv1.ListGamesRequest{
					PageSize:    1,
					PageToken:   first.GetNextPageToken(),
					ShowDeleted: true,
				})

				expectStatus(err, codes.InvalidArgument)
				Expect(err).To(HaveViolatedField("page_token"))
			})

		It("names page_size on an InvalidArgument for a negative page_size", func(ctx SpecContext) {
			_, err := client.ListGames(ctx, &ledgerv1.ListGamesRequest{PageSize: -1})

			expectStatus(err, codes.InvalidArgument)
			Expect(err).To(HaveViolatedField("page_size"))
		})
	})

	Describe("UpdateGame", func() {
		update := func(ctx context.Context, game *ledgerv1.Game, paths ...string) (*ledgerv1.Game, error) {
			req := &ledgerv1.UpdateGameRequest{Game: game}

			if len(paths) > 0 {
				req.UpdateMask = &fieldmaskpb.FieldMask{Paths: paths}
			}

			return client.UpdateGame(ctx, req)
		}

		It("replaces the name translations the mask selects", func(ctx SpecContext) {
			created := createGame(ctx)
			translations := map[string]string{"en": "TNT Run", "fr": "Course TNT"}

			updated, err := update(ctx, &ledgerv1.Game{
				Name:             created.GetName(),
				NameTranslations: translations,
			}, "name_translations")

			Expect(err).ToNot(HaveOccurred())
			Expect(updated.GetName()).To(Equal(created.GetName()))
			Expect(updated.GetType()).To(Equal(created.GetType()))
			Expect(updated.GetNameTranslations()).To(Equal(translations))
			Expect(updated.GetCreateTime()).To(BeComparableTo(created.GetCreateTime(), protocmp.Transform()))
			Expect(updated.GetUpdateTime().AsTime()).To(BeTemporally(">", created.GetUpdateTime().AsTime()))
		})

		It("stores the update the get returns", func(ctx SpecContext) {
			created := createGame(ctx)

			updated, err := update(ctx, &ledgerv1.Game{
				Name:             created.GetName(),
				NameTranslations: map[string]string{"en": "TNT Run"},
			}, "name_translations")
			Expect(err).ToNot(HaveOccurred())

			found, err := client.GetGame(ctx, &ledgerv1.GetGameRequest{Name: created.GetName()})

			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeComparableTo(updated, protocmp.Transform()))
		})

		It("updates every writable field without a mask", func(ctx SpecContext) {
			created := createGame(ctx)

			updated, err := update(ctx, &ledgerv1.Game{
				Name:             created.GetName(),
				NameTranslations: map[string]string{"en": "TNT Run"},
			})

			Expect(err).ToNot(HaveOccurred())
			Expect(updated.GetNameTranslations()).To(Equal(map[string]string{"en": "TNT Run"}))
			Expect(updated.GetType()).To(Equal(created.GetType()))
		})

		It("validates the full writable set without a mask", func(ctx SpecContext) {
			created := createGame(ctx)

			_, err := update(ctx, &ledgerv1.Game{Name: created.GetName()})

			expectStatus(err, codes.InvalidArgument)
			Expect(err).To(HaveViolatedField("game.name_translations"))
		})

		It("leaves out of validation the fields the mask does not select", func(ctx SpecContext) {
			created := createGame(ctx)

			updated, err := update(ctx, &ledgerv1.Game{
				Name:             created.GetName(),
				Type:             ledgerv1.Game_TYPE_UNSPECIFIED,
				NameTranslations: map[string]string{"en": "TNT Run"},
			}, "name_translations")

			Expect(err).ToNot(HaveOccurred())
			Expect(updated.GetType()).To(Equal(created.GetType()))
		})

		It("takes the immutable type under an unchanged value", func(ctx SpecContext) {
			created := createGame(ctx)

			updated, err := update(ctx, &ledgerv1.Game{
				Name:             created.GetName(),
				Type:             created.GetType(),
				NameTranslations: map[string]string{"en": "TNT Run"},
			}, "type", "name_translations")

			Expect(err).ToNot(HaveOccurred())
			Expect(updated.GetType()).To(Equal(created.GetType()))
			Expect(updated.GetNameTranslations()).To(Equal(map[string]string{"en": "TNT Run"}))
		})

		It("names game.type on an InvalidArgument for a changed type", func(ctx SpecContext) {
			created := createGame(ctx)

			_, err := update(ctx, &ledgerv1.Game{
				Name: created.GetName(),
				Type: ledgerv1.Game_TYPE_GAME,
			}, "type")

			expectStatus(err, codes.InvalidArgument)
			Expect(err).To(HaveReason("IMMUTABLE_FIELD"))
			Expect(err).To(HaveViolatedField("game.type"))

			found, err := client.GetGame(ctx, &ledgerv1.GetGameRequest{Name: created.GetName()})
			Expect(err).ToNot(HaveOccurred())
			Expect(found.GetType()).To(Equal(created.GetType()))
		})

		It("returns a FailedPrecondition for a deleted game", func(ctx SpecContext) {
			name := seedDeletedGame(ctx)

			_, err := update(ctx, &ledgerv1.Game{
				Name:             name,
				NameTranslations: map[string]string{"en": "TNT Run"},
			}, "name_translations")

			expectStatus(err, codes.FailedPrecondition)
			Expect(err).To(HaveReason("GAME_DELETED"))
			Expect(err).To(HaveErrorMetadata("name", name))
		})

		It("returns a NotFound for a game that does not exist", func(ctx SpecContext) {
			name := "games/" + testkit.ResourceID("absent")

			_, err := update(ctx, &ledgerv1.Game{
				Name:             name,
				NameTranslations: map[string]string{"en": "TNT Run"},
			}, "name_translations")

			expectStatus(err, codes.NotFound)
			Expect(err).To(HaveReason("GAME_NOT_FOUND"))
			Expect(err).To(HaveErrorMetadata("name", name))
		})

		It("names update_mask on an InvalidArgument for a path outside the update paths",
			func(ctx SpecContext) {
				created := createGame(ctx)

				_, err := update(ctx, &ledgerv1.Game{Name: created.GetName()}, "create_time")

				expectStatus(err, codes.InvalidArgument)
				Expect(err).To(HaveReason("INVALID_UPDATE_MASK"))
				Expect(err).To(HaveViolatedField("update_mask"))
			})

		It("returns an InvalidArgument without a game", func(ctx SpecContext) {
			_, err := client.UpdateGame(ctx, &ledgerv1.UpdateGameRequest{})

			expectStatus(err, codes.InvalidArgument)
		})

		DescribeTable("names game.name on an InvalidArgument for a malformed name",
			func(ctx SpecContext, name string) {
				_, err := update(ctx, &ledgerv1.Game{
					Name:             name,
					NameTranslations: map[string]string{"en": "TNT Run"},
				}, "name_translations")

				expectStatus(err, codes.InvalidArgument)
				Expect(err).To(HaveViolatedField("game.name"))
			},
			Entry("empty", ""),
			Entry("of another collection", "users/spleef"),
			Entry("with an id outside the id pattern", "games/Spleef"),
		)

		DescribeTable("rejects selected fields that break their rules",
			func(ctx SpecContext, field string, game *ledgerv1.Game, paths ...string) {
				game.Name = createGame(ctx).GetName()

				_, err := update(ctx, game, paths...)

				expectStatus(err, codes.InvalidArgument)
				Expect(err).To(HaveViolatedField(field))
			},
			Entry("without an english name", "game.name_translations",
				&ledgerv1.Game{NameTranslations: map[string]string{"de": "Spleef"}}, "name_translations"),
			Entry("with an empty english name", "game.name_translations",
				&ledgerv1.Game{NameTranslations: map[string]string{"en": ""}}, "name_translations"),
			Entry("with a malformed language tag", `game.name_translations["EN"]`,
				&ledgerv1.Game{NameTranslations: map[string]string{"en": "Spleef", "EN": "Spleef"}},
				"name_translations"),
			Entry("with an unspecified type", "game.type",
				&ledgerv1.Game{Type: ledgerv1.Game_TYPE_UNSPECIFIED}, "type"),
			Entry("with an undefined type", "game.type",
				&ledgerv1.Game{Type: 99}, "type"),
		)

		Describe("with allow_missing", func() {
			upsert := func(ctx context.Context, game *ledgerv1.Game, paths ...string) (*ledgerv1.Game, error) {
				req := &ledgerv1.UpdateGameRequest{Game: game, AllowMissing: true}

				if len(paths) > 0 {
					req.UpdateMask = &fieldmaskpb.FieldMask{Paths: paths}
				}

				return client.UpdateGame(ctx, req)
			}

			It("creates a missing game under the id of its name and ignores the mask", func(ctx SpecContext) {
				name := "games/" + testkit.ResourceID("upserted")
				game := newGame()
				game.Name = name

				created, err := upsert(ctx, game, "name_translations")

				Expect(err).ToNot(HaveOccurred())
				Expect(created.GetName()).To(Equal(name))
				Expect(created.GetType()).To(Equal(ledgerv1.Game_TYPE_MINIGAME))
				Expect(created.GetNameTranslations()).To(Equal(game.GetNameTranslations()))
				Expect(created.GetCreateTime().AsTime()).To(BeTemporally("~", time.Now(), time.Minute))

				found, err := client.GetGame(ctx, &ledgerv1.GetGameRequest{Name: name})

				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeComparableTo(created, protocmp.Transform()))
			})

			It("updates an existing game under the mask", func(ctx SpecContext) {
				created := createGame(ctx)

				updated, err := upsert(ctx, &ledgerv1.Game{
					Name:             created.GetName(),
					NameTranslations: map[string]string{"en": "TNT Run"},
				}, "name_translations")

				Expect(err).ToNot(HaveOccurred())
				Expect(updated.GetType()).To(Equal(created.GetType()))
				Expect(updated.GetNameTranslations()).To(Equal(map[string]string{"en": "TNT Run"}))
				Expect(updated.GetCreateTime()).To(BeComparableTo(created.GetCreateTime(), protocmp.Transform()))
			})

			It("names game.type on an InvalidArgument for a changed type of an existing game",
				func(ctx SpecContext) {
					created := createGame(ctx)

					_, err := upsert(ctx, &ledgerv1.Game{
						Name: created.GetName(),
						Type: ledgerv1.Game_TYPE_GAME,
					}, "type")

					expectStatus(err, codes.InvalidArgument)
					Expect(err).To(HaveReason("IMMUTABLE_FIELD"))
				})

			It("returns a FailedPrecondition for a deleted game", func(ctx SpecContext) {
				name := seedDeletedGame(ctx)
				game := newGame()
				game.Name = name

				_, err := upsert(ctx, game)

				expectStatus(err, codes.FailedPrecondition)
				Expect(err).To(HaveReason("GAME_DELETED"))

				found, err := client.GetGame(ctx, &ledgerv1.GetGameRequest{Name: name})
				Expect(err).ToNot(HaveOccurred())
				Expect(found.GetDeleteTime()).ToNot(BeNil())
			})

			DescribeTable("rejects a missing game that breaks the create rules",
				func(ctx SpecContext, field string, mutate func(*ledgerv1.Game)) {
					name := "games/" + testkit.ResourceID("upserted")
					game := newGame()
					game.Name = name
					mutate(game)

					_, err := upsert(ctx, game, "name_translations")

					expectStatus(err, codes.InvalidArgument)
					Expect(err).To(HaveViolatedField(field))

					_, err = client.GetGame(ctx, &ledgerv1.GetGameRequest{Name: name})
					expectStatus(err, codes.NotFound)
				},
				Entry("without a type", "game.type", func(game *ledgerv1.Game) {
					game.Type = ledgerv1.Game_TYPE_UNSPECIFIED
				}),
				Entry("without an english name", "game.name_translations", func(game *ledgerv1.Game) {
					game.NameTranslations = map[string]string{"de": "Spleef"}
				}),
			)
		})
	})

	Describe("DeleteGame", func() {
		It("marks the game deleted and schedules its purge", func(ctx SpecContext) {
			created := createGame(ctx)

			deleted, err := client.DeleteGame(ctx, &ledgerv1.DeleteGameRequest{Name: created.GetName()})

			Expect(err).ToNot(HaveOccurred())
			Expect(deleted.GetName()).To(Equal(created.GetName()))
			Expect(deleted.GetDeleteTime().AsTime()).To(BeTemporally("~", time.Now(), time.Minute))
			Expect(deleted.GetPurgeTime().AsTime()).To(
				BeTemporally("~", deleted.GetDeleteTime().AsTime().Add(30*24*time.Hour), time.Second))
		})

		It("keeps the game for the get", func(ctx SpecContext) {
			created := createGame(ctx)

			deleted, err := client.DeleteGame(ctx, &ledgerv1.DeleteGameRequest{Name: created.GetName()})
			Expect(err).ToNot(HaveOccurred())

			found, err := client.GetGame(ctx, &ledgerv1.GetGameRequest{Name: created.GetName()})

			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeComparableTo(deleted, protocmp.Transform()))
		})

		It("leaves the game out of the list", func(ctx SpecContext) {
			created := createGame(ctx)

			_, err := client.DeleteGame(ctx, &ledgerv1.DeleteGameRequest{Name: created.GetName()})
			Expect(err).ToNot(HaveOccurred())

			Expect(listAll(ctx, &ledgerv1.ListGamesRequest{})).ToNot(ContainElement(created.GetName()))
		})

		It("returns a NotFound for a game already deleted", func(ctx SpecContext) {
			name := seedDeletedGame(ctx)

			_, err := client.DeleteGame(ctx, &ledgerv1.DeleteGameRequest{Name: name})

			expectStatus(err, codes.NotFound)
			Expect(err).To(HaveReason("GAME_ALREADY_DELETED"))
			Expect(err).To(HaveErrorMetadata("name", name))
		})

		It("returns a NotFound for a game that does not exist", func(ctx SpecContext) {
			name := "games/" + testkit.ResourceID("absent")

			_, err := client.DeleteGame(ctx, &ledgerv1.DeleteGameRequest{Name: name})

			expectStatus(err, codes.NotFound)
			Expect(err).To(HaveReason("GAME_NOT_FOUND"))
		})

		DescribeTable("names name on an InvalidArgument for a malformed name",
			func(ctx SpecContext, name string) {
				_, err := client.DeleteGame(ctx, &ledgerv1.DeleteGameRequest{Name: name})

				expectStatus(err, codes.InvalidArgument)
				Expect(err).To(HaveViolatedField("name"))
			},
			Entry("empty", ""),
			Entry("of another collection", "users/spleef"),
			Entry("with an id outside the id pattern", "games/Spleef"),
		)
	})

	Describe("UndeleteGame", func() {
		It("restores a deleted game", func(ctx SpecContext) {
			name := seedDeletedGame(ctx)

			restored, err := client.UndeleteGame(ctx, &ledgerv1.UndeleteGameRequest{Name: name})

			Expect(err).ToNot(HaveOccurred())
			Expect(restored.GetName()).To(Equal(name))
			Expect(restored.GetDeleteTime()).To(BeNil())
			Expect(restored.GetPurgeTime()).To(BeNil())
			Expect(listAll(ctx, &ledgerv1.ListGamesRequest{})).To(ContainElement(name))
		})

		It("restores the game a delete removed", func(ctx SpecContext) {
			created := createGame(ctx)

			_, err := client.DeleteGame(ctx, &ledgerv1.DeleteGameRequest{Name: created.GetName()})
			Expect(err).ToNot(HaveOccurred())

			restored, err := client.UndeleteGame(ctx, &ledgerv1.UndeleteGameRequest{Name: created.GetName()})

			Expect(err).ToNot(HaveOccurred())
			Expect(restored.GetType()).To(Equal(created.GetType()))
			Expect(restored.GetNameTranslations()).To(Equal(created.GetNameTranslations()))
			Expect(restored.GetCreateTime()).To(BeComparableTo(created.GetCreateTime(), protocmp.Transform()))
		})

		It("returns an AlreadyExists for a game that is not deleted", func(ctx SpecContext) {
			created := createGame(ctx)

			_, err := client.UndeleteGame(ctx, &ledgerv1.UndeleteGameRequest{Name: created.GetName()})

			expectStatus(err, codes.AlreadyExists)
			Expect(err).To(HaveReason("GAME_NOT_DELETED"))
			Expect(err).To(HaveErrorMetadata("name", created.GetName()))
		})

		It("returns a NotFound for a game that does not exist", func(ctx SpecContext) {
			name := "games/" + testkit.ResourceID("absent")

			_, err := client.UndeleteGame(ctx, &ledgerv1.UndeleteGameRequest{Name: name})

			expectStatus(err, codes.NotFound)
			Expect(err).To(HaveReason("GAME_NOT_FOUND"))
		})

		DescribeTable("names name on an InvalidArgument for a malformed name",
			func(ctx SpecContext, name string) {
				_, err := client.UndeleteGame(ctx, &ledgerv1.UndeleteGameRequest{Name: name})

				expectStatus(err, codes.InvalidArgument)
				Expect(err).To(HaveViolatedField("name"))
			},
			Entry("empty", ""),
			Entry("of another collection", "users/spleef"),
			Entry("with an id outside the id pattern", "games/Spleef"),
		)
	})
})
