package api

import "testing"

func TestNormalizeWebsiteDomain_Consistency(t *testing.T) {
	tests := []struct {
		input   string
		want    string
		wantErr bool
	}{
		{"example.com", "example.com", false},
		{"*.example.com", "*.example.com", false},
		{"localhost:3000", "localhost:3000", false},
		{"*.example.com:8080", "*.example.com:8080", false},
		{"https://blog.example.com", "blog.example.com", false},
	}
	for _, tt := range tests {
		got, err := normalizeWebsiteDomain(tt.input)
		if (err != nil) != tt.wantErr {
			t.Errorf("normalizeWebsiteDomain(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			continue
		}
		if got != tt.want {
			t.Errorf("normalizeWebsiteDomain(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
