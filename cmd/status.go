package cmd

import (
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
