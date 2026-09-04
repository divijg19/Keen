package keen

import "testing"

// TestShortHash pins the canonical short-commit-hash helper shared by the
// textual and interactive renderers. Normal hashes are truncated to seven
// characters; unusually short or empty hashes pass through verbatim.
func TestShortHash(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"a1b2c3d4e5f6", "a1b2c3d"},
		{"abcdefg", "abcdefg"},
		{"abc", "abc"},
		{"", ""},
	}
	for _, c := range cases {
		if got := shortHash(c.in); got != c.want {
			t.Errorf("shortHash(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
