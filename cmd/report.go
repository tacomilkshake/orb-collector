package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/tacomilkshake/orb-collector/internal/report"
)

var reportDetail bool

func newReportCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "report",
		Short: "Generate comparison table with percentiles across all tests",
		RunE:  runReport,
	}
	cmd.Flags().BoolVar(&reportDetail, "detail", false, "Show per-test detail")
	return cmd
}

func runReport(cmd *cobra.Command, args []string) error {
	rows, err := db.GetReportRows()
	if err != nil {
		return fmt.Errorf("get report data: %w", err)
	}

	report.PrintReport(os.Stdout, rows)

	// Detail view with latency distribution
	if reportDetail {
		tests, err := db.GetCompletedTests()
		if err != nil {
			return fmt.Errorf("get tests: %w", err)
		}
		for _, t := range tests {
			fmt.Printf("\n## %s -- ch%d / %dMHz\n", t.Name, t.Channel, t.WidthMHz)
			if t.APName.Valid {
				fmt.Printf("  AP: %s", t.APName.String)
				if t.APRSSI.Valid {
					fmt.Printf(" RSSI=%d", t.APRSSI.Int64)
				}
				if t.APSNR.Valid {
					fmt.Printf(" SNR=%d", t.APSNR.Int64)
				}
				fmt.Println()
			}
			if t.EndTime.Valid {
				duration := t.EndTime.Time.Sub(t.StartTime)
				fmt.Printf("  Duration: %.0fs\n", duration.Seconds())
			}

			// Latency distribution
			buckets, err := db.DB().Query(`
				SELECT
					COALESCE(SUM(CASE WHEN latency_avg_us < 10000 THEN 1 ELSE 0 END), 0),
					COALESCE(SUM(CASE WHEN latency_avg_us >= 10000 AND latency_avg_us < 20000 THEN 1 ELSE 0 END), 0),
					COALESCE(SUM(CASE WHEN latency_avg_us >= 20000 AND latency_avg_us < 30000 THEN 1 ELSE 0 END), 0),
					COALESCE(SUM(CASE WHEN latency_avg_us >= 30000 AND latency_avg_us < 50000 THEN 1 ELSE 0 END), 0),
					COALESCE(SUM(CASE WHEN latency_avg_us >= 50000 THEN 1 ELSE 0 END), 0),
					COUNT(*)
				FROM responsiveness WHERE test_id = ?`, t.ID)
			if err != nil {
				continue
			}
			if buckets.Next() {
				var b [6]int64
				if err := buckets.Scan(&b[0], &b[1], &b[2], &b[3], &b[4], &b[5]); err != nil {
					continue
				}
				if b[5] > 0 {
					fmt.Println("  Latency distribution:")
					labels := []string{"<10ms", "10-20ms", "20-30ms", "30-50ms", ">50ms"}
					for i, label := range labels {
						pct := float64(b[i]) / float64(b[5]) * 100
						bar := ""
						for j := 0; j < int(pct/2); j++ {
							bar += "#"
						}
						fmt.Printf("    %8s: %4d (%5.1f%%) %s\n", label, b[i], pct, bar)
					}
				}
			}
			buckets.Close()
		}
	}

	return nil
}
