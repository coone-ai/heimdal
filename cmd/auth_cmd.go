package cmd

import (
	"fmt"

	"github.com/ai-la/cli/internal/auth"
	"github.com/spf13/cobra"
)

var authCmd = &cobra.Command{
	Use:     "auth",
	Short:   "Session management commands",
	GroupID: "session",
}

var authLoginCmd = &cobra.Command{
	Use:   "login",
	Short: "Sign in",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runLogin()
	},
}

var authLogoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Sign out",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := auth.ClearToken(); err != nil {
			return err
		}
		fmt.Println("Logged out.")
		return nil
	},
}

func init() {
	authLoginCmd.Flags().StringVar(&loginAppURL, "app-url", "", "Login app URL override")
	authLoginCmd.Flags().StringVar(&loginAPIURL, "api-url", "", "CLI API base URL override")
	authLoginCmd.Flags().BoolVar(&loginDev, "dev", false, "Use dev URLs (app: http://localhost, api: http://localhost:5002)")

	authCmd.AddCommand(authLoginCmd)
	authCmd.AddCommand(authLogoutCmd)
	rootCmd.AddCommand(authCmd)
}
