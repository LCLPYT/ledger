package v1_test

import (
	"context"
	"strings"
	"time"

	"github.com/gofrs/uuid/v5"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/quadrubo/golib/testkit/matchers"
	"google.golang.org/grpc/codes"
	"google.golang.org/protobuf/testing/protocmp"
	"google.golang.org/protobuf/types/known/structpb"

	ledgerv1 "github.com/lclpyt/ledger/gen/go/apis/ledger/v1"
)

func newPlayer() string {
	GinkgoHelper()

	id, err := uuid.NewV4()
	Expect(err).ToNot(HaveOccurred())

	return "players/" + id.String()
}

func settingName(player, path string) string {
	return player + "/settings/" + path
}

func putSetting(ctx context.Context, name string, value *structpb.Value) (*ledgerv1.PlayerSetting, error) {
	return playerSettingsClient.UpdatePlayerSetting(ctx, &ledgerv1.UpdatePlayerSettingRequest{
		PlayerSetting: &ledgerv1.PlayerSetting{Name: name, Value: value},
	})
}

func mustPutSetting(ctx context.Context, name string, value *structpb.Value) *ledgerv1.PlayerSetting {
	GinkgoHelper()

	setting, err := putSetting(ctx, name, value)
	Expect(err).ToNot(HaveOccurred())

	return setting
}

func listAllSettings(ctx context.Context, req *ledgerv1.ListPlayerSettingsRequest) []string {
	GinkgoHelper()

	var names []string

	for {
		resp, err := playerSettingsClient.ListPlayerSettings(ctx, req)
		Expect(err).ToNot(HaveOccurred())

		if req.GetPageSize() > 0 {
			Expect(len(resp.GetPlayerSettings())).To(BeNumerically("<=", req.GetPageSize()))
		}

		for _, setting := range resp.GetPlayerSettings() {
			names = append(names, setting.GetName())
		}

		if resp.GetNextPageToken() == "" {
			return names
		}

		req.PageToken = resp.GetNextPageToken()
	}
}

var _ = Describe("PlayerSettingsService", func() {
	Describe("UpdatePlayerSetting", func() {
		It("creates a setting that does not exist yet", func(ctx SpecContext) {
			name := settingName(newPlayer(), "general.preferred_language")

			setting := mustPutSetting(ctx, name, structpb.NewStringValue("de"))

			Expect(setting.GetName()).To(Equal(name))
			Expect(setting.GetValue().GetStringValue()).To(Equal("de"))
			Expect(setting.GetCreateTime().AsTime()).To(BeTemporally("~", time.Now(), time.Minute))
			Expect(setting.GetUpdateTime().AsTime()).To(BeTemporally("~", time.Now(), time.Minute))
		})

		It("replaces the value of an existing setting", func(ctx SpecContext) {
			name := settingName(newPlayer(), "ap2.preferred_color")
			created := mustPutSetting(ctx, name, structpb.NewStringValue("red"))

			updated := mustPutSetting(ctx, name, structpb.NewStringValue("blue"))

			Expect(updated.GetValue().GetStringValue()).To(Equal("blue"))
			Expect(updated.GetCreateTime().AsTime()).To(Equal(created.GetCreateTime().AsTime()))
			Expect(updated.GetUpdateTime().AsTime()).To(BeTemporally(">", created.GetUpdateTime().AsTime()))
		})

		DescribeTable("stores every kind of value",
			func(ctx SpecContext, value any) {
				v, err := structpb.NewValue(value)
				Expect(err).ToNot(HaveOccurred())
				name := settingName(newPlayer(), "game.ap2.pvp_tournament.kit.soup.item_organization")

				stored := mustPutSetting(ctx, name, v)
				found, err := playerSettingsClient.GetPlayerSetting(ctx, &ledgerv1.GetPlayerSettingRequest{Name: name})

				Expect(err).ToNot(HaveOccurred())
				Expect(stored.GetValue()).To(BeComparableTo(v, protocmp.Transform()))
				Expect(found).To(BeComparableTo(stored, protocmp.Transform()))
			},
			Entry("string", "en"),
			Entry("number", 1.5),
			Entry("bool", true),
			Entry("list", []any{"sword", 3.0, false}),
			Entry("struct", map[string]any{"slots": []any{"soup", "sword"}, "nested": map[string]any{"a": 1.0}}),
			Entry("empty struct", map[string]any{}),
		)

		DescribeTable("rejects a path overlapping an existing setting",
			func(ctx SpecContext, existing, path string) {
				player := newPlayer()
				mustPutSetting(ctx, settingName(player, existing), structpb.NewBoolValue(true))

				_, err := putSetting(ctx, settingName(player, path), structpb.NewBoolValue(true))

				expectStatus(err, codes.FailedPrecondition)
				Expect(err).To(HaveReason("PLAYER_SETTING_PATH_CONFLICT"))
				Expect(err).To(HaveErrorMetadata("existing", settingName(player, existing)))
			},
			Entry("below an existing setting", "ap2", "ap2.preferred_color"),
			Entry("deep below an existing setting", "ap2", "ap2.kit.soup.slots"),
			Entry("above an existing setting", "ap2.preferred_color", "ap2"),
			Entry("far above an existing setting", "game.ap2.pvp", "game"),
		)

		DescribeTable("accepts a path next to an existing setting",
			func(ctx SpecContext, existing, path string) {
				player := newPlayer()
				mustPutSetting(ctx, settingName(player, existing), structpb.NewBoolValue(true))

				_, err := putSetting(ctx, settingName(player, path), structpb.NewBoolValue(true))

				Expect(err).ToNot(HaveOccurred())
			},
			Entry("sibling", "ap2.a", "ap2.b"),
			Entry("sharing a text prefix", "ap2", "ap2x"),
			Entry("sharing a text prefix with an underscore", "ap2", "ap2_x"),
			Entry("below a text prefix lookalike", "ap2x", "ap2.color"),
		)

		It("does not see the settings of another player as overlapping", func(ctx SpecContext) {
			mustPutSetting(ctx, settingName(newPlayer(), "ap2"), structpb.NewBoolValue(true))

			_, err := putSetting(ctx, settingName(newPlayer(), "ap2.preferred_color"), structpb.NewBoolValue(true))

			Expect(err).ToNot(HaveOccurred())
		})

		DescribeTable("rejects an invalid request",
			func(ctx SpecContext, field string, mutate func(*ledgerv1.UpdatePlayerSettingRequest)) {
				req := &ledgerv1.UpdatePlayerSettingRequest{
					PlayerSetting: &ledgerv1.PlayerSetting{
						Name:  settingName(newPlayer(), "general.preferred_language"),
						Value: structpb.NewStringValue("en"),
					},
				}
				mutate(req)

				_, err := playerSettingsClient.UpdatePlayerSetting(ctx, req)

				expectStatus(err, codes.InvalidArgument)
				Expect(err).To(HaveViolatedField(field))
			},
			Entry("without a setting", "player_setting", func(req *ledgerv1.UpdatePlayerSettingRequest) {
				req.PlayerSetting = nil
			}),
			Entry("without a value", "player_setting.value", func(req *ledgerv1.UpdatePlayerSettingRequest) {
				req.PlayerSetting.Value = nil
			}),
			Entry("with a null value", "player_setting.value", func(req *ledgerv1.UpdatePlayerSettingRequest) {
				req.PlayerSetting.Value = structpb.NewNullValue()
			}),
			Entry("with an oversized value", "player_setting.value", func(req *ledgerv1.UpdatePlayerSettingRequest) {
				req.PlayerSetting.Value = structpb.NewStringValue(strings.Repeat("a", 70*1024))
			}),
			Entry("without a name", "player_setting.name", func(req *ledgerv1.UpdatePlayerSettingRequest) {
				req.PlayerSetting.Name = ""
			}),
			Entry("with an uppercase player id", "player_setting.name", func(req *ledgerv1.UpdatePlayerSettingRequest) {
				req.PlayerSetting.Name = strings.ToUpper(newPlayer()[len("players/"):])
				req.PlayerSetting.Name = settingName("players/"+req.PlayerSetting.Name, "ap2")
			}),
			Entry("with a player id of another uuid version", "player_setting.name",
				func(req *ledgerv1.UpdatePlayerSettingRequest) {
					req.PlayerSetting.Name = settingName("players/0190a7c2-5b1e-7a3c-9d4e-123456789abc", "ap2")
				}),
			Entry("with an uppercase path", "player_setting.name", func(req *ledgerv1.UpdatePlayerSettingRequest) {
				req.PlayerSetting.Name = settingName(newPlayer(), "General")
			}),
			Entry("with an empty path segment", "player_setting.name", func(req *ledgerv1.UpdatePlayerSettingRequest) {
				req.PlayerSetting.Name = settingName(newPlayer(), "a..b")
			}),
			Entry("with a trailing dot", "player_setting.name", func(req *ledgerv1.UpdatePlayerSettingRequest) {
				req.PlayerSetting.Name = settingName(newPlayer(), "a.")
			}),
			Entry("with a path segment starting with a digit", "player_setting.name",
				func(req *ledgerv1.UpdatePlayerSettingRequest) {
					req.PlayerSetting.Name = settingName(newPlayer(), "ap2.1st")
				}),
			Entry("with a path deeper than 16 segments", "player_setting.name",
				func(req *ledgerv1.UpdatePlayerSettingRequest) {
					req.PlayerSetting.Name = settingName(newPlayer(), strings.Repeat("a.", 16)+"a")
				}),
			Entry("with a path longer than 255 characters", "player_setting.name",
				func(req *ledgerv1.UpdatePlayerSettingRequest) {
					segment := strings.Repeat("a", 60)
					req.PlayerSetting.Name = settingName(newPlayer(), strings.Join([]string{segment, segment, segment, segment, segment}, "."))
				}),
		)
	})

	Describe("GetPlayerSetting", func() {
		It("returns a NotFound for a setting that does not exist", func(ctx SpecContext) {
			name := settingName(newPlayer(), "general.preferred_language")

			_, err := playerSettingsClient.GetPlayerSetting(ctx, &ledgerv1.GetPlayerSettingRequest{Name: name})

			expectStatus(err, codes.NotFound)
			Expect(err).To(HaveReason("PLAYER_SETTING_NOT_FOUND"))
			Expect(err).To(HaveErrorMetadata("name", name))
		})

		It("returns a NotFound for an ancestor path of a setting", func(ctx SpecContext) {
			player := newPlayer()
			mustPutSetting(ctx, settingName(player, "ap2.preferred_color"), structpb.NewStringValue("red"))

			_, err := playerSettingsClient.GetPlayerSetting(ctx,
				&ledgerv1.GetPlayerSettingRequest{Name: settingName(player, "ap2")})

			expectStatus(err, codes.NotFound)
		})

		DescribeTable("names name on an InvalidArgument for a malformed name",
			func(ctx SpecContext, name string) {
				_, err := playerSettingsClient.GetPlayerSetting(ctx, &ledgerv1.GetPlayerSettingRequest{Name: name})

				expectStatus(err, codes.InvalidArgument)
				Expect(err).To(HaveViolatedField("name"))
			},
			Entry("empty", ""),
			Entry("of another collection", "games/spleef"),
			Entry("without a path", "players/9b2d5d43-62a4-4b8e-8a8d-1f5d0e1f2a3b/settings/"),
			Entry("with a malformed player id", "players/steve/settings/ap2"),
			Entry("with a trailing slash segment", "players/9b2d5d43-62a4-4b8e-8a8d-1f5d0e1f2a3b/settings/ap2/x"),
		)
	})

	Describe("DeletePlayerSetting", func() {
		It("removes the setting", func(ctx SpecContext) {
			name := settingName(newPlayer(), "ap2.preferred_color")
			mustPutSetting(ctx, name, structpb.NewStringValue("red"))

			_, err := playerSettingsClient.DeletePlayerSetting(ctx, &ledgerv1.DeletePlayerSettingRequest{Name: name})
			Expect(err).ToNot(HaveOccurred())

			_, err = playerSettingsClient.GetPlayerSetting(ctx, &ledgerv1.GetPlayerSettingRequest{Name: name})
			expectStatus(err, codes.NotFound)
		})

		It("frees the path for a descendant setting", func(ctx SpecContext) {
			player := newPlayer()
			mustPutSetting(ctx, settingName(player, "ap2"), structpb.NewStringValue("red"))

			_, err := playerSettingsClient.DeletePlayerSetting(ctx,
				&ledgerv1.DeletePlayerSettingRequest{Name: settingName(player, "ap2")})
			Expect(err).ToNot(HaveOccurred())

			_, err = putSetting(ctx, settingName(player, "ap2.preferred_color"), structpb.NewStringValue("red"))
			Expect(err).ToNot(HaveOccurred())
		})

		It("returns a NotFound for a setting that does not exist", func(ctx SpecContext) {
			name := settingName(newPlayer(), "ap2.preferred_color")

			_, err := playerSettingsClient.DeletePlayerSetting(ctx, &ledgerv1.DeletePlayerSettingRequest{Name: name})

			expectStatus(err, codes.NotFound)
			Expect(err).To(HaveReason("PLAYER_SETTING_NOT_FOUND"))
		})
	})

	Describe("ListPlayerSettings", func() {
		It("returns the settings of the player ordered by path", func(ctx SpecContext) {
			player := newPlayer()
			for _, path := range []string{"general.preferred_language", "ap2.preferred_color", "game.ap2.pvp"} {
				mustPutSetting(ctx, settingName(player, path), structpb.NewBoolValue(true))
			}
			mustPutSetting(ctx, settingName(newPlayer(), "ap2.preferred_color"), structpb.NewBoolValue(true))

			names := listAllSettings(ctx, &ledgerv1.ListPlayerSettingsRequest{Parent: player})

			Expect(names).To(Equal([]string{
				settingName(player, "ap2.preferred_color"),
				settingName(player, "game.ap2.pvp"),
				settingName(player, "general.preferred_language"),
			}))
		})

		It("returns the settings the get returns", func(ctx SpecContext) {
			player := newPlayer()
			created := mustPutSetting(ctx, settingName(player, "ap2.kit"), structpb.NewListValue(&structpb.ListValue{
				Values: []*structpb.Value{structpb.NewStringValue("soup")},
			}))

			resp, err := playerSettingsClient.ListPlayerSettings(ctx, &ledgerv1.ListPlayerSettingsRequest{Parent: player})

			Expect(err).ToNot(HaveOccurred())
			Expect(resp.GetPlayerSettings()).To(ConsistOf(BeComparableTo(created, protocmp.Transform())))
		})

		It("walks the settings by keyset without repeating one", func(ctx SpecContext) {
			player := newPlayer()
			for _, path := range []string{"a", "b", "c", "d", "e"} {
				mustPutSetting(ctx, settingName(player, path), structpb.NewBoolValue(true))
			}

			paged := listAllSettings(ctx, &ledgerv1.ListPlayerSettingsRequest{Parent: player, PageSize: 2})
			unpaged := listAllSettings(ctx, &ledgerv1.ListPlayerSettingsRequest{Parent: player})

			Expect(paged).To(HaveLen(5))
			Expect(paged).To(Equal(unpaged))
		})

		It("restricts the result to the subtree of path_prefix", func(ctx SpecContext) {
			player := newPlayer()
			for _, path := range []string{"ap2.color", "ap2.kit.soup", "ap2x", "ap2_x", "game.ap2.pvp"} {
				mustPutSetting(ctx, settingName(player, path), structpb.NewBoolValue(true))
			}

			names := listAllSettings(ctx, &ledgerv1.ListPlayerSettingsRequest{Parent: player, PathPrefix: "ap2"})

			Expect(names).To(Equal([]string{settingName(player, "ap2.color"), settingName(player, "ap2.kit.soup")}))
		})

		It("matches the setting at path_prefix itself", func(ctx SpecContext) {
			player := newPlayer()
			mustPutSetting(ctx, settingName(player, "ap2.color"), structpb.NewBoolValue(true))
			mustPutSetting(ctx, settingName(player, "ap2.colors"), structpb.NewBoolValue(true))

			names := listAllSettings(ctx, &ledgerv1.ListPlayerSettingsRequest{Parent: player, PathPrefix: "ap2.color"})

			Expect(names).To(Equal([]string{settingName(player, "ap2.color")}))
		})

		It("rejects a token under another path_prefix", func(ctx SpecContext) {
			player := newPlayer()
			mustPutSetting(ctx, settingName(player, "ap2.a"), structpb.NewBoolValue(true))
			mustPutSetting(ctx, settingName(player, "ap2.b"), structpb.NewBoolValue(true))

			first, err := playerSettingsClient.ListPlayerSettings(ctx,
				&ledgerv1.ListPlayerSettingsRequest{Parent: player, PageSize: 1})
			Expect(err).ToNot(HaveOccurred())

			_, err = playerSettingsClient.ListPlayerSettings(ctx, &ledgerv1.ListPlayerSettingsRequest{
				Parent:     player,
				PathPrefix: "ap2",
				PageToken:  first.GetNextPageToken(),
			})

			expectStatus(err, codes.InvalidArgument)
			Expect(err).To(HaveViolatedField("page_token"))
		})

		DescribeTable("rejects an invalid request",
			func(ctx SpecContext, field string, req *ledgerv1.ListPlayerSettingsRequest) {
				_, err := playerSettingsClient.ListPlayerSettings(ctx, req)

				expectStatus(err, codes.InvalidArgument)
				Expect(err).To(HaveViolatedField(field))
			},
			Entry("without a parent", "parent", &ledgerv1.ListPlayerSettingsRequest{}),
			Entry("with a malformed parent", "parent", &ledgerv1.ListPlayerSettingsRequest{Parent: "players/steve"}),
			Entry("with a negative page_size", "page_size", &ledgerv1.ListPlayerSettingsRequest{
				Parent:   "players/9b2d5d43-62a4-4b8e-8a8d-1f5d0e1f2a3b",
				PageSize: -1,
			}),
			Entry("with a malformed path_prefix", "path_prefix", &ledgerv1.ListPlayerSettingsRequest{
				Parent:     "players/9b2d5d43-62a4-4b8e-8a8d-1f5d0e1f2a3b",
				PathPrefix: "ap2.",
			}),
		)
	})
})
