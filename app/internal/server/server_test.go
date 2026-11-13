package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/adrianabelgarcia/aws-ecs-fargate-service/app/internal/items"
	"github.com/adrianabelgarcia/aws-ecs-fargate-service/app/internal/metrics"
)

const testToken = "0123456789abcdef" // gitleaks:allow (test fixture, not a secret)

type stubRepo struct {
	getErr  error
	listErr error
}

func (s *stubRepo) List(context.Context, int, int) (items.Page, error) {
	return items.Page{}, s.listErr
}
func (s *stubRepo) Get(context.Context, string) (items.Item, error) {
	return items.Item{}, s.getErr
}
func (s *stubRepo) Insert(context.Context, items.Item) error { return nil }

type harness struct {
	handler   http.Handler
	readiness *Readiness
	metrics   *metrics.Metrics
	logs      *bytes.Buffer
}

func newHarness(t *testing.T, token string, repo items.Repository) harness {
	t.Helper()
	logs := &bytes.Buffer{}
	logger := slog.New(slog.NewJSONHandler(logs, nil))
	if repo == nil {
		repo = items.NewMemoryRepository()
	}
	m := metrics.New("test", "0.0.0")
	ready := NewReadiness()
	ready.SetReady(true)
	h := NewHandler(Deps{
		Logger:    logger,
		Items:     items.NewService(repo),
		Metrics:   m,
		Readiness: ready,
		Version:   "0.0.0",
		MaxPage:   50,
		APIToken:  token,
	})
	return harness{handler: h, readiness: ready, metrics: m, logs: logs}
}

func do(t *testing.T, h http.Handler, method, target, body string, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	var r io.Reader
	if body != "" {
		r = strings.NewReader(body)
	}
	req := httptest.NewRequestWithContext(context.Background(), method, target, r)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func decode(t *testing.T, rec *httptest.ResponseRecorder) Envelope {
	t.Helper()
	var env Envelope
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("invalid JSON body %q: %v", rec.Body.String(), err)
	}
	return env
}

func TestHealthz(t *testing.T) {
	t.Parallel()
	hn := newHarness(t, "", nil)
	hn.readiness.SetReady(false) // liveness must not depend on readiness
	rec := do(t, hn.handler, http.MethodGet, "/healthz", "", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
}

func TestReadyz(t *testing.T) {
	t.Parallel()
	hn := newHarness(t, "", nil)
	if rec := do(t, hn.handler, http.MethodGet, "/readyz", "", nil); rec.Code != http.StatusOK {
		t.Errorf("ready status = %d, want 200", rec.Code)
	}
	hn.readiness.SetReady(false)
	rec := do(t, hn.handler, http.MethodGet, "/readyz", "", nil)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("draining status = %d, want 503", rec.Code)
	}
	if env := decode(t, rec); env.Error == nil || env.Error.Code != "not_ready" {
		t.Errorf("error body = %+v, want code not_ready", env.Error)
	}
}

func TestCreateAndGetItem(t *testing.T) {
	t.Parallel()
	hn := newHarness(t, "", nil)

	rec := do(t, hn.handler, http.MethodPost, "/api/v1/items", `{"name":"Lamp","description":"warm"}`, nil)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create status = %d body=%s", rec.Code, rec.Body)
	}
	var created struct {
		Data items.Item `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if loc := rec.Header().Get("Location"); loc != "/api/v1/items/"+created.Data.ID {
		t.Errorf("Location = %q", loc)
	}

	get := do(t, hn.handler, http.MethodGet, "/api/v1/items/"+created.Data.ID, "", nil)
	if get.Code != http.StatusOK {
		t.Fatalf("get status = %d", get.Code)
	}
	if got := do(t, hn.handler, http.MethodGet, "/api/v1/items/nope", "", nil); got.Code != http.StatusNotFound {
		t.Errorf("missing item status = %d, want 404", got.Code)
	}
	long := strings.Repeat("x", maxIDLength+1)
	if got := do(t, hn.handler, http.MethodGet, "/api/v1/items/"+long, "", nil); got.Code != http.StatusBadRequest {
		t.Errorf("overlong id status = %d, want 400", got.Code)
	}
}

func TestCreateItem_ErrorPaths(t *testing.T) {
	t.Parallel()
	hn := newHarness(t, "", nil)
	tests := []struct {
		name     string
		body     string
		wantCode int
		wantErr  string
	}{
		{"malformed json", `{"name":`, http.StatusBadRequest, "invalid_json"},
		{"unknown field", `{"name":"a","colour":"red"}`, http.StatusBadRequest, "invalid_json"},
		{"validation", `{"name":"   "}`, http.StatusUnprocessableEntity, "validation_failed"},
		{"too large", `{"name":"` + strings.Repeat("a", maxBodyBytes) + `"}`, http.StatusRequestEntityTooLarge, "payload_too_large"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := do(t, hn.handler, http.MethodPost, "/api/v1/items", tt.body, nil)
			if rec.Code != tt.wantCode {
				t.Fatalf("status = %d, want %d (body %s)", rec.Code, tt.wantCode, rec.Body)
			}
			if env := decode(t, rec); env.Error == nil || env.Error.Code != tt.wantErr {
				t.Errorf("error = %+v, want code %s", env.Error, tt.wantErr)
			}
		})
	}
}

func TestCreateItem_RequiresBearerWhenTokenConfigured(t *testing.T) {
	t.Parallel()
	hn := newHarness(t, testToken, nil)
	body := `{"name":"Lamp"}`
	tests := []struct {
		name    string
		headers map[string]string
		want    int
	}{
		{"missing header", nil, http.StatusUnauthorized},
		{"wrong scheme", map[string]string{"Authorization": "Basic abc"}, http.StatusUnauthorized},
		{"wrong token", map[string]string{"Authorization": "Bearer nope"}, http.StatusUnauthorized},
		{"correct token", map[string]string{"Authorization": "Bearer " + testToken}, http.StatusCreated},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := do(t, hn.handler, http.MethodPost, "/api/v1/items", body, tt.headers)
			if rec.Code != tt.want {
				t.Fatalf("status = %d, want %d", rec.Code, tt.want)
			}
			if tt.want == http.StatusUnauthorized && rec.Header().Get("WWW-Authenticate") == "" {
				t.Error("401 must carry WWW-Authenticate")
			}
		})
	}
	// Reads stay public even when writes are protected.
	if rec := do(t, hn.handler, http.MethodGet, "/api/v1/items", "", nil); rec.Code != http.StatusOK {
		t.Errorf("public list status = %d, want 200", rec.Code)
	}
}

func TestListItems_Pagination(t *testing.T) {
	t.Parallel()
	hn := newHarness(t, "", nil)
	for _, n := range []string{"a", "b", "c"} {
		do(t, hn.handler, http.MethodPost, "/api/v1/items", `{"name":"`+n+`"}`, nil)
	}
	rec := do(t, hn.handler, http.MethodGet, "/api/v1/items?limit=2&offset=1", "", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	env := decode(t, rec)
	data, _ := env.Data.([]any)
	if len(data) != 2 {
		t.Errorf("page length = %d, want 2", len(data))
	}
	meta, _ := env.Meta.(map[string]any)
	if meta["total"] != float64(3) || meta["limit"] != float64(2) || meta["offset"] != float64(1) {
		t.Errorf("meta = %+v", meta)
	}

	empty := do(t, hn.handler, http.MethodGet, "/api/v1/items", "", nil)
	if !strings.Contains(empty.Body.String(), `"limit":20`) {
		t.Errorf("default page size not applied: %s", empty.Body)
	}
}

func TestListItems_InvalidQuery(t *testing.T) {
	t.Parallel()
	hn := newHarness(t, "", nil)
	for _, q := range []string{"limit=0", "limit=51", "limit=x", "offset=-1", "offset=2000000"} {
		rec := do(t, hn.handler, http.MethodGet, "/api/v1/items?"+q, "", nil)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("query %q status = %d, want 400", q, rec.Code)
		}
	}
}

func TestInternalErrorsAreGeneric(t *testing.T) {
	t.Parallel()
	repo := &stubRepo{getErr: errors.New("dynamo throttled: table secret-table-name"), listErr: errors.New("boom")}
	hn := newHarness(t, "", repo)

	rec := do(t, hn.handler, http.MethodGet, "/api/v1/items/abc", "", nil)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
	if strings.Contains(rec.Body.String(), "secret-table-name") {
		t.Error("internal error detail leaked to client")
	}
	if !strings.Contains(hn.logs.String(), "dynamo throttled") {
		t.Error("internal error detail should be logged server-side")
	}
	if rec := do(t, hn.handler, http.MethodGet, "/api/v1/items", "", nil); rec.Code != http.StatusInternalServerError {
		t.Errorf("list failure status = %d, want 500", rec.Code)
	}
}

func TestRequestIDHandling(t *testing.T) {
	t.Parallel()
	hn := newHarness(t, "", nil)

	rec := do(t, hn.handler, http.MethodGet, "/healthz", "", map[string]string{"X-Request-Id": "abc-123"})
	if got := rec.Header().Get("X-Request-Id"); got != "abc-123" {
		t.Errorf("inbound id not honoured: %q", got)
	}
	rec = do(t, hn.handler, http.MethodGet, "/healthz", "", map[string]string{"X-Request-Id": "bad id with spaces"})
	if got := rec.Header().Get("X-Request-Id"); got == "" || got == "bad id with spaces" {
		t.Errorf("malformed inbound id should be replaced, got %q", got)
	}
}

func TestSecurityHeadersPresent(t *testing.T) {
	t.Parallel()
	hn := newHarness(t, "", nil)
	rec := do(t, hn.handler, http.MethodGet, "/healthz", "", nil)
	for _, h := range []string{"X-Content-Type-Options", "X-Frame-Options", "Strict-Transport-Security", "Cache-Control"} {
		if rec.Header().Get(h) == "" {
			t.Errorf("missing header %s", h)
		}
	}
}

func TestAccessLogAndMetricsUseRoutePattern(t *testing.T) {
	t.Parallel()
	hn := newHarness(t, "", nil)
	do(t, hn.handler, http.MethodGet, "/api/v1/items/unknown-id", "", nil)
	do(t, hn.handler, http.MethodGet, "/some/random/path?secret=1", "", nil)

	logs := hn.logs.String()
	if !strings.Contains(logs, `"route":"GET /api/v1/items/{id}"`) {
		t.Errorf("access log should use route pattern, logs: %s", logs)
	}
	if strings.Contains(logs, "secret=1") {
		t.Error("query strings must not be logged")
	}

	mrec := httptest.NewRecorder()
	NewMetricsHandler(hn.metrics).ServeHTTP(mrec, httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/metrics", nil))
	body := mrec.Body.String()
	if !strings.Contains(body, `route="unmatched"`) {
		t.Errorf("unmatched requests should share one label; metrics snippet missing")
	}
	if !strings.Contains(body, "http_requests_total") || !strings.Contains(body, "http_request_duration_seconds_bucket") {
		t.Error("expected HTTP metrics in exposition output")
	}
}

func TestRecovererConvertsPanicTo500(t *testing.T) {
	t.Parallel()
	logs := &bytes.Buffer{}
	logger := slog.New(slog.NewJSONHandler(logs, nil))
	h := recoverer(logger)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("kaboom")
	}))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/x", nil))
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
	if !strings.Contains(logs.String(), "panic recovered") {
		t.Error("panic should be logged")
	}
}

func TestStatusRecorderDefaults(t *testing.T) {
	t.Parallel()
	rec := httptest.NewRecorder()
	sr := &statusRecorder{ResponseWriter: rec}
	if _, err := sr.Write([]byte("hi")); err != nil {
		t.Fatal(err)
	}
	if sr.status != http.StatusOK || sr.bytes != 2 {
		t.Errorf("status=%d bytes=%d, want 200/2", sr.status, sr.bytes)
	}
}

func TestHelpers(t *testing.T) {
	t.Parallel()
	if levelFor(503) != slog.LevelError || levelFor(404) != slog.LevelWarn || levelFor(200) != slog.LevelInfo {
		t.Error("levelFor mapping incorrect")
	}
	if truncate("abcdef", 3) != "abc" || truncate("ab", 3) != "ab" {
		t.Error("truncate incorrect")
	}
	if RequestIDFrom(context.Background()) != "" {
		t.Error("RequestIDFrom on empty context should be empty")
	}
	if newRequestID() == "" {
		t.Error("newRequestID returned empty string")
	}
}
