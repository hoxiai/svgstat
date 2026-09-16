package project

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"
)

type PostgresRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

func (r *PostgresRepository) GetByID(ctx context.Context, id string) (*Project, error) {
	query := `
		SELECT id, user_id, external_project_id, tenant_id, slug, name, description, status, visibility,
			public_token_hash, default_theme, default_locale, render_enabled, badge_enabled,
			widget_enabled, chart_enabled, website_tracking_enabled, website_domains,
			last_synced_at, deleted_at, created_at, updated_at
		FROM projects
		WHERE id = $1 AND deleted_at IS NULL
	`

	var p Project
	err := r.pool.QueryRow(ctx, query, id).Scan(
		&p.ID, &p.UserID, &p.ExternalProjectID, &p.TenantID, &p.Slug, &p.Name, &p.Description,
		&p.Status, &p.Visibility, &p.PublicTokenHash, &p.DefaultTheme, &p.DefaultLocale,
		&p.RenderEnabled, &p.BadgeEnabled, &p.WidgetEnabled, &p.ChartEnabled,
		&p.WebsiteTrackingEnabled, &p.WebsiteDomains,
		&p.LastSyncedAt, &p.DeletedAt, &p.CreatedAt, &p.UpdatedAt,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get project: %w", err)
	}

	return &p, nil
}

func (r *PostgresRepository) GetByIDAndUser(ctx context.Context, id, userID string) (*Project, error) {
	query := `
		SELECT id, user_id, external_project_id, tenant_id, slug, name, description, status, visibility,
			public_token_hash, default_theme, default_locale, render_enabled, badge_enabled,
			widget_enabled, chart_enabled, website_tracking_enabled, website_domains,
			last_synced_at, deleted_at, created_at, updated_at
		FROM projects
		WHERE id = $1 AND user_id = $2 AND deleted_at IS NULL
	`

	var p Project
	err := r.pool.QueryRow(ctx, query, id, userID).Scan(
		&p.ID, &p.UserID, &p.ExternalProjectID, &p.TenantID, &p.Slug, &p.Name, &p.Description,
		&p.Status, &p.Visibility, &p.PublicTokenHash, &p.DefaultTheme, &p.DefaultLocale,
		&p.RenderEnabled, &p.BadgeEnabled, &p.WidgetEnabled, &p.ChartEnabled,
		&p.WebsiteTrackingEnabled, &p.WebsiteDomains,
		&p.LastSyncedAt, &p.DeletedAt, &p.CreatedAt, &p.UpdatedAt,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get project: %w", err)
	}

	return &p, nil
}

func (r *PostgresRepository) ListByUser(ctx context.Context, userID string) ([]*Project, error) {
	query := `
		SELECT id, user_id, external_project_id, tenant_id, slug, name, description, status, visibility,
			public_token_hash, default_theme, default_locale, render_enabled, badge_enabled,
			widget_enabled, chart_enabled, website_tracking_enabled, website_domains,
			last_synced_at, deleted_at, created_at, updated_at
		FROM projects
		WHERE user_id = $1 AND deleted_at IS NULL
		ORDER BY created_at DESC
	`

	rows, err := r.pool.Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to list projects: %w", err)
	}
	defer rows.Close()

	var projects []*Project
	for rows.Next() {
		var p Project
		err := rows.Scan(
			&p.ID, &p.UserID, &p.ExternalProjectID, &p.TenantID, &p.Slug, &p.Name, &p.Description,
			&p.Status, &p.Visibility, &p.PublicTokenHash, &p.DefaultTheme, &p.DefaultLocale,
			&p.RenderEnabled, &p.BadgeEnabled, &p.WidgetEnabled, &p.ChartEnabled,
			&p.WebsiteTrackingEnabled, &p.WebsiteDomains,
			&p.LastSyncedAt, &p.DeletedAt, &p.CreatedAt, &p.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan project: %w", err)
		}
		projects = append(projects, &p)
	}

	return projects, rows.Err()
}

func (r *PostgresRepository) ListAll(ctx context.Context) ([]*Project, error) {
	query := `
		SELECT id, user_id, external_project_id, tenant_id, slug, name, description, status, visibility,
			public_token_hash, default_theme, default_locale, render_enabled, badge_enabled,
			widget_enabled, chart_enabled, website_tracking_enabled, website_domains,
			last_synced_at, deleted_at, created_at, updated_at
		FROM projects
		WHERE deleted_at IS NULL
		ORDER BY created_at ASC
	`

	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to list projects: %w", err)
	}
	defer rows.Close()

	var projects []*Project
	for rows.Next() {
		var p Project
		err := rows.Scan(
			&p.ID, &p.UserID, &p.ExternalProjectID, &p.TenantID, &p.Slug, &p.Name, &p.Description,
			&p.Status, &p.Visibility, &p.PublicTokenHash, &p.DefaultTheme, &p.DefaultLocale,
			&p.RenderEnabled, &p.BadgeEnabled, &p.WidgetEnabled, &p.ChartEnabled,
			&p.WebsiteTrackingEnabled, &p.WebsiteDomains,
			&p.LastSyncedAt, &p.DeletedAt, &p.CreatedAt, &p.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan project: %w", err)
		}
		projects = append(projects, &p)
	}

	return projects, rows.Err()
}

func (r *PostgresRepository) GetBySlug(ctx context.Context, slug string) (*Project, error) {
	query := `
		SELECT id, user_id, external_project_id, tenant_id, slug, name, description, status, visibility,
			public_token_hash, default_theme, default_locale, render_enabled, badge_enabled,
			widget_enabled, chart_enabled, website_tracking_enabled, website_domains,
			last_synced_at, deleted_at, created_at, updated_at
		FROM projects
		WHERE slug = $1 AND deleted_at IS NULL
	`

	var p Project
	err := r.pool.QueryRow(ctx, query, slug).Scan(
		&p.ID, &p.UserID, &p.ExternalProjectID, &p.TenantID, &p.Slug, &p.Name, &p.Description,
		&p.Status, &p.Visibility, &p.PublicTokenHash, &p.DefaultTheme, &p.DefaultLocale,
		&p.RenderEnabled, &p.BadgeEnabled, &p.WidgetEnabled, &p.ChartEnabled,
		&p.WebsiteTrackingEnabled, &p.WebsiteDomains,
		&p.LastSyncedAt, &p.DeletedAt, &p.CreatedAt, &p.UpdatedAt,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get project by slug: %w", err)
	}

	return &p, nil
}

func (r *PostgresRepository) GetByExternalID(ctx context.Context, externalID string) (*Project, error) {
	query := `
		SELECT id, user_id, external_project_id, tenant_id, slug, name, description, status, visibility,
			public_token_hash, default_theme, default_locale, render_enabled, badge_enabled,
			widget_enabled, chart_enabled, website_tracking_enabled, website_domains,
			last_synced_at, deleted_at, created_at, updated_at
		FROM projects
		WHERE external_project_id = $1 AND deleted_at IS NULL
	`

	var p Project
	err := r.pool.QueryRow(ctx, query, externalID).Scan(
		&p.ID, &p.UserID, &p.ExternalProjectID, &p.TenantID, &p.Slug, &p.Name, &p.Description,
		&p.Status, &p.Visibility, &p.PublicTokenHash, &p.DefaultTheme, &p.DefaultLocale,
		&p.RenderEnabled, &p.BadgeEnabled, &p.WidgetEnabled, &p.ChartEnabled,
		&p.WebsiteTrackingEnabled, &p.WebsiteDomains,
		&p.LastSyncedAt, &p.DeletedAt, &p.CreatedAt, &p.UpdatedAt,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get project by external ID: %w", err)
	}

	return &p, nil
}

func (r *PostgresRepository) Create(ctx context.Context, project *Project) error {
	query := `
		INSERT INTO projects (
			id, user_id, external_project_id, tenant_id, slug, name, description, status, visibility,
			public_token_hash, default_theme, default_locale, render_enabled, badge_enabled,
			widget_enabled, chart_enabled, last_synced_at, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, NOW(), NOW())
	`

	_, err := r.pool.Exec(ctx, query,
		project.ID, project.UserID, project.ExternalProjectID, project.TenantID, project.Slug, project.Name,
		project.Description, project.Status, project.Visibility, project.PublicTokenHash,
		project.DefaultTheme, project.DefaultLocale, project.RenderEnabled, project.BadgeEnabled,
		project.WidgetEnabled, project.ChartEnabled, project.WebsiteTrackingEnabled,
		project.WebsiteDomains, project.LastSyncedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to create project: %w", err)
	}

	return nil
}

func (r *PostgresRepository) Update(ctx context.Context, project *Project) error {
	query := `
		UPDATE projects
		SET user_id = $2, external_project_id = $3, tenant_id = $4, slug = $5, name = $6, description = $7,
			status = $8, visibility = $9, public_token_hash = $10, default_theme = $11,
			default_locale = $12, render_enabled = $13, badge_enabled = $14, widget_enabled = $15,
			chart_enabled = $16, website_tracking_enabled = $17, website_domains = $18,
			last_synced_at = $19, deleted_at = $20, updated_at = NOW()
		WHERE id = $1
	`

	_, err := r.pool.Exec(ctx, query,
		project.ID, project.UserID, project.ExternalProjectID, project.TenantID, project.Slug, project.Name,
		project.Description, project.Status, project.Visibility, project.PublicTokenHash,
		project.DefaultTheme, project.DefaultLocale, project.RenderEnabled, project.BadgeEnabled,
		project.WidgetEnabled, project.ChartEnabled, project.WebsiteTrackingEnabled,
		project.WebsiteDomains, project.LastSyncedAt, project.DeletedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to update project: %w", err)
	}

	return nil
}

func (r *PostgresRepository) Delete(ctx context.Context, id, userID string) error {
	query := `
		UPDATE projects
		SET deleted_at = NOW(), updated_at = NOW()
		WHERE id = $1 AND user_id = $2
	`

	_, err := r.pool.Exec(ctx, query, id, userID)
	if err != nil {
		return fmt.Errorf("failed to delete project: %w", err)
	}

	return nil
}

func (r *PostgresRepository) Upsert(ctx context.Context, project *Project) error {
	query := `
		INSERT INTO projects (
			id, user_id, external_project_id, tenant_id, slug, name, description, status, visibility,
			public_token_hash, default_theme, default_locale, render_enabled, badge_enabled,
			widget_enabled, chart_enabled, last_synced_at, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, NOW(), NOW())
		ON CONFLICT (id) DO UPDATE SET
			user_id = EXCLUDED.user_id,
			external_project_id = EXCLUDED.external_project_id,
			tenant_id = EXCLUDED.tenant_id,
			slug = EXCLUDED.slug,
			name = EXCLUDED.name,
			description = EXCLUDED.description,
			status = EXCLUDED.status,
			visibility = EXCLUDED.visibility,
			public_token_hash = EXCLUDED.public_token_hash,
			default_theme = EXCLUDED.default_theme,
			default_locale = EXCLUDED.default_locale,
			render_enabled = EXCLUDED.render_enabled,
			badge_enabled = EXCLUDED.badge_enabled,
			widget_enabled = EXCLUDED.widget_enabled,
			chart_enabled = EXCLUDED.chart_enabled,
			last_synced_at = EXCLUDED.last_synced_at,
			updated_at = NOW()
	`

	_, err := r.pool.Exec(ctx, query,
		project.ID, project.UserID, project.ExternalProjectID, project.TenantID, project.Slug, project.Name,
		project.Description, project.Status, project.Visibility, project.PublicTokenHash,
		project.DefaultTheme, project.DefaultLocale, project.RenderEnabled, project.BadgeEnabled,
		project.WidgetEnabled, project.ChartEnabled, project.LastSyncedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to upsert project: %w", err)
	}

	return nil
}

func (r *PostgresRepository) GetLimits(ctx context.Context, projectID string) (*ProjectLimits, error) {
	query := `
		SELECT id, project_id, plan_code, max_requests_per_minute, cache_ttl_seconds,
			badge_enabled, widget_enabled, chart_enabled, render_enabled,
			effective_from, effective_until, created_at, updated_at
		FROM project_limits
		WHERE project_id = $1
	`

	var l ProjectLimits
	err := r.pool.QueryRow(ctx, query, projectID).Scan(
		&l.ID, &l.ProjectID, &l.PlanCode, &l.MaxRequestsPerMinute, &l.CacheTTLSeconds,
		&l.BadgeEnabled, &l.WidgetEnabled, &l.ChartEnabled, &l.RenderEnabled,
		&l.EffectiveFrom, &l.EffectiveUntil, &l.CreatedAt, &l.UpdatedAt,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get project limits: %w", err)
	}

	return &l, nil
}

func (r *PostgresRepository) UpsertLimits(ctx context.Context, limits *ProjectLimits) error {
	query := `
		INSERT INTO project_limits (
			id, project_id, plan_code, max_requests_per_minute, cache_ttl_seconds,
			badge_enabled, widget_enabled, chart_enabled, render_enabled,
			effective_from, effective_until, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, NOW(), NOW())
		ON CONFLICT (project_id) DO UPDATE SET
			plan_code = EXCLUDED.plan_code,
			max_requests_per_minute = EXCLUDED.max_requests_per_minute,
			cache_ttl_seconds = EXCLUDED.cache_ttl_seconds,
			badge_enabled = EXCLUDED.badge_enabled,
			widget_enabled = EXCLUDED.widget_enabled,
			chart_enabled = EXCLUDED.chart_enabled,
			render_enabled = EXCLUDED.render_enabled,
			effective_from = EXCLUDED.effective_from,
			effective_until = EXCLUDED.effective_until,
			updated_at = NOW()
	`

	_, err := r.pool.Exec(ctx, query,
		limits.ID, limits.ProjectID, limits.PlanCode, limits.MaxRequestsPerMinute,
		limits.CacheTTLSeconds, limits.BadgeEnabled, limits.WidgetEnabled,
		limits.ChartEnabled, limits.RenderEnabled, limits.EffectiveFrom, limits.EffectiveUntil,
	)

	if err != nil {
		return fmt.Errorf("failed to upsert project limits: %w", err)
	}

	return nil
}

func (r *PostgresRepository) GetWidgetSettings(ctx context.Context, projectID, widgetKey string) (*WidgetSettings, error) {
	query := `
		SELECT id, project_id, widget_key, status, settings, version, created_at, updated_at
		FROM widget_settings
		WHERE project_id = $1 AND widget_key = $2
	`

	var s WidgetSettings
	var settingsJSON []byte
	err := r.pool.QueryRow(ctx, query, projectID, widgetKey).Scan(
		&s.ID, &s.ProjectID, &s.WidgetKey, &s.Status, &settingsJSON,
		&s.Version, &s.CreatedAt, &s.UpdatedAt,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get widget settings: %w", err)
	}

	if err := json.Unmarshal(settingsJSON, &s.Settings); err != nil {
		log.Warn().Err(err).Msg("Failed to unmarshal widget settings")
		s.Settings = make(map[string]interface{})
	}

	return &s, nil
}

func (r *PostgresRepository) UpsertWidgetSettings(ctx context.Context, settings *WidgetSettings) error {
	settingsJSON, err := json.Marshal(settings.Settings)
	if err != nil {
		return fmt.Errorf("failed to marshal widget settings: %w", err)
	}

	query := `
		INSERT INTO widget_settings (
			id, project_id, widget_key, status, settings, version, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, NOW(), NOW())
		ON CONFLICT (project_id, widget_key) DO UPDATE SET
			status = EXCLUDED.status,
			settings = EXCLUDED.settings,
			version = EXCLUDED.version,
			updated_at = NOW()
	`

	_, err = r.pool.Exec(ctx, query,
		settings.ID, settings.ProjectID, settings.WidgetKey, settings.Status, settingsJSON, settings.Version,
	)

	if err != nil {
		return fmt.Errorf("failed to upsert widget settings: %w", err)
	}

	return nil
}

func (r *PostgresRepository) GetCounter(ctx context.Context, projectID, counterName string) (int64, error) {
	query := `
		SELECT value
		FROM project_counters
		WHERE project_id = $1 AND counter_name = $2
	`

	var value int64
	err := r.pool.QueryRow(ctx, query, projectID, counterName).Scan(&value)
	if err != nil {
		if err == pgx.ErrNoRows {
			return 0, nil
		}
		return 0, fmt.Errorf("failed to get counter: %w", err)
	}

	return value, nil
}

func (r *PostgresRepository) UpsertCounter(ctx context.Context, projectID, counterName string, value int64) error {
	query := `
		INSERT INTO project_counters (
			project_id, counter_name, value, created_at, updated_at
		) VALUES ($1, $2, $3, NOW(), NOW())
		ON CONFLICT (project_id, counter_name) DO UPDATE SET
			value = GREATEST(project_counters.value, EXCLUDED.value),
			updated_at = NOW()
	`

	_, err := r.pool.Exec(ctx, query, projectID, counterName, value)
	if err != nil {
		return fmt.Errorf("failed to upsert counter: %w", err)
	}

	return nil
}

func (r *PostgresRepository) ListCounters(ctx context.Context, projectID string) (map[string]int64, error) {
	query := `
		SELECT counter_name, value
		FROM project_counters
		WHERE project_id = $1
	`

	rows, err := r.pool.Query(ctx, query, projectID)
	if err != nil {
		return nil, fmt.Errorf("failed to list counters: %w", err)
	}
	defer rows.Close()

	counters := make(map[string]int64)
	for rows.Next() {
		var name string
		var val int64
		if err := rows.Scan(&name, &val); err != nil {
			return nil, fmt.Errorf("failed to scan counter: %w", err)
		}
		counters[name] = val
	}

	return counters, rows.Err()
}
