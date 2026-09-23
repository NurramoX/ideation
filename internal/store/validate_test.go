package store

import (
	"errors"
	"strings"
	"testing"

	"github.com/NurramoX/ideation/internal/api"
)

func TestCreateRejectsInvalidContent(t *testing.T) {
	tags := func(n int) []string {
		var out []string
		for i := range n {
			out = append(out, "t"+strings.Repeat("x", i))
		}
		return out
	}
	attrs := func(n int) map[string]string {
		out := map[string]string{}
		for i := range n {
			out["k"+strings.Repeat("x", i)] = "v"
		}
		return out
	}
	for name, req := range map[string]api.CreateRequest{
		"empty title":             {Title: ""},
		"blank title":             {Title: " \t "},
		"title over 400 chars":    {Title: strings.Repeat("é", 401)},
		"multi-line title":        {Title: "one\ntwo"},
		"title with a control":    {Title: "bell\a"},
		"title with a separator":  {Title: "one two"},
		"title not UTF-8":         {Title: "bad \xff"},
		"body not UTF-8":          {Title: "t", Body: "bad \xc3"},
		"empty tag":               {Title: "t", Tags: []string{""}},
		"tag with a space":        {Title: "t", Tags: []string{"a b"}},
		"tag starting with -":     {Title: "t", Tags: []string{"-a"}},
		"tag starting with _":     {Title: "t", Tags: []string{"_a"}},
		"non-ASCII tag":           {Title: "t", Tags: []string{"café"}},
		"Kelvin-sign tag":         {Title: "t", Tags: []string{"K"}},
		"tag over 128 bytes":      {Title: "t", Tags: []string{strings.Repeat("a", 129)}},
		"129 tags":                {Title: "t", Tags: tags(129)},
		"invalid key":             {Title: "t", Attributes: map[string]string{"a.b": "v"}},
		"key over 128 bytes":      {Title: "t", Attributes: map[string]string{strings.Repeat("a", 129): "v"}},
		"empty value":             {Title: "t", Attributes: map[string]string{"k": ""}},
		"blank value":             {Title: "t", Attributes: map[string]string{"k": "  "}},
		"value over 2000 bytes":   {Title: "t", Attributes: map[string]string{"k": strings.Repeat("a", 2001)}},
		"multi-line value":        {Title: "t", Attributes: map[string]string{"k": "a\nb"}},
		"value not UTF-8":         {Title: "t", Attributes: map[string]string{"k": "\xff"}},
		"unknown status":          {Title: "t", Attributes: map[string]string{"status": "maybe"}},
		"key given twice":         {Title: "t", Attributes: map[string]string{"k": "a", "K": "b"}},
		"128 attributes + status": {Title: "t", Attributes: attrs(128)},
	} {
		t.Run(name, func(t *testing.T) {
			s, _ := open(t)
			_, err := s.Create(ctx, req)
			wantInvalid(t, err)
		})
	}
	for _, k := range api.ReservedKeys {
		t.Run("reserved key "+k, func(t *testing.T) {
			s, _ := open(t)
			_, err := s.Create(ctx, api.CreateRequest{Title: "t", Attributes: map[string]string{strings.ToUpper(k): "v"}})
			wantInvalid(t, err)
		})
	}
}

func TestCreateAcceptsTheLimits(t *testing.T) {
	s, _ := open(t)
	tags := []string{strings.Repeat("z", 128), "a0-_"}
	for i := range 126 {
		tags = append(tags, "t"+strings.Repeat("x", i))
	}
	attrs := map[string]string{"status": "done", strings.Repeat("k", 128): "v"}
	for i := range 126 {
		attrs["k"+strings.Repeat("x", i)] = strings.Repeat("v", 2000)
	}
	idea := create(t, s, api.CreateRequest{
		Title:      strings.Repeat("é", 400),
		Body:       strings.Repeat("a", api.MaxBody),
		Tags:       tags,
		Attributes: attrs,
	})
	if len(idea.Tags) != 128 || len(idea.Attributes) != 128 || len(idea.Body) != api.MaxBody {
		t.Errorf("got %d tags, %d attributes, %d body bytes", len(idea.Tags), len(idea.Attributes), len(idea.Body))
	}
}

func TestBodyOverTheLimitIsTooLarge(t *testing.T) {
	s, _ := open(t)
	big := strings.Repeat("a", api.MaxBody+1)
	if _, err := s.Create(ctx, api.CreateRequest{Title: "t", Body: big}); !errors.Is(err, ErrTooLarge) {
		t.Errorf("Create: err = %v, want ErrTooLarge", err)
	}
	idea := create(t, s, api.CreateRequest{Title: "t"})
	if _, err := s.PutBody(ctx, idea.ID, api.Precondition{}, []byte(big)); !errors.Is(err, ErrTooLarge) {
		t.Errorf("PutBody: err = %v, want ErrTooLarge", err)
	}
	if _, err := s.Patch(ctx, idea.ID, api.Precondition{}, api.Patch{Body: &big}); !errors.Is(err, ErrTooLarge) {
		t.Errorf("Patch: err = %v, want ErrTooLarge", err)
	}
}
