package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func newDumpCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "dump TEST_ID",
		Short: "Export test data as JSON",
		Args:  cobra.ExactArgs(1),
		RunE:  runDump,
	}
}

func runDump(cmd *cobra.Command, args []string) error {
	var testID int64
	if _, err := fmt.Sscanf(args[0], "%d", &testID); err != nil {
		return fmt.Errorf("invalid test ID: %s", args[0])
	}

	test, resp, wifi, speed, err := db.DumpTestData(testID)
	if err != nil {
		return fmt.Errorf("dump test %d: %w", testID, err)
	}

	output := map[string]any{
		"test":           test,
		"responsiveness": resp,
		"wifi_link":      wifi,
		"speed_results":  speed,
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(output)
}
