package server

import (
	"net/http"

	"github.com/NurramoX/ideation/internal/api"
)

func (h *handler) tags(w http.ResponseWriter, r *http.Request) {
	tags, err := h.store.Tags(r.Context())
	vocabulary(w, tags, err)
}

func (h *handler) attributes(w http.ResponseWriter, r *http.Request) {
	keys, err := h.store.Attributes(r.Context())
	vocabulary(w, keys, err)
}

func (h *handler) attributeValues(w http.ResponseWriter, r *http.Request) {
	values, err := h.store.AttributeValues(r.Context(), r.PathValue("key"))
	vocabulary(w, values, err)
}

// vocabulary answers a vocabulary endpoint with a JSON array, never null.
func vocabulary[T api.TagCount | api.KeyCount | api.ValueCount](w http.ResponseWriter, items []T, err error) {
	if err != nil {
		storeError(w, err)
		return
	}
	if items == nil {
		items = []T{}
	}
	writeJSON(w, http.StatusOK, items)
}
