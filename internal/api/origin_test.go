package api

import "testing"

func TestWebsiteOriginAllowed_EmptyOrigin(t *testing.T) {
	// When no domains are configured, empty origin (e.g. same-origin, curl, sendBeacon) must be allowed
	if !websiteOriginAllowed("", nil) {
		t.Errorf("websiteOriginAllowed(\"\", nil) = false, want true")
	}
	if !websiteOriginAllowed("", []string{}) {
		t.Errorf("websiteOriginAllowed(\"\", []) = false, want true")
	}

	// When domains are configured, empty origin must be rejected
	if websiteOriginAllowed("", []string{"example.com"}) {
		t.Errorf("websiteOriginAllowed(\"\", [\"example.com\"]) = true, want false")
	}

	// Valid origin matching domains
	if !websiteOriginAllowed("https://example.com", []string{"example.com"}) {
		t.Errorf("websiteOriginAllowed(\"https://example.com\", [\"example.com\"]) = false, want true")
	}

	// Valid origin not matching domains
	if websiteOriginAllowed("https://evil.com", []string{"example.com"}) {
		t.Errorf("websiteOriginAllowed(\"https://evil.com\", [\"example.com\"]) = true, want false")
	}
}
