package api

import "testing"

func TestParseID(t *testing.T) {
	for s, want := range map[string]int64{"1": 1, "42": 42, "9223372036854775807": 1<<63 - 1} {
		if got, ok := ParseID(s); !ok || got != want {
			t.Errorf("ParseID(%q) = %d, %v; want %d", s, got, ok, want)
		}
	}
	for _, s := range []string{"", "0", "01", "-1", "+1", "1.0", " 1", "1e3", "٣", "9223372036854775808"} {
		if got, ok := ParseID(s); ok {
			t.Errorf("ParseID(%q) = %d, want rejected", s, got)
		}
	}
}
