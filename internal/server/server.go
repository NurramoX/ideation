// Package server is the HTTP API (spec §5) over a store.Store.
package server

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/NurramoX/ideation/internal/api"
	"github.com/NurramoX/ideation/internal/store"
)

// New returns the API handler. now is the clock used to resolve relative
// dates in filters, in its own location.
func New(s store.Store, now func() time.Time) http.Handler {
	h := &handler{store: s, now: now}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", h.service)
	mux.HandleFunc("GET /ideas", h.list)
	mux.HandleFunc("POST /ideas", h.create)
	mux.HandleFunc("GET /ideas/{id}", h.get)
	mux.HandleFunc("PATCH /ideas/{id}", h.patch)
	mux.HandleFunc("DELETE /ideas/{id}", h.delete)
	mux.HandleFunc("GET /ideas/{id}/body", h.getBody)
	mux.HandleFunc("PUT /ideas/{id}/body", h.putBody)
	mux.HandleFunc("PUT /ideas/{id}/tags/{tag}", h.putTag)
	mux.HandleFunc("DELETE /ideas/{id}/tags/{tag}", h.deleteTag)
	mux.HandleFunc("PUT /ideas/{id}/attributes/{key}", h.putAttribute)
	mux.HandleFunc("DELETE /ideas/{id}/attributes/{key}", h.deleteAttribute)
	mux.HandleFunc("PUT /ideas/{id}/reviewed", h.markReviewed)
	mux.HandleFunc("GET /tags", h.tags)
	mux.HandleFunc("GET /attributes", h.attributes)
	mux.HandleFunc("GET /attributes/{key}", h.attributeValues)
	return problems(mux)
}

type handler struct {
	store store.Store
	now   func() time.Time
}

func (h *handler) service(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, api.Service{Service: "ideation", API: api.APIVersion})
}

// writeJSON sends v as an application/json body.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
