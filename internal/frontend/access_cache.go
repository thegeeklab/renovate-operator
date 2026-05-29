package frontend

import (
	"sync"
	"time"
)

const defaultAccessCacheTTL = 60 * time.Second

type accessCacheEntry struct {
	repos     map[string]bool
	expiresAt time.Time
}

type accessCache struct {
	mu      sync.Mutex
	entries map[string]accessCacheEntry
	ttl     time.Duration
	now     func() time.Time
}

func newAccessCache(ttl time.Duration) *accessCache {
	return &accessCache{
		entries: make(map[string]accessCacheEntry),
		ttl:     ttl,
		now:     time.Now,
	}
}

// get returns the cached repo set if fresh, otherwise (nil, false).
// Expired entries are evicted on access.
func (c *accessCache) get(key string) (map[string]bool, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	e, ok := c.entries[key]
	if !ok {
		return nil, false
	}
	if c.now().After(e.expiresAt) {
		delete(c.entries, key)
		return nil, false
	}

	return e.repos, true
}

func (c *accessCache) set(key string, repos map[string]bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.entries[key] = accessCacheEntry{
		repos:     repos,
		expiresAt: c.now().Add(c.ttl),
	}
}

func (c *accessCache) invalidate(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.entries, key)
}
