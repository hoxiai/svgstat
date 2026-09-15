ALTER TABLE daily_statistics
    ADD COLUMN IF NOT EXISTS sources JSONB NOT NULL DEFAULT '{}',
    ADD COLUMN IF NOT EXISTS mediums JSONB NOT NULL DEFAULT '{}',
    ADD COLUMN IF NOT EXISTS campaigns JSONB NOT NULL DEFAULT '{}';

COMMENT ON COLUMN daily_statistics.sources IS 'UTM source or inferred referring domain counts';
COMMENT ON COLUMN daily_statistics.mediums IS 'UTM medium or inferred traffic medium counts';
COMMENT ON COLUMN daily_statistics.campaigns IS 'UTM campaign counts';
