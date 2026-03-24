package cmd

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/tacomilkshake/orb-collector/internal/orb"
	"github.com/tacomilkshake/orb-collector/internal/store"
)

// setupTestDB sets the package-level db to an in-memory store and returns cleanup.
func setupTestDB(t *testing.T) *store.Store {
	t.Helper()
	s, err := store.New(":memory:")
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	db = s
	apConn = nil
	orbServer = ""
	dbPath = ":memory:"
	orbTargets = []OrbTarget{{Client: orb.NewClient("127.0.0.1", 8000), DeviceID: "127.0.0.1"}}
	t.Cleanup(func() {
		db = nil
		s.Close()
	})
	return s
}

// captureStdout captures stdout during fn execution and returns the output.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = w

	fn()

	w.Close()
	os.Stdout = old

	out, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("read pipe: %v", err)
	}
	return string(out)
}

// fakeCmd creates a minimal cobra.Command for testing run functions that need cmd.Flags().
func fakeCmd() *cobra.Command {
	return &cobra.Command{Use: "test"}
}

func TestRunStatusEmpty(t *testing.T) {
	setupTestDB(t)

	out := captureStdout(t, func() {
		err := runStatus(fakeCmd(), nil)
		if err != nil {
			t.Errorf("runStatus: %v", err)
		}
	})

	if !strings.Contains(out, "No active test") {
		t.Errorf("missing 'No active test' in: %s", out)
	}
	if !strings.Contains(out, "DB totals: 0 tests") {
		t.Errorf("missing totals in: %s", out)
	}
	if !strings.Contains(out, "Collector NOT running") {
		t.Errorf("missing collector status in: %s", out)
	}
}

func TestRunStatusWithActiveTest(t *testing.T) {
	s := setupTestDB(t)

	s.BeginTest(store.BeginTestParams{
		Name: "active_test", Channel: 36, WidthMHz: 80, FreqMHz: 5180,
	})

	out := captureStdout(t, func() {
		err := runStatus(fakeCmd(), nil)
		if err != nil {
			t.Errorf("runStatus: %v", err)
		}
	})

	if !strings.Contains(out, "Active test #1: active_test") {
		t.Errorf("missing active test in: %s", out)
	}
	if !strings.Contains(out, "DB totals: 1 tests") {
		t.Errorf("missing totals in: %s", out)
	}
}

func TestRunStatusWithOrbServer(t *testing.T) {
	setupTestDB(t)
	orbServer = "10.0.1.5:7443"
	defer func() { orbServer = "" }()

	out := captureStdout(t, func() {
		runStatus(fakeCmd(), nil)
	})

	if !strings.Contains(out, "Orb Server: 10.0.1.5:7443") {
		t.Errorf("missing orb server in: %s", out)
	}
}

func TestRunStatusWithLatestReading(t *testing.T) {
	s := setupTestDB(t)

	// Insert a responsiveness record
	ts := json.Number("1700000000000")
	tsInt, _ := ts.Int64()
	latency := int64(10000)
	jitter := int64(500)
	pktLoss := 0.5
	netName := "MyWiFi"
	s.InsertResponsiveness(
		[]orb.ResponsivenessRecord{{
			Timestamp:     tsInt,
			LatencyAvgUS:  &latency,
			JitterAvgUS:   &jitter,
			PacketLossPct: &pktLoss,
			NetworkName:   &netName,
		}},
		[]json.RawMessage{json.RawMessage(`{}`)},
		nil, "dev1",
	)

	out := captureStdout(t, func() {
		runStatus(fakeCmd(), nil)
	})

	if !strings.Contains(out, "Latest: latency=10.00ms") {
		t.Errorf("missing latency in: %s", out)
	}
	if !strings.Contains(out, "SSID=MyWiFi") {
		t.Errorf("missing SSID in: %s", out)
	}
}

func TestRunBeginAndEnd(t *testing.T) {
	s := setupTestDB(t)

	// Create a begin command with proper flags
	cmd := newBeginCmd()
	cmd.SetArgs([]string{"test_run", "--channel", "36", "--width", "80", "--freq", "5180"})

	out := captureStdout(t, func() {
		err := cmd.Execute()
		if err != nil {
			t.Fatalf("begin: %v", err)
		}
	})

	if !strings.Contains(out, "Test #1: test_run") {
		t.Errorf("missing test info in begin output: %s", out)
	}
	if !strings.Contains(out, "ch36 / 80MHz / 5180MHz") {
		t.Errorf("missing channel info in begin output: %s", out)
	}

	// Verify the test exists
	active, _ := s.GetActiveTest()
	if active == nil {
		t.Fatal("expected active test after begin")
	}
	if active.Name != "test_run" {
		t.Errorf("name = %q", active.Name)
	}

	// End it
	out = captureStdout(t, func() {
		err := runEnd(fakeCmd(), nil)
		if err != nil {
			t.Fatalf("end: %v", err)
		}
	})

	if !strings.Contains(out, "Test #1: test_run -- ended") {
		t.Errorf("missing end info: %s", out)
	}
	if !strings.Contains(out, "Duration:") {
		t.Errorf("missing duration: %s", out)
	}
}

func TestRunEndNoActiveTest(t *testing.T) {
	setupTestDB(t)

	out := captureStdout(t, func() {
		err := runEnd(fakeCmd(), nil)
		if err != nil {
			t.Fatalf("end: %v", err)
		}
	})

	if !strings.Contains(out, "No active test to end") {
		t.Errorf("missing no-active message: %s", out)
	}
}

func TestRunBeginAutoEndsPrevious(t *testing.T) {
	s := setupTestDB(t)

	// Start first test
	cmd1 := newBeginCmd()
	cmd1.SetArgs([]string{"first", "--channel", "1", "--width", "20", "--freq", "2412"})
	captureStdout(t, func() { cmd1.Execute() })

	// Start second test (should auto-end first)
	cmd2 := newBeginCmd()
	cmd2.SetArgs([]string{"second", "--channel", "6", "--width", "40", "--freq", "2437"})
	out := captureStdout(t, func() { cmd2.Execute() })

	if !strings.Contains(out, "Auto-ended previous test: first") {
		t.Errorf("missing auto-end message: %s", out)
	}

	// First should be completed
	test, _ := s.GetTest(1)
	if test == nil || !test.EndTime.Valid {
		t.Error("first test should have end_time")
	}

	// Second should be active
	active, _ := s.GetActiveTest()
	if active == nil || active.Name != "second" {
		t.Error("second test should be active")
	}
}

func TestRunBeginWithOptionalFlags(t *testing.T) {
	s := setupTestDB(t)

	cmd := newBeginCmd()
	cmd.SetArgs([]string{"detailed", "--channel", "149", "--width", "160", "--freq", "5745",
		"--ap", "AP-01", "--rssi", "-55", "--snr", "30", "--notes", "test notes"})

	captureStdout(t, func() {
		err := cmd.Execute()
		if err != nil {
			t.Fatalf("begin: %v", err)
		}
	})

	test, _ := s.GetTest(1)
	if test == nil {
		t.Fatal("expected test")
	}
	if !test.APName.Valid || test.APName.String != "AP-01" {
		t.Errorf("ap_name = %v", test.APName)
	}
	if !test.APRSSI.Valid || test.APRSSI.Int64 != -55 {
		t.Errorf("ap_rssi = %v", test.APRSSI)
	}
	if !test.Notes.Valid || test.Notes.String != "test notes" {
		t.Errorf("notes = %v", test.Notes)
	}
}

func TestRunReportEmpty(t *testing.T) {
	setupTestDB(t)
	reportDetail = false

	out := captureStdout(t, func() {
		err := runReport(fakeCmd(), nil)
		if err != nil {
			t.Errorf("report: %v", err)
		}
	})

	if !strings.Contains(out, "No completed tests found") {
		t.Errorf("missing empty message: %s", out)
	}
}

func TestRunReportWithData(t *testing.T) {
	s := setupTestDB(t)
	reportDetail = false

	id, _ := s.BeginTest(store.BeginTestParams{
		Name: "report_test", Channel: 36, WidthMHz: 80, FreqMHz: 5180,
	})

	// Insert data so the report has something
	ts := int64(1700000000000)
	for i := range 10 {
		latency := int64(5000 + i*1000)
		jitter := int64(500)
		loss := 0.1
		s.InsertResponsiveness(
			[]orb.ResponsivenessRecord{{
				Timestamp:     ts + int64(i)*1000,
				LatencyAvgUS:  &latency,
				JitterAvgUS:   &jitter,
				PacketLossPct: &loss,
			}},
			[]json.RawMessage{json.RawMessage(`{}`)},
			&id, "dev1",
		)
	}
	s.EndTest(id)

	out := captureStdout(t, func() {
		err := runReport(fakeCmd(), nil)
		if err != nil {
			t.Errorf("report: %v", err)
		}
	})

	if !strings.Contains(out, "WiFi Channel/Width Test Results") {
		t.Errorf("missing header: %s", out)
	}
	if !strings.Contains(out, "report_test") {
		t.Errorf("missing test name: %s", out)
	}
}

func TestRunReportWithDetail(t *testing.T) {
	s := setupTestDB(t)
	reportDetail = true
	defer func() { reportDetail = false }()

	id, _ := s.BeginTest(store.BeginTestParams{
		Name: "detail_test", Channel: 36, WidthMHz: 80, FreqMHz: 5180,
	})

	ts := int64(1700000000000)
	for i := range 5 {
		latency := int64(5000 + i*2000)
		jitter := int64(500)
		loss := 0.1
		s.InsertResponsiveness(
			[]orb.ResponsivenessRecord{{
				Timestamp:     ts + int64(i)*1000,
				LatencyAvgUS:  &latency,
				JitterAvgUS:   &jitter,
				PacketLossPct: &loss,
			}},
			[]json.RawMessage{json.RawMessage(`{}`)},
			&id, "dev1",
		)
	}
	s.EndTest(id)

	out := captureStdout(t, func() {
		err := runReport(fakeCmd(), nil)
		if err != nil {
			t.Errorf("report: %v", err)
		}
	})

	if !strings.Contains(out, "detail_test -- ch36 / 80MHz") {
		t.Errorf("missing detail header: %s", out)
	}
	if !strings.Contains(out, "Latency distribution") {
		t.Errorf("missing latency distribution: %s", out)
	}
	if !strings.Contains(out, "<10ms") {
		t.Errorf("missing bucket label: %s", out)
	}
}

func TestRunReportDetailWithAPInfo(t *testing.T) {
	s := setupTestDB(t)
	reportDetail = true
	defer func() { reportDetail = false }()

	rssi := -55
	snr := 30
	id, _ := s.BeginTest(store.BeginTestParams{
		Name: "ap_detail", Channel: 6, WidthMHz: 20, FreqMHz: 2437,
		APName: "AP-01", APRSSI: &rssi, APSNR: &snr,
	})

	ts := int64(1700000000000)
	latency := int64(5000)
	jitter := int64(500)
	loss := 0.1
	s.InsertResponsiveness(
		[]orb.ResponsivenessRecord{{
			Timestamp:     ts,
			LatencyAvgUS:  &latency,
			JitterAvgUS:   &jitter,
			PacketLossPct: &loss,
		}},
		[]json.RawMessage{json.RawMessage(`{}`)},
		&id, "dev1",
	)
	s.EndTest(id)

	out := captureStdout(t, func() {
		runReport(fakeCmd(), nil)
	})

	if !strings.Contains(out, "AP: AP-01") {
		t.Errorf("missing AP name: %s", out)
	}
	if !strings.Contains(out, "RSSI=-55") {
		t.Errorf("missing RSSI: %s", out)
	}
	if !strings.Contains(out, "SNR=30") {
		t.Errorf("missing SNR: %s", out)
	}
	if !strings.Contains(out, "Duration:") {
		t.Errorf("missing duration: %s", out)
	}
}

func TestRunDump(t *testing.T) {
	s := setupTestDB(t)

	id, _ := s.BeginTest(store.BeginTestParams{
		Name: "dump_test", Channel: 6, WidthMHz: 20, FreqMHz: 2437,
	})

	ts := int64(1700000000000)
	latency := int64(5000)
	s.InsertResponsiveness(
		[]orb.ResponsivenessRecord{{Timestamp: ts, LatencyAvgUS: &latency}},
		[]json.RawMessage{json.RawMessage(`{"test":true}`)},
		&id, "dev1",
	)

	out := captureStdout(t, func() {
		err := runDump(fakeCmd(), []string{"1"})
		if err != nil {
			t.Fatalf("dump: %v", err)
		}
	})

	// Should be valid JSON
	var result map[string]any
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("invalid JSON output: %v\noutput: %s", err, out)
	}

	// Should have test metadata
	test, ok := result["test"].(map[string]any)
	if !ok {
		t.Fatal("missing test key")
	}
	if test["name"] != "dump_test" {
		t.Errorf("test name = %v", test["name"])
	}

	// Should have responsiveness data
	resp, ok := result["responsiveness"].([]any)
	if !ok || len(resp) != 1 {
		t.Errorf("responsiveness = %v", result["responsiveness"])
	}
}

func TestRunDumpInvalidTestID(t *testing.T) {
	setupTestDB(t)

	err := runDump(fakeCmd(), []string{"abc"})
	if err == nil {
		t.Fatal("expected error for invalid test ID")
	}
	if !strings.Contains(err.Error(), "invalid test ID") {
		t.Errorf("error = %v", err)
	}
}

func TestRunDumpNonExistentTest(t *testing.T) {
	setupTestDB(t)

	err := runDump(fakeCmd(), []string{"999"})
	if err == nil {
		t.Fatal("expected error for non-existent test")
	}
}

func TestPidFilePath(t *testing.T) {
	dbPath = "/data/orb-collector.duckdb"
	defer func() { dbPath = "" }()

	got := pidFilePath()
	want := "/data/orb-collector.pid"
	if got != want {
		t.Errorf("pidFilePath = %q, want %q", got, want)
	}
}

func TestPidFilePathNoExtension(t *testing.T) {
	dbPath = "/data/mydb"
	defer func() { dbPath = "" }()

	got := pidFilePath()
	want := "/data/mydb.pid"
	if got != want {
		t.Errorf("pidFilePath = %q, want %q", got, want)
	}
}

func TestHandleStatusWithLatestReading(t *testing.T) {
	srv, s := newTestAPIServer(t)

	// Insert a responsiveness record
	latency := int64(15000)
	jitter := int64(2000)
	loss := 1.5
	netName := "TestSSID"
	s.InsertResponsiveness(
		[]orb.ResponsivenessRecord{{
			Timestamp:     1700000000000,
			LatencyAvgUS:  &latency,
			JitterAvgUS:   &jitter,
			PacketLossPct: &loss,
			NetworkName:   &netName,
		}},
		[]json.RawMessage{json.RawMessage(`{}`)},
		nil, "dev1",
	)

	w := captureHTTPStatus(t, srv)
	var resp statusResponse
	json.Unmarshal(w, &resp)

	if resp.LatestReading == nil {
		t.Fatal("expected latest reading")
	}
	if resp.LatestReading.LatencyMS == nil || *resp.LatestReading.LatencyMS != 15.0 {
		t.Errorf("latency = %v", resp.LatestReading.LatencyMS)
	}
	if resp.LatestReading.JitterMS == nil || *resp.LatestReading.JitterMS != 2.0 {
		t.Errorf("jitter = %v", resp.LatestReading.JitterMS)
	}
	if resp.LatestReading.LossPct == nil || *resp.LatestReading.LossPct != 1.5 {
		t.Errorf("loss = %v", resp.LatestReading.LossPct)
	}
	if resp.LatestReading.NetworkName == nil || *resp.LatestReading.NetworkName != "TestSSID" {
		t.Errorf("network = %v", resp.LatestReading.NetworkName)
	}
}

func TestHandleStatusWithTotals(t *testing.T) {
	srv, s := newTestAPIServer(t)

	s.BeginTest(store.BeginTestParams{Name: "t1", Channel: 1, WidthMHz: 20, FreqMHz: 2412})

	ts := int64(1700000000000)
	s.InsertResponsiveness(
		[]orb.ResponsivenessRecord{{Timestamp: ts}},
		[]json.RawMessage{json.RawMessage(`{}`)},
		nil, "dev1",
	)

	w := captureHTTPStatus(t, srv)
	var resp statusResponse
	json.Unmarshal(w, &resp)

	if resp.Totals.Tests != 1 {
		t.Errorf("tests = %d", resp.Totals.Tests)
	}
	if resp.Totals.Resp != 1 {
		t.Errorf("resp = %d", resp.Totals.Resp)
	}
}

func captureHTTPStatus(t *testing.T, srv *apiServer) []byte {
	t.Helper()
	req := httptest.NewRequest("GET", "/api/status", nil)
	w := httptest.NewRecorder()
	srv.handleStatus(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
	return w.Body.Bytes()
}
