package cmd

import (
	"context"
	"fmt"
	"strings"

	"github.com/ai-la/cli/internal/auth"
	"github.com/ai-la/cli/internal/runner"
	"github.com/spf13/cobra"
)

var orgCmd = &cobra.Command{
	Use:     "org",
	Short:   "Organization context commands",
	GroupID: "context",
}

func init() {
	orgCmd.AddCommand(orgListCmd())
	orgCmd.AddCommand(orgCurrentCmd())
	orgCmd.AddCommand(orgUseCmd())
}

func orgListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List organizations you belong to",
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := runner.NewClientFromToken(nil)
			if err != nil {
				return err
			}
			resp, err := client.ListOrganizations(context.Background())
			if err != nil {
				return err
			}
			currentID := ""
			if ts, err := auth.LoadToken(); err == nil && ts != nil {
				currentID = strings.TrimSpace(ts.ActiveOrgID)
			}
			for _, item := range resp.Organizations {
				mark := " "
				if item.Organization.ID == currentID {
					mark = "*"
				}
				fmt.Printf("%s %s  %s\n", mark, item.Organization.ID, item.Organization.Name)
			}
			return nil
		},
	}
}

func orgCurrentCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "current",
		Short: "Show active organization context",
		RunE: func(cmd *cobra.Command, args []string) error {
			ts, err := auth.LoadToken()
			if err != nil || ts == nil || strings.TrimSpace(ts.ActiveOrgID) == "" {
				fmt.Println("No active organization selected.")
				return nil
			}
			fmt.Printf("Active organization: %s\n", strings.TrimSpace(ts.ActiveOrgID))
			if strings.TrimSpace(ts.ActiveProjectID) != "" {
				fmt.Printf("Active project: %s\n", strings.TrimSpace(ts.ActiveProjectID))
			}
			return nil
		},
	}
}

func orgUseCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "use <org-id>",
		Short: "Set active organization context",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			input := strings.TrimSpace(args[0])
			if input == "" {
				return fmt.Errorf("org id cannot be empty")
			}

			client, err := runner.NewClientFromToken(nil)
			if err != nil {
				return err
			}
			resp, err := client.ListOrganizations(context.Background())
			if err != nil {
				return err
			}

			matches := make([]runner.OrganizationWithRole, 0, 2)
			for _, item := range resp.Organizations {
				id := strings.TrimSpace(item.Organization.ID)
				if id == input || strings.HasPrefix(id, input) {
					matches = append(matches, item)
				}
			}
			if len(matches) == 0 {
				return fmt.Errorf("organization not found: %s", input)
			}
			if len(matches) > 1 {
				fmt.Println("Multiple organizations match this prefix. Please use a longer ID:")
				for _, item := range matches {
					fmt.Printf("- %s  %s\n", item.Organization.ID, item.Organization.Name)
				}
				return fmt.Errorf("ambiguous organization prefix")
			}
			orgID := strings.TrimSpace(matches[0].Organization.ID)

			ts, err := auth.LoadToken()
			if err != nil {
				return err
			}
			ts.ActiveOrgID = orgID
			// Organization switch invalidates previous active project context.
			ts.ActiveProjectID = ""
			if err := auth.SaveToken(ts); err != nil {
				return err
			}
			fmt.Printf("Active organization set to: %s\n", orgID)
			return nil
		},
	}
}
