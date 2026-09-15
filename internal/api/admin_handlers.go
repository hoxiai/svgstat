package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/gorilla/mux"
	"github.com/rs/zerolog/log"
	adminservice "github.com/svgstat/svgstat/internal/admin"
	"github.com/svgstat/svgstat/internal/auth"
)

func (a *App) adminMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, ok := r.Context().Value("user").(*auth.User)
		if !ok || user.Role != "admin" {
			a.jsonError(w, "Forbidden", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (a *App) handleAdminMe(w http.ResponseWriter, r *http.Request) {
	a.jsonSuccess(w, r.Context().Value("user"))
}

func (a *App) handleAdminOverview(w http.ResponseWriter, r *http.Request) {
	overview, err := a.admin.GetOverview(r.Context())
	if err != nil {
		log.Error().Err(err).Msg("Failed to load admin overview")
		a.jsonError(w, "Failed to load overview", http.StatusInternalServerError)
		return
	}
	a.jsonSuccess(w, overview)
}

func (a *App) handleAdminUsers(w http.ResponseWriter, r *http.Request) {
	page, err := a.admin.ListUsers(r.Context(), adminQuery(r))
	if err != nil {
		log.Error().Err(err).Msg("Failed to list admin users")
		a.jsonError(w, "Failed to list users", http.StatusInternalServerError)
		return
	}
	a.jsonSuccess(w, page)
}

func (a *App) handleAdminUser(w http.ResponseWriter, r *http.Request) {
	user, projects, err := a.admin.GetUser(r.Context(), mux.Vars(r)["id"])
	if err != nil {
		a.handleAdminError(w, err, "Failed to load user")
		return
	}
	a.jsonSuccess(w, map[string]interface{}{"user": user, "projects": projects})
}

func (a *App) handleAdminUserStatus(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Status string `json:"status"`
	}
	if !decodeAdminJSON(w, r, &request) {
		return
	}
	adminUser := r.Context().Value("user").(*auth.User)
	user, err := a.admin.UpdateUserStatus(r.Context(), adminUser.ID, mux.Vars(r)["id"], strings.ToLower(strings.TrimSpace(request.Status)), a.clientIP(r))
	if err != nil {
		a.handleAdminError(w, err, "Failed to update user")
		return
	}
	a.jsonSuccess(w, user)
}

func (a *App) handleAdminProjects(w http.ResponseWriter, r *http.Request) {
	page, err := a.admin.ListProjects(r.Context(), adminQuery(r))
	if err != nil {
		log.Error().Err(err).Msg("Failed to list admin projects")
		a.jsonError(w, "Failed to list projects", http.StatusInternalServerError)
		return
	}
	a.jsonSuccess(w, page)
}

func (a *App) handleAdminProject(w http.ResponseWriter, r *http.Request) {
	project, err := a.admin.GetProject(r.Context(), mux.Vars(r)["id"])
	if err != nil {
		a.handleAdminError(w, err, "Failed to load project")
		return
	}
	a.jsonSuccess(w, project)
}

func (a *App) handleAdminProjectStatus(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Status string `json:"status"`
	}
	if !decodeAdminJSON(w, r, &request) {
		return
	}
	adminUser := r.Context().Value("user").(*auth.User)
	project, err := a.admin.UpdateProjectStatus(r.Context(), adminUser.ID, mux.Vars(r)["id"], strings.ToLower(strings.TrimSpace(request.Status)), a.clientIP(r))
	if err != nil {
		a.handleAdminError(w, err, "Failed to update project")
		return
	}
	a.jsonSuccess(w, project)
}

func (a *App) handleAdminProjectCapabilities(w http.ResponseWriter, r *http.Request) {
	var request adminservice.Capabilities
	if !decodeAdminJSON(w, r, &request) {
		return
	}
	adminUser := r.Context().Value("user").(*auth.User)
	project, err := a.admin.UpdateProjectCapabilities(r.Context(), adminUser.ID, mux.Vars(r)["id"], a.clientIP(r), request)
	if err != nil {
		a.handleAdminError(w, err, "Failed to update project capabilities")
		return
	}
	a.jsonSuccess(w, project)
}

func (a *App) handleAdminError(w http.ResponseWriter, err error, fallback string) {
	switch {
	case errors.Is(err, adminservice.ErrNotFound):
		a.jsonError(w, "Not found", http.StatusNotFound)
	case errors.Is(err, adminservice.ErrInvalidStatus):
		a.jsonError(w, "Invalid request", http.StatusBadRequest)
	case errors.Is(err, adminservice.ErrSelfDisable), errors.Is(err, adminservice.ErrProtectedEntry):
		a.jsonError(w, err.Error(), http.StatusConflict)
	default:
		log.Error().Err(err).Msg(fallback)
		a.jsonError(w, fallback, http.StatusInternalServerError)
	}
}

func adminQuery(r *http.Request) adminservice.Query {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	pageSize, _ := strconv.Atoi(r.URL.Query().Get("pageSize"))
	return adminservice.Query{
		Search: r.URL.Query().Get("q"), Status: r.URL.Query().Get("status"), Page: page, PageSize: pageSize,
	}
}

func decodeAdminJSON(w http.ResponseWriter, r *http.Request, target interface{}) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 16<<10)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "error": "Invalid request"})
		return false
	}
	return true
}
