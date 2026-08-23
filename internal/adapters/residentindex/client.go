package residentindex

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"nwd-deakr/internal/domain"
)

type Client struct {
	BaseURL    string
	HTTPClient *http.Client
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
