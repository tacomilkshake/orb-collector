package store

import (
	"database/sql"
	"encoding/json"
	"testing"
	"time"

	"github.com/tacomilkshake/orb-collector/internal/connector"
	"github.com/tacomilkshake/orb-collector/internal/orb"
)

// newTestStore creates an in-memory DuckDB store for testing.
func newTestStore(t *testing.T) *Store {
	t.Helper()
	s, err := New(":memory:")
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestNewAndClose(t *testing.T) {
	s := newTestStore(t)
	if s.DB() == nil {
		t.Fatal("expected non-nil DB")
	}
}

func TestBeginAndGetActiveTest(t *testing.T) {
	s := newTestStore(t)

	// No active test initially
	active, err := s.GetActiveTest()
	if err != nil {
		t.Fatalf("get active: %v", err)
	}
	if active != nil {
		t.Fatal("expected nil active test")
	}

	// Begin a test
	id, err := s.BeginTest(BeginTestParams{
		Name:     "test1",
		Channel:  36,
		WidthMHz: 80,
		FreqMHz:  5180,
	})
	if err != nil {
		t.Fatalf("begin test: %v", err)
	}
	if id <= 0 {
		t.Fatalf("expected positive ID, got %d", id)
	}

	// Should be active now
	active, err = s.GetActiveTest()
	if err != nil {
		t.Fatalf("get active: %v", err)
	}
	if active == nil {
		t.Fatal("expected active test")
	}
	if active.Name != "test1" {
		t.Errorf("name = %q, want %q", active.Name, "test1")
	}
	if active.Channel != 36 {
		t.Errorf("channel = %d, want 36", active.Channel)
	}
}

func TestBeginTestWithOptionalParams(t *testing.T) {
	s := newTestStore(t)
	rssi := -55
	snr := 30
	ps := true
	id, err := s.BeginTest(BeginTestParams{
		Name:        "full_test",
		Channel:     149,
		WidthMHz:    160,
		FreqMHz:     5745,
		APPlatform:  "omada",
		APName:      "AP-01",
		APRSSI:      &rssi,
		APSNR:       &snr,
		APPowerSave: &ps,
		APWifiMode:  "ax",
		Notes:       "testing all params",
	})
	if err != nil {
		t.Fatalf("begin test: %v", err)
	}

	test, err := s.GetTest(id)
	if err != nil {
		t.Fatalf("get test: %v", err)
	}
	if !test.APPlatform.Valid || test.APPlatform.String != "omada" {
		t.Errorf("ap_platform = %v", test.APPlatform)
	}
	if !test.APRSSI.Valid || test.APRSSI.Int64 != -55 {
		t.Errorf("ap_rssi = %v", test.APRSSI)
	}
	if !test.Notes.Valid || test.Notes.String != "testing all params" {
		t.Errorf("notes = %v", test.Notes)
	}
}

func TestGetTestNotFound(t *testing.T) {
	s := newTestStore(t)
	test, err := s.GetTest(9999)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if test != nil {
		t.Fatal("expected nil for non-existent test")
	}
}

func TestEndTest(t *testing.T) {
	s := newTestStore(t)

	id, err := s.BeginTest(BeginTestParams{
		Name: "to_end", Channel: 1, WidthMHz: 20, FreqMHz: 2412,
	})
	if err != nil {
		t.Fatalf("begin: %v", err)
	}

	resp, wifi, err := s.EndTest(id)
	if err != nil {
		t.Fatalf("end test: %v", err)
	}
	// No data inserted, so tagged counts should be 0
	if resp != 0 || wifi != 0 {
		t.Errorf("tagged = (%d, %d), want (0, 0)", resp, wifi)
	}

	// Should no longer be active
	active, err := s.GetActiveTest()
	if err != nil {
		t.Fatalf("get active: %v", err)
	}
	if active != nil {
		t.Fatal("expected no active test after end")
	}

	// Should appear in completed tests
	completed, err := s.GetCompletedTests()
	if err != nil {
		t.Fatalf("get completed: %v", err)
	}
	if len(completed) != 1 {
		t.Fatalf("completed = %d, want 1", len(completed))
	}
	if completed[0].Name != "to_end" {
		t.Errorf("name = %q, want %q", completed[0].Name, "to_end")
	}
}

func TestEndTestWithPreTaggedRecords(t *testing.T) {
	s := newTestStore(t)

	id, err := s.BeginTest(BeginTestParams{
		Name: "tag_test", Channel: 6, WidthMHz: 20, FreqMHz: 2437,
	})
	if err != nil {
		t.Fatalf("begin: %v", err)
	}

	// Insert records already tagged with test_id
	ts := time.Now().UnixMilli()
	for i := range 3 {
		s.InsertResponsiveness(
			[]orb.ResponsivenessRecord{{Timestamp: ts + int64(i)*1000}},
			[]json.RawMessage{json.RawMessage(`{}`)},
			&id, "dev1",
		)
	}
	for i := range 2 {
		s.InsertWifiLink(
			[]orb.WifiLinkRecord{{Timestamp: ts + int64(i)*1000}},
			[]json.RawMessage{json.RawMessage(`{}`)},
			&id, "dev1",
		)
	}

	_, _, err = s.EndTest(id)
	if err != nil {
		t.Fatalf("end test: %v", err)
	}

	// Verify the test is ended and counts are correct
	test, _ := s.GetTest(id)
	if test == nil || !test.EndTime.Valid {
		t.Error("expected test to have end_time")
	}

	respCount, _ := s.CountResponsiveness(id)
	if respCount != 3 {
		t.Errorf("resp count = %d, want 3", respCount)
	}
	wifiCount, _ := s.CountWifiLink(id)
	if wifiCount != 2 {
		t.Errorf("wifi count = %d, want 2", wifiCount)
	}
}

func intPtr(v int) *int       { return &v }
func int64Ptr(v int64) *int64 { return &v }
func strPtr(v string) *string { return &v }
func f64Ptr(v float64) *float64 { return &v }

func TestInsertResponsiveness(t *testing.T) {
	s := newTestStore(t)

	ts := time.Now().UnixMilli()
	records := []orb.ResponsivenessRecord{
		{
			Timestamp:    ts,
			IntervalMS:   intPtr(1000),
			NetworkName:  strPtr("TestNet"),
			LatencyAvgUS: int64Ptr(5000),
			JitterAvgUS:  int64Ptr(500),
		},
		{
			Timestamp:    ts + 1000,
			IntervalMS:   intPtr(1000),
			NetworkName:  strPtr("TestNet"),
			LatencyAvgUS: int64Ptr(6000),
		},
	}
	rawRecords := make([]json.RawMessage, len(records))
	for i, r := range records {
		b, _ := json.Marshal(r)
		rawRecords[i] = b
	}

	n, err := s.InsertResponsiveness(records, rawRecords, nil, "dev1")
	if err != nil {
		t.Fatalf("insert: %v", err)
	}
	if n != 2 {
		t.Errorf("inserted = %d, want 2", n)
	}

	// Duplicate insert: ON CONFLICT DO NOTHING silently succeeds per row,
	// so the count reflects exec calls without errors, not actual new rows.
	n, err = s.InsertResponsiveness(records, rawRecords, nil, "dev1")
	if err != nil {
		t.Fatalf("dup insert: %v", err)
	}
	// Verify no additional rows in DB
	var count int64
	s.db.QueryRow("SELECT COUNT(*) FROM responsiveness").Scan(&count)
	if count != 2 {
		t.Errorf("total rows = %d, want 2 (dedup should prevent new rows)", count)
	}
}

func TestInsertWifiLink(t *testing.T) {
	s := newTestStore(t)

	ts := time.Now().UnixMilli()
	records := []orb.WifiLinkRecord{
		{
			Timestamp:   ts,
			IntervalMS:  intPtr(1000),
			NetworkName: strPtr("TestNet"),
			RSSIAvg:     intPtr(-55),
			SNRAvg:      intPtr(30),
		},
	}
	rawRecords := make([]json.RawMessage, len(records))
	for i, r := range records {
		b, _ := json.Marshal(r)
		rawRecords[i] = b
	}

	n, err := s.InsertWifiLink(records, rawRecords, nil, "dev1")
	if err != nil {
		t.Fatalf("insert: %v", err)
	}
	if n != 1 {
		t.Errorf("inserted = %d, want 1", n)
	}

	// Dedup: verify no additional rows
	s.InsertWifiLink(records, rawRecords, nil, "dev1")
	var count int64
	s.db.QueryRow("SELECT COUNT(*) FROM wifi_link").Scan(&count)
	if count != 1 {
		t.Errorf("total rows = %d, want 1 (dedup should prevent new rows)", count)
	}
}

func TestInsertSpeedResults(t *testing.T) {
	s := newTestStore(t)

	ts := time.Now().UnixMilli()
	records := []orb.SpeedResultsRecord{
		{
			Timestamp:    ts,
			DownloadKbps: int64Ptr(50000),
			UploadKbps:   int64Ptr(25000),
		},
	}
	raw := make([]json.RawMessage, 1)
	b, _ := json.Marshal(records[0])
	raw[0] = b

	n, err := s.InsertSpeedResults(records, raw, nil, "dev1")
	if err != nil {
		t.Fatalf("insert: %v", err)
	}
	if n != 1 {
		t.Errorf("inserted = %d, want 1", n)
	}
}

func TestInsertScores(t *testing.T) {
	s := newTestStore(t)

	ts := time.Now().UnixMilli()
	records := []orb.ScoresRecord{
		{
			Timestamp:           ts,
			OrbScore:            85.5,
			ResponsivenessScore: 90.0,
			ReliabilityScore:    80.0,
			SpeedScore:          75.0,
		},
	}
	raw := make([]json.RawMessage, 1)
	b, _ := json.Marshal(records[0])
	raw[0] = b

	n, err := s.InsertScores(records, raw, nil, "dev1")
	if err != nil {
		t.Fatalf("insert: %v", err)
	}
	if n != 1 {
		t.Errorf("inserted = %d, want 1", n)
	}
}

func TestInsertWithTestID(t *testing.T) {
	s := newTestStore(t)

	id, err := s.BeginTest(BeginTestParams{
		Name: "tagged", Channel: 36, WidthMHz: 80, FreqMHz: 5180,
	})
	if err != nil {
		t.Fatalf("begin: %v", err)
	}

	ts := time.Now().UnixMilli()
	records := []orb.ResponsivenessRecord{{Timestamp: ts, LatencyAvgUS: int64Ptr(5000)}}
	raw := []json.RawMessage{json.RawMessage(`{}`)}

	n, err := s.InsertResponsiveness(records, raw, &id, "dev1")
	if err != nil {
		t.Fatalf("insert: %v", err)
	}
	if n != 1 {
		t.Errorf("inserted = %d, want 1", n)
	}

	count, err := s.CountResponsiveness(id)
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 1 {
		t.Errorf("count = %d, want 1", count)
	}
}

func TestInsertAPSnapshots(t *testing.T) {
	s := newTestStore(t)

	snapshots := []connector.ClientInfo{
		{MAC: "AA:BB:CC:DD:EE:01", RSSI: intPtr(-50), Raw: map[string]any{"mac": "AA:BB:CC:DD:EE:01"}},
		{MAC: "AA:BB:CC:DD:EE:02", RSSI: intPtr(-60), Raw: map[string]any{"mac": "AA:BB:CC:DD:EE:02"}},
	}

	n, err := s.InsertAPSnapshots(nil, snapshots, "omada")
	if err != nil {
		t.Fatalf("insert: %v", err)
	}
	if n != 2 {
		t.Errorf("inserted = %d, want 2", n)
	}
}

func TestCountMethods(t *testing.T) {
	s := newTestStore(t)

	id, _ := s.BeginTest(BeginTestParams{
		Name: "count_test", Channel: 1, WidthMHz: 20, FreqMHz: 2412,
	})

	ts := time.Now().UnixMilli()

	// Insert records with test_id
	respRecs := []orb.ResponsivenessRecord{{Timestamp: ts}}
	s.InsertResponsiveness(respRecs, []json.RawMessage{json.RawMessage(`{}`)}, &id, "d")

	wifiRecs := []orb.WifiLinkRecord{{Timestamp: ts}}
	s.InsertWifiLink(wifiRecs, []json.RawMessage{json.RawMessage(`{}`)}, &id, "d")

	scoreRecs := []orb.ScoresRecord{{Timestamp: ts}}
	s.InsertScores(scoreRecs, []json.RawMessage{json.RawMessage(`{}`)}, &id, "d")

	resp, err := s.CountResponsiveness(id)
	if err != nil || resp != 1 {
		t.Errorf("resp count = %d, err = %v", resp, err)
	}

	wifi, err := s.CountWifiLink(id)
	if err != nil || wifi != 1 {
		t.Errorf("wifi count = %d, err = %v", wifi, err)
	}

	scores, err := s.CountScores(id)
	if err != nil || scores != 1 {
		t.Errorf("scores count = %d, err = %v", scores, err)
	}
}

func TestTotalCounts(t *testing.T) {
	s := newTestStore(t)

	// Empty DB
	tests, resp, wifi, speed, scores, err := s.TotalCounts()
	if err != nil {
		t.Fatalf("total counts: %v", err)
	}
	if tests != 0 || resp != 0 || wifi != 0 || speed != 0 || scores != 0 {
		t.Errorf("empty totals = (%d,%d,%d,%d,%d), want all 0", tests, resp, wifi, speed, scores)
	}

	// Add some data
	s.BeginTest(BeginTestParams{Name: "t1", Channel: 1, WidthMHz: 20, FreqMHz: 2412})

	ts := time.Now().UnixMilli()
	s.InsertResponsiveness([]orb.ResponsivenessRecord{{Timestamp: ts}}, []json.RawMessage{json.RawMessage(`{}`)}, nil, "d")
	s.InsertWifiLink([]orb.WifiLinkRecord{{Timestamp: ts}}, []json.RawMessage{json.RawMessage(`{}`)}, nil, "d")

	tests, resp, wifi, speed, scores, _ = s.TotalCounts()
	if tests != 1 {
		t.Errorf("tests = %d, want 1", tests)
	}
	if resp != 1 {
		t.Errorf("resp = %d, want 1", resp)
	}
	if wifi != 1 {
		t.Errorf("wifi = %d, want 1", wifi)
	}
}

func TestGetLatestReading(t *testing.T) {
	s := newTestStore(t)

	// Empty
	lr, err := s.GetLatestReading()
	if err != nil {
		t.Fatalf("get latest: %v", err)
	}
	if lr != nil {
		t.Fatal("expected nil for empty DB")
	}

	// Add a record
	ts := time.Now().UnixMilli()
	records := []orb.ResponsivenessRecord{
		{
			Timestamp:    ts,
			LatencyAvgUS: int64Ptr(10000),
			JitterAvgUS:  int64Ptr(1000),
			NetworkName:  strPtr("MyWiFi"),
		},
	}
	raw := []json.RawMessage{json.RawMessage(`{}`)}
	s.InsertResponsiveness(records, raw, nil, "dev1")

	lr, err = s.GetLatestReading()
	if err != nil {
		t.Fatalf("get latest: %v", err)
	}
	if lr == nil {
		t.Fatal("expected non-nil reading")
	}
	if !lr.LatencyAvgUS.Valid || lr.LatencyAvgUS.Int64 != 10000 {
		t.Errorf("latency = %v", lr.LatencyAvgUS)
	}
	if !lr.NetworkName.Valid || lr.NetworkName.String != "MyWiFi" {
		t.Errorf("network = %v", lr.NetworkName)
	}
}

func TestGetReportRows(t *testing.T) {
	s := newTestStore(t)

	// No completed tests
	rows, err := s.GetReportRows()
	if err != nil {
		t.Fatalf("report: %v", err)
	}
	if len(rows) != 0 {
		t.Errorf("expected 0 rows, got %d", len(rows))
	}

	// Create a completed test with data
	id, _ := s.BeginTest(BeginTestParams{
		Name: "report_test", Channel: 36, WidthMHz: 80, FreqMHz: 5180,
	})

	ts := time.Now().UnixMilli()
	for i := range 10 {
		latency := int64(5000 + i*1000)
		s.InsertResponsiveness(
			[]orb.ResponsivenessRecord{{
				Timestamp:    ts + int64(i)*1000,
				LatencyAvgUS: &latency,
				JitterAvgUS:  int64Ptr(500),
				PacketLossPct: f64Ptr(0.1),
			}},
			[]json.RawMessage{json.RawMessage(`{}`)},
			&id, "dev1",
		)
	}

	s.EndTest(id)

	rows, err = s.GetReportRows()
	if err != nil {
		t.Fatalf("report: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("rows = %d, want 1", len(rows))
	}
	if rows[0].Name != "report_test" {
		t.Errorf("name = %q", rows[0].Name)
	}
	if rows[0].N != 10 {
		t.Errorf("n = %d, want 10", rows[0].N)
	}
	if rows[0].AvgMS <= 0 {
		t.Errorf("avg_ms = %f, want > 0", rows[0].AvgMS)
	}
}

func TestDumpTestData(t *testing.T) {
	s := newTestStore(t)

	id, _ := s.BeginTest(BeginTestParams{
		Name: "dump_test", Channel: 6, WidthMHz: 20, FreqMHz: 2437,
	})

	ts := time.Now().UnixMilli()
	s.InsertResponsiveness(
		[]orb.ResponsivenessRecord{{Timestamp: ts, LatencyAvgUS: int64Ptr(5000)}},
		[]json.RawMessage{json.RawMessage(`{"test":true}`)},
		&id, "dev1",
	)

	test, resp, wifi, speed, scores, err := s.DumpTestData(id)
	if err != nil {
		t.Fatalf("dump: %v", err)
	}
	if test == nil {
		t.Fatal("expected non-nil test map")
	}
	if test["name"] != "dump_test" {
		t.Errorf("test name = %v", test["name"])
	}
	if len(resp) != 1 {
		t.Errorf("resp = %d, want 1", len(resp))
	}
	if len(wifi) != 0 {
		t.Errorf("wifi = %d, want 0", len(wifi))
	}
	if len(speed) != 0 {
		t.Errorf("speed = %d, want 0", len(speed))
	}
	if len(scores) != 0 {
		t.Errorf("scores = %d, want 0", len(scores))
	}
}

func TestNullHelpers(t *testing.T) {
	// nullStr
	ns := nullStr("")
	if ns.Valid {
		t.Error("empty string should be null")
	}
	ns = nullStr("hello")
	if !ns.Valid || ns.String != "hello" {
		t.Errorf("nullStr(hello) = %v", ns)
	}

	// nullInt
	ni := nullInt(nil)
	if ni.Valid {
		t.Error("nil int should be null")
	}
	v := 42
	ni = nullInt(&v)
	if !ni.Valid || ni.Int64 != 42 {
		t.Errorf("nullInt(42) = %v", ni)
	}

	// nullBool
	nb := nullBool(nil)
	if nb.Valid {
		t.Error("nil bool should be null")
	}
	b := true
	nb = nullBool(&b)
	if !nb.Valid || !nb.Bool {
		t.Errorf("nullBool(true) = %v", nb)
	}

	// nilInt64
	if nilInt64(nil) != nil {
		t.Error("nilInt64(nil) should be nil")
	}
	i := int64(99)
	if nilInt64(&i) != int64(99) {
		t.Errorf("nilInt64(99) = %v", nilInt64(&i))
	}
}

func TestGetCompletedTestsEmpty(t *testing.T) {
	s := newTestStore(t)
	tests, err := s.GetCompletedTests()
	if err != nil {
		t.Fatalf("get completed: %v", err)
	}
	if len(tests) != 0 {
		t.Errorf("expected 0 completed tests, got %d", len(tests))
	}
}

func TestMultipleTestLifecycle(t *testing.T) {
	s := newTestStore(t)

	id1, _ := s.BeginTest(BeginTestParams{Name: "t1", Channel: 1, WidthMHz: 20, FreqMHz: 2412})
	s.EndTest(id1)

	id2, _ := s.BeginTest(BeginTestParams{Name: "t2", Channel: 6, WidthMHz: 40, FreqMHz: 2437})

	// t2 should be active
	active, _ := s.GetActiveTest()
	if active == nil || active.ID != id2 {
		t.Error("expected t2 to be active")
	}

	// 1 completed test
	completed, _ := s.GetCompletedTests()
	if len(completed) != 1 || completed[0].ID != id1 {
		t.Errorf("completed = %v", completed)
	}

	s.EndTest(id2)

	// No active, 2 completed
	active, _ = s.GetActiveTest()
	if active != nil {
		t.Error("expected no active test")
	}
	completed, _ = s.GetCompletedTests()
	if len(completed) != 2 {
		t.Errorf("completed = %d, want 2", len(completed))
	}
}

func TestInsertResponsivenessNoRaw(t *testing.T) {
	s := newTestStore(t)

	// More records than rawRecords: should auto-marshal
	ts := time.Now().UnixMilli()
	records := []orb.ResponsivenessRecord{
		{Timestamp: ts, LatencyAvgUS: int64Ptr(5000)},
		{Timestamp: ts + 1000, LatencyAvgUS: int64Ptr(6000)},
	}
	// Only provide 1 raw record for 2 records
	raw := []json.RawMessage{json.RawMessage(`{}`)}

	n, err := s.InsertResponsiveness(records, raw, nil, "dev1")
	if err != nil {
		t.Fatalf("insert: %v", err)
	}
	if n != 2 {
		t.Errorf("inserted = %d, want 2", n)
	}
}

func TestInsertAPSnapshotsWithTestID(t *testing.T) {
	s := newTestStore(t)
	id, _ := s.BeginTest(BeginTestParams{Name: "ap_test", Channel: 1, WidthMHz: 20, FreqMHz: 2412})

	snapshots := []connector.ClientInfo{
		{MAC: "AA:BB:CC:DD:EE:01", Raw: map[string]any{"mac": "AA:BB:CC:DD:EE:01"}},
	}

	n, err := s.InsertAPSnapshots(&id, snapshots, "omada")
	if err != nil {
		t.Fatalf("insert: %v", err)
	}
	if n != 1 {
		t.Errorf("inserted = %d, want 1", n)
	}

	// Verify the snapshot has the test_id
	var tid sql.NullInt64
	s.db.QueryRow("SELECT test_id FROM ap_snapshots LIMIT 1").Scan(&tid)
	if !tid.Valid || tid.Int64 != id {
		t.Errorf("test_id = %v, want %d", tid, id)
	}
}
