package tui

import (
	"slices"
	"strings"
	"testing"
	"time"
)

// threeIdeas is a fake with ideas 1, 2 and 3, newest first.
func threeIdeas() *fakeClient {
	fc := newFake(testNow)
	fc.add("First idea", time.Hour, "rust")
	fc.add("Second idea", 2*time.Hour)
	fc.add("Third idea", 3*time.Hour, "go", "cli")
	return fc
}

func TestMovingTheSelectionNeverMarksReviewed(t *testing.T) {
	h := newHarness(t, threeIdeas(), "")
	for _, step := range []struct {
		key  string
		want int64
	}{{"j", 2}, {"down", 3}, {"j", 3}, {"k", 2}, {"up", 1}, {"k", 1}, {"G", 3}, {"g", 1}} {
		h.press(step.key)
		if got := h.selected(); got != step.want {
			t.Fatalf("after %q selected %d, want %d", step.key, got, step.want)
		}
	}
	if w := h.fc.writes(); len(w) != 0 {
		t.Errorf("moving wrote %v", w)
	}
}

func TestSkipMarksReviewedAndAdvances(t *testing.T) {
	for _, key := range []string{"space", "enter", "n"} {
		t.Run(key, func(t *testing.T) {
			h := newHarness(t, threeIdeas(), "")
			h.press(key)
			if !slices.Equal(h.fc.writes(), []string{"PUT 1 reviewed"}) {
				t.Errorf("writes = %v", h.fc.writes())
			}
			if h.selected() != 2 {
				t.Errorf("selected %d, want 2", h.selected())
			}
			if !strings.Contains(h.view(), " ✓") {
				t.Errorf("no ✓ on the reviewed row:\n%s", h.view())
			}
		})
	}
}

func TestLaterAdvancesWithoutMarkingReviewed(t *testing.T) {
	h := newHarness(t, threeIdeas(), "")
	h.press("z")
	if h.selected() != 2 || len(h.fc.writes()) != 0 {
		t.Errorf("selected %d, writes %v", h.selected(), h.fc.writes())
	}
	if strings.Contains(h.view(), "✓") {
		t.Errorf("later marked a row reviewed:\n%s", h.view())
	}
}

func TestAdvanceStopsAtTheLastRow(t *testing.T) {
	h := newHarness(t, threeIdeas(), "")
	h.press("G", "n")
	if h.selected() != 3 {
		t.Errorf("selected %d, want 3", h.selected())
	}
}

func TestStatusKeysSetStatusThenMarkReviewedAndAdvance(t *testing.T) {
	for key, status := range map[string]string{"a": "active", "d": "done", "x": "dropped", "r": "raw"} {
		t.Run(key, func(t *testing.T) {
			h := newHarness(t, threeIdeas(), "")
			h.press(key)
			want := []string{"PUT 1 attribute status=" + status, "PUT 1 reviewed"}
			if !slices.Equal(h.fc.writes(), want) {
				t.Errorf("writes = %v, want %v", h.fc.writes(), want)
			}
			if h.selected() != 2 {
				t.Errorf("selected %d, want 2", h.selected())
			}
			if got := h.m.rows[0].Attributes["status"]; got != status {
				t.Errorf("row status = %q, want %q", got, status)
			}
		})
	}
}

func TestAStatusChangeOnTheLastRowRefreshesItsPreview(t *testing.T) {
	h := newHarness(t, threeIdeas(), "")
	h.press("G", "d")
	if h.selected() != 3 {
		t.Fatalf("selected %d, want to stay on 3", h.selected())
	}
	if v := h.view(); !strings.Contains(v, "#3 · done") || !strings.Contains(v, "v2") {
		t.Errorf("preview not refreshed:\n%s", v)
	}
}

func TestAFailedStatusChangeShowsTheErrorAndDoesNotMarkReviewed(t *testing.T) {
	fc := threeIdeas()
	fc.failNext["PUT 1 attribute"] = problem(422, "bad status")
	h := newHarness(t, fc, "")
	h.press("d")
	if !slices.Equal(fc.writes(), []string{"PUT 1 attribute status=done"}) {
		t.Errorf("writes = %v", fc.writes())
	}
	if !strings.Contains(h.view(), "bad status") {
		t.Errorf("status line lacks the error:\n%s", h.view())
	}
	if h.m.rows[0].Attributes["status"] != "raw" {
		t.Errorf("row changed despite the failure")
	}
}

func TestQuit(t *testing.T) {
	for _, key := range []string{"q", "ctrl+c"} {
		h := newHarness(t, threeIdeas(), "")
		h.press(key)
		if !h.quit {
			t.Errorf("%s did not quit", key)
		}
	}
}

func TestHelpShowsTheKeysAndAnyKeyClosesIt(t *testing.T) {
	h := newHarness(t, threeIdeas(), "")
	h.press("?")
	if v := h.view(); !strings.Contains(v, "mark reviewed") || !strings.Contains(v, "cycle order") {
		t.Fatalf("help lacks the keys:\n%s", v)
	}
	h.press("j")
	if strings.Contains(h.view(), "cycle order") || h.selected() != 1 {
		t.Errorf("the key closing help also acted, or help stayed")
	}
}
