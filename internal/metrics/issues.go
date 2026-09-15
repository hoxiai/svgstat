package metrics

import (
	"context"
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/svgstat/svgstat/internal/analytics"
)

type IssueGoal struct {
	Name      string
	EventName string
}

type Issue struct {
	Code          string   `json:"code"`
	Severity      string   `json:"severity"`
	Category      string   `json:"category"`
	Subject       string   `json:"subject,omitempty"`
	Current       float64  `json:"current"`
	Previous      float64  `json:"previous"`
	ChangePercent *float64 `json:"changePercent,omitempty"`
	Unit          string   `json:"unit"`
}

type IssueReport struct {
	ProjectID   string  `json:"projectId"`
	Days        int     `json:"days"`
	Status      string  `json:"status"`
	EvaluatedAt string  `json:"evaluatedAt"`
	Issues      []Issue `json:"issues"`
}

type issuePeriod struct {
	PV            int64
	UV            int64
	Events        map[string]int64
	EventVisitors map[string]int64
	EventValues   map[string]map[string]float64
}

func newIssuePeriod() issuePeriod {
	return issuePeriod{Events: map[string]int64{}, EventVisitors: map[string]int64{}, EventValues: map[string]map[string]float64{}}
}

func (s *Service) GetIssueReport(ctx context.Context, projectID string, days int, now time.Time, goals []IssueGoal) (*IssueReport, error) {
	if days != 7 && days != 30 && days != 90 {
		return nil, fmt.Errorf("unsupported issue range: %d", days)
	}
	end := utcDay(now)
	start := end.AddDate(0, 0, -(days - 1))
	previousEnd := start.AddDate(0, 0, -1)
	previousStart := previousEnd.AddDate(0, 0, -(days - 1))
	stored := map[string]*analytics.DailyStats{}
	rows, err := s.pool.Query(ctx, `SELECT date,pv,uv,events,event_visitors,event_values FROM daily_statistics WHERE project_id=$1 AND date BETWEEN $2 AND $3`, projectID, previousStart, end)
	if err != nil {
		return nil, fmt.Errorf("failed to query issue analysis: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var date time.Time
		stats := emptyEventStats()
		if err := rows.Scan(&date, &stats.PV, &stats.UV, &stats.Events, &stats.EventVisitors, &stats.EventValues); err != nil {
			return nil, fmt.Errorf("failed to scan issue analysis: %w", err)
		}
		stored[utcDay(date).Format("2006-01-02")] = stats
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to read issue analysis: %w", err)
	}
	if s.live != nil {
		if live, err := s.live.GetTodayStats(ctx, projectID); err == nil && live != nil {
			today := end.Format("2006-01-02")
			stored[today] = maxIssueStats(stored[today], live)
		}
	}
	current, previous := newIssuePeriod(), newIssuePeriod()
	var lastActive *time.Time
	for key, stats := range stored {
		date, _ := time.Parse("2006-01-02", key)
		if stats.PV > 0 && (lastActive == nil || date.After(*lastActive)) {
			value := date
			lastActive = &value
		}
		if !date.Before(start) {
			addIssueStats(&current, stats)
		} else {
			addIssueStats(&previous, stats)
		}
	}
	return buildIssueReport(projectID, days, now, current, previous, lastActive, goals), nil
}

func maxIssueStats(stored, live *analytics.DailyStats) *analytics.DailyStats {
	merged := maxEventStats(stored, live)
	if stored == nil || live == nil {
		return merged
	}
	merged.PV = maxInt64(stored.PV, live.PV)
	return merged
}

func addIssueStats(period *issuePeriod, stats *analytics.DailyStats) {
	period.PV += stats.PV
	period.UV += stats.UV
	for key, value := range stats.Events {
		period.Events[key] += value
	}
	for key, value := range stats.EventVisitors {
		period.EventVisitors[key] += value
	}
	for event, values := range stats.EventValues {
		if period.EventValues[event] == nil {
			period.EventValues[event] = map[string]float64{}
		}
		for unit, value := range values {
			period.EventValues[event][unit] += value
		}
	}
}

func buildIssueReport(projectID string, days int, now time.Time, current, previous issuePeriod, lastActive *time.Time, goals []IssueGoal) *IssueReport {
	report := &IssueReport{ProjectID: projectID, Days: days, Status: "healthy", EvaluatedAt: now.UTC().Format(time.RFC3339), Issues: []Issue{}}
	if previous.PV >= 20 && (lastActive == nil || utcDay(now).Sub(*lastActive) >= 48*time.Hour) {
		report.Issues = append(report.Issues, newIssue("collection_stopped", "critical", "collection", "", float64(current.PV), float64(previous.PV), "pageviews"))
	} else if previous.PV >= 100 && float64(current.PV) <= float64(previous.PV)*0.7 {
		report.Issues = append(report.Issues, newIssue("traffic_drop", "warning", "traffic", "", float64(current.PV), float64(previous.PV), "pageviews"))
	}
	for _, goal := range goals {
		previousConverters := previous.EventVisitors[goal.EventName]
		if previous.UV < 50 || previousConverters < 5 {
			continue
		}
		previousRate := percentage(previousConverters, previous.UV)
		currentRate := percentage(current.EventVisitors[goal.EventName], current.UV)
		if currentRate > previousRate-2 || currentRate > previousRate*0.7 {
			continue
		}
		severity := "warning"
		if currentRate <= previousRate*0.5 && previousRate-currentRate >= 5 {
			severity = "critical"
		}
		report.Issues = append(report.Issues, newIssue("conversion_drop", severity, "conversion", goal.Name, currentRate, previousRate, "percent"))
	}
	for _, metric := range []string{"lcp", "inp", "cls"} {
		currentAverage, currentSamples := issueVital(current, metric)
		previousAverage, previousSamples := issueVital(previous, metric)
		if currentSamples < 20 {
			continue
		}
		rating := webVitalRating(metric, currentAverage)
		if rating == "good" {
			continue
		}
		code, severity := "vital_needs_improvement", "warning"
		if rating == "poor" {
			code, severity = "vital_poor", "critical"
		}
		previousValue := float64(0)
		if previousSamples >= 20 {
			previousValue = previousAverage
		}
		unit := "milliseconds"
		if metric == "cls" {
			unit = "score"
		}
		report.Issues = append(report.Issues, newIssue(code, severity, "performance", metric, currentAverage, previousValue, unit))
	}
	appendErrorIssue := func(eventName, category string) {
		currentRate := percentage(current.EventVisitors[eventName], current.UV)
		previousRate := percentage(previous.EventVisitors[eventName], previous.UV)
		if current.EventVisitors[eventName] < 5 || currentRate < 5 {
			return
		}
		code := category + "_errors_high"
		if previousRate > 0 && currentRate >= previousRate*2 && currentRate-previousRate >= 5 {
			code = category + "_errors_spike"
		}
		severity := "warning"
		if currentRate >= 20 {
			severity = "critical"
		}
		report.Issues = append(report.Issues, newIssue(code, severity, "errors", "", currentRate, previousRate, "percent"))
	}
	appendErrorIssue("js_error", "javascript")
	appendErrorIssue("resource_error", "resource")
	if len(report.Issues) == 0 && current.PV+previous.PV < 40 && qualitySamples(current)+qualitySamples(previous) < 20 {
		report.Status = "insufficient_data"
	}
	sort.SliceStable(report.Issues, func(left, right int) bool {
		return issueSeverityRank(report.Issues[left].Severity) > issueSeverityRank(report.Issues[right].Severity)
	})
	if len(report.Issues) > 10 {
		report.Issues = report.Issues[:10]
	}
	for _, issue := range report.Issues {
		if issue.Severity == "critical" {
			report.Status = "critical"
			break
		}
		report.Status = "warning"
	}
	return report
}

func newIssue(code, severity, category, subject string, current, previous float64, unit string) Issue {
	issue := Issue{Code: code, Severity: severity, Category: category, Subject: subject, Current: current, Previous: previous, Unit: unit}
	if previous != 0 {
		change := (current - previous) / math.Abs(previous) * 100
		issue.ChangePercent = &change
	}
	return issue
}

func percentage(value, total int64) float64 {
	if total <= 0 {
		return 0
	}
	return float64(value) / float64(total) * 100
}

func issueVital(period issuePeriod, metric string) (float64, int64) {
	eventName := "web_vital_" + metric
	samples := period.Events[eventName]
	if samples == 0 {
		return 0, 0
	}
	return period.EventValues[eventName]["XXX"] / float64(samples), samples
}

func qualitySamples(period issuePeriod) int64 {
	return period.Events["web_vital_lcp"] + period.Events["web_vital_inp"] + period.Events["web_vital_cls"]
}

func issueSeverityRank(severity string) int {
	if severity == "critical" {
		return 2
	}
	return 1
}
