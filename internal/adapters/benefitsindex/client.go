package benefitsindex

import (
	"context"
	"encoding/xml"
	"fmt"
	"net/http"
	"strings"

	"nwd-deakr/internal/domain"
)

type xmlResponse struct {
	Record domain.BenefitRecord `xml:"Record"`
}

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

func (c *Client) GetBenefit(ctx context.Context, ref string) (*domain.BenefitRecord, error) {
	if ref == "" {
		return nil, fmt.Errorf("benefit reference is empty")
	}

	upstreamURL := c.BaseURL + "/records/" + ref

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
		return nil, fmt.Errorf("request benefits register: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("benefit record not found")
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("benefits register returned HTTP %d", resp.StatusCode)
	}

	var result xmlResponse

	if err := xml.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode benefits response: %w", err)
	}

	return &result.Record, nil
}
