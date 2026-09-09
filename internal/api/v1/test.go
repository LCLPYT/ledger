package v1

import (
	"context"
	ledgerv1 "ledger/gen/go/apis/ledger/v1"
)

func newTestAPI() *testAPI {
	return &testAPI{}
}

type testAPI struct {
	ledgerv1.UnimplementedTestServiceServer
}

func (t testAPI) Test(_ context.Context, request *ledgerv1.TestRequest) (*ledgerv1.TestResponse, error) {
	return &ledgerv1.TestResponse{
		Text: request.Msg + " - works",
	}, nil
}
