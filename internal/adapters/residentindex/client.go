package residentindex

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"nwd-deakr/internal/domain"
)

const (
	defaultPageSize = 25
	defaultMaxPages = 100
)

type Client struct {
	BaseURL    string
	HTTPClient *http.Client

	MaxPages int
}

func NewClient(baseURL string, httpClient *http.Client) *Client {
	return &Client{
		BaseURL:    strings.TrimRight(baseURL, "/"),
		HTTPClient: httpClient,
	}
}

func (c *Client) GetResident(ctx context.Context, id string) (*domain.Resident, domain.SourceStatus) {
	start := time.Now()

	status := domain.SourceStatus{
		Status: "unavailable",
	}

	if id == "" {
		status.ErrorMessage = "resident id is empty"
		return nil, status
	}

	upstreamURL := c.BaseURL + "/residents/" + url.PathEscape(id)

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
		status.ErrorMessage = err.Error()
		status.LatencyMs = time.Since(start).Milliseconds()
		return nil, status
	}
	defer resp.Body.Close()

	status.HTTPCode = resp.StatusCode
	status.LatencyMs = time.Since(start).Milliseconds()

	if resp.StatusCode == http.StatusNotFound {
		status.Status = "not_found"
		return nil, status
	}

	if resp.StatusCode != http.StatusOK {
		status.ErrorMessage = fmt.Sprintf("upstream returned HTTP %d", resp.StatusCode)
		return nil, status
	}

	var resident domain.Resident

	if err := json.NewDecoder(resp.Body).Decode(&resident); err != nil {
		status.ErrorMessage = fmt.Sprintf("decode response: %v", err)
		return nil, status
	}

	status.Status = "ok"

	return &resident, status
}

func (c *Client) GetResidents(ctx context.Context) ([]domain.Resident, domain.PaginationStatus, domain.SourceStatus) {
	start := time.Now()

	status := domain.SourceStatus{
		Status: "ok",
	}
	pagination := domain.PaginationStatus{}

	residents := make([]domain.Resident, 0)
	byID := make(map[string]domain.Resident)

	maxPages := defaultMaxPages
	if c.MaxPages > 0 {
		maxPages = c.MaxPages
	}

	var failReason string
	finished := false

	for page := 1; page <= maxPages; page++ {
		upstreamURL := fmt.Sprintf(
			"%s/residents?page=%d&page_size=%d",
			c.BaseURL,
			page,
			defaultPageSize,
		)

		req, err := http.NewRequestWithContext(
			ctx,
			http.MethodGet,
			upstreamURL,
			nil,
		)
		if err != nil {
			status.Status = "unavailable"
			status.ErrorMessage = fmt.Sprintf("build request for page %d: %v", page, err)
			failReason = "request_creation_failed"
			break
		}

		resp, err := c.HTTPClient.Do(req)
		if err != nil {
			status.Status = "unavailable"
			status.ErrorMessage = fmt.Sprintf("fetch page %d: %v", page, err)
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				failReason = "context_canceled"
			} else {
				failReason = "upstream_failure"
			}
			break
		}

		status.HTTPCode = resp.StatusCode

		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			status.Status = "unavailable"
			status.ErrorMessage = fmt.Sprintf(
				"upstream returned HTTP %d on page %d",
				resp.StatusCode,
				page,
			)
			failReason = "upstream_failure"
			break
		}

		var result domain.ResidentPage

		err = json.NewDecoder(resp.Body).Decode(&result)
		resp.Body.Close()

		if err != nil {
			status.Status = "unavailable"
			status.ErrorMessage = fmt.Sprintf("decode page %d: %v", page, err)
			failReason = "invalid_response"
			break
		}

		pagination.PagesFetched++
		pagination.RecordsSeen += len(result.Results)
		pagination.ReportedTotal = result.Total

		if len(result.Results) == 0 && result.HasMore {
			status.Status = "unavailable"
			status.ErrorMessage = fmt.Sprintf(
				"empty page %d while has_more is true",
				page,
			)
			failReason = "pagination_anomaly"
			break
		}

		for _, resident := range result.Results {
			existing, exists := byID[resident.ID]
			if exists {
				pagination.Duplicates++
				if existing != resident {
					pagination.Conflicts++
				}
				continue
			}

			byID[resident.ID] = resident
			residents = append(residents, resident)
		}

		if !result.HasMore {
			finished = true
			break
		}
	}

	pagination.Unique = len(residents)

	switch {
	case failReason != "":
		pagination.Complete = false
		pagination.Reason = failReason
	case !finished:
		status.Status = "unavailable"
		status.ErrorMessage = fmt.Sprintf(
			"maximum pagination limit of %d pages reached",
			maxPages,
		)
		pagination.Complete = false
		pagination.Reason = "max_pages_reached"
	default:
		if pagination.Unique == pagination.ReportedTotal && pagination.ReportedTotal > 0 {
			pagination.Complete = true
		} else {
			pagination.Complete = false
			pagination.Reason = "total_mismatch"
		}
	}

	status.LatencyMs = time.Since(start).Milliseconds()

	return residents, pagination, status
}
