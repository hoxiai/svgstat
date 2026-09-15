package renderer

import (
	"strings"
	"testing"
)

func TestNormalizeColor(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{name: "named", raw: "brightgreen", want: "#4c1"},
		{name: "hex", raw: "7c3aed", want: "#7c3aed"},
		{name: "trimmed hex", raw: "  fff  ", want: "#fff"},
		{name: "reject CSS URL", raw: "url(javascript:alert(1))", want: ""},
		{name: "reject arbitrary CSS", raw: "red;stroke:black", want: ""},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := normalizeColor(test.raw); got != test.want {
				t.Fatalf("normalizeColor(%q) = %q, want %q", test.raw, got, test.want)
			}
		})
	}
}

func TestPrepareHomepageAllowsHTTPOnly(t *testing.T) {
	tests := []struct {
		raw  string
		want string
	}{
		{raw: "example.com/path?q=1", want: "https://example.com/"},
		{raw: "http://example.com/path", want: "http://example.com/"},
		{raw: "javascript://example.com/alert", want: ""},
		{raw: "ftp://example.com/file", want: ""},
	}

	for _, test := range tests {
		if got := prepareHomepage(test.raw); got != test.want {
			t.Errorf("prepareHomepage(%q) = %q, want %q", test.raw, got, test.want)
		}
	}
}

func TestRenderBadgeEscapesLabel(t *testing.T) {
	svg, err := New().RenderBadge(BadgeData{Label: `<script>alert(1)</script>`, Value: "1", Color: "red"})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(svg, "<script>") {
		t.Fatalf("rendered SVG contains an unescaped script element: %s", svg)
	}
	if !strings.Contains(svg, "&lt;script&gt;") {
		t.Fatalf("rendered SVG does not contain the escaped label: %s", svg)
	}
}
