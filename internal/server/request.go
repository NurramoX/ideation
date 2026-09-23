package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/NurramoX/ideation/internal/api"
)

// maxJSON caps a JSON request. A body at api.MaxBody may grow up to six-fold
// when every byte is escaped as \u00XX, so the cap leaves the store to answer
// 413 for an over-long body.
const maxJSON = 6*api.MaxBody + 1<<20

// pathID reads the {id} path segment: a positive decimal with digits only.
// On failure it has already answered 400.
func pathID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	s := r.PathValue("id")
	for _, c := range s {
		if c < '0' || c > '9' {
			fail(w, http.StatusBadRequest, fmt.Sprintf("id %q is not a decimal number", s))
			return 0, false
		}
	}
	id, err := strconv.ParseInt(s, 10, 64)
	if err != nil || id < 1 {
		fail(w, http.StatusBadRequest, fmt.Sprintf("id %q is not a valid idea id", s))
		return 0, false
	}
	return id, true
}

// decodeJSON reads exactly one JSON value into v, rejecting unknown fields
// and trailing data. Malformed JSON is 400 and an oversized request 413. On
// failure it has already answered.
func decodeJSON(w http.ResponseWriter, r *http.Request, v any) bool {
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxJSON))
	dec.DisallowUnknownFields()
	err := dec.Decode(v)
	if err == nil {
		if _, extra := dec.Token(); extra != io.EOF {
			err = errors.New("unexpected data after the JSON value")
		}
	}
	var tooBig *http.MaxBytesError
	switch {
	case err == nil:
		return true
	case errors.As(err, &tooBig):
		fail(w, http.StatusRequestEntityTooLarge, "request body too large")
	case errors.Is(err, io.EOF):
		fail(w, http.StatusBadRequest, "empty request body, want JSON")
	default:
		fail(w, http.StatusBadRequest, "malformed JSON: "+err.Error())
	}
	return false
}
