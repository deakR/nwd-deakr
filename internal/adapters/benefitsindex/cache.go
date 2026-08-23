package benefitsindex

import (
	"sync"
	"time"

	"nwd-deakr/internal/domain"
)

const defaultCacheTTL = 5 * time.Minute

type cacheEntry struct {
	record    *domain.BenefitRecord
	fetchedAt time.Time
}

type Cache struct {
	mu      sync.RWMutex
	ttl     time.Duration
	entries map[string]*cacheEntry
}

func NewCache(ttl time.Duration) *Cache {
	if ttl <= 0 {
		ttl = defaultCacheTTL
	}

	return &Cache{
		ttl:     ttl,
		entries: make(map[string]*cacheEntry),
	}
}

func (c *Cache) get(ref string) (*domain.BenefitRecord, bool, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, found := c.entries[ref]
	if !found {
		return nil, false, false
	}

	return entry.record, true, time.Since(entry.fetchedAt) > c.ttl
}

func (c *Cache) set(ref string, record *domain.BenefitRecord) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.entries[ref] = &cacheEntry{
		record:    record,
		fetchedAt: time.Now(),
	}
}

func (c *Cache) drop(ref string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.entries, ref)
}
