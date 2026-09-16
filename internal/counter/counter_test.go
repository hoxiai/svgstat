package counter

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/hoxiai/svgstat/internal/cache"
	"github.com/hoxiai/svgstat/internal/config"
	"github.com/hoxiai/svgstat/internal/project"
)

type stubProjectRepo struct {
	counters map[string]int64
}

func (s *stubProjectRepo) GetByID(ctx context.Context, id string) (*project.Project, error) {
	return nil, nil
}
func (s *stubProjectRepo) GetByIDAndUser(ctx context.Context, id, userID string) (*project.Project, error) {
	return nil, nil
}
func (s *stubProjectRepo) ListByUser(ctx context.Context, userID string) ([]*project.Project, error) {
	return nil, nil
}
func (s *stubProjectRepo) ListAll(ctx context.Context) ([]*project.Project, error) {
	return nil, nil
}
func (s *stubProjectRepo) GetBySlug(ctx context.Context, slug string) (*project.Project, error) {
	return nil, nil
}
func (s *stubProjectRepo) GetByExternalID(ctx context.Context, externalID string) (*project.Project, error) {
	return nil, nil
}
func (s *stubProjectRepo) Create(ctx context.Context, p *project.Project) error { return nil }
func (s *stubProjectRepo) Update(ctx context.Context, p *project.Project) error { return nil }
func (s *stubProjectRepo) Delete(ctx context.Context, id, userID string) error  { return nil }
func (s *stubProjectRepo) Upsert(ctx context.Context, p *project.Project) error { return nil }
func (s *stubProjectRepo) GetLimits(ctx context.Context, projectID string) (*project.ProjectLimits, error) {
	return nil, nil
}
func (s *stubProjectRepo) UpsertLimits(ctx context.Context, limits *project.ProjectLimits) error {
	return nil
}
func (s *stubProjectRepo) GetWidgetSettings(ctx context.Context, projectID, widgetKey string) (*project.WidgetSettings, error) {
	return nil, nil
}
func (s *stubProjectRepo) UpsertWidgetSettings(ctx context.Context, settings *project.WidgetSettings) error {
	return nil
}

func (s *stubProjectRepo) GetCounter(ctx context.Context, projectID, counterName string) (int64, error) {
	if s.counters == nil {
		return 0, nil
	}
	return s.counters[projectID+":"+counterName], nil
}

func (s *stubProjectRepo) UpsertCounter(ctx context.Context, projectID, counterName string, value int64) error {
	if s.counters == nil {
		s.counters = make(map[string]int64)
	}
	if value > s.counters[projectID+":"+counterName] {
		s.counters[projectID+":"+counterName] = value
	}
	return nil
}

func (s *stubProjectRepo) ListCounters(ctx context.Context, projectID string) (map[string]int64, error) {
	res := make(map[string]int64)
	prefix := projectID + ":"
	for k, v := range s.counters {
		if len(k) > len(prefix) && k[:len(prefix)] == prefix {
			res[k[len(prefix):]] = v
		}
	}
	return res, nil
}

func TestCounterColdStartFallback(t *testing.T) {
	cfg := &config.Config{
		Redis: config.RedisConfig{
			Addr: "localhost:6379",
		},
	}
	c, err := cache.New(cfg)
	if err != nil {
		t.Skip("Skipping test: Redis not reachable on localhost:6379")
	}
	defer c.Close()

	ctx := context.Background()
	projectID := fmt.Sprintf("test-proj-%d", time.Now().UnixNano())
	counterName := "visits"
	key := cache.BuildKey("project", projectID, "counter", counterName)

	// Ensure Redis key does not exist
	_ = c.Delete(ctx, key)

	repo := &stubProjectRepo{
		counters: map[string]int64{
			projectID + ":" + counterName: 100,
		},
	}

	cnt := New(c, repo, time.Hour)

	// 1. Get should load from DB and return 100
	val, err := cnt.Get(ctx, projectID, counterName)
	if err != nil {
		t.Fatalf("cnt.Get() error = %v", err)
	}
	if val != 100 {
		t.Fatalf("cnt.Get() = %d, want 100", val)
	}

	// 2. Increment should now increment from 100 -> 101
	newVal, err := cnt.Increment(ctx, projectID, counterName)
	if err != nil {
		t.Fatalf("cnt.Increment() error = %v", err)
	}
	if newVal != 101 {
		t.Fatalf("cnt.Increment() = %d, want 101", newVal)
	}

	// 3. Clean up
	_ = c.Delete(ctx, key)
	_ = c.Delete(ctx, cache.BuildKey("project", projectID, "counters"))
}

func TestCounterSetAndPersistence(t *testing.T) {
	cfg := &config.Config{
		Redis: config.RedisConfig{
			Addr: "localhost:6379",
		},
	}
	c, err := cache.New(cfg)
	if err != nil {
		t.Skip("Skipping test: Redis not reachable on localhost:6379")
	}
	defer c.Close()

	ctx := context.Background()
	projectID := fmt.Sprintf("test-proj-%d", time.Now().UnixNano())
	counterName := "badges"
	key := cache.BuildKey("project", projectID, "counter", counterName)

	repo := &stubProjectRepo{}
	cnt := New(c, repo, time.Hour)

	if err := cnt.Set(ctx, projectID, counterName, 555); err != nil {
		t.Fatalf("cnt.Set() error = %v", err)
	}

	// Verify in repo
	if repo.counters[projectID+":"+counterName] != 555 {
		t.Fatalf("repo counter = %d, want 555", repo.counters[projectID+":"+counterName])
	}

	// Verify in Redis
	val, err := cnt.Get(ctx, projectID, counterName)
	if err != nil {
		t.Fatalf("cnt.Get() error = %v", err)
	}
	if val != 555 {
		t.Fatalf("cnt.Get() = %d, want 555", val)
	}

	_ = c.Delete(ctx, key)
	_ = c.Delete(ctx, cache.BuildKey("project", projectID, "counters"))
}
