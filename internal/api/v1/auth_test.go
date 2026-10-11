package v1_test

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"time"

	"github.com/aarondl/opt/omit"
	"github.com/aarondl/opt/omitnull"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/quadrubo/golib/testkit"
	. "github.com/quadrubo/golib/testkit/matchers"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/status"

	ledgerv1 "github.com/lclpyt/ledger/gen/go/apis/ledger/v1"
	"github.com/lclpyt/ledger/gen/go/bob/model"
	apiv1 "github.com/lclpyt/ledger/internal/api/v1"
	"github.com/lclpyt/ledger/internal/auth"
)

type bearerCredentials string

func (c bearerCredentials) GetRequestMetadata(context.Context, ...string) (map[string]string, error) {
	return map[string]string{"authorization": "Bearer " + string(c)}, nil
}

func (bearerCredentials) RequireTransportSecurity() bool { return false }

func authedConn(token string) *grpc.ClientConn {
	GinkgoHelper()

	conn, err := grpc.NewClient(suite.Addr(),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithPerRPCCredentials(bearerCredentials(token)))
	Expect(err).ToNot(HaveOccurred())
	DeferCleanup(conn.Close)

	return conn
}

var roleCounter atomic.Int64

// createAPIKey stores a key holding one role with the permissions and returns its token.
func createAPIKey(ctx context.Context, permissions ...string) string {
	GinkgoHelper()

	return createAPIKeyWith(ctx, &model.APIKeySetter{}, permissions...)
}

func createAPIKeyWith(ctx context.Context, setter *model.APIKeySetter, permissions ...string) string {
	GinkgoHelper()

	token, id, hash, err := auth.NewToken()
	Expect(err).ToNot(HaveOccurred())

	role := fmt.Sprintf("role_%d", roleCounter.Add(1))

	_, err = model.Roles.Insert(&model.RoleSetter{Name: omit.From(role)}).One(ctx, db)
	Expect(err).ToNot(HaveOccurred())

	for _, permission := range permissions {
		_, err = model.RolePermissions.Insert(&model.RolePermissionSetter{
			RoleName:   omit.From(role),
			Permission: omit.From(permission),
		}).One(ctx, db)
		Expect(err).ToNot(HaveOccurred())
	}

	setter.ID = omit.From(id)
	setter.Name = omit.From(role + "_key")
	setter.SecretHash = omit.From(hash)

	_, err = model.APIKeys.Insert(setter).One(ctx, db)
	Expect(err).ToNot(HaveOccurred())

	_, err = model.APIKeyRoles.Insert(&model.APIKeyRoleSetter{
		APIKeyID: omit.From(id),
		RoleName: omit.From(role),
	}).One(ctx, db)
	Expect(err).ToNot(HaveOccurred())

	return token
}

func listGamesWith(ctx context.Context, conn *grpc.ClientConn) error {
	_, err := ledgerv1.NewGamesServiceClient(conn).ListGames(ctx, &ledgerv1.ListGamesRequest{})

	return err
}

var _ = Describe("Authentication", func() {
	Describe("gRPC", func() {
		It("covers every method the services declare", func() {
			permissions, err := apiv1.MethodPermissions()
			Expect(err).ToNot(HaveOccurred())

			for _, desc := range []grpc.ServiceDesc{
				ledgerv1.GamesService_ServiceDesc,
				ledgerv1.PlayerSettingsService_ServiceDesc,
			} {
				Expect(desc.Streams).To(BeEmpty(), "streams bypass the unary auth interceptor")

				for _, method := range desc.Methods {
					Expect(permissions).To(
						HaveKey(fmt.Sprintf("/%s/%s", desc.ServiceName, method.MethodName)))
				}
			}
		})

		It("rejects a call without credentials", func(ctx SpecContext) {
			err := listGamesWith(ctx, suite.Conn())

			Expect(status.Code(err)).To(Equal(codes.Unauthenticated))
			Expect(err).To(HaveReason("MISSING_CREDENTIALS"))
			Expect(err).To(HaveErrorDomain(errorDomain))
		})

		It("rejects a malformed token", func(ctx SpecContext) {
			err := listGamesWith(ctx, authedConn("not-a-token"))

			Expect(status.Code(err)).To(Equal(codes.Unauthenticated))
			Expect(err).To(HaveReason("INVALID_CREDENTIALS"))
		})

		It("rejects an unknown key", func(ctx SpecContext) {
			token, _, _, err := auth.NewToken()
			Expect(err).ToNot(HaveOccurred())

			err = listGamesWith(ctx, authedConn(token))

			Expect(err).To(HaveReason("INVALID_CREDENTIALS"))
		})

		It("rejects a known key with a wrong secret", func(ctx SpecContext) {
			token := createAPIKey(ctx, "*")
			Expect(listGamesWith(ctx, authedConn(token))).To(Succeed())

			wrong := token[:len(token)-1] + string(token[len(token)-1]^1)

			err := listGamesWith(ctx, authedConn(wrong))

			Expect(err).To(HaveReason("INVALID_CREDENTIALS"))
		})

		It("rejects a revoked key", func(ctx SpecContext) {
			token := createAPIKeyWith(ctx, &model.APIKeySetter{RevokeTime: omitnull.From(time.Now())}, "*")

			err := listGamesWith(ctx, authedConn(token))

			Expect(err).To(HaveReason("INVALID_CREDENTIALS"))
		})

		It("rejects an expired key", func(ctx SpecContext) {
			token := createAPIKeyWith(ctx,
				&model.APIKeySetter{ExpireTime: omitnull.From(time.Now().Add(-time.Minute))}, "*")

			err := listGamesWith(ctx, authedConn(token))

			Expect(err).To(HaveReason("INVALID_CREDENTIALS"))
		})

		It("accepts a key that expires later", func(ctx SpecContext) {
			token := createAPIKeyWith(ctx,
				&model.APIKeySetter{ExpireTime: omitnull.From(time.Now().Add(time.Hour))}, "games.read")

			Expect(listGamesWith(ctx, authedConn(token))).To(Succeed())
		})

		It("denies a call the key lacks the permission for", func(ctx SpecContext) {
			client := ledgerv1.NewGamesServiceClient(authedConn(createAPIKey(ctx, "games.read")))

			_, err := client.CreateGame(ctx, &ledgerv1.CreateGameRequest{Game: newGame()})

			Expect(status.Code(err)).To(Equal(codes.PermissionDenied))
			Expect(err).To(HaveReason("MISSING_PERMISSION"))
			Expect(err).To(HaveErrorMetadata("permission", "games.write"))
		})

		It("checks permissions before validating the request", func(ctx SpecContext) {
			client := ledgerv1.NewGamesServiceClient(authedConn(createAPIKey(ctx)))

			_, err := client.CreateGame(ctx, &ledgerv1.CreateGameRequest{})

			Expect(err).To(HaveReason("MISSING_PERMISSION"))
		})

		It("allows a call a wildcard permission covers", func(ctx SpecContext) {
			client := ledgerv1.NewGamesServiceClient(authedConn(createAPIKey(ctx, "games.*")))

			_, err := client.CreateGame(ctx, &ledgerv1.CreateGameRequest{
				GameId: testkit.ResourceID("game"),
				Game:   newGame(),
			})

			Expect(err).ToNot(HaveOccurred())
		})

		It("records when a key was last used", func(ctx SpecContext) {
			token := createAPIKey(ctx, "games.read")
			id, _, _ := auth.ParseToken(token)

			Expect(listGamesWith(ctx, authedConn(token))).To(Succeed())

			key, err := model.FindAPIKey(ctx, db, id)
			Expect(err).ToNot(HaveOccurred())
			Expect(key.LastUsedTime.MustGet()).To(BeTemporally("~", time.Now(), time.Minute))
		})

		It("leaves the health service public", func(ctx SpecContext) {
			_, err := healthpb.NewHealthClient(suite.Conn()).Check(ctx, &healthpb.HealthCheckRequest{})

			Expect(err).ToNot(HaveOccurred())
		})
	})

	Describe("HTTP", func() {
		var server *httptest.Server

		BeforeEach(func() {
			files := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write([]byte("content"))
			})

			authenticator := auth.NewAuthenticator(db, time.Minute)
			server = httptest.NewServer(auth.RequireHTTP(authenticator, slog.New(slog.DiscardHandler), "assets.read:maps", files))
			DeferCleanup(server.Close)
		})

		get := func(ctx context.Context, authorization string) *http.Response {
			GinkgoHelper()

			req, err := http.NewRequestWithContext(ctx, http.MethodGet, server.URL, nil)
			Expect(err).ToNot(HaveOccurred())

			if authorization != "" {
				req.Header.Set("Authorization", authorization)
			}

			resp, err := server.Client().Do(req)
			Expect(err).ToNot(HaveOccurred())
			DeferCleanup(resp.Body.Close)

			return resp
		}

		It("rejects a request without credentials", func(ctx SpecContext) {
			resp := get(ctx, "")

			Expect(resp.StatusCode).To(Equal(http.StatusUnauthorized))
			Expect(resp.Header.Get("WWW-Authenticate")).To(Equal("Bearer"))
		})

		It("rejects a request with another scheme", func(ctx SpecContext) {
			resp := get(ctx, "Basic "+createAPIKey(ctx, "*"))

			Expect(resp.StatusCode).To(Equal(http.StatusUnauthorized))
		})

		It("rejects an invalid token", func(ctx SpecContext) {
			resp := get(ctx, "Bearer not-a-token")

			Expect(resp.StatusCode).To(Equal(http.StatusUnauthorized))
		})

		It("forbids a key without access to the repo", func(ctx SpecContext) {
			resp := get(ctx, "Bearer "+createAPIKey(ctx, "assets.read:skins"))

			Expect(resp.StatusCode).To(Equal(http.StatusForbidden))
		})

		It("serves a key with access to the repo", func(ctx SpecContext) {
			resp := get(ctx, "bearer "+createAPIKey(ctx, "assets.read:*"))

			Expect(resp.StatusCode).To(Equal(http.StatusOK))
		})
	})
})
