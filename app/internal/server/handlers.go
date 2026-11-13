package server

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/adrianabelgarcia/aws-ecs-fargate-service/app/internal/items"
)

const (
	defaultPageSize = 20
	maxOffset       = 1_000_000
	maxIDLength     = 64
	maxBodyBytes    = 1 << 16
)

type statusBody struct {
	Status  string `json:"status"`
	Version string `json:"version,omitempty"`
}

type listMeta struct {
	Total  int `json:"total"`
	Limit  int `json:"limit"`
	Offset int `json:"offset"`
}

type createRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type handlers struct {
	logger    *slog.Logger
	items     *items.Service
	readiness *Readiness
	version   string
	maxPage   int
}

// healthz is a liveness probe: it only proves the process can serve HTTP.
func (h *handlers) healthz(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, Envelope{Data: statusBody{Status: "ok"}})
}

// readyz is a readiness probe: it fails while the instance is draining.
func (h *handlers) readyz(w http.ResponseWriter, r *http.Request) {
	if !h.readiness.IsReady() {
		writeError(w, r, http.StatusServiceUnavailable, "not_ready", "service is draining")
		return
	}
	writeJSON(w, http.StatusOK, Envelope{Data: statusBody{Status: "ready", Version: h.version}})
}

func (h *handlers) listItems(w http.ResponseWriter, r *http.Request) {
	limit, offset, err := parsePagination(r, h.maxPage)
	if err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid_query", err.Error())
		return
	}
	page, err := h.items.List(r.Context(), offset, limit)
	if err != nil {
		h.serverError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, Envelope{
		Data: page.Items,
		Meta: listMeta{Total: page.Total, Limit: page.Limit, Offset: page.Offset},
	})
}

func (h *handlers) getItem(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" || len(id) > maxIDLength {
		writeError(w, r, http.StatusBadRequest, "invalid_id", "item id is malformed")
		return
	}
	item, err := h.items.Get(r.Context(), id)
	switch {
	case errors.Is(err, items.ErrNotFound):
		writeError(w, r, http.StatusNotFound, "not_found", "item not found")
	case err != nil:
		h.serverError(w, r, err)
	default:
		writeJSON(w, http.StatusOK, Envelope{Data: item})
	}
}

func (h *handlers) createItem(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()

	var in createRequest
	if err := dec.Decode(&in); err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			writeError(w, r, http.StatusRequestEntityTooLarge, "payload_too_large", "request body exceeds limit")
			return
		}
		writeError(w, r, http.StatusBadRequest, "invalid_json", "request body must be a valid JSON object")
		return
	}

	item, err := h.items.Create(r.Context(), items.CreateInput{Name: in.Name, Description: in.Description})
	var vErr *items.ValidationError
	switch {
	case errors.As(err, &vErr):
		writeError(w, r, http.StatusUnprocessableEntity, "validation_failed", vErr.Message)
	case err != nil:
		h.serverError(w, r, err)
	default:
		w.Header().Set("Location", "/api/v1/items/"+item.ID)
		writeJSON(w, http.StatusCreated, Envelope{Data: item})
	}
}

// serverError logs the detailed cause server-side and returns a generic message.
func (h *handlers) serverError(w http.ResponseWriter, r *http.Request, err error) {
	logError(h.logger, r, err)
	writeError(w, r, http.StatusInternalServerError, "internal", "internal server error")
}

func parsePagination(r *http.Request, maxPage int) (limit, offset int, err error) {
	q := r.URL.Query()
	limit, offset = defaultPageSize, 0
	if raw := q.Get("limit"); raw != "" {
		n, convErr := strconv.Atoi(raw)
		if convErr != nil || n < 1 || n > maxPage {
			return 0, 0, errors.New("limit must be an integer between 1 and " + strconv.Itoa(maxPage))
		}
		limit = n
	}
	if raw := q.Get("offset"); raw != "" {
		n, convErr := strconv.Atoi(raw)
		if convErr != nil || n < 0 || n > maxOffset {
			return 0, 0, errors.New("offset must be an integer between 0 and " + strconv.Itoa(maxOffset))
		}
		offset = n
	}
	return limit, offset, nil
}
