package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"

	"github.com/tacomilkshake/orb-collector/internal/store"
)

// apiServer runs alongside the collector loop, sharing the same *store.Store.
type apiServer struct {
	store     *store.Store
	startTime time.Time
}

// startAPIServer starts the HTTP API server in a goroutine.
func startAPIServer(s *store.Store, port int) {
	srv := &apiServer{
		store:     s,
		startTime: time.Now(),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/begin", srv.handleBegin)
	mux.HandleFunc("POST /api/end", srv.handleEnd)
	mux.HandleFunc("GET /api/status", srv.handleStatus)
	mux.HandleFunc("GET /api/health", srv.handleHealth)

	addr := fmt.Sprintf("127.0.0.1:%d", port)
	fmt.Printf("[api] HTTP server listening on %s\n", addr)

	go func() {
		if err := http.ListenAndServe(addr, mux); err != nil {
			fmt.Printf("[api] HTTP server error: %s\n", err)
		}
	}()
}

// beginRequest is the JSON body for POST /api/begin.
type beginRequest struct {
	Name    string `json:"name"`
	Channel int    `json:"channel"`
	Width   int    `json:"width"`
	Freq    int    `json:"freq"`
}

// beginResponse is the JSON response for POST /api/begin.
type beginResponse struct {
	TestID    int64  `json:"test_id"`
	Name      string `json:"name"`
	StartedAt string `json:"started_at"`
}

func (srv *apiServer) handleBegin(w http.ResponseWriter, r *http.Request) {
	var req beginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpError(w, http.StatusBadRequest, "invalid JSON: %s", err)
		return
	}

	if req.Name == "" || req.Channel == 0 || req.Width == 0 || req.Freq == 0 {
		httpError(w, http.StatusBadRequest, "name, channel, width, and freq are required")
		return
	}

	// Auto-end any active test
	active, err := srv.store.GetActiveTest()
	if err != nil {
		httpError(w, http.StatusInternalServerError, "check active test: %s", err)
		return
	}
	if active != nil {
		if _, _, err := srv.store.EndTest(active.ID); err != nil {
			httpError(w, http.StatusInternalServerError, "end previous test: %s", err)
			return
		}
	}

	params := store.BeginTestParams{
		Name:     req.Name,
		Channel:  req.Channel,
		WidthMHz: req.Width,
		FreqMHz:  req.Freq,
	}
	if apConn != nil {
		params.APPlatform = apConn.Name()
	}

	id, err := srv.store.BeginTest(params)
	if err != nil {
		httpError(w, http.StatusInternalServerError, "begin test: %s", err)
		return
	}

	jsonResponse(w, http.StatusOK, beginResponse{
		TestID:    id,
		Name:      req.Name,
		StartedAt: time.Now().UTC().Format(time.RFC3339),
	})
}

// endResponse is the JSON response for POST /api/end.
type endResponse struct {
	TestID    int64          `json:"test_id"`
	Name      string         `json:"name"`
	DurationS float64        `json:"duration_s"`
	Samples   endSampleCount `json:"samples"`
}

type endSampleCount struct {
	Resp   int64 `json:"resp"`
	Wifi   int64 `json:"wifi"`
	Scores int64 `json:"scores"`
}

func (srv *apiServer) handleEnd(w http.ResponseWriter, r *http.Request) {
	active, err := srv.store.GetActiveTest()
	if err != nil {
		httpError(w, http.StatusInternalServerError, "check active test: %s", err)
		return
	}
	if active == nil {
		httpError(w, http.StatusNotFound, "no active test to end")
		return
	}

	_, _, err = srv.store.EndTest(active.ID)
	if err != nil {
		httpError(w, http.StatusInternalServerError, "end test: %s", err)
		return
	}

	duration := time.Since(active.StartTime)
	respCount, _ := srv.store.CountResponsiveness(active.ID)
	wifiCount, _ := srv.store.CountWifiLink(active.ID)
	scoresCount, _ := srv.store.CountScores(active.ID)

	jsonResponse(w, http.StatusOK, endResponse{
		TestID:    active.ID,
		Name:      active.Name,
		DurationS: duration.Seconds(),
		Samples: endSampleCount{
			Resp:   respCount,
			Wifi:   wifiCount,
			Scores: scoresCount,
		},
	})
}

// statusResponse is the JSON response for GET /api/status.
type statusResponse struct {
	CollectorRunning bool              `json:"collector_running"`
	ActiveTest       *statusActiveTest `json:"active_test,omitempty"`
	LatestReading    *statusReading    `json:"latest_reading,omitempty"`
	Totals           statusTotals      `json:"totals"`
}

type statusActiveTest struct {
	TestID   int64   `json:"test_id"`
	Name     string  `json:"name"`
	Channel  int     `json:"channel"`
	WidthMHz int     `json:"width_mhz"`
	ElapsedS float64 `json:"elapsed_s"`
	Samples  int64   `json:"samples"`
}

type statusReading struct {
	LatencyMS   *float64 `json:"latency_ms,omitempty"`
	JitterMS    *float64 `json:"jitter_ms,omitempty"`
	LossPct     *float64 `json:"loss_pct,omitempty"`
	NetworkName *string  `json:"network_name,omitempty"`
	AgeS        float64  `json:"age_s"`
}

type statusTotals struct {
	Tests  int64 `json:"tests"`
	Resp   int64 `json:"resp"`
	Wifi   int64 `json:"wifi"`
	Scores int64 `json:"scores"`
	Speed  int64 `json:"speed"`
}

func (srv *apiServer) handleStatus(w http.ResponseWriter, r *http.Request) {
	resp := statusResponse{
		CollectorRunning: true,
	}

	active, err := srv.store.GetActiveTest()
	if err == nil && active != nil {
		elapsed := time.Since(active.StartTime)
		respCount, _ := srv.store.CountResponsiveness(active.ID)
		resp.ActiveTest = &statusActiveTest{
			TestID:   active.ID,
			Name:     active.Name,
			Channel:  active.Channel,
			WidthMHz: active.WidthMHz,
			ElapsedS: elapsed.Seconds(),
			Samples:  respCount,
		}
	}

	latest, err := srv.store.GetLatestReading()
	if err == nil && latest != nil {
		age := time.Since(latest.OrbTimestamp)
		reading := &statusReading{
			AgeS: age.Seconds(),
		}
		if latest.LatencyAvgUS.Valid {
			v := float64(latest.LatencyAvgUS.Int64) / 1000.0
			reading.LatencyMS = &v
		}
		if latest.JitterAvgUS.Valid {
			v := float64(latest.JitterAvgUS.Int64) / 1000.0
			reading.JitterMS = &v
		}
		if latest.PacketLossPct.Valid {
			reading.LossPct = &latest.PacketLossPct.Float64
		}
		if latest.NetworkName.Valid {
			reading.NetworkName = &latest.NetworkName.String
		}
		resp.LatestReading = reading
	}

	tests, respN, wifi, speed, scores, _ := srv.store.TotalCounts()
	resp.Totals = statusTotals{
		Tests:  tests,
		Resp:   respN,
		Wifi:   wifi,
		Scores: scores,
		Speed:  speed,
	}

	jsonResponse(w, http.StatusOK, resp)
}

// healthResponse is the JSON response for GET /api/health.
type healthResponse struct {
	Status  string  `json:"status"`
	UptimeS float64 `json:"uptime_s"`
}

func (srv *apiServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	jsonResponse(w, http.StatusOK, healthResponse{
		Status:  "ok",
		UptimeS: time.Since(srv.startTime).Seconds(),
	})
}

// apiReachable checks whether the collector HTTP API is responding.
func apiReachable(port int) bool {
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("localhost:%d", port), 500*time.Millisecond)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

// apiURL returns the base URL for the collector HTTP API.
func apiURL(port int) string {
	return fmt.Sprintf("http://localhost:%d", port)
}

// proxyToAPI sends a request to the collector HTTP API and returns the response body.
func proxyToAPI(method, url string, body io.Reader) ([]byte, int, error) {
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, 0, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	return data, resp.StatusCode, err
}

func jsonResponse(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func httpError(w http.ResponseWriter, status int, format string, args ...any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	msg := fmt.Sprintf(format, args...)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
