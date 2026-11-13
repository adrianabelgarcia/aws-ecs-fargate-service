package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"

	"github.com/adrianabelgarcia/aws-ecs-fargate-service/app/internal/config"
)

// envOnly returns a lookup that exposes only the given keys.
func envOnly(m map[string]string) config.Lookup {
	return func(k string) (string, bool) {
		v, ok := m[k]
		return v, ok
	}
}

// busyWildcardPort holds ":0" open for the duration of the test. Holding the
// listener (rather than freeing a port number) means the port cannot be
// reclaimed by another test or process while the test runs.
func busyWildcardPort(t *testing.T) (int, func()) {
	t.Helper()
	var lc net.ListenConfig
	ln, err := lc.Listen(context.Background(), "tcp", ":0")
	if err != nil {
		t.Fatal(err)
	}
	return ln.Addr().(*net.TCPAddr).Port, func() { _ = ln.Close() }
}

func TestRun_InvalidConfigExits2(t *testing.T) {
	t.Parallel()
	var stderr bytes.Buffer
	code := run(context.Background(), nil, envOnly(map[string]string{"HTTP_PORT": "nope"}), &bytes.Buffer{}, &stderr)
	if code != exitBadInput || stderr.Len() == 0 {
		t.Fatalf("run() = %d, stderr %q; want exit 2 with message", code, stderr.String())
	}
}

func TestRun_HealthcheckSubcommand(t *testing.T) {
	t.Parallel()
	healthy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(healthy.Close)
	failing := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	t.Cleanup(failing.Close)

	probe := func(srv *httptest.Server) int {
		t.Helper()
		u, err := url.Parse(srv.URL)
		if err != nil {
			t.Fatal(err)
		}
		lookup := envOnly(map[string]string{"HTTP_PORT": u.Port()})
		return run(context.Background(), []string{"healthcheck"}, lookup, &bytes.Buffer{}, &bytes.Buffer{})
	}
	if code := probe(healthy); code != exitOK {
		t.Fatalf("healthy probe exit = %d, want 0", code)
	}
	if code := probe(failing); code != exitFailure {
		t.Fatalf("unhealthy probe exit = %d, want 1", code)
	}
}

func TestRun_DelegatesToServerWithConfig(t *testing.T) {
	// Not parallel: it swaps the package-level serve hook.
	original := serve
	t.Cleanup(func() { serve = original })

	var got config.Config
	serve = func(_ context.Context, cfg config.Config, _ io.Writer) error {
		got = cfg
		return nil
	}
	lookup := envOnly(map[string]string{"HTTP_PORT": "9001", "METRICS_PORT": "9002"})
	if code := run(context.Background(), nil, lookup, &bytes.Buffer{}, &bytes.Buffer{}); code != exitOK {
		t.Fatalf("run() = %d, want 0", code)
	}
	if got.HTTPPort != 9001 || got.MetricsPort != 9002 {
		t.Errorf("server received ports %d/%d, want 9001/9002", got.HTTPPort, got.MetricsPort)
	}

	serve = func(context.Context, config.Config, io.Writer) error { return errors.New("boom") }
	var stderr bytes.Buffer
	if code := run(context.Background(), nil, lookup, &bytes.Buffer{}, &stderr); code != exitFailure {
		t.Fatalf("run() with server error = %d, want 1", code)
	}
	if !bytes.Contains(stderr.Bytes(), []byte("boom")) {
		t.Errorf("server error not reported on stderr: %q", stderr.String())
	}
}

func TestRun_ListenFailureExits1(t *testing.T) {
	t.Parallel()
	port, release := busyWildcardPort(t)
	defer release()
	lookup := envOnly(map[string]string{"HTTP_PORT": strconv.Itoa(port)})
	var stderr bytes.Buffer
	if code := run(context.Background(), nil, lookup, &bytes.Buffer{}, &stderr); code != exitFailure {
		t.Fatalf("run() = %d, want 1 when the port is taken", code)
	}
}
