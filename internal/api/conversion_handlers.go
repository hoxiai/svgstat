package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/mux"
	"github.com/hoxiai/svgstat/internal/auth"
	"github.com/hoxiai/svgstat/internal/conversion"
	"github.com/hoxiai/svgstat/internal/metrics"
	"github.com/rs/zerolog/log"
)

type goalInput struct {
	Name      string `json:"name"`
	EventName string `json:"eventName"`
}
type funnelInput struct {
	Name  string   `json:"name"`
	Steps []string `json:"steps"`
}

func (a *App) authorizeProject(w http.ResponseWriter, r *http.Request) (string, bool) {
	user := r.Context().Value("user").(*auth.User)
	id := mux.Vars(r)["id"]
	project, err := a.projectRepo.GetByIDAndUser(r.Context(), id, user.ID)
	if err != nil {
		a.jsonError(w, "Failed to get project", http.StatusInternalServerError)
		return "", false
	}
	if project == nil {
		a.jsonError(w, "Project not found", http.StatusNotFound)
		return "", false
	}
	return id, true
}

func analysisDays(r *http.Request) (int, bool) {
	days := 30
	if raw := r.URL.Query().Get("days"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || (parsed != 7 && parsed != 30 && parsed != 90) {
			return 0, false
		}
		days = parsed
	}
	return days, true
}

func (a *App) handleGetProjectEvents(w http.ResponseWriter, r *http.Request) {
	id, ok := a.authorizeProject(w, r)
	if !ok {
		return
	}
	days, ok := analysisDays(r)
	if !ok {
		a.jsonError(w, "Days must be one of 7, 30, or 90", http.StatusBadRequest)
		return
	}
	report, err := a.metrics.GetEventReport(r.Context(), id, days, time.Now())
	if err != nil {
		log.Error().Err(err).Msg("Failed to get event analysis")
		a.jsonError(w, "Failed to get event analysis", http.StatusInternalServerError)
		return
	}
	a.jsonSuccess(w, report)
}

func (a *App) handleGetProjectIssues(w http.ResponseWriter, r *http.Request) {
	id, ok := a.authorizeProject(w, r)
	if !ok {
		return
	}
	days, ok := analysisDays(r)
	if !ok {
		a.jsonError(w, "Days must be one of 7, 30, or 90", http.StatusBadRequest)
		return
	}
	goals, err := a.conversion.ListGoals(r.Context(), id)
	if err != nil {
		log.Error().Err(err).Msg("Failed to list goals for issue analysis")
		a.jsonError(w, "Failed to analyze project issues", http.StatusInternalServerError)
		return
	}
	issueGoals := make([]metrics.IssueGoal, len(goals))
	for index, goal := range goals {
		issueGoals[index] = metrics.IssueGoal{Name: goal.Name, EventName: goal.EventName}
	}
	report, err := a.metrics.GetIssueReport(r.Context(), id, days, time.Now(), issueGoals)
	if err != nil {
		log.Error().Err(err).Msg("Failed to analyze project issues")
		a.jsonError(w, "Failed to analyze project issues", http.StatusInternalServerError)
		return
	}
	a.jsonSuccess(w, report)
}

func (a *App) handleListGoals(w http.ResponseWriter, r *http.Request) {
	id, ok := a.authorizeProject(w, r)
	if !ok {
		return
	}
	items, err := a.conversion.ListGoals(r.Context(), id)
	if err != nil {
		a.jsonError(w, "Failed to list goals", http.StatusInternalServerError)
		return
	}
	a.jsonSuccess(w, items)
}

func (a *App) handleCreateGoal(w http.ResponseWriter, r *http.Request) {
	id, ok := a.authorizeProject(w, r)
	if !ok {
		return
	}
	var input goalInput
	if err := decodeJSONBody(w, r, &input); err != nil {
		a.jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}
	item, err := a.conversion.CreateGoal(r.Context(), id, input.Name, input.EventName)
	if err != nil {
		a.writeConversionError(w, err, "create goal")
		return
	}
	a.jsonSuccess(w, item)
}

func (a *App) handleUpdateGoal(w http.ResponseWriter, r *http.Request) {
	id, ok := a.authorizeProject(w, r)
	if !ok {
		return
	}
	var input goalInput
	if err := decodeJSONBody(w, r, &input); err != nil {
		a.jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}
	item, err := a.conversion.UpdateGoal(r.Context(), id, mux.Vars(r)["goalId"], input.Name, input.EventName)
	if err != nil {
		a.writeConversionError(w, err, "update goal")
		return
	}
	a.jsonSuccess(w, item)
}

func (a *App) handleDeleteGoal(w http.ResponseWriter, r *http.Request) {
	id, ok := a.authorizeProject(w, r)
	if !ok {
		return
	}
	if err := a.conversion.DeleteGoal(r.Context(), id, mux.Vars(r)["goalId"]); err != nil {
		a.jsonError(w, "Failed to delete goal", http.StatusInternalServerError)
		return
	}
	a.jsonSuccess(w, map[string]bool{"deleted": true})
}

func (a *App) handleListFunnels(w http.ResponseWriter, r *http.Request) {
	id, ok := a.authorizeProject(w, r)
	if !ok {
		return
	}
	items, err := a.conversion.ListFunnels(r.Context(), id)
	if err != nil {
		a.jsonError(w, "Failed to list funnels", http.StatusInternalServerError)
		return
	}
	a.jsonSuccess(w, items)
}

func (a *App) handleCreateFunnel(w http.ResponseWriter, r *http.Request) {
	id, ok := a.authorizeProject(w, r)
	if !ok {
		return
	}
	var input funnelInput
	if err := decodeJSONBody(w, r, &input); err != nil {
		a.jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}
	item, err := a.conversion.CreateFunnel(r.Context(), id, input.Name, input.Steps)
	if err != nil {
		a.writeConversionError(w, err, "create funnel")
		return
	}
	a.jsonSuccess(w, item)
}

func (a *App) handleUpdateFunnel(w http.ResponseWriter, r *http.Request) {
	id, ok := a.authorizeProject(w, r)
	if !ok {
		return
	}
	var input funnelInput
	if err := decodeJSONBody(w, r, &input); err != nil {
		a.jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}
	item, err := a.conversion.UpdateFunnel(r.Context(), id, mux.Vars(r)["funnelId"], input.Name, input.Steps)
	if err != nil {
		a.writeConversionError(w, err, "update funnel")
		return
	}
	a.jsonSuccess(w, item)
}

func (a *App) handleDeleteFunnel(w http.ResponseWriter, r *http.Request) {
	id, ok := a.authorizeProject(w, r)
	if !ok {
		return
	}
	if err := a.conversion.DeleteFunnel(r.Context(), id, mux.Vars(r)["funnelId"]); err != nil {
		a.jsonError(w, "Failed to delete funnel", http.StatusInternalServerError)
		return
	}
	a.jsonSuccess(w, map[string]bool{"deleted": true})
}

func (a *App) handleGetFunnelAnalysis(w http.ResponseWriter, r *http.Request) {
	id, ok := a.authorizeProject(w, r)
	if !ok {
		return
	}
	days, ok := analysisDays(r)
	if !ok {
		a.jsonError(w, "Days must be one of 7, 30, or 90", http.StatusBadRequest)
		return
	}
	funnel, err := a.conversion.GetFunnel(r.Context(), id, mux.Vars(r)["funnelId"])
	if err != nil {
		a.jsonError(w, "Funnel not found", http.StatusNotFound)
		return
	}
	report, err := a.metrics.GetFunnelReport(r.Context(), id, funnel.ID, funnel.Steps, days, time.Now())
	if err != nil {
		a.jsonError(w, "Failed to get funnel analysis", http.StatusInternalServerError)
		return
	}
	a.jsonSuccess(w, report)
}

func decodeJSONBody(w http.ResponseWriter, r *http.Request, target interface{}) error {
	r.Body = http.MaxBytesReader(w, r.Body, 16<<10)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return &requestError{message: "Invalid request"}
	}
	return nil
}

func (a *App) writeConversionError(w http.ResponseWriter, err error, operation string) {
	var validation *conversion.ValidationError
	if errors.As(err, &validation) {
		a.jsonError(w, validation.Error(), http.StatusBadRequest)
		return
	}
	log.Error().Err(err).Str("operation", operation).Msg("Conversion operation failed")
	a.jsonError(w, "Conversion operation failed", http.StatusInternalServerError)
}

type requestError struct{ message string }

func (e *requestError) Error() string { return e.message }
