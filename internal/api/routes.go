package api

import (
	"net/http"

	"github.com/gorilla/mux"
)

func (a *App) SetupRoutes() *mux.Router {
	r := mux.NewRouter()

	r.Use(a.loggingMiddleware)
	r.Use(a.securityHeadersMiddleware)
	r.Use(a.csrfMiddleware)

	r.HandleFunc("/health", a.handleHealth).Methods("GET")
	r.HandleFunc("/ready", a.handleReady).Methods("GET")
	r.HandleFunc("/metrics", a.handleMetrics).Methods("GET")

	r.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.FileServer(http.Dir("web/static"))))
	r.PathPrefix("/components/").Handler(http.StripPrefix("/components/", http.FileServer(http.Dir("web/components"))))
	r.PathPrefix("/admin/static/").Handler(http.StripPrefix("/admin/static/", http.FileServer(http.Dir("web/admin/static"))))

	api := r.PathPrefix("/api/v1").Subrouter()
	api.Handle("/auth/register", a.authRateLimitMiddleware(http.HandlerFunc(a.handleRegister))).Methods("POST")
	api.Handle("/auth/login", a.authRateLimitMiddleware(http.HandlerFunc(a.handleLogin))).Methods("POST")
	api.HandleFunc("/auth/logout", a.handleLogout).Methods("POST")
	api.HandleFunc("/auth/me", a.handleGetMe).Methods("GET")
	api.HandleFunc("/collect", a.handleCollect).Methods("POST", "OPTIONS")

	projects := api.PathPrefix("/projects").Subrouter()
	projects.Use(a.authMiddleware)
	projects.HandleFunc("", a.handleGetProjects).Methods("GET")
	projects.HandleFunc("", a.handleCreateProject).Methods("POST")
	projects.HandleFunc("/{id}", a.handleGetProject).Methods("GET")
	projects.HandleFunc("/{id}", a.handleUpdateProject).Methods("PUT")
	projects.HandleFunc("/{id}", a.handleDeleteProject).Methods("DELETE")
	projects.HandleFunc("/{id}/stats", a.handleGetProjectStats).Methods("GET")
	projects.HandleFunc("/{id}/stats/trend", a.handleGetProjectTrend).Methods("GET")
	projects.HandleFunc("/{id}/stats/realtime", a.handleGetProjectRealtime).Methods("GET")
	projects.HandleFunc("/{id}/analysis", a.handleGetProjectAnalysis).Methods("GET")
	projects.HandleFunc("/{id}/session-quality", a.handleGetProjectSessionQuality).Methods("GET")
	projects.HandleFunc("/{id}/events", a.handleGetProjectEvents).Methods("GET")
	projects.HandleFunc("/{id}/issues", a.handleGetProjectIssues).Methods("GET")
	projects.HandleFunc("/{id}/diagnostics", a.handleGetProjectDiagnostics).Methods("GET")
	projects.HandleFunc("/{id}/diagnostics", a.handleClearProjectDiagnostics).Methods("DELETE")
	projects.HandleFunc("/{id}/goals", a.handleListGoals).Methods("GET")
	projects.HandleFunc("/{id}/goals", a.handleCreateGoal).Methods("POST")
	projects.HandleFunc("/{id}/goals/{goalId}", a.handleUpdateGoal).Methods("PUT")
	projects.HandleFunc("/{id}/goals/{goalId}", a.handleDeleteGoal).Methods("DELETE")
	projects.HandleFunc("/{id}/funnels", a.handleListFunnels).Methods("GET")
	projects.HandleFunc("/{id}/funnels", a.handleCreateFunnel).Methods("POST")
	projects.HandleFunc("/{id}/funnels/{funnelId}", a.handleUpdateFunnel).Methods("PUT")
	projects.HandleFunc("/{id}/funnels/{funnelId}", a.handleDeleteFunnel).Methods("DELETE")
	projects.HandleFunc("/{id}/funnels/{funnelId}/analysis", a.handleGetFunnelAnalysis).Methods("GET")
	projects.HandleFunc("/{id}/installation", a.handleGetProjectInstallation).Methods("GET")
	projects.HandleFunc("/{id}/website", a.handleUpdateWebsiteTracking).Methods("PUT")
	projects.HandleFunc("/{id}/visitors", a.handleGetProjectVisitors).Methods("GET")

	admin := api.PathPrefix("/admin").Subrouter()
	admin.Use(a.authMiddleware)
	admin.Use(a.adminMiddleware)
	admin.HandleFunc("/me", a.handleAdminMe).Methods("GET")
	admin.HandleFunc("/overview", a.handleAdminOverview).Methods("GET")
	admin.HandleFunc("/users", a.handleAdminUsers).Methods("GET")
	admin.HandleFunc("/users/{id}", a.handleAdminUser).Methods("GET")
	admin.HandleFunc("/users/{id}/status", a.handleAdminUserStatus).Methods("PATCH")
	admin.HandleFunc("/projects", a.handleAdminProjects).Methods("GET")
	admin.HandleFunc("/projects/{id}", a.handleAdminProject).Methods("GET")
	admin.HandleFunc("/projects/{id}/status", a.handleAdminProjectStatus).Methods("PATCH")
	admin.HandleFunc("/projects/{id}/capabilities", a.handleAdminProjectCapabilities).Methods("PATCH")

	r.HandleFunc("/svg/{projectSlug}/counter/{name}.svg", a.handleCounterSVG).Methods("GET")
	r.HandleFunc("/svg/{projectSlug}/badge/{name}.svg", a.handleBadgeSVG).Methods("GET")
	r.HandleFunc("/sdk.js", a.handleWebsiteSDK).Methods("GET")
	r.HandleFunc("/admin", a.handleAdminSPA).Methods("GET")
	r.HandleFunc("/admin/{path:.*}", a.handleAdminSPA).Methods("GET")

	r.PathPrefix("/").HandlerFunc(a.handleSPA).Methods("GET")

	return r
}
