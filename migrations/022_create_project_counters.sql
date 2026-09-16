CREATE TABLE IF NOT EXISTS project_counters (
    project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    counter_name TEXT NOT NULL,
    value BIGINT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (project_id, counter_name)
);

CREATE INDEX IF NOT EXISTS idx_project_counters_project_id ON project_counters(project_id);

COMMENT ON TABLE project_counters IS 'Persistent storage for cumulative project badge counters';
COMMENT ON COLUMN project_counters.value IS 'Latest cumulative count persisted from Redis';
