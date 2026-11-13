package app

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/adrianabelgarcia/aws-ecs-fargate-service/app/internal/config"
)

func testConfig() config.Config {
	return config.Config{
		ServiceName:     "items-api",
		ServiceVersion:  "test",
		Environment:     "test",
		HTTPPort:        1,
		MetricsPort:     2,
		LogLevel:        slog.LevelDebug,
		ShutdownTimeout: 5 * time.Second,
		ReadyDrainDelay: 0,
		MaxPageSize:     100,
		APIToken:        "0123456789abcdef",
	}
}

func listen(t *testing.T) net.Listener {
	t.Helper()
	var lc net.ListenConfig
	ln, err := lc.Listen(context.Background(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	return ln
}

func getURL(url string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	return http.DefaultClient.Do(req)
}

func TestServe_LifecycleAndGracefulShutdown(t *testing.T) {
	t.Parallel()
	apiLn, metricsLn := listen(t), listen(t)
	logs := &bytes.Buffer{}
	logger := NewLogger(testConfig(), logs)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- Serve(ctx, testConfig(), logger, apiLn, metricsLn) }()

	base := "http://" + apiLn.Addr().String()
	waitForStatus(t, base+"/readyz", http.StatusOK)

	resp, err := getURL(base + "/api/v1/items")
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("list status = %d", resp.StatusCode)
	}

	mresp, err := getURL("http://" + metricsLn.Addr().String() + "/metrics")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(mresp.Body)
	_ = mresp.Body.Close()
	if !strings.Contains(string(body), "go_goroutines") {
		t.Error("metrics endpoint should expose Go runtime metrics")
	}

	cancel()
	if err := <-done; err != nil {
		t.Fatalf("Serve() = %v, want nil after graceful shutdown", err)
	}
	if !strings.Contains(logs.String(), "shutdown complete") {
		t.Errorf("expected shutdown log, got: %s", logs.String())
	}
}

func TestServe_ReportsUnexpectedServerFailure(t *testing.T) {
	t.Parallel()
	apiLn := listen(t)
	metricsLn := listen(t)
	_ = metricsLn.Close() // a listener that fails immediately forces the error path

	err := Serve(context.Background(), testConfig(), NewLogger(testConfig(), io.Discard), apiLn, metricsLn)
	if err == nil {
		t.Fatal("Serve() = nil, want error from closed metrics listener")
	}
}

// wildcardListener occupies ":0" because Run binds the wildcard address.
func wildcardListener(t *testing.T) net.Listener {
	t.Helper()
	var lc net.ListenConfig
	ln, err := lc.Listen(context.Background(), "tcp", ":0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	return ln
}

func TestRun_FailsWhenPortInUse(t *testing.T) {
	t.Parallel()
	busy := wildcardListener(t)
	cfg := testConfig()
	cfg.HTTPPort = busy.Addr().(*net.TCPAddr).Port
	cfg.MetricsPort = freePort(t)
	if err := Run(context.Background(), cfg, io.Discard); err == nil || !strings.Contains(err.Error(), "listen api") {
		t.Fatalf("Run() = %v, want listen api error", err)
	}
}

func TestRun_FailsWhenMetricsPortInUse(t *testing.T) {
	t.Parallel()
	busy := wildcardListener(t)
	cfg := testConfig()
	cfg.HTTPPort = freePort(t)
	cfg.MetricsPort = busy.Addr().(*net.TCPAddr).Port
	if err := Run(context.Background(), cfg, io.Discard); err == nil || !strings.Contains(err.Error(), "listen metrics") {
		t.Fatalf("Run() = %v, want listen metrics error", err)
	}
}

func TestProbe(t *testing.T) {
	t.Parallel()
	ok := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }))
	defer ok.Close()
	bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusServiceUnavailable) }))
	defer bad.Close()

	if err := Probe(context.Background(), ok.URL); err != nil {
		t.Errorf("Probe(2xx) = %v", err)
	}
	if err := Probe(context.Background(), bad.URL); err == nil || !strings.Contains(err.Error(), "503") {
		t.Errorf("Probe(503) = %v, want status error", err)
	}
	if err := Probe(context.Background(), "http://[::1"); err == nil {
		t.Error("Probe(invalid url) should fail")
	}
	if err := Probe(context.Background(), "http://127.0.0.1:1/healthz"); err == nil {
		t.Error("Probe(refused) should fail")
	}
}

func TestNewLoggerStampsIdentity(t *testing.T) {
	t.Parallel()
	buf := &bytes.Buffer{}
	NewLogger(testConfig(), buf).Info("hello")
	var line map[string]any
	if err := json.Unmarshal(buf.Bytes(), &line); err != nil {
		t.Fatalf("logger output is not JSON: %v", err)
	}
	if line["service"] != "items-api" || line["env"] != "test" || line["version"] != "test" {
		t.Errorf("identity fields missing: %v", line)
	}
}

func waitForStatus(t *testing.T, url string, want int) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := getURL(url)
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode == want {
				return
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("%s never returned %d", url, want)
}

func freePort(t *testing.T) int {
	t.Helper()
	return listen(t).Addr().(*net.TCPAddr).Port
}
