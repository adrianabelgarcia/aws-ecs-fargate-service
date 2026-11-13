package main

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"
	"time"
)

func freePort(t *testing.T) int {
	t.Helper()
	var lc net.ListenConfig
	ln, err := lc.Listen(context.Background(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = ln.Close() }()
	return ln.Addr().(*net.TCPAddr).Port
}

func TestRun_InvalidConfigExits2(t *testing.T) {
	t.Parallel()
	var stderr bytes.Buffer
	code := run(context.Background(), nil, func(k string) (string, bool) {
		if k == "HTTP_PORT" {
			return "nope", true
		}
		return "", false
	}, &bytes.Buffer{}, &stderr)
	if code != exitBadInput || stderr.Len() == 0 {
		t.Fatalf("run() = %d, stderr %q; want exit 2 with message", code, stderr.String())
	}
}

func TestRun_HealthcheckSubcommand(t *testing.T) {
	t.Parallel()
	healthy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(healthy.Close)
	u, _ := url.Parse(healthy.URL)
	port := u.Port()

	lookup := func(k string) (string, bool) {
		if k == "HTTP_PORT" {
			return port, true
		}
		return "", false
	}
	if code := run(context.Background(), []string{"healthcheck"}, lookup, &bytes.Buffer{}, &bytes.Buffer{}); code != exitOK {
		t.Fatalf("healthy probe exit = %d, want 0", code)
	}

	// Nothing listens on the freed port, so the probe must fail.
	dead := strconv.Itoa(freePort(t))
	lookup = func(k string) (string, bool) {
		if k == "HTTP_PORT" {
			return dead, true
		}
		return "", false
	}
	var stderr bytes.Buffer
	if code := run(context.Background(), []string{"healthcheck"}, lookup, &bytes.Buffer{}, &stderr); code != exitFailure {
		t.Fatalf("unhealthy probe exit = %d, want 1", code)
	}
}

func TestRun_ServesUntilContextCancelled(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	api, metricsPort := freePort(t), freePort(t)
	lookup := func(k string) (string, bool) {
		switch k {
		case "HTTP_PORT":
			return strconv.Itoa(api), true
		case "METRICS_PORT":
			return strconv.Itoa(metricsPort), true
		case "READY_DRAIN_DELAY":
			return "0s", true
		}
		return "", false
	}
	done := make(chan int, 1)
	go func() { done <- run(ctx, nil, lookup, &bytes.Buffer{}, &bytes.Buffer{}) }()

	waitReady(t, fmt.Sprintf("http://127.0.0.1:%d/readyz", api))
	cancel()
	select {
	case code := <-done:
		if code != exitOK {
			t.Fatalf("run() = %d, want 0", code)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("run did not return after cancellation")
	}
}

func TestRun_ListenFailureExits1(t *testing.T) {
	t.Parallel()
	var lc net.ListenConfig
	busy, err := lc.Listen(context.Background(), "tcp", ":0") // run binds the wildcard address
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = busy.Close() }()
	port := strconv.Itoa(busy.Addr().(*net.TCPAddr).Port)
	lookup := func(k string) (string, bool) {
		if k == "HTTP_PORT" {
			return port, true
		}
		return "", false
	}
	var stderr bytes.Buffer
	if code := run(context.Background(), nil, lookup, &bytes.Buffer{}, &stderr); code != exitFailure {
		t.Fatalf("run() = %d, want 1 when the port is taken", code)
	}
}

func waitReady(t *testing.T, target string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		req, reqErr := http.NewRequestWithContext(context.Background(), http.MethodGet, target, nil)
		if reqErr != nil {
			t.Fatal(reqErr)
		}
		resp, err := http.DefaultClient.Do(req)
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("service never became ready at %s", target)
}
