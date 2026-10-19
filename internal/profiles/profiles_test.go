package profiles

import (
	"testing"
	"time"
)

func TestActive(t *testing.T) {
	c := Config{
		Profiles: []Profile{{ID: "review"}, {ID: "tests"}},
		Segments: []Segment{
			{Profile: "review", Start: "08:30", End: "18:00", Days: []int{1, 2, 3, 4, 5}},
		},
		Fallback: "tests",
	}
	night := Config{
		Profiles: c.Profiles,
		// Sunday 22:00 wraps onto Monday 02:00.
		Segments: []Segment{{Profile: "review", Start: "22:00", End: "02:00", Days: []int{0}}},
	}

	// 2026-10-19 is a Monday.
	at := func(day int, hhmm string) time.Time {
		parsed, _ := time.Parse("15:04", hhmm)
		return time.Date(2026, 10, 19+day, parsed.Hour(), parsed.Minute(), 0, 0, time.Local)
	}
	cases := []struct {
		name string
		c    Config
		now  time.Time
		want string
	}{
		{"weekday morning", c, at(0, "08:30"), "review"},
		{"weekday before", c, at(0, "08:29"), "tests"},
		{"weekday end is exclusive", c, at(4, "18:00"), "tests"},
		{"saturday", c, at(5, "12:00"), "tests"},
		{"sunday night", night, at(6, "23:00"), "review"},
		{"wraps onto monday", night, at(0, "01:59"), "review"},
		{"after the wrap", night, at(0, "02:00"), ""},
	}
	for _, tc := range cases {
		p, ok := tc.c.Active(tc.now)
		got := ""
		if ok {
			got = p.ID
		}
		if got != tc.want {
			t.Errorf("%s: got %q, want %q", tc.name, got, tc.want)
		}
	}
}

func TestLimits(t *testing.T) {
	p := Profile{MaxTurns: 50, MaxMinutes: 30}
	if p.TurnLimit() != 50 || p.TimeLimit() != 30*time.Minute {
		t.Errorf("limits %d, %s", p.TurnLimit(), p.TimeLimit())
	}
	p.UnlimitedTurns, p.UnlimitedTime = true, true
	if p.TurnLimit() != 0 || p.TimeLimit() != 0 {
		t.Errorf("unlimited gave %d, %s", p.TurnLimit(), p.TimeLimit())
	}
}
