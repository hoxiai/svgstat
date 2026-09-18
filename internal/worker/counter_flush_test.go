package worker

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/hoxiai/svgstat/internal/analytics"
	"github.com/hoxiai/svgstat/internal/cache"
	"github.com/hoxiai/svgstat/internal/config"
	"github.com/hoxiai/svgstat/internal/counter"
	"github.com/hoxiai/svgstat/internal/project"
	"github.com/joho/godotenv"
)

type counterFlushRepo struct {
	project.Repository
	projects []*project.Project
	upserted map[string]int64
}

func (r *counterFlushRepo) ListAll(ctx context.Context) ([]*project.Project, error) {
	return r.projects, nil
}

func (r *counterFlushRepo) GetCounter(ctx context.Context, projectID, counterName string) (int64, error) {
	return 0, nil
}

func (r *counterFlushRepo) UpsertCounter(ctx context.Context, projectID, counterName string, value int64) error {
	r.upserted[counterName] = value
	return nil
}

func TestFlushCountersPersistsExactRegisteredNames(t *testing.T) {
	_ = godotenv.Load("../../.env")
	c, err := cache.New(config.Load())
	if err != nil {
		t.Skip("Skipping test: Redis not reachable on localhost:6379")
	}
	defer c.Close()

	ctx := context.Background()
	projectID := fmt.Sprintf("test-flush-%d", time.Now().UnixNano())
	repo := &counterFlushRepo{projects: []*project.Project{{ID: projectID}}, upserted: map[string]int64{}}
	cnt := counter.New(c, repo, time.Hour)
	names := []string{"views", "views@post:history:1"}
	for _, name := range names {
		if _, err := cnt.IncrementWithAnalytics(ctx, projectID, name); err != nil {
			t.Fatalf("IncrementWithAnalytics(%s) error = %v", name, err)
		}
	}
	registryKey := cache.BuildKey("project", projectID, "counters")
	// A registered name whose counter key already expired must be pruned.
	_ = c.GetClient().SAdd(ctx, registryKey, "expired").Err()
	defer func() {
		keys, _ := c.GetClient().Keys(ctx, cache.BuildKey("project", projectID, "*")).Result()
		if len(keys) > 0 {
			_ = c.Delete(ctx, keys...)
		}
	}()

	w := New(analytics.New(c, repo, nil, time.Hour, "salt"), repo, nil, time.Minute)
	if err := w.flushCounters(ctx, repo.projects); err != nil {
		t.Fatalf("flushCounters() error = %v", err)
	}

	if len(repo.upserted) != len(names) {
		t.Fatalf("upserted = %#v, want exactly %v", repo.upserted, names)
	}
	for _, name := range names {
		if repo.upserted[name] != 1 {
			t.Errorf("upserted[%q] = %d, want 1", name, repo.upserted[name])
		}
	}
	if member, _ := c.GetClient().SIsMember(ctx, registryKey, "expired").Result(); member {
		t.Error("expired counter name still registered, want pruned")
	}
}
