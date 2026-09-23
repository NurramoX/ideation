package server

import (
	"context"
	"maps"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/NurramoX/ideation/internal/api"
	"github.com/NurramoX/ideation/internal/store"
)

// fakeStore is a minimal in-memory store.Store that keeps the interface's
// promises the server relies on: versions advance only on a real change,
// Preconditions are checked when not zero, status defaults to raw and cannot
// be removed, and bodies over api.MaxBody are ErrTooLarge. It does no other
// validation beyond an empty title. List ignores the filter and records the
// Query it was given.
type fakeStore struct {
	mu        sync.Mutex
	ideas     map[int64]*api.Idea
	next      int64
	lastQuery store.Query
	listed    bool
}

func newFakeStore() *fakeStore { return &fakeStore{ideas: map[int64]*api.Idea{}, next: 1} }

var fakeNow = api.Time{Time: time.Date(2026, 9, 22, 14, 3, 7, 412e6, time.UTC)}

func (f *fakeStore) Create(_ context.Context, req api.CreateRequest) (api.Idea, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if strings.TrimSpace(req.Title) == "" {
		return api.Idea{}, &store.InvalidError{Msg: "title is empty"}
	}
	if len(req.Body) > api.MaxBody {
		return api.Idea{}, store.ErrTooLarge
	}
	attrs := map[string]string{api.StatusKey: api.StatusRaw}
	maps.Copy(attrs, req.Attributes)
	tags := slices.Clone(req.Tags)
	if tags == nil {
		tags = []string{}
	}
	slices.Sort(tags)
	idea := &api.Idea{
		Meta: api.Meta{ID: f.next, Title: req.Title, Tags: tags, Attributes: attrs,
			Version: 1, CreatedAt: fakeNow, UpdatedAt: fakeNow},
		Body: req.Body,
	}
	f.ideas[idea.ID] = idea
	f.next++
	return clone(idea), nil
}

func (f *fakeStore) Get(_ context.Context, id int64) (api.Idea, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	idea, ok := f.ideas[id]
	if !ok {
		return api.Idea{}, store.ErrNotFound
	}
	return clone(idea), nil
}

func (f *fakeStore) List(_ context.Context, q store.Query) (api.List, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.lastQuery, f.listed = q, true
	var ids []int64
	for id := range f.ideas {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	l := api.List{Total: len(ids)}
	for _, id := range ids {
		l.Ideas = append(l.Ideas, clone(f.ideas[id]).Meta)
	}
	return l, nil
}

// write finds the idea, checks pre, applies change and advances the version
// when change reports a real change.
func (f *fakeStore) write(id int64, pre api.Precondition, change func(*api.Idea) (bool, error)) (int64, error) {
	idea, ok := f.ideas[id]
	if !ok {
		return 0, store.ErrNotFound
	}
	if !pre.Force && pre.Version > 0 && pre.Version != idea.Version {
		return 0, &store.StaleError{Current: idea.Version}
	}
	changed, err := change(idea)
	if err != nil {
		return 0, err
	}
	if changed {
		idea.Version++
	}
	return idea.Version, nil
}

func (f *fakeStore) Patch(_ context.Context, id int64, pre api.Precondition, p api.Patch) (api.Idea, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	_, err := f.write(id, pre, func(idea *api.Idea) (bool, error) {
		changed := false
		if p.Title != nil && *p.Title != idea.Title {
			if strings.TrimSpace(*p.Title) == "" {
				return false, &store.InvalidError{Msg: "title is empty"}
			}
			idea.Title, changed = *p.Title, true
		}
		if p.Body != nil && *p.Body != idea.Body {
			idea.Body, changed = *p.Body, true
		}
		if p.Tags != nil {
			tags := slices.Sorted(slices.Values(*p.Tags))
			if !slices.Equal(tags, idea.Tags) {
				idea.Tags, changed = tags, true
			}
		}
		for k, v := range p.Attributes {
			c, err := setAttr(idea, k, v)
			if err != nil {
				return false, err
			}
			changed = changed || c
		}
		return changed, nil
	})
	if err != nil {
		return api.Idea{}, err
	}
	return clone(f.ideas[id]), nil
}

func setAttr(idea *api.Idea, k string, v *string) (bool, error) {
	old, had := idea.Attributes[k]
	if v == nil {
		if k == api.StatusKey {
			return false, &store.InvalidError{Msg: "status cannot be removed"}
		}
		delete(idea.Attributes, k)
		return had, nil
	}
	if *v == "" {
		return false, &store.InvalidError{Msg: "empty value"}
	}
	idea.Attributes[k] = *v
	return !had || old != *v, nil
}

func (f *fakeStore) PutBody(_ context.Context, id int64, pre api.Precondition, body []byte) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.write(id, pre, func(idea *api.Idea) (bool, error) {
		if len(body) > api.MaxBody {
			return false, store.ErrTooLarge
		}
		changed := idea.Body != string(body)
		idea.Body = string(body)
		return changed, nil
	})
}

func (f *fakeStore) PutTag(_ context.Context, id int64, pre api.Precondition, tag string) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.write(id, pre, func(idea *api.Idea) (bool, error) {
		if slices.Contains(idea.Tags, tag) {
			return false, nil
		}
		idea.Tags = append(idea.Tags, tag)
		slices.Sort(idea.Tags)
		return true, nil
	})
}

func (f *fakeStore) DeleteTag(_ context.Context, id int64, pre api.Precondition, tag string) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.write(id, pre, func(idea *api.Idea) (bool, error) {
		i := slices.Index(idea.Tags, tag)
		if i < 0 {
			return false, nil
		}
		idea.Tags = slices.Delete(idea.Tags, i, i+1)
		return true, nil
	})
}

func (f *fakeStore) PutAttribute(_ context.Context, id int64, pre api.Precondition, key, value string) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.write(id, pre, func(idea *api.Idea) (bool, error) {
		return setAttr(idea, key, &value)
	})
}

func (f *fakeStore) DeleteAttribute(_ context.Context, id int64, pre api.Precondition, key string) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.write(id, pre, func(idea *api.Idea) (bool, error) {
		return setAttr(idea, key, nil)
	})
}

func (f *fakeStore) MarkReviewed(_ context.Context, id int64) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	idea, ok := f.ideas[id]
	if !ok {
		return store.ErrNotFound
	}
	t := fakeNow
	idea.ReviewedAt = &t
	return nil
}

func (f *fakeStore) Delete(_ context.Context, id int64, pre api.Precondition) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, err := f.write(id, pre, func(*api.Idea) (bool, error) { return false, nil }); err != nil {
		return err
	}
	delete(f.ideas, id)
	return nil
}

func (f *fakeStore) Tags(context.Context) ([]api.TagCount, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	counts := map[string]int{}
	for _, idea := range f.ideas {
		for _, t := range idea.Tags {
			counts[t]++
		}
	}
	var out []api.TagCount
	for _, t := range slices.Sorted(maps.Keys(counts)) {
		out = append(out, api.TagCount{Tag: t, Count: counts[t]})
	}
	return out, nil
}

func (f *fakeStore) Attributes(context.Context) ([]api.KeyCount, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	counts := map[string]int{}
	for _, idea := range f.ideas {
		for k := range idea.Attributes {
			counts[k]++
		}
	}
	var out []api.KeyCount
	for _, k := range slices.Sorted(maps.Keys(counts)) {
		out = append(out, api.KeyCount{Key: k, Count: counts[k]})
	}
	return out, nil
}

func (f *fakeStore) AttributeValues(_ context.Context, key string) ([]api.ValueCount, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	counts := map[string]int{}
	for _, idea := range f.ideas {
		if v, ok := idea.Attributes[key]; ok {
			counts[v]++
		}
	}
	var out []api.ValueCount
	for _, v := range slices.Sorted(maps.Keys(counts)) {
		out = append(out, api.ValueCount{Value: v, Count: counts[v]})
	}
	return out, nil
}

func (f *fakeStore) Close() error { return nil }

func clone(idea *api.Idea) api.Idea {
	c := *idea
	c.Tags = slices.Clone(idea.Tags)
	c.Attributes = maps.Clone(idea.Attributes)
	return c
}
