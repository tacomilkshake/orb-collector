package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/tacomilkshake/orb-collector/internal/connector"
	"github.com/tacomilkshake/orb-collector/internal/orb"
)

// Test represents a row in the tests table.
type Test struct {
	ID          int64
	Name        string
	Channel     int
	WidthMHz    int
	FreqMHz     int
	StartTime   time.Time
	EndTime     sql.NullTime
	APPlatform  sql.NullString
	APName      sql.NullString
	APRSSI      sql.NullInt64
	APSNR       sql.NullInt64
	APChannel   sql.NullInt64
	APPowerSave sql.NullBool
	APRxRate    sql.NullInt64
	APTxRate    sql.NullInt64
	APWifiMode  sql.NullString
	Notes       sql.NullString
}

// GetActiveTest returns the currently running test (no end_time), or nil.
func (s *Store) GetActiveTest() (*Test, error) {
	row := s.db.QueryRow(`
		SELECT id, name, channel, width_mhz, freq_mhz, start_time, end_time,
		       ap_platform, ap_name, ap_rssi, ap_snr, ap_channel, ap_power_save,
		       ap_rx_rate, ap_tx_rate, ap_wifi_mode, notes
		FROM tests WHERE end_time IS NULL ORDER BY id DESC LIMIT 1`)
	return scanTest(row)
}

// GetTest returns a test by ID.
func (s *Store) GetTest(id int64) (*Test, error) {
	row := s.db.QueryRow(`
		SELECT id, name, channel, width_mhz, freq_mhz, start_time, end_time,
		       ap_platform, ap_name, ap_rssi, ap_snr, ap_channel, ap_power_save,
		       ap_rx_rate, ap_tx_rate, ap_wifi_mode, notes
		FROM tests WHERE id = ?`, id)
	return scanTest(row)
}

// GetCompletedTests returns all tests with an end_time.
func (s *Store) GetCompletedTests() ([]Test, error) {
	rows, err := s.db.Query(`
		SELECT id, name, channel, width_mhz, freq_mhz, start_time, end_time,
		       ap_platform, ap_name, ap_rssi, ap_snr, ap_channel, ap_power_save,
		       ap_rx_rate, ap_tx_rate, ap_wifi_mode, notes
		FROM tests WHERE end_time IS NOT NULL ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tests []Test
	for rows.Next() {
		t, err := scanTestRow(rows)
		if err != nil {
			return nil, err
		}
		tests = append(tests, *t)
	}
	return tests, rows.Err()
}

func scanTest(row *sql.Row) (*Test, error) {
	var t Test
	err := row.Scan(&t.ID, &t.Name, &t.Channel, &t.WidthMHz, &t.FreqMHz,
		&t.StartTime, &t.EndTime,
		&t.APPlatform, &t.APName, &t.APRSSI, &t.APSNR, &t.APChannel, &t.APPowerSave,
		&t.APRxRate, &t.APTxRate, &t.APWifiMode, &t.Notes)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &t, nil
}

type scannable interface {
	Scan(dest ...any) error
}

func scanTestRow(row scannable) (*Test, error) {
	var t Test
	err := row.Scan(&t.ID, &t.Name, &t.Channel, &t.WidthMHz, &t.FreqMHz,
		&t.StartTime, &t.EndTime,
		&t.APPlatform, &t.APName, &t.APRSSI, &t.APSNR, &t.APChannel, &t.APPowerSave,
		&t.APRxRate, &t.APTxRate, &t.APWifiMode, &t.Notes)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

// BeginTestParams holds parameters for starting a test.
type BeginTestParams struct {
	Name        string
	Channel     int
	WidthMHz    int
	FreqMHz     int
	APPlatform  string
	APName      string
	APRSSI      *int
	APSNR       *int
	APChannel   *int
	APPowerSave *bool
	APRxRate    *int
	APTxRate    *int
	APWifiMode  string
	Notes       string
}

// BeginTest inserts a new test and returns its ID.
func (s *Store) BeginTest(p BeginTestParams) (int64, error) {
	var id int64
	err := s.db.QueryRow(`
		INSERT INTO tests (name, channel, width_mhz, freq_mhz, start_time,
		    ap_platform, ap_name, ap_rssi, ap_snr, ap_channel, ap_power_save,
		    ap_rx_rate, ap_tx_rate, ap_wifi_mode, notes)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		RETURNING id`,
		p.Name, p.Channel, p.WidthMHz, p.FreqMHz, time.Now().UTC(),
		nullStr(p.APPlatform), nullStr(p.APName),
		nullInt(p.APRSSI), nullInt(p.APSNR), nullInt(p.APChannel),
		nullBool(p.APPowerSave),
		nullInt(p.APRxRate), nullInt(p.APTxRate),
		nullStr(p.APWifiMode), nullStr(p.Notes),
	).Scan(&id)
	return id, err
}

// EndTest sets end_time on the given test and tags untagged records.
func (s *Store) EndTest(testID int64) (respTagged, wifiTagged int64, err error) {
	now := time.Now().UTC()
	_, err = s.db.Exec("UPDATE tests SET end_time = ? WHERE id = ?", now, testID)
	if err != nil {
		return 0, 0, fmt.Errorf("end test: %w", err)
	}

	// Get start time
	var startTime time.Time
	err = s.db.QueryRow("SELECT start_time FROM tests WHERE id = ?", testID).Scan(&startTime)
	if err != nil {
		return 0, 0, fmt.Errorf("get start time: %w", err)
	}

	// Tag untagged responsiveness records
	res, err := s.db.Exec(`
		UPDATE responsiveness SET test_id = ?
		WHERE test_id IS NULL AND orb_timestamp BETWEEN ? AND ?`,
		testID, startTime, now)
	if err != nil {
		return 0, 0, fmt.Errorf("tag responsiveness: %w", err)
	}
	respTagged, _ = res.RowsAffected()

	// Tag untagged wifi_link records
	res, err = s.db.Exec(`
		UPDATE wifi_link SET test_id = ?
		WHERE test_id IS NULL AND orb_timestamp BETWEEN ? AND ?`,
		testID, startTime, now)
	if err != nil {
		return 0, 0, fmt.Errorf("tag wifi_link: %w", err)
	}
	wifiTagged, _ = res.RowsAffected()

	// Tag untagged speed_results records
	_, _ = s.db.Exec(`
		UPDATE speed_results SET test_id = ?
		WHERE test_id IS NULL AND orb_timestamp BETWEEN ? AND ?`,
		testID, startTime, now)

	// Tag untagged scores records
	_, _ = s.db.Exec(`
		UPDATE scores SET test_id = ?
		WHERE test_id IS NULL AND orb_timestamp BETWEEN ? AND ?`,
		testID, startTime, now)

	return respTagged, wifiTagged, nil
}

// InsertResponsiveness inserts records, deduplicating by (orb_device_id, orb_timestamp).
// Returns number of records inserted.
func (s *Store) InsertResponsiveness(records []orb.ResponsivenessRecord, rawRecords []json.RawMessage, testID *int64, orbDeviceID string) (int, error) {
	inserted := 0
	for i, r := range records {
		orbTS := time.UnixMilli(r.Timestamp).UTC()
		var rawJSON string
		if i < len(rawRecords) {
			rawJSON = string(rawRecords[i])
		} else {
			b, _ := json.Marshal(r)
			rawJSON = string(b)
		}

		_, err := s.db.Exec(`
			INSERT INTO responsiveness (
				test_id, orb_device_id, collected_at, orb_timestamp, interval_ms,
				network_name, bssid,
				router_latency_avg_us, router_latency_count, router_latency_lost,
				router_lag_avg_us, router_lag_count,
				router_jitter_avg_us, router_packet_loss_pct,
				latency_avg_us, latency_count, latency_lost_count,
				lag_avg_us, lag_count, jitter_avg_us, packet_loss_pct,
				raw
			) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)
			ON CONFLICT DO NOTHING`,
			nilInt64(testID), orbDeviceID, time.Now().UTC(), orbTS, r.IntervalMS,
			r.NetworkName, r.BSSID,
			r.RouterLatencyAvgUS, r.RouterLatencyCount, r.RouterLatencyLost,
			r.RouterLagAvgUS, r.RouterLagCount,
			r.RouterJitterAvgUS, r.RouterPacketLossPct,
			r.LatencyAvgUS, r.LatencyCount, r.LatencyLostCount,
			r.LagAvgUS, r.LagCount, r.JitterAvgUS, r.PacketLossPct,
			rawJSON,
		)
		if err != nil {
			// ON CONFLICT DO NOTHING means duplicate; only count real errors
			continue
		}
		inserted++
	}
	return inserted, nil
}

// InsertWifiLink inserts records, deduplicating by (orb_device_id, orb_timestamp).
func (s *Store) InsertWifiLink(records []orb.WifiLinkRecord, rawRecords []json.RawMessage, testID *int64, orbDeviceID string) (int, error) {
	inserted := 0
	for i, r := range records {
		orbTS := time.UnixMilli(r.Timestamp).UTC()
		var rawJSON string
		if i < len(rawRecords) {
			rawJSON = string(rawRecords[i])
		} else {
			b, _ := json.Marshal(r)
			rawJSON = string(b)
		}

		_, err := s.db.Exec(`
			INSERT INTO wifi_link (
				test_id, orb_device_id, collected_at, orb_timestamp, interval_ms,
				network_name, bssid,
				rssi_avg, snr_avg, noise_avg,
				tx_rate_mbps, rx_rate_mbps,
				frequency_mhz, channel_number, channel_band,
				channel_width, phy_mode,
				raw
			) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)
			ON CONFLICT DO NOTHING`,
			nilInt64(testID), orbDeviceID, time.Now().UTC(), orbTS, r.IntervalMS,
			r.NetworkName, r.BSSID,
			r.RSSIAvg, r.SNRAvg, r.NoiseAvg,
			r.TxRateMbps, r.RxRateMbps,
			r.FrequencyMHz, r.ChannelNumber, r.ChannelBand,
			r.ChannelWidth, r.PhyMode,
			rawJSON,
		)
		if err != nil {
			continue
		}
		inserted++
	}
	return inserted, nil
}

// InsertSpeedResults inserts records, deduplicating by (orb_device_id, orb_timestamp).
func (s *Store) InsertSpeedResults(records []orb.SpeedResultsRecord, rawRecords []json.RawMessage, testID *int64, orbDeviceID string) (int, error) {
	inserted := 0
	for i, r := range records {
		orbTS := time.UnixMilli(r.Timestamp).UTC()
		var rawJSON string
		if i < len(rawRecords) {
			rawJSON = string(rawRecords[i])
		} else {
			b, _ := json.Marshal(r)
			rawJSON = string(b)
		}

		_, err := s.db.Exec(`
			INSERT INTO speed_results (
				test_id, orb_device_id, collected_at, orb_timestamp,
				network_name, bssid,
				download_kbps, upload_kbps,
				download_bytes, upload_bytes,
				server_name, raw
			) VALUES (?,?,?,?,?,?,?,?,?,?,?,?)
			ON CONFLICT DO NOTHING`,
			nilInt64(testID), orbDeviceID, time.Now().UTC(), orbTS,
			r.NetworkName, r.BSSID,
			r.DownloadKbps, r.UploadKbps,
			r.DownloadBytes, r.UploadBytes,
			r.ServerName,
			rawJSON,
		)
		if err != nil {
			continue
		}
		inserted++
	}
	return inserted, nil
}

// InsertScores inserts records, deduplicating by (orb_device_id, orb_timestamp).
func (s *Store) InsertScores(records []orb.ScoresRecord, rawRecords []json.RawMessage, testID *int64, orbDeviceID string) (int, error) {
	inserted := 0
	for i, r := range records {
		orbTS := time.UnixMilli(r.Timestamp).UTC()
		var rawJSON string
		if i < len(rawRecords) {
			rawJSON = string(rawRecords[i])
		} else {
			b, _ := json.Marshal(r)
			rawJSON = string(b)
		}

		_, err := s.db.Exec(`
			INSERT INTO scores (
				test_id, orb_device_id, collected_at, orb_timestamp, interval_ms,
				network_name, bssid,
				orb_score, responsiveness_score, reliability_score, speed_score,
				lag_avg_us, lag_count,
				download_avg_kbps, upload_avg_kbps,
				speed_age_ms, speed_count,
				unresponsive_ms, measured_ms,
				raw
			) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)
			ON CONFLICT DO NOTHING`,
			nilInt64(testID), orbDeviceID, time.Now().UTC(), orbTS, r.IntervalMS,
			r.NetworkName, r.BSSID,
			r.OrbScore, r.ResponsivenessScore, r.ReliabilityScore, r.SpeedScore,
			r.LagAvgUS, r.LagCount,
			r.DownloadAvgKbps, r.UploadAvgKbps,
			r.SpeedAgeMS, r.SpeedCount,
			r.UnresponsiveMS, r.MeasuredMS,
			rawJSON,
		)
		if err != nil {
			continue
		}
		inserted++
	}
	return inserted, nil
}

// InsertAPSnapshot inserts an AP client snapshot.
func (s *Store) InsertAPSnapshot(info *connector.ClientInfo, testID *int64, platform string, clientMAC string) error {
	if info == nil {
		return nil
	}

	rawJSON, _ := json.Marshal(info.Raw)

	_, err := s.db.Exec(`
		INSERT INTO ap_snapshots (
			test_id, client_mac, timestamp, platform, rssi, snr, signal, channel, band,
			ap_name, power_save, rx_rate, tx_rate, wifi_mode, activity, raw
		) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		nilInt64(testID), clientMAC, time.Now().UTC(), platform,
		info.RSSI, info.SNR, info.Signal, info.Channel, info.Band,
		info.APName, info.PowerSave, info.RxRate, info.TxRate, info.WifiMode, info.Activity,
		string(rawJSON),
	)
	return err
}

// InsertAPSnapshots inserts all client snapshots in one transaction.
func (s *Store) InsertAPSnapshots(testID *int64, snapshots []connector.ClientInfo, platform string) (int, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return 0, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	now := time.Now().UTC()
	inserted := 0
	for _, info := range snapshots {
		rawJSON, _ := json.Marshal(info.Raw)
		_, err := tx.Exec(`
			INSERT INTO ap_snapshots (
				test_id, client_mac, timestamp, platform, rssi, snr, signal, channel, band,
				ap_name, power_save, rx_rate, tx_rate, wifi_mode, activity, raw
			) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
			nilInt64(testID), info.MAC, now, platform,
			info.RSSI, info.SNR, info.Signal, info.Channel, info.Band,
			info.APName, info.PowerSave, info.RxRate, info.TxRate, info.WifiMode, info.Activity,
			string(rawJSON),
		)
		if err != nil {
			continue
		}
		inserted++
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit tx: %w", err)
	}
	return inserted, nil
}

// CountResponsiveness returns the number of responsiveness records for a test.
func (s *Store) CountResponsiveness(testID int64) (int64, error) {
	var count int64
	err := s.db.QueryRow("SELECT COUNT(*) FROM responsiveness WHERE test_id = ?", testID).Scan(&count)
	return count, err
}

// CountWifiLink returns the number of wifi_link records for a test.
func (s *Store) CountWifiLink(testID int64) (int64, error) {
	var count int64
	err := s.db.QueryRow("SELECT COUNT(*) FROM wifi_link WHERE test_id = ?", testID).Scan(&count)
	return count, err
}

// CountSpeedResults returns the number of speed_results records for a test.
func (s *Store) CountSpeedResults(testID int64) (int64, error) {
	var count int64
	err := s.db.QueryRow("SELECT COUNT(*) FROM speed_results WHERE test_id = ?", testID).Scan(&count)
	return count, err
}

// CountScores returns the number of scores records for a test.
func (s *Store) CountScores(testID int64) (int64, error) {
	var count int64
	err := s.db.QueryRow("SELECT COUNT(*) FROM scores WHERE test_id = ?", testID).Scan(&count)
	return count, err
}

// TotalCounts returns total record counts across all data.
func (s *Store) TotalCounts() (tests, resp, wifi, speed, scores int64, err error) {
	s.db.QueryRow("SELECT COUNT(*) FROM tests").Scan(&tests)
	s.db.QueryRow("SELECT COUNT(*) FROM responsiveness").Scan(&resp)
	s.db.QueryRow("SELECT COUNT(*) FROM wifi_link").Scan(&wifi)
	s.db.QueryRow("SELECT COUNT(*) FROM speed_results").Scan(&speed)
	s.db.QueryRow("SELECT COUNT(*) FROM scores").Scan(&scores)
	return
}

// LatestReading holds the most recent responsiveness reading.
type LatestReading struct {
	OrbTimestamp  time.Time
	LatencyAvgUS  sql.NullInt64
	JitterAvgUS   sql.NullInt64
	PacketLossPct sql.NullFloat64
	NetworkName   sql.NullString
}

// GetLatestReading returns the most recent responsiveness record.
func (s *Store) GetLatestReading() (*LatestReading, error) {
	var lr LatestReading
	err := s.db.QueryRow(`
		SELECT orb_timestamp, latency_avg_us, jitter_avg_us,
		       packet_loss_pct, network_name
		FROM responsiveness ORDER BY orb_timestamp DESC LIMIT 1`).Scan(
		&lr.OrbTimestamp, &lr.LatencyAvgUS, &lr.JitterAvgUS,
		&lr.PacketLossPct, &lr.NetworkName)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &lr, nil
}

// ReportRow holds aggregated stats for a single test's report.
type ReportRow struct {
	TestID    int64
	Name      string
	Channel   int
	WidthMHz  int
	N         int64
	Missed    int64
	P05MS     float64
	P10MS     float64
	P50MS     float64
	P90MS     float64
	P95MS     float64
	AvgMS     float64
	MinMS     float64
	MaxMS     float64
	JitterMS  float64
	LossPct   float64
	AvgRSSI   sql.NullFloat64
	AvgSNR    sql.NullFloat64
	AvgTxRate sql.NullFloat64
	// From test metadata as fallback
	APRSSI sql.NullInt64
	APSNR  sql.NullInt64
	// Speed test throughput
	AvgDownloadKbps sql.NullFloat64
	AvgUploadKbps   sql.NullFloat64
	SpeedCount      int64
}

// GetReportRows returns aggregated report data for all completed tests.
func (s *Store) GetReportRows() ([]ReportRow, error) {
	rows, err := s.db.Query(`
		WITH stats AS (
			SELECT
				t.id AS test_id,
				t.name,
				t.channel,
				t.width_mhz,
				t.ap_rssi,
				t.ap_snr,
				COUNT(*) AS total,
				COUNT(r.latency_avg_us) AS n,
				COUNT(*) - COUNT(r.latency_avg_us) AS missed,
				PERCENTILE_CONT(0.05) WITHIN GROUP (ORDER BY r.latency_avg_us) / 1000.0 AS p05_ms,
				PERCENTILE_CONT(0.10) WITHIN GROUP (ORDER BY r.latency_avg_us) / 1000.0 AS p10_ms,
				PERCENTILE_CONT(0.50) WITHIN GROUP (ORDER BY r.latency_avg_us) / 1000.0 AS p50_ms,
				PERCENTILE_CONT(0.90) WITHIN GROUP (ORDER BY r.latency_avg_us) / 1000.0 AS p90_ms,
				PERCENTILE_CONT(0.95) WITHIN GROUP (ORDER BY r.latency_avg_us) / 1000.0 AS p95_ms,
				AVG(r.latency_avg_us) / 1000.0 AS avg_ms,
				MIN(r.latency_avg_us) / 1000.0 AS min_ms,
				MAX(r.latency_avg_us) / 1000.0 AS max_ms,
				AVG(r.jitter_avg_us) / 1000.0 AS jitter_ms,
				AVG(r.packet_loss_pct) AS loss_pct
			FROM tests t
			JOIN responsiveness r ON r.test_id = t.id
			WHERE t.end_time IS NOT NULL
			GROUP BY t.id, t.name, t.channel, t.width_mhz, t.ap_rssi, t.ap_snr
			HAVING COUNT(r.latency_avg_us) > 0
		),
		wifi_stats AS (
			SELECT
				w.test_id,
				AVG(w.rssi_avg) AS avg_rssi,
				AVG(w.snr_avg) AS avg_snr,
				AVG(w.tx_rate_mbps) AS avg_tx_rate
			FROM wifi_link w
			GROUP BY w.test_id
		),
		speed_stats AS (
			SELECT
				sp.test_id,
				AVG(sp.download_kbps) AS avg_download_kbps,
				AVG(sp.upload_kbps) AS avg_upload_kbps,
				COUNT(*) AS speed_count
			FROM speed_results sp
			GROUP BY sp.test_id
		)
		SELECT
			s.test_id, s.name, s.channel, s.width_mhz,
			s.n, s.missed,
			s.p05_ms, s.p10_ms, s.p50_ms, s.p90_ms, s.p95_ms,
			s.avg_ms, s.min_ms, s.max_ms, s.jitter_ms,
			COALESCE(s.loss_pct, 0),
			ws.avg_rssi, ws.avg_snr, ws.avg_tx_rate,
			s.ap_rssi, s.ap_snr,
			ss.avg_download_kbps, ss.avg_upload_kbps, COALESCE(ss.speed_count, 0)
		FROM stats s
		LEFT JOIN wifi_stats ws ON ws.test_id = s.test_id
		LEFT JOIN speed_stats ss ON ss.test_id = s.test_id
		ORDER BY s.test_id`)
	if err != nil {
		return nil, fmt.Errorf("report query: %w", err)
	}
	defer rows.Close()

	var results []ReportRow
	for rows.Next() {
		var r ReportRow
		if err := rows.Scan(
			&r.TestID, &r.Name, &r.Channel, &r.WidthMHz,
			&r.N, &r.Missed,
			&r.P05MS, &r.P10MS, &r.P50MS, &r.P90MS, &r.P95MS,
			&r.AvgMS, &r.MinMS, &r.MaxMS, &r.JitterMS, &r.LossPct,
			&r.AvgRSSI, &r.AvgSNR, &r.AvgTxRate,
			&r.APRSSI, &r.APSNR,
			&r.AvgDownloadKbps, &r.AvgUploadKbps, &r.SpeedCount,
		); err != nil {
			return nil, fmt.Errorf("scan report row: %w", err)
		}
		results = append(results, r)
	}
	return results, rows.Err()
}

// DumpTestData returns all data for a test as maps for JSON export.
func (s *Store) DumpTestData(testID int64) (test map[string]any, resp []map[string]any, wifi []map[string]any, speed []map[string]any, scores []map[string]any, err error) {
	// Test metadata
	row := s.db.QueryRow(`SELECT * FROM tests WHERE id = ?`, testID)
	test, err = scanRowAsMap(row, []string{
		"id", "name", "channel", "width_mhz", "freq_mhz", "start_time", "end_time",
		"ap_platform", "ap_name", "ap_rssi", "ap_snr", "ap_channel", "ap_power_save",
		"ap_rx_rate", "ap_tx_rate", "ap_wifi_mode", "notes",
	})
	if err != nil {
		return
	}

	// Responsiveness
	resp, err = queryAsMapSlice(s.db, `SELECT * FROM responsiveness WHERE test_id = ? ORDER BY orb_timestamp`, testID)
	if err != nil {
		return
	}

	// WiFi link
	wifi, err = queryAsMapSlice(s.db, `SELECT * FROM wifi_link WHERE test_id = ? ORDER BY orb_timestamp`, testID)
	if err != nil {
		return
	}

	// Speed results
	speed, err = queryAsMapSlice(s.db, `SELECT * FROM speed_results WHERE test_id = ? ORDER BY orb_timestamp`, testID)
	if err != nil {
		return
	}

	// Scores
	scores, err = queryAsMapSlice(s.db, `SELECT * FROM scores WHERE test_id = ? ORDER BY orb_timestamp`, testID)
	return
}

// Helper functions

func nullStr(s string) sql.NullString {
	if s == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: s, Valid: true}
}

func nullInt(p *int) sql.NullInt64 {
	if p == nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: int64(*p), Valid: true}
}

func nullBool(p *bool) sql.NullBool {
	if p == nil {
		return sql.NullBool{}
	}
	return sql.NullBool{Bool: *p, Valid: true}
}

func nilInt64(p *int64) any {
	if p == nil {
		return nil
	}
	return *p
}

func scanRowAsMap(row *sql.Row, cols []string) (map[string]any, error) {
	vals := make([]any, len(cols))
	ptrs := make([]any, len(cols))
	for i := range vals {
		ptrs[i] = &vals[i]
	}
	if err := row.Scan(ptrs...); err != nil {
		return nil, err
	}
	m := make(map[string]any, len(cols))
	for i, col := range cols {
		m[col] = vals[i]
	}
	return m, nil
}

func queryAsMapSlice(db *sql.DB, query string, args ...any) ([]map[string]any, error) {
	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	var results []map[string]any
	for rows.Next() {
		vals := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return nil, err
		}
		m := make(map[string]any, len(cols))
		for i, col := range cols {
			m[col] = vals[i]
		}
		results = append(results, m)
	}
	return results, rows.Err()
}
