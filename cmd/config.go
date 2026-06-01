package cmd

import (
	"fmt"
	"os"

	"github.com/ai-la/cli/internal/output"
	"github.com/spf13/cobra"

	"github.com/ai-la/cli/internal/config"
)

var configCmd = &cobra.Command{
	Use:     "config",
	Short:   "Configuration commands",
	GroupID: "setup",
}

var validateCmd = &cobra.Command{
	Use:           "validate",
	Short:         "Validate heimdal.yaml",
	SilenceUsage:  true,
	SilenceErrors: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		path, _ := cmd.Flags().GetString("file")

		cfg, err := config.Load(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "\n  %s %v\n\n", output.ErrorMark(), err)
			os.Exit(2)
			return nil
		}

		res := config.Validate(cfg, path)

		fmt.Println()
		for _, c := range res.Checks {
			if c.OK {
				fmt.Printf("  %s %s\n", output.SuccessMark(), c.Label)
			} else {
				detail := ""
				if c.Detail != "" {
					detail = " — " + c.Detail
				}
				fmt.Printf("  %s %s%s\n", output.ErrorMark(), c.Label, detail)
			}
		}
		fmt.Println()

		if res.Valid {
			fmt.Printf("  %s\n\n", output.SuccessMark()+" Config valid.")
			return nil
		}

		fmt.Fprintf(os.Stderr, "  %s\n\n", output.ErrorMark()+" Config invalid.")
		os.Exit(2)
		return nil
	},
}

func init() {
	validateCmd.Flags().StringP("file", "f", "heimdal.yaml", "Config file path")
	configCmd.AddCommand(validateCmd)
}
