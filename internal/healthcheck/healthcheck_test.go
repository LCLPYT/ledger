package healthcheck_test

import (
	"bytes"
	"context"
	"log/slog"
	"net"
	"net/http"
	"sync"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/quadrubo/golib/app"
	"github.com/quadrubo/golib/config"
	"github.com/quadrubo/golib/grpcserver"
	"github.com/samber/do/v2"
	"github.com/stephenafamo/bob"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"

	"github.com/lclpyt/ledger/internal/assetrepo"
	"github.com/lclpyt/ledger/internal/auth"
	"github.com/lclpyt/ledger/internal/healthcheck"
)

func TestHealthcheck(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Healthcheck Suite")
}

type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()

	return b.buf.String()
}

// deps stands in for the logging and auth modules. The repo is public, so the authenticator stays unused.
type deps struct {
	log *slog.Logger
}

func (d *deps) Name() string { return "deps" }

func (d *deps) Provide(_ context.Context, i do.Injector) error {
	do.ProvideValue(i, d.log)
	do.ProvideValue(i, auth.NewAuthenticator(bob.DB{}, 0))

	return nil
}

func freeAddr() string {
	GinkgoHelper()

	lis, err := net.Listen("tcp", "127.0.0.1:0")
	Expect(err).ToNot(HaveOccurred())
	Expect(lis.Close()).To(Succeed())

	return lis.Addr().String()
}

func port(addr string) string {
	GinkgoHelper()

	_, p, err := net.SplitHostPort(addr)
	Expect(err).ToNot(HaveOccurred())

	return p
}

func repos() map[string]any {
	return map[string]any{"public": map[string]any{"path": GinkgoT().TempDir(), "public": true}}
}

func probe(ctx context.Context, settings map[string]any) error {
	GinkgoHelper()

	a, err := app.New(ctx, []app.Module{config.StaticModule(map[string]any{"modules": settings})})
	Expect(err).ToNot(HaveOccurred())

	return healthcheck.Probe(ctx, a.Injector())
}

var _ = Describe("Probe", Ordered, func() {
	var (
		grpcAddr  string
		assetAddr string
		healthSrv *health.Server
		logs      *syncBuffer
	)

	BeforeAll(func(ctx SpecContext) {
		logs = &syncBuffer{}
		assetAddr = freeAddr()

		a, err := app.New(ctx, []app.Module{
			config.StaticModule(map[string]any{
				"modules": map[string]any{
					"grpcserver": map[string]any{"addr": "127.0.0.1:0"},
					"asset_repo": map[string]any{"addr": assetAddr, "repos": repos()},
				},
			}),
			&deps{log: slog.New(slog.NewTextHandler(logs, nil))},
			grpcserver.Module(),
			assetrepo.Module(),
		})
		Expect(err).ToNot(HaveOccurred())

		grpcAddr = string(do.MustInvoke[grpcserver.Addr](a.Injector()))
		healthSrv = do.MustInvoke[*health.Server](a.Injector())

		runCtx, cancel := context.WithCancel(context.Background())
		done := make(chan error, 1)

		go func() { done <- a.Run(runCtx) }()

		DeferCleanup(func() {
			cancel()
			Expect(<-done).To(Succeed())
		})

		Eventually(func() error {
			resp, err := http.Get("http://" + assetAddr + assetrepo.HealthPath)
			if err != nil {
				return err
			}

			return resp.Body.Close()
		}).Should(Succeed())
	})

	It("succeeds when the grpc server and the asset repo answer", func(ctx SpecContext) {
		Expect(probe(ctx, map[string]any{
			"grpcserver": map[string]any{"addr": grpcAddr},
			"asset_repo": map[string]any{"addr": assetAddr, "repos": repos()},
		})).To(Succeed())
	})

	It("does not log asset repo health checks", func(ctx SpecContext) {
		Expect(probe(ctx, map[string]any{
			"grpcserver": map[string]any{"addr": grpcAddr},
			"asset_repo": map[string]any{"addr": assetAddr, "repos": repos()},
		})).To(Succeed())

		Expect(logs.String()).ToNot(ContainSubstring(assetrepo.HealthPath))
	})

	It("dials the loopback interface for an address on all interfaces", func(ctx SpecContext) {
		Expect(probe(ctx, map[string]any{
			"grpcserver": map[string]any{"addr": ":" + port(grpcAddr)},
			"asset_repo": map[string]any{"addr": "0.0.0.0:" + port(assetAddr), "repos": repos()},
		})).To(Succeed())
	})

	It("fails when the asset repo does not answer", func(ctx SpecContext) {
		Expect(probe(ctx, map[string]any{
			"grpcserver": map[string]any{"addr": grpcAddr},
			"asset_repo": map[string]any{"addr": freeAddr(), "repos": repos()},
		})).To(MatchError(ContainSubstring("asset repo health check")))
	})

	It("skips the asset repo when no repos are configured", func(ctx SpecContext) {
		Expect(probe(ctx, map[string]any{
			"grpcserver": map[string]any{"addr": grpcAddr},
			"asset_repo": map[string]any{"addr": freeAddr()},
		})).To(Succeed())
	})

	It("fails when the grpc server is not serving", func(ctx SpecContext) {
		healthSrv.SetServingStatus("", healthpb.HealthCheckResponse_NOT_SERVING)
		DeferCleanup(func() { healthSrv.SetServingStatus("", healthpb.HealthCheckResponse_SERVING) })

		Expect(probe(ctx, map[string]any{
			"grpcserver": map[string]any{"addr": grpcAddr},
		})).To(MatchError(ContainSubstring("NOT_SERVING")))
	})

	It("only connects to the grpc server when its health service is disabled", func(ctx SpecContext) {
		healthSrv.SetServingStatus("", healthpb.HealthCheckResponse_NOT_SERVING)
		DeferCleanup(func() { healthSrv.SetServingStatus("", healthpb.HealthCheckResponse_SERVING) })

		Expect(probe(ctx, map[string]any{
			"grpcserver": map[string]any{"addr": grpcAddr, "health": false},
		})).To(Succeed())
	})

	It("fails when the grpc server does not answer", func(ctx SpecContext) {
		Expect(probe(ctx, map[string]any{
			"grpcserver": map[string]any{"addr": freeAddr()},
		})).To(HaveOccurred())
	})
})
