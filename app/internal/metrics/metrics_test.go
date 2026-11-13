package metrics

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestObserveRecordsStatusClassesAndHandlerExposes(t *testing.T) {
	t.Parallel()
	m := New("svc", "1.0.0")
	m.Observe(http.MethodGet, "GET /x", 200, 10*time.Millisecond)
	m.Observe(http.MethodGet, "GET /x", 404, 10*time.Millisecond)
	m.Observe(http.MethodGet, "GET /x", 302, 10*time.Millisecond)
	m.Observe(http.MethodGet, "GET /x", 503, 10*time.Millisecond)
	m.InFlight().Inc()

	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/metrics", nil)
	m.Handler().ServeHTTP(rec, req)
	body := rec.Body.String()
	for _, class := range []string{`code="2xx"`, `code="3xx"`, `code="4xx"`, `code="5xx"`} {
		if !strings.Contains(body, class) {
			t.Errorf("missing %s in exposition", class)
		}
	}
	if !strings.Contains(body, `service="svc"`) || !strings.Contains(body, "http_requests_in_flight") {
		t.Error("const labels or in-flight gauge missing")
	}
}

func TestStatusClass(t *testing.T) {
	t.Parallel()
	cases := map[int]string{200: "2xx", 301: "3xx", 404: "4xx", 500: "5xx", 599: "5xx"}
	for code, want := range cases {
		if got := statusClass(code); got != want {
			t.Errorf("statusClass(%d) = %s, want %s", code, got, want)
		}
	}
}
