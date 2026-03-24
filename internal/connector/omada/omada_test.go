package omada

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestName(t *testing.T) {
	c := New("http://localhost:8086")
	if c.Name() != "omada" {
		t.Errorf("Name() = %q", c.Name())
	}
}

func TestGetClient(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/client" {
			t.Errorf("path = %q", r.URL.Path)
		}
		mac := r.URL.Query().Get("mac")
		if mac != "AA:BB:CC:DD:EE:FF" {
			t.Errorf("mac = %q", mac)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"mac":    "AA:BB:CC:DD:EE:FF",
			"rssi":   -55,
			"snr":    30,
			"apName": "AP-01",
		})
	}))
	defer srv.Close()

	c := New(srv.URL)
	info, err := c.GetClient("AA:BB:CC:DD:EE:FF")
	if err != nil {
		t.Fatalf("GetClient: %v", err)
	}
	if info.MAC != "AA:BB:CC:DD:EE:FF" {
		t.Errorf("MAC = %q", info.MAC)
	}
	if info.RSSI == nil || *info.RSSI != -55 {
		t.Errorf("RSSI = %v", info.RSSI)
	}
	if info.Raw == nil {
		t.Error("expected non-nil Raw")
	}
}

func TestGetClientHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprint(w, `{"error":"not found"}`)
	}))
	defer srv.Close()

	c := New(srv.URL)
	_, err := c.GetClient("AA:BB:CC:DD:EE:FF")
	if err == nil {
		t.Fatal("expected error for 404")
	}
}

func TestGetClientConnectionError(t *testing.T) {
	c := New("http://192.0.2.1:1")
	c.httpClient.Timeout = 100 * time.Millisecond
	_, err := c.GetClient("AA:BB:CC:DD:EE:FF")
	if err == nil {
		t.Fatal("expected error for unreachable host")
	}
}

func TestGetAllClients(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/clients" {
			t.Errorf("path = %q", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"client_count": 2,
			"clients": []map[string]any{
				{"mac": "AA:BB:CC:DD:EE:01", "rssi": -50, "channel": 36},
				{"mac": "AA:BB:CC:DD:EE:02", "rssi": -60, "channel": 149},
			},
		})
	}))
	defer srv.Close()

	c := New(srv.URL)
	clients, err := c.GetAllClients()
	if err != nil {
		t.Fatalf("GetAllClients: %v", err)
	}
	if len(clients) != 2 {
		t.Fatalf("clients = %d, want 2", len(clients))
	}
	if clients[0].MAC != "AA:BB:CC:DD:EE:01" {
		t.Errorf("MAC[0] = %q", clients[0].MAC)
	}
	if clients[1].MAC != "AA:BB:CC:DD:EE:02" {
		t.Errorf("MAC[1] = %q", clients[1].MAC)
	}
	if clients[0].RSSI == nil || *clients[0].RSSI != -50 {
		t.Errorf("RSSI[0] = %v", clients[0].RSSI)
	}
	if clients[0].Raw == nil {
		t.Error("expected non-nil Raw")
	}
}

func TestGetAllClientsHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, `error`)
	}))
	defer srv.Close()

	c := New(srv.URL)
	_, err := c.GetAllClients()
	if err == nil {
		t.Fatal("expected error for 500")
	}
}

func TestGetAllClientsInvalidJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `not json`)
	}))
	defer srv.Close()

	c := New(srv.URL)
	_, err := c.GetAllClients()
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestGetAllClientsEmpty(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"client_count": 0,
			"clients":      []map[string]any{},
		})
	}))
	defer srv.Close()

	c := New(srv.URL)
	clients, err := c.GetAllClients()
	if err != nil {
		t.Fatalf("GetAllClients: %v", err)
	}
	if len(clients) != 0 {
		t.Errorf("clients = %d, want 0", len(clients))
	}
}
