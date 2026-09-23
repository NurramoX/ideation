package tui

import (
	"slices"
	"strings"
	"testing"
)

func TestRetagAppliesPerTagCallsAndStays(t *testing.T) {
	fc := threeIdeas()
	h := newHarness(t, fc, "")
	h.press("G", "t") // idea 3: go cli
	if got := h.m.tags.Value(); got != "cli go" {
		t.Fatalf("tag editor = %q, want the current tags", got)
	}
	if !strings.Contains(h.view(), "tags: cli go") {
		t.Errorf("view lacks the tag editor:\n%s", h.view())
	}
	h.press("ctrl+u")
	h.typeText("go Rust tools rust")
	h.press("enter")
	want := []string{"PUT 3 tag rust", "PUT 3 tag tools", "DELETE 3 tag cli"}
	if !slices.Equal(fc.writes(), want) {
		t.Errorf("writes = %v, want %v", fc.writes(), want)
	}
	if h.selected() != 3 {
		t.Errorf("selected %d, want to stay on 3", h.selected())
	}
	if got := h.m.rows[2].Tags; !slices.Equal(got, []string{"go", "rust", "tools"}) {
		t.Errorf("row tags = %v", got)
	}
	if v := h.view(); !strings.Contains(v, "go rust tools") || !strings.Contains(v, "v4") {
		t.Errorf("view not updated:\n%s", v)
	}
}

func TestRetagEscCancels(t *testing.T) {
	fc := threeIdeas()
	h := newHarness(t, fc, "")
	h.press("t", "ctrl+u", "esc")
	if len(fc.writes()) != 0 {
		t.Errorf("writes = %v", fc.writes())
	}
	h.press("j")
	if h.selected() != 2 {
		t.Errorf("keys do not reach the list after esc")
	}
}

func TestRetagStopsAtTheFirstFailure(t *testing.T) {
	fc := threeIdeas()
	fc.failNext["PUT 1 tag b"] = problem(422, "bad tag")
	h := newHarness(t, fc, "")
	h.press("t", "ctrl+u")
	h.typeText("a b c")
	h.press("enter")
	if want := []string{"PUT 1 tag a", "PUT 1 tag b"}; !slices.Equal(fc.writes(), want) {
		t.Errorf("writes = %v, want %v", fc.writes(), want)
	}
	if got := h.m.rows[0].Tags; !slices.Equal(got, []string{"a", "rust"}) {
		t.Errorf("row tags = %v, want what was applied", got)
	}
	if !strings.Contains(h.view(), "bad tag") {
		t.Errorf("error not shown")
	}
}

func TestDeleteAfterYesIsGuardedByThePreviewVersionAndAdvances(t *testing.T) {
	fc := threeIdeas()
	fc.ideas[1].Version = 7
	h := newHarness(t, fc, "")
	h.press("D")
	if !strings.Contains(h.view(), `delete idea 1 "First idea"? [y/N]`) {
		t.Fatalf("no prompt:\n%s", h.view())
	}
	h.press("y")
	if want := []string{"DELETE 1 if-match \"7\""}; !slices.Equal(fc.writes(), want) {
		t.Errorf("writes = %v, want %v", fc.writes(), want)
	}
	if h.selected() != 2 {
		t.Errorf("selected %d, want 2", h.selected())
	}
	if !slices.Equal(h.rowIDs(), []int64{1, 2, 3}) || !h.m.rows[0].deleted {
		t.Errorf("the deleted row must stay, marked, until a requery")
	}
}

func TestDeleteAnythingButYesKeeps(t *testing.T) {
	for _, key := range []string{"n", "N", "esc", "enter"} {
		fc := threeIdeas()
		h := newHarness(t, fc, "")
		h.press("D", key)
		if len(fc.writes()) != 0 || h.selected() != 1 {
			t.Errorf("%s: writes %v, selected %d", key, fc.writes(), h.selected())
		}
	}
}

func TestDeleteOfAChangedIdeaRefreshesThePreview(t *testing.T) {
	fc := threeIdeas()
	h := newHarness(t, fc, "")
	fc.bump(1, "Their newer body.\n")
	h.press("D", "y")
	v := h.view()
	if !strings.Contains(v, "changed, not deleted") {
		t.Errorf("view lacks the conflict:\n%s", v)
	}
	if !strings.Contains(v, "Their newer body.") {
		t.Errorf("preview not refreshed:\n%s", v)
	}
	if h.selected() != 1 || fc.idea(1) == nil {
		t.Errorf("the idea was deleted or the selection moved")
	}
	// The refreshed Version now guards the next delete.
	h.press("D", "y")
	if fc.idea(1) != nil {
		t.Errorf("second delete failed: %v", fc.writes())
	}
}

func TestEditAndDeleteWaitForThePreview(t *testing.T) {
	for _, key := range []string{"D", "e"} {
		fc := threeIdeas()
		h := newHarness(t, fc, "")
		h.step(keyMsg("j")) // the preview of idea 2 has not loaded
		h.press(key)
		if !strings.Contains(h.view(), "the preview is still loading") || h.m.mode != modeList {
			t.Errorf("%s acted without the preview's Version:\n%s", key, h.view())
		}
	}
}

func TestAttributesAreReadOnly(t *testing.T) {
	fc := threeIdeas()
	h := newHarness(t, fc, "")
	// No key reaches an attribute other than status.
	for _, k := range []string{"s", "S", "u", "U", "A", "backspace"} {
		h.press(k)
	}
	for _, w := range fc.writes() {
		if strings.Contains(w, "attribute") {
			t.Errorf("attribute write %q", w)
		}
	}
}
