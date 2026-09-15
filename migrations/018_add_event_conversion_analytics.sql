ALTER TABLE daily_statistics
    ADD COLUMN IF NOT EXISTS events JSONB NOT NULL DEFAULT '{}',
    ADD COLUMN IF NOT EXISTS event_visitors JSONB NOT NULL DEFAULT '{}',
    ADD COLUMN IF NOT EXISTS event_sources JSONB NOT NULL DEFAULT '{}',
    ADD COLUMN IF NOT EXISTS event_mediums JSONB NOT NULL DEFAULT '{}',
    ADD COLUMN IF NOT EXISTS event_campaigns JSONB NOT NULL DEFAULT '{}',
    ADD COLUMN IF NOT EXISTS event_values JSONB NOT NULL DEFAULT '{}',
    ADD COLUMN IF NOT EXISTS funnel_steps JSONB NOT NULL DEFAULT '{}';

CREATE TABLE IF NOT EXISTS conversion_goals (
    id TEXT PRIMARY KEY,
    project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    event_name TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT unique_conversion_goal_name UNIQUE (project_id, name)
);

CREATE INDEX IF NOT EXISTS idx_conversion_goals_project_id ON conversion_goals(project_id);

CREATE TABLE IF NOT EXISTS funnels (
    id TEXT PRIMARY KEY,
    project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    steps JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT unique_funnel_name UNIQUE (project_id, name),
    CONSTRAINT chk_funnel_steps_array CHECK (jsonb_typeof(steps) = 'array')
);

CREATE INDEX IF NOT EXISTS idx_funnels_project_id ON funnels(project_id);

COMMENT ON COLUMN daily_statistics.events IS 'Custom event occurrence counts by normalized event name';
COMMENT ON COLUMN daily_statistics.event_visitors IS 'Daily anonymous unique converters by event name';
COMMENT ON COLUMN daily_statistics.event_sources IS 'Daily anonymous unique converters grouped by event and attributed source';
COMMENT ON COLUMN daily_statistics.event_mediums IS 'Daily anonymous unique converters grouped by event and attributed medium';
COMMENT ON COLUMN daily_statistics.event_campaigns IS 'Daily anonymous unique converters grouped by event and attributed campaign';
COMMENT ON COLUMN daily_statistics.event_values IS 'Custom event numeric value totals grouped by event and currency';
COMMENT ON COLUMN daily_statistics.funnel_steps IS 'Daily anonymous unique visitors reaching each configured funnel step';
