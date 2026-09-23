package filter

import (
	"reflect"
	"testing"
	"time"
)

func TestParseDates(t *testing.T) {
	at := func(y int, m time.Month, d, h, min, s, ms int) time.Time {
		return time.Date(y, m, d, h, min, s, ms*int(time.Millisecond), berlin)
	}
	day := func(y int, m time.Month, d int) time.Time { return at(y, m, d, 0, 0, 0, 0) }
	utc := func(y int, m time.Month, d, h, min, s, ms int) time.Time {
		return time.Date(y, m, d, h, min, s, ms*int(time.Millisecond), time.UTC)
	}
	var inf time.Time

	tests := []struct {
		src      string
		field    DateField
		from, to time.Time
	}{
		// Periods, read in now's location.
		{"created:2026", Created, day(2026, 1, 1), day(2027, 1, 1)},
		{"created:2026-09", Created, day(2026, 9, 1), day(2026, 10, 1)},
		{"created:2026-10", Created, day(2026, 10, 1), day(2026, 11, 1)},
		{"created:2026-12", Created, day(2026, 12, 1), day(2027, 1, 1)},
		{"created:2026-09-01", Created, day(2026, 9, 1), day(2026, 9, 2)},
		{"created:2028-02-29", Created, day(2028, 2, 29), day(2028, 3, 1)},
		{"updated:2026-10-25", Updated, day(2026, 10, 25), day(2026, 10, 26)}, // 25-hour day
		{"updated:2026-03-29", Updated, day(2026, 3, 29), day(2026, 3, 30)},   // 23-hour day
		{"reviewed:today", Reviewed, day(2026, 9, 23), day(2026, 9, 24)},
		{"Reviewed:Today", Reviewed, day(2026, 9, 23), day(2026, 9, 24)},
		{`created:"2026-09-01T10:00:00Z"`, Created, utc(2026, 9, 1, 10, 0, 0, 0), utc(2026, 9, 1, 10, 0, 0, 1)},
		{`created:"2026-09-01T10:00:00.4129+02:00"`, Created, at(2026, 9, 1, 10, 0, 0, 412), at(2026, 9, 1, 10, 0, 0, 413)},

		// Ranges, inclusive at both ends.
		{"created:2026-09..2026-10", Created, day(2026, 9, 1), day(2026, 11, 1)},
		{"created:2026-09-01..2026-09-01", Created, day(2026, 9, 1), day(2026, 9, 2)},
		{"created:2026..", Created, day(2026, 1, 1), inf},
		{"created:..2026-09-01", Created, inf, day(2026, 9, 2)},
		{"created:today..", Created, day(2026, 9, 23), inf},
		{`updated:"2026-09-01T10:00:00Z..2026-09-02T00:00:00Z"`, Updated, utc(2026, 9, 1, 10, 0, 0, 0), utc(2026, 9, 2, 0, 0, 0, 1)},

		// Relative instants count back from now, as range ends.
		{"reviewed:90d..", Reviewed, at(2026, 6, 25, 15, 4, 5, 0), inf},
		{"reviewed:..90d", Reviewed, inf, at(2026, 6, 25, 15, 4, 5, 1)},
		{"reviewed:2w..", Reviewed, at(2026, 9, 9, 15, 4, 5, 0), inf},
		{"reviewed:6m..", Reviewed, at(2026, 3, 23, 15, 4, 5, 0), inf}, // across DST: wall clock kept
		{"reviewed:1y..", Reviewed, at(2025, 9, 23, 15, 4, 5, 0), inf},
		{"reviewed:0d..", Reviewed, now, inf},
		{"reviewed:90D..", Reviewed, at(2026, 6, 25, 15, 4, 5, 0), inf},
		{"created:90d..30d", Created, at(2026, 6, 25, 15, 4, 5, 0), at(2026, 8, 24, 15, 4, 5, 1)},
		{"created:2026..7d", Created, day(2026, 1, 1), at(2026, 9, 16, 15, 4, 5, 1)},
	}
	for _, tt := range tests {
		t.Run(tt.src, func(t *testing.T) {
			e, err := Parse(tt.src, now)
			if err != nil {
				t.Fatalf("Parse(%q): %v", tt.src, err)
			}
			d, ok := e.(Date)
			if !ok {
				t.Fatalf("Parse(%q) = %#v, want a Date", tt.src, e)
			}
			if d.Field != tt.field || !d.From.Equal(tt.from) || !d.To.Equal(tt.to) {
				t.Errorf("Parse(%q) = %v [%v, %v), want %v [%v, %v)",
					tt.src, d.Field, d.From, d.To, tt.field, tt.from, tt.to)
			}
		})
	}
}

func TestParseReviewQueue(t *testing.T) {
	e, err := Parse("status:raw,active -reviewed:90d..", now)
	if err != nil {
		t.Fatal(err)
	}
	want := And{[]Expr{
		Or{[]Expr{Attr{"status", "raw"}, Attr{"status", "active"}}},
		Not{Date{Reviewed, time.Date(2026, 6, 25, 15, 4, 5, 0, berlin), time.Time{}}},
	}}
	if !reflect.DeepEqual(e, want) {
		t.Errorf("got %#v\nwant %#v", e, want)
	}
}

func TestParseAnyOfDates(t *testing.T) {
	e, err := Parse("created:2025,2026-09", now)
	if err != nil {
		t.Fatal(err)
	}
	want := Or{[]Expr{
		Date{Created, time.Date(2025, 1, 1, 0, 0, 0, 0, berlin), time.Date(2026, 1, 1, 0, 0, 0, 0, berlin)},
		Date{Created, time.Date(2026, 9, 1, 0, 0, 0, 0, berlin), time.Date(2026, 10, 1, 0, 0, 0, 0, berlin)},
	}}
	if !reflect.DeepEqual(e, want) {
		t.Errorf("got %#v\nwant %#v", e, want)
	}
}
