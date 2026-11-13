package server

import (
	"encoding/json"
	"log/slog"
	"net/http"
)

// Envelope is the single response shape used by every endpoint.
//
//	success: {"data": ..., "meta": {...}}
//	failure: {"error": {"code": "...", "message": "...", "request_id": "..."}}
type Envelope struct {
	Data  any    `json:"data,omitempty"`
	Meta  any    `json:"meta,omitempty"`
	Error *Error `json:"error,omitempty"`
}

// Error is the client-facing error body.
type Error struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	RequestID string `json:"request_id,omitempty"`
}

func writeJSON(w http.ResponseWriter, status int, body Envelope) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(body); err != nil {
		// The status line is already sent; log and move on.
		slog.Default().Error("write response", "err", err)
	}
}

func writeError(w http.ResponseWriter, r *http.Request, status int, code, message string) {
	writeJSON(w, status, Envelope{Error: &Error{
		Code:      code,
		Message:   message,
		RequestID: RequestIDFrom(r.Context()),
	}})
}

// logError records the internal cause of a failed request. It is never returned to clients.
func logError(logger *slog.Logger, r *http.Request, err error) {
	logger.Error("request failed",
		"request_id", RequestIDFrom(r.Context()),
		"method", r.Method,
		"path", r.URL.Path,
		"err", err,
	)
}
