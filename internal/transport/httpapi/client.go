package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/prods/nvimon/internal/model"
)

type Client struct {
	BaseURL    string
	HTTPClient *http.Client
	AuthToken  string
}

func (c *Client) Snapshot(ctx context.Context) (model.HostSnapshot, error) {
	var snapshot model.HostSnapshot

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+"/v1/snapshot", nil)
	if err != nil {
		return snapshot, err
	}

	if c.AuthToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.AuthToken)
	}

	httpClient := c.HTTPClient
	if httpClient == nil {
		httpClient = http.DefaultClient
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return snapshot, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return snapshot, fmt.Errorf("snapshot request failed: status=%d", resp.StatusCode)
	}

	if err := json.NewDecoder(resp.Body).Decode(&snapshot); err != nil {
		return snapshot, err
	}

	return snapshot, nil
}
