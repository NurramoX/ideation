package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"slices"
	"time"

	"github.com/NurramoX/ideation/internal/api"
)

// metaColumns selects an idea's metadata from idea i, tags and attributes
// included as JSON; scanMeta reads them back.
const metaColumns = `
	i.id, i.title, i.version, i.created_at, i.updated_at, i.reviewed_at,
	(SELECT json_group_array(tag) FROM idea_tag WHERE idea_id = i.id),
	(SELECT json_group_object(key, value) FROM idea_attribute WHERE idea_id = i.id)`

type scanner interface{ Scan(dest ...any) error }

func scanMeta(row scanner, extra ...any) (api.Meta, error) {
	var (
		m                          api.Meta
		created, updated, tagsJSON string
		attrsJSON                  string
		reviewed                   sql.NullString
	)
	dest := append([]any{&m.ID, &m.Title, &m.Version, &created, &updated, &reviewed, &tagsJSON, &attrsJSON}, extra...)
	if err := row.Scan(dest...); err != nil {
		return m, err
	}
	var err error
	if m.CreatedAt, err = parseStamp(created); err != nil {
		return m, err
	}
	if m.UpdatedAt, err = parseStamp(updated); err != nil {
		return m, err
	}
	if reviewed.Valid {
		t, err := parseStamp(reviewed.String)
		if err != nil {
			return m, err
		}
		m.ReviewedAt = &t
	}
	if err := json.Unmarshal([]byte(tagsJSON), &m.Tags); err != nil {
		return m, err
	}
	slices.Sort(m.Tags)
	if err := json.Unmarshal([]byte(attrsJSON), &m.Attributes); err != nil {
		return m, err
	}
	return m, nil
}

func parseStamp(s string) (api.Time, error) {
	t, err := time.Parse(api.TimeLayout, s)
	return api.Time{Time: t}, err
}

type querier interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
}

// load reads the whole idea id.
func load(ctx context.Context, q querier, id int64) (api.Idea, error) {
	var idea api.Idea
	row := q.QueryRowContext(ctx, `SELECT `+metaColumns+`, i.body FROM idea i WHERE i.id = ?`, id)
	m, err := scanMeta(row, &idea.Body)
	if errors.Is(err, sql.ErrNoRows) {
		return idea, ErrNotFound
	}
	idea.Meta = m
	return idea, err
}

func (s *sqliteStore) Get(ctx context.Context, id int64) (api.Idea, error) {
	return load(ctx, s.db, id)
}

// Tags lists every tag in use with the number of ideas carrying it, sorted
// by tag.
func (s *sqliteStore) Tags(ctx context.Context) ([]api.TagCount, error) {
	out := []api.TagCount{}
	err := each(ctx, s.db, `SELECT tag, count(*) FROM idea_tag GROUP BY tag ORDER BY tag`, nil, func(r *sql.Rows) error {
		var c api.TagCount
		if err := r.Scan(&c.Tag, &c.Count); err != nil {
			return err
		}
		out = append(out, c)
		return nil
	})
	return out, err
}

// Attributes lists every attribute key in use, status included, with the
// number of ideas carrying it, sorted by key.
func (s *sqliteStore) Attributes(ctx context.Context) ([]api.KeyCount, error) {
	out := []api.KeyCount{}
	err := each(ctx, s.db, `SELECT key, count(*) FROM idea_attribute GROUP BY key ORDER BY key`, nil, func(r *sql.Rows) error {
		var c api.KeyCount
		if err := r.Scan(&c.Key, &c.Count); err != nil {
			return err
		}
		out = append(out, c)
		return nil
	})
	return out, err
}

// AttributeValues lists the values of key in use, exactly as stored, with
// the number of ideas carrying each, sorted by value. The key is lowercased;
// an unknown or invalid key has no values.
func (s *sqliteStore) AttributeValues(ctx context.Context, key string) ([]api.ValueCount, error) {
	out := []api.ValueCount{}
	err := each(ctx, s.db, `
		SELECT value, count(*) FROM idea_attribute WHERE key = ?
		GROUP BY value ORDER BY value`, []any{api.LowerLabel(key)}, func(r *sql.Rows) error {
		var c api.ValueCount
		if err := r.Scan(&c.Value, &c.Count); err != nil {
			return err
		}
		out = append(out, c)
		return nil
	})
	return out, err
}

// each runs query and calls fn on every row.
func each(ctx context.Context, q querier, query string, args []any, fn func(*sql.Rows) error) error {
	rows, err := q.QueryContext(ctx, query, args...)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		if err := fn(rows); err != nil {
			return err
		}
	}
	return rows.Err()
}
