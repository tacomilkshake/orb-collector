package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"
)

func newStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show collector/test/latest reading status",
		RunE:  runStatus,
	}
}

func runStatus(cmd *cobra.Command, args []string) error {
	// If collector API is running, proxy the request
	if db == nil {
		return runStatusViaAPI()
	}

	// Check collector PID
	pidPath := pidFilePath()
	if data, err := os.ReadFile(pidPath); err == nil {
		pidStr := strings.TrimSpace(string(data))
		if pid, err := strconv.Atoi(pidStr); err == nil {
			if err := syscall.Kill(pid, 0); err == nil {
				fmt.Printf("[status] Collector running (PID %d)\n", pid)
			} else {
				fmt.Println("[status] Collector NOT running (stale PID file)")
				os.Remove(pidPath)
			}
		}
	} else {
		fmt.Println("[status] Collector NOT running")
	}

	// Orb Server
	if orbServer != "" {
		fmt.Printf("[status] Orb Server: %s\n", orbServer)
	}

	// Active test
	active, err := db.GetActiveTest()
	if err != nil {
		return fmt.Errorf("check active test: %w", err)
	}
	if active != nil {
		elapsed := time.Since(active.StartTime)
		respCount, _ := db.CountResponsiveness(active.ID)
		fmt.Printf("[status] Active test #%d: %s (ch%d/%dMHz)\n",
			active.ID, active.Name, active.Channel, active.WidthMHz)
		fmt.Printf("[status] Elapsed: %.0fs | Samples: %d\n", elapsed.Seconds(), respCount)
	} else {
		fmt.Println("[status] No active test")
	}

	// Latest reading
	latest, err := db.GetLatestReading()
	if err != nil {
		return fmt.Errorf("get latest: %w", err)
	}
	if latest != nil {
		age := time.Since(latest.OrbTimestamp)
		latencyMS := "nil"
		if latest.LatencyAvgUS.Valid {
			latencyMS = fmt.Sprintf("%.2fms", float64(latest.LatencyAvgUS.Int64)/1000.0)
		}
		jitterMS := "nil"
		if latest.JitterAvgUS.Valid {
			jitterMS = fmt.Sprintf("%.2fms", float64(latest.JitterAvgUS.Int64)/1000.0)
		}
		lossPct := "nil"
		if latest.PacketLossPct.Valid {
			lossPct = fmt.Sprintf("%.1f%%", latest.PacketLossPct.Float64)
		}
		ssid := ""
		if latest.NetworkName.Valid {
			ssid = latest.NetworkName.String
		}
		fmt.Printf("[status] Latest: latency=%s jitter=%s loss=%s SSID=%s (%.0fs ago)\n",
			latencyMS, jitterMS, lossPct, ssid, age.Seconds())
	}

	// Total counts
	tests, resp, wifi, speed, scores, _ := db.TotalCounts()
	fmt.Printf("[status] DB totals: %d tests, %d resp records, %d wifi records, %d scores records, %d speed records\n", tests, resp, wifi, scores, speed)

	return nil
}

func runStatusViaAPI() error {
	data, status, err := proxyToAPI("GET", apiURL(apiPort)+"/api/status", nil)
	if err != nil {
		return fmt.Errorf("API request failed: %w", err)
	}
	if status != 200 {
		return fmt.Errorf("API error (HTTP %d): %s", status, string(data))
	}

	var resp statusResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return fmt.Errorf("parse API response: %w", err)
	}

	if resp.CollectorRunning {
		fmt.Println("[status] Collector running (via API)")
	}

	if resp.ActiveTest != nil {
		t := resp.ActiveTest
		fmt.Printf("[status] Active test #%d: %s (ch%d/%dMHz)\n",
			t.TestID, t.Name, t.Channel, t.WidthMHz)
		fmt.Printf("[status] Elapsed: %.0fs | Samples: %d\n", t.ElapsedS, t.Samples)
	} else {
		fmt.Println("[status] No active test")
	}

	if resp.LatestReading != nil {
		lr := resp.LatestReading
		latencyMS := "nil"
		if lr.LatencyMS != nil {
			latencyMS = fmt.Sprintf("%.2fms", *lr.LatencyMS)
		}
		jitterMS := "nil"
		if lr.JitterMS != nil {
			jitterMS = fmt.Sprintf("%.2fms", *lr.JitterMS)
		}
		lossPct := "nil"
		if lr.LossPct != nil {
			lossPct = fmt.Sprintf("%.1f%%", *lr.LossPct)
		}
		ssid := ""
		if lr.NetworkName != nil {
			ssid = *lr.NetworkName
		}
		fmt.Printf("[status] Latest: latency=%s jitter=%s loss=%s SSID=%s (%.0fs ago)\n",
			latencyMS, jitterMS, lossPct, ssid, lr.AgeS)
	}

	fmt.Printf("[status] DB totals: %d tests, %d resp records, %d wifi records, %d scores records, %d speed records\n",
		resp.Totals.Tests, resp.Totals.Resp, resp.Totals.Wifi, resp.Totals.Scores, resp.Totals.Speed)

	return nil
}
