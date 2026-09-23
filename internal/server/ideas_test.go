package server

import (
	"net/http"
	"strings"
	"testing"

	"github.com/NurramoX/ideation/internal/api"
)

func TestCreateReturnsTheEnvelope(t *testing.T) {
	ts, _ := newTestServer(t)
	resp := do(t, ts, "POST", "/ideas", `{"title":"Borrow checker","body":"# hi\n","tags":["rust"],"attributes":{"status":"active"}}`)
	wantStatus(t, resp, http.StatusCreated)
	if got := resp.Header.Get("Location"); got != "/ideas/1" {
		t.Fatalf("Location %q", got)
	}
	if got := resp.Header.Get("ETag"); got != `"1"` {
		t.Fatalf("ETag %q", got)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "application/json" {
		t.Fatalf("Content-Type %q", ct)
	}
	idea := decode[api.Idea](t, resp)
	if idea.ID != 1 || idea.Title != "Borrow checker" || idea.Body != "# hi\n" || idea.Version != 1 ||
		idea.Attributes["status"] != "active" || len(idea.Tags) != 1 {
		t.Fatalf("envelope %+v", idea)
	}
}

func TestCreateRejectsBadRequests(t *testing.T) {
	ts, _ := newTestServer(t)
	for _, tc := range []struct {
		name, body string
		code       int
	}{
		{"malformed JSON", `{"title":`, http.StatusBadRequest},
		{"no body", ``, http.StatusBadRequest},
		{"trailing data", `{"title":"a"} {}`, http.StatusBadRequest},
		{"unknown field", `{"title":"a","colour":"red"}`, http.StatusBadRequest},
		{"wrong type", `{"title":5}`, http.StatusBadRequest},
		{"missing title", `{"body":"x"}`, http.StatusUnprocessableEntity},
		{"null title", `{"title":null}`, http.StatusUnprocessableEntity},
		{"empty title", `{"title":"  "}`, http.StatusUnprocessableEntity},
	} {
		t.Run(tc.name, func(t *testing.T) {
			wantProblem(t, do(t, ts, "POST", "/ideas", tc.body), tc.code)
		})
	}
}

func TestCreateOverMaxBodyIs413(t *testing.T) {
	ts, _ := newTestServer(t)
	big := strings.Repeat("a", api.MaxBody+1)
	wantProblem(t, do(t, ts, "POST", "/ideas", `{"title":"a","body":"`+big+`"}`), http.StatusRequestEntityTooLarge)
}

func TestGetIdea(t *testing.T) {
	ts, _ := newTestServer(t)
	create(t, ts, `{"title":"a","body":"text"}`)

	resp := do(t, ts, "GET", "/ideas/1", "")
	wantStatus(t, resp, http.StatusOK)
	if got := resp.Header.Get("ETag"); got != `"1"` {
		t.Fatalf("ETag %q", got)
	}
	idea := decode[api.Idea](t, resp)
	if idea.Body != "text" || idea.Version != 1 {
		t.Fatalf("envelope %+v", idea)
	}

	head := do(t, ts, "HEAD", "/ideas/1", "")
	wantStatus(t, head, http.StatusOK)
	if head.Header.Get("ETag") != `"1"` || readBody(t, head) != "" {
		t.Fatal("HEAD should carry the ETag and no body")
	}
}

func TestGetIdeaErrors(t *testing.T) {
	ts, _ := newTestServer(t)
	if p := wantProblem(t, do(t, ts, "GET", "/ideas/7", ""), http.StatusNotFound); p.Detail != "idea 7 not found" {
		t.Errorf("detail %q, want it to name the id", p.Detail)
	}
	for _, id := range []string{"abc", "0", "-1", "+1", "1.0", "99999999999999999999"} {
		wantProblem(t, do(t, ts, "GET", "/ideas/"+id, ""), http.StatusBadRequest)
	}
}

func TestGetIdeaIfNoneMatch(t *testing.T) {
	ts, _ := newTestServer(t)
	create(t, ts, `{"title":"a"}`)
	for _, inm := range []string{`"1"`, `W/"1"`, `"4", "1"`, `*`} {
		resp := do(t, ts, "GET", "/ideas/1", "", "If-None-Match", inm)
		wantStatus(t, resp, http.StatusNotModified)
		if resp.Header.Get("ETag") != `"1"` {
			t.Fatalf("304 for %s without ETag", inm)
		}
	}
	wantStatus(t, do(t, ts, "GET", "/ideas/1", "", "If-None-Match", `"2"`), http.StatusOK)
	wantStatus(t, do(t, ts, "HEAD", "/ideas/1", "", "If-None-Match", `"1"`), http.StatusNotModified)
}

func TestPatchRequiresIfMatch(t *testing.T) {
	ts, _ := newTestServer(t)
	create(t, ts, `{"title":"a"}`)
	wantProblem(t, do(t, ts, "PATCH", "/ideas/1", `{"title":"b"}`), http.StatusPreconditionRequired)
	wantProblem(t, do(t, ts, "PATCH", "/ideas/1", `{"title":"b"}`, "If-Match", "seven"), http.StatusBadRequest)
	wantProblem(t, do(t, ts, "PATCH", "/ideas/1", `{"title":"b"}`, "If-Match", `W/"1"`), http.StatusBadRequest)
}

func TestPatchStaleIs412WithCurrentVersion(t *testing.T) {
	ts, _ := newTestServer(t)
	create(t, ts, `{"title":"a"}`)
	wantStatus(t, do(t, ts, "PATCH", "/ideas/1", `{"title":"b"}`, "If-Match", `"1"`), http.StatusOK)

	resp := do(t, ts, "PATCH", "/ideas/1", `{"title":"c"}`, "If-Match", `"1"`)
	p := wantProblem(t, resp, http.StatusPreconditionFailed)
	if p.CurrentVersion != 2 || resp.Header.Get("ETag") != `"2"` {
		t.Fatalf("412 %+v, ETag %q", p, resp.Header.Get("ETag"))
	}
}

func TestPatchOmitsBodyUnlessSent(t *testing.T) {
	ts, _ := newTestServer(t)
	create(t, ts, `{"title":"a","body":"old"}`)

	resp := do(t, ts, "PATCH", "/ideas/1", `{"title":"b","tags":["x"],"attributes":{"effort":"small"}}`, "If-Match", `"1"`)
	wantStatus(t, resp, http.StatusOK)
	if resp.Header.Get("ETag") != `"2"` {
		t.Fatalf("ETag %q", resp.Header.Get("ETag"))
	}
	raw := readBody(t, resp)
	if strings.Contains(raw, `"body"`) {
		t.Fatalf("PATCH without body returned one: %s", raw)
	}
	if !strings.Contains(raw, `"title":"b"`) || !strings.Contains(raw, `"version":2`) || !strings.Contains(raw, `"effort":"small"`) {
		t.Fatalf("envelope %s", raw)
	}

	resp = do(t, ts, "PATCH", "/ideas/1", `{"body":""}`, "If-Match", "*")
	wantStatus(t, resp, http.StatusOK)
	if raw := readBody(t, resp); !strings.Contains(raw, `"body":""`) {
		t.Fatalf("PATCH with an empty body should return it: %s", raw)
	}
}

func TestPatchRemovesAttributesWithNull(t *testing.T) {
	ts, _ := newTestServer(t)
	create(t, ts, `{"title":"a","attributes":{"effort":"small"}}`)
	resp := do(t, ts, "PATCH", "/ideas/1", `{"attributes":{"effort":null}}`, "If-Match", `"1"`)
	wantStatus(t, resp, http.StatusOK)
	if idea := decode[api.PatchedIdea](t, resp); len(idea.Attributes) != 1 {
		t.Fatalf("attributes %v", idea.Attributes)
	}
	wantProblem(t, do(t, ts, "PATCH", "/ideas/1", `{"attributes":{"status":null}}`, "If-Match", "*"), http.StatusUnprocessableEntity)
}

func TestPatchRejectsBadRequests(t *testing.T) {
	ts, _ := newTestServer(t)
	create(t, ts, `{"title":"a"}`)
	for _, tc := range []struct {
		name, body string
		code       int
	}{
		{"malformed", `{`, http.StatusBadRequest},
		{"not an object", `[]`, http.StatusBadRequest},
		{"unknown field", `{"status":"done"}`, http.StatusBadRequest},
		{"wrong type", `{"tags":"x"}`, http.StatusBadRequest},
		{"null title", `{"title":null}`, http.StatusUnprocessableEntity},
		{"null body", `{"body":null}`, http.StatusUnprocessableEntity},
		{"null tags", `{"tags":null}`, http.StatusUnprocessableEntity},
		{"null attributes", `{"attributes":null}`, http.StatusUnprocessableEntity},
		{"empty title", `{"title":""}`, http.StatusUnprocessableEntity},
	} {
		t.Run(tc.name, func(t *testing.T) {
			wantProblem(t, do(t, ts, "PATCH", "/ideas/1", tc.body, "If-Match", "*"), tc.code)
		})
	}
	wantProblem(t, do(t, ts, "PATCH", "/ideas/9", `{}`, "If-Match", "*"), http.StatusNotFound)
}

func TestDeleteIdea(t *testing.T) {
	ts, _ := newTestServer(t)
	create(t, ts, `{"title":"a"}`)
	wantProblem(t, do(t, ts, "DELETE", "/ideas/1", ""), http.StatusPreconditionRequired)
	wantProblem(t, do(t, ts, "DELETE", "/ideas/1", "", "If-Match", `"3"`), http.StatusPreconditionFailed)
	wantStatus(t, do(t, ts, "DELETE", "/ideas/1", "", "If-Match", `"1"`), http.StatusNoContent)
	wantProblem(t, do(t, ts, "GET", "/ideas/1", ""), http.StatusNotFound)
	wantProblem(t, do(t, ts, "DELETE", "/ideas/1", "", "If-Match", "*"), http.StatusNotFound)
}
