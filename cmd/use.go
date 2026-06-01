package cmd

import (
	"fmt"
	"strings"

	"github.com/ai-la/cli/internal/runner"
	"github.com/spf13/cobra"
)

var useCmd = &cobra.Command{
	Use:     "use <project-id>",
	Short:   "Set active project context",
	GroupID: "context",
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := runner.NewClientFromToken(nil)
		if err != nil {
			return err
		}
		input := strings.TrimSpace(args[0])
		projectID, err := resolveProjectIDPrefixWithClient(client, input)
		if err != nil {
			return err
		}
		if projectID == "" {
			return fmt.Errorf("project id cannot be empty")
		}
		if err := setActiveProjectContextID(projectID); err != nil {
			return err
		}
		fmt.Printf("Active project set to: %s\n", projectID)
		return nil
	},
}

var projectCmd = &cobra.Command{
	Use:     "project",
	Short:   "Project context commands",
	GroupID: "context",
}

var projectCurrentCmd = &cobra.Command{
	Use:   "current",
	Short: "Show active project context",
	RunE: func(cmd *cobra.Command, args []string) error {
		projectID := activeProjectContextID()
		if projectID == "" {
			fmt.Println("No active project selected.")
			return nil
		}
		fmt.Printf("Active project: %s\n", projectID)
		return nil
	},
}

func init() {
	// Keep root-level access ergonomic: `heimdal use <project-id>`.
	rootCmd.AddCommand(useCmd)
	projectCmd.AddCommand(projectCurrentCmd)
	rootCmd.AddCommand(projectCmd)
}

func resolveProjectIDPrefix(input string) (string, error) {
	client, err := runner.NewClientFromToken(nil)
	if err != nil {
		return "", err
	}
	return resolveProjectIDPrefixWithClient(client, input)
}
