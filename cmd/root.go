package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var globalDev bool

var rootCmd = &cobra.Command{
	Use:     "heimdal",
	Aliases: []string{"coval"},
	Short: "Heimdal CLI — Auto Run Test",
	Long: `Heimdal CLI — Auto Run Test

Quick start:
  1) auth login
  2) org list
  3) org use <org-id>
  4) use <project-id>
  5) test auto --test-id AT-01 --scenario A
`,
	SilenceUsage:  true,
	SilenceErrors: true,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		if globalDev {
			_ = os.Setenv("HEIMDAL_DEV", "1")
		}
	},
}

func init() {
	rootCmd.AddGroup(
		&cobra.Group{ID: "context", Title: "Context Commands"},
		&cobra.Group{ID: "setup", Title: "Project Setup Commands"},
		&cobra.Group{ID: "run", Title: "Test Execution Commands"},
		&cobra.Group{ID: "session", Title: "Session Commands"},
	)

	rootCmd.PersistentFlags().BoolVar(&globalDev, "dev", false, "Use dev environment defaults (app: http://localhost, api: http://localhost:5002)")
	_ = rootCmd.PersistentFlags().MarkHidden("dev")
	loginCmd.Hidden = true
	logoutCmd.Hidden = true
	loginCmd.GroupID = "session"
	logoutCmd.GroupID = "session"
	initCmd.GroupID = "setup"
	configCmd.GroupID = "setup"

	rootCmd.AddCommand(loginCmd)
	rootCmd.AddCommand(logoutCmd)
	rootCmd.AddCommand(orgCmd)
	rootCmd.AddCommand(initCmd)
	rootCmd.AddCommand(configCmd)
	// testCmd kendi init()'inde rootCmd'ye ekleniyor (test.go)
}

// Execute is the entry point called from main.go.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
