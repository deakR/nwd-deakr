package benefitsindex

import (
	"context"
	"encoding/xml"
	"fmt"
	"net/http"
	"strings"
	"time"

	"nwd-deakr/internal/domain"
)

const (
	defaultMaxAttempts  = 3
	defaultRetryBackoff = 50 * time.Millisecond
)

type xmlResponse struct {
	Record domain.BenefitRecord `xml:"Record"`
}

type Client struct {
	BaseURL      string
	HTTPClient   *http.Client
	MaxAttempts  int
	RetryBackoff time.Duration
	breaker      *CircuitBreaker
	cache        *Cache
}

func NewClient(baseURL string, httpClient *http.Client) *Client {
	return &Client{
		BaseURL:      strings.TrimRight(baseURL, "/"),
		HTTPClient:   httpClient,
		MaxAttempts:  defaultMaxAttempts,
		RetryBackoff: defaultRetryBackoff,
		breaker:      NewCircuitBreaker(defaultFailureLimit, defaultCooldown),
		cache:        NewCache(defaultCacheTTL),
	}
}

func (c *Client) GetBenefit(ctx context.Context, ref string) (*domain.BenefitRecord, domain.SourceStatus) {
	start := time.Now()

	if ref == "" {
		return nil, domain.SourceStatus{
			Status:       "unavailable",
			ErrorMessage: "benefit reference is empty",
			LatencyMs:    time.Since(start).Milliseconds(),
		}
	}

	if record, found, stale := c.cache.get(ref); found && !stale {
		return record, domain.SourceStatus{
			Status:    "ok",
			LatencyMs: time.Since(start).Milliseconds(),
		}
	}

	if allowed, _ := c.breaker.Allow(); !allowed {
		if record, found, _ := c.cache.get(ref); found {
			return record, domain.SourceStatus{
				Status:       "stale",
				ErrorMessage: "benefits register circuit open; serving cached record",
				LatencyMs:    time.Since(start).Milliseconds(),
			}
		}

		return nil, domain.SourceStatus{
			Status:       "circuit_open",
			ErrorMessage: "benefits register circuit open",
			LatencyMs:    time.Since(start).Milliseconds(),
		}
	}

	record, status := c.fetchBenefit(ctx, ref)

	switch status.Status {
	case "ok":
		c.cache.set(ref, record)
		c.breaker.RecordSuccess()
	case "not_found":
		c.cache.drop(ref)
	default:
		c.breaker.RecordFailure()

		if cached, found, _ := c.cache.get(ref); found {
			return cached, domain.SourceStatus{
				Status:       "stale",
				ErrorMessage: "benefits register unavailable; serving cached record",
				LatencyMs:    time.Since(start).Milliseconds(),
			}
		}
	}

	return record, status
}

func (c *Client) fetchBenefit(ctx context.Context, ref string) (*domain.BenefitRecord, domain.SourceStatus) {
	start := time.Now()

	status := domain.SourceStatus{
		Status: "unavailable",
	}

	upstreamURL := c.BaseURL + "/records/" + ref

	maxAttempts := c.MaxAttempts
	if maxAttempts <= 0 {
		maxAttempts = defaultMaxAttempts
	}

	backoff := c.RetryBackoff
	if backoff <= 0 {
		backoff = defaultRetryBackoff
	}

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		req, err := http.NewRequestWithContext(
			ctx,
			http.MethodGet,
			upstreamURL,
			nil,
		)
		if err != nil {
			status.ErrorMessage = err.Error()
			status.LatencyMs = time.Since(start).Milliseconds()
			return nil, status
		}

		resp, err := c.HTTPClient.Do(req)
		if err != nil {
			status.ErrorMessage = fmt.Sprintf("attempt %d/%d: %v", attempt, maxAttempts, err)
			if attempt < maxAttempts {
				select {
				case <-ctx.Done():
					status.ErrorMessage = ctx.Err().Error()
					status.LatencyMs = time.Since(start).Milliseconds()
					return nil, status
				case <-time.After(backoff):
					continue
				}
			}
			status.LatencyMs = time.Since(start).Milliseconds()
			return nil, status
		}

		status.HTTPCode = resp.StatusCode

		if resp.StatusCode == http.StatusNotFound {
			resp.Body.Close()
			status.Status = "not_found"
			status.LatencyMs = time.Since(start).Milliseconds()
			return nil, status
		}

		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			status.ErrorMessage = fmt.Sprintf("attempt %d/%d: upstream returned HTTP %d", attempt, maxAttempts, resp.StatusCode)
			if attempt < maxAttempts {
				select {
				case <-ctx.Done():
					status.ErrorMessage = ctx.Err().Error()
					status.LatencyMs = time.Since(start).Milliseconds()
					return nil, status
				case <-time.After(backoff):
					continue
				}
			}
			status.LatencyMs = time.Since(start).Milliseconds()
			return nil, status
		}

		var result xmlResponse
		err = xml.NewDecoder(resp.Body).Decode(&result)
		resp.Body.Close()

		if err != nil {
			status.ErrorMessage = fmt.Sprintf("decode response: %v", err)
			status.LatencyMs = time.Since(start).Milliseconds()
			return nil, status
		}

		status.Status = "ok"
		status.ErrorMessage = ""
		status.LatencyMs = time.Since(start).Milliseconds()
		return &result.Record, status
	}

	status.LatencyMs = time.Since(start).Milliseconds()
	return nil, status
}
