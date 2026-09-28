package metrics

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/hoxiai/svgstat/internal/analytics"
)

type TodayOverview struct {
	ProjectID           string                 `json:"projectId"`
	Date                string                 `json:"date"`
	PV                  int64                  `json:"pv"`
	UV                  int64                  `json:"uv"`
	IP                  int64                  `json:"ip"`
	AvgPageviewsPerUser float64                `json:"avgPageviewsPerUser"`
	YesterdayFull       PeriodTotals           `json:"yesterdayFull"`
	YesterdaySamePeriod PeriodTotals           `json:"yesterdaySamePeriod"`
	Changes             PeriodChanges          `json:"changes"`
	TodayHourly         map[string]HourlyPoint `json:"todayHourly"`
	YesterdayHourly     map[string]HourlyPoint `json:"yesterdayHourly"`
	CurrentHour         int                    `json:"currentHour"`
}

type HourlyPoint struct {
	PV int64 `json:"pv"`
	UV int64 `json:"uv"`
	IP int64 `json:"ip"`
}

func (s *Service) GetTodayOverview(ctx context.Context, projectID string, now time.Time) (*TodayOverview, error) {
	utc := now.UTC()
	date := utc.Format("2006-01-02")
	yesterdayDate := utc.AddDate(0, 0, -1).Format("2006-01-02")
	currentHour := utc.Hour()

	var todayStats *analytics.DailyStats
	if s.live != nil {
		live, err := s.live.GetTodayStats(ctx, projectID)
		if err != nil {
			return nil, fmt.Errorf("failed to get live today stats: %w", err)
		}
		todayStats = live
	}
	if todayStats == nil {
		todayStats = &analytics.DailyStats{
			ProjectID: projectID,
			Date:      date,
		}
	}

	var (
		yesterdayFull       PeriodTotals
		yesterdayHourlyJSON []byte
	)
	if s.pool != nil {
		var (
			yPV int64
			yUV int64
			yIP int64
		)
		err := s.pool.QueryRow(ctx, `
			SELECT pv, uv, ip, hourly
			FROM daily_statistics
			WHERE project_id = $1 AND date = $2
		`, projectID, yesterdayDate).Scan(&yPV, &yUV, &yIP, &yesterdayHourlyJSON)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("failed to query yesterday statistics: %w", err)
		}
		if err == nil {
			yesterdayFull.PV = yPV
			yesterdayFull.UV = yUV
			yesterdayFull.IP = yIP
		}
	}

	var yesterdayHourlyRaw map[string]map[string]int64
	if len(yesterdayHourlyJSON) > 0 {
		_ = json.Unmarshal(yesterdayHourlyJSON, &yesterdayHourlyRaw)
	}

	yesterdayHourly := make(map[string]HourlyPoint, 24)
	for h := 0; h < 24; h++ {
		key := fmt.Sprintf("%02d", h)
		var pt HourlyPoint
		if yesterdayHourlyRaw != nil {
			if pvMap, ok := yesterdayHourlyRaw["pv"]; ok {
				pt.PV = pvMap[key]
			}
			if uvMap, ok := yesterdayHourlyRaw["uv"]; ok {
				pt.UV = uvMap[key]
			}
			if ipMap, ok := yesterdayHourlyRaw["ip"]; ok {
				pt.IP = ipMap[key]
			}
		}
		yesterdayHourly[key] = pt
	}

	var yesterdaySamePeriod PeriodTotals
	for h := 0; h <= currentHour; h++ {
		key := fmt.Sprintf("%02d", h)
		pt := yesterdayHourly[key]
		yesterdaySamePeriod.PV += pt.PV
		yesterdaySamePeriod.UV += pt.UV
		yesterdaySamePeriod.IP += pt.IP
	}

	todayHourly := make(map[string]HourlyPoint, currentHour+1)
	for h := 0; h <= currentHour; h++ {
		key := fmt.Sprintf("%02d", h)
		var pt HourlyPoint
		if todayStats.Hourly != nil {
			if pvMap, ok := todayStats.Hourly["pv"]; ok {
				pt.PV = pvMap[key]
			}
			if uvMap, ok := todayStats.Hourly["uv"]; ok {
				pt.UV = uvMap[key]
			}
			if ipMap, ok := todayStats.Hourly["ip"]; ok {
				pt.IP = ipMap[key]
			}
		}
		todayHourly[key] = pt
	}

	todayTotals := PeriodTotals{
		PV: todayStats.PV,
		UV: todayStats.UV,
		IP: todayStats.IP,
	}
	changes := periodChanges(todayTotals, yesterdaySamePeriod)

	var avgPageviews float64
	if todayStats.UV > 0 {
		avgPageviews = float64(todayStats.PV) / float64(todayStats.UV)
	}

	return &TodayOverview{
		ProjectID:           projectID,
		Date:                date,
		PV:                  todayStats.PV,
		UV:                  todayStats.UV,
		IP:                  todayStats.IP,
		AvgPageviewsPerUser: avgPageviews,
		YesterdayFull:       yesterdayFull,
		YesterdaySamePeriod: yesterdaySamePeriod,
		Changes:             changes,
		TodayHourly:         todayHourly,
		YesterdayHourly:     yesterdayHourly,
		CurrentHour:         currentHour,
	}, nil
}
