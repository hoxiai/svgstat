package project

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"

	"github.com/hoxiai/svgstat/internal/cache"
)

type runtimeCacheBackend interface {
	Get(ctx context.Context, key string) (string, error)
	Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error
	Delete(ctx context.Context, keys ...string) error
}

type runtimeInvalidationBackend interface {
	Publish(ctx context.Context, channel, message string) error
	Subscribe(ctx context.Context, channel string, handler func(string)) error
}

type runtimeCacheObserver interface {
	ObserveRuntimeCache(result string)
}

type runtimeProjectRepository interface {
	GetBySlug(ctx context.Context, slug string) (*Project, error)
	ListAll(ctx context.Context) ([]*Project, error)
}

type runtimeCacheRecord struct {
	Project *Project `json:"project,omitempty"`
	Missing bool     `json:"missing,omitempty"`
}

type runtimeMemoryEntry struct {
	record    runtimeCacheRecord
	expiresAt time.Time
}

type RuntimeCache struct {
	repository runtimeProjectRepository
	backend    runtimeCacheBackend
	backendTTL time.Duration
	memoryTTL  time.Duration
	memory     sync.Map
	loads      singleflight.Group
	instanceID string
	observer   runtimeCacheObserver
}

const runtimeInvalidationChannel = "svgstat:runtime:project:invalidate"

type invalidationMessage struct {
	Sender string   `json:"sender"`
	Slugs  []string `json:"slugs"`
}

func NewRuntimeCache(repository runtimeProjectRepository, backend runtimeCacheBackend, ttl time.Duration) *RuntimeCache {
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}
	memoryTTL := ttl
	if memoryTTL > 10*time.Second {
		memoryTTL = 10 * time.Second
	}
	return &RuntimeCache{repository: repository, backend: backend, backendTTL: ttl, memoryTTL: memoryTTL, instanceID: runtimeInstanceID()}
}

func (c *RuntimeCache) SetObserver(observer runtimeCacheObserver) { c.observer = observer }

func (c *RuntimeCache) StartInvalidationSubscriber(ctx context.Context) {
	backend, ok := c.backend.(runtimeInvalidationBackend)
	if !ok {
		return
	}
	go func() {
		for ctx.Err() == nil {
			err := backend.Subscribe(ctx, runtimeInvalidationChannel, c.handleInvalidation)
			if err == nil || ctx.Err() != nil {
				return
			}
			timer := time.NewTimer(time.Second)
			select {
			case <-ctx.Done():
				timer.Stop()
				return
			case <-timer.C:
			}
		}
	}()
}

func (c *RuntimeCache) GetBySlug(ctx context.Context, slug string) (*Project, error) {
	if record, ok := c.getMemory(slug, time.Now()); ok {
		c.observe("memory_hit")
		return cloneRuntimeProject(record.Project), nil
	}

	result, err, _ := c.loads.Do(slug, func() (interface{}, error) {
		if record, ok := c.getMemory(slug, time.Now()); ok {
			c.observe("memory_hit")
			return cloneRuntimeProject(record.Project), nil
		}
		return c.load(ctx, slug)
	})
	if err != nil || result == nil {
		return nil, err
	}
	return result.(*Project), nil
}

func (c *RuntimeCache) Warm(ctx context.Context) error {
	projects, err := c.repository.ListAll(ctx)
	if err != nil {
		return fmt.Errorf("failed to warm runtime project cache: %w", err)
	}
	for _, project := range projects {
		record := runtimeCacheRecord{Project: cloneRuntimeProject(project)}
		c.storeMemory(project.Slug, record, c.memoryTTL)
		_ = c.storeBackend(ctx, project.Slug, record, c.backendTTL)
	}
	return nil
}

func (c *RuntimeCache) Put(ctx context.Context, project *Project) error {
	if project == nil || project.Slug == "" {
		return nil
	}
	record := runtimeCacheRecord{Project: cloneRuntimeProject(project)}
	c.storeMemory(project.Slug, record, c.memoryTTL)
	if err := c.storeBackend(ctx, project.Slug, record, c.backendTTL); err != nil {
		return err
	}
	return c.broadcast(ctx, project.Slug)
}

func (c *RuntimeCache) Invalidate(ctx context.Context, slugs ...string) error {
	keys := make([]string, 0, len(slugs))
	for _, slug := range slugs {
		if slug == "" {
			continue
		}
		c.memory.Delete(slug)
		keys = append(keys, runtimeProjectCacheKey(slug))
	}
	if len(keys) == 0 || c.backend == nil {
		return c.broadcast(ctx, slugs...)
	}
	if err := c.backend.Delete(ctx, keys...); err != nil {
		return err
	}
	return c.broadcast(ctx, slugs...)
}

func (c *RuntimeCache) load(ctx context.Context, slug string) (*Project, error) {
	if c.backend != nil {
		encoded, err := c.backend.Get(ctx, runtimeProjectCacheKey(slug))
		if err == nil {
			var record runtimeCacheRecord
			if json.Unmarshal([]byte(encoded), &record) == nil {
				c.storeMemory(slug, record, c.memoryTTL)
				c.observe("redis_hit")
				return cloneRuntimeProject(record.Project), nil
			}
		}
	}

	project, err := c.repository.GetBySlug(ctx, slug)
	c.observe("postgres_fallback")
	if err != nil {
		return nil, err
	}
	if project == nil {
		ttl := c.negativeTTL()
		record := runtimeCacheRecord{Missing: true}
		c.storeMemory(slug, record, ttl)
		_ = c.storeBackend(ctx, slug, record, ttl)
		return nil, nil
	}

	record := runtimeCacheRecord{Project: cloneRuntimeProject(project)}
	c.storeMemory(slug, record, c.memoryTTL)
	_ = c.storeBackend(ctx, slug, record, c.backendTTL)
	return cloneRuntimeProject(project), nil
}

func (c *RuntimeCache) broadcast(ctx context.Context, slugs ...string) error {
	backend, ok := c.backend.(runtimeInvalidationBackend)
	if !ok {
		return nil
	}
	message, err := json.Marshal(invalidationMessage{Sender: c.instanceID, Slugs: slugs})
	if err != nil {
		return err
	}
	return backend.Publish(ctx, runtimeInvalidationChannel, string(message))
}

func (c *RuntimeCache) handleInvalidation(payload string) {
	var message invalidationMessage
	if json.Unmarshal([]byte(payload), &message) != nil || message.Sender == c.instanceID {
		return
	}
	for _, slug := range message.Slugs {
		c.memory.Delete(slug)
	}
}

func (c *RuntimeCache) observe(result string) {
	if c.observer != nil {
		c.observer.ObserveRuntimeCache(result)
	}
}

func runtimeInstanceID() string {
	value := make([]byte, 12)
	_, _ = rand.Read(value)
	return fmt.Sprintf("%x", value)
}

func (c *RuntimeCache) getMemory(slug string, now time.Time) (runtimeCacheRecord, bool) {
	value, ok := c.memory.Load(slug)
	if !ok {
		return runtimeCacheRecord{}, false
	}
	entry := value.(runtimeMemoryEntry)
	if !now.Before(entry.expiresAt) {
		c.memory.Delete(slug)
		return runtimeCacheRecord{}, false
	}
	return entry.record, true
}

func (c *RuntimeCache) storeMemory(slug string, record runtimeCacheRecord, ttl time.Duration) {
	record.Project = cloneRuntimeProject(record.Project)
	c.memory.Store(slug, runtimeMemoryEntry{record: record, expiresAt: time.Now().Add(ttl)})
}

func (c *RuntimeCache) storeBackend(ctx context.Context, slug string, record runtimeCacheRecord, ttl time.Duration) error {
	if c.backend == nil {
		return nil
	}
	encoded, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("failed to encode runtime project: %w", err)
	}
	if err := c.backend.Set(ctx, runtimeProjectCacheKey(slug), encoded, ttl); err != nil {
		return fmt.Errorf("failed to cache runtime project: %w", err)
	}
	return nil
}

func (c *RuntimeCache) negativeTTL() time.Duration {
	if c.memoryTTL < 10*time.Second {
		return c.memoryTTL
	}
	return 10 * time.Second
}

func runtimeProjectCacheKey(slug string) string {
	return cache.BuildKey("runtime", "project", "slug", slug)
}

func cloneRuntimeProject(project *Project) *Project {
	if project == nil {
		return nil
	}
	clone := *project
	clone.WebsiteDomains = append([]string(nil), project.WebsiteDomains...)
	return &clone
}
