package wiring

import (
	"github.com/quadrubo/golib/app"
	"github.com/quadrubo/golib/grpcerr"
	"github.com/quadrubo/golib/grpcinterceptor/errmap"
	"github.com/quadrubo/golib/grpcinterceptor/recovery"
	"github.com/quadrubo/golib/grpcinterceptor/validate"
	"github.com/quadrubo/golib/grpcserver"
	"github.com/quadrubo/golib/logging"
)

func Modules(configModule app.Module) []app.Module {
	return []app.Module{
		configModule,
		logging.Module(),
		grpcerr.Module(),
		validate.Module(),
		grpcserver.Module(grpcserver.WithUnaryInterceptors(
			recovery.Unary,
			errmap.Unary,
			validate.Unary,
		)),
	}
}
