package v1

import (
	"context"
	"fmt"

	ledgerv1 "github.com/lclpyt/ledger/gen/go/apis/ledger/v1"

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
		return fmt.Errorf("apiv1: failed to invoke the server: %w", err)
	}

	ledgerv1.RegisterTestServiceServer(server, newTestAPI())

	return nil
}
