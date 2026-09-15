package api

import (
	"net/http"
	"time"
)

func (a *App) handleGetProjectDiagnostics(w http.ResponseWriter, r *http.Request) {
	id, ok := a.authorizeProject(w, r)
	if !ok {
		return
	}
	project, err := a.projectRepo.GetByID(r.Context(), id)
	if err != nil || project == nil {
		a.jsonError(w, "Failed to get diagnostics", http.StatusInternalServerError)
		return
	}
	report, err := a.diagnostics.GetReport(r.Context(), id, project.WebsiteTrackingEnabled, project.WebsiteDomains, time.Now())
	if err != nil {
		a.jsonError(w, "Failed to get diagnostics", http.StatusInternalServerError)
		return
	}
	a.jsonSuccess(w, report)
}

func (a *App) handleClearProjectDiagnostics(w http.ResponseWriter, r *http.Request) {
	id, ok := a.authorizeProject(w, r)
	if !ok {
		return
	}
	if err := a.diagnostics.Clear(r.Context(), id); err != nil {
		a.jsonError(w, "Failed to clear diagnostics", http.StatusInternalServerError)
		return
	}
	a.jsonSuccess(w, map[string]bool{"cleared": true})
}
