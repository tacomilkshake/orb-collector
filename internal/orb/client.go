package orb

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	defaultTimeout = 5 * time.Second
	callerID       = "orb-collector"
)

// Client communicates with the Orb local API.
type Client struct {
	host       string
	port       int
	httpClient *http.Client
}

// NewClient creates an Orb API client.
func NewClient(host string, port int) *Client {
	return &Client{
		host: host,
		port: port,
		httpClient: &http.Client{
			Timeout: defaultTimeout,
		},
	}
}

// datasetURL builds the Orb dataset URL.
func (c *Client) datasetURL(dataset string) string {
	return fmt.Sprintf("http://%s:%d/api/v2/datasets/%s.json?id=%s", c.host, c.port, dataset, callerID)
}

// fetchDataset fetches a dataset and returns the raw JSON bytes.
func (c *Client) fetchDataset(dataset string) ([]byte, error) {
	req, err := http.NewRequest("GET", c.datasetURL(dataset), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	return io.ReadAll(resp.Body)
}

// ResponsivenessEndpoints lists endpoints to try in preference order.
var ResponsivenessEndpoints = []string{"responsiveness_1s", "responsiveness_15s"}

// WifiLinkEndpoints lists endpoints to try in preference order.
var WifiLinkEndpoints = []string{"wifi_link_1s", "wifi_link_15s"}

// ScoresEndpoints lists endpoints to try in preference order.
var ScoresEndpoints = []string{"scores_1s", "scores_1m"}

// SpeedResultsEndpoints lists endpoints to try in preference order.
var SpeedResultsEndpoints = []string{"speed_results"}

// FetchResponsiveness tries endpoints in order and returns the first non-empty result.
func (c *Client) FetchResponsiveness() ([]ResponsivenessRecord, json.RawMessage, string, error) {
	return fetchWithFallback[ResponsivenessRecord](c, ResponsivenessEndpoints)
}

// FetchWifiLink tries endpoints in order and returns the first non-empty result.
func (c *Client) FetchWifiLink() ([]WifiLinkRecord, json.RawMessage, string, error) {
	return fetchWithFallback[WifiLinkRecord](c, WifiLinkEndpoints)
}

// fetchWithFallback tries each endpoint and returns the first with data.
// Returns typed records, the raw JSON array, and the endpoint name used.
func fetchWithFallback[T any](c *Client, endpoints []string) ([]T, json.RawMessage, string, error) {
	for _, ep := range endpoints {
		data, err := c.fetchDataset(ep)
		if err != nil {
			continue
		}

		// Parse as raw array first
		var rawArray []json.RawMessage
		if err := json.Unmarshal(data, &rawArray); err != nil {
			continue
		}
		if len(rawArray) == 0 {
			continue
		}

		// Parse typed records
		var records []T
		if err := json.Unmarshal(data, &records); err != nil {
			continue
		}

		return records, data, ep, nil
	}
	return nil, nil, endpoints[0], nil
}

// FetchRawRecords fetches a dataset and returns each record as raw JSON + its typed form.
func (c *Client) FetchResponsivenessRaw() ([]ResponsivenessRecord, []json.RawMessage, string, error) {
	return fetchRawWithFallback[ResponsivenessRecord](c, ResponsivenessEndpoints)
}

// FetchWifiLinkRaw fetches wifi_link with raw JSON per record.
func (c *Client) FetchWifiLinkRaw() ([]WifiLinkRecord, []json.RawMessage, string, error) {
	return fetchRawWithFallback[WifiLinkRecord](c, WifiLinkEndpoints)
}

// FetchScores tries endpoints in order and returns the first non-empty result.
func (c *Client) FetchScores() ([]ScoresRecord, json.RawMessage, string, error) {
	return fetchWithFallback[ScoresRecord](c, ScoresEndpoints)
}

// FetchScoresRaw fetches scores with raw JSON per record.
func (c *Client) FetchScoresRaw() ([]ScoresRecord, []json.RawMessage, string, error) {
	return fetchRawWithFallback[ScoresRecord](c, ScoresEndpoints)
}

// FetchSpeedResults fetches the speed_results dataset.
func (c *Client) FetchSpeedResults() ([]SpeedResultsRecord, json.RawMessage, string, error) {
	return fetchWithFallback[SpeedResultsRecord](c, SpeedResultsEndpoints)
}

// FetchSpeedResultsRaw fetches speed_results with raw JSON per record.
func (c *Client) FetchSpeedResultsRaw() ([]SpeedResultsRecord, []json.RawMessage, string, error) {
	return fetchRawWithFallback[SpeedResultsRecord](c, SpeedResultsEndpoints)
}

func fetchRawWithFallback[T any](c *Client, endpoints []string) ([]T, []json.RawMessage, string, error) {
	for _, ep := range endpoints {
		data, err := c.fetchDataset(ep)
		if err != nil {
			continue
		}

		var rawArray []json.RawMessage
		if err := json.Unmarshal(data, &rawArray); err != nil {
			continue
		}
		if len(rawArray) == 0 {
			continue
		}

		var records []T
		if err := json.Unmarshal(data, &records); err != nil {
			continue
		}

		return records, rawArray, ep, nil
	}
	return nil, nil, endpoints[0], nil
}
