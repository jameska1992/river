package musicbrainz

import "testing"

func TestParseYear(t *testing.T) {
	cases := map[string]int{
		"1959":       1959, // YYYY
		"1967-06":    1967, // YYYY-MM
		"1969-09-26": 1969, // YYYY-MM-DD
		"":           0,    // empty
		"19":         0,    // too short
		"abcd-01":    0,    // non-numeric year
		"20x1":       0,    // non-digit within the year
	}
	for in, want := range cases {
		if got := parseYear(in); got != want {
			t.Errorf("parseYear(%q) = %d, want %d", in, got, want)
		}
	}
}
