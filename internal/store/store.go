// Package store keeps ideas in SQLite (spec §2, §3): validation of every
// data-model rule, migrations, the full-text index and the Filter compiler.
// Only the daemon opens it.
package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/NurramoX/ideation/internal/api"
	"github.com/NurramoX/ideation/internal/filter"

	_ "modernc.org/sqlite"
)

// ErrNotFound: an unknown or deleted id (404).
var ErrNotFound = errors.New("idea not found")

// ErrTooLarge: a body over api.MaxBody (413).
var ErrTooLarge = errors.New("body over 10 MB")

// InvalidError: content that breaks a data-model rule (422).
type InvalidError struct{ Msg string }

func (e *InvalidError) Error() string { return e.Msg }

// StaleError: the Precondition named a Version other than the current one
// (412).
type StaleError struct{ Current int64 }

func (e *StaleError) Error() string {
	return fmt.Sprintf("stale version, current is %d", e.Current)
}

// Sort fields.
const (
	SortUpdated  = "updated"
	SortCreated  = "created"
	SortReviewed = "reviewed" // never-reviewed counts as oldest
	SortTitle    = "title"
	SortRank     = "rank" // only with a ranked text term
)

// Query selects and orders ideas. The server has already resolved the
// defaults, so Sort and Desc are always set deliberately, and rejected
// sort=rank without a ranked term.
type Query struct {
	Filter filter.Expr // nil matches every idea
	Sort   string
	Desc   bool
	Limit  int // 0 means no limit
	Offset int
}

// Store is the idea store. Every write validates its input (InvalidError,
// ErrTooLarge), checks the Precondition when it is not zero (StaleError;
// Force always passes), and returns the Version after the write. A write that
// changes nothing moves neither version nor updated_at. The server, not the
// store, decides where a Precondition is required.
type Store interface {
	Create(ctx context.Context, req api.CreateRequest) (api.Idea, error)
	Get(ctx context.Context, id int64) (api.Idea, error)
	List(ctx context.Context, q Query) (api.List, error)
	Patch(ctx context.Context, id int64, pre api.Precondition, p api.Patch) (api.Idea, error)
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
	// Close checkpoints the WAL (TRUNCATE) and closes the database.
	Close() error
}

// Open opens (creating if needed) the database at path and runs the
// migrations. now is the clock for every timestamp the store sets.
func Open(path string, now func() time.Time) (Store, error) {
	return nil, errors.New("store.Open: not implemented")
}
