package orb

import (
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNewClient(t *testing.T) {
	c := NewClient("10.0.1.47", 8000)
	if c.host != "10.0.1.47" {
		t.Errorf("host = %q", c.host)
	}
	if c.port != 8000 {
		t.Errorf("port = %d", c.port)
	}
}

func TestBaseURL(t *testing.T) {
	c := NewClient("192.168.1.1", 9000)
	want := "http://192.168.1.1:9000"
	if got := c.BaseURL(); got != want {
		t.Errorf("BaseURL() = %q, want %q", got, want)
	}
}

func TestDatasetURL(t *testing.T) {
	c := NewClient("host", 8000)
	got := c.datasetURL("responsiveness_1s")
	want := "http://host:8000/api/v2/datasets/responsiveness_1s.json?id=orb-collector"
	if got != want {
		t.Errorf("datasetURL = %q, want %q", got, want)
	}
}

func TestFetchDatasetSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `[{"timestamp":1234}]`)
	}))
	defer srv.Close()

	// Parse the test server URL to get host/port
	c := &Client{
		host:       srv.Listener.Addr().(*net.TCPAddr).IP.String(),
		port:       srv.Listener.Addr().(*net.TCPAddr).Port,
		httpClient: srv.Client(),
	}

	data, err := c.fetchDataset("test")
	if err != nil {
		t.Fatalf("fetchDataset: %v", err)
	}
	if string(data) != `[{"timestamp":1234}]` {
		t.Errorf("data = %q", string(data))
	}
}

func TestFetchDatasetHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := &Client{
		host:       srv.Listener.Addr().(*net.TCPAddr).IP.String(),
		port:       srv.Listener.Addr().(*net.TCPAddr).Port,
		httpClient: srv.Client(),
	}

	_, err := c.fetchDataset("test")
	if err == nil {
		t.Fatal("expected error for 500 response")
	}
}

func TestFetchDatasetConnectionError(t *testing.T) {
	c := NewClient("192.0.2.1", 1) // non-routable address
	c.httpClient.Timeout = 100 * time.Millisecond

	_, err := c.fetchDataset("test")
	if err == nil {
		t.Fatal("expected error for unreachable host")
	}
}

func TestFetchResponsivenessRaw(t *testing.T) {
	called := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called++
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `[{"timestamp":1000,"latency_avg_us":5000}]`)
	}))
	defer srv.Close()

	c := &Client{
		host:       srv.Listener.Addr().(*net.TCPAddr).IP.String(),
		port:       srv.Listener.Addr().(*net.TCPAddr).Port,
		httpClient: srv.Client(),
	}

	records, rawRecords, ep, err := c.FetchResponsivenessRaw()
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("records = %d, want 1", len(records))
	}
	if records[0].Timestamp != 1000 {
		t.Errorf("timestamp = %d", records[0].Timestamp)
	}
	if len(rawRecords) != 1 {
		t.Errorf("rawRecords = %d", len(rawRecords))
	}
	if ep != "responsiveness_1s" {
		t.Errorf("endpoint = %q", ep)
	}
	// Should only call the first endpoint since it succeeded
	if called != 1 {
		t.Errorf("called = %d, want 1", called)
	}
}

func TestFetchWifiLinkRaw(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `[{"timestamp":2000,"rssi_avg":-55}]`)
	}))
	defer srv.Close()

	c := &Client{
		host:       srv.Listener.Addr().(*net.TCPAddr).IP.String(),
		port:       srv.Listener.Addr().(*net.TCPAddr).Port,
		httpClient: srv.Client(),
	}

	records, _, ep, err := c.FetchWifiLinkRaw()
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if len(records) != 1 || records[0].Timestamp != 2000 {
		t.Errorf("records = %v", records)
	}
	if ep != "wifi_link_1s" {
		t.Errorf("endpoint = %q", ep)
	}
}

func TestFetchScoresRaw(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `[{"timestamp":3000,"orb_score":85.5}]`)
	}))
	defer srv.Close()

	c := &Client{
		host:       srv.Listener.Addr().(*net.TCPAddr).IP.String(),
		port:       srv.Listener.Addr().(*net.TCPAddr).Port,
		httpClient: srv.Client(),
	}

	records, _, ep, err := c.FetchScoresRaw()
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if len(records) != 1 || records[0].OrbScore != 85.5 {
		t.Errorf("records = %v", records)
	}
	if ep != "scores_1s" {
		t.Errorf("endpoint = %q", ep)
	}
}

func TestFetchSpeedResultsRaw(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `[{"timestamp":4000,"download_kbps":50000}]`)
	}))
	defer srv.Close()

	c := &Client{
		host:       srv.Listener.Addr().(*net.TCPAddr).IP.String(),
		port:       srv.Listener.Addr().(*net.TCPAddr).Port,
		httpClient: srv.Client(),
	}

	records, _, ep, err := c.FetchSpeedResultsRaw()
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if len(records) != 1 {
		t.Errorf("records = %d", len(records))
	}
	if ep != "speed_results" {
		t.Errorf("endpoint = %q", ep)
	}
}

func TestFallbackToSecondEndpoint(t *testing.T) {
	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount == 1 {
			// First endpoint returns empty array
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `[]`)
			return
		}
		// Second endpoint has data
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `[{"timestamp":5000}]`)
	}))
	defer srv.Close()

	c := &Client{
		host:       srv.Listener.Addr().(*net.TCPAddr).IP.String(),
		port:       srv.Listener.Addr().(*net.TCPAddr).Port,
		httpClient: srv.Client(),
	}

	records, _, ep, err := c.FetchResponsivenessRaw()
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("records = %d, want 1", len(records))
	}
	if ep != "responsiveness_15s" {
		t.Errorf("endpoint = %q, want responsiveness_15s", ep)
	}
}

func TestAllEndpointsFail(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := &Client{
		host:       srv.Listener.Addr().(*net.TCPAddr).IP.String(),
		port:       srv.Listener.Addr().(*net.TCPAddr).Port,
		httpClient: srv.Client(),
	}

	records, rawRecords, ep, err := c.FetchResponsivenessRaw()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if records != nil {
		t.Errorf("expected nil records")
	}
	if rawRecords != nil {
		t.Errorf("expected nil rawRecords")
	}
	// Should return first endpoint name as default
	if ep != "responsiveness_1s" {
		t.Errorf("ep = %q", ep)
	}
}

func TestInvalidJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `not json`)
	}))
	defer srv.Close()

	c := &Client{
		host:       srv.Listener.Addr().(*net.TCPAddr).IP.String(),
		port:       srv.Listener.Addr().(*net.TCPAddr).Port,
		httpClient: srv.Client(),
	}

	records, _, _, err := c.FetchResponsivenessRaw()
	// Should not error — just return nil when all endpoints fail to parse
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if records != nil {
		t.Error("expected nil records for invalid JSON")
	}
}
