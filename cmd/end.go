package cmd

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

func newEndCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "end",
		Short: "Mark the end of the current test window",
		RunE:  runEnd,
	}
}

func runEnd(cmd *cobra.Command, args []string) error {
	// If collector API is running, proxy the request
	if db == nil {
		return runEndViaAPI()
	}

	active, err := db.GetActiveTest()
	if err != nil {
		return fmt.Errorf("check active test: %w", err)
	}
	if active == nil {
		fmt.Println("[end] No active test to end.")
		return nil
	}

	respTagged, wifiTagged, err := db.EndTest(active.ID)
	if err != nil {
		return fmt.Errorf("end test: %w", err)
	}

	duration := time.Since(active.StartTime)
	respCount, _ := db.CountResponsiveness(active.ID)
	wifiCount, _ := db.CountWifiLink(active.ID)

	fmt.Printf("[end] Test #%d: %s -- ended\n", active.ID, active.Name)
	fmt.Printf("[end] Duration: %.1fs\n", duration.Seconds())
	fmt.Printf("[end] Samples: %d responsiveness, %d wifi_link\n", respCount, wifiCount)
	if respTagged > 0 || wifiTagged > 0 {
		fmt.Printf("[end] Late-tagged: %d resp, %d wifi\n", respTagged, wifiTagged)
	}

	return nil
}

func runEndViaAPI() error {
	data, status, err := proxyToAPI("POST", apiURL(apiPort)+"/api/end", nil)
	if err != nil {
		return fmt.Errorf("API request failed: %w", err)
	}
	if status == 404 {
		fmt.Println("[end] No active test to end.")
		return nil
	}
	if status != 200 {
		return fmt.Errorf("API error (HTTP %d): %s", status, string(data))
	}

	var resp endResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return fmt.Errorf("parse API response: %w", err)
	}

	fmt.Printf("[end] Test #%d: %s -- ended (via API)\n", resp.TestID, resp.Name)
	fmt.Printf("[end] Duration: %.1fs\n", resp.DurationS)
	fmt.Printf("[end] Samples: %d resp, %d wifi, %d scores\n",
		resp.Samples.Resp, resp.Samples.Wifi, resp.Samples.Scores)
	return nil
}
