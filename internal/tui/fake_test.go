package tui

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/NurramoX/ideation/internal/api"
	"github.com/NurramoX/ideation/internal/client"
)

// fakeClient is an in-memory client.Client. It keeps ideas in insertion
// order, checks If-Match like the server, and records every call.
type fakeClient struct {
	mu    sync.Mutex
	ideas map[int64]*api.Idea
	order []int64
	next  int64
	now   time.Time

	// lists maps a filter to the ids it selects, in the server's order;
	// a filter not in the map selects every idea.
	lists map[string][]int64
	// listErr maps a filter to the error List returns for it.
	listErr map[string]error
	// failNext makes the next call whose recorded name starts with the key
	// fail with the error.
	failNext map[string]error

	calls  []string
	params []client.ListParams
}

func newFake(now time.Time) *fakeClient {
	return &fakeClient{
		ideas:    map[int64]*api.Idea{},
		now:      now,
		lists:    map[string][]int64{},
		listErr:  map[string]error{},
		failNext: map[string]error{},
		next:     1,
	}
}

// add creates an idea; age is how long ago it was created and updated.
func (f *fakeClient) add(title string, age time.Duration, tags ...string) int64 {
	f.mu.Lock()
	defer f.mu.Unlock()
	id := f.next
	f.next++
	at := api.Time{Time: f.now.Add(-age)}
	tags = append([]string{}, tags...)
	slices.Sort(tags)
	f.ideas[id] = &api.Idea{
		Meta: api.Meta{
			ID: id, Title: title, Tags: tags, Version: 1,
			Attributes: map[string]string{api.StatusKey: api.StatusRaw},
			CreatedAt:  at, UpdatedAt: at,
		},
		Body: "Body of " + title + ".\n",
	}
	f.order = append(f.order, id)
	return id
}

// bump changes an idea behind the TUI's back, as another writer would.
func (f *fakeClient) bump(id int64, body string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.ideas[id].Body = body
	f.ideas[id].Version++
}

func (f *fakeClient) idea(id int64) *api.Idea {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.ideas[id]
}

func (f *fakeClient) record(format string, args ...any) error {
	call := fmt.Sprintf(format, args...)
	f.calls = append(f.calls, call)
	for prefix, err := range f.failNext {
		if strings.HasPrefix(call, prefix) {
			delete(f.failNext, prefix)
			return err
		}
	}
	return nil
}

// writes returns the recorded calls other than reads.
func (f *fakeClient) writes() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []string
	for _, c := range f.calls {
		if !strings.HasPrefix(c, "GET") && !strings.HasPrefix(c, "LIST") {
			out = append(out, c)
		}
	}
	return out
}

func (f *fakeClient) countCalls(prefix string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	n := 0
	for _, c := range f.calls {
		if strings.HasPrefix(c, prefix) {
			n++
		}
	}
	return n
}

func problem(status int, detail string) *client.ProblemError {
	return &client.ProblemError{Problem: api.Problem{Title: fmt.Sprintf("status %d", status), Status: status, Detail: detail}}
}

func (f *fakeClient) lookup(id int64) (*api.Idea, error) {
	i, ok := f.ideas[id]
	if !ok {
		return nil, problem(404, "no idea "+fmt.Sprint(id))
	}
	return i, nil
}

func (f *fakeClient) check(i *api.Idea, pre api.Precondition) error {
	if pre.IsZero() || pre.Force || pre.Version == i.Version {
		return nil
	}
	p := problem(412, "stale")
	p.Problem.CurrentVersion = i.Version
	return p
}

func copyIdea(i *api.Idea) api.Idea {
	c := *i
	c.Tags = slices.Clone(i.Tags)
	c.Attributes = map[string]string{}
	for k, v := range i.Attributes {
		c.Attributes[k] = v
	}
	return c
}

func (f *fakeClient) Ping(ctx context.Context) (api.Service, error) {
	return api.Service{Service: "ideation", API: api.APIVersion}, nil
}

func (f *fakeClient) Create(ctx context.Context, req api.CreateRequest) (api.Idea, error) {
	panic("not used by the TUI")
}

func (f *fakeClient) Get(ctx context.Context, id int64) (api.Idea, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.record("GET %d", id); err != nil {
		return api.Idea{}, err
	}
	i, err := f.lookup(id)
	if err != nil {
		return api.Idea{}, err
	}
	return copyIdea(i), nil
}

func (f *fakeClient) Revalidate(ctx context.Context, id int64, version int64) (api.Idea, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.record("GET %d if-none-match %d", id, version); err != nil {
		return api.Idea{}, false, err
	}
	i, err := f.lookup(id)
	if err != nil {
		return api.Idea{}, false, err
	}
	if i.Version == version {
		return api.Idea{}, true, nil
	}
	return copyIdea(i), false, nil
}

func (f *fakeClient) List(ctx context.Context, p client.ListParams) (api.List, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.params = append(f.params, p)
	if err := f.record("LIST %q", p.Filter); err != nil {
		return api.List{}, err
	}
	if err := f.listErr[p.Filter]; err != nil {
		return api.List{}, err
	}
	ids, ok := f.lists[p.Filter]
	if !ok {
		ids = f.order
	}
	out := api.List{Ideas: []api.Meta{}}
	for _, id := range ids {
		if i, ok := f.ideas[id]; ok {
			out.Ideas = append(out.Ideas, copyIdea(i).Meta)
		}
	}
	out.Total = len(out.Ideas)
	return out, nil
}

func (f *fakeClient) Patch(ctx context.Context, id int64, pre api.Precondition, p api.Patch) (api.PatchedIdea, error) {
	panic("not used by the TUI")
}

func (f *fakeClient) Body(ctx context.Context, id int64) ([]byte, int64, error) {
	panic("not used by the TUI")
}

func (f *fakeClient) PutBody(ctx context.Context, id int64, pre api.Precondition, body []byte) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.record("PUT %d body if-match %s", id, pre.Header()); err != nil {
		return 0, err
	}
	i, err := f.lookup(id)
	if err != nil {
		return 0, err
	}
	if err := f.check(i, pre); err != nil {
		return 0, err
	}
	if i.Body != string(body) {
		i.Body = string(body)
		i.Version++
	}
	return i.Version, nil
}

func (f *fakeClient) PutTag(ctx context.Context, id int64, pre api.Precondition, tag string) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.record("PUT %d tag %s", id, tag); err != nil {
		return 0, err
	}
	i, err := f.lookup(id)
	if err != nil {
		return 0, err
	}
	if !slices.Contains(i.Tags, tag) {
		i.Tags = append(i.Tags, tag)
		slices.Sort(i.Tags)
		i.Version++
	}
	return i.Version, nil
}

func (f *fakeClient) DeleteTag(ctx context.Context, id int64, pre api.Precondition, tag string) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.record("DELETE %d tag %s", id, tag); err != nil {
		return 0, err
	}
	i, err := f.lookup(id)
	if err != nil {
		return 0, err
	}
	if n := slices.Index(i.Tags, tag); n >= 0 {
		i.Tags = slices.Delete(i.Tags, n, n+1)
		i.Version++
	}
	return i.Version, nil
}

func (f *fakeClient) PutAttribute(ctx context.Context, id int64, pre api.Precondition, key, value string) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.record("PUT %d attribute %s=%s", id, key, value); err != nil {
		return 0, err
	}
	i, err := f.lookup(id)
	if err != nil {
		return 0, err
	}
	if i.Attributes[key] != value {
		i.Attributes[key] = value
		i.Version++
	}
	return i.Version, nil
}

func (f *fakeClient) DeleteAttribute(ctx context.Context, id int64, pre api.Precondition, key string) (int64, error) {
	panic("attributes are read-only in Review")
}

func (f *fakeClient) MarkReviewed(ctx context.Context, id int64) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.record("PUT %d reviewed", id); err != nil {
		return err
	}
	i, err := f.lookup(id)
	if err != nil {
		return err
	}
	i.ReviewedAt = &api.Time{Time: f.now}
	return nil
}

func (f *fakeClient) Delete(ctx context.Context, id int64, pre api.Precondition) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.record("DELETE %d if-match %s", id, pre.Header()); err != nil {
		return err
	}
	i, err := f.lookup(id)
	if err != nil {
		return err
	}
	if err := f.check(i, pre); err != nil {
		return err
	}
	delete(f.ideas, id)
	f.order = slices.DeleteFunc(f.order, func(x int64) bool { return x == id })
	return nil
}

func (f *fakeClient) Tags(ctx context.Context) ([]api.TagCount, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	counts := map[string]int{}
	for _, i := range f.ideas {
		for _, t := range i.Tags {
			counts[t]++
		}
	}
	out := []api.TagCount{}
	for t, n := range counts {
		out = append(out, api.TagCount{Tag: t, Count: n})
	}
	slices.SortFunc(out, func(a, b api.TagCount) int { return strings.Compare(a.Tag, b.Tag) })
	return out, nil
}

func (f *fakeClient) Attributes(ctx context.Context) ([]api.KeyCount, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	counts := map[string]int{}
	for _, i := range f.ideas {
		for k := range i.Attributes {
			counts[k]++
		}
	}
	out := []api.KeyCount{}
	for k, n := range counts {
		out = append(out, api.KeyCount{Key: k, Count: n})
	}
	slices.SortFunc(out, func(a, b api.KeyCount) int { return strings.Compare(a.Key, b.Key) })
	return out, nil
}

func (f *fakeClient) AttributeValues(ctx context.Context, key string) ([]api.ValueCount, error) {
	return []api.ValueCount{}, nil
}
