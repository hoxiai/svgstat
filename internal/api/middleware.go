package api

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/mux"
	"github.com/rs/zerolog/log"
)

type rateLimitEntry struct {
	count       int
	windowStart time.Time
}

type rateLimiter struct {
	mu          sync.Mutex
	entries     map[string]rateLimitEntry
	limit       int
	window      time.Duration
	lastCleanup time.Time
	backend     distributedRateLimiter
	prefix      string
}

type distributedRateLimiter interface {
	AllowRate(context.Context, string, int, time.Duration) (bool, time.Duration, error)
}

func newRateLimiter(limit int, window time.Duration, prefix string, backend distributedRateLimiter) *rateLimiter {
	return &rateLimiter{entries: make(map[string]rateLimitEntry), limit: limit, window: window, prefix: prefix, backend: backend}
}

func (l *rateLimiter) allow(ctx context.Context, key string, now time.Time) bool {
	if l.backend != nil {
		allowed, _, err := l.backend.AllowRate(ctx, rateLimitKey(l.prefix, key), l.limit, l.window)
		if err == nil {
			return allowed
		}
		log.Warn().Err(err).Str("limiter", l.prefix).Msg("Distributed rate limiter unavailable; using local fallback")
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.lastCleanup.IsZero() || now.Sub(l.lastCleanup) >= l.window {
		for entryKey, entry := range l.entries {
			if now.Sub(entry.windowStart) >= l.window {
				delete(l.entries, entryKey)
			}
		}
		l.lastCleanup = now
	}

	entry := l.entries[key]
	if entry.windowStart.IsZero() || now.Sub(entry.windowStart) >= l.window {
		l.entries[key] = rateLimitEntry{count: 1, windowStart: now}
		return true
	}
	if entry.count >= l.limit {
		return false
	}
	entry.count++
	l.entries[key] = entry
	return true
}

func (a *App) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		requestID := strings.TrimSpace(r.Header.Get("X-Request-ID"))
		if requestID == "" || len(requestID) > 128 {
			requestID = newRequestID()
		}
		w.Header().Set("X-Request-ID", requestID)
		r = r.WithContext(context.WithValue(r.Context(), requestIDContextKey{}, requestID))
		response := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		log.Info().
			Str("request_id", requestID).
			Str("method", r.Method).
			Str("path", r.URL.Path).
			Msg("Request started")

		next.ServeHTTP(response, r)
		route := "unmatched"
		if current := mux.CurrentRoute(r); current != nil {
			if template, err := current.GetPathTemplate(); err == nil {
				route = template
			}
		}
		a.observability.ObserveHTTP(route, response.status, time.Since(start))

		log.Info().
			Str("request_id", requestID).
			Str("method", r.Method).
			Str("path", r.URL.Path).
			Int("status", response.status).
			Dur("duration", time.Since(start)).
			Msg("Request completed")
	})
}

func (a *App) securityHeadersMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		next.ServeHTTP(w, r)
	})
}

func (a *App) csrfMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if a.config != nil && !a.config.HTTP.CSRFCheckEnabled {
			next.ServeHTTP(w, r)
			return
		}
		if r.Method == http.MethodGet || r.Method == http.MethodHead || r.Method == http.MethodOptions || r.URL.Path == "/api/v1/collect" || strings.HasPrefix(r.URL.Path, "/api/v1/auth/") {
			next.ServeHTTP(w, r)
			return
		}
		if _, err := r.Cookie("session_token"); err != nil || strings.HasPrefix(r.Header.Get("Authorization"), "Bearer ") {
			next.ServeHTTP(w, r)
			return
		}

		source := r.Header.Get("Origin")
		if source == "" {
			source = r.Header.Get("Referer")
		}
		parsed, err := url.Parse(source)
		if err != nil || source == "" || !a.sameOrigin(parsed, r) {
			requestHost := r.Host
			if a.requestMeta != nil {
				requestHost = a.requestMeta.Host(r)
			}
			requestScheme := "http"
			if a.requestMeta != nil {
				requestScheme = a.requestMeta.Scheme(r)
			} else if r.TLS != nil {
				requestScheme = "https"
			}
			log.Warn().
				Str("origin", source).
				Str("request_host", requestHost).
				Str("request_scheme", requestScheme).
				Str("remote_addr", r.RemoteAddr).
				Msg("Cross-origin request rejected by CSRF middleware")

			a.jsonError(w, "Cross-origin request rejected", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (a *App) sameOrigin(source *url.URL, r *http.Request) bool {
	requestHost := r.Host
	if a.requestMeta != nil {
		requestHost = a.requestMeta.Host(r)
	}

	hostMatched := strings.EqualFold(source.Host, requestHost) ||
		strings.EqualFold(stripPort(source.Host), stripPort(requestHost))
	if !hostMatched {
		return false
	}

	requestScheme := "http"
	if a.requestMeta != nil {
		requestScheme = a.requestMeta.Scheme(r)
	} else if r.TLS != nil {
		requestScheme = "https"
	}

	if strings.EqualFold(source.Scheme, requestScheme) {
		return true
	}

	// Allow https origin when request reached backend as http through a trusted proxy
	if source.Scheme == "https" && a.requestMeta != nil && a.requestMeta.Trusted(r.RemoteAddr) {
		return true
	}

	return false
}

func stripPort(hostport string) string {
	host, _, err := net.SplitHostPort(hostport)
	if err == nil {
		return host
	}
	return hostport
}

func (a *App) authRateLimitMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !a.authLimiter.allow(r.Context(), a.clientIP(r), time.Now()) {
			w.Header().Set("Retry-After", "900")
			a.jsonError(w, "Too many authentication attempts", http.StatusTooManyRequests)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (a *App) clientIP(request *http.Request) string {
	return a.requestMeta.ClientIP(request)
}

type requestIDContextKey struct{}

type statusRecorder struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
}

func (w *statusRecorder) WriteHeader(status int) {
	if w.wroteHeader {
		return
	}
	w.wroteHeader = true
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func newRequestID() string {
	value := make([]byte, 12)
	_, _ = rand.Read(value)
	return hex.EncodeToString(value)
}

func rateLimitKey(prefix, key string) string {
	hash := sha256.Sum256([]byte(key))
	return "svgstat:ratelimit:" + prefix + ":" + hex.EncodeToString(hash[:16])
}

func (a *App) getAuthToken(r *http.Request) string {
	cookie, err := r.Cookie("session_token")
	if err == nil {
		return cookie.Value
	}

	authHeader := r.Header.Get("Authorization")
	if strings.HasPrefix(authHeader, "Bearer ") {
		return strings.TrimPrefix(authHeader, "Bearer ")
	}

	return ""
}
