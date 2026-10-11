package auth

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/quadrubo/golib/grpcerr"
	"github.com/quadrubo/golib/grpcserver"
	"github.com/quadrubo/golib/logging"
	"github.com/samber/do/v2"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

// Public marks a method that needs no credentials.
const Public = ""

const healthServicePrefix = "/grpc.health.v1.Health/"

// MethodPermissions maps a full gRPC method name to the permission a call needs.
// A method missing from the map is denied for every caller.
type MethodPermissions map[string]string

var (
	errMissingCredentials = grpcerr.Unauthenticated(
		"MISSING_CREDENTIALS",
		"the call carries no bearer token",
	)
	errInvalidCredentials = grpcerr.Unauthenticated(
		"INVALID_CREDENTIALS",
		"the bearer token is invalid, expired or revoked",
	)
)

// gRPC reflection is unaffected by auth currently.
// It is intended to be turned off in prod.

// Unary returns an interceptor that authenticates a call and checks the permission its method requires.
// The permissions are loaded once while the interceptor is created, failing it on an invalid definition.
func Unary(loadPermissions func() (MethodPermissions, error)) grpcserver.UnaryInterceptor {
	return func(i do.Injector) (grpc.UnaryServerInterceptor, error) {
		permissions, err := loadPermissions()

		if err != nil {
			return nil, fmt.Errorf("auth: failed to load method permissions: %w", err)
		}

		log, err := logging.Component(i, "auth")

		if err != nil {
			return nil, fmt.Errorf("auth: failed to inject logger: %w", err)
		}

		authenticator, err := do.Invoke[*Authenticator](i)

		if err != nil {
			return nil, fmt.Errorf("auth: failed to inject the authenticator: %w", err)
		}

		return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, next grpc.UnaryHandler) (any, error) {
			if strings.HasPrefix(info.FullMethod, healthServicePrefix) {
				return next(ctx, req)
			}

			permission, ok := permissions[info.FullMethod]

			if !ok {
				log.ErrorContext(ctx, "method has no permission mapped", slog.String("method", info.FullMethod))

				return nil, grpcerr.PermissionDenied(
					"METHOD_NOT_PERMITTED",
					"the method {method} is not available", grpcerr.Arg("method", info.FullMethod),
				)
			}

			if permission == Public {
				return next(ctx, req)
			}

			token, ok := tokenFromMetadata(ctx)

			if !ok {
				return nil, errMissingCredentials
			}

			principal, err := authenticator.Authenticate(ctx, token)

			if errors.Is(err, ErrInvalidToken) {
				return nil, errInvalidCredentials
			}

			if err != nil {
				return nil, err
			}

			if !principal.Can(permission) {
				return nil, grpcerr.PermissionDenied(
					"MISSING_PERMISSION",
					"the caller lacks the permission {permission}",
					grpcerr.Arg("permission", permission),
				)
			}

			return next(NewContext(ctx, principal), req)
		}, nil
	}
}

func tokenFromMetadata(ctx context.Context) (string, bool) {
	md, ok := metadata.FromIncomingContext(ctx)

	if !ok {
		return "", false
	}

	values := md.Get("authorization")

	if len(values) != 1 {
		return "", false
	}

	return bearerToken(values[0])
}

func bearerToken(header string) (string, bool) {
	scheme, token, ok := strings.Cut(header, " ")

	if !ok || !strings.EqualFold(scheme, "Bearer") {
		return "", false
	}

	token = strings.TrimSpace(token)

	return token, token != ""
}
