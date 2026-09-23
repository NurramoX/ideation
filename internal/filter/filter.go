// Package filter parses the Filter language (spec §4, ADR 0002) into a syntax
// tree. Only the server gives a Filter meaning; the CLI and the Review TUI
// parse it only to find the tags and keys for the vocabulary hint.
package filter

import (
	"errors"
	"time"
)

// Expr is a node of the syntax tree. A nil Expr is the empty filter, which
// matches every idea.
type Expr interface{ expr() }

// And matches when every term matches. Juxtaposition is AND.
type And struct{ Terms []Expr }

// Or matches when any term matches (`or`, and the any-of list `key:a,b`).
type Or struct{ Terms []Expr }

// Not negates X (`-term`).
type Not struct{ X Expr }

// TextField is the FTS column a text term searches.
type TextField int

const (
	TextAny   TextField = iota // bare word or string: title and body
	TextTitle                  // title:...
	TextBody                   // body:...
)

// Text is one text term. Phrase is the term as the user wrote it, unquoted
// and unescaped; the store sends it to FTS5 as one quoted phrase.
type Text struct {
	Field  TextField
	Phrase string
}

// Attr matches an attribute, status included, whose value equals Value under
// Unicode simple case folding. Key is lowercased.
type Attr struct {
	Key   string
	Value string
}

// Tag matches an idea carrying the tag. Tag is lowercased.
type Tag struct{ Tag string }

// ID matches the idea with this id.
type ID struct{ ID int64 }

// Has tests presence. Key is lowercased and is one of "tag" (any tag),
// "reviewed" (reviewed at least once), "created" or "updated" (always true),
// or an attribute key. has:id, has:title, has:body and has:has are parse
// errors.
type Has struct{ Key string }

// DateField is the timestamp a date term tests.
type DateField int

const (
	Created DateField = iota
	Updated
	Reviewed
)

// Date matches when the timestamp lies in the half-open interval
// [From, To). A zero From or To is unbounded. Periods and relative instants
// are already resolved against the now and time zone given to Parse, and the
// inclusive `..` range ends are turned into the half-open form: `2026-09`
// is [Sep 1, Oct 1), `..90d` is [-inf, now-90d+1ms). A null timestamp
// (never reviewed) matches no Date term.
type Date struct {
	Field    DateField
	From, To time.Time
}

func (And) expr()  {}
func (Or) expr()   {}
func (Not) expr()  {}
func (Text) expr() {}
func (Attr) expr() {}
func (Tag) expr()  {}
func (ID) expr()   {}
func (Has) expr()  {}
func (Date) expr() {}

// Error is a parse error.
type Error struct {
	Position int // 1-based rune position of the offending character
	Msg      string
}

func (e *Error) Error() string { return e.Msg }

// Parse parses src. Periods are read in now's location, and relative
// instants count back from now. The empty (or all-whitespace) filter parses
// to nil. Every error is an *Error.
func Parse(src string, now time.Time) (Expr, error) {
	return nil, errors.New("filter.Parse: not implemented")
}

// Ranked returns the text terms that are top-level AND terms of e: the ones
// that are ranked and snippeted. It returns nil when there are none.
func Ranked(e Expr) []Text {
	return nil
}

// Vocabulary returns the tags and attribute keys that e mentions, each
// sorted and deduplicated, for the vocabulary hint. Keys include those of
// Has terms except "tag", "reviewed", "created" and "updated"; "status" is
// never returned.
func Vocabulary(e Expr) (tags, keys []string) {
	return nil, nil
}

// Caret renders src with a caret under the 1-based rune position pos, as two
// lines without a trailing newline, for showing a parse error.
func Caret(src string, pos int) string {
	return src
}
