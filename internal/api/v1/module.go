package v1

import (
	"context"
	"fmt"

	"buf.build/go/protovalidate"
	ledgerv1 "github.com/lclpyt/ledger/gen/go/apis/ledger/v1"
	"github.com/lclpyt/ledger/internal/games"
	"github.com/stephenafamo/bob"

	"github.com/quadrubo/golib/app"
	"github.com/samber/do/v2"
	"google.golang.org/grpc"
)

func Module() app.Module {
	return &module{}
}

type module struct{}

var _ app.Provider = (*module)(nil)

func (m *module) Name() string { return "apiv1" }

func (m *module) Provide(_ context.Context, i do.Injector) error {
	server, err := do.Invoke[*grpc.Server](i)

	if err != nil {
		return fmt.Errorf("apiv1: failed to inject the server: %w", err)
	}

	db, err := do.Invoke[bob.DB](i)

	if err != nil {
		return fmt.Errorf("apiv1: failed to inject the db: %w", err)
	}

	validator, err := do.Invoke[protovalidate.Validator](i)

	if err != nil {
		return fmt.Errorf("apiv1: failed to inject the validator: %w", err)
	}

	gamesService := games.NewService(db)

	ledgerv1.RegisterTestServiceServer(server, newTestAPI())
	ledgerv1.RegisterGamesServiceServer(server, newGamesApi(gamesService, validator))

	return nil
}
