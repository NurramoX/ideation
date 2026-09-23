package server

import (
	"mime"
	"net/http"

	"github.com/NurramoX/ideation/internal/api"
)

// target reads the id and the optional If-Match of a tag or attribute
// operation. On failure it has already answered.
func target(w http.ResponseWriter, r *http.Request) (int64, api.Precondition, bool) {
	id, ok := pathID(w, r)
	if !ok {
		return 0, api.Precondition{}, false
	}
	pre, ok := precondition(w, r, false)
	return id, pre, ok
}

// written answers a tag or attribute operation: 204 with the new ETag.
func written(w http.ResponseWriter, version int64, err error) {
	if err != nil {
		storeError(w, err)
		return
	}
	setVersion(w, version)
	w.WriteHeader(http.StatusNoContent)
}

func (h *handler) putTag(w http.ResponseWriter, r *http.Request) {
	if id, pre, ok := target(w, r); ok {
		version, err := h.store.PutTag(r.Context(), id, pre, r.PathValue("tag"))
		written(w, version, err)
	}
}

func (h *handler) deleteTag(w http.ResponseWriter, r *http.Request) {
	if id, pre, ok := target(w, r); ok {
		version, err := h.store.DeleteTag(r.Context(), id, pre, r.PathValue("tag"))
		written(w, version, err)
	}
}

// putAttribute takes the raw request body as the value. The Content-Type's
// media type is not looked at, so text/plain, curl's form default and no
// Content-Type at all are all fine, but a charset other than UTF-8 is a 415.
// The store trims the value.
func (h *handler) putAttribute(w http.ResponseWriter, r *http.Request) {
	id, pre, ok := target(w, r)
	if !ok {
		return
	}
	if ct := r.Header.Get("Content-Type"); ct != "" {
		if _, params, err := mime.ParseMediaType(ct); err != nil || !utf8Charset(params) {
			fail(w, http.StatusUnsupportedMediaType, "the value must be sent as UTF-8 text")
			return
		}
	}
	value, ok := readRaw(w, r)
	if !ok {
		return
	}
	version, err := h.store.PutAttribute(r.Context(), id, pre, r.PathValue("key"), string(value))
	written(w, version, err)
}

func (h *handler) deleteAttribute(w http.ResponseWriter, r *http.Request) {
	if id, pre, ok := target(w, r); ok {
		version, err := h.store.DeleteAttribute(r.Context(), id, pre, r.PathValue("key"))
		written(w, version, err)
	}
}

// markReviewed needs no If-Match and moves no Version, so it sends no ETag.
func (h *handler) markReviewed(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	if err := h.store.MarkReviewed(r.Context(), id); err != nil {
		storeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
