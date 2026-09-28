-- migrations/025_add_daily_statistics_ip.sql
ALTER TABLE daily_statistics
ADD COLUMN IF NOT EXISTS ip BIGINT NOT NULL DEFAULT 0,
ADD COLUMN IF NOT EXISTS hourly JSONB NOT NULL DEFAULT '{}'::jsonb;

COMMENT ON COLUMN daily_statistics.ip IS 'Unique IP count for the day';
COMMENT ON COLUMN daily_statistics.hourly IS 'Hourly distribution: {pv: {"00": 10, ...}, uv: {...}, ip: {...}}';
