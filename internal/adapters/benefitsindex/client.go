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

func (c *Client) GetBenefit(ctx context.Context, ref string) (*domain.BenefitRecord, domain.SourceStatus) {
	start := time.Now()

	status := domain.SourceStatus{
		Status: "unavailable",
	}

	if ref == "" {
		status.ErrorMessage = "benefit reference is empty"
		return nil, status
	}

	upstreamURL := c.BaseURL + "/records/" + ref

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

	var result xmlResponse

	if err := xml.NewDecoder(resp.Body).Decode(&result); err != nil {
		status.ErrorMessage = fmt.Sprintf("decode response: %v", err)
		return nil, status
	}

	status.Status = "ok"

	return &result.Record, status
}
