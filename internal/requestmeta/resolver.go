package requestmeta

import (
	"net"
	"net/http"
	"strings"
)

type Resolver struct {
	trusted []*net.IPNet
}

func NewResolver(entries []string) *Resolver {
	resolver := &Resolver{}
	for _, entry := range entries {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		if ip := net.ParseIP(entry); ip != nil {
			bits := 128
			if ip.To4() != nil {
				bits = 32
			}
			resolver.trusted = append(resolver.trusted, &net.IPNet{IP: ip, Mask: net.CIDRMask(bits, bits)})
			continue
		}
		if _, network, err := net.ParseCIDR(entry); err == nil {
			resolver.trusted = append(resolver.trusted, network)
		}
	}
	return resolver
}

func (r *Resolver) Trusted(remoteAddr string) bool {
	ip := remoteIP(remoteAddr)
	if ip == nil {
		return false
	}
	for _, network := range r.trusted {
		if network.Contains(ip) {
			return true
		}
	}
	return false
}

func (r *Resolver) ClientIP(request *http.Request) string {
	if r.Trusted(request.RemoteAddr) {
		forwarded := strings.Split(request.Header.Get("X-Forwarded-For"), ",")
		for index := len(forwarded) - 1; index >= 0; index-- {
			candidate := net.ParseIP(strings.TrimSpace(forwarded[index]))
			if candidate != nil && !r.trustedIP(candidate) {
				return candidate.String()
			}
		}
		if realIP := strings.TrimSpace(request.Header.Get("X-Real-IP")); net.ParseIP(realIP) != nil {
			return realIP
		}
	}
	ip := remoteIP(request.RemoteAddr)
	if ip == nil {
		return ""
	}
	return ip.String()
}

func (r *Resolver) trustedIP(ip net.IP) bool {
	for _, network := range r.trusted {
		if network.Contains(ip) {
			return true
		}
	}
	return false
}

func (r *Resolver) Scheme(request *http.Request) string {
	if request.TLS != nil {
		return "https"
	}
	if r.Trusted(request.RemoteAddr) {
		if forwarded := strings.ToLower(firstHeaderValue(request.Header.Get("X-Forwarded-Proto"))); forwarded == "http" || forwarded == "https" {
			return forwarded
		}
	}
	return "http"
}

func remoteIP(remoteAddr string) net.IP {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		host = remoteAddr
	}
	return net.ParseIP(strings.TrimSpace(host))
}

func firstHeaderValue(value string) string {
	return strings.TrimSpace(strings.Split(value, ",")[0])
}
