// Package omada implements the APConnector interface via the omada-bridge REST API.
package omada

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/tacomilkshake/orb-optimizer/internal/connector"
)

// Connector talks to omada-bridge's REST API.
type Connector struct {
	baseURL    string
	httpClient *http.Client
}

// New creates an Omada connector. baseURL is the omada-bridge REST base, e.g. "http://omada-bridge:8086".
func New(baseURL string) *Connector {
	return &Connector{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

// Name returns the connector name.
func (c *Connector) Name() string { return "omada" }

// GetClient fetches client stats from omada-bridge REST API.
func (c *Connector) GetClient(mac string) (*connector.ClientInfo, error) {
	url := fmt.Sprintf("%s/api/client?mac=%s", c.baseURL, mac)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("omada: build request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("omada: request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("omada: read body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("omada: HTTP %d: %s", resp.StatusCode, string(body))
	}

	var raw map[string]any
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("omada: parse JSON: %w", err)
	}

	var info connector.ClientInfo
	if err := json.Unmarshal(body, &info); err != nil {
		return nil, fmt.Errorf("omada: parse client info: %w", err)
	}
	info.Raw = raw

	return &info, nil
}

// allClientsResponse is the JSON shape of GET /api/clients.
type allClientsResponse struct {
	ClientCount int              `json:"client_count"`
	Clients     []map[string]any `json:"clients"`
}

// GetAllClients fetches all wireless clients from omada-bridge REST API.
func (c *Connector) GetAllClients() ([]connector.ClientInfo, error) {
	url := fmt.Sprintf("%s/api/clients", c.baseURL)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("omada: build request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("omada: request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("omada: read body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("omada: HTTP %d: %s", resp.StatusCode, string(body))
	}

	var parsed allClientsResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("omada: parse JSON: %w", err)
	}

	clients := make([]connector.ClientInfo, 0, len(parsed.Clients))
	for _, raw := range parsed.Clients {
		// Re-marshal each client map so we can unmarshal into ClientInfo
		b, err := json.Marshal(raw)
		if err != nil {
			continue
		}
		var info connector.ClientInfo
		if err := json.Unmarshal(b, &info); err != nil {
			continue
		}
		info.Raw = raw
		clients = append(clients, info)
	}

	return clients, nil
}
