-- migrations/026_add_daily_statistics_ai_visits.sql
ALTER TABLE daily_statistics
ADD COLUMN IF NOT EXISTS ai_visits BIGINT NOT NULL DEFAULT 0;

COMMENT ON COLUMN daily_statistics.ai_visits IS 'Daily AI crawler and search agent visit count';
