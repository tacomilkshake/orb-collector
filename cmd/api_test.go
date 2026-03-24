package cmd

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/tacomilkshake/orb-collector/internal/store"
)

func newTestAPIServer(t *testing.T) (*apiServer, *store.Store) {
	t.Helper()
	s, err := store.New(":memory:")
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	srv := &apiServer{store: s, startTime: time.Now()}
	return srv, s
}

func TestHandleHealth(t *testing.T) {
	srv, _ := newTestAPIServer(t)
	req := httptest.NewRequest("GET", "/api/health", nil)
	w := httptest.NewRecorder()

	srv.handleHealth(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d", w.Code)
	}
	var resp healthResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if resp.Status != "ok" {
		t.Errorf("status = %q", resp.Status)
	}
	if resp.UptimeS < 0 {
		t.Errorf("uptime = %f", resp.UptimeS)
	}
}

func TestHandleStatusEmpty(t *testing.T) {
	srv, _ := newTestAPIServer(t)
	req := httptest.NewRequest("GET", "/api/status", nil)
	w := httptest.NewRecorder()

	srv.handleStatus(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d", w.Code)
	}
	var resp statusResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if !resp.CollectorRunning {
		t.Error("expected collector_running = true")
	}
	if resp.ActiveTest != nil {
		t.Error("expected no active test")
	}
	if resp.Totals.Tests != 0 {
		t.Errorf("tests = %d", resp.Totals.Tests)
	}
}

func TestHandleBeginAndEnd(t *testing.T) {
	srv, _ := newTestAPIServer(t)

	// Begin a test
	body, _ := json.Marshal(beginRequest{
		Name: "api_test", Channel: 36, Width: 80, Freq: 5180,
	})
	req := httptest.NewRequest("POST", "/api/begin", bytes.NewReader(body))
	w := httptest.NewRecorder()
	srv.handleBegin(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("begin status = %d, body = %s", w.Code, w.Body.String())
	}
	var beginResp beginResponse
	json.Unmarshal(w.Body.Bytes(), &beginResp)
	if beginResp.TestID <= 0 {
		t.Errorf("test_id = %d", beginResp.TestID)
	}
	if beginResp.Name != "api_test" {
		t.Errorf("name = %q", beginResp.Name)
	}

	// Status should show active test
	req = httptest.NewRequest("GET", "/api/status", nil)
	w = httptest.NewRecorder()
	srv.handleStatus(w, req)
	var status statusResponse
	json.Unmarshal(w.Body.Bytes(), &status)
	if status.ActiveTest == nil {
		t.Fatal("expected active test")
	}
	if status.ActiveTest.Name != "api_test" {
		t.Errorf("active name = %q", status.ActiveTest.Name)
	}

	// End the test
	req = httptest.NewRequest("POST", "/api/end", nil)
	w = httptest.NewRecorder()
	srv.handleEnd(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("end status = %d, body = %s", w.Code, w.Body.String())
	}
	var endResp endResponse
	json.Unmarshal(w.Body.Bytes(), &endResp)
	if endResp.TestID != beginResp.TestID {
		t.Errorf("end test_id = %d, want %d", endResp.TestID, beginResp.TestID)
	}
}

func TestHandleBeginInvalidJSON(t *testing.T) {
	srv, _ := newTestAPIServer(t)
	req := httptest.NewRequest("POST", "/api/begin", bytes.NewReader([]byte("not json")))
	w := httptest.NewRecorder()
	srv.handleBegin(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestHandleBeginMissingFields(t *testing.T) {
	srv, _ := newTestAPIServer(t)
	body, _ := json.Marshal(beginRequest{Name: "test"}) // missing channel/width/freq
	req := httptest.NewRequest("POST", "/api/begin", bytes.NewReader(body))
	w := httptest.NewRecorder()
	srv.handleBegin(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestHandleEndNoActiveTest(t *testing.T) {
	srv, _ := newTestAPIServer(t)
	req := httptest.NewRequest("POST", "/api/end", nil)
	w := httptest.NewRecorder()
	srv.handleEnd(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", w.Code)
	}
}

func TestHandleBeginAutoEndsPrevious(t *testing.T) {
	srv, _ := newTestAPIServer(t)

	// Begin first test
	body, _ := json.Marshal(beginRequest{Name: "test1", Channel: 36, Width: 80, Freq: 5180})
	req := httptest.NewRequest("POST", "/api/begin", bytes.NewReader(body))
	w := httptest.NewRecorder()
	srv.handleBegin(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("first begin: %d", w.Code)
	}
	var first beginResponse
	json.Unmarshal(w.Body.Bytes(), &first)

	// Begin second test (should auto-end first)
	body, _ = json.Marshal(beginRequest{Name: "test2", Channel: 149, Width: 160, Freq: 5745})
	req = httptest.NewRequest("POST", "/api/begin", bytes.NewReader(body))
	w = httptest.NewRecorder()
	srv.handleBegin(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("second begin: %d", w.Code)
	}
	var second beginResponse
	json.Unmarshal(w.Body.Bytes(), &second)

	if second.TestID == first.TestID {
		t.Error("second test should have different ID")
	}

	// First test should be ended (not active)
	test, _ := srv.store.GetTest(first.TestID)
	if test == nil || !test.EndTime.Valid {
		t.Error("first test should have end_time")
	}
}

func TestJsonResponse(t *testing.T) {
	w := httptest.NewRecorder()
	jsonResponse(w, http.StatusCreated, map[string]string{"key": "value"})

	if w.Code != http.StatusCreated {
		t.Errorf("status = %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("content-type = %q", ct)
	}
	var m map[string]string
	json.Unmarshal(w.Body.Bytes(), &m)
	if m["key"] != "value" {
		t.Errorf("body = %v", m)
	}
}

func TestHttpError(t *testing.T) {
	w := httptest.NewRecorder()
	httpError(w, http.StatusBadRequest, "bad %s", "input")

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d", w.Code)
	}
	var m map[string]string
	json.Unmarshal(w.Body.Bytes(), &m)
	if m["error"] != "bad input" {
		t.Errorf("error = %q", m["error"])
	}
}

func TestAPIReachable(t *testing.T) {
	// No server running on this port
	if apiReachable(59999) {
		t.Error("expected false for unused port")
	}
}

func TestAPIURL(t *testing.T) {
	got := apiURL(8080)
	want := "http://localhost:8080"
	if got != want {
		t.Errorf("apiURL = %q, want %q", got, want)
	}
}
