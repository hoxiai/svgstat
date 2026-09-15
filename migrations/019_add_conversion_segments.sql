ALTER TABLE daily_statistics
    ADD COLUMN IF NOT EXISTS audience_segments JSONB NOT NULL DEFAULT '{}',
    ADD COLUMN IF NOT EXISTS event_segments JSONB NOT NULL DEFAULT '{}',
    ADD COLUMN IF NOT EXISTS funnel_segments JSONB NOT NULL DEFAULT '{}';

COMMENT ON COLUMN daily_statistics.audience_segments IS 'Daily anonymous unique visitors grouped by analysis dimension';
COMMENT ON COLUMN daily_statistics.event_segments IS 'Daily anonymous unique converters grouped by event and analysis dimension';
COMMENT ON COLUMN daily_statistics.funnel_segments IS 'Daily anonymous unique funnel step visitors grouped by funnel and analysis dimension';
