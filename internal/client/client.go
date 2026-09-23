// Package client speaks the HTTP API (spec §5) over the daemon's Unix
// socket. It is used by the CLI and the Review TUI.
package client

import (
	"context"
	"fmt"

	"github.com/NurramoX/ideation/internal/api"
)

// ProblemError is any non-2xx answer, carrying the server's problem+json
// document and its raw bytes (the CLI prints them verbatim with --json).
type ProblemError struct {
	Problem api.Problem
	Raw     []byte
}

func (e *ProblemError) Error() string {
	if e.Problem.Detail != "" {
		return fmt.Sprintf("%s: %s", e.Problem.Title, e.Problem.Detail)
	}
	return e.Problem.Title
}

// UnreachableError: the socket could not be dialled.
type UnreachableError struct{ Err error }

func (e *UnreachableError) Error() string { return "daemon unreachable: " + e.Err.Error() }
func (e *UnreachableError) Unwrap() error { return e.Err }

// ListParams are the query parameters of GET /ideas. Empty strings and zero
// numbers are left out, so the server applies its defaults.
type ListParams struct {
	Filter string
	Sort   string
	Order  string // "asc" or "desc"
	Limit  int
	Offset int
}

// Client is the API. Every write returns the Version after it. Errors are a
// *ProblemError, an *UnreachableError, or a transport or decoding error.
type Client interface {
	Ping(ctx context.Context) (api.Service, error)
	Create(ctx context.Context, req api.CreateRequest) (api.Idea, error)
	Get(ctx context.Context, id int64) (api.Idea, error)
	// Revalidate is a GET with If-None-Match: version. On a 304 it returns
	// notModified and a zero Idea.
	Revalidate(ctx context.Context, id int64, version int64) (idea api.Idea, notModified bool, err error)
	List(ctx context.Context, p ListParams) (api.List, error)
	Patch(ctx context.Context, id int64, pre api.Precondition, p api.Patch) (api.PatchedIdea, error)
	Body(ctx context.Context, id int64) (body []byte, version int64, err error)
	PutBody(ctx context.Context, id int64, pre api.Precondition, body []byte) (version int64, err error)
	PutTag(ctx context.Context, id int64, pre api.Precondition, tag string) (version int64, err error)
	DeleteTag(ctx context.Context, id int64, pre api.Precondition, tag string) (version int64, err error)
	PutAttribute(ctx context.Context, id int64, pre api.Precondition, key, value string) (version int64, err error)
	DeleteAttribute(ctx context.Context, id int64, pre api.Precondition, key string) (version int64, err error)
	MarkReviewed(ctx context.Context, id int64) error
	Delete(ctx context.Context, id int64, pre api.Precondition) error
	Tags(ctx context.Context) ([]api.TagCount, error)
	Attributes(ctx context.Context) ([]api.KeyCount, error)
	AttributeValues(ctx context.Context, key string) ([]api.ValueCount, error)
}

// New returns a Client that dials the Unix socket at sock. It does not dial
// until the first call.
func New(sock string) Client {
	return nil
}
