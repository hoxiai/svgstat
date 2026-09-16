package counter

import (
	"context"
	"fmt"
	"time"

	"github.com/hoxiai/svgstat/internal/cache"
	"github.com/hoxiai/svgstat/internal/project"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"
)

type Counter struct {
	cache       *cache.Cache
	projectRepo project.Repository
	keyTTL      time.Duration
}

func New(cache *cache.Cache, projectRepo project.Repository, keyTTL time.Duration) *Counter {
	if keyTTL <= 0 {
		keyTTL = 72 * time.Hour
	}
	return &Counter{
		cache:       cache,
		projectRepo: projectRepo,
		keyTTL:      keyTTL,
	}
}

func (c *Counter) ensureLoaded(ctx context.Context, projectID, counterName string) {
	if c.projectRepo == nil {
		return
	}
	key := cache.BuildKey("project", projectID, "counter", counterName)
	exists, err := c.cache.Exists(ctx, key)
	if err != nil {
		log.Warn().Err(err).Str("project_id", projectID).Str("counter", counterName).Msg("Failed to check counter existence in cache")
		return
	}
	if !exists {
		dbVal, err := c.projectRepo.GetCounter(ctx, projectID, counterName)
		if err != nil {
			log.Warn().Err(err).Str("project_id", projectID).Str("counter", counterName).Msg("Failed to load counter from database")
			return
		}
		if dbVal > 0 {
			_, _ = c.cache.SetNX(ctx, key, fmt.Sprintf("%d", dbVal), 0)
		}
	}
	registryKey := cache.BuildKey("project", projectID, "counters")
	_ = c.cache.GetClient().SAdd(ctx, registryKey, counterName).Err()
}

func (c *Counter) Increment(ctx context.Context, projectID, counterName string) (int64, error) {
	c.ensureLoaded(ctx, projectID, counterName)

	key := cache.BuildKey("project", projectID, "counter", counterName)
	registryKey := cache.BuildKey("project", projectID, "counters")

	pipe := c.cache.Pipeline()
	incrCmd := pipe.Incr(ctx, key)
	pipe.SAdd(ctx, registryKey, counterName)

	if _, err := pipe.Exec(ctx); err != nil {
		return 0, fmt.Errorf("failed to increment counter: %w", err)
	}
	return incrCmd.Val(), nil
}

func (c *Counter) Get(ctx context.Context, projectID, counterName string) (int64, error) {
	key := cache.BuildKey("project", projectID, "counter", counterName)
	valueStr, err := c.cache.Get(ctx, key)
	if err != nil {
		if err == redis.Nil {
			if c.projectRepo != nil {
				dbVal, dbErr := c.projectRepo.GetCounter(ctx, projectID, counterName)
				if dbErr == nil && dbVal > 0 {
					_, _ = c.cache.SetNX(ctx, key, fmt.Sprintf("%d", dbVal), 0)
					registryKey := cache.BuildKey("project", projectID, "counters")
					_ = c.cache.GetClient().SAdd(ctx, registryKey, counterName).Err()
					return dbVal, nil
				}
			}
			return 0, nil
		}
		return 0, fmt.Errorf("failed to get counter: %w", err)
	}

	var value int64
	_, err = fmt.Sscanf(valueStr, "%d", &value)
	if err != nil {
		return 0, fmt.Errorf("failed to parse counter value: %w", err)
	}

	return value, nil
}

func (c *Counter) Set(ctx context.Context, projectID, counterName string, value int64) error {
	key := cache.BuildKey("project", projectID, "counter", counterName)
	registryKey := cache.BuildKey("project", projectID, "counters")

	pipe := c.cache.Pipeline()
	pipe.Set(ctx, key, fmt.Sprintf("%d", value), 0)
	pipe.SAdd(ctx, registryKey, counterName)
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("failed to set counter: %w", err)
	}

	if c.projectRepo != nil {
		if err := c.projectRepo.UpsertCounter(ctx, projectID, counterName, value); err != nil {
			log.Warn().Err(err).Str("project_id", projectID).Str("counter", counterName).Msg("Failed to persist counter to DB")
		}
	}

	return nil
}

func (c *Counter) IncrementWithAnalytics(ctx context.Context, projectID, counterName string) (int64, error) {
	c.ensureLoaded(ctx, projectID, counterName)

	key := cache.BuildKey("project", projectID, "counter", counterName)
	registryKey := cache.BuildKey("project", projectID, "counters")

	pipe := c.cache.Pipeline()
	incrCmd := pipe.Incr(ctx, key)
	pipe.SAdd(ctx, registryKey, counterName)

	// UTC day bucket, consistent with analytics keys and worker flush windows.
	date := time.Now().UTC().Format("2006-01-02")
	historyKey := cache.BuildKey("project", projectID, "counter", counterName, "history", date)
	pipe.Incr(ctx, historyKey)
	// The day-scoped history key expires like other analytics keys; the
	// cumulative counter key above stays permanent.
	pipe.Expire(ctx, historyKey, c.keyTTL)

	_, err := pipe.Exec(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to increment counter with analytics: %w", err)
	}

	return incrCmd.Val(), nil
}
