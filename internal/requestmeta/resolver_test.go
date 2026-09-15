package requestmeta

import (
	"net/http/httptest"
	"testing"
)

func TestResolverTrustsForwardedHeadersOnlyFromConfiguredProxy(t *testing.T) {
	resolver := NewResolver([]string{"10.0.0.0/8"})
	request := httptest.NewRequest("GET", "http://example.com", nil)
	request.RemoteAddr = "10.1.2.3:1234"
	request.Header.Set("X-Forwarded-For", "203.0.113.9, 10.1.2.3")
	request.Header.Set("X-Forwarded-Proto", "https")
	if got := resolver.ClientIP(request); got != "203.0.113.9" {
		t.Fatalf("ClientIP() = %q", got)
	}
	if got := resolver.Scheme(request); got != "https" {
		t.Fatalf("Scheme() = %q", got)
	}

	request.RemoteAddr = "198.51.100.2:4321"
	if got := resolver.ClientIP(request); got != "198.51.100.2" {
		t.Fatalf("untrusted ClientIP() = %q", got)
	}
	if got := resolver.Scheme(request); got != "http" {
		t.Fatalf("untrusted Scheme() = %q", got)
	}
}

func TestResolverRejectsSpoofedLeftmostForwardedAddress(t *testing.T) {
	resolver := NewResolver([]string{"10.0.0.0/8"})
	request := httptest.NewRequest("GET", "http://example.com", nil)
	request.RemoteAddr = "10.1.2.3:1234"
	request.Header.Set("X-Forwarded-For", "192.0.2.99, 203.0.113.8, 10.2.3.4")
	if got := resolver.ClientIP(request); got != "203.0.113.8" {
		t.Fatalf("ClientIP() = %q, want nearest untrusted hop", got)
	}
}
