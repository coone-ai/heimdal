package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// These are set at build time via ldflags:
//
//	-X github.com/ai-la/cli/cmd.Version=v1.2.3
//	-X github.com/ai-la/cli/cmd.Commit=abc1234
//	-X github.com/ai-la/cli/cmd.Date=2025-01-01T00:00:00Z
var (
	Version = "dev"
	Commit  = "none"
	Date    = "unknown"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Show CLI version",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("heimdal %s (commit: %s, built: %s)\n", DisplayVersion(), Commit, Date)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
