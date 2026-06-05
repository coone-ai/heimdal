package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/ai-la/cli/internal/config"
	"github.com/ai-la/cli/internal/output"
	"github.com/ai-la/cli/internal/runner"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var autoCmd = &cobra.Command{
	Use:     "auto",
	Short:   "Auto Run Test commands",
	GroupID: "run",
}

func init() {
	rootCmd.AddCommand(autoCmd)
	rootProjectsCmd := autoProjectsCmd()
	rootProjectsCmd.GroupID = "setup"
	rootCmd.AddCommand(rootProjectsCmd)
	rootIntegrationsCmd := autoIntegrationsCmd()
	rootIntegrationsCmd.GroupID = "setup"
	rootCmd.AddCommand(rootIntegrationsCmd)
	rootKnowledgeBasesCmd := autoKnowledgeBasesCmd()
	rootKnowledgeBasesCmd.GroupID = "setup"
	rootCmd.AddCommand(rootKnowledgeBasesCmd)
	runCmd := buildAutoCmd()
	runCmd.Use = "run"
	runCmd.Short = "Run an Auto Run test"
	autoCmd.AddCommand(runCmd)
	autoCmd.AddCommand(autoRunsCmd())
	autoCmd.AddCommand(autoDatasetsCmd())
	autoCmd.AddCommand(autoResultsCmd())
	autoCmd.AddCommand(autoProjectsCmd())
	autoCmd.AddCommand(autoIntegrationsCmd())
	autoCmd.AddCommand(autoKnowledgeBasesCmd())
	// Backward-compat alias; keep hidden so UX stays focused on `auto runs`.
	testsAliasCmd := autoTestsCmd()
	testsAliasCmd.Hidden = true
	autoCmd.AddCommand(testsAliasCmd)
}

func autoTestsCmd() *cobra.Command {
	var configPath string
	cmd := &cobra.Command{
		Use:   "tests",
		Short: "List available Auto Run test suites",
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := clientFromConfig(configPath)
			if err != nil {
				return err
			}
			return printAutoRunTestsCatalog(client, "")
		},
	}
	cmd.Flags().StringVar(&configPath, "config", "./heimdal.yaml", "Config file path")
	return cmd
}

func autoRunsCmd() *cobra.Command {
	var configPath, projectID, testID, scenario string
	var limit, offset int
	cmd := &cobra.Command{
		Use:   "runs",
		Short: "List Auto Run execution history",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, client, err := configAndClient(configPath)
			if err != nil {
				return err
			}
			projectID = firstNonEmptyLocal(projectID, activeProjectContextID(), cfg.Project.ID)
			if strings.TrimSpace(projectID) == "" {
				// Unified UX: `auto runs` can also act as the test catalog entrypoint.
				return printAutoRunTestsCatalog(client, testID)
			}
			projectID, err = resolveProjectIDPrefixWithClient(client, projectID)
			if err != nil {
				return err
			}
			if projectID == "" {
				return fmt.Errorf("--project or project.id is required")
			}
			resp, err := client.ListAutoRunRuns(context.Background(), projectID, testID, scenario, limit, offset)
			if err != nil {
				return err
			}
			for _, r := range resp.Runs {
				score := "-"
				if r.OverallScore != nil {
					score = fmt.Sprintf("%.1f", *r.OverallScore)
				}
				shortID := r.ID
				if len(shortID) > 5 {
					shortID = shortID[:5]
				}
				fmt.Printf("%-5s  %-5s %s  %-9s score=%-6s verdict=%-5s version=%s\n",
					shortID, r.TestID, r.Scenario, r.Status, score, r.Verdict, r.RunVersion)
			}
			fmt.Printf("total=%d\n", resp.Total)
			if strings.TrimSpace(testID) != "" {
				fmt.Println()
				if err := printAutoRunTestsCatalog(client, testID); err != nil {
					return err
				}
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&configPath, "config", "./heimdal.yaml", "Config file path")
	cmd.Flags().StringVar(&projectID, "project", "", "Project ID")
	cmd.Flags().StringVar(&testID, "test-id", "", "Test ID filtresi")
	cmd.Flags().StringVar(&scenario, "scenario", "", "Scenario filtresi")
	cmd.Flags().IntVar(&limit, "limit", 20, "Limit")
	cmd.Flags().IntVar(&offset, "offset", 0, "Offset")
	return cmd
}

func autoDatasetsCmd() *cobra.Command {
	var configPath, projectID, testID, scenario string
	var limit, offset int
	cmd := &cobra.Command{
		Use:   "datasets",
		Short: "List frozen benchmark snapshots",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, client, err := configAndClient(configPath)
			if err != nil {
				return err
			}
			projectID = firstNonEmptyLocal(projectID, activeProjectContextID(), cfg.Project.ID)
			projectID, err = resolveProjectIDPrefixWithClient(client, projectID)
			if err != nil {
				return err
			}
			testID = firstNonEmptyLocal(testID, cfg.AutoRun.TestID)
			scenario = firstNonEmptyLocal(scenario, cfg.AutoRun.Scenario)
			if projectID == "" || testID == "" {
				return fmt.Errorf("--project and --test-id are required")
			}
			resp, err := client.ListAutoRunDatasets(context.Background(), projectID, testID, scenario, limit, offset)
			if err != nil {
				return err
			}
			for _, d := range resp.Datasets {
				fmt.Printf("%s  %-5s %s  questions=%d version=%s hash=%s created=%s\n",
					d.ID, d.TestID, d.Scenario, d.QuestionCount, d.BenchmarkVersion, d.SnapshotHash, d.CreatedAt)
			}
			fmt.Printf("total=%d\n", resp.Total)
			return nil
		},
	}
	cmd.Flags().StringVar(&configPath, "config", "./heimdal.yaml", "Config file path")
	cmd.Flags().StringVar(&projectID, "project", "", "Project ID")
	cmd.Flags().StringVar(&testID, "test-id", "", "Test ID")
	cmd.Flags().StringVar(&scenario, "scenario", "", "Scenario")
	cmd.Flags().IntVar(&limit, "limit", 50, "Limit")
	cmd.Flags().IntVar(&offset, "offset", 0, "Offset")
	return cmd
}

func autoResultsCmd() *cobra.Command {
	var configPath, projectID string
	cmd := &cobra.Command{
		Use:   "results <test-run-id>",
		Short: "Show Auto Run test result details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, client, err := configAndClient(configPath)
			if err != nil {
				return err
			}

			runID := strings.TrimSpace(args[0])
			if !looksLikeFullUUID(runID) {
				projectID = firstNonEmptyLocal(projectID, activeProjectContextID(), cfg.Project.ID)
				projectID, err = resolveProjectIDPrefixWithClient(client, projectID)
				if err != nil {
					return err
				}
				if projectID == "" {
					return fmt.Errorf("--project (or active project) is required when using a short run id prefix")
				}
				runID, err = resolveRunIDPrefixWithClient(client, projectID, runID)
				if err != nil {
					return err
				}
			}

			resp, err := client.GetAutoRunResults(context.Background(), runID)
			if err != nil {
				return err
			}
			if resp.TestRun == nil {
				return fmt.Errorf("result is not ready yet: status=%s progress=%.0f", resp.Status, resp.Progress)
			}
			report := &runner.RunReport{ProjectID: "", RunID: resp.TestRun.ID}
			// Keep the terminal renderer consistent with `heimdal test auto`.
			// The API returns scores on a 0-100 scale; the renderer expects 0-1.
			score := 0.0
			if resp.TestRun.OverallScore != nil {
				score = *resp.TestRun.OverallScore
			}
			report.TotalCases = resp.TestRun.TotalQuestions
			report.Summary = &runner.RunSummary{
				ReliabilityScore: score / 100,
				TotalCases:       resp.TestRun.TotalQuestions,
				FailedCases:      resp.TestRun.FailedQuestions + resp.TestRun.ErrorQuestions,
				Decision:         strings.ToLower(resp.TestRun.Verdict),
			}
			if strings.EqualFold(resp.TestRun.Verdict, "FAIL") {
				report.ExitCode = 1
			}
			output.PrintReport(report)
			return nil
		},
	}
	cmd.Flags().StringVar(&configPath, "config", "./heimdal.yaml", "Config file path")
	cmd.Flags().StringVar(&projectID, "project", "", "Project ID (required for short run-id prefixes)")
	return cmd
}

func autoIntegrationsCmd() *cobra.Command {
	var configPath, projectID string
	cmd := &cobra.Command{
		Use:   "integrations",
		Short: "Manage project integrations",
		Long: `Manage backend integrations used by Auto Run tests.

Subcommands:
  (no subcommand)  List integrations in the active project context.
  pull             Read an existing backend integration config into heimdal.yaml.
  apply            Push the integration block from heimdal.yaml to backend and publish it.
`,
		Example: strings.TrimSpace(`
  heimdal integrations --project <project-id>
  heimdal integrations pull <integration-id>
  heimdal integrations apply
  heimdal integrations apply --integration <integration-id>
`),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 1 && (args[0] == "help" || args[0] == "?") {
				return cmd.Help()
			}
			if len(args) > 0 {
				return fmt.Errorf("unexpected arguments: %s (use `heimdal integrations --help`)", strings.Join(args, " "))
			}
			if !hasActiveOrgContext() {
				return fmt.Errorf("no active organization selected; run `heimdal org list` and `heimdal org use <org-id>` first")
			}
			cfg, client, err := configAndClient(configPath)
			if err != nil {
				return err
			}
			projectID = firstNonEmptyLocal(projectID, activeProjectContextID(), cfg.Project.ID)
			projectID, err = resolveProjectIDPrefixWithClient(client, projectID)
			if err != nil {
				return err
			}
			resp, err := client.ListIntegrations(context.Background(), projectID)
			if err != nil {
				return err
			}
			for _, i := range resp.Integrations {
				active := ""
				if i.IsActive {
					active = " active"
				}
				fmt.Printf("%s  %s%s\n", i.ID, i.Name, active)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&configPath, "config", "./heimdal.yaml", "Config file path")
	cmd.Flags().StringVar(&projectID, "project", "", "Project ID")
	cmd.AddCommand(autoIntegrationsPullCmd())
	cmd.AddCommand(autoIntegrationsApplyCmd())
	return cmd
}

func autoIntegrationsPullCmd() *cobra.Command {
	var configPath, projectID string
	cmd := &cobra.Command{
		Use:   "pull <integration-id>",
		Short: "Pull active backend integration config into heimdal.yaml",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, client, err := configAndClient(configPath)
			if err != nil {
				return err
			}

			projectID = firstNonEmptyLocal(projectID, activeProjectContextID(), cfg.Project.ID)
			projectID, err = resolveProjectIDPrefixWithClient(client, projectID)
			if err != nil {
				return err
			}

			integrationID, err := resolveIntegrationIDPrefixWithClient(client, projectID, args[0])
			if err != nil {
				return err
			}

			activeCfg, err := client.GetActiveIntegrationConfig(context.Background(), integrationID)
			if err != nil {
				return err
			}

			doc, err := loadOrCreateConfigDocument(configPath)
			if err != nil {
				return err
			}
			if projectID != "" {
				setPath(doc, []string{"project", "id"}, projectID)
			}
			setPath(doc, []string{"auto_run", "integration_id"}, integrationID)
			mergePulledIntegration(doc, activeCfg)
			if err := saveConfigDocument(configPath, doc); err != nil {
				return err
			}

			fmt.Printf("Pulled integration %s into %s\n", integrationID, configPath)
			return nil
		},
	}
	cmd.Flags().StringVar(&configPath, "config", "./heimdal.yaml", "Config file path")
	cmd.Flags().StringVar(&projectID, "project", "", "Project ID")
	return cmd
}

func autoIntegrationsApplyCmd() *cobra.Command {
	var configPath, projectID, integrationID string
	cmd := &cobra.Command{
		Use:   "apply",
		Short: "Apply integration block from heimdal.yaml to backend",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, client, err := configAndClient(configPath)
			if err != nil {
				return err
			}

			projectID = firstNonEmptyLocal(projectID, activeProjectContextID(), cfg.Project.ID)
			projectID, err = resolveProjectIDPrefixWithClient(client, projectID)
			if err != nil {
				return err
			}
			if strings.TrimSpace(projectID) == "" {
				return fmt.Errorf("project id is required; use `heimdal use <project-id>` or pass `--project`")
			}

			intCfg := cfg.Integration
			if strings.TrimSpace(intCfg.Endpoint) == "" && strings.TrimSpace(intCfg.BaseURL) != "" {
				intCfg.Endpoint = intCfg.BaseURL
			}
			if strings.TrimSpace(intCfg.Endpoint) == "" {
				return fmt.Errorf("integration.endpoint is required in %s", configPath)
			}

			integrationID = firstNonEmptyLocal(integrationID, cfg.AutoRun.IntegrationID)
			if strings.TrimSpace(integrationID) != "" {
				integrationID, err = resolveIntegrationIDPrefixWithClient(client, projectID, integrationID)
				if err != nil {
					return err
				}
			}
			if strings.TrimSpace(integrationID) == "" {
				name := strings.TrimSpace(cfg.Project.Name)
				if name == "" {
					name = "Heimdal CLI Integration"
				}
				created, err := client.CreateIntegration(context.Background(), runner.IntegrationCreateRequest{
					Name:        name,
					Description: "Managed from heimdal.yaml",
					ProjectID:   projectID,
				})
				if err != nil {
					return err
				}
				integrationID = strings.TrimSpace(created.ID)
				if integrationID == "" {
					return fmt.Errorf("integration create succeeded but no integration id returned")
				}
			}

			req, err := buildIntegrationConfigCreateRequest(intCfg, projectID)
			if err != nil {
				return err
			}
			createdCfg, err := client.CreateIntegrationConfig(context.Background(), integrationID, req)
			if err != nil {
				return err
			}
			configID := strings.TrimSpace(createdCfg.Config.ID)
			if configID == "" {
				return fmt.Errorf("integration config create succeeded but no config id returned")
			}
			if _, err := client.PublishIntegrationConfig(context.Background(), integrationID, configID); err != nil {
				return err
			}

			doc, err := loadOrCreateConfigDocument(configPath)
			if err != nil {
				return err
			}
			setPath(doc, []string{"project", "id"}, projectID)
			setPath(doc, []string{"auto_run", "integration_id"}, integrationID)
			if err := saveConfigDocument(configPath, doc); err != nil {
				return err
			}

			fmt.Printf("Applied integration config to backend integration: %s\n", integrationID)
			return nil
		},
	}
	cmd.Flags().StringVar(&configPath, "config", "./heimdal.yaml", "Config file path")
	cmd.Flags().StringVar(&projectID, "project", "", "Project ID")
	cmd.Flags().StringVar(&integrationID, "integration", "", "Integration ID (existing backend integration)")
	return cmd
}

func autoProjectsCmd() *cobra.Command {
	var configPath string
	cmd := &cobra.Command{
		Use:   "projects",
		Short: "List accessible projects",
		RunE: func(cmd *cobra.Command, args []string) error {
			if !hasActiveOrgContext() {
				return fmt.Errorf("no active organization selected; run `heimdal org list` and `heimdal org use <org-id>` first")
			}
			client, err := clientFromConfig(configPath)
			if err != nil {
				return err
			}
			resp, err := client.ListProjects(context.Background())
			if err != nil {
				return err
			}
			activeOrgID := activeOrgContextID()
			for _, p := range resp.Projects {
				if activeOrgID != "" && !strings.EqualFold(strings.TrimSpace(p.OrganizationID), activeOrgID) {
					continue
				}
				fmt.Printf("%s  %s\n", p.ID, p.Name)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&configPath, "config", "./heimdal.yaml", "Config file path")
	return cmd
}

func autoKnowledgeBasesCmd() *cobra.Command {
	var configPath, projectID string
	cmd := &cobra.Command{
		Use:   "knowledge-bases",
		Short: "List project knowledge bases",
		Long: `List knowledge bases available for a project.

Use this command to find knowledge base IDs for tests that need KB context
(for example TG-01 / TG-02, and DL-01 when dynamic probes are enabled).`,
		Example: strings.TrimSpace(`
  heimdal knowledge-bases --project <project-id>
  heimdal use <project-id>
  heimdal knowledge-bases
`),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 1 && (args[0] == "help" || args[0] == "?") {
				return cmd.Help()
			}
			if len(args) > 0 {
				return fmt.Errorf("unexpected arguments: %s (use `heimdal knowledge-bases --help`)", strings.Join(args, " "))
			}
			if !hasActiveOrgContext() {
				return fmt.Errorf("no active organization selected; run `heimdal org list` and `heimdal org use <org-id>` first")
			}
			cfg, client, err := configAndClient(configPath)
			if err != nil {
				return err
			}
			projectID = firstNonEmptyLocal(projectID, activeProjectContextID(), cfg.Project.ID)
			projectID, err = resolveProjectIDPrefixWithClient(client, projectID)
			if err != nil {
				return err
			}
			if projectID == "" {
				return fmt.Errorf("project id is required; use `heimdal use <project-id>` or pass `--project`")
			}

			resp, err := client.ListKnowledgeBases(context.Background(), projectID)
			if err != nil {
				return err
			}
			for _, kb := range resp.KnowledgeBases {
				status := strings.TrimSpace(kb.Status)
				if status == "" {
					status = "unknown"
				}
				def := ""
				if kb.IsDefault {
					def = " default"
				}
				fmt.Printf("%s  %s  status=%s items=%d%s\n", kb.ID, kb.Name, status, kb.ItemCount, def)
			}
			fmt.Printf("total=%d\n", resp.Total)
			return nil
		},
	}
	cmd.Flags().StringVar(&configPath, "config", "./heimdal.yaml", "Config file path")
	cmd.Flags().StringVar(&projectID, "project", "", "Project ID")
	cmd.AddCommand(knowledgeBasesUploadCmd())
	cmd.AddCommand(knowledgeBasesStatusCmd())
	return cmd
}

func knowledgeBasesUploadCmd() *cobra.Command {
	var configPath, projectID, name, description, sourceLanguage, targetLanguage string
	var watch bool
	cmd := &cobra.Command{
		Use:   "upload <file> [file...]",
		Short: "Upload local files as a project knowledge base",
		Long: `Upload one or more local files as a project knowledge base.

Supported files: .xlsx, .csv, .pdf, .docx, .txt.
Processing continues asynchronously after upload; use --watch or
` + "`heimdal knowledge-bases status <kb-id>`" + ` to track progress.`,
		Example: strings.TrimSpace(`
  heimdal knowledge-bases upload ./faq.xlsx --name "Support KB"
  heimdal knowledge-bases upload ./faq.xlsx ./policy.pdf --name "Retail Banking KB" --project <project-id> --watch
`),
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return fmt.Errorf("at least one file is required")
			}
			for _, filePath := range args {
				if _, err := os.Stat(filePath); err != nil {
					return fmt.Errorf("file not found: %s", filePath)
				}
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if !hasActiveOrgContext() {
				return fmt.Errorf("no active organization selected; run `heimdal org list` and `heimdal org use <org-id>` first")
			}
			cfg, client, err := configAndClient(configPath)
			if err != nil {
				return err
			}
			projectID = firstNonEmptyLocal(projectID, activeProjectContextID(), cfg.Project.ID)
			projectID, err = resolveProjectIDPrefixWithClient(client, projectID)
			if err != nil {
				return err
			}
			if projectID == "" {
				return fmt.Errorf("project id is required; use `heimdal use <project-id>` or pass `--project`")
			}
			name = strings.TrimSpace(name)
			if name == "" {
				return fmt.Errorf("knowledge base name is required; pass `--name`")
			}

			resp, err := client.UploadKnowledgeBase(context.Background(), runner.KnowledgeBaseUploadRequest{
				FilePaths:      args,
				Name:           name,
				Description:    strings.TrimSpace(description),
				ProjectID:      projectID,
				SourceLanguage: strings.TrimSpace(sourceLanguage),
				TargetLanguage: strings.TrimSpace(targetLanguage),
			})
			if err != nil {
				return err
			}

			fmt.Println("Knowledge base upload started")
			fmt.Printf("%s  %s  status=%s items=%d\n", resp.ID, resp.Name, fallback(resp.Status, "pending"), resp.ItemCount)
			fmt.Println("Track progress:")
			fmt.Printf("  heimdal knowledge-bases status %s --watch\n", resp.ID)
			if watch {
				fmt.Println()
				return watchKnowledgeBaseJob(client, resp.ID)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&configPath, "config", "./heimdal.yaml", "Config file path")
	cmd.Flags().StringVar(&projectID, "project", "", "Project ID")
	cmd.Flags().StringVar(&name, "name", "", "Knowledge base name")
	cmd.Flags().StringVar(&description, "description", "", "Knowledge base description")
	cmd.Flags().StringVar(&sourceLanguage, "source-language", "tr", "Source language")
	cmd.Flags().StringVar(&targetLanguage, "target-language", "en", "Target language")
	cmd.Flags().BoolVar(&watch, "watch", false, "Watch processing progress until completion")
	return cmd
}

func knowledgeBasesStatusCmd() *cobra.Command {
	var configPath, projectID string
	var watch bool
	cmd := &cobra.Command{
		Use:   "status <knowledge-base-id>",
		Short: "Show knowledge base processing status",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !hasActiveOrgContext() {
				return fmt.Errorf("no active organization selected; run `heimdal org list` and `heimdal org use <org-id>` first")
			}
			cfg, client, err := configAndClient(configPath)
			if err != nil {
				return err
			}
			projectID = firstNonEmptyLocal(projectID, activeProjectContextID(), cfg.Project.ID)
			projectID, err = resolveProjectIDPrefixWithClient(client, projectID)
			if err != nil {
				return err
			}
			kbID, err := resolveKnowledgeBaseIDPrefixWithClient(client, projectID, args[0])
			if err != nil {
				return err
			}
			if watch {
				return watchKnowledgeBaseJob(client, kbID)
			}
			detail, err := client.GetJobByEntity(context.Background(), "knowledge_base", kbID)
			if err != nil {
				return err
			}
			printJobDetail(detail)
			return nil
		},
	}
	cmd.Flags().StringVar(&configPath, "config", "./heimdal.yaml", "Config file path")
	cmd.Flags().StringVar(&projectID, "project", "", "Project ID")
	cmd.Flags().BoolVar(&watch, "watch", false, "Watch processing progress until completion")
	return cmd
}

func clientFromConfig(path string) (*runner.HeimdalClient, error) {
	_, client, err := configAndClient(path)
	return client, err
}

func configAndClient(path string) (*config.Config, *runner.HeimdalClient, error) {
	cfg := &config.Config{}
	if _, err := os.Stat(path); err == nil {
		loaded, err := config.Load(path)
		if err != nil {
			return nil, nil, err
		}
		cfg = loaded
	}
	client, err := runner.NewClientFromToken(cfg)
	if err != nil {
		return nil, nil, err
	}
	return cfg, client, nil
}

func firstNonEmptyLocal(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func resolveProjectIDPrefixWithClient(client *runner.HeimdalClient, input string) (string, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return "", nil
	}
	activeOrgID := activeOrgContextID()
	resp, err := client.ListProjects(context.Background())
	if err != nil {
		return "", err
	}
	matches := make([]string, 0, 2)
	for _, p := range resp.Projects {
		if activeOrgID != "" && !strings.EqualFold(strings.TrimSpace(p.OrganizationID), activeOrgID) {
			continue
		}
		id := strings.TrimSpace(p.ID)
		if id == input || strings.HasPrefix(id, input) {
			matches = append(matches, id)
		}
	}
	if len(matches) == 0 {
		return "", fmt.Errorf("project not found: %s", input)
	}
	if len(matches) > 1 {
		return "", fmt.Errorf("ambiguous project id prefix: %s", input)
	}
	return matches[0], nil
}

func resolveIntegrationIDPrefixWithClient(client *runner.HeimdalClient, projectID, input string) (string, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return "", fmt.Errorf("integration id cannot be empty")
	}
	resp, err := client.ListIntegrations(context.Background(), projectID)
	if err != nil {
		return "", err
	}
	matches := make([]string, 0, 2)
	for _, i := range resp.Integrations {
		id := strings.TrimSpace(i.ID)
		if id == input || strings.HasPrefix(id, input) {
			matches = append(matches, id)
		}
	}
	if len(matches) == 0 {
		return "", fmt.Errorf("integration not found: %s", input)
	}
	if len(matches) > 1 {
		return "", fmt.Errorf("ambiguous integration id prefix: %s", input)
	}
	return matches[0], nil
}

func resolveKnowledgeBaseIDPrefixWithClient(client *runner.HeimdalClient, projectID, input string) (string, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return "", fmt.Errorf("knowledge base id cannot be empty")
	}
	if len(input) == 36 {
		return input, nil
	}
	projectID = strings.TrimSpace(projectID)
	if projectID == "" {
		return "", fmt.Errorf("project id is required to resolve a knowledge base prefix; use `heimdal use <project-id>` or pass `--project`")
	}
	resp, err := client.ListKnowledgeBases(context.Background(), projectID)
	if err != nil {
		return "", err
	}
	matches := make([]string, 0, 2)
	for _, kb := range resp.KnowledgeBases {
		id := strings.TrimSpace(kb.ID)
		if id == input || strings.HasPrefix(id, input) {
			matches = append(matches, id)
		}
	}
	if len(matches) == 0 {
		return "", fmt.Errorf("knowledge base not found: %s", input)
	}
	if len(matches) > 1 {
		return "", fmt.Errorf("ambiguous knowledge base id prefix: %s", input)
	}
	return matches[0], nil
}

func watchKnowledgeBaseJob(client *runner.HeimdalClient, kbID string) error {
	for {
		detail, err := client.GetJobByEntity(context.Background(), "knowledge_base", kbID)
		if err != nil {
			return err
		}
		printJobDetail(detail)
		status := strings.ToLower(strings.TrimSpace(detail.Job.Status))
		if status == "completed" || status == "failed" || status == "cancelled" {
			return nil
		}
		time.Sleep(3 * time.Second)
	}
}

func printJobDetail(detail *runner.JobDetailResponse) {
	job := detail.Job
	fmt.Printf("Job:      %s\n", shortID(job.ID))
	fmt.Printf("KB:       %s\n", shortID(job.EntityID))
	fmt.Printf("Status:   %s\n", fallback(job.Status, "unknown"))
	fmt.Printf("Progress: %.0f%%", job.Progress)
	if job.TotalItems > 0 {
		fmt.Printf("  items=%d/%d", job.ProcessedItems, job.TotalItems)
	}
	fmt.Println()
	if strings.TrimSpace(job.CurrentStep) != "" {
		fmt.Printf("Current:  %s\n", job.CurrentStep)
	}
	if strings.TrimSpace(job.ErrorMessage) != "" {
		fmt.Printf("Error:    %s\n", job.ErrorMessage)
	}
	if len(detail.Steps) > 0 {
		fmt.Println("Steps:")
		start := 0
		if len(detail.Steps) > 5 {
			start = len(detail.Steps) - 5
		}
		for _, step := range detail.Steps[start:] {
			label := strings.TrimSpace(step.StepName)
			if label == "" {
				label = "step"
			}
			fmt.Printf("  %s  %s\n", fallback(step.Status, "unknown"), label)
		}
	}
	fmt.Println()
}

func shortID(id string) string {
	id = strings.TrimSpace(id)
	if len(id) <= 15 {
		return id
	}
	return id[:7] + "..." + id[len(id)-6:]
}

func fallback(value, fallbackValue string) string {
	if strings.TrimSpace(value) == "" {
		return fallbackValue
	}
	return strings.TrimSpace(value)
}

func buildIntegrationConfigCreateRequest(intCfg config.IntegrationConfig, projectID string) (runner.IntegrationConfigCreateRequest, error) {
	method := strings.ToUpper(strings.TrimSpace(intCfg.Method))
	if method == "" {
		method = "POST"
	}
	urlValue := strings.TrimSpace(intCfg.Endpoint)
	if urlValue == "" {
		urlValue = strings.TrimSpace(intCfg.BaseURL)
	}
	if urlValue == "" {
		return runner.IntegrationConfigCreateRequest{}, fmt.Errorf("integration.endpoint is required")
	}

	templateObj := intCfg.RequestTemplate
	if len(templateObj) == 0 {
		templateObj = map[string]any{
			"query": "{{QUESTION}}",
		}
	}
	bodyTemplateBytes, err := json.Marshal(templateObj)
	if err != nil {
		return runner.IntegrationConfigCreateRequest{}, fmt.Errorf("failed to marshal integration.request_template: %w", err)
	}

	authKeys := make([]string, 0, len(intCfg.Headers))
	for k := range intCfg.Headers {
		if strings.TrimSpace(k) != "" {
			authKeys = append(authKeys, k)
		}
	}
	sort.Strings(authKeys)
	authConfig := make([]runner.IntegrationAuthConfigItem, 0, len(authKeys))
	for _, k := range authKeys {
		authConfig = append(authConfig, runner.IntegrationAuthConfigItem{
			Type:  "header",
			Key:   k,
			Value: intCfg.Headers[k],
		})
	}

	timeout := intCfg.TimeoutSeconds
	if timeout <= 0 {
		timeout = 45
	}

	return runner.IntegrationConfigCreateRequest{
		Method:              method,
		URL:                 urlValue,
		AuthConfig:          authConfig,
		RequestBodyTemplate: string(bodyTemplateBytes),
		ResponsePath:        strings.TrimSpace(intCfg.Response.AnswerPath),
		Description:         "Applied from heimdal.yaml",
		ProjectID:           strings.TrimSpace(projectID),
		TimeoutSeconds:      timeout,
	}, nil
}

func mergePulledIntegration(doc map[string]any, activeCfg *runner.IntegrationConfigResponse) {
	if doc == nil || activeCfg == nil {
		return
	}
	setPath(doc, []string{"integration", "type"}, "custom_http")
	setPath(doc, []string{"integration", "endpoint"}, strings.TrimSpace(activeCfg.URL))
	setPath(doc, []string{"integration", "method"}, strings.ToUpper(strings.TrimSpace(activeCfg.Method)))
	setPath(doc, []string{"integration", "timeout_seconds"}, activeCfg.TimeoutSeconds)

	headers := map[string]any{
		"Content-Type": "application/json",
	}
	for _, a := range activeCfg.AuthConfig {
		if !strings.EqualFold(strings.TrimSpace(a.Type), "header") {
			continue
		}
		key := strings.TrimSpace(a.Key)
		if key == "" {
			continue
		}
		switch {
		case strings.TrimSpace(a.Value) != "":
			headers[key] = a.Value
		case strings.TrimSpace(a.SecretRef) != "":
			headers[key] = fmt.Sprintf("${SECRET_REF:%s}", strings.TrimSpace(a.SecretRef))
		}
	}
	setPath(doc, []string{"integration", "headers"}, headers)

	var templateBody any
	templateRaw := strings.TrimSpace(activeCfg.RequestBodyTemplate)
	if templateRaw != "" && json.Unmarshal([]byte(templateRaw), &templateBody) == nil {
		setPath(doc, []string{"integration", "request_template"}, templateBody)
	} else if templateRaw != "" {
		setPath(doc, []string{"integration", "request_template"}, map[string]any{"raw_template": templateRaw})
	}

	setPath(doc, []string{"integration", "response", "answer_path"}, strings.TrimSpace(activeCfg.ResponsePath))
	// Keep explicit keys so users can extend mappings manually.
	if _, ok := getPath(doc, []string{"integration", "response", "tool_traces_path"}); !ok {
		setPath(doc, []string{"integration", "response", "tool_traces_path"}, "")
	}
	if _, ok := getPath(doc, []string{"integration", "response", "retrieved_chunks_path"}); !ok {
		setPath(doc, []string{"integration", "response", "retrieved_chunks_path"}, "")
	}
}

func loadOrCreateConfigDocument(path string) (map[string]any, error) {
	doc := map[string]any{}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			doc["version"] = 1
			return doc, nil
		}
		return nil, err
	}
	if len(strings.TrimSpace(string(data))) == 0 {
		doc["version"] = 1
		return doc, nil
	}
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("yaml parse error: %w", err)
	}
	if _, ok := doc["version"]; !ok {
		doc["version"] = 1
	}
	return doc, nil
}

func saveConfigDocument(path string, doc map[string]any) error {
	b, err := yaml.Marshal(doc)
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o644)
}

func setPath(doc map[string]any, path []string, value any) {
	if len(path) == 0 {
		return
	}
	curr := doc
	for i := 0; i < len(path)-1; i++ {
		key := path[i]
		next, ok := curr[key]
		if !ok {
			child := map[string]any{}
			curr[key] = child
			curr = child
			continue
		}
		child, ok := next.(map[string]any)
		if !ok {
			child = map[string]any{}
			curr[key] = child
		}
		curr = child
	}
	curr[path[len(path)-1]] = value
}

func getPath(doc map[string]any, path []string) (any, bool) {
	if len(path) == 0 {
		return nil, false
	}
	var current any = doc
	for _, p := range path {
		m, ok := current.(map[string]any)
		if !ok {
			return nil, false
		}
		next, ok := m[p]
		if !ok {
			return nil, false
		}
		current = next
	}
	return current, true
}

func resolveRunIDPrefixWithClient(client *runner.HeimdalClient, projectID, input string) (string, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return "", fmt.Errorf("run id cannot be empty")
	}
	resp, err := client.ListAutoRunRuns(context.Background(), projectID, "", "", 200, 0)
	if err != nil {
		return "", err
	}
	matches := make([]string, 0, 3)
	for _, r := range resp.Runs {
		id := strings.TrimSpace(r.ID)
		if id == input || strings.HasPrefix(id, input) {
			matches = append(matches, id)
		}
	}
	if len(matches) == 0 {
		return "", fmt.Errorf("run not found for prefix: %s", input)
	}
	if len(matches) > 1 {
		return "", fmt.Errorf("ambiguous run id prefix: %s (matches: %s)", input, strings.Join(matches[:minInt(3, len(matches))], ", "))
	}
	return matches[0], nil
}

func looksLikeFullUUID(v string) bool {
	v = strings.TrimSpace(v)
	return len(v) == 36 && strings.Count(v, "-") == 4
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func printAutoRunTestsCatalog(client *runner.HeimdalClient, selectedTestID string) error {
	resp, err := client.GetAutoRunTests(context.Background())
	if err != nil {
		return err
	}
	selectedTestID = strings.TrimSpace(selectedTestID)
	if selectedTestID == "" {
		for _, t := range resp.Tests {
			fmt.Printf("%-6s %-34s scenarios=%s", t.ID, t.Name, strings.Join(t.Scenarios, ","))
			if t.RequiresKB {
				fmt.Print(" kb")
			}
			if t.RequiresSystemPrompt {
				fmt.Print(" system_prompt")
			}
			fmt.Println()
		}
		return nil
	}

	for _, t := range resp.Tests {
		if !strings.EqualFold(strings.TrimSpace(t.ID), selectedTestID) {
			continue
		}
		fmt.Printf("test_id=%s\n", t.ID)
		fmt.Printf("name=%s\n", t.Name)
		fmt.Printf("scenarios=%s\n", strings.Join(t.Scenarios, ","))
		fmt.Printf("description=%s\n", strings.TrimSpace(t.DescriptionShort))
		if len(t.Tags) > 0 {
			fmt.Printf("tags=%s\n", strings.Join(t.Tags, ","))
		}
		fmt.Printf("question_source=%s\n", t.QuestionSource)
		fmt.Printf("requires_model=%t requires_kb=%t requires_system_prompt=%t\n", t.RequiresModel, t.RequiresKB, t.RequiresSystemPrompt)
		if threshold, ok := resp.VerdictThresholds[t.ID]; ok {
			fmt.Printf("verdict_threshold=%.1f\n", threshold)
		}
		if failThreshold, ok := resp.FailThresholds[t.ID]; ok {
			fmt.Printf("fail_threshold=%.1f\n", failThreshold)
		}
		return nil
	}
	return fmt.Errorf("test not found: %s", selectedTestID)
}
