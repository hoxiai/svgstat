ALTER TABLE daily_statistics
ADD COLUMN IF NOT EXISTS regions JSONB NOT NULL DEFAULT '{}',
ADD COLUMN IF NOT EXISTS cities JSONB NOT NULL DEFAULT '{}';

COMMENT ON COLUMN daily_statistics.regions IS 'Region-level visitor counts';
COMMENT ON COLUMN daily_statistics.cities IS 'City-level visitor counts';
