ALTER TABLE daily_statistics
    ADD COLUMN IF NOT EXISTS sessions BIGINT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS bounces BIGINT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS session_duration_seconds BIGINT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS session_pageviews BIGINT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS entrances JSONB NOT NULL DEFAULT '{}',
    ADD COLUMN IF NOT EXISTS exits JSONB NOT NULL DEFAULT '{}',
    ADD COLUMN IF NOT EXISTS page_flows JSONB NOT NULL DEFAULT '{}',
    ADD COLUMN IF NOT EXISTS session_segments JSONB NOT NULL DEFAULT '{}';

COMMENT ON COLUMN daily_statistics.sessions IS 'Anonymous 30-minute website sessions';
COMMENT ON COLUMN daily_statistics.bounces IS 'Single-page website sessions';
COMMENT ON COLUMN daily_statistics.session_duration_seconds IS 'Engaged seconds measured between page views within a session';
COMMENT ON COLUMN daily_statistics.session_pageviews IS 'Page views attributed to website sessions';
COMMENT ON COLUMN daily_statistics.entrances IS 'Session entry page counts';
COMMENT ON COLUMN daily_statistics.exits IS 'Session latest page counts used as exit pages';
COMMENT ON COLUMN daily_statistics.page_flows IS 'Aggregated adjacent page transitions without visitor-level trajectories';
COMMENT ON COLUMN daily_statistics.session_segments IS 'Session quality aggregates grouped by first-touch channel and device dimensions';
