// Package server is the HTTP API (spec §5) over a store.Store.
package server

import (
	"net/http"
	"time"

	"github.com/NurramoX/ideation/internal/store"
)

// New returns the API handler. now is the clock used to resolve relative
// dates in filters, in its own location.
func New(s store.Store, now func() time.Time) http.Handler {
	return http.NotFoundHandler()
}
