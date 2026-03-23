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
		fmt.Printf("[collector] AP: %s (clients=%s) every %s\n", apConn.Name(), strings.Join(clientMACs, ","), apPollInterval)
	}
	fmt.Println("[collector] Press Ctrl+C to stop")

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
		respEndpoint  string
		wifiEndpoint  string
		speedEndpoint string
		pollCount     int
		totalResp     int
		totalWifi     int
		totalSpeed    int
		totalAP       int
		lastAPPoll    time.Time
		lastSpeedPoll time.Time
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

		// AP: poll every 30s (iterate all client MACs)
		if apConn != nil && time.Since(lastAPPoll) >= apPollInterval {
			for _, mac := range clientMACs {
				info, err := apConn.GetClient(mac)
				if err == nil && info != nil {
					if err := db.InsertAPSnapshot(info, testID, apConn.Name(), mac); err == nil {
						totalAP++
						ps := "OFF"
						if info.PowerSave != nil && *info.PowerSave {
							ps = "ON"
						}
						ch := 0
						if info.Channel != nil {
							ch = *info.Channel
						}
						rssi := 0
						if info.RSSI != nil {
							rssi = *info.RSSI
						}
						snr := 0
						if info.SNR != nil {
							snr = *info.SNR
						}
						fmt.Printf("[collector] AP[%s]: ch%d RSSI=%d SNR=%d powerSave=%s\n", mac, ch, rssi, snr, ps)
					}
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

		if pollCount%statusLogInterval == 0 {
			testLabel := "no active test"
			if activeTest != nil {
				testLabel = fmt.Sprintf("test=%s", activeTest.Name)
			}
			fmt.Printf("[collector] polls=%d resp=%d wifi=%d speed=%d ap=%d | %s\n",
				pollCount, totalResp, totalWifi, totalSpeed, totalAP, testLabel)
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
