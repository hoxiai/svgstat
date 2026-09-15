package project

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

type runtimeRepositoryStub struct {
	mu       sync.Mutex
	projects map[string]*Project
	loads    int
	list     []*Project
}

func (r *runtimeRepositoryStub) GetBySlug(_ context.Context, slug string) (*Project, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.loads++
	return cloneRuntimeProject(r.projects[slug]), nil
}

func (r *runtimeRepositoryStub) ListAll(context.Context) ([]*Project, error) {
	return r.list, nil
}

type runtimeBackendStub struct {
	mu     sync.Mutex
	values map[string]string
	writes int
	fail   bool
}

func (b *runtimeBackendStub) Get(_ context.Context, key string) (string, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.fail {
		return "", errors.New("backend unavailable")
	}
	value, ok := b.values[key]
	if !ok {
		return "", errors.New("cache miss")
	}
	return value, nil
}

func (b *runtimeBackendStub) Set(_ context.Context, key string, value interface{}, _ time.Duration) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.fail {
		return errors.New("backend unavailable")
	}
	b.values[key] = string(value.([]byte))
	b.writes++
	return nil
}

func (b *runtimeBackendStub) Delete(_ context.Context, keys ...string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	for _, key := range keys {
		delete(b.values, key)
	}
	return nil
}

func TestRuntimeCacheUsesMemoryAfterFirstLoad(t *testing.T) {
	repository := &runtimeRepositoryStub{projects: map[string]*Project{"demo": {ID: "1", Slug: "demo", WebsiteDomains: []string{"example.com"}}}}
	backend := &runtimeBackendStub{values: make(map[string]string)}
	cache := NewRuntimeCache(repository, backend, time.Minute)

	first, err := cache.GetBySlug(context.Background(), "demo")
	if err != nil || first == nil {
		t.Fatalf("first load = %#v, %v", first, err)
	}
	first.Name = "mutated"
	first.WebsiteDomains[0] = "evil.example"
	second, err := cache.GetBySlug(context.Background(), "demo")
	if err != nil || second.Name == "mutated" || second.WebsiteDomains[0] == "evil.example" {
		t.Fatalf("cached project was not isolated: %#v, %v", second, err)
	}
	if repository.loads != 1 {
		t.Fatalf("repository loads = %d, want 1", repository.loads)
	}
}

func TestRuntimeCacheCollapsesConcurrentMisses(t *testing.T) {
	repository := &runtimeRepositoryStub{projects: map[string]*Project{"demo": {ID: "1", Slug: "demo"}}}
	cache := NewRuntimeCache(repository, nil, time.Minute)

	var waitGroup sync.WaitGroup
	for range 20 {
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			project, err := cache.GetBySlug(context.Background(), "demo")
			if err != nil || project == nil {
				t.Errorf("GetBySlug() = %#v, %v", project, err)
			}
		}()
	}
	waitGroup.Wait()
	if repository.loads != 1 {
		t.Fatalf("repository loads = %d, want 1", repository.loads)
	}
}

func TestRuntimeCacheFallsBackWhenBackendFails(t *testing.T) {
	repository := &runtimeRepositoryStub{projects: map[string]*Project{"demo": {ID: "1", Slug: "demo"}}}
	backend := &runtimeBackendStub{values: make(map[string]string), fail: true}
	cache := NewRuntimeCache(repository, backend, time.Minute)

	project, err := cache.GetBySlug(context.Background(), "demo")
	if err != nil || project == nil {
		t.Fatalf("GetBySlug() = %#v, %v", project, err)
	}
}

func TestRuntimeCacheInvalidatesRenamedSlug(t *testing.T) {
	repository := &runtimeRepositoryStub{projects: map[string]*Project{}}
	backend := &runtimeBackendStub{values: make(map[string]string)}
	cache := NewRuntimeCache(repository, backend, time.Minute)
	if err := cache.Put(context.Background(), &Project{ID: "1", Slug: "old"}); err != nil {
		t.Fatal(err)
	}
	if err := cache.Invalidate(context.Background(), "old"); err != nil {
		t.Fatal(err)
	}
	project, err := cache.GetBySlug(context.Background(), "old")
	if err != nil || project != nil {
		t.Fatalf("invalidated project = %#v, %v", project, err)
	}
}

func TestRuntimeCacheCapsMemoryTTL(t *testing.T) {
	cache := NewRuntimeCache(&runtimeRepositoryStub{}, nil, 5*time.Minute)
	if cache.memoryTTL != 10*time.Second {
		t.Fatalf("memory TTL = %s, want 10s", cache.memoryTTL)
	}
	if cache.backendTTL != 5*time.Minute {
		t.Fatalf("backend TTL = %s, want 5m", cache.backendTTL)
	}
}

func TestRuntimeCacheHandlesRemoteInvalidation(t *testing.T) {
	repository := &runtimeRepositoryStub{projects: map[string]*Project{"demo": {ID: "1", Slug: "demo"}}}
	cache := NewRuntimeCache(repository, nil, time.Minute)
	if _, err := cache.GetBySlug(context.Background(), "demo"); err != nil {
		t.Fatal(err)
	}
	payload := `{"sender":"another-instance","slugs":["demo"]}`
	cache.handleInvalidation(payload)
	if _, err := cache.GetBySlug(context.Background(), "demo"); err != nil {
		t.Fatal(err)
	}
	if repository.loads != 2 {
		t.Fatalf("repository loads = %d, want 2", repository.loads)
	}
}

func TestRuntimeCacheIgnoresOwnInvalidation(t *testing.T) {
	cache := NewRuntimeCache(&runtimeRepositoryStub{}, nil, time.Minute)
	cache.storeMemory("demo", runtimeCacheRecord{Project: &Project{ID: "1", Slug: "demo"}}, time.Minute)
	payload := `{"sender":"` + cache.instanceID + `","slugs":["demo"]}`
	cache.handleInvalidation(payload)
	if project, _ := cache.GetBySlug(context.Background(), "demo"); project == nil {
		t.Fatal("own invalidation removed local write-through value")
	}
}
