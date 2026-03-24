package report

import (
	"bytes"
	"database/sql"
	"strings"
	"testing"

	"github.com/tacomilkshake/orb-collector/internal/store"
)

func TestVerdictExcellent(t *testing.T) {
	v := Verdict(5.0, 8.0, 0)
	if v != "excellent" {
		t.Errorf("verdict = %q, want excellent", v)
	}
}

func TestVerdictVeryGood(t *testing.T) {
	v := Verdict(12.0, 18.0, 0)
	if v != "very good" {
		t.Errorf("verdict = %q, want very good", v)
	}
}

func TestVerdictGood(t *testing.T) {
	v := Verdict(20.0, 30.0, 0)
	if v != "good" {
		t.Errorf("verdict = %q, want good", v)
	}
}

func TestVerdictPowerSaveCycling(t *testing.T) {
	v := Verdict(35.0, 45.0, 0)
	if v != "power_save_cycling" {
		t.Errorf("verdict = %q, want power_save_cycling", v)
	}
}

func TestVerdictHighLatency(t *testing.T) {
	v := Verdict(50.0, 60.0, 0)
	if v != "high_latency" {
		t.Errorf("verdict = %q, want high_latency", v)
	}
}

func TestVerdictWithDropouts(t *testing.T) {
	v := Verdict(5.0, 8.0, 15.0)
	if !strings.Contains(v, "+dropouts") {
		t.Errorf("verdict = %q, want +dropouts", v)
	}
}

func TestVerdictWithSpikes(t *testing.T) {
	// Max > 3 * avg, but miss < 10
	v := Verdict(5.0, 20.0, 5.0)
	if !strings.Contains(v, "+spikes") {
		t.Errorf("verdict = %q, want +spikes", v)
	}
}

func TestVerdictDropoutsOverridesSpikes(t *testing.T) {
	// Both conditions met: dropouts takes precedence
	v := Verdict(5.0, 20.0, 15.0)
	if !strings.Contains(v, "+dropouts") {
		t.Errorf("verdict = %q, want +dropouts", v)
	}
	if strings.Contains(v, "+spikes") {
		t.Errorf("verdict = %q, should not have +spikes when +dropouts", v)
	}
}

func TestPrintReportEmpty(t *testing.T) {
	var buf bytes.Buffer
	PrintReport(&buf, nil)
	if !strings.Contains(buf.String(), "No completed tests found") {
		t.Errorf("output = %q", buf.String())
	}
}

func TestPrintReportWithData(t *testing.T) {
	var buf bytes.Buffer
	rows := []store.ReportRow{
		{
			TestID:   1,
			Name:     "ch36_80",
			Channel:  36,
			WidthMHz: 80,
			N:        100,
			Missed:   2,
			P05MS:    3.5,
			P10MS:    4.0,
			P50MS:    7.0,
			P90MS:    12.0,
			P95MS:    15.0,
			AvgMS:    8.0,
			MinMS:    2.0,
			MaxMS:    20.0,
			JitterMS: 1.5,
			LossPct:  0.1,
		},
	}
	PrintReport(&buf, rows)

	out := buf.String()
	if !strings.Contains(out, "WiFi Channel/Width Test Results") {
		t.Error("missing header")
	}
	if !strings.Contains(out, "ch36_80") {
		t.Error("missing test name")
	}
	if !strings.Contains(out, "Best: ch36_80") {
		t.Error("missing best line")
	}
}

func TestPrintReportWithSpeed(t *testing.T) {
	var buf bytes.Buffer
	rows := []store.ReportRow{
		{
			TestID:          1,
			Name:            "speed_test",
			Channel:         149,
			WidthMHz:        160,
			N:               50,
			AvgMS:           6.0,
			AvgDownloadKbps: sql.NullFloat64{Float64: 500000, Valid: true},
			AvgUploadKbps:   sql.NullFloat64{Float64: 250000, Valid: true},
			SpeedCount:      5,
		},
	}
	PrintReport(&buf, rows)

	out := buf.String()
	if !strings.Contains(out, "Throughput") {
		t.Error("missing throughput section")
	}
	if !strings.Contains(out, "speed_test") {
		t.Error("missing test in throughput table")
	}
}

func TestPrintReportWithRSSI(t *testing.T) {
	var buf bytes.Buffer
	rows := []store.ReportRow{
		{
			TestID:   1,
			Name:     "rssi_test",
			Channel:  6,
			WidthMHz: 20,
			N:        50,
			AvgMS:    10.0,
			AvgRSSI:  sql.NullFloat64{Float64: -55, Valid: true},
			AvgSNR:   sql.NullFloat64{Float64: 30, Valid: true},
		},
	}
	PrintReport(&buf, rows)

	out := buf.String()
	if !strings.Contains(out, "-55") {
		t.Error("missing RSSI value")
	}
}

func TestPrintReportFallbackRSSI(t *testing.T) {
	var buf bytes.Buffer
	rows := []store.ReportRow{
		{
			TestID:   1,
			Name:     "fallback_test",
			Channel:  6,
			WidthMHz: 20,
			N:        50,
			AvgMS:    10.0,
			// No wifi RSSI, use AP fallback
			APRSSI: sql.NullInt64{Int64: -60, Valid: true},
			APSNR:  sql.NullInt64{Int64: 25, Valid: true},
		},
	}
	PrintReport(&buf, rows)

	out := buf.String()
	if !strings.Contains(out, "-60") {
		t.Error("missing fallback RSSI")
	}
}

func TestDashes(t *testing.T) {
	d := dashes(5)
	if d != "-----" {
		t.Errorf("dashes(5) = %q", d)
	}
	if dashes(0) != "" {
		t.Error("dashes(0) should be empty")
	}
}

func TestFmtOptWithFallback(t *testing.T) {
	tests := []struct {
		name          string
		wifiVal       float64
		wifiValid     bool
		fallbackVal   int64
		fallbackValid bool
		want          string
	}{
		{"wifi valid", -55.0, true, 0, false, "-55"},
		{"fallback valid", 0, false, -60, true, "-60"},
		{"both invalid", 0, false, 0, false, ""},
		{"wifi takes precedence", -55.0, true, -60, true, "-55"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := fmtOptWithFallback(tt.wifiVal, tt.wifiValid, tt.fallbackVal, tt.fallbackValid)
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestPrintReportMultipleTests(t *testing.T) {
	var buf bytes.Buffer
	rows := []store.ReportRow{
		{TestID: 1, Name: "test_a", Channel: 36, WidthMHz: 80, N: 50, AvgMS: 15.0},
		{TestID: 2, Name: "test_b", Channel: 149, WidthMHz: 160, N: 50, AvgMS: 8.0},
	}
	PrintReport(&buf, rows)

	out := buf.String()
	if !strings.Contains(out, "test_a") || !strings.Contains(out, "test_b") {
		t.Error("missing test names")
	}
	if !strings.Contains(out, "Best: test_b") {
		t.Error("test_b should be best (lower avg)")
	}
}
