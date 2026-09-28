-- Per-visit attribution dimensions: search/ad keywords (utm_term or a search
-- referrer's query), medium-source channel pairs, and on-site search terms.
ALTER TABLE daily_statistics
ADD COLUMN IF NOT EXISTS terms JSONB NOT NULL DEFAULT '{}'::jsonb,
ADD COLUMN IF NOT EXISTS channels JSONB NOT NULL DEFAULT '{}'::jsonb,
ADD COLUMN IF NOT EXISTS site_searches JSONB NOT NULL DEFAULT '{}'::jsonb;

COMMENT ON COLUMN daily_statistics.sources IS 'Visits by traffic source (per pageview before migration 024)';
COMMENT ON COLUMN daily_statistics.terms IS 'Visits by keyword: utm_term, or the query of a search referrer';
COMMENT ON COLUMN daily_statistics.channels IS 'Visits by medium and source, keyed medium<US>source';
COMMENT ON COLUMN daily_statistics.site_searches IS 'On-site search terms, counted per search';
