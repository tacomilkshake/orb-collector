package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"github.com/tacomilkshake/orb-collector/internal/store"
)

var beginFlags struct {
	channel   int
	width     int
	freq      int
	ap        string
	rssi      int
	snr       int
	apChannel int
	powerSave bool
	rxRate    int
	txRate    int
	wifiMode  string
	notes     string
	// Track which flags were explicitly set
	rssiSet      bool
	snrSet       bool
	apChannelSet bool
	powerSaveSet bool
	rxRateSet    bool
	txRateSet    bool
}

func newBeginCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "begin NAME",
		Short: "Mark the start of a test window",
		Args:  cobra.ExactArgs(1),
		RunE:  runBegin,
	}

	cmd.Flags().IntVar(&beginFlags.channel, "channel", 0, "Channel number (required)")
	cmd.Flags().IntVar(&beginFlags.width, "width", 0, "Channel width in MHz (required)")
	cmd.Flags().IntVar(&beginFlags.freq, "freq", 0, "Frequency in MHz (required)")
	cmd.Flags().StringVar(&beginFlags.ap, "ap", "", "AP name")
	cmd.Flags().IntVar(&beginFlags.rssi, "rssi", 0, "AP-reported RSSI")
	cmd.Flags().IntVar(&beginFlags.snr, "snr", 0, "AP-reported SNR")
	cmd.Flags().IntVar(&beginFlags.apChannel, "ap-channel", 0, "AP-reported channel")
	cmd.Flags().BoolVar(&beginFlags.powerSave, "power-save", false, "Power save enabled")
	cmd.Flags().IntVar(&beginFlags.rxRate, "rx-rate", 0, "AP-reported RX rate")
	cmd.Flags().IntVar(&beginFlags.txRate, "tx-rate", 0, "AP-reported TX rate")
	cmd.Flags().StringVar(&beginFlags.wifiMode, "wifi-mode", "", "WiFi mode")
	cmd.Flags().StringVar(&beginFlags.notes, "notes", "", "Test notes")

	_ = cmd.MarkFlagRequired("channel")
	_ = cmd.MarkFlagRequired("width")
	_ = cmd.MarkFlagRequired("freq")

	return cmd
}

func runBegin(cmd *cobra.Command, args []string) error {
	name := args[0]

	// If collector API is running, proxy the request
	if db == nil {
		return runBeginViaAPI(name)
	}

	// Auto-end any active test
	active, err := db.GetActiveTest()
	if err != nil {
		return fmt.Errorf("check active test: %w", err)
	}
	if active != nil {
		db.EndTest(active.ID)
		fmt.Printf("[begin] Auto-ended previous test: %s\n", active.Name)
	}

	// Build params with optional fields
	params := store.BeginTestParams{
		Name:       name,
		Channel:    beginFlags.channel,
		WidthMHz:   beginFlags.width,
		FreqMHz:    beginFlags.freq,
		APPlatform: "",
		APName:     beginFlags.ap,
		APWifiMode: beginFlags.wifiMode,
		Notes:      beginFlags.notes,
	}

	if apConn != nil {
		params.APPlatform = apConn.Name()
	}

	if cmd.Flags().Changed("rssi") {
		params.APRSSI = &beginFlags.rssi
	}
	if cmd.Flags().Changed("snr") {
		params.APSNR = &beginFlags.snr
	}
	if cmd.Flags().Changed("ap-channel") {
		params.APChannel = &beginFlags.apChannel
	}
	if cmd.Flags().Changed("power-save") {
		params.APPowerSave = &beginFlags.powerSave
	}
	if cmd.Flags().Changed("rx-rate") {
		params.APRxRate = &beginFlags.rxRate
	}
	if cmd.Flags().Changed("tx-rate") {
		params.APTxRate = &beginFlags.txRate
	}

	id, err := db.BeginTest(params)
	if err != nil {
		return fmt.Errorf("begin test: %w", err)
	}

	fmt.Printf("[begin] Test #%d: %s\n", id, name)
	fmt.Printf("[begin] ch%d / %dMHz / %dMHz\n", beginFlags.channel, beginFlags.width, beginFlags.freq)
	if beginFlags.ap != "" {
		fmt.Printf("[begin] AP: %s RSSI=%d SNR=%d\n", beginFlags.ap, beginFlags.rssi, beginFlags.snr)
	}
	fmt.Printf("[begin] Started at %s\n", time.Now().UTC().Format(time.RFC3339))

	return nil
}

func runBeginViaAPI(name string) error {
	reqBody := beginRequest{
		Name:    name,
		Channel: beginFlags.channel,
		Width:   beginFlags.width,
		Freq:    beginFlags.freq,
	}
	body, _ := json.Marshal(reqBody)

	data, status, err := proxyToAPI("POST", apiURL(apiPort)+"/api/begin", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("API request failed: %w", err)
	}
	if status != 200 {
		return fmt.Errorf("API error (HTTP %d): %s", status, string(data))
	}

	var resp beginResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return fmt.Errorf("parse API response: %w", err)
	}

	fmt.Printf("[begin] Test #%d: %s (via API)\n", resp.TestID, resp.Name)
	fmt.Printf("[begin] Started at %s\n", resp.StartedAt)
	return nil
}
