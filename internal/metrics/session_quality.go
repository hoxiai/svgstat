package metrics

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/hoxiai/svgstat/internal/analytics"
)

type SessionQualityTotals struct {
	Sessions               int64   `json:"sessions"`
	Bounces                int64   `json:"bounces"`
	Pageviews              int64   `json:"pageviews"`
	DurationSeconds        int64   `json:"durationSeconds"`
	BounceRate             float64 `json:"bounceRate"`
	AveragePages           float64 `json:"averagePages"`
	AverageDurationSeconds float64 `json:"averageDurationSeconds"`
}

type SessionQualityPage struct {
	Path  string `json:"path"`
	Count int64  `json:"count"`
}

type SessionQualityFlow struct {
	From  string `json:"from"`
	To    string `json:"to"`
	Count int64  `json:"count"`
}

type SessionQualitySegment struct {
	Value                  string  `json:"value"`
	Sessions               int64   `json:"sessions"`
	BounceRate             float64 `json:"bounceRate"`
	AveragePages           float64 `json:"averagePages"`
	AverageDurationSeconds float64 `json:"averageDurationSeconds"`
}

type SessionQualityReport struct {
	ProjectID string                             `json:"projectId"`
	Days      int                                `json:"days"`
	StartDate string                             `json:"startDate"`
	EndDate   string                             `json:"endDate"`
	Totals    SessionQualityTotals               `json:"totals"`
	Entrances []SessionQualityPage               `json:"entrances"`
	Exits     []SessionQualityPage               `json:"exits"`
	Flows     []SessionQualityFlow               `json:"flows"`
	Segments  map[string][]SessionQualitySegment `json:"segments"`
}

func (s *Service) GetSessionQuality(ctx context.Context, projectID string, days int, now time.Time) (*SessionQualityReport, error) {
	if days != 7 && days != 30 && days != 90 {
		return nil, fmt.Errorf("unsupported session quality range: %d", days)
	}
	end := utcDay(now)
	start := end.AddDate(0, 0, -(days - 1))
	stored := map[string]*analytics.DailyStats{}
	rows, err := s.pool.Query(ctx, `
		SELECT date, sessions, bounces, session_duration_seconds, session_pageviews,
		       entrances, exits, page_flows, session_segments
		FROM daily_statistics
		WHERE project_id=$1 AND date BETWEEN $2 AND $3
	`, projectID, start, end)
	if err != nil {
		return nil, fmt.Errorf("failed to query session quality: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var date time.Time
		stats := emptySessionQualityStats()
		if err := rows.Scan(&date, &stats.Sessions, &stats.Bounces, &stats.SessionDurationSeconds, &stats.SessionPageviews, &stats.Entrances, &stats.Exits, &stats.PageFlows, &stats.SessionSegments); err != nil {
			return nil, fmt.Errorf("failed to scan session quality: %w", err)
		}
		stored[utcDay(date).Format("2006-01-02")] = stats
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to read session quality: %w", err)
	}
	if s.live != nil {
		if live, err := s.live.GetTodayStats(ctx, projectID); err == nil && live != nil && live.Sessions > 0 {
			stored[end.Format("2006-01-02")] = latestSessionQualityStats(stored[end.Format("2006-01-02")], live)
		}
	}
	aggregated := emptySessionQualityStats()
	for _, stats := range stored {
		addSessionQualityStats(aggregated, stats)
	}
	return buildSessionQualityReport(projectID, days, start, end, aggregated), nil
}

func latestSessionQualityStats(stored, live *analytics.DailyStats) *analytics.DailyStats {
	if stored == nil {
		return live
	}
	if live == nil {
		return stored
	}
	if live.Sessions > stored.Sessions || (live.Sessions == stored.Sessions && live.SessionPageviews >= stored.SessionPageviews) {
		return live
	}
	return stored
}

func emptySessionQualityStats() *analytics.DailyStats {
	return &analytics.DailyStats{
		Entrances:       map[string]int64{},
		Exits:           map[string]int64{},
		PageFlows:       map[string]int64{},
		SessionSegments: map[string]map[string]analytics.SessionQualityCounts{},
	}
}

func addSessionQualityStats(target, source *analytics.DailyStats) {
	if source == nil {
		return
	}
	target.Sessions += source.Sessions
	target.Bounces += source.Bounces
	target.SessionDurationSeconds += source.SessionDurationSeconds
	target.SessionPageviews += source.SessionPageviews
	addMapCounts(target.Entrances, source.Entrances)
	addMapCounts(target.Exits, source.Exits)
	addMapCounts(target.PageFlows, source.PageFlows)
	for dimension, values := range source.SessionSegments {
		if target.SessionSegments[dimension] == nil {
			target.SessionSegments[dimension] = map[string]analytics.SessionQualityCounts{}
		}
		for value, counts := range values {
			current := target.SessionSegments[dimension][value]
			current.Sessions += counts.Sessions
			current.Bounces += counts.Bounces
			current.Pageviews += counts.Pageviews
			current.DurationSeconds += counts.DurationSeconds
			target.SessionSegments[dimension][value] = current
		}
	}
}

func addMapCounts(target, source map[string]int64) {
	for key, value := range source {
		target[key] += value
	}
}

func qualityTotals(sessions, bounces, pageviews, duration int64) SessionQualityTotals {
	result := SessionQualityTotals{Sessions: sessions, Bounces: bounces, Pageviews: pageviews, DurationSeconds: duration}
	if sessions > 0 {
		result.BounceRate = float64(bounces) / float64(sessions) * 100
		result.AveragePages = float64(pageviews) / float64(sessions)
		result.AverageDurationSeconds = float64(duration) / float64(sessions)
	}
	return result
}

func buildSessionQualityReport(projectID string, days int, start, end time.Time, stats *analytics.DailyStats) *SessionQualityReport {
	report := &SessionQualityReport{
		ProjectID: projectID,
		Days:      days,
		StartDate: start.Format("2006-01-02"),
		EndDate:   end.Format("2006-01-02"),
		Totals:    qualityTotals(stats.Sessions, stats.Bounces, stats.SessionPageviews, stats.SessionDurationSeconds),
		Entrances: topSessionPages(stats.Entrances, 10),
		Exits:     topSessionPages(stats.Exits, 10),
		Flows:     topSessionFlows(stats.PageFlows, 10),
		Segments:  map[string][]SessionQualitySegment{},
	}
	for dimension, values := range stats.SessionSegments {
		for value, counts := range values {
			totals := qualityTotals(counts.Sessions, counts.Bounces, counts.Pageviews, counts.DurationSeconds)
			report.Segments[dimension] = append(report.Segments[dimension], SessionQualitySegment{Value: value, Sessions: totals.Sessions, BounceRate: totals.BounceRate, AveragePages: totals.AveragePages, AverageDurationSeconds: totals.AverageDurationSeconds})
		}
		sort.Slice(report.Segments[dimension], func(left, right int) bool {
			if report.Segments[dimension][left].Sessions == report.Segments[dimension][right].Sessions {
				return report.Segments[dimension][left].Value < report.Segments[dimension][right].Value
			}
			return report.Segments[dimension][left].Sessions > report.Segments[dimension][right].Sessions
		})
		if len(report.Segments[dimension]) > 10 {
			report.Segments[dimension] = report.Segments[dimension][:10]
		}
	}
	return report
}

func topSessionPages(values map[string]int64, limit int) []SessionQualityPage {
	result := make([]SessionQualityPage, 0, len(values))
	for path, count := range values {
		if count > 0 {
			result = append(result, SessionQualityPage{Path: path, Count: count})
		}
	}
	sort.Slice(result, func(left, right int) bool {
		if result[left].Count == result[right].Count {
			return result[left].Path < result[right].Path
		}
		return result[left].Count > result[right].Count
	})
	if len(result) > limit {
		result = result[:limit]
	}
	return result
}

func topSessionFlows(values map[string]int64, limit int) []SessionQualityFlow {
	result := make([]SessionQualityFlow, 0, len(values))
	for field, count := range values {
		parts := strings.SplitN(field, "\x1f", 2)
		if count > 0 && len(parts) == 2 {
			result = append(result, SessionQualityFlow{From: parts[0], To: parts[1], Count: count})
		}
	}
	sort.Slice(result, func(left, right int) bool {
		if result[left].Count == result[right].Count {
			return result[left].From+result[left].To < result[right].From+result[right].To
		}
		return result[left].Count > result[right].Count
	})
	if len(result) > limit {
		result = result[:limit]
	}
	return result
}
