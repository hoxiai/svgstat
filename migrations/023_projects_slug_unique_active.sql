-- Soft-deleted projects keep their slug; only active projects need unique slugs.
ALTER TABLE projects DROP CONSTRAINT IF EXISTS projects_slug_key;

CREATE UNIQUE INDEX IF NOT EXISTS idx_projects_slug_active
ON projects(slug)
WHERE deleted_at IS NULL;
