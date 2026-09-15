package metrics

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/svgstat/svgstat/internal/analytics"
)

type liveStatsReader interface {
	GetTodayStats(ctx context.Context, projectID string) (*analytics.DailyStats, error)
	GetInstallationTimes(ctx context.Context, projectID string) (*analytics.InstallationTimes, error)
	GetRealtimeStats(ctx context.Context, projectID string, now time.Time) (*analytics.RealtimeStats, error)
}

type Service struct {
	pool *pgxpool.Pool
	live liveStatsReader
}

type TrendPoint struct {
	Date     string `json:"date"`
	PV       int64  `json:"pv"`
	UV       int64  `json:"uv"`
	Requests int64  `json:"requests"`
	Bots     int64  `json:"bots"`
}

type Trend struct {
	ProjectID string       `json:"projectId"`
	Days      int          `json:"days"`
	StartDate string       `json:"startDate"`
	EndDate   string       `json:"endDate"`
	Points    []TrendPoint `json:"points"`
	Totals    TrendPoint   `json:"totals"`
}

type InstallationStatus struct {
	ProjectID   string     `json:"projectId"`
	Status      string     `json:"status"`
	FirstSeenAt *time.Time `json:"firstSeenAt,omitempty"`
	LastSeenAt  *time.Time `json:"lastSeenAt,omitempty"`
}

type PeriodTotals struct {
	PV       int64 `json:"pv"`
	UV       int64 `json:"uv"`
	Requests int64 `json:"requests"`
	Bots     int64 `json:"bots"`
}

type PeriodChanges struct {
	PV       *float64 `json:"pv"`
	UV       *float64 `json:"uv"`
	Requests *float64 `json:"requests"`
	Bots     *float64 `json:"bots"`
}

type Analysis struct {
	ProjectID  string                      `json:"projectId"`
	Days       int                         `json:"days"`
	StartDate  string                      `json:"startDate"`
	EndDate    string                      `json:"endDate"`
	Current    PeriodTotals                `json:"current"`
	Previous   PeriodTotals                `json:"previous"`
	Changes    PeriodChanges               `json:"changes"`
	Breakdowns map[string]map[string]int64 `json:"breakdowns"`
}

type EventSummary struct {
	Name      string                         `json:"name"`
	Count     int64                          `json:"count"`
	Visitors  int64                          `json:"visitors"`
	Rate      float64                        `json:"rate"`
	Sources   map[string]int64               `json:"sources"`
	Mediums   map[string]int64               `json:"mediums"`
	Campaigns map[string]int64               `json:"campaigns"`
	Values    map[string]float64             `json:"values"`
	Trend     []EventTrendPoint              `json:"trend"`
	Segments  map[string][]ConversionSegment `json:"segments"`
	Details   map[string][]EventDetail       `json:"details"`
}

type EventDetail struct {
	Value    string `json:"value"`
	Visitors int64  `json:"visitors"`
}

type EventTrendPoint struct {
	Date     string  `json:"date"`
	Count    int64   `json:"count"`
	Visitors int64   `json:"visitors"`
	Rate     float64 `json:"rate"`
}
type ConversionSegment struct {
	Value      string  `json:"value"`
	Audience   int64   `json:"audience"`
	Converters int64   `json:"converters"`
	Rate       float64 `json:"rate"`
}

type EventReport struct {
	ProjectID string         `json:"projectId"`
	Days      int            `json:"days"`
	StartDate string         `json:"startDate"`
	EndDate   string         `json:"endDate"`
	Visitors  int64          `json:"visitors"`
	Events    []EventSummary `json:"events"`
	Quality   WebQuality     `json:"quality"`
}

type WebQuality struct {
	Status       string                   `json:"status"`
	Vitals       []WebVital               `json:"vitals"`
	JavaScript   QualityErrorSummary      `json:"javascript"`
	Resources    QualityErrorSummary      `json:"resources"`
	ErrorDetails map[string][]EventDetail `json:"errorDetails"`
}

type WebVital struct {
	Name         string        `json:"name"`
	Average      float64       `json:"average"`
	Samples      int64         `json:"samples"`
	Visitors     int64         `json:"visitors"`
	Rating       string        `json:"rating"`
	Distribution []EventDetail `json:"distribution"`
}

type QualityErrorSummary struct {
	Count    int64 `json:"count"`
	Visitors int64 `json:"visitors"`
}

type FunnelStep struct {
	EventName      string  `json:"eventName"`
	Visitors       int64   `json:"visitors"`
	ConversionRate float64 `json:"conversionRate"`
	DropOffRate    float64 `json:"dropOffRate"`
}

type FunnelReport struct {
	ProjectID string                     `json:"projectId"`
	FunnelID  string                     `json:"funnelId"`
	Days      int                        `json:"days"`
	StartDate string                     `json:"startDate"`
	EndDate   string                     `json:"endDate"`
	Steps     []FunnelStep               `json:"steps"`
	Segments  map[string][]FunnelSegment `json:"segments"`
}

type FunnelSegment struct {
	Value          string  `json:"value"`
	Steps          []int64 `json:"steps"`
	ConversionRate float64 `json:"conversionRate"`
}

func New(pool *pgxpool.Pool, live liveStatsReader) *Service {
	return &Service{pool: pool, live: live}
}

func (s *Service) GetTrend(ctx context.Context, projectID string, days int, now time.Time) (*Trend, error) {
	if days != 7 && days != 30 && days != 90 {
		return nil, fmt.Errorf("unsupported trend range: %d", days)
	}
	end := utcDay(now)
	start := end.AddDate(0, 0, -(days - 1))

	rows, err := s.pool.Query(ctx, `
		SELECT date, pv, uv, requests, bots
		FROM daily_statistics
		WHERE project_id = $1 AND date BETWEEN $2 AND $3
		ORDER BY date ASC
	`, projectID, start, end)
	if err != nil {
		return nil, fmt.Errorf("failed to query historical statistics: %w", err)
	}
	defer rows.Close()

	stored := make(map[string]TrendPoint, days)
	for rows.Next() {
		var date time.Time
		var point TrendPoint
		if err := rows.Scan(&date, &point.PV, &point.UV, &point.Requests, &point.Bots); err != nil {
			return nil, fmt.Errorf("failed to scan historical statistics: %w", err)
		}
		point.Date = date.UTC().Format("2006-01-02")
		stored[point.Date] = point
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to read historical statistics: %w", err)
	}

	if s.live != nil {
		if today, err := s.live.GetTodayStats(ctx, projectID); err == nil && today != nil {
			date := end.Format("2006-01-02")
			stored[date] = maxPoint(stored[date], TrendPoint{
				Date: date, PV: today.PV, UV: today.UV, Requests: today.Requests, Bots: today.Bots,
			})
		}
	}

	return buildTrend(projectID, days, start, end, stored), nil
}

func (s *Service) GetInstallationStatus(ctx context.Context, projectID string) (*InstallationStatus, error) {
	var firstSeenAt *time.Time
	var lastSeenAt *time.Time
	if s.live != nil {
		if times, err := s.live.GetInstallationTimes(ctx, projectID); err == nil && times != nil {
			firstSeenAt = times.FirstSeenAt
			lastSeenAt = times.LastSeenAt
		}
	}

	var storedFirst *time.Time
	var storedLast *time.Time
	if err := s.pool.QueryRow(ctx, `
		SELECT MIN(created_at), MAX(updated_at)
		FROM daily_statistics
		WHERE project_id = $1 AND (requests > 0 OR pv > 0)
	`, projectID).Scan(&storedFirst, &storedLast); err != nil {
		return nil, fmt.Errorf("failed to query installation status: %w", err)
	}
	firstSeenAt = earlierTime(firstSeenAt, storedFirst)
	lastSeenAt = laterTime(lastSeenAt, storedLast)

	status := "pending"
	if firstSeenAt != nil || lastSeenAt != nil {
		status = "installed"
	}
	return &InstallationStatus{ProjectID: projectID, Status: status, FirstSeenAt: firstSeenAt, LastSeenAt: lastSeenAt}, nil
}

func (s *Service) GetRealtimeStats(ctx context.Context, projectID string, now time.Time) (*analytics.RealtimeStats, error) {
	if s.live == nil {
		return &analytics.RealtimeStats{ProjectID: projectID}, nil
	}
	return s.live.GetRealtimeStats(ctx, projectID, now)
}

func (s *Service) GetAnalysis(ctx context.Context, projectID string, days int, now time.Time) (*Analysis, error) {
	if days != 7 && days != 30 && days != 90 {
		return nil, fmt.Errorf("unsupported analysis range: %d", days)
	}
	end := utcDay(now)
	start := end.AddDate(0, 0, -(days - 1))
	previousEnd := start.AddDate(0, 0, -1)
	previousStart := previousEnd.AddDate(0, 0, -(days - 1))
	result := &Analysis{
		ProjectID:  projectID,
		Days:       days,
		StartDate:  start.Format("2006-01-02"),
		EndDate:    end.Format("2006-01-02"),
		Breakdowns: emptyBreakdowns(),
	}
	rows, err := s.pool.Query(ctx, `
		SELECT date, pv, uv, requests, bots, paths, referrers, countries, devices, browsers, sources, mediums, campaigns
		FROM daily_statistics
		WHERE project_id = $1 AND date BETWEEN $2 AND $3
	`, projectID, previousStart, end)
	if err != nil {
		return nil, fmt.Errorf("failed to query analysis: %w", err)
	}
	defer rows.Close()
	var todayStored *analytics.DailyStats
	for rows.Next() {
		var date time.Time
		var stats analytics.DailyStats
		if err := rows.Scan(&date, &stats.PV, &stats.UV, &stats.Requests, &stats.Bots,
			&stats.Paths, &stats.Referrers, &stats.Countries, &stats.Devices, &stats.Browsers,
			&stats.Sources, &stats.Mediums, &stats.Campaigns); err != nil {
			return nil, fmt.Errorf("failed to scan analysis: %w", err)
		}
		day := utcDay(date)
		if day.Equal(end) {
			todayStored = &stats
			continue
		}
		if !day.Before(start) {
			addPeriodStats(&result.Current, result.Breakdowns, &stats)
		} else {
			addPeriodTotals(&result.Previous, &stats)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to read analysis: %w", err)
	}
	if s.live != nil {
		if live, err := s.live.GetTodayStats(ctx, projectID); err == nil && live != nil {
			live = maxDailyStats(todayStored, live)
			addPeriodStats(&result.Current, result.Breakdowns, live)
		} else if todayStored != nil {
			addPeriodStats(&result.Current, result.Breakdowns, todayStored)
		}
	} else if todayStored != nil {
		addPeriodStats(&result.Current, result.Breakdowns, todayStored)
	}
	trimBreakdowns(result.Breakdowns, 20)
	result.Changes = periodChanges(result.Current, result.Previous)
	return result, nil
}

func (s *Service) GetEventReport(ctx context.Context, projectID string, days int, now time.Time) (*EventReport, error) {
	if days != 7 && days != 30 && days != 90 {
		return nil, fmt.Errorf("unsupported event range: %d", days)
	}
	end := utcDay(now)
	start := end.AddDate(0, 0, -(days - 1))
	report := &EventReport{ProjectID: projectID, Days: days, StartDate: start.Format("2006-01-02"), EndDate: end.Format("2006-01-02"), Events: []EventSummary{}}
	stored := make(map[string]*analytics.DailyStats, days)
	rows, err := s.pool.Query(ctx, `SELECT date,uv,events,event_visitors,event_sources,event_mediums,event_campaigns,event_values,audience_segments,event_segments FROM daily_statistics WHERE project_id=$1 AND date BETWEEN $2 AND $3`, projectID, start, end)
	if err != nil {
		return nil, fmt.Errorf("failed to query event analysis: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var date time.Time
		stats := emptyEventStats()
		if err := rows.Scan(&date, &stats.UV, &stats.Events, &stats.EventVisitors, &stats.EventSources, &stats.EventMediums, &stats.EventCampaigns, &stats.EventValues, &stats.AudienceSegments, &stats.EventSegments); err != nil {
			return nil, fmt.Errorf("failed to scan event analysis: %w", err)
		}
		stored[utcDay(date).Format("2006-01-02")] = stats
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	todayKey := end.Format("2006-01-02")
	if s.live != nil {
		if live, err := s.live.GetTodayStats(ctx, projectID); err == nil && live != nil {
			stored[todayKey] = maxEventStats(stored[todayKey], live)
		}
	}
	aggregated := emptyEventStats()
	for _, stats := range stored {
		addEventStats(aggregated, stats)
	}
	report.Visitors = aggregated.UV
	report.Quality = buildWebQuality(aggregated)
	for name, count := range aggregated.Events {
		if isQualityEvent(name) {
			continue
		}
		rate := float64(0)
		if report.Visitors > 0 {
			rate = float64(aggregated.EventVisitors[name]) / float64(report.Visitors) * 100
		}
		summary := EventSummary{Name: name, Count: count, Visitors: aggregated.EventVisitors[name], Rate: rate, Sources: aggregated.EventSources[name], Mediums: aggregated.EventMediums[name], Campaigns: aggregated.EventCampaigns[name], Values: aggregated.EventValues[name], Trend: make([]EventTrendPoint, 0, days), Segments: buildConversionSegments(aggregated, name), Details: buildEventDetails(aggregated, name)}
		for date := start; !date.After(end); date = date.AddDate(0, 0, 1) {
			key := date.Format("2006-01-02")
			daily := stored[key]
			point := EventTrendPoint{Date: key}
			if daily != nil {
				point.Count = daily.Events[name]
				point.Visitors = daily.EventVisitors[name]
				if daily.UV > 0 {
					point.Rate = float64(point.Visitors) / float64(daily.UV) * 100
				}
			}
			summary.Trend = append(summary.Trend, point)
		}
		report.Events = append(report.Events, summary)
	}
	sort.Slice(report.Events, func(i, j int) bool {
		if report.Events[i].Count == report.Events[j].Count {
			return report.Events[i].Name < report.Events[j].Name
		}
		return report.Events[i].Count > report.Events[j].Count
	})
	return report, nil
}

func (s *Service) GetFunnelReport(ctx context.Context, projectID, funnelID string, stepNames []string, days int, now time.Time) (*FunnelReport, error) {
	if days != 7 && days != 30 && days != 90 {
		return nil, fmt.Errorf("unsupported funnel range: %d", days)
	}
	end := utcDay(now)
	start := end.AddDate(0, 0, -(days - 1))
	counts := make([]int64, len(stepNames))
	segmentCounts := map[string]map[string][]int64{}
	rows, err := s.pool.Query(ctx, `SELECT date,funnel_steps,funnel_segments FROM daily_statistics WHERE project_id=$1 AND date BETWEEN $2 AND $3`, projectID, start, end)
	if err != nil {
		return nil, fmt.Errorf("failed to query funnel analysis: %w", err)
	}
	defer rows.Close()
	var todayStored []int64
	var todayStoredSegments map[string]map[string][]int64
	for rows.Next() {
		var date time.Time
		var stored map[string][]int64
		var storedSegments map[string]map[string]map[string][]int64
		if err := rows.Scan(&date, &stored, &storedSegments); err != nil {
			return nil, err
		}
		if utcDay(date).Equal(end) {
			todayStored = stored[funnelID]
			todayStoredSegments = storedSegments[funnelID]
		} else {
			addStepCounts(counts, stored[funnelID])
			addFunnelSegmentCounts(segmentCounts, storedSegments[funnelID], len(stepNames))
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	today := todayStored
	todaySegments := todayStoredSegments
	if s.live != nil {
		if live, err := s.live.GetTodayStats(ctx, projectID); err == nil && live != nil {
			today = maxStepCounts(todayStored, live.FunnelSteps[funnelID])
			todaySegments = maxFunnelSegmentCounts(todayStoredSegments, live.FunnelSegments[funnelID], len(stepNames))
		}
	}
	addStepCounts(counts, today)
	addFunnelSegmentCounts(segmentCounts, todaySegments, len(stepNames))
	normalizeFunnelCounts(counts)
	for _, values := range segmentCounts {
		for _, steps := range values {
			normalizeFunnelCounts(steps)
		}
	}
	report := &FunnelReport{ProjectID: projectID, FunnelID: funnelID, Days: days, StartDate: start.Format("2006-01-02"), EndDate: end.Format("2006-01-02"), Steps: make([]FunnelStep, len(stepNames)), Segments: buildFunnelSegments(segmentCounts)}
	for index, name := range stepNames {
		conversion, dropOff := float64(0), float64(0)
		if counts[0] > 0 {
			conversion = float64(counts[index]) / float64(counts[0]) * 100
		}
		if index > 0 && counts[index-1] > 0 {
			dropOff = (1 - float64(counts[index])/float64(counts[index-1])) * 100
		}
		report.Steps[index] = FunnelStep{EventName: name, Visitors: counts[index], ConversionRate: conversion, DropOffRate: dropOff}
	}
	return report, nil
}

func emptyEventStats() *analytics.DailyStats {
	return &analytics.DailyStats{Events: map[string]int64{}, EventVisitors: map[string]int64{}, EventSources: map[string]map[string]int64{}, EventMediums: map[string]map[string]int64{}, EventCampaigns: map[string]map[string]int64{}, EventValues: map[string]map[string]float64{}, AudienceSegments: map[string]map[string]int64{}, EventSegments: map[string]map[string]map[string]int64{}}
}

func addEventStats(target, stats *analytics.DailyStats) {
	if stats == nil {
		return
	}
	target.UV += stats.UV
	for key, value := range stats.Events {
		target.Events[key] += value
	}
	for key, value := range stats.EventVisitors {
		target.EventVisitors[key] += value
	}
	addNestedCounts(target.EventSources, stats.EventSources)
	addNestedCounts(target.EventMediums, stats.EventMediums)
	addNestedCounts(target.EventCampaigns, stats.EventCampaigns)
	addNestedCounts(target.AudienceSegments, stats.AudienceSegments)
	addEventSegmentCounts(target.EventSegments, stats.EventSegments)
	for event, values := range stats.EventValues {
		if target.EventValues[event] == nil {
			target.EventValues[event] = map[string]float64{}
		}
		for currency, value := range values {
			target.EventValues[event][currency] += value
		}
	}
}

func addNestedCounts(target, source map[string]map[string]int64) {
	for event, values := range source {
		if target[event] == nil {
			target[event] = map[string]int64{}
		}
		for key, value := range values {
			target[event][key] += value
		}
	}
}

func maxEventStats(stored, live *analytics.DailyStats) *analytics.DailyStats {
	if stored == nil {
		return live
	}
	if live == nil {
		return stored
	}
	result := emptyEventStats()
	result.UV = maxInt64(stored.UV, live.UV)
	result.Events = maxMap(stored.Events, live.Events)
	result.EventVisitors = maxMap(stored.EventVisitors, live.EventVisitors)
	result.EventSources = maxNestedCounts(stored.EventSources, live.EventSources)
	result.EventMediums = maxNestedCounts(stored.EventMediums, live.EventMediums)
	result.EventCampaigns = maxNestedCounts(stored.EventCampaigns, live.EventCampaigns)
	result.EventValues = maxNestedValues(stored.EventValues, live.EventValues)
	result.AudienceSegments = maxNestedCounts(stored.AudienceSegments, live.AudienceSegments)
	result.EventSegments = maxEventSegmentCounts(stored.EventSegments, live.EventSegments)
	return result
}

func addEventSegmentCounts(target, source map[string]map[string]map[string]int64) {
	for event, dimensions := range source {
		if target[event] == nil {
			target[event] = map[string]map[string]int64{}
		}
		addNestedCounts(target[event], dimensions)
	}
}
func maxEventSegmentCounts(left, right map[string]map[string]map[string]int64) map[string]map[string]map[string]int64 {
	result := map[string]map[string]map[string]int64{}
	addEventSegmentCounts(result, left)
	for event, dimensions := range right {
		if result[event] == nil {
			result[event] = map[string]map[string]int64{}
		}
		result[event] = maxNestedCounts(result[event], dimensions)
	}
	return result
}

func buildConversionSegments(stats *analytics.DailyStats, eventName string) map[string][]ConversionSegment {
	result := map[string][]ConversionSegment{}
	for dimension, converters := range stats.EventSegments[eventName] {
		for value, count := range converters {
			audience := stats.AudienceSegments[dimension][value]
			rate := float64(0)
			if audience > 0 {
				rate = float64(count) / float64(audience) * 100
			}
			result[dimension] = append(result[dimension], ConversionSegment{Value: value, Audience: audience, Converters: count, Rate: rate})
		}
		sort.Slice(result[dimension], func(i, j int) bool {
			if result[dimension][i].Converters == result[dimension][j].Converters {
				return result[dimension][i].Value < result[dimension][j].Value
			}
			return result[dimension][i].Converters > result[dimension][j].Converters
		})
		if len(result[dimension]) > 10 {
			result[dimension] = result[dimension][:10]
		}
	}
	return result
}

func buildEventDetails(stats *analytics.DailyStats, eventName string) map[string][]EventDetail {
	result := map[string][]EventDetail{}
	for dimension, values := range stats.EventSegments[eventName] {
		if !strings.HasPrefix(dimension, "property_") {
			continue
		}
		key := strings.TrimPrefix(dimension, "property_")
		for value, visitors := range values {
			result[key] = append(result[key], EventDetail{Value: value, Visitors: visitors})
		}
		sort.Slice(result[key], func(left, right int) bool {
			if result[key][left].Visitors == result[key][right].Visitors {
				return result[key][left].Value < result[key][right].Value
			}
			return result[key][left].Visitors > result[key][right].Visitors
		})
		if len(result[key]) > 10 {
			result[key] = result[key][:10]
		}
	}
	return result
}

func isQualityEvent(name string) bool {
	return name == "web_vital_lcp" || name == "web_vital_inp" || name == "web_vital_cls" || name == "js_error" || name == "resource_error"
}

func buildWebQuality(stats *analytics.DailyStats) WebQuality {
	quality := WebQuality{Status: "no_data", Vitals: []WebVital{}, ErrorDetails: map[string][]EventDetail{}}
	quality.JavaScript = QualityErrorSummary{Count: stats.Events["js_error"], Visitors: stats.EventVisitors["js_error"]}
	quality.Resources = QualityErrorSummary{Count: stats.Events["resource_error"], Visitors: stats.EventVisitors["resource_error"]}
	for key, values := range buildEventDetails(stats, "js_error") {
		quality.ErrorDetails["js_"+key] = values
	}
	for key, values := range buildEventDetails(stats, "resource_error") {
		quality.ErrorDetails["resource_"+key] = values
	}
	statusRank := map[string]int{"no_data": 0, "good": 1, "needs_improvement": 2, "poor": 3}
	for _, name := range []string{"lcp", "inp", "cls"} {
		eventName := "web_vital_" + name
		samples := stats.Events[eventName]
		if samples == 0 {
			continue
		}
		average := stats.EventValues[eventName]["XXX"] / float64(samples)
		rating := webVitalRating(name, average)
		vital := WebVital{Name: name, Average: average, Samples: samples, Visitors: stats.EventVisitors[eventName], Rating: rating, Distribution: buildEventDetails(stats, eventName)["rating"]}
		quality.Vitals = append(quality.Vitals, vital)
		if statusRank[rating] > statusRank[quality.Status] {
			quality.Status = rating
		}
	}
	if quality.JavaScript.Count > 0 || quality.Resources.Count > 0 {
		if statusRank["needs_improvement"] > statusRank[quality.Status] {
			quality.Status = "needs_improvement"
		}
	}
	return quality
}

func webVitalRating(name string, value float64) string {
	good, poor := 0.1, 0.25
	if name == "lcp" {
		good, poor = 2500, 4000
	} else if name == "inp" {
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

func addFunnelSegmentCounts(target, source map[string]map[string][]int64, stepCount int) {
	for dimension, values := range source {
		if target[dimension] == nil {
			target[dimension] = map[string][]int64{}
		}
		for value, counts := range values {
			if len(target[dimension][value]) != stepCount {
				target[dimension][value] = make([]int64, stepCount)
			}
			addStepCounts(target[dimension][value], counts)
		}
	}
}
func maxFunnelSegmentCounts(left, right map[string]map[string][]int64, stepCount int) map[string]map[string][]int64 {
	result := map[string]map[string][]int64{}
	addFunnelSegmentCounts(result, left, stepCount)
	for dimension, values := range right {
		if result[dimension] == nil {
			result[dimension] = map[string][]int64{}
		}
		for value, counts := range values {
			result[dimension][value] = maxStepCounts(result[dimension][value], counts)
		}
	}
	return result
}
func buildFunnelSegments(counts map[string]map[string][]int64) map[string][]FunnelSegment {
	result := map[string][]FunnelSegment{}
	for dimension, values := range counts {
		for value, steps := range values {
			rate := float64(0)
			if len(steps) > 0 && steps[0] > 0 {
				rate = float64(steps[len(steps)-1]) / float64(steps[0]) * 100
			}
			result[dimension] = append(result[dimension], FunnelSegment{Value: value, Steps: steps, ConversionRate: rate})
		}
		sort.Slice(result[dimension], func(i, j int) bool {
			if result[dimension][i].ConversionRate == result[dimension][j].ConversionRate {
				return result[dimension][i].Value < result[dimension][j].Value
			}
			return result[dimension][i].ConversionRate > result[dimension][j].ConversionRate
		})
		if len(result[dimension]) > 10 {
			result[dimension] = result[dimension][:10]
		}
	}
	return result
}

func normalizeFunnelCounts(counts []int64) {
	for index := 1; index < len(counts); index++ {
		if counts[index] > counts[index-1] {
			counts[index] = counts[index-1]
		}
	}
}

func maxNestedCounts(left, right map[string]map[string]int64) map[string]map[string]int64 {
	result := map[string]map[string]int64{}
	addNestedCounts(result, left)
	for event, values := range right {
		if result[event] == nil {
			result[event] = map[string]int64{}
		}
		for key, value := range values {
			if value > result[event][key] {
				result[event][key] = value
			}
		}
	}
	return result
}

func maxNestedValues(left, right map[string]map[string]float64) map[string]map[string]float64 {
	result := map[string]map[string]float64{}
	for event, values := range left {
		result[event] = map[string]float64{}
		for key, value := range values {
			result[event][key] = value
		}
	}
	for event, values := range right {
		if result[event] == nil {
			result[event] = map[string]float64{}
		}
		for key, value := range values {
			if value > result[event][key] {
				result[event][key] = value
			}
		}
	}
	return result
}

func addStepCounts(target, source []int64) {
	for index := range target {
		if index < len(source) {
			target[index] += source[index]
		}
	}
}
func maxStepCounts(left, right []int64) []int64 {
	size := len(left)
	if len(right) > size {
		size = len(right)
	}
	result := make([]int64, size)
	for index := range result {
		if index < len(left) {
			result[index] = left[index]
		}
		if index < len(right) && right[index] > result[index] {
			result[index] = right[index]
		}
	}
	return result
}

func emptyBreakdowns() map[string]map[string]int64 {
	return map[string]map[string]int64{
		"paths": {}, "referrers": {}, "countries": {}, "devices": {}, "browsers": {},
		"sources": {}, "mediums": {}, "campaigns": {},
	}
}

func addPeriodTotals(target *PeriodTotals, stats *analytics.DailyStats) {
	target.PV += stats.PV
	target.UV += stats.UV
	target.Requests += stats.Requests
	target.Bots += stats.Bots
}

func addPeriodStats(target *PeriodTotals, breakdowns map[string]map[string]int64, stats *analytics.DailyStats) {
	addPeriodTotals(target, stats)
	for name, values := range map[string]map[string]int64{
		"paths": stats.Paths, "referrers": stats.Referrers, "countries": stats.Countries,
		"devices": stats.Devices, "browsers": stats.Browsers, "sources": stats.Sources,
		"mediums": stats.Mediums, "campaigns": stats.Campaigns,
	} {
		for key, value := range values {
			breakdowns[name][key] += value
		}
	}
}

func periodChanges(current, previous PeriodTotals) PeriodChanges {
	return PeriodChanges{
		PV:       percentageChange(current.PV, previous.PV),
		UV:       percentageChange(current.UV, previous.UV),
		Requests: percentageChange(current.Requests, previous.Requests),
		Bots:     percentageChange(current.Bots, previous.Bots),
	}
}

func percentageChange(current, previous int64) *float64 {
	if previous == 0 {
		return nil
	}
	value := float64(current-previous) / float64(previous) * 100
	return &value
}

func trimBreakdowns(breakdowns map[string]map[string]int64, limit int) {
	for name, values := range breakdowns {
		type entry struct {
			key   string
			value int64
		}
		entries := make([]entry, 0, len(values))
		for key, value := range values {
			entries = append(entries, entry{key: key, value: value})
		}
		sort.Slice(entries, func(left, right int) bool {
			if entries[left].value == entries[right].value {
				return entries[left].key < entries[right].key
			}
			return entries[left].value > entries[right].value
		})
		trimmed := make(map[string]int64, min(limit, len(entries)))
		for index := 0; index < len(entries) && index < limit; index++ {
			trimmed[entries[index].key] = entries[index].value
		}
		breakdowns[name] = trimmed
	}
}

func maxDailyStats(stored, live *analytics.DailyStats) *analytics.DailyStats {
	if stored == nil {
		return live
	}
	if live == nil {
		return stored
	}
	merged := *live
	merged.PV = maxInt64(stored.PV, live.PV)
	merged.UV = maxInt64(stored.UV, live.UV)
	merged.Requests = maxInt64(stored.Requests, live.Requests)
	merged.Bots = maxInt64(stored.Bots, live.Bots)
	merged.Paths = maxMap(stored.Paths, live.Paths)
	merged.Referrers = maxMap(stored.Referrers, live.Referrers)
	merged.Countries = maxMap(stored.Countries, live.Countries)
	merged.Devices = maxMap(stored.Devices, live.Devices)
	merged.Browsers = maxMap(stored.Browsers, live.Browsers)
	merged.Sources = maxMap(stored.Sources, live.Sources)
	merged.Mediums = maxMap(stored.Mediums, live.Mediums)
	merged.Campaigns = maxMap(stored.Campaigns, live.Campaigns)
	return &merged
}

func maxMap(left, right map[string]int64) map[string]int64 {
	result := make(map[string]int64, len(left)+len(right))
	for key, value := range left {
		result[key] = value
	}
	for key, value := range right {
		if value > result[key] {
			result[key] = value
		}
	}
	return result
}

func earlierTime(left, right *time.Time) *time.Time {
	if left == nil {
		return right
	}
	if right == nil || left.Before(*right) {
		return left
	}
	return right
}

func laterTime(left, right *time.Time) *time.Time {
	if left == nil {
		return right
	}
	if right == nil || left.After(*right) {
		return left
	}
	return right
}

func buildTrend(projectID string, days int, start, end time.Time, stored map[string]TrendPoint) *Trend {
	trend := &Trend{
		ProjectID: projectID,
		Days:      days,
		StartDate: start.Format("2006-01-02"),
		EndDate:   end.Format("2006-01-02"),
		Points:    make([]TrendPoint, 0, days),
	}
	for date := start; !date.After(end); date = date.AddDate(0, 0, 1) {
		key := date.Format("2006-01-02")
		point := stored[key]
		point.Date = key
		trend.Points = append(trend.Points, point)
		trend.Totals.PV += point.PV
		trend.Totals.UV += point.UV
		trend.Totals.Requests += point.Requests
		trend.Totals.Bots += point.Bots
	}
	return trend
}

func maxPoint(left, right TrendPoint) TrendPoint {
	return TrendPoint{
		Date:     right.Date,
		PV:       maxInt64(left.PV, right.PV),
		UV:       maxInt64(left.UV, right.UV),
		Requests: maxInt64(left.Requests, right.Requests),
		Bots:     maxInt64(left.Bots, right.Bots),
	}
}

func maxInt64(left, right int64) int64 {
	if left > right {
		return left
	}
	return right
}

func utcDay(value time.Time) time.Time {
	utc := value.UTC()
	return time.Date(utc.Year(), utc.Month(), utc.Day(), 0, 0, 0, 0, time.UTC)
}
