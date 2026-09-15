package conversion

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/svgstat/svgstat/internal/cache"
)

var eventNamePattern = regexp.MustCompile(`^[a-z][a-z0-9_.-]{0,63}$`)

type ValidationError struct{ Message string }

func (e *ValidationError) Error() string   { return e.Message }
func validationError(message string) error { return &ValidationError{Message: message} }

type Goal struct {
	ID        string    `json:"id"`
	ProjectID string    `json:"projectId"`
	Name      string    `json:"name"`
	EventName string    `json:"eventName"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type Funnel struct {
	ID        string    `json:"id"`
	ProjectID string    `json:"projectId"`
	Name      string    `json:"name"`
	Steps     []string  `json:"steps"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type Service struct {
	pool  *pgxpool.Pool
	cache *cache.Cache
}

func New(pool *pgxpool.Pool, cache *cache.Cache) *Service {
	return &Service{pool: pool, cache: cache}
}

func NormalizeEventName(value string) (string, error) {
	value = strings.ToLower(strings.TrimSpace(value))
	if !eventNamePattern.MatchString(value) {
		return "", validationError("event name must start with a letter and contain only lowercase letters, numbers, dots, underscores, or hyphens")
	}
	return value, nil
}

func ValidateGoal(name, eventName string) (string, string, error) {
	name = strings.TrimSpace(name)
	if name == "" || len([]rune(name)) > 80 {
		return "", "", validationError("goal name must be between 1 and 80 characters")
	}
	normalized, err := NormalizeEventName(eventName)
	return name, normalized, err
}

func ValidateFunnel(name string, steps []string) (string, []string, error) {
	name = strings.TrimSpace(name)
	if name == "" || len([]rune(name)) > 80 {
		return "", nil, validationError("funnel name must be between 1 and 80 characters")
	}
	if len(steps) < 2 || len(steps) > 8 {
		return "", nil, validationError("funnel must contain between 2 and 8 steps")
	}
	normalized := make([]string, len(steps))
	for index, step := range steps {
		value, err := NormalizeEventName(step)
		if err != nil {
			return "", nil, validationError(fmt.Sprintf("invalid funnel step %d: %s", index+1, err))
		}
		normalized[index] = value
	}
	return name, normalized, nil
}

func (s *Service) Warm(ctx context.Context) error {
	var cursor uint64
	for {
		keys, next, err := s.cache.GetClient().Scan(ctx, cursor, cache.BuildKey("project", "*", "funnel_definitions"), 100).Result()
		if err != nil {
			return fmt.Errorf("failed to reset funnel cache: %w", err)
		}
		if len(keys) > 0 {
			if err := s.cache.GetClient().Del(ctx, keys...).Err(); err != nil {
				return fmt.Errorf("failed to reset funnel cache: %w", err)
			}
		}
		cursor = next
		if cursor == 0 {
			break
		}
	}
	rows, err := s.pool.Query(ctx, `SELECT id, project_id, name, steps, created_at, updated_at FROM funnels`)
	if err != nil {
		return fmt.Errorf("failed to list funnels for cache warm: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var funnel Funnel
		if err := rows.Scan(&funnel.ID, &funnel.ProjectID, &funnel.Name, &funnel.Steps, &funnel.CreatedAt, &funnel.UpdatedAt); err != nil {
			return fmt.Errorf("failed to scan funnel for cache warm: %w", err)
		}
		if err := s.cacheFunnel(ctx, &funnel); err != nil {
			return err
		}
	}
	return rows.Err()
}

func (s *Service) ListGoals(ctx context.Context, projectID string) ([]Goal, error) {
	rows, err := s.pool.Query(ctx, `SELECT id, project_id, name, event_name, created_at, updated_at FROM conversion_goals WHERE project_id = $1 ORDER BY created_at`, projectID)
	if err != nil {
		return nil, fmt.Errorf("failed to list conversion goals: %w", err)
	}
	defer rows.Close()
	items := make([]Goal, 0)
	for rows.Next() {
		var item Goal
		if err := rows.Scan(&item.ID, &item.ProjectID, &item.Name, &item.EventName, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Service) CreateGoal(ctx context.Context, projectID, name, eventName string) (*Goal, error) {
	name, eventName, err := ValidateGoal(name, eventName)
	if err != nil {
		return nil, err
	}
	var count int
	if err := s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM conversion_goals WHERE project_id=$1`, projectID).Scan(&count); err != nil {
		return nil, fmt.Errorf("failed to count conversion goals: %w", err)
	}
	if count >= 20 {
		return nil, validationError("a project can have at most 20 conversion goals")
	}
	item := &Goal{ID: newID(), ProjectID: projectID, Name: name, EventName: eventName}
	err = s.pool.QueryRow(ctx, `INSERT INTO conversion_goals (id, project_id, name, event_name) VALUES ($1,$2,$3,$4) RETURNING created_at, updated_at`, item.ID, projectID, name, eventName).Scan(&item.CreatedAt, &item.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to create conversion goal: %w", err)
	}
	return item, nil
}

func (s *Service) UpdateGoal(ctx context.Context, projectID, id, name, eventName string) (*Goal, error) {
	name, eventName, err := ValidateGoal(name, eventName)
	if err != nil {
		return nil, err
	}
	item := &Goal{ID: id, ProjectID: projectID, Name: name, EventName: eventName}
	err = s.pool.QueryRow(ctx, `UPDATE conversion_goals SET name=$3,event_name=$4,updated_at=NOW() WHERE id=$1 AND project_id=$2 RETURNING created_at,updated_at`, id, projectID, name, eventName).Scan(&item.CreatedAt, &item.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to update conversion goal: %w", err)
	}
	return item, nil
}

func (s *Service) DeleteGoal(ctx context.Context, projectID, id string) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM conversion_goals WHERE id=$1 AND project_id=$2`, id, projectID)
	return err
}

func (s *Service) ListFunnels(ctx context.Context, projectID string) ([]Funnel, error) {
	rows, err := s.pool.Query(ctx, `SELECT id, project_id, name, steps, created_at, updated_at FROM funnels WHERE project_id=$1 ORDER BY created_at`, projectID)
	if err != nil {
		return nil, fmt.Errorf("failed to list funnels: %w", err)
	}
	defer rows.Close()
	items := make([]Funnel, 0)
	for rows.Next() {
		var item Funnel
		if err := rows.Scan(&item.ID, &item.ProjectID, &item.Name, &item.Steps, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Service) GetFunnel(ctx context.Context, projectID, id string) (*Funnel, error) {
	var item Funnel
	err := s.pool.QueryRow(ctx, `SELECT id, project_id, name, steps, created_at, updated_at FROM funnels WHERE id=$1 AND project_id=$2`, id, projectID).Scan(&item.ID, &item.ProjectID, &item.Name, &item.Steps, &item.CreatedAt, &item.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to get funnel: %w", err)
	}
	return &item, nil
}

func (s *Service) CreateFunnel(ctx context.Context, projectID, name string, steps []string) (*Funnel, error) {
	name, steps, err := ValidateFunnel(name, steps)
	if err != nil {
		return nil, err
	}
	var count int
	if err := s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM funnels WHERE project_id=$1`, projectID).Scan(&count); err != nil {
		return nil, fmt.Errorf("failed to count funnels: %w", err)
	}
	if count >= 10 {
		return nil, validationError("a project can have at most 10 funnels")
	}
	item := &Funnel{ID: newID(), ProjectID: projectID, Name: name, Steps: steps}
	encodedSteps, _ := json.Marshal(steps)
	err = s.pool.QueryRow(ctx, `INSERT INTO funnels (id,project_id,name,steps) VALUES ($1,$2,$3,$4) RETURNING created_at,updated_at`, item.ID, projectID, name, encodedSteps).Scan(&item.CreatedAt, &item.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to create funnel: %w", err)
	}
	if err := s.cacheFunnel(ctx, item); err != nil {
		return nil, err
	}
	return item, nil
}

func (s *Service) UpdateFunnel(ctx context.Context, projectID, id, name string, steps []string) (*Funnel, error) {
	name, steps, err := ValidateFunnel(name, steps)
	if err != nil {
		return nil, err
	}
	existing, err := s.GetFunnel(ctx, projectID, id)
	if err != nil {
		return nil, err
	}
	if !slices.Equal(existing.Steps, steps) {
		return nil, validationError("funnel steps cannot be changed; create a new funnel to preserve historical accuracy")
	}
	item := &Funnel{ID: id, ProjectID: projectID, Name: name, Steps: steps}
	encodedSteps, _ := json.Marshal(steps)
	err = s.pool.QueryRow(ctx, `UPDATE funnels SET name=$3,steps=$4,updated_at=NOW() WHERE id=$1 AND project_id=$2 RETURNING created_at,updated_at`, id, projectID, name, encodedSteps).Scan(&item.CreatedAt, &item.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to update funnel: %w", err)
	}
	if err := s.cacheFunnel(ctx, item); err != nil {
		return nil, err
	}
	return item, nil
}

func (s *Service) DeleteFunnel(ctx context.Context, projectID, id string) error {
	if _, err := s.pool.Exec(ctx, `DELETE FROM funnels WHERE id=$1 AND project_id=$2`, id, projectID); err != nil {
		return err
	}
	return s.cache.GetClient().HDel(ctx, cache.BuildKey("project", projectID, "funnel_definitions"), id).Err()
}

func (s *Service) cacheFunnel(ctx context.Context, funnel *Funnel) error {
	encoded, err := json.Marshal(funnel.Steps)
	if err != nil {
		return err
	}
	return s.cache.GetClient().HSet(ctx, cache.BuildKey("project", funnel.ProjectID, "funnel_definitions"), funnel.ID, encoded).Err()
}

func newID() string {
	value := make([]byte, 16)
	_, _ = rand.Read(value)
	return hex.EncodeToString(value)
}
