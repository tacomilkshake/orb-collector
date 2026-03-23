// Package main provides the orb-collector CLI.
package main

import (
	"os"

	"github.com/tacomilkshake/orb-collector/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
