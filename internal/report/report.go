// Package report generates comparison tables from test data.
package report

import (
	"fmt"
	"io"
	"math"
	"time"

	"github.com/tacomilkshake/orb-collector/internal/store"
)

// Verdict returns a human-readable assessment of the test results.
func Verdict(avgMS, maxMS float64, missPct float64) string {
	var v string
	switch {
	case avgMS < 10:
		v = "excellent"
	case avgMS < 15:
		v = "very good"
	case avgMS < 25:
		v = "good"
	case avgMS < 40:
		v = "power_save_cycling"
	default:
		v = "high_latency"
	}

	if missPct > 10 {
		v += " +dropouts"
	} else if maxMS > avgMS*3 {
		v += " +spikes"
	}

	return v
}

// PrintReport writes the comparison table to w.
func PrintReport(w io.Writer, rows []store.ReportRow) {
	if len(rows) == 0 {
		fmt.Fprintln(w, "No completed tests found.")
		return
	}

	fmt.Fprintln(w)
	fmt.Fprintln(w, "# WiFi Channel/Width Test Results")
	fmt.Fprintf(w, "# Generated: %s\n", time.Now().UTC().Format(time.RFC3339))
	fmt.Fprintln(w)

	// Header
	fmt.Fprintf(w, "| %-28s | %3s | %5s | %5s | %5s | %7s | %7s | %7s | %7s | %7s | %7s | %7s | %7s | %7s | %5s | %5s | %4s | %6s | %-18s |\n",
		"Test", "Ch", "Width", "N", "Miss", "P05", "P10", "P50", "P90", "P95", "Avg", "Min", "Max", "Jitter", "Loss%", "RSSI", "SNR", "TxMbps", "Verdict")
	fmt.Fprintf(w, "|%s|%s|%s|%s|%s|%s|%s|%s|%s|%s|%s|%s|%s|%s|%s|%s|%s|%s|%s|\n",
		dashes(30), dashes(5), dashes(7), dashes(7), dashes(7),
		dashes(9), dashes(9), dashes(9), dashes(9), dashes(9),
		dashes(9), dashes(9), dashes(9), dashes(9), dashes(7),
		dashes(7), dashes(6), dashes(8), dashes(20))

	// Check if any test has speed data
	hasSpeed := false
	for _, r := range rows {
		if r.SpeedCount > 0 {
			hasSpeed = true
			break
		}
	}

	if hasSpeed {
		fmt.Fprintln(w)
		fmt.Fprintln(w, "# Throughput (Orb Server Speed Tests)")
		fmt.Fprintf(w, "| %-28s | %8s | %8s | %5s |\n", "Test", "DL Mbps", "UL Mbps", "Tests")
		fmt.Fprintf(w, "|%s|%s|%s|%s|\n", dashes(30), dashes(10), dashes(10), dashes(7))
		for _, r := range rows {
			if r.SpeedCount == 0 {
				continue
			}
			dlMbps := ""
			if r.AvgDownloadKbps.Valid {
				dlMbps = fmt.Sprintf("%.1f", r.AvgDownloadKbps.Float64/1000.0)
			}
			ulMbps := ""
			if r.AvgUploadKbps.Valid {
				ulMbps = fmt.Sprintf("%.1f", r.AvgUploadKbps.Float64/1000.0)
			}
			fmt.Fprintf(w, "| %-28s | %8s | %8s | %5d |\n", r.Name, dlMbps, ulMbps, r.SpeedCount)
		}
		fmt.Fprintln(w)
	}

	bestAvg := math.Inf(1)
	bestTest := ""

	for _, r := range rows {
		total := r.N + r.Missed
		missPct := 0.0
		if total > 0 {
			missPct = float64(r.Missed) / float64(total) * 100
		}

		verdict := Verdict(r.AvgMS, r.MaxMS, missPct)

		if r.AvgMS < bestAvg {
			bestAvg = r.AvgMS
			bestTest = r.Name
		}

		rssi := fmtOptWithFallback(r.AvgRSSI.Float64, r.AvgRSSI.Valid, r.APRSSI.Int64, r.APRSSI.Valid)
		snr := fmtOptWithFallback(r.AvgSNR.Float64, r.AvgSNR.Valid, r.APSNR.Int64, r.APSNR.Valid)
		txRate := ""
		if r.AvgTxRate.Valid {
			txRate = fmt.Sprintf("%.0f", r.AvgTxRate.Float64)
		}
		missStr := "0%"
		if r.Missed > 0 {
			missStr = fmt.Sprintf("%.0f%%", missPct)
		}

		fmt.Fprintf(w, "| %-28s | %3d | %4dM | %5d | %5s | %6.2f | %6.2f | %6.2f | %6.2f | %6.2f | %6.2f | %6.2f | %6.2f | %6.2f | %5.1f | %5s | %4s | %6s | %-18s |\n",
			r.Name, r.Channel, r.WidthMHz, r.N, missStr,
			r.P05MS, r.P10MS, r.P50MS, r.P90MS, r.P95MS,
			r.AvgMS, r.MinMS, r.MaxMS, r.JitterMS, r.LossPct,
			rssi, snr, txRate, verdict)
	}

	fmt.Fprintln(w)
	if bestTest != "" {
		fmt.Fprintf(w, "**Best: %s** (avg %.1fms)\n", bestTest, bestAvg)
	}
	fmt.Fprintln(w)
}

func dashes(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = '-'
	}
	return string(b)
}

func fmtOptWithFallback(wifiVal float64, wifiValid bool, fallbackVal int64, fallbackValid bool) string {
	if wifiValid {
		return fmt.Sprintf("%.0f", wifiVal)
	}
	if fallbackValid {
		return fmt.Sprintf("%d", fallbackVal)
	}
	return ""
}
