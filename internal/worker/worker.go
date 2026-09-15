package worker

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"
	"github.com/svgstat/svgstat/internal/analytics"
	"github.com/svgstat/svgstat/internal/project"
)

// Worker persists Redis analytics aggregates into PostgreSQL daily_statistics.
// Redis stays the real-time store; PostgreSQL becomes the historical record.
type Worker struct {
	analytics   *analytics.Analytics
	projectRepo project.Repository
	pool        *pgxpool.Pool
	interval    time.Duration
	lockID      int64
	observer    interface{ ObserveWorkerFlush(time.Duration, error) }
}

func New(analytics *analytics.Analytics, projectRepo project.Repository, pool *pgxpool.Pool, interval time.Duration) *Worker {
	if interval <= 0 {
		interval = 15 * time.Minute
	}
	return &Worker{
		analytics:   analytics,
		projectRepo: projectRepo,
		pool:        pool,
		interval:    interval,
	}
}

func (w *Worker) SetLeadership(lockID int64, observer interface{ ObserveWorkerFlush(time.Duration, error) }) {
	w.lockID = lockID
	w.observer = observer
}

// Start flushes once immediately, then on a ticker, until ctx is cancelled.
// It never blocks HTTP request handling and must run in its own goroutine.
func (w *Worker) Start(ctx context.Context) {
	log.Info().Dur("interval", w.interval).Msg("Analytics flush worker started")
	if err := w.flushIfLeader(ctx); err != nil {
		log.Error().Err(err).Msg("Initial analytics flush failed")
	}

	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			// Best-effort final flush so the last minutes of traffic are not
			// lost on deploys. Uses a fresh context because ctx is cancelled.
			flushCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			if err := w.flushIfLeader(flushCtx); err != nil {
				log.Error().Err(err).Msg("Final analytics flush failed")
			}
			log.Info().Msg("Analytics flush worker stopped")
			return
		case <-ticker.C:
			if err := w.flushIfLeader(ctx); err != nil {
				log.Error().Err(err).Msg("Analytics flush failed")
			}
		}
	}
}

func (w *Worker) flushIfLeader(ctx context.Context) error {
	if w.lockID == 0 {
		return w.observedFlush(ctx)
	}
	connection, err := w.pool.Acquire(ctx)
	if err != nil {
		return err
	}
	defer connection.Release()
	var acquired bool
	if err := connection.QueryRow(ctx, `SELECT pg_try_advisory_lock($1)`, w.lockID).Scan(&acquired); err != nil {
		return err
	}
	if !acquired {
		return nil
	}
	defer func() { _, _ = connection.Exec(context.Background(), `SELECT pg_advisory_unlock($1)`, w.lockID) }()
	return w.observedFlush(ctx)
}

func (w *Worker) observedFlush(ctx context.Context) error {
	started := time.Now()
	err := w.FlushOnce(ctx)
	if w.observer != nil {
		w.observer.ObserveWorkerFlush(time.Since(started), err)
	}
	return err
}

// FlushOnce persists today and yesterday for every project. Today is flushed
// repeatedly so history stays fresh; yesterday is flushed so keys may expire.
func (w *Worker) FlushOnce(ctx context.Context) error {
	projects, err := w.projectRepo.ListAll(ctx)
	if err != nil {
		return fmt.Errorf("failed to list projects: %w", err)
	}

	dates := flushDates(time.Now())
	flushed := 0

	for _, p := range projects {
		for _, date := range dates {
			stats, err := w.analytics.GetStats(ctx, p.ID, date)
			if err != nil {
				log.Error().Err(err).Str("project_id", p.ID).Str("date", date).Msg("Failed to read Redis stats")
				continue
			}
			if !hasTraffic(stats) {
				continue
			}
			if err := w.upsertDailyStats(ctx, stats); err != nil {
				log.Error().Err(err).Str("project_id", p.ID).Str("date", date).Msg("Failed to persist stats")
				continue
			}
			flushed++
		}
	}

	log.Debug().Int("rows", flushed).Msg("Analytics flush completed")
	return nil
}

func hasTraffic(stats *analytics.DailyStats) bool {
	return stats != nil && (stats.PV > 0 || stats.Requests > 0 || stats.Bots > 0 || len(stats.Events) > 0)
}

// flushDates returns the UTC dates a flush covers: today (partial) and
// yesterday (final).
func flushDates(now time.Time) []string {
	utc := now.UTC()
	return []string{
		utc.Format("2006-01-02"),
		utc.AddDate(0, 0, -1).Format("2006-01-02"),
	}
}

func (w *Worker) upsertDailyStats(ctx context.Context, stats *analytics.DailyStats) error {
	args, err := dailyStatsArgs(stats)
	if err != nil {
		return err
	}

	query := `
		INSERT INTO daily_statistics (
			id, project_id, date, pv, uv, requests, bots,
			referrers, countries, regions, cities, devices, browsers, paths, ips,
			sources, mediums, campaigns, events, event_visitors, event_sources,
			event_mediums, event_campaigns, event_values, funnel_steps,
			audience_segments, event_segments, funnel_segments,
			sessions, bounces, session_duration_seconds, session_pageviews,
			entrances, exits, page_flows, session_segments
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22, $23, $24, $25, $26, $27, $28, $29, $30, $31, $32, $33, $34, $35, $36)
		ON CONFLICT (project_id, date) DO UPDATE SET
			pv = EXCLUDED.pv,
			uv = EXCLUDED.uv,
			requests = EXCLUDED.requests,
			bots = EXCLUDED.bots,
			referrers = EXCLUDED.referrers,
			countries = EXCLUDED.countries,
			regions = EXCLUDED.regions,
			cities = EXCLUDED.cities,
			devices = EXCLUDED.devices,
			browsers = EXCLUDED.browsers,
			paths = EXCLUDED.paths,
			ips = EXCLUDED.ips,
			sources = EXCLUDED.sources,
			mediums = EXCLUDED.mediums,
			campaigns = EXCLUDED.campaigns,
			events = EXCLUDED.events,
			event_visitors = EXCLUDED.event_visitors,
			event_sources = EXCLUDED.event_sources,
			event_mediums = EXCLUDED.event_mediums,
			event_campaigns = EXCLUDED.event_campaigns,
			event_values = EXCLUDED.event_values,
			funnel_steps = EXCLUDED.funnel_steps,
			audience_segments = EXCLUDED.audience_segments,
			event_segments = EXCLUDED.event_segments,
			funnel_segments = EXCLUDED.funnel_segments,
			sessions = CASE WHEN EXCLUDED.sessions > daily_statistics.sessions OR (EXCLUDED.sessions = daily_statistics.sessions AND EXCLUDED.session_pageviews >= daily_statistics.session_pageviews) THEN EXCLUDED.sessions ELSE daily_statistics.sessions END,
			bounces = CASE WHEN EXCLUDED.sessions > daily_statistics.sessions OR (EXCLUDED.sessions = daily_statistics.sessions AND EXCLUDED.session_pageviews >= daily_statistics.session_pageviews) THEN EXCLUDED.bounces ELSE daily_statistics.bounces END,
			session_duration_seconds = CASE WHEN EXCLUDED.sessions > daily_statistics.sessions OR (EXCLUDED.sessions = daily_statistics.sessions AND EXCLUDED.session_pageviews >= daily_statistics.session_pageviews) THEN EXCLUDED.session_duration_seconds ELSE daily_statistics.session_duration_seconds END,
			session_pageviews = CASE WHEN EXCLUDED.sessions > daily_statistics.sessions OR (EXCLUDED.sessions = daily_statistics.sessions AND EXCLUDED.session_pageviews >= daily_statistics.session_pageviews) THEN EXCLUDED.session_pageviews ELSE daily_statistics.session_pageviews END,
			entrances = CASE WHEN EXCLUDED.sessions > daily_statistics.sessions OR (EXCLUDED.sessions = daily_statistics.sessions AND EXCLUDED.session_pageviews >= daily_statistics.session_pageviews) THEN EXCLUDED.entrances ELSE daily_statistics.entrances END,
			exits = CASE WHEN EXCLUDED.sessions > daily_statistics.sessions OR (EXCLUDED.sessions = daily_statistics.sessions AND EXCLUDED.session_pageviews >= daily_statistics.session_pageviews) THEN EXCLUDED.exits ELSE daily_statistics.exits END,
			page_flows = CASE WHEN EXCLUDED.sessions > daily_statistics.sessions OR (EXCLUDED.sessions = daily_statistics.sessions AND EXCLUDED.session_pageviews >= daily_statistics.session_pageviews) THEN EXCLUDED.page_flows ELSE daily_statistics.page_flows END,
			session_segments = CASE WHEN EXCLUDED.sessions > daily_statistics.sessions OR (EXCLUDED.sessions = daily_statistics.sessions AND EXCLUDED.session_pageviews >= daily_statistics.session_pageviews) THEN EXCLUDED.session_segments ELSE daily_statistics.session_segments END,
			updated_at = NOW()
	`

	_, err = w.pool.Exec(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("failed to upsert daily statistics: %w", err)
	}
	return nil
}

// dailyStatsArgs builds the upsert arguments in INSERT column order.
func dailyStatsArgs(stats *analytics.DailyStats) ([]interface{}, error) {
	if stats.Events == nil {
		stats.Events = map[string]int64{}
	}
	if stats.EventVisitors == nil {
		stats.EventVisitors = map[string]int64{}
	}
	if stats.EventSources == nil {
		stats.EventSources = map[string]map[string]int64{}
	}
	if stats.EventMediums == nil {
		stats.EventMediums = map[string]map[string]int64{}
	}
	if stats.EventCampaigns == nil {
		stats.EventCampaigns = map[string]map[string]int64{}
	}
	if stats.EventValues == nil {
		stats.EventValues = map[string]map[string]float64{}
	}
	if stats.FunnelSteps == nil {
		stats.FunnelSteps = map[string][]int64{}
	}
	if stats.AudienceSegments == nil {
		stats.AudienceSegments = map[string]map[string]int64{}
	}
	if stats.EventSegments == nil {
		stats.EventSegments = map[string]map[string]map[string]int64{}
	}
	if stats.FunnelSegments == nil {
		stats.FunnelSegments = map[string]map[string]map[string][]int64{}
	}
	if stats.Entrances == nil {
		stats.Entrances = map[string]int64{}
	}
	if stats.Exits == nil {
		stats.Exits = map[string]int64{}
	}
	if stats.PageFlows == nil {
		stats.PageFlows = map[string]int64{}
	}
	if stats.SessionSegments == nil {
		stats.SessionSegments = map[string]map[string]analytics.SessionQualityCounts{}
	}
	referrers, err := json.Marshal(stats.Referrers)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal referrers: %w", err)
	}
	countries, err := json.Marshal(stats.Countries)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal countries: %w", err)
	}
	regions, err := json.Marshal(stats.Regions)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal regions: %w", err)
	}
	cities, err := json.Marshal(stats.Cities)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal cities: %w", err)
	}
	devices, err := json.Marshal(stats.Devices)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal devices: %w", err)
	}
	browsers, err := json.Marshal(stats.Browsers)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal browsers: %w", err)
	}
	paths, err := json.Marshal(stats.Paths)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal paths: %w", err)
	}
	ips, err := json.Marshal(stats.IPs)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal ips: %w", err)
	}
	sources, err := json.Marshal(stats.Sources)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal sources: %w", err)
	}
	mediums, err := json.Marshal(stats.Mediums)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal mediums: %w", err)
	}
	campaigns, err := json.Marshal(stats.Campaigns)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal campaigns: %w", err)
	}
	events, err := json.Marshal(stats.Events)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal events: %w", err)
	}
	eventVisitors, err := json.Marshal(stats.EventVisitors)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal event visitors: %w", err)
	}
	eventSources, err := json.Marshal(stats.EventSources)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal event sources: %w", err)
	}
	eventMediums, err := json.Marshal(stats.EventMediums)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal event mediums: %w", err)
	}
	eventCampaigns, err := json.Marshal(stats.EventCampaigns)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal event campaigns: %w", err)
	}
	eventValues, err := json.Marshal(stats.EventValues)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal event values: %w", err)
	}
	funnelSteps, err := json.Marshal(stats.FunnelSteps)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal funnel steps: %w", err)
	}
	audienceSegments, err := json.Marshal(stats.AudienceSegments)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal audience segments: %w", err)
	}
	eventSegments, err := json.Marshal(stats.EventSegments)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal event segments: %w", err)
	}
	funnelSegments, err := json.Marshal(stats.FunnelSegments)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal funnel segments: %w", err)
	}
	entrances, err := json.Marshal(stats.Entrances)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal entrances: %w", err)
	}
	exits, err := json.Marshal(stats.Exits)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal exits: %w", err)
	}
	pageFlows, err := json.Marshal(stats.PageFlows)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal page flows: %w", err)
	}
	sessionSegments, err := json.Marshal(stats.SessionSegments)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal session segments: %w", err)
	}

	return []interface{}{
		newID(), stats.ProjectID, stats.Date,
		stats.PV, stats.UV, stats.Requests, stats.Bots,
		referrers, countries, regions, cities, devices, browsers, paths, ips,
		sources, mediums, campaigns, events, eventVisitors, eventSources,
		eventMediums, eventCampaigns, eventValues, funnelSteps,
		audienceSegments, eventSegments, funnelSegments,
		stats.Sessions, stats.Bounces, stats.SessionDurationSeconds, stats.SessionPageviews,
		entrances, exits, pageFlows, sessionSegments,
	}, nil
}

func newID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
