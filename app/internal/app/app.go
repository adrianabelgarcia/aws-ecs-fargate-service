// Package app owns process lifecycle: it starts the API and metrics servers,
// and on shutdown drains traffic before closing listeners.
//
// Shutdown sequence (matches the ECS stop timeout and ALB deregistration delay):
//  1. Readiness flips to 503 so the ALB stops sending new requests.
//  2. READY_DRAIN_DELAY elapses so the ALB observes the failing target.
//  3. http.Server.Shutdown waits for in-flight requests up to SHUTDOWN_TIMEOUT.
package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/adrianabelgarcia/aws-ecs-fargate-service/app/internal/config"
	"github.com/adrianabelgarcia/aws-ecs-fargate-service/app/internal/items"
	"github.com/adrianabelgarcia/aws-ecs-fargate-service/app/internal/metrics"
	"github.com/adrianabelgarcia/aws-ecs-fargate-service/app/internal/server"
)

const (
	readHeaderTimeout = 5 * time.Second
	readTimeout       = 10 * time.Second
	writeTimeout      = 15 * time.Second
	idleTimeout       = 60 * time.Second
	maxHeaderBytes    = 1 << 16
)

// NewLogger returns a JSON logger that stamps every line with service identity.
func NewLogger(cfg config.Config, w io.Writer) *slog.Logger {
	h := slog.NewJSONHandler(w, &slog.HandlerOptions{Level: cfg.LogLevel})
	return slog.New(h).With(
		slog.String("service", cfg.ServiceName),
		slog.String("version", cfg.ServiceVersion),
		slog.String("env", cfg.Environment),
	)
}

// Run binds the configured ports and serves until ctx is cancelled.
func Run(ctx context.Context, cfg config.Config, logOut io.Writer) error {
	var lc net.ListenConfig
	apiLn, err := lc.Listen(ctx, "tcp", fmt.Sprintf(":%d", cfg.HTTPPort))
	if err != nil {
		return fmt.Errorf("listen api: %w", err)
	}
	metricsLn, err := lc.Listen(ctx, "tcp", fmt.Sprintf(":%d", cfg.MetricsPort))
	if err != nil {
		_ = apiLn.Close()
		return fmt.Errorf("listen metrics: %w", err)
	}
	return Serve(ctx, cfg, NewLogger(cfg, logOut), apiLn, metricsLn)
}

// Serve runs both servers on the supplied listeners. It is separated from Run so
// tests can use ephemeral ports.
func Serve(ctx context.Context, cfg config.Config, logger *slog.Logger, apiLn, metricsLn net.Listener) error {
	slog.SetDefault(logger)

	m := metrics.New(cfg.ServiceName, cfg.ServiceVersion)
	readiness := server.NewReadiness()
	svc := items.NewService(items.NewMemoryRepository())

	api := &http.Server{
		Handler: server.NewHandler(server.Deps{
			Logger:    logger,
			Items:     svc,
			Metrics:   m,
			Readiness: readiness,
			Version:   cfg.ServiceVersion,
			MaxPage:   cfg.MaxPageSize,
			APIToken:  cfg.APIToken,
		}),
		ReadHeaderTimeout: readHeaderTimeout,
		ReadTimeout:       readTimeout,
		WriteTimeout:      writeTimeout,
		IdleTimeout:       idleTimeout,
		MaxHeaderBytes:    maxHeaderBytes,
	}
	metricsSrv := &http.Server{
		Handler:           server.NewMetricsHandler(m),
		ReadHeaderTimeout: readHeaderTimeout,
	}

	errCh := make(chan error, 2)
	go serve(api, apiLn, errCh)
	go serve(metricsSrv, metricsLn, errCh)

	readiness.SetReady(true)
	logger.Info("service started",
		slog.Int("http_port", cfg.HTTPPort),
		slog.Int("metrics_port", cfg.MetricsPort),
		slog.Bool("write_auth_enabled", cfg.APIToken != ""),
	)

	var serveErr error
	select {
	case <-ctx.Done():
		logger.Info("shutdown signal received")
	case serveErr = <-errCh:
		logger.Error("server stopped unexpectedly", slog.Any("err", serveErr))
	}

	shutdownErr := drain(logger, cfg, readiness, api, metricsSrv)
	return errors.Join(serveErr, shutdownErr)
}

func serve(srv *http.Server, ln net.Listener, errCh chan<- error) {
	if err := srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
		errCh <- err
	}
}

func drain(logger *slog.Logger, cfg config.Config, readiness *server.Readiness, servers ...*http.Server) error {
	readiness.SetReady(false)
	logger.Info("draining", slog.Duration("ready_drain_delay", cfg.ReadyDrainDelay))
	time.Sleep(cfg.ReadyDrainDelay)

	ctx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()

	var errs []error
	for _, srv := range servers {
		if err := srv.Shutdown(ctx); err != nil {
			errs = append(errs, fmt.Errorf("shutdown: %w", err))
		}
	}
	if len(errs) == 0 {
		logger.Info("shutdown complete")
	}
	return errors.Join(errs...)
}

// Probe performs a GET against url and returns an error unless the response is 2xx.
// It powers the `healthcheck` subcommand used by the ECS container health check,
// since the distroless runtime image has no shell or curl.
func Probe(ctx context.Context, url string) error {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return fmt.Errorf("probe %s returned %d", url, resp.StatusCode)
	}
	return nil
}
