package benefitsindex

import (
	"context"
	"sync"
	"time"

	"nwd-deakr/internal/domain"
)

type Catalogue struct {
	mu        sync.Mutex
	client    *Client
	ttl       time.Duration
	snapshot  []domain.BenefitRecord
	fetchedAt time.Time
	hasData   bool
}

func NewCatalogue(client *Client, ttl time.Duration) *Catalogue {
	if ttl <= 0 {
		ttl = defaultCacheTTL
	}

	return &Catalogue{
		client: client,
		ttl:    ttl,
	}
}

func (c *Catalogue) Get(ctx context.Context) ([]domain.BenefitRecord, bool, int64) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.hasData && time.Since(c.fetchedAt) <= c.ttl {
		return c.snapshot, true, c.fetchedAt.UnixMilli()
	}

	records, status := c.client.GetAllBenefits(ctx)
	if status.Status == "ok" {
		c.snapshot = records
		c.fetchedAt = time.Now()
		c.hasData = true
		return c.snapshot, true, c.fetchedAt.UnixMilli()
	}

	if c.hasData {
		return c.snapshot, false, c.fetchedAt.UnixMilli()
	}

	return nil, false, 0
}
