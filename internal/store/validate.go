package store

import (
	"fmt"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/NurramoX/ideation/internal/api"
)

// Limits of the data model (spec §2).
const (
	maxTitleRunes = 400
	maxLabelBytes = 128 // a tag or an attribute key
	maxTags       = 128
	maxAttributes = 128
	maxValueBytes = 2000
)

func invalid(format string, args ...any) error {
	return &InvalidError{Msg: fmt.Sprintf(format, args...)}
}

// title trims t and checks it: 1–400 characters on a single line, with no
// control characters.
func title(t string) (string, error) {
	t = strings.TrimSpace(t)
	if !utf8.ValidString(t) {
		return "", invalid("title is not valid UTF-8")
	}
	if t == "" {
		return "", invalid("title is empty")
	}
	if n := utf8.RuneCountInString(t); n > maxTitleRunes {
		return "", invalid("title is %d characters, at most %d allowed", n, maxTitleRunes)
	}
	if !singleLine(t) {
		return "", invalid("title must be a single line without control characters")
	}
	return t, nil
}

// singleLine reports whether s has no control characters and no line or
// paragraph separator.
func singleLine(s string) bool {
	return !strings.ContainsFunc(s, func(r rune) bool {
		return unicode.IsControl(r) || r == '\u2028' || r == '\u2029'
	})
}

// body checks a body: at most api.MaxBody bytes (ErrTooLarge) of valid UTF-8.
func body(b string) error {
	if len(b) > api.MaxBody {
		return ErrTooLarge
	}
	if !utf8.ValidString(b) {
		return invalid("body is not valid UTF-8")
	}
	return nil
}

// label lowercases a tag or attribute key and checks it against
// [a-z0-9][a-z0-9_-]* and the length limit. Only ASCII is lowercased, since
// nothing else can pass.
func label(what, s string) (string, error) {
	s = asciiLower(s)
	if s == "" {
		return "", invalid("%s is empty", what)
	}
	if len(s) > maxLabelBytes {
		return "", invalid("%s %q is %d bytes, at most %d allowed", what, s, len(s), maxLabelBytes)
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		ok := c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || i > 0 && (c == '_' || c == '-')
		if !ok {
			return "", invalid("%s %q must match [a-z0-9][a-z0-9_-]*", what, s)
		}
	}
	return s, nil
}

func asciiLower(s string) string {
	return strings.Map(func(r rune) rune {
		if r >= 'A' && r <= 'Z' {
			return r + 'a' - 'A'
		}
		return r
	}, s)
}

func tag(s string) (string, error) { return label("tag", s) }

// tagSet validates tags and returns them sorted and deduplicated.
func tagSet(tags []string) ([]string, error) {
	out := make([]string, 0, len(tags))
	for _, t := range tags {
		t, err := tag(t)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	slices.Sort(out)
	out = slices.Compact(out)
	if len(out) > maxTags {
		return nil, invalid("%d tags, at most %d allowed", len(out), maxTags)
	}
	return out, nil
}

// key validates an attribute key: a label that is not reserved.
func key(s string) (string, error) {
	k, err := label("attribute key", s)
	if err != nil {
		return "", err
	}
	if slices.Contains(api.ReservedKeys, k) {
		return "", invalid("%q is a reserved key", k)
	}
	return k, nil
}

// value trims and checks the value of attribute k: single-line UTF-8 of
// 1–2000 bytes. Status is lowercased and must be one of api.Statuses.
func value(k, v string) (string, error) {
	v = strings.TrimSpace(v)
	if !utf8.ValidString(v) {
		return "", invalid("value of %q is not valid UTF-8", k)
	}
	if v == "" {
		return "", invalid("value of %q is empty; remove the key instead", k)
	}
	if len(v) > maxValueBytes {
		return "", invalid("value of %q is %d bytes, at most %d allowed", k, len(v), maxValueBytes)
	}
	if !singleLine(v) {
		return "", invalid("value of %q must be a single line without control characters", k)
	}
	if k == api.StatusKey {
		v = strings.ToLower(v)
		if !slices.Contains(api.Statuses, v) {
			return "", invalid("status %q is not one of %s", v, strings.Join(api.Statuses, ", "))
		}
	}
	return v, nil
}

// attribute validates one key and its value.
func attribute(k, v string) (string, string, error) {
	k, err := key(k)
	if err != nil {
		return "", "", err
	}
	v, err = value(k, v)
	return k, v, err
}
