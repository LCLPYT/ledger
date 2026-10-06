package v1

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	ledgerv1 "github.com/lclpyt/ledger/gen/go/apis/ledger/v1"
	"github.com/lclpyt/ledger/gen/go/bob/model"
	"github.com/lclpyt/ledger/internal/playersettings"
	"github.com/quadrubo/golib/apperr"
	"github.com/quadrubo/golib/protoconv"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/structpb"
)

const maxSettingValueSize = 64 * 1024

func newPlayerSettingsApi(playerSettingsService *playersettings.Service) *playerSettingsApi {
	return &playerSettingsApi{
		playerSettingsService: playerSettingsService,
	}
}

type playerSettingsApi struct {
	ledgerv1.UnimplementedPlayerSettingsServiceServer

	playerSettingsService *playersettings.Service
}

func (p *playerSettingsApi) GetPlayerSetting(
	ctx context.Context,
	request *ledgerv1.GetPlayerSettingRequest,
) (*ledgerv1.PlayerSetting, error) {
	playerID, path, err := playersettings.ParseName(request.GetName())

	if err != nil {
		return nil, apperr.InvalidResourceName("name", err, request.GetName())
	}

	setting, err := p.playerSettingsService.Get(ctx, playerID, path)

	if err != nil {
		return nil, fmt.Errorf("player settings: failed to get setting %q: %w", request.GetName(), err)
	}

	return playerSettingToProto(setting)
}

func (p *playerSettingsApi) ListPlayerSettings(
	ctx context.Context,
	request *ledgerv1.ListPlayerSettingsRequest,
) (*ledgerv1.ListPlayerSettingsResponse, error) {
	playerID, err := playersettings.ParseParent(request.GetParent())

	if err != nil {
		return nil, apperr.InvalidParent(err, request.GetParent())
	}

	order, err := playersettings.Paging.ParseOrderBy("")

	if err != nil {
		return nil, fmt.Errorf("player settings: failed to parse order: %w", err)
	}

	scope := []string{request.GetParent(), request.GetPathPrefix()}

	page, err := order.Page(int(request.GetPageSize()), request.GetPageToken(), scope)

	if err != nil {
		return nil, apperr.InvalidPageToken(err, request.GetPageToken())
	}

	rows, nextPageToken, err := p.playerSettingsService.List(ctx, playerID, request.GetPathPrefix(), page)

	if err != nil {
		return nil, fmt.Errorf("player settings: failed to list settings: %w", err)
	}

	settings := make([]*ledgerv1.PlayerSetting, 0, len(rows))

	for _, row := range rows {
		setting, err := playerSettingToProto(row)

		if err != nil {
			return nil, err
		}

		settings = append(settings, setting)
	}

	return &ledgerv1.ListPlayerSettingsResponse{
		PlayerSettings: settings,
		NextPageToken:  nextPageToken,
	}, nil
}

func (p *playerSettingsApi) UpdatePlayerSetting(
	ctx context.Context,
	request *ledgerv1.UpdatePlayerSettingRequest,
) (*ledgerv1.PlayerSetting, error) {
	name := request.GetPlayerSetting().GetName()
	playerID, path, err := playersettings.ParseName(name)

	if err != nil {
		return nil, apperr.InvalidResourceName("player_setting.name", err, name)
	}

	value, err := encodeSettingValue(request.GetPlayerSetting().GetValue())

	if err != nil {
		return nil, apperr.InvalidField("INVALID_SETTING_VALUE", "player_setting.value", "name", name, err)
	}

	setting, err := p.playerSettingsService.Upsert(ctx, playerID, path, value)

	if err != nil {
		return nil, fmt.Errorf("player settings: failed to update setting %q: %w", name, err)
	}

	return playerSettingToProto(setting)
}

func (p *playerSettingsApi) DeletePlayerSetting(
	ctx context.Context,
	request *ledgerv1.DeletePlayerSettingRequest,
) (*emptypb.Empty, error) {
	playerID, path, err := playersettings.ParseName(request.GetName())

	if err != nil {
		return nil, apperr.InvalidResourceName("name", err, request.GetName())
	}

	if err := p.playerSettingsService.Delete(ctx, playerID, path); err != nil {
		return nil, fmt.Errorf("player settings: failed to delete setting %q: %w", request.GetName(), err)
	}

	return &emptypb.Empty{}, nil
}

func encodeSettingValue(value *structpb.Value) (json.RawMessage, error) {
	if _, ok := value.GetKind().(*structpb.Value_NullValue); ok || value.GetKind() == nil {
		return nil, errors.New("value must not be null, delete the setting instead")
	}

	encoded, err := protojson.Marshal(value)

	if err != nil {
		return nil, fmt.Errorf("value cannot be encoded as json: %w", err)
	}

	if len(encoded) > maxSettingValueSize {
		return nil, fmt.Errorf("value exceeds %d bytes when encoded as json", maxSettingValueSize)
	}

	return encoded, nil
}

func playerSettingToProto(row *model.PlayerSetting) (*ledgerv1.PlayerSetting, error) {
	value := &structpb.Value{}

	if err := protojson.Unmarshal(row.Value.Val, value); err != nil {
		return nil, fmt.Errorf("player settings: failed to decode value of %q: %w", row.Path, err)
	}

	return &ledgerv1.PlayerSetting{
		Name:       playersettings.FormatName(row.PlayerID, row.Path),
		Value:      value,
		CreateTime: protoconv.TimeToProto(row.CreateTime),
		UpdateTime: protoconv.TimeToProto(row.UpdateTime),
	}, nil
}
