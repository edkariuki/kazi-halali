package httpapi

import "testing"

func TestAllowedOrigin(t *testing.T) {
	cases := []struct {
		config, origin string
		want           bool
	}{
		{"*", "https://anything.example", true},
		{"http://localhost:3000", "http://localhost:3000", true},
		{"http://localhost:3000", "http://localhost:3001", false},
		{"https://kazi.vercel.app, http://localhost:3000", "http://localhost:3000", true},
		{"https://*.vercel.app", "https://kazi-git-main-ed.vercel.app", true},
		{"https://*.vercel.app", "https://evilvercel.app", false},
		{"https://*.vercel.app", "http://kazi.vercel.app", false},
		{"https://kazi.vercel.app/", "https://kazi.vercel.app", true},
	}
	for _, c := range cases {
		if got := allowedOrigin(c.config, c.origin); got != c.want {
			t.Errorf("allowedOrigin(%q, %q) = %v, want %v", c.config, c.origin, got, c.want)
		}
	}
}
