package admin

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/svgstat/svgstat/internal/project"
)

var (
	ErrNotFound       = errors.New("not found")
	ErrInvalidStatus  = errors.New("invalid status")
	ErrSelfDisable    = errors.New("administrators cannot disable themselves")
	ErrProtectedEntry = errors.New("protected project")
)

type cachePinger interface {
	Ping(ctx context.Context) error
}

type runtimeWriter interface {
	Put(ctx context.Context, project *project.Project) error
}

type Service struct {
	pool    *pgxpool.Pool
	cache   cachePinger
	runtime runtimeWriter
}

type Query struct {
	Search   string
	Status   string
	Page     int
	PageSize int
}

type Page[T any] struct {
	Items      []T `json:"items"`
	Page       int `json:"page"`
	PageSize   int `json:"pageSize"`
	Total      int `json:"total"`
	TotalPages int `json:"totalPages"`
}

type Summary struct {
	Users            int64 `json:"users"`
	NewUsersToday    int64 `json:"newUsersToday"`
	Projects         int64 `json:"projects"`
	ActiveProjects   int64 `json:"activeProjects"`
	DisabledProjects int64 `json:"disabledProjects"`
	PV               int64 `json:"pv"`
	UV               int64 `json:"uv"`
	Requests         int64 `json:"requests"`
	Bots             int64 `json:"bots"`
}

type TrendPoint struct {
	Date     string `json:"date"`
	PV       int64  `json:"pv"`
	UV       int64  `json:"uv"`
	Requests int64  `json:"requests"`
	Bots     int64  `json:"bots"`
}

type RecentUser struct {
	ID        string    `json:"id"`
	Email     string    `json:"email"`
	Name      string    `json:"name"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"createdAt"`
}

type RecentProject struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	Slug       string    `json:"slug"`
	Status     string    `json:"status"`
	OwnerEmail string    `json:"ownerEmail"`
	CreatedAt  time.Time `json:"createdAt"`
}

type TopProject struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Slug     string `json:"slug"`
	Requests int64  `json:"requests"`
}

type Health struct {
	Database string `json:"database"`
	Redis    string `json:"redis"`
}

type Overview struct {
	Summary        Summary         `json:"summary"`
	Trend          []TrendPoint    `json:"trend"`
	TopProjects    []TopProject    `json:"topProjects"`
	RecentUsers    []RecentUser    `json:"recentUsers"`
	RecentProjects []RecentProject `json:"recentProjects"`
	Health         Health          `json:"health"`
}

type User struct {
	ID           string     `json:"id"`
	Email        string     `json:"email"`
	Name         string     `json:"name"`
	Status       string     `json:"status"`
	Role         string     `json:"role"`
	ProjectCount int64      `json:"projectCount"`
	LastLoginAt  *time.Time `json:"lastLoginAt,omitempty"`
	CreatedAt    time.Time  `json:"createdAt"`
	UpdatedAt    time.Time  `json:"updatedAt"`
}

type Project struct {
	ID            string    `json:"id"`
	UserID        string    `json:"userId"`
	OwnerEmail    string    `json:"ownerEmail"`
	Name          string    `json:"name"`
	Slug          string    `json:"slug"`
	Description   string    `json:"description"`
	Status        string    `json:"status"`
	Visibility    string    `json:"visibility"`
	RenderEnabled bool      `json:"renderEnabled"`
	BadgeEnabled  bool      `json:"badgeEnabled"`
	WidgetEnabled bool      `json:"widgetEnabled"`
	ChartEnabled  bool      `json:"chartEnabled"`
	Requests7d    int64     `json:"requests7d"`
	CreatedAt     time.Time `json:"createdAt"`
	UpdatedAt     time.Time `json:"updatedAt"`
}

type Capabilities struct {
	RenderEnabled *bool `json:"renderEnabled"`
	BadgeEnabled  *bool `json:"badgeEnabled"`
	WidgetEnabled *bool `json:"widgetEnabled"`
	ChartEnabled  *bool `json:"chartEnabled"`
}

func NewService(pool *pgxpool.Pool, cache cachePinger, runtime runtimeWriter) *Service {
	return &Service{pool: pool, cache: cache, runtime: runtime}
}

func NormalizeQuery(query Query) Query {
	query.Search = strings.TrimSpace(query.Search)
	query.Status = strings.TrimSpace(strings.ToLower(query.Status))
	if query.Page < 1 {
		query.Page = 1
	}
	if query.PageSize < 1 {
		query.PageSize = 20
	}
	if query.PageSize > 100 {
		query.PageSize = 100
	}
	return query
}

func (s *Service) GetOverview(ctx context.Context) (*Overview, error) {
	overview := &Overview{
		Trend:          make([]TrendPoint, 0, 7),
		TopProjects:    []TopProject{},
		RecentUsers:    []RecentUser{},
		RecentProjects: []RecentProject{},
		Health:         Health{Database: "ok", Redis: "ok"},
	}
	if err := s.pool.QueryRow(ctx, `
		SELECT
			(SELECT COUNT(*) FROM users),
			(SELECT COUNT(*) FROM users WHERE created_at >= date_trunc('day', NOW())),
			(SELECT COUNT(*) FROM projects WHERE deleted_at IS NULL),
			(SELECT COUNT(*) FROM projects WHERE deleted_at IS NULL AND status = 'active'),
			(SELECT COUNT(*) FROM projects WHERE deleted_at IS NULL AND status IN ('disabled', 'archived')),
			COALESCE((SELECT SUM(pv) FROM daily_statistics WHERE date = CURRENT_DATE), 0),
			COALESCE((SELECT SUM(uv) FROM daily_statistics WHERE date = CURRENT_DATE), 0),
			COALESCE((SELECT SUM(requests) FROM daily_statistics WHERE date = CURRENT_DATE), 0),
			COALESCE((SELECT SUM(bots) FROM daily_statistics WHERE date = CURRENT_DATE), 0)
	`).Scan(
		&overview.Summary.Users, &overview.Summary.NewUsersToday, &overview.Summary.Projects,
		&overview.Summary.ActiveProjects, &overview.Summary.DisabledProjects, &overview.Summary.PV,
		&overview.Summary.UV, &overview.Summary.Requests, &overview.Summary.Bots,
	); err != nil {
		return nil, fmt.Errorf("failed to query overview summary: %w", err)
	}
	if err := s.loadTrend(ctx, overview); err != nil {
		return nil, err
	}
	if err := s.loadOverviewLists(ctx, overview); err != nil {
		return nil, err
	}
	if err := s.pool.Ping(ctx); err != nil {
		overview.Health.Database = "error"
	}
	if s.cache == nil || s.cache.Ping(ctx) != nil {
		overview.Health.Redis = "error"
	}
	return overview, nil
}

func (s *Service) loadTrend(ctx context.Context, overview *Overview) error {
	rows, err := s.pool.Query(ctx, `
		SELECT date, SUM(pv), SUM(uv), SUM(requests), SUM(bots)
		FROM daily_statistics
		WHERE date >= CURRENT_DATE - INTERVAL '6 days'
		GROUP BY date ORDER BY date
	`)
	if err != nil {
		return fmt.Errorf("failed to query overview trend: %w", err)
	}
	defer rows.Close()
	stored := make(map[string]TrendPoint)
	for rows.Next() {
		var date time.Time
		var point TrendPoint
		if err := rows.Scan(&date, &point.PV, &point.UV, &point.Requests, &point.Bots); err != nil {
			return fmt.Errorf("failed to scan overview trend: %w", err)
		}
		point.Date = date.Format("2006-01-02")
		stored[point.Date] = point
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("failed to read overview trend: %w", err)
	}
	today := time.Now().UTC()
	for daysAgo := 6; daysAgo >= 0; daysAgo-- {
		date := today.AddDate(0, 0, -daysAgo).Format("2006-01-02")
		point := stored[date]
		point.Date = date
		overview.Trend = append(overview.Trend, point)
	}
	return nil
}

func (s *Service) loadOverviewLists(ctx context.Context, overview *Overview) error {
	rows, err := s.pool.Query(ctx, `
		SELECT p.id, p.name, p.slug, COALESCE(SUM(d.requests), 0) AS requests
		FROM projects p
		LEFT JOIN daily_statistics d ON d.project_id = p.id AND d.date >= CURRENT_DATE - INTERVAL '6 days'
		WHERE p.deleted_at IS NULL
		GROUP BY p.id, p.name, p.slug
		ORDER BY requests DESC, p.created_at DESC LIMIT 5
	`)
	if err != nil {
		return fmt.Errorf("failed to query top projects: %w", err)
	}
	for rows.Next() {
		var item TopProject
		if err := rows.Scan(&item.ID, &item.Name, &item.Slug, &item.Requests); err != nil {
			rows.Close()
			return fmt.Errorf("failed to scan top project: %w", err)
		}
		overview.TopProjects = append(overview.TopProjects, item)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return fmt.Errorf("failed to read top projects: %w", err)
	}
	rows.Close()

	rows, err = s.pool.Query(ctx, `SELECT id, email, COALESCE(name, ''), status, created_at FROM users ORDER BY created_at DESC LIMIT 5`)
	if err != nil {
		return fmt.Errorf("failed to query recent users: %w", err)
	}
	for rows.Next() {
		var item RecentUser
		if err := rows.Scan(&item.ID, &item.Email, &item.Name, &item.Status, &item.CreatedAt); err != nil {
			rows.Close()
			return fmt.Errorf("failed to scan recent user: %w", err)
		}
		overview.RecentUsers = append(overview.RecentUsers, item)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return fmt.Errorf("failed to read recent users: %w", err)
	}
	rows.Close()

	rows, err = s.pool.Query(ctx, `
		SELECT p.id, p.name, p.slug, p.status, COALESCE(u.email, ''), p.created_at
		FROM projects p LEFT JOIN users u ON u.id = p.user_id
		WHERE p.deleted_at IS NULL ORDER BY p.created_at DESC LIMIT 5
	`)
	if err != nil {
		return fmt.Errorf("failed to query recent projects: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var item RecentProject
		if err := rows.Scan(&item.ID, &item.Name, &item.Slug, &item.Status, &item.OwnerEmail, &item.CreatedAt); err != nil {
			return fmt.Errorf("failed to scan recent project: %w", err)
		}
		overview.RecentProjects = append(overview.RecentProjects, item)
	}
	return rows.Err()
}

func (s *Service) ListUsers(ctx context.Context, query Query) (*Page[User], error) {
	query = NormalizeQuery(query)
	filter := `%` + query.Search + `%`
	var total int
	if err := s.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM users
		WHERE ($1 = '' OR email ILIKE $2 OR COALESCE(name, '') ILIKE $2)
		AND ($3 = '' OR status = $3)
	`, query.Search, filter, query.Status).Scan(&total); err != nil {
		return nil, fmt.Errorf("failed to count users: %w", err)
	}
	rows, err := s.pool.Query(ctx, `
		SELECT u.id, u.email, COALESCE(u.name, ''), u.status, u.role,
			COUNT(DISTINCT p.id), u.last_login_at, u.created_at, u.updated_at
		FROM users u
		LEFT JOIN projects p ON p.user_id = u.id AND p.deleted_at IS NULL
		WHERE ($1 = '' OR u.email ILIKE $2 OR COALESCE(u.name, '') ILIKE $2)
		AND ($3 = '' OR u.status = $3)
		GROUP BY u.id, u.email, u.name, u.status, u.role, u.last_login_at, u.created_at, u.updated_at
		ORDER BY u.created_at DESC LIMIT $4 OFFSET $5
	`, query.Search, filter, query.Status, query.PageSize, (query.Page-1)*query.PageSize)
	if err != nil {
		return nil, fmt.Errorf("failed to list users: %w", err)
	}
	defer rows.Close()
	items := make([]User, 0, query.PageSize)
	for rows.Next() {
		var item User
		if err := rows.Scan(&item.ID, &item.Email, &item.Name, &item.Status, &item.Role, &item.ProjectCount, &item.LastLoginAt, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan user: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to read users: %w", err)
	}
	return newPage(items, query, total), nil
}

func (s *Service) GetUser(ctx context.Context, id string) (*User, []Project, error) {
	var found User
	err := s.pool.QueryRow(ctx, `
		SELECT u.id, u.email, COALESCE(u.name, ''), u.status, u.role,
			COUNT(DISTINCT p.id), u.last_login_at, u.created_at, u.updated_at
		FROM users u
		LEFT JOIN projects p ON p.user_id = u.id AND p.deleted_at IS NULL
		WHERE u.id = $1
		GROUP BY u.id, u.email, u.name, u.status, u.role, u.last_login_at, u.created_at, u.updated_at
	`, id).Scan(&found.ID, &found.Email, &found.Name, &found.Status, &found.Role, &found.ProjectCount, &found.LastLoginAt, &found.CreatedAt, &found.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil, ErrNotFound
	}
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get user: %w", err)
	}
	projects, err := s.listProjectsByUser(ctx, id)
	return &found, projects, err
}

func (s *Service) UpdateUserStatus(ctx context.Context, adminUserID, userID, status, ipAddress string) (*User, error) {
	if status != "active" && status != "disabled" {
		return nil, ErrInvalidStatus
	}
	if adminUserID == userID && status == "disabled" {
		return nil, ErrSelfDisable
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to begin user update: %w", err)
	}
	defer tx.Rollback(ctx)
	var user User
	if err := tx.QueryRow(ctx, `
		SELECT id, email, COALESCE(name, ''), status, role, created_at, updated_at
		FROM users WHERE id = $1 FOR UPDATE
	`, userID).Scan(&user.ID, &user.Email, &user.Name, &user.Status, &user.Role, &user.CreatedAt, &user.UpdatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("failed to get user for update: %w", err)
	}
	before := map[string]interface{}{"status": user.Status}
	if _, err := tx.Exec(ctx, `UPDATE users SET status = $2, updated_at = NOW() WHERE id = $1`, userID, status); err != nil {
		return nil, fmt.Errorf("failed to update user status: %w", err)
	}
	if status == "disabled" {
		if _, err := tx.Exec(ctx, `DELETE FROM sessions WHERE user_id = $1`, userID); err != nil {
			return nil, fmt.Errorf("failed to revoke user sessions: %w", err)
		}
	}
	if err := insertAudit(ctx, tx, adminUserID, "user.status.update", "user", userID, before, map[string]interface{}{"status": status}, ipAddress); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("failed to commit user update: %w", err)
	}
	user.Status = status
	user.UpdatedAt = time.Now()
	return &user, nil
}

func (s *Service) ListProjects(ctx context.Context, query Query) (*Page[Project], error) {
	query = NormalizeQuery(query)
	filter := `%` + query.Search + `%`
	var total int
	if err := s.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM projects p LEFT JOIN users u ON u.id = p.user_id
		WHERE p.deleted_at IS NULL
		AND ($1 = '' OR p.name ILIKE $2 OR p.slug ILIKE $2 OR COALESCE(u.email, '') ILIKE $2)
		AND ($3 = '' OR p.status = $3)
	`, query.Search, filter, query.Status).Scan(&total); err != nil {
		return nil, fmt.Errorf("failed to count projects: %w", err)
	}
	rows, err := s.pool.Query(ctx, `
		SELECT p.id, COALESCE(p.user_id, ''), COALESCE(u.email, ''), p.name, p.slug, COALESCE(p.description, ''),
			p.status, p.visibility, p.render_enabled, p.badge_enabled, p.widget_enabled, p.chart_enabled,
			COALESCE(SUM(d.requests), 0), p.created_at, p.updated_at
		FROM projects p
		LEFT JOIN users u ON u.id = p.user_id
		LEFT JOIN daily_statistics d ON d.project_id = p.id AND d.date >= CURRENT_DATE - INTERVAL '6 days'
		WHERE p.deleted_at IS NULL
		AND ($1 = '' OR p.name ILIKE $2 OR p.slug ILIKE $2 OR COALESCE(u.email, '') ILIKE $2)
		AND ($3 = '' OR p.status = $3)
		GROUP BY p.id, p.user_id, u.email, p.name, p.slug, p.description, p.status, p.visibility,
			p.render_enabled, p.badge_enabled, p.widget_enabled, p.chart_enabled, p.created_at, p.updated_at
		ORDER BY p.created_at DESC LIMIT $4 OFFSET $5
	`, query.Search, filter, query.Status, query.PageSize, (query.Page-1)*query.PageSize)
	if err != nil {
		return nil, fmt.Errorf("failed to list projects: %w", err)
	}
	defer rows.Close()
	items := make([]Project, 0, query.PageSize)
	for rows.Next() {
		var item Project
		if err := rows.Scan(&item.ID, &item.UserID, &item.OwnerEmail, &item.Name, &item.Slug, &item.Description,
			&item.Status, &item.Visibility, &item.RenderEnabled, &item.BadgeEnabled, &item.WidgetEnabled,
			&item.ChartEnabled, &item.Requests7d, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan project: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to read projects: %w", err)
	}
	return newPage(items, query, total), nil
}

func (s *Service) GetProject(ctx context.Context, id string) (*Project, error) {
	var item Project
	err := s.pool.QueryRow(ctx, `
		SELECT p.id, COALESCE(p.user_id, ''), COALESCE(u.email, ''), p.name, p.slug, COALESCE(p.description, ''),
			p.status, p.visibility, p.render_enabled, p.badge_enabled, p.widget_enabled, p.chart_enabled,
			COALESCE(SUM(d.requests), 0), p.created_at, p.updated_at
		FROM projects p
		LEFT JOIN users u ON u.id = p.user_id
		LEFT JOIN daily_statistics d ON d.project_id = p.id AND d.date >= CURRENT_DATE - INTERVAL '6 days'
		WHERE p.id = $1 AND p.deleted_at IS NULL
		GROUP BY p.id, p.user_id, u.email, p.name, p.slug, p.description, p.status, p.visibility,
			p.render_enabled, p.badge_enabled, p.widget_enabled, p.chart_enabled, p.created_at, p.updated_at
	`, id).Scan(&item.ID, &item.UserID, &item.OwnerEmail, &item.Name, &item.Slug, &item.Description,
		&item.Status, &item.Visibility, &item.RenderEnabled, &item.BadgeEnabled, &item.WidgetEnabled,
		&item.ChartEnabled, &item.Requests7d, &item.CreatedAt, &item.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get project: %w", err)
	}
	return &item, nil
}

func (s *Service) UpdateProjectStatus(ctx context.Context, adminUserID, projectID, status, ipAddress string) (*Project, error) {
	if status != "active" && status != "disabled" && status != "archived" {
		return nil, ErrInvalidStatus
	}
	if projectID == "free" || projectID == "demo" {
		return nil, ErrProtectedEntry
	}
	return s.updateProject(ctx, adminUserID, projectID, "project.status.update", ipAddress, func(current *project.Project) map[string]interface{} {
		current.Status = status
		return map[string]interface{}{"status": status}
	})
}

func (s *Service) UpdateProjectCapabilities(ctx context.Context, adminUserID, projectID, ipAddress string, capabilities Capabilities) (*Project, error) {
	return s.updateProject(ctx, adminUserID, projectID, "project.capabilities.update", ipAddress, func(current *project.Project) map[string]interface{} {
		after := make(map[string]interface{})
		if capabilities.RenderEnabled != nil {
			current.RenderEnabled = *capabilities.RenderEnabled
			after["renderEnabled"] = current.RenderEnabled
		}
		if capabilities.BadgeEnabled != nil {
			current.BadgeEnabled = *capabilities.BadgeEnabled
			after["badgeEnabled"] = current.BadgeEnabled
		}
		if capabilities.WidgetEnabled != nil {
			current.WidgetEnabled = *capabilities.WidgetEnabled
			after["widgetEnabled"] = current.WidgetEnabled
		}
		if capabilities.ChartEnabled != nil {
			current.ChartEnabled = *capabilities.ChartEnabled
			after["chartEnabled"] = current.ChartEnabled
		}
		return after
	})
}

func (s *Service) updateProject(ctx context.Context, adminUserID, projectID, action, ipAddress string, change func(*project.Project) map[string]interface{}) (*Project, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to begin project update: %w", err)
	}
	defer tx.Rollback(ctx)
	current, err := getRuntimeProject(ctx, tx, projectID)
	if err != nil {
		return nil, err
	}
	before := map[string]interface{}{
		"status": current.Status, "renderEnabled": current.RenderEnabled, "badgeEnabled": current.BadgeEnabled,
		"widgetEnabled": current.WidgetEnabled, "chartEnabled": current.ChartEnabled,
	}
	after := change(current)
	if len(after) == 0 {
		return nil, ErrInvalidStatus
	}
	if _, err := tx.Exec(ctx, `
		UPDATE projects SET status = $2, render_enabled = $3, badge_enabled = $4,
			widget_enabled = $5, chart_enabled = $6, updated_at = NOW() WHERE id = $1
	`, current.ID, current.Status, current.RenderEnabled, current.BadgeEnabled, current.WidgetEnabled, current.ChartEnabled); err != nil {
		return nil, fmt.Errorf("failed to update project: %w", err)
	}
	if err := insertAudit(ctx, tx, adminUserID, action, "project", projectID, before, after, ipAddress); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("failed to commit project update: %w", err)
	}
	current.UpdatedAt = time.Now()
	if s.runtime != nil {
		if err := s.runtime.Put(ctx, current); err != nil {
			return nil, fmt.Errorf("project updated but runtime cache refresh failed: %w", err)
		}
	}
	return s.GetProject(ctx, projectID)
}

func (s *Service) listProjectsByUser(ctx context.Context, userID string) ([]Project, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT p.id, COALESCE(p.user_id, ''), COALESCE(u.email, ''), p.name, p.slug, COALESCE(p.description, ''),
			p.status, p.visibility, p.render_enabled, p.badge_enabled, p.widget_enabled, p.chart_enabled,
			COALESCE(SUM(d.requests), 0), p.created_at, p.updated_at
		FROM projects p LEFT JOIN users u ON u.id = p.user_id
		LEFT JOIN daily_statistics d ON d.project_id = p.id AND d.date >= CURRENT_DATE - INTERVAL '6 days'
		WHERE p.deleted_at IS NULL AND p.user_id = $1
		GROUP BY p.id, p.user_id, u.email, p.name, p.slug, p.description, p.status, p.visibility,
			p.render_enabled, p.badge_enabled, p.widget_enabled, p.chart_enabled, p.created_at, p.updated_at
		ORDER BY p.created_at DESC
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to list user projects: %w", err)
	}
	defer rows.Close()
	items := []Project{}
	for rows.Next() {
		var item Project
		if err := rows.Scan(&item.ID, &item.UserID, &item.OwnerEmail, &item.Name, &item.Slug, &item.Description,
			&item.Status, &item.Visibility, &item.RenderEnabled, &item.BadgeEnabled, &item.WidgetEnabled,
			&item.ChartEnabled, &item.Requests7d, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan user project: %w", err)
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func getRuntimeProject(ctx context.Context, tx pgx.Tx, id string) (*project.Project, error) {
	var item project.Project
	err := tx.QueryRow(ctx, `
		SELECT id, COALESCE(user_id, ''), external_project_id, tenant_id, slug, name, COALESCE(description, ''), status,
			visibility, COALESCE(public_token_hash, ''), COALESCE(default_theme, ''), COALESCE(default_locale, ''),
			render_enabled, badge_enabled, widget_enabled, chart_enabled, website_tracking_enabled, website_domains,
			last_synced_at, deleted_at, created_at, updated_at
		FROM projects WHERE id = $1 AND deleted_at IS NULL FOR UPDATE
	`, id).Scan(
		&item.ID, &item.UserID, &item.ExternalProjectID, &item.TenantID, &item.Slug, &item.Name, &item.Description,
		&item.Status, &item.Visibility, &item.PublicTokenHash, &item.DefaultTheme, &item.DefaultLocale,
		&item.RenderEnabled, &item.BadgeEnabled, &item.WidgetEnabled, &item.ChartEnabled,
		&item.WebsiteTrackingEnabled, &item.WebsiteDomains,
		&item.LastSyncedAt, &item.DeletedAt, &item.CreatedAt, &item.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get project for update: %w", err)
	}
	return &item, nil
}

func insertAudit(ctx context.Context, tx pgx.Tx, adminUserID, action, targetType, targetID string, before, after interface{}, ipAddress string) error {
	beforeJSON, err := json.Marshal(before)
	if err != nil {
		return fmt.Errorf("failed to encode audit before data: %w", err)
	}
	afterJSON, err := json.Marshal(after)
	if err != nil {
		return fmt.Errorf("failed to encode audit after data: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO admin_audit_logs (id, admin_user_id, action, target_type, target_id, before_data, after_data, ip_address)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`, newID(), adminUserID, action, targetType, targetID, beforeJSON, afterJSON, ipAddress); err != nil {
		return fmt.Errorf("failed to write admin audit log: %w", err)
	}
	return nil
}

func newPage[T any](items []T, query Query, total int) *Page[T] {
	totalPages := 0
	if total > 0 {
		totalPages = (total + query.PageSize - 1) / query.PageSize
	}
	return &Page[T]{Items: items, Page: query.Page, PageSize: query.PageSize, Total: total, TotalPages: totalPages}
}

func newID() string {
	value := make([]byte, 16)
	_, _ = rand.Read(value)
	return hex.EncodeToString(value)
}
