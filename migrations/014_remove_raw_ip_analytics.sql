UPDATE daily_statistics
SET ips = '{}'::jsonb,
    updated_at = NOW()
WHERE ips <> '{}'::jsonb;

COMMENT ON COLUMN daily_statistics.ips IS 'Salted anonymous network identifiers; raw IP addresses are not stored';
