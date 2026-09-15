package api

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/mail"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/rs/zerolog/log"
	"github.com/svgstat/svgstat/internal/analytics"
	"github.com/svgstat/svgstat/internal/auth"
	"github.com/svgstat/svgstat/internal/conversion"
	"github.com/svgstat/svgstat/internal/diagnostics"
	"github.com/svgstat/svgstat/internal/project"
	"github.com/svgstat/svgstat/internal/renderer"
)

func (a *App) jsonError(w http.ResponseWriter, message string, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": false,
		"error":   message,
	})
}

func (a *App) jsonSuccess(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"data":    data,
	})
}

func (a *App) setSessionCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     "session_token",
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   a.config.HTTP.CookieSecure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   7 * 24 * 60 * 60,
	})
}

func (a *App) clearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     "session_token",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   a.config.HTTP.CookieSecure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
}

func (a *App) handleSPA(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "web/spa.html")
}

func (a *App) handleAdminSPA(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "web/admin/index.html")
}

func (a *App) handleWebsiteSDK(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=300")
	http.ServeFile(w, r, "web/static/js/sdk.js")
}

func (a *App) handleRegister(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 16<<10)
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
		Name     string `json:"name"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		a.jsonError(w, "Invalid request", http.StatusBadRequest)
		return
	}

	req.Email = strings.TrimSpace(strings.ToLower(req.Email))
	req.Name = strings.TrimSpace(req.Name)
	if err := validateRegistration(req.Email, req.Password, req.Name); err != nil {
		a.jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}

	user, err := a.auth.Register(r.Context(), req.Email, req.Password, req.Name)
	if err != nil {
		if err == auth.ErrUserExists {
			a.jsonError(w, "User already exists", http.StatusConflict)
			return
		}
		log.Error().Err(err).Msg("Failed to register user")
		a.jsonError(w, "Failed to register", http.StatusInternalServerError)
		return
	}

	session, err := a.auth.CreateSession(r.Context(), user.ID)
	if err != nil {
		log.Error().Err(err).Msg("Failed to create session")
		a.jsonError(w, "Failed to create session", http.StatusInternalServerError)
		return
	}

	a.setSessionCookie(w, session.Token)
	a.jsonSuccess(w, map[string]interface{}{
		"user":    user,
		"session": session,
	})
}

func (a *App) handleLogin(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 8<<10)
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		a.jsonError(w, "Invalid request", http.StatusBadRequest)
		return
	}

	req.Email = strings.TrimSpace(strings.ToLower(req.Email))
	if req.Email == "" || req.Password == "" || len(req.Password) > 72 {
		a.jsonError(w, "Invalid credentials", http.StatusUnauthorized)
		return
	}

	session, err := a.auth.Login(r.Context(), req.Email, req.Password)
	if err != nil {
		if err == auth.ErrInvalidCredentials {
			a.jsonError(w, "Invalid credentials", http.StatusUnauthorized)
			return
		}
		log.Error().Err(err).Msg("Failed to login")
		a.jsonError(w, "Failed to login", http.StatusInternalServerError)
		return
	}

	user, err := a.auth.GetUser(r.Context(), session.UserID)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get user")
		a.jsonError(w, "Failed to get user", http.StatusInternalServerError)
		return
	}

	a.setSessionCookie(w, session.Token)
	a.jsonSuccess(w, map[string]interface{}{
		"user":    user,
		"session": session,
	})
}

func (a *App) handleLogout(w http.ResponseWriter, r *http.Request) {
	token := a.getAuthToken(r)
	if token != "" {
		_ = a.auth.Logout(r.Context(), token)
	}
	a.clearSessionCookie(w)
	a.jsonSuccess(w, nil)
}

func (a *App) handleGetMe(w http.ResponseWriter, r *http.Request) {
	token := a.getAuthToken(r)
	if token == "" {
		a.jsonError(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	user, err := a.auth.ValidateSession(r.Context(), token)
	if err != nil {
		a.jsonError(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	a.jsonSuccess(w, user)
}

func (a *App) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := a.getAuthToken(r)
		if token == "" {
			a.jsonError(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		user, err := a.auth.ValidateSession(r.Context(), token)
		if err != nil {
			a.jsonError(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		ctx := context.WithValue(r.Context(), "user", user)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (a *App) handleGetProjects(w http.ResponseWriter, r *http.Request) {
	user := r.Context().Value("user").(*auth.User)

	projects, err := a.projectRepo.ListByUser(r.Context(), user.ID)
	if err != nil {
		log.Error().Err(err).Msg("Failed to list projects")
		a.jsonError(w, "Failed to list projects", http.StatusInternalServerError)
		return
	}

	a.jsonSuccess(w, projects)
}

func (a *App) handleCreateProject(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 32<<10)
	user := r.Context().Value("user").(*auth.User)

	var req struct {
		Name        string `json:"name"`
		Slug        string `json:"slug"`
		Description string `json:"description"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		a.jsonError(w, "Invalid request", http.StatusBadRequest)
		return
	}

	req.Name = strings.TrimSpace(req.Name)
	req.Slug = strings.TrimSpace(strings.ToLower(req.Slug))
	req.Description = strings.TrimSpace(req.Description)
	if err := validateProject(req.Name, req.Slug, req.Description); err != nil {
		a.jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}

	b := make([]byte, 16)
	rand.Read(b)
	id := fmt.Sprintf("%x", b)

	// 生成唯一的 external_project_id
	extID := make([]byte, 16)
	rand.Read(extID)
	externalID := fmt.Sprintf("%x", extID)

	// 使用用户ID作为租户ID
	tenantID := user.ID

	p := &project.Project{
		ID:                     id,
		UserID:                 user.ID,
		ExternalProjectID:      externalID,
		TenantID:               tenantID,
		Slug:                   req.Slug,
		Name:                   req.Name,
		Description:            req.Description,
		Status:                 "active",
		Visibility:             "public",
		WebsiteTrackingEnabled: true,
		WebsiteDomains:         []string{},
		RenderEnabled:          true,
		BadgeEnabled:           true,
		WidgetEnabled:          true,
		ChartEnabled:           false,
	}

	if err := a.projectRepo.Create(r.Context(), p); err != nil {
		log.Error().Err(err).Msg("Failed to create project")
		a.jsonError(w, "Failed to create project", http.StatusInternalServerError)
		return
	}
	if err := a.runtime.Put(r.Context(), p); err != nil {
		log.Warn().Err(err).Str("project_id", p.ID).Msg("Failed to refresh runtime project cache")
	}

	a.jsonSuccess(w, p)
}

func (a *App) handleGetProject(w http.ResponseWriter, r *http.Request) {
	user := r.Context().Value("user").(*auth.User)
	vars := mux.Vars(r)
	id := vars["id"]

	p, err := a.projectRepo.GetByIDAndUser(r.Context(), id, user.ID)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get project")
		a.jsonError(w, "Failed to get project", http.StatusInternalServerError)
		return
	}

	if p == nil {
		a.jsonError(w, "Project not found", http.StatusNotFound)
		return
	}

	a.jsonSuccess(w, p)
}

func (a *App) handleUpdateProject(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 32<<10)
	user := r.Context().Value("user").(*auth.User)
	vars := mux.Vars(r)
	id := vars["id"]

	var req struct {
		Name        string `json:"name"`
		Slug        string `json:"slug"`
		Description string `json:"description"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		a.jsonError(w, "Invalid request", http.StatusBadRequest)
		return
	}

	p, err := a.projectRepo.GetByIDAndUser(r.Context(), id, user.ID)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get project")
		a.jsonError(w, "Failed to get project", http.StatusInternalServerError)
		return
	}

	if p == nil {
		a.jsonError(w, "Project not found", http.StatusNotFound)
		return
	}
	previousSlug := p.Slug

	if req.Name != "" {
		p.Name = strings.TrimSpace(req.Name)
	}
	if req.Slug != "" {
		p.Slug = strings.TrimSpace(strings.ToLower(req.Slug))
	}
	if req.Description != "" {
		p.Description = strings.TrimSpace(req.Description)
	}
	if err := validateProject(p.Name, p.Slug, p.Description); err != nil {
		a.jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := a.projectRepo.Update(r.Context(), p); err != nil {
		log.Error().Err(err).Msg("Failed to update project")
		a.jsonError(w, "Failed to update project", http.StatusInternalServerError)
		return
	}
	if err := a.runtime.Invalidate(r.Context(), previousSlug, p.Slug); err != nil {
		log.Warn().Err(err).Str("project_id", p.ID).Msg("Failed to invalidate runtime project cache")
	}
	if err := a.runtime.Put(r.Context(), p); err != nil {
		log.Warn().Err(err).Str("project_id", p.ID).Msg("Failed to refresh runtime project cache")
	}

	a.jsonSuccess(w, p)
}

func (a *App) handleUpdateWebsiteTracking(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 32<<10)
	user := r.Context().Value("user").(*auth.User)
	id := mux.Vars(r)["id"]
	var req struct {
		Enabled bool     `json:"enabled"`
		Domains []string `json:"domains"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		a.jsonError(w, "Invalid request", http.StatusBadRequest)
		return
	}
	domains, err := normalizeWebsiteDomains(req.Domains)
	if err != nil {
		a.jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}
	p, err := a.projectRepo.GetByIDAndUser(r.Context(), id, user.ID)
	if err != nil {
		a.jsonError(w, "Failed to get project", http.StatusInternalServerError)
		return
	}
	if p == nil {
		a.jsonError(w, "Project not found", http.StatusNotFound)
		return
	}
	p.WebsiteTrackingEnabled = req.Enabled
	p.WebsiteDomains = domains
	if err := a.projectRepo.Update(r.Context(), p); err != nil {
		a.jsonError(w, "Failed to update website tracking", http.StatusInternalServerError)
		return
	}
	if err := a.runtime.Put(r.Context(), p); err != nil {
		log.Warn().Err(err).Str("project_id", p.ID).Msg("Failed to refresh runtime project cache")
	}
	a.jsonSuccess(w, p)
}

func (a *App) handleCollect(w http.ResponseWriter, r *http.Request) {
	setCollectionCORSHeaders(w, r.Header.Get("Origin"))
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 16<<10)
	if err := r.ParseForm(); err != nil {
		a.jsonError(w, "Invalid event", http.StatusBadRequest)
		return
	}
	slug := strings.TrimSpace(strings.ToLower(r.FormValue("project")))
	path := strings.TrimSpace(r.FormValue("path"))
	referrer := strings.TrimSpace(r.FormValue("referrer"))
	visitorID := strings.TrimSpace(r.FormValue("visitor"))
	eventType := strings.TrimSpace(strings.ToLower(r.FormValue("type")))
	mode := strings.TrimSpace(strings.ToLower(r.FormValue("mode")))
	if eventType == "" {
		eventType = "pageview"
	}
	if mode == "" {
		mode = "live"
	}
	entry := diagnostics.Entry{Mode: mode, Type: eventType, EventName: strings.TrimSpace(strings.ToLower(r.FormValue("event"))), Path: path, Origin: r.Header.Get("Origin"), Referrer: referrer}
	if err := validateCollectionEvent(slug, path, referrer, visitorID); err != nil {
		a.jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}
	limitKey := a.clientIP(r) + "|" + slug + "|" + visitorID
	if !a.collectLimiter.allow(r.Context(), limitKey, time.Now()) {
		w.Header().Set("Retry-After", "60")
		a.jsonError(w, "Too many events", http.StatusTooManyRequests)
		return
	}
	p, err := a.runtime.GetBySlug(r.Context(), slug)
	if err != nil || p == nil {
		a.jsonError(w, "Project not found", http.StatusNotFound)
		return
	}
	if p.Status != "active" || !p.WebsiteTrackingEnabled {
		a.recordDiagnostic(r, p.ID, entry, "rejected", "tracking_disabled")
		a.jsonError(w, "Website tracking disabled", http.StatusForbidden)
		return
	}
	if !websiteOriginAllowed(r.Header.Get("Origin"), p.WebsiteDomains) {
		a.recordDiagnostic(r, p.ID, entry, "rejected", "origin_not_allowed")
		a.jsonError(w, "Origin not allowed", http.StatusForbidden)
		return
	}
	if mode != "live" && mode != "test" {
		a.recordDiagnostic(r, p.ID, entry, "rejected", "invalid_mode")
		a.jsonError(w, "Invalid collection mode", http.StatusBadRequest)
		return
	}
	_, _, entry.Source, entry.Medium, entry.Campaign = analytics.WebsiteAttribution(path, referrer)
	var trackErr error
	if eventType == "pageview" {
		if shouldWriteAnalytics(mode) {
			trackErr = a.analytics.TrackPageview(r.Context(), r, p.ID, path, referrer, visitorID)
		}
	} else if eventType == "event" {
		event, err := parseCustomEvent(r.FormValue("event"), r.FormValue("properties"), path, referrer, visitorID)
		if err != nil {
			a.recordDiagnostic(r, p.ID, entry, "rejected", diagnosticReason(err))
			a.jsonError(w, err.Error(), http.StatusBadRequest)
			return
		}
		entry.EventName = event.Name
		entry.PropertyKeys = event.PropertyKeys
		if shouldWriteAnalytics(mode) {
			trackErr = a.analytics.TrackEvent(r.Context(), r, p.ID, event)
		}
	} else {
		a.recordDiagnostic(r, p.ID, entry, "rejected", "invalid_event_type")
		a.jsonError(w, "Invalid event type", http.StatusBadRequest)
		return
	}
	if trackErr != nil {
		a.recordDiagnostic(r, p.ID, entry, "rejected", "storage_unavailable")
		log.Error().Err(trackErr).Str("project_id", p.ID).Msg("Failed to collect website event")
		a.jsonError(w, "Failed to collect event", http.StatusServiceUnavailable)
		return
	}
	a.recordDiagnostic(r, p.ID, entry, "accepted", "")
	w.WriteHeader(http.StatusNoContent)
}

func shouldWriteAnalytics(mode string) bool { return mode == "live" }

func (a *App) recordDiagnostic(r *http.Request, projectID string, entry diagnostics.Entry, status, reason string) {
	entry.Status, entry.Reason = status, reason
	if err := a.diagnostics.Record(r.Context(), projectID, entry); err != nil {
		log.Warn().Err(err).Str("project_id", projectID).Msg("Failed to record collection diagnostic")
	}
}

func diagnosticReason(err error) string {
	message := strings.ToLower(err.Error())
	switch {
	case strings.Contains(message, "event name"):
		return "invalid_event_name"
	case strings.Contains(message, "properties"), strings.Contains(message, "property"):
		return "invalid_properties"
	case strings.Contains(message, "currency"):
		return "invalid_currency"
	default:
		return "invalid_event"
	}
}

func parseCustomEvent(name, rawProperties, path, referrer, visitorID string) (analytics.EventData, error) {
	name, err := conversion.NormalizeEventName(name)
	if err != nil {
		return analytics.EventData{}, err
	}
	properties := make(map[string]interface{})
	if strings.TrimSpace(rawProperties) != "" {
		decoder := json.NewDecoder(strings.NewReader(rawProperties))
		decoder.UseNumber()
		if err := decoder.Decode(&properties); err != nil {
			return analytics.EventData{}, fmt.Errorf("Invalid event properties")
		}
		if err := decoder.Decode(&struct{}{}); err != io.EOF {
			return analytics.EventData{}, fmt.Errorf("Invalid event properties")
		}
	}
	if len(properties) > 10 {
		return analytics.EventData{}, fmt.Errorf("At most 10 event properties are allowed")
	}
	result := analytics.EventData{Name: name, Path: path, Referrer: referrer, VisitorID: visitorID, Currency: "XXX", PropertyKeys: diagnostics.PropertyKeys(properties)}
	for key, raw := range properties {
		if !eventPropertyPattern.MatchString(key) {
			return analytics.EventData{}, fmt.Errorf("Invalid event property key")
		}
		switch value := raw.(type) {
		case string:
			if len([]rune(value)) > 256 {
				return analytics.EventData{}, fmt.Errorf("Event property value is too long")
			}
			if key == "currency" {
				currency := strings.ToUpper(strings.TrimSpace(value))
				if len(currency) != 3 || !regexp.MustCompile(`^[A-Z]{3}$`).MatchString(currency) {
					return analytics.EventData{}, fmt.Errorf("Invalid event currency")
				}
				result.Currency = currency
			}
		case json.Number:
			number, err := value.Float64()
			if err != nil || number < 0 || number > 1e12 {
				return analytics.EventData{}, fmt.Errorf("Invalid numeric event property")
			}
			if key == "value" {
				result.Value = &number
			}
		case bool:
		case nil:
			return analytics.EventData{}, fmt.Errorf("Event properties must be scalar values")
		default:
			return analytics.EventData{}, fmt.Errorf("Event properties must be scalar values")
		}
	}
	result.Dimensions = automaticEventDimensions(name, properties)
	if strings.HasPrefix(name, "web_vital_") {
		metric := strings.TrimPrefix(name, "web_vital_")
		if result.Value == nil || !validWebVitalValue(metric, *result.Value) {
			return analytics.EventData{}, fmt.Errorf("Invalid web vital value")
		}
		result.Dimensions = map[string]string{"rating": automaticWebVitalRating(metric, *result.Value)}
	}
	return result, nil
}

func validWebVitalValue(metric string, value float64) bool {
	switch metric {
	case "lcp":
		return value >= 0 && value <= 600000
	case "inp":
		return value >= 0 && value <= 60000
	case "cls":
		return value >= 0 && value <= 100
	default:
		return false
	}
}

func automaticWebVitalRating(metric string, value float64) string {
	good, poor := 0.1, 0.25
	if metric == "lcp" {
		good, poor = 2500, 4000
	} else if metric == "inp" {
		good, poor = 200, 500
	}
	if value <= good {
		return "good"
	}
	if value <= poor {
		return "needs_improvement"
	}
	return "poor"
}

func automaticEventDimensions(eventName string, properties map[string]interface{}) map[string]string {
	allowed := map[string][]string{
		"outbound_click": {"target_domain", "target_path"},
		"file_download":  {"file_extension", "target_domain", "target_path"},
		"contact_click":  {"contact_type"},
		"form_submit":    {"form_id", "method", "target_domain", "target_path"},
		"web_vital_lcp":  {"rating"},
		"web_vital_inp":  {"rating"},
		"web_vital_cls":  {"rating"},
		"js_error":       {"error_type"},
		"resource_error": {"resource_type", "target_domain", "target_path"},
	}[eventName]
	result := map[string]string{}
	for _, key := range allowed {
		value, ok := properties[key].(string)
		if !ok {
			continue
		}
		if value = normalizeAutomaticEventDimension(key, value); value != "" {
			result[key] = value
		}
	}
	return result
}

func normalizeAutomaticEventDimension(key, value string) string {
	value = strings.ReplaceAll(strings.TrimSpace(value), compositeEventSeparator, "")
	switch key {
	case "target_path":
		if index := strings.IndexAny(value, "?#"); index >= 0 {
			value = value[:index]
		}
		if !strings.HasPrefix(value, "/") || len(value) > 256 {
			return ""
		}
	case "target_domain":
		parsed, err := url.Parse("https://" + strings.ToLower(value))
		if err != nil || parsed.User != nil || parsed.Hostname() == "" || parsed.Port() != "" || parsed.Path != "" || parsed.RawQuery != "" || parsed.Fragment != "" {
			return ""
		}
		value = parsed.Hostname()
		if len(value) > 253 {
			return ""
		}
	case "file_extension":
		value = strings.ToLower(value)
		if !regexp.MustCompile(`^[a-z0-9]{1,16}$`).MatchString(value) {
			return ""
		}
	case "contact_type":
		if value != "email" && value != "phone" {
			return ""
		}
	case "method":
		value = strings.ToLower(value)
		if !regexp.MustCompile(`^(get|post|put|patch|delete)$`).MatchString(value) {
			return ""
		}
	case "form_id":
		if !regexp.MustCompile(`^[a-zA-Z0-9_.:-]{1,64}$`).MatchString(value) {
			return ""
		}
	case "rating":
		if value != "good" && value != "needs_improvement" && value != "poor" {
			return ""
		}
	case "error_type":
		if !regexp.MustCompile(`^(type_error|reference_error|syntax_error|range_error|unhandled_rejection|unknown)$`).MatchString(value) {
			return ""
		}
	case "resource_type":
		if !regexp.MustCompile(`^(script|stylesheet|image|media|iframe|other)$`).MatchString(value) {
			return ""
		}
	}
	return value
}

const compositeEventSeparator = "\x1f"

func setCollectionCORSHeaders(w http.ResponseWriter, origin string) {
	if origin == "" {
		return
	}
	w.Header().Set("Access-Control-Allow-Origin", origin)
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	w.Header().Set("Access-Control-Max-Age", "86400")
	w.Header().Add("Vary", "Origin")
}

func validateCollectionEvent(slug, path, referrer, visitorID string) error {
	if !slugPattern.MatchString(slug) {
		return fmt.Errorf("Invalid project key")
	}
	if path == "" || !strings.HasPrefix(path, "/") || len(path) > 2048 {
		return fmt.Errorf("Invalid page path")
	}
	if len(referrer) > 2048 {
		return fmt.Errorf("Referrer is too long")
	}
	if len(visitorID) > 64 || (visitorID != "" && !metricPattern.MatchString(visitorID)) {
		return fmt.Errorf("Invalid visitor ID")
	}
	return nil
}

func normalizeWebsiteDomains(values []string) ([]string, error) {
	if len(values) > 50 {
		return nil, fmt.Errorf("At most 50 domains are allowed")
	}
	seen := make(map[string]struct{})
	result := make([]string, 0, len(values))
	for _, value := range values {
		domain, err := normalizeWebsiteDomain(value)
		if err != nil {
			return nil, err
		}
		if domain == "" {
			continue
		}
		if _, ok := seen[domain]; !ok {
			seen[domain] = struct{}{}
			result = append(result, domain)
		}
	}
	return result, nil
}

func normalizeWebsiteDomain(value string) (string, error) {
	value = strings.TrimSpace(strings.ToLower(value))
	if value == "" {
		return "", nil
	}
	wildcard := strings.HasPrefix(value, "*.")
	if wildcard {
		value = strings.TrimPrefix(value, "*.")
	}
	if !strings.Contains(value, "://") {
		value = "https://" + value
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.Host == "" || parsed.User != nil || parsed.Path != "" || parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", fmt.Errorf("Invalid website domain")
	}
	host := strings.ToLower(parsed.Host)
	if wildcard {
		host = "*." + strings.ToLower(parsed.Hostname())
	}
	if len(host) > 253 {
		return "", fmt.Errorf("Website domain is too long")
	}
	return host, nil
}

func websiteOriginAllowed(origin string, domains []string) bool {
	parsed, err := url.Parse(strings.TrimSpace(origin))
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return false
	}
	if len(domains) == 0 {
		return true
	}
	host := strings.ToLower(parsed.Host)
	hostname := strings.ToLower(parsed.Hostname())
	for _, domain := range domains {
		domain = strings.ToLower(domain)
		if host == domain || hostname == domain {
			return true
		}
		if strings.HasPrefix(domain, "*.") {
			base := strings.TrimPrefix(domain, "*.")
			if hostname != base && strings.HasSuffix(hostname, "."+base) {
				return true
			}
		}
	}
	return false
}

func (a *App) handleDeleteProject(w http.ResponseWriter, r *http.Request) {
	user := r.Context().Value("user").(*auth.User)
	vars := mux.Vars(r)
	id := vars["id"]
	p, err := a.projectRepo.GetByIDAndUser(r.Context(), id, user.ID)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get project")
		a.jsonError(w, "Failed to get project", http.StatusInternalServerError)
		return
	}
	if p == nil {
		a.jsonError(w, "Project not found", http.StatusNotFound)
		return
	}

	if err := a.projectRepo.Delete(r.Context(), id, user.ID); err != nil {
		log.Error().Err(err).Msg("Failed to delete project")
		a.jsonError(w, "Failed to delete project", http.StatusInternalServerError)
		return
	}
	if err := a.runtime.Invalidate(r.Context(), p.Slug); err != nil {
		log.Warn().Err(err).Str("project_id", p.ID).Msg("Failed to invalidate runtime project cache")
	}

	a.jsonSuccess(w, nil)
}

func (a *App) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"ok"}`))
}

func (a *App) handleReady(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	if err := a.db.Pool.Ping(ctx); err != nil {
		a.jsonError(w, "Database unavailable", http.StatusServiceUnavailable)
		return
	}
	if err := a.cache.Ping(ctx); err != nil {
		a.jsonError(w, "Redis unavailable", http.StatusServiceUnavailable)
		return
	}
	a.jsonSuccess(w, map[string]string{"status": "ready"})
}

func (a *App) handleMetrics(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
	a.observability.WritePrometheus(w)
}

// resolveCounterName keys counts by page_id when one is given, so every
// embedding page gets its own independent counter (visitor-badge semantics);
// embeds without page_id share one count per counter name.
func resolveCounterName(r *http.Request, name string) string {
	if pageID := strings.TrimSpace(r.URL.Query().Get("page_id")); pageID != "" {
		return name + "@" + pageID
	}
	return name
}

var slugPattern = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?$`)
var metricPattern = regexp.MustCompile(`^[A-Za-z0-9](?:[A-Za-z0-9_.-]{0,62}[A-Za-z0-9])?$`)
var eventPropertyPattern = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_.-]{0,63}$`)

func validateRegistration(email, password, name string) error {
	address, err := mail.ParseAddress(email)
	if err != nil || address.Address != email || len(email) > 254 {
		return fmt.Errorf("Enter a valid email address")
	}
	if len(password) < 8 || len(password) > 72 {
		return fmt.Errorf("Password must be between 8 and 72 characters")
	}
	if len([]rune(name)) > 80 {
		return fmt.Errorf("Name must be 80 characters or fewer")
	}
	return nil
}

func validateProject(name, slug, description string) error {
	if name == "" || len([]rune(name)) > 100 {
		return fmt.Errorf("Project name must be between 1 and 100 characters")
	}
	if !slugPattern.MatchString(slug) {
		return fmt.Errorf("Slug must use 1-64 lowercase letters, numbers, or hyphens")
	}
	if len([]rune(description)) > 500 {
		return fmt.Errorf("Description must be 500 characters or fewer")
	}
	return nil
}

func validateSVGRequest(r *http.Request, name string) error {
	if !metricPattern.MatchString(name) {
		return fmt.Errorf("Invalid metric name")
	}
	if len([]rune(r.URL.Query().Get("label"))) > 64 {
		return fmt.Errorf("Label must be 64 characters or fewer")
	}
	if len(r.URL.Query().Get("page_id")) > 256 {
		return fmt.Errorf("Page ID must be 256 characters or fewer")
	}
	if len(r.URL.Query().Get("homepage")) > 2048 {
		return fmt.Errorf("Homepage URL is too long")
	}
	if preview := r.URL.Query().Get("preview"); preview != "" && preview != "0" && preview != "1" {
		return fmt.Errorf("Preview must be 0 or 1")
	}
	return nil
}

func isPreviewRequest(r *http.Request) bool {
	return r.URL.Query().Get("preview") == "1"
}

// setNoCacheHeaders defeats intermediary caches, in particular GitHub's Camo
// image proxy (fronted by a CDN that keys off max-age/s-maxage and a past
// Expires rather than no-cache/no-store alone).
func setNoCacheHeaders(w http.ResponseWriter) {
	w.Header().Set("Cache-Control", "no-cache, no-store, max-age=0, s-maxage=0, must-revalidate, proxy-revalidate")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Expires", time.Now().UTC().Add(-10*time.Minute).Format(http.TimeFormat))
}

func (a *App) handleCounterSVG(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	projectSlug := vars["projectSlug"]
	name := vars["name"]
	if err := validateSVGRequest(r, name); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	proj, err := a.runtime.GetBySlug(r.Context(), projectSlug)
	if err != nil {
		log.Error().Err(err).Str("slug", projectSlug).Msg("Failed to get project")
		http.Error(w, "Project not found", http.StatusNotFound)
		return
	}
	if proj == nil {
		http.Error(w, "Project not found", http.StatusNotFound)
		return
	}

	if proj.Status != "active" || !proj.RenderEnabled {
		http.Error(w, "Rendering disabled", http.StatusForbidden)
		return
	}

	preview := isPreviewRequest(r)
	var value int64
	if preview {
		value, err = a.counter.Get(r.Context(), proj.ID, resolveCounterName(r, name))
	} else {
		value, err = a.counter.IncrementWithAnalytics(r.Context(), proj.ID, resolveCounterName(r, name))
	}
	if err != nil {
		log.Error().Err(err).Str("project_id", proj.ID).Str("counter", name).Msg("Failed to increment counter")
		a.writeUnavailableSVG(w)
		return
	}

	if !preview {
		_ = a.analytics.TrackRequest(r.Context(), r, proj.ID)
	}

	color := r.URL.Query().Get("color")
	label := r.URL.Query().Get("label")
	homepage := a.getSVGHomepage(r)

	svg, err := a.renderer.RenderCounter(renderer.CounterData{
		Value:       value,
		Label:       label,
		Color:       color,
		HomepageURL: homepage,
	})
	if err != nil {
		log.Error().Err(err).Msg("Failed to render counter")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "image/svg+xml")
	setNoCacheHeaders(w)
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(svg))
}

func (a *App) handleBadgeSVG(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	projectSlug := vars["projectSlug"]
	name := vars["name"]
	if err := validateSVGRequest(r, name); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	proj, err := a.runtime.GetBySlug(r.Context(), projectSlug)
	if err != nil {
		log.Error().Err(err).Str("slug", projectSlug).Msg("Failed to get project")
		http.Error(w, "Project not found", http.StatusNotFound)
		return
	}
	if proj == nil {
		http.Error(w, "Project not found", http.StatusNotFound)
		return
	}

	if proj.Status != "active" || !proj.BadgeEnabled {
		http.Error(w, "Badges disabled", http.StatusForbidden)
		return
	}

	preview := isPreviewRequest(r)
	var value int64
	if preview {
		value, err = a.counter.Get(r.Context(), proj.ID, resolveCounterName(r, name))
	} else {
		value, err = a.counter.Increment(r.Context(), proj.ID, resolveCounterName(r, name))
	}
	if err != nil {
		log.Error().Err(err).Str("project_id", proj.ID).Str("counter", name).Msg("Failed to increment counter")
		a.writeUnavailableSVG(w)
		return
	}

	if !preview {
		_ = a.analytics.TrackRequest(r.Context(), r, proj.ID)
	}

	color := r.URL.Query().Get("color")
	style := r.URL.Query().Get("style")
	label := r.URL.Query().Get("label")
	homepage := a.getSVGHomepage(r)
	if label == "" {
		label = name
	}

	svg, err := a.renderer.RenderBadge(renderer.BadgeData{
		Label:       label,
		Value:       fmt.Sprintf("%d", value),
		Color:       color,
		Style:       style,
		HomepageURL: homepage,
	})
	if err != nil {
		log.Error().Err(err).Msg("Failed to render badge")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "image/svg+xml")
	setNoCacheHeaders(w)
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(svg))
}

func (a *App) writeUnavailableSVG(w http.ResponseWriter) {
	svg, err := a.renderer.RenderBadge(renderer.BadgeData{Label: "svgstat", Value: "unavailable", Color: "lightgrey", Style: "flat"})
	if err != nil {
		http.Error(w, "Service unavailable", http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "image/svg+xml")
	w.Header().Set("X-SVGStat-Degraded", "true")
	setNoCacheHeaders(w)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(svg))
}

func (a *App) handleGetProjectStats(w http.ResponseWriter, r *http.Request) {
	user := r.Context().Value("user").(*auth.User)
	vars := mux.Vars(r)
	id := vars["id"]

	p, err := a.projectRepo.GetByIDAndUser(r.Context(), id, user.ID)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get project")
		a.jsonError(w, "Failed to get project", http.StatusInternalServerError)
		return
	}

	if p == nil {
		a.jsonError(w, "Project not found", http.StatusNotFound)
		return
	}

	stats, err := a.analytics.GetTodayStats(r.Context(), id)
	if err != nil {
		log.Error().Err(err).Str("project_id", id).Msg("Failed to get stats")
		a.jsonError(w, "Failed to get stats", http.StatusInternalServerError)
		return
	}

	a.jsonSuccess(w, stats)
}

func (a *App) handleGetProjectTrend(w http.ResponseWriter, r *http.Request) {
	user := r.Context().Value("user").(*auth.User)
	id := mux.Vars(r)["id"]

	p, err := a.projectRepo.GetByIDAndUser(r.Context(), id, user.ID)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get project")
		a.jsonError(w, "Failed to get project", http.StatusInternalServerError)
		return
	}
	if p == nil {
		a.jsonError(w, "Project not found", http.StatusNotFound)
		return
	}

	days := 30
	if raw := r.URL.Query().Get("days"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || (parsed != 7 && parsed != 30 && parsed != 90) {
			a.jsonError(w, "Days must be one of 7, 30, or 90", http.StatusBadRequest)
			return
		}
		days = parsed
	}

	trend, err := a.metrics.GetTrend(r.Context(), id, days, time.Now())
	if err != nil {
		log.Error().Err(err).Str("project_id", id).Msg("Failed to get project trend")
		a.jsonError(w, "Failed to get project trend", http.StatusInternalServerError)
		return
	}
	a.jsonSuccess(w, trend)
}

func (a *App) handleGetProjectRealtime(w http.ResponseWriter, r *http.Request) {
	user := r.Context().Value("user").(*auth.User)
	id := mux.Vars(r)["id"]
	p, err := a.projectRepo.GetByIDAndUser(r.Context(), id, user.ID)
	if err != nil {
		a.jsonError(w, "Failed to get project", http.StatusInternalServerError)
		return
	}
	if p == nil {
		a.jsonError(w, "Project not found", http.StatusNotFound)
		return
	}
	stats, err := a.metrics.GetRealtimeStats(r.Context(), id, time.Now())
	if err != nil {
		log.Error().Err(err).Str("project_id", id).Msg("Failed to get realtime statistics")
		a.jsonError(w, "Failed to get realtime statistics", http.StatusInternalServerError)
		return
	}
	a.jsonSuccess(w, stats)
}

func (a *App) handleGetProjectAnalysis(w http.ResponseWriter, r *http.Request) {
	user := r.Context().Value("user").(*auth.User)
	id := mux.Vars(r)["id"]
	p, err := a.projectRepo.GetByIDAndUser(r.Context(), id, user.ID)
	if err != nil {
		a.jsonError(w, "Failed to get project", http.StatusInternalServerError)
		return
	}
	if p == nil {
		a.jsonError(w, "Project not found", http.StatusNotFound)
		return
	}
	days := 30
	if raw := r.URL.Query().Get("days"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || (parsed != 7 && parsed != 30 && parsed != 90) {
			a.jsonError(w, "Days must be one of 7, 30, or 90", http.StatusBadRequest)
			return
		}
		days = parsed
	}
	analysis, err := a.metrics.GetAnalysis(r.Context(), id, days, time.Now())
	if err != nil {
		log.Error().Err(err).Str("project_id", id).Msg("Failed to get project analysis")
		a.jsonError(w, "Failed to get project analysis", http.StatusInternalServerError)
		return
	}
	a.jsonSuccess(w, analysis)
}

func (a *App) handleGetProjectSessionQuality(w http.ResponseWriter, r *http.Request) {
	id, ok := a.authorizeProject(w, r)
	if !ok {
		return
	}
	days, ok := analysisDays(r)
	if !ok {
		a.jsonError(w, "Days must be one of 7, 30, or 90", http.StatusBadRequest)
		return
	}
	report, err := a.metrics.GetSessionQuality(r.Context(), id, days, time.Now())
	if err != nil {
		log.Error().Err(err).Str("project_id", id).Msg("Failed to get session quality")
		a.jsonError(w, "Failed to get session quality", http.StatusInternalServerError)
		return
	}
	a.jsonSuccess(w, report)
}

func (a *App) handleGetProjectInstallation(w http.ResponseWriter, r *http.Request) {
	user := r.Context().Value("user").(*auth.User)
	id := mux.Vars(r)["id"]
	p, err := a.projectRepo.GetByIDAndUser(r.Context(), id, user.ID)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get project")
		a.jsonError(w, "Failed to get project", http.StatusInternalServerError)
		return
	}
	if p == nil {
		a.jsonError(w, "Project not found", http.StatusNotFound)
		return
	}

	status, err := a.metrics.GetInstallationStatus(r.Context(), id)
	if err != nil {
		log.Error().Err(err).Str("project_id", id).Msg("Failed to get installation status")
		a.jsonError(w, "Failed to get installation status", http.StatusInternalServerError)
		return
	}
	a.jsonSuccess(w, status)
}

// getSVGHomepage decides the click-through target of the rendered SVG: an
// explicit ?homepage= override, otherwise this service's own domain — the
// request carries no Referer/Origin when GitHub's Camo proxy fetches it.
func (a *App) getSVGHomepage(r *http.Request) string {
	homepage := strings.TrimSpace(r.URL.Query().Get("homepage"))
	if homepage != "" {
		return homepage
	}

	if r.Host == "" {
		return ""
	}
	scheme := a.requestMeta.Scheme(r)
	return scheme + "://" + r.Host
}

func (a *App) handleGetProjectVisitors(w http.ResponseWriter, r *http.Request) {
	user := r.Context().Value("user").(*auth.User)
	vars := mux.Vars(r)
	id := vars["id"]

	p, err := a.projectRepo.GetByIDAndUser(r.Context(), id, user.ID)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get project")
		a.jsonError(w, "Failed to get project", http.StatusInternalServerError)
		return
	}

	if p == nil {
		a.jsonError(w, "Project not found", http.StatusNotFound)
		return
	}

	page := 1
	pageSize := 20
	if raw := r.URL.Query().Get("page"); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			page = parsed
		}
	}
	if raw := r.URL.Query().Get("page_size"); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			pageSize = parsed
		}
	}

	visitors, err := a.analytics.GetTodayVisitors(r.Context(), id, analytics.VisitorQuery{
		Page:     page,
		PageSize: pageSize,
		Device:   r.URL.Query().Get("device"),
		Browser:  r.URL.Query().Get("browser"),
		Path:     r.URL.Query().Get("path"),
		Sort:     r.URL.Query().Get("sort"),
	})
	if err != nil {
		log.Error().Err(err).Str("project_id", id).Msg("Failed to get visitors")
		a.jsonError(w, "Failed to get visitors", http.StatusInternalServerError)
		return
	}

	a.jsonSuccess(w, visitors)
}
