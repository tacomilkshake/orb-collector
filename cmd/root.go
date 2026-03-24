// Package cmd provides the CLI commands for orb-collector.
package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tacomilkshake/orb-collector/internal/connector"
	"github.com/tacomilkshake/orb-collector/internal/connector/omada"
	"github.com/tacomilkshake/orb-collector/internal/orb"
	"github.com/tacomilkshake/orb-collector/internal/store"
)

var (
	dbPath      string
	orbHost     string
	orbPort     int
	orbHosts    string // comma-separated host:port pairs
	orbServer   string
	apConnector string
	apURL       string
	clientMAC   string
	clientMACs  []string // parsed from comma-separated clientMAC
	apiPort     int
)

// OrbTarget represents a single Orb device to poll.
type OrbTarget struct {
	Client   *orb.Client
	DeviceID string // host used as device identifier
}

// Initialized in PersistentPreRunE.
var (
	db         *store.Store
	orbClient  *orb.Client // single client (legacy)
	orbTargets []OrbTarget // multiple orb targets
	apConn     connector.APConnector
)

var rootCmd = &cobra.Command{
	Use:   "orb-collector",
	Short: "WiFi performance test harness with Orb network sensors",
	Long: `orb-collector collects data from Orb network sensors and manages
AP radio settings via pluggable connectors. It supports:

  - Continuous data collection from Orb + AP polling
  - Test window management (begin/end)
  - Performance reports with percentile analysis
  - AP client stats via pluggable connectors`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// For begin/end/status: if the collector API is reachable, skip DB init
		// (these commands will proxy to the HTTP API instead)
		switch cmd.Name() {
		case "begin", "end", "status":
			if apiReachable(apiPort) {
				return nil // skip DB init; command will use HTTP API
			}
		}

		// Initialize store — use read-only for commands that don't write
		var err error
		switch cmd.Name() {
		case "status", "report", "dump":
			db, err = store.NewReadOnly(dbPath)
		default:
			db, err = store.New(dbPath)
		}
		if err != nil {
			return fmt.Errorf("open database: %w", err)
		}

		// Initialize Orb client(s)
		orbTargets = nil
		if orbHosts != "" {
			// Multi-orb mode: parse comma-separated host:port pairs
			for _, entry := range strings.Split(orbHosts, ",") {
				entry = strings.TrimSpace(entry)
				if entry == "" {
					continue
				}
				host, port := entry, orbPort
				if h, p, ok := strings.Cut(entry, ":"); ok {
					host = h
					parsed, err := fmt.Sscanf(p, "%d", &port)
					if err != nil || parsed != 1 {
						return fmt.Errorf("invalid orb-hosts port in %q: %w", entry, err)
					}
				}
				c := orb.NewClient(host, port)
				orbTargets = append(orbTargets, OrbTarget{Client: c, DeviceID: host})
			}
		} else {
			// Single-orb mode (legacy flags)
			c := orb.NewClient(orbHost, orbPort)
			orbTargets = append(orbTargets, OrbTarget{Client: c, DeviceID: orbHost})
		}
		orbClient = orbTargets[0].Client // keep for compatibility

		// Initialize AP connector
		switch apConnector {
		case "omada":
			apConn = omada.New(apURL)
		case "none", "":
			apConn = nil
		default:
			return fmt.Errorf("unknown AP connector: %s", apConnector)
		}

		// Parse comma-separated client MACs
		clientMACs = nil
		for _, m := range strings.Split(clientMAC, ",") {
			m = strings.TrimSpace(m)
			if m != "" {
				clientMACs = append(clientMACs, m)
			}
		}

		return nil
	},
	TraverseChildren: true,
	SilenceUsage:     true,
	SilenceErrors:    true,
}

// Execute runs the root command.
func Execute() error {
	err := rootCmd.Execute()
	if db != nil {
		db.Close()
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err)
	}
	return err
}

func init() {
	rootCmd.PersistentFlags().StringVar(&dbPath, "db", "orb-collector.duckdb", "DuckDB database path")
	rootCmd.PersistentFlags().StringVar(&orbHost, "orb-host", "10.0.1.47", "Orb sensor host (single orb)")
	rootCmd.PersistentFlags().IntVar(&orbPort, "orb-port", 8000, "Orb sensor port (single orb)")
	rootCmd.PersistentFlags().StringVar(&orbHosts, "orb-hosts", "", "Comma-separated orb host:port pairs (overrides --orb-host/--orb-port)")
	rootCmd.PersistentFlags().StringVar(&apConnector, "ap-connector", "omada", "AP connector (omada, none)")
	rootCmd.PersistentFlags().StringVar(&apURL, "ap-url", "http://omada-bridge:8086", "AP connector base URL")
	rootCmd.PersistentFlags().StringVar(&clientMAC, "client-mac", "20-F0-94-22-78-0D", "Client MAC address(es) to monitor (comma-separated for multiple)")
	rootCmd.PersistentFlags().StringVar(&orbServer, "orb-server", "", "Local Orb Server address (e.g. 10.0.1.5:7443) for status/report display")
	rootCmd.PersistentFlags().IntVar(&apiPort, "api-port", 8080, "HTTP API port for collector")

	rootCmd.AddCommand(
		newCollectCmd(),
		newBeginCmd(),
		newEndCmd(),
		newStatusCmd(),
		newReportCmd(),
		newDumpCmd(),
	)
}
