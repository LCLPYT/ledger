package v1

import (
	ledgerv1 "github.com/lclpyt/ledger/gen/go/apis/ledger/v1"
	"github.com/lclpyt/ledger/internal/auth"
)

const (
	PermissionGamesRead           = "games.read"
	PermissionGamesWrite          = "games.write"
	PermissionPlayerSettingsRead  = "player_settings.read"
	PermissionPlayerSettingsWrite = "player_settings.write"
)

// TODO remove this map and replace with method attribute if possible
var MethodPermissions = auth.MethodPermissions{
	ledgerv1.GamesService_GetGame_FullMethodName:      PermissionGamesRead,
	ledgerv1.GamesService_ListGames_FullMethodName:    PermissionGamesRead,
	ledgerv1.GamesService_CreateGame_FullMethodName:   PermissionGamesWrite,
	ledgerv1.GamesService_UpdateGame_FullMethodName:   PermissionGamesWrite,
	ledgerv1.GamesService_DeleteGame_FullMethodName:   PermissionGamesWrite,
	ledgerv1.GamesService_UndeleteGame_FullMethodName: PermissionGamesWrite,

	ledgerv1.PlayerSettingsService_GetPlayerSetting_FullMethodName:    PermissionPlayerSettingsRead,
	ledgerv1.PlayerSettingsService_ListPlayerSettings_FullMethodName:  PermissionPlayerSettingsRead,
	ledgerv1.PlayerSettingsService_UpdatePlayerSetting_FullMethodName: PermissionPlayerSettingsWrite,
	ledgerv1.PlayerSettingsService_DeletePlayerSetting_FullMethodName: PermissionPlayerSettingsWrite,
}
