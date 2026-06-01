package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/ai-la/cli/internal/auth"
)

func hasActiveOrgContext() bool {
	if strings.TrimSpace(os.Getenv("HEIMDAL_ORG_ID")) != "" {
		return true
	}
	ts, err := auth.LoadToken()
	if err != nil || ts == nil {
		return false
	}
	return strings.TrimSpace(ts.ActiveOrgID) != ""
}

func activeOrgContextID() string {
	if v := strings.TrimSpace(os.Getenv("HEIMDAL_ORG_ID")); v != "" {
		return v
	}
	ts, err := auth.LoadToken()
	if err != nil || ts == nil {
		return ""
	}
	return strings.TrimSpace(ts.ActiveOrgID)
}

func activeProjectContextID() string {
	ts, err := auth.LoadToken()
	if err != nil || ts == nil {
		return ""
	}
	return strings.TrimSpace(ts.ActiveProjectID)
}

func setActiveProjectContextID(projectID string) error {
	projectID = strings.TrimSpace(projectID)
	if projectID == "" {
		return fmt.Errorf("project id cannot be empty")
	}
	ts, err := auth.LoadToken()
	if err != nil {
		return err
	}
	ts.ActiveProjectID = projectID
	return auth.SaveToken(ts)
}
