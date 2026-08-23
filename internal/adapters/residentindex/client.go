package residentindex

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

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

func (c *Client) GetResident(ctx context.Context, id string) (*domain.Resident, error) {
	if id == "" {
		return nil, fmt.Errorf("resident id is empty")
	}

	upstreamURL := c.BaseURL + "/residents/" + url.PathEscape(id)

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		upstreamURL,
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request resident index: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("resident not found")
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("resident index returned HTTP %d", resp.StatusCode)
	}

	var resident domain.Resident

	if err := json.NewDecoder(resp.Body).Decode(&resident); err != nil {
		return nil, fmt.Errorf("decode resident response: %w", err)
	}

	return &resident, nil
}
