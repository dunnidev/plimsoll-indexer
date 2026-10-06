package config

import "testing"

func TestKeepAwakeURL(t *testing.T) {
	cases := []struct {
		explicit, render, want string
	}{
		{"", "", ""},
		{"", "https://plimsoll-indexer.onrender.com", "https://plimsoll-indexer.onrender.com/healthz"},
		{"", "https://x.onrender.com/", "https://x.onrender.com/healthz"},
		{"https://example.org/ping", "https://x.onrender.com", "https://example.org/ping"},
		{"none", "https://x.onrender.com", ""},
	}
	for _, c := range cases {
		if got := keepAwakeURL(c.explicit, c.render); got != c.want {
			t.Errorf("keepAwakeURL(%q, %q) = %q, want %q", c.explicit, c.render, got, c.want)
		}
	}
}
