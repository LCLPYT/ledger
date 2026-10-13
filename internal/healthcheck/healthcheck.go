package healthcheck

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/quadrubo/golib/config"
	"github.com/quadrubo/golib/grpcserver"
	"github.com/samber/do/v2"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"

	"github.com/lclpyt/ledger/internal/assetrepo"
)

const (
	grpcServerKey = "modules.grpcserver"

	timeout = 3 * time.Second
)

// Probe checks the gRPC server and, when it serves any repo, the asset repo of the instance the config describes.
func Probe(ctx context.Context, i do.Injector) error {
	// The defaults mirror the ones of grpcserver, since it does not export them.
	grpcCfg, err := config.Load(i, "healthcheck", grpcServerKey, grpcserver.Config{
		Addr:            ":50051",
		GracePeriod:     5 * time.Second,
		Reflection:      true,
		Health:          true,
		MaxReceiveBytes: 4 * 1024 * 1024,
	})

	if err != nil {
		return err
	}

	assetAddr, err := assetrepo.Addr(i)

	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	grpcAddr, err := dialAddr(grpcCfg.Addr)

	if err != nil {
		return err
	}

	if grpcCfg.Health {
		err = checkGRPC(ctx, grpcAddr)
	} else {
		err = checkTCP(ctx, grpcAddr)
	}

	if err != nil {
		return err
	}

	if assetAddr == "" {
		return nil
	}

	assetAddr, err = dialAddr(assetAddr)

	if err != nil {
		return err
	}

	return checkHTTP(ctx, "http://"+assetAddr+assetrepo.HealthPath)
}

// dialAddr turns a listen address on all interfaces into one on the loopback interface.
func dialAddr(listenAddr string) (string, error) {
	host, port, err := net.SplitHostPort(listenAddr)

	if err != nil {
		return "", fmt.Errorf("healthcheck: invalid address %q: %w", listenAddr, err)
	}

	switch host {
	case "", "0.0.0.0":
		host = "127.0.0.1"
	case "::":
		host = "::1"
	}

	return net.JoinHostPort(host, port), nil
}

func checkGRPC(ctx context.Context, addr string) error {
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))

	if err != nil {
		return fmt.Errorf("healthcheck: failed to dial grpc server at %s: %w", addr, err)
	}

	defer func() { _ = conn.Close() }()

	resp, err := healthpb.NewHealthClient(conn).Check(ctx, &healthpb.HealthCheckRequest{})

	if err != nil {
		return fmt.Errorf("healthcheck: grpc health check at %s failed: %w", addr, err)
	}

	if resp.GetStatus() != healthpb.HealthCheckResponse_SERVING {
		return fmt.Errorf("healthcheck: grpc server at %s is %s", addr, resp.GetStatus())
	}

	return nil
}

func checkTCP(ctx context.Context, addr string) error {
	var dialer net.Dialer

	conn, err := dialer.DialContext(ctx, "tcp", addr)

	if err != nil {
		return fmt.Errorf("healthcheck: failed to connect to grpc server at %s: %w", addr, err)
	}

	return conn.Close()
}

func checkHTTP(ctx context.Context, url string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)

	if err != nil {
		return fmt.Errorf("healthcheck: failed to build request for %s: %w", url, err)
	}

	resp, err := http.DefaultClient.Do(req)

	if err != nil {
		return fmt.Errorf("healthcheck: asset repo health check at %s failed: %w", url, err)
	}

	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("healthcheck: asset repo at %s answered with %s", url, resp.Status)
	}

	return nil
}
