package cmd

import (
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"
)

const (
	orbPollInterval   = 1 * time.Second
	apPollInterval    = 30 * time.Second
	speedPollInterval = 60 * time.Second
	statusLogInterval = 60
)

func newCollectCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "collect",
		Short: "Continuously poll Orb (1s) and AP (30s), store in DuckDB",
		RunE:  runCollect,
	}
}

func runCollect(cmd *cobra.Command, args []string) error {
	fmt.Printf("[collector] DB: %s\n", dbPath)
	for _, t := range orbTargets {
		fmt.Printf("[collector] Orb: %s (device=%s) every %s\n", t.Client.BaseURL(), t.DeviceID, orbPollInterval)
	}
	if orbServer != "" {
		fmt.Printf("[collector] Orb Server: %s\n", orbServer)
	}
	fmt.Printf("[collector] Speed results: every %s\n", speedPollInterval)
	if apConn != nil {
		fmt.Printf("[collector] AP: %s (all clients) every %s\n", apConn.Name(), apPollInterval)
	}
	fmt.Println("[collector] Press Ctrl+C to stop")

	// Start HTTP API server
	startAPIServer(db, apiPort)

	// Write PID file
	pidPath := pidFilePath()
	if err := os.WriteFile(pidPath, []byte(fmt.Sprintf("%d", os.Getpid())), 0600); err != nil {
		return fmt.Errorf("write PID file: %w", err)
	}

	// Cleanup on signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	defer func() {
		os.Remove(pidPath)
		fmt.Println("[collector] Stopped.")
	}()

	// Per-target endpoint tracking
	type targetState struct {
		respEndpoint   string
		wifiEndpoint   string
		speedEndpoint  string
		scoresEndpoint string
	}
	targetStates := make([]targetState, len(orbTargets))

	var (
		pollCount     int
		totalResp     int
		totalWifi     int
		totalSpeed    int
		totalScores   int
		totalAP       int
		lastAPPoll    time.Time
		lastSpeedPoll time.Time
	)

	for {
		// Check for shutdown signal (non-blocking)
		select {
		case <-sigCh:
			return nil
		default:
		}

		loopStart := time.Now()

		// Get active test
		activeTest, err := db.GetActiveTest()
		if err != nil {
			fmt.Fprintf(os.Stderr, "[collector] get active test: %s\n", err)
		}
		var testID *int64
		if activeTest != nil {
			testID = &activeTest.ID
		}

		// Orb: poll each target every cycle (1s)
		var nResp, nWifi, nScores int
		for i, t := range orbTargets {
			ts := &targetStates[i]

			respRecords, respRaw, ep, _ := t.Client.FetchResponsivenessRaw()
			if len(respRecords) > 0 && ep != ts.respEndpoint {
				ts.respEndpoint = ep
				fmt.Printf("[collector] [%s] Using %s for responsiveness\n", t.DeviceID, ep)
			}
			n, _ := db.InsertResponsiveness(respRecords, respRaw, testID, t.DeviceID)
			nResp += n

			wifiRecords, wifiRaw, ep2, _ := t.Client.FetchWifiLinkRaw()
			if len(wifiRecords) > 0 && ep2 != ts.wifiEndpoint {
				ts.wifiEndpoint = ep2
				fmt.Printf("[collector] [%s] Using %s for wifi_link\n", t.DeviceID, ep2)
			}
			n, _ = db.InsertWifiLink(wifiRecords, wifiRaw, testID, t.DeviceID)
			nWifi += n

			scoresRecords, scoresRaw, ep3, _ := t.Client.FetchScoresRaw()
			if len(scoresRecords) > 0 && ep3 != ts.scoresEndpoint {
				ts.scoresEndpoint = ep3
				fmt.Printf("[collector] [%s] Using %s for scores\n", t.DeviceID, ep3)
			}
			n, _ = db.InsertScores(scoresRecords, scoresRaw, testID, t.DeviceID)
			nScores += n
		}

		// AP: poll every 30s (all wireless clients)
		if apConn != nil && time.Since(lastAPPoll) >= apPollInterval {
			clients, err := apConn.GetAllClients()
			if err != nil {
				fmt.Fprintf(os.Stderr, "[collector] AP GetAllClients: %s\n", err)
			} else {
				n, err := db.InsertAPSnapshots(testID, clients, apConn.Name())
				if err != nil {
					fmt.Fprintf(os.Stderr, "[collector] AP insert: %s\n", err)
				} else {
					totalAP += n
					fmt.Printf("[collector] AP: %d clients snapshot\n", n)
				}
			}
			lastAPPoll = time.Now()
		}

		// Speed results: poll every 60s (from each target)
		if time.Since(lastSpeedPoll) >= speedPollInterval {
			for i, t := range orbTargets {
				ts := &targetStates[i]
				speedRecords, speedRaw, ep, _ := t.Client.FetchSpeedResultsRaw()
				if len(speedRecords) > 0 && ep != ts.speedEndpoint {
					ts.speedEndpoint = ep
					fmt.Printf("[collector] [%s] Using %s for speed_results\n", t.DeviceID, ep)
				}
				nSpeed, _ := db.InsertSpeedResults(speedRecords, speedRaw, testID, t.DeviceID)
				totalSpeed += nSpeed
				if nSpeed > 0 {
					fmt.Printf("[collector] [%s] Speed: +%d records\n", t.DeviceID, nSpeed)
				}
			}
			lastSpeedPoll = time.Now()
		}

		pollCount++
		totalResp += nResp
		totalWifi += nWifi
		totalScores += nScores

		if pollCount%statusLogInterval == 0 {
			testLabel := "no active test"
			if activeTest != nil {
				testLabel = fmt.Sprintf("test=%s", activeTest.Name)
			}
			fmt.Printf("[collector] polls=%d resp=%d wifi=%d scores=%d speed=%d ap=%d orbs=%d | %s\n",
				pollCount, totalResp, totalWifi, totalScores, totalSpeed, totalAP, len(orbTargets), testLabel)
		}

		// Sleep remainder of 1s interval, but wake on signal
		elapsed := time.Since(loopStart)
		if sleep := orbPollInterval - elapsed; sleep > 0 {
			select {
			case <-sigCh:
				return nil
			case <-time.After(sleep):
			}
		}
	}
}

func pidFilePath() string {
	ext := filepath.Ext(dbPath)
	return strings.TrimSuffix(dbPath, ext) + ".pid"
}
