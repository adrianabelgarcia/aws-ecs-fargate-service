package server

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"log/slog"
	"net/http"
	"regexp"
	"runtime/debug"
	"strings"
	"time"

	"github.com/adrianabelgarcia/aws-ecs-fargate-service/app/internal/metrics"
)

var validRequestID = regexp.MustCompile(`^[A-Za-z0-9._-]{1,64}$`)

// Middleware wraps a handler.
type Middleware func(http.Handler) http.Handler

// chain applies middlewares so that the first one in the list is the outermost.
func chain(h http.Handler, mws ...Middleware) http.Handler {
	for i := len(mws) - 1; i >= 0; i-- {
		h = mws[i](h)
	}
	return h
}

// securityHeaders sets conservative defaults. The ALB terminates TLS, so HSTS
// is set here as well for defence in depth.
func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "no-referrer")
		h.Set("Cache-Control", "no-store")
		h.Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		next.ServeHTTP(w, r)
	})
}

// requestID honours a well-formed inbound X-Request-Id or generates one, then
// echoes it back so clients and logs can be correlated.
func requestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-Request-Id")
		if !validRequestID.MatchString(id) {
			id = newRequestID()
		}
		w.Header().Set("X-Request-Id", id)
		next.ServeHTTP(w, r.WithContext(withRequestID(r.Context(), id)))
	})
}

func newRequestID() string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "unavailable"
	}
	return hex.EncodeToString(b[:])
}

// statusRecorder captures the status code and bytes written.
type statusRecorder struct {
	http.ResponseWriter
	status int
	bytes  int
}

func (s *statusRecorder) WriteHeader(code int) {
	if s.status == 0 {
		s.status = code
	}
	s.ResponseWriter.WriteHeader(code)
}

func (s *statusRecorder) Write(b []byte) (int, error) {
	if s.status == 0 {
		s.status = http.StatusOK
	}
	n, err := s.ResponseWriter.Write(b)
	s.bytes += n
	return n, err
}

// accessLog emits one structured line per request and records metrics.
// Query strings are deliberately not logged to avoid leaking user input.
func accessLog(logger *slog.Logger, m *metrics.Metrics, mux *http.ServeMux) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			route := routeLabel(mux, r)
			m.InFlight().Inc()
			defer m.InFlight().Dec()

			rec := &statusRecorder{ResponseWriter: w}
			next.ServeHTTP(rec, r)

			status := rec.status
			if status == 0 {
				status = http.StatusOK
			}
			elapsed := time.Since(start)
			m.Observe(r.Method, route, status, elapsed)

			logger.LogAttrs(r.Context(), levelFor(status),
				"http_request",
				slog.String("request_id", RequestIDFrom(r.Context())),
				slog.String("method", r.Method),
				slog.String("route", route),
				slog.String("path", r.URL.Path),
				slog.Int("status", status),
				slog.Int("bytes", rec.bytes),
				slog.Float64("duration_ms", float64(elapsed.Microseconds())/1000),
				slog.String("user_agent", truncate(r.UserAgent(), 128)),
			)
		})
	}
}

// recoverer converts panics into a 500 response and a logged stack trace.
func recoverer(logger *slog.Logger) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if v := recover(); v != nil {
					logger.Error("panic recovered",
						"request_id", RequestIDFrom(r.Context()),
						"panic", v,
						"stack", string(debug.Stack()),
					)
					writeError(w, r, http.StatusInternalServerError, "internal", "internal server error")
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

// requireBearer enforces a static bearer token using a constant-time comparison
// of digests, so neither the token nor its length leaks through timing.
func requireBearer(token string) Middleware {
	want := sha256.Sum256([]byte(token))
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			const prefix = "Bearer "
			header := r.Header.Get("Authorization")
			if !strings.HasPrefix(header, prefix) {
				w.Header().Set("WWW-Authenticate", `Bearer realm="items-api"`)
				writeError(w, r, http.StatusUnauthorized, "unauthorized", "missing bearer token")
				return
			}
			got := sha256.Sum256([]byte(strings.TrimPrefix(header, prefix)))
			if subtle.ConstantTimeCompare(got[:], want[:]) != 1 {
				w.Header().Set("WWW-Authenticate", `Bearer realm="items-api", error="invalid_token"`)
				writeError(w, r, http.StatusUnauthorized, "unauthorized", "invalid bearer token")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// routeLabel returns the ServeMux pattern that will serve r, or a constant for
// unmatched requests so attackers cannot explode metric cardinality. The mux is
// queried directly because middleware that copies the request would hide the
// pattern that the mux writes onto it.
func routeLabel(mux *http.ServeMux, r *http.Request) string {
	_, pattern := mux.Handler(r)
	if pattern == "" {
		return "unmatched"
	}
	return pattern
}

func levelFor(status int) slog.Level {
	switch {
	case status >= 500:
		return slog.LevelError
	case status >= 400:
		return slog.LevelWarn
	default:
		return slog.LevelInfo
	}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
