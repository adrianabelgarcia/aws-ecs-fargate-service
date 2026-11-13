// Command api runs the items HTTP service.
//
//	api                 serve HTTP until SIGINT/SIGTERM
//	api healthcheck     probe the local /healthz endpoint (used by the ECS health check)
package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/adrianabelgarcia/aws-ecs-fargate-service/app/internal/app"
	"github.com/adrianabelgarcia/aws-ecs-fargate-service/app/internal/config"
)

const (
	exitOK       = 0
	exitFailure  = 1
	exitBadInput = 2
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	code := run(ctx, os.Args[1:], os.LookupEnv, os.Stdout, os.Stderr)
	stop()
	os.Exit(code)
}

// run is main with its dependencies injected so it can be tested.
func run(ctx context.Context, args []string, lookup config.Lookup, stdout, stderr io.Writer) int {
	cfg, err := config.Load(lookup)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, "invalid configuration:", err)
		return exitBadInput
	}

	if len(args) > 0 && args[0] == "healthcheck" {
		url := fmt.Sprintf("http://127.0.0.1:%d/healthz", cfg.HTTPPort)
		if err := app.Probe(ctx, url); err != nil {
			_, _ = fmt.Fprintln(stderr, "healthcheck failed:", err)
			return exitFailure
		}
		return exitOK
	}

	if err := app.Run(ctx, cfg, stdout); err != nil {
		_, _ = fmt.Fprintln(stderr, "service exited with error:", err)
		return exitFailure
	}
	return exitOK
}
