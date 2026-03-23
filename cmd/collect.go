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
	orbDeviceID := orbHost // use host as device identifier

	fmt.Printf("[collector] DB: %s\n", dbPath)
	fmt.Printf("[collector] Orb: http://%s:%d (device=%s) every %s\n", orbHost, orbPort, orbDeviceID, orbPollInterval)
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
	if err := os.WriteFile(pidPath, []byte(fmt.Sprintf("%d", os.Getpid())), 0644); err != nil {
		return fmt.Errorf("write PID file: %w", err)
	}

	// Cleanup on signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		os.Remove(pidPath)
		fmt.Println("\n[collector] Stopped.")
		os.Exit(0)
	}()

	var (
		respEndpoint   string
		wifiEndpoint   string
		speedEndpoint  string
		scoresEndpoint string
		pollCount      int
		totalResp      int
		totalWifi      int
		totalSpeed     int
		totalScores    int
		totalAP        int
		lastAPPoll     time.Time
		lastSpeedPoll  time.Time
	)

	for {
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

		// Orb: poll every cycle (1s)
		respRecords, respRaw, ep, _ := orbClient.FetchResponsivenessRaw()
		if len(respRecords) > 0 && ep != respEndpoint {
			respEndpoint = ep
			fmt.Printf("[collector] Using %s for responsiveness\n", ep)
		}
		nResp, _ := db.InsertResponsiveness(respRecords, respRaw, testID, orbDeviceID)

		wifiRecords, wifiRaw, ep, _ := orbClient.FetchWifiLinkRaw()
		if len(wifiRecords) > 0 && ep != wifiEndpoint {
			wifiEndpoint = ep
			fmt.Printf("[collector] Using %s for wifi_link\n", ep)
		}
		nWifi, _ := db.InsertWifiLink(wifiRecords, wifiRaw, testID, orbDeviceID)

		// Scores: poll every cycle (1s)
		scoresRecords, scoresRaw, ep, _ := orbClient.FetchScoresRaw()
		if len(scoresRecords) > 0 && ep != scoresEndpoint {
			scoresEndpoint = ep
			fmt.Printf("[collector] Using %s for scores\n", ep)
		}
		nScores, _ := db.InsertScores(scoresRecords, scoresRaw, testID, orbDeviceID)

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

		// Speed results: poll every 60s
		if time.Since(lastSpeedPoll) >= speedPollInterval {
			speedRecords, speedRaw, ep, _ := orbClient.FetchSpeedResultsRaw()
			if len(speedRecords) > 0 && ep != speedEndpoint {
				speedEndpoint = ep
				fmt.Printf("[collector] Using %s for speed_results\n", ep)
			}
			nSpeed, _ := db.InsertSpeedResults(speedRecords, speedRaw, testID, orbDeviceID)
			totalSpeed += nSpeed
			if nSpeed > 0 {
				fmt.Printf("[collector] Speed: +%d records\n", nSpeed)
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
			fmt.Printf("[collector] polls=%d resp=%d wifi=%d scores=%d speed=%d ap=%d | %s\n",
				pollCount, totalResp, totalWifi, totalScores, totalSpeed, totalAP, testLabel)
		}

		// Sleep remainder of 1s interval
		elapsed := time.Since(loopStart)
		if sleep := orbPollInterval - elapsed; sleep > 0 {
			time.Sleep(sleep)
		}
	}
}

func pidFilePath() string {
	ext := filepath.Ext(dbPath)
	return strings.TrimSuffix(dbPath, ext) + ".pid"
}
