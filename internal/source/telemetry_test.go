package source

import "testing"

func TestAtoi(t *testing.T) {
	cases := []struct {
		in   string
		want int
	}{
		{"85", 85},
		{"341.55", 341},
		{"[N/A]", 0},
		{" 7 ", 7},
		{"", 0},
	}
	for _, c := range cases {
		if got := atoi(c.in); got != c.want {
			t.Errorf("atoi(%q) = %d, want %d", c.in, got, c.want)
		}
	}
}

func TestShortHost(t *testing.T) {
	cases := []struct{ in, want string }{
		{"gpu-001", "gpu-001"},
		{"gpu-001.cluster.local", "gpu-001"},
		{"", ""},
	}
	for _, c := range cases {
		if got := shortHost(c.in); got != c.want {
			t.Errorf("shortHost(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
