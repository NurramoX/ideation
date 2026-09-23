package store

import (
	"database/sql"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/NurramoX/ideation/internal/api"
)

func TestPragmas(t *testing.T) {
	s, _ := open(t)
	db := s.(*sqliteStore).db
	for pragma, want := range map[string]string{
		"journal_mode": "wal",
		"foreign_keys": "1",
		"busy_timeout": "5000",
		"synchronous":  "2", // FULL
	} {
		var got string
		if err := db.QueryRow("PRAGMA " + pragma).Scan(&got); err != nil {
			t.Fatal(err)
		}
		if got != want {
			t.Errorf("%s = %s, want %s", pragma, got, want)
		}
	}
	if n := db.Stats().MaxOpenConnections; n != 1 {
		t.Errorf("MaxOpenConnections = %d, want 1", n)
	}
}

func TestReopenKeepsIdeas(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ideation.db")
	c := newClock()
	s, err := Open(path, c.Now)
	if err != nil {
		t.Fatal(err)
	}
	idea := create(t, s, api.CreateRequest{Title: "kept", Body: "searchable words"})
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	// Close checkpointed and truncated the WAL.
	if fi, err := os.Stat(path + "-wal"); err == nil && fi.Size() != 0 {
		t.Errorf("WAL is %d bytes after Close", fi.Size())
	}

	s, err = Open(path, c.Now)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if got := get(t, s, idea.ID); got.Title != "kept" {
		t.Errorf("reopened: %+v", got)
	}
}

func TestOpenRefusesANewerDatabase(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ideation.db")
	s, err := Open(path, newClock().Now)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	scripts, err := migrations()
	if err != nil {
		t.Fatal(err)
	}
	setUserVersion(t, path, len(scripts)+1)

	if s, err := Open(path, newClock().Now); err == nil {
		s.Close()
		t.Fatal("Open succeeded on a database newer than the binary")
	} else if !strings.Contains(err.Error(), "newer") {
		t.Errorf("err = %v", err)
	}
}

func TestOpenRefusesAPathWithAQuestionMark(t *testing.T) {
	if _, err := Open(filepath.Join(t.TempDir(), "what?.db"), newClock().Now); err == nil {
		t.Error("Open accepted a path with '?'")
	}
}

func setUserVersion(t *testing.T, path string, v int) {
	t.Helper()
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec("PRAGMA user_version = " + strconv.Itoa(v)); err != nil {
		t.Fatal(err)
	}
}

func userVersion(t *testing.T, db *sql.DB) int {
	t.Helper()
	var v int
	if err := db.QueryRow("PRAGMA user_version").Scan(&v); err != nil {
		t.Fatal(err)
	}
	return v
}

// A failing migration leaves the database at the last good version, with
// none of its own statements applied.
func TestFailedMigrationRollsBack(t *testing.T) {
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "m.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	scripts, err := migrations()
	if err != nil {
		t.Fatal(err)
	}
	broken := append(scripts, "CREATE TABLE half (x); THIS IS NOT SQL;")
	if err := migrate(ctx, db, broken); err == nil {
		t.Fatal("broken migration succeeded")
	}
	if v := userVersion(t, db); v != len(scripts) {
		t.Errorf("user_version = %d, want %d", v, len(scripts))
	}
	var n int
	if err := db.QueryRow(`SELECT count(*) FROM sqlite_master WHERE name = 'half'`).Scan(&n); err != nil || n != 0 {
		t.Errorf("table of the failed migration exists (n=%d, err=%v)", n, err)
	}
	// Running the good migrations again is a no-op.
	if err := migrate(ctx, db, scripts); err != nil {
		t.Errorf("re-migrate: %v", err)
	}
}

// The full-text index stays consistent with the ideas through every kind of
// write.
func TestFullTextIndexStaysConsistent(t *testing.T) {
	s, _, idea := seed(t)
	other := create(t, s, api.CreateRequest{Title: "Other", Body: "more words"})
	if _, err := s.Patch(ctx, idea.ID, none, api.Patch{Title: str("Renamed"), Body: str("fresh text")}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.PutBody(ctx, other.ID, none, []byte("replaced body")); err != nil {
		t.Fatal(err)
	}
	if err := s.MarkReviewed(ctx, idea.ID); err != nil {
		t.Fatal(err)
	}
	if err := s.Delete(ctx, other.ID, none); err != nil {
		t.Fatal(err)
	}
	db := s.(*sqliteStore).db
	if _, err := db.Exec(`INSERT INTO idea_fts(idea_fts, rank) VALUES ('integrity-check', 1)`); err != nil {
		t.Errorf("integrity-check: %v", err)
	}
	for q, want := range map[string]int{`"renamed"`: 1, `"fresh"`: 1, `"borrow"`: 0, `"body"`: 0, `"replaced"`: 0} {
		var n int
		if err := db.QueryRow(`SELECT count(*) FROM idea_fts WHERE idea_fts MATCH ?`, q).Scan(&n); err != nil {
			t.Fatal(err)
		}
		if n != want {
			t.Errorf("MATCH %s: %d rows, want %d", q, n, want)
		}
	}
}
