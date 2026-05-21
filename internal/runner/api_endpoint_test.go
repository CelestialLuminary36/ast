package runner

import "testing"

func TestNormalizeEndpoint(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"https://api.anthropic.com/v1/messages", "https://api.anthropic.com/"},
		{"https://api.anthropic.com/v1/messages/", "https://api.anthropic.com/"},
		{"https://proxy.example.com/v1/messages", "https://proxy.example.com/"},
		{"https://proxy.example.com/anthropic/v1/messages", "https://proxy.example.com/anthropic/"},
		{"https://proxy.example.com/", "https://proxy.example.com/"},
		{"https://proxy.example.com", "https://proxy.example.com/"},
	}

	for _, c := range cases {
		got := normalizeEndpointForBaseURL(c.in)
		if got != c.want {
			t.Errorf("normalizeEndpointForBaseURL(%q) = %q; want %q", c.in, got, c.want)
		}
	}
}
