package cmd

import (
	"fmt"

	"github.com/ai-la/cli/internal/output"
	"github.com/spf13/cobra"
)

var demoCmd = &cobra.Command{
	Use:   "demo",
	Short: "Show a terminal UI example",
	Long:  "Displays styled terminal UI components similar to Claude Code.",
	Run: func(cmd *cobra.Command, args []string) {
		runDemo()
	},
}

func init() {
	rootCmd.AddCommand(demoCmd)
}

func runDemo() {
	pane := output.ClaudePane{
		Version: DisplayVersion(),
		Prompt:  "eval the refund agent and fix any regressions",
		Steps: []output.PaneStep{
			{output.ToolBash, "deepeval test run agents/checkout.py", "faithfulness 0.64", output.PaneStepWarn},
			{output.ToolEdit, "agents/retriever.py", "scoped to active refund policies", output.PaneStepInfo},
			{output.ToolBash, "deepeval test run agents/checkout.py", "faithfulness 0.98", output.PaneStepDone},
			{output.ToolNone, "All metrics green — ready to commit.", "", output.PaneStepInfo},
		},
		Input: `Try "ship it"`,
		Width: 74,
	}

	fmt.Println()
	fmt.Println(pane.Render())
	fmt.Println()
}
