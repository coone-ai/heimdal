package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/ai-la/cli/internal/config"
	"github.com/ai-la/cli/internal/output"
	"github.com/ai-la/cli/internal/runner"
	"github.com/spf13/cobra"
)

// ── Command tree: heimdal test ──────────────────────────────────────────────────

// testCmd is the parent `heimdal test` command.
// Sub-commands hang off this (e.g. `heimdal test auto`).
var testCmd = &cobra.Command{
	Use:     "test",
	Short:   "Run test commands",
	GroupID: "run",
}

func init() {
	rootCmd.AddCommand(testCmd)
	testCmd.AddCommand(buildAutoCmd())
}

// ── heimdal test auto ───────────────────────────────────────────────────────────

func buildAutoCmd() *cobra.Command {
	var (
		configPath             string
		projectID              string
		version                string
		suites                 []string
		count                  int
		testsetID              string
		usePrevious            bool
		generateNew            bool
		freeze                 bool
		noFreeze               bool
		failUnder              float64
		noStore                bool
		redact                 bool
		verbose                bool
		jsonOutput             bool
		testID                 string
		scenario               string
		integrationID          string
		knowledgeBaseID        string
		systemPrompt           string
		qaFile                 string
		questionsFile          string
		questions              []string
		language               string
		appContext             string
		attackFocus            string
		aggressiveness         string
		dynamicProbeGeneration bool
	)

	cmd := &cobra.Command{
		Use:   "auto",
		Short: "Run AILab Auto Run tests",
		Long: `Runs AILab Auto Run tests, waits for completion, and prints
terminal summary plus optional JSON output.`,
		Example: strings.TrimSpace(`
  heimdal test auto --test-id AT-01 --scenario A --integration <integration-id>
  heimdal test auto --test-id CS-01 --scenario A --integration <integration-id>
  heimdal test auto --test-id TG-01 --scenario A --integration <integration-id> --knowledge-base <kb-id>
  heimdal test auto --test-id TG-02 --scenario A --integration <integration-id> --knowledge-base <kb-id>
  heimdal test auto --test-id DL-01 --scenario A --integration <integration-id> --knowledge-base <kb-id> --dynamic-probes
  heimdal test auto --test-id IM-01 --scenario A --integration <integration-id> --system-prompt "You are a banking assistant"

  heimdal knowledge-bases --project <project-id>
`),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 1 && (args[0] == "help" || args[0] == "?") {
				return cmd.Help()
			}
			if len(args) > 0 {
				return fmt.Errorf("unexpected arguments: %s (use `heimdal test auto --help`)", strings.Join(args, " "))
			}
			// ── Load config ───────────────────────────────────────────────────
			configMissing := false
			cfg, err := config.Load(configPath)
			if err != nil {
				if os.IsNotExist(err) {
					cfg = &config.Config{Version: 1}
					configMissing = true
				} else {
					exitWithError(err, 2, jsonOutput)
					return nil
				}
			}

			// project override
			if projectID != "" {
				cfg.Project.ID = projectID
			}
			if cfg.Project.ID == "" {
				cfg.Project.ID = activeProjectContextID()
			}
			if cfg.Project.ID != "" {
				if resolvedProjectID, resolveErr := resolveProjectIDPrefix(cfg.Project.ID); resolveErr == nil {
					cfg.Project.ID = resolvedProjectID
				}
			}
			if testID != "" {
				cfg.AutoRun.TestID = testID
			}
			if scenario != "" {
				cfg.AutoRun.Scenario = scenario
			}
			if integrationID != "" {
				cfg.AutoRun.IntegrationID = integrationID
			}
			if knowledgeBaseID != "" {
				cfg.AutoRun.KnowledgeBaseID = knowledgeBaseID
			}
			if systemPrompt != "" {
				cfg.AutoRun.SystemPrompt = systemPrompt
			}
			if qaFile != "" {
				cfg.AutoRun.QAItemsFile = qaFile
			}
			if questionsFile != "" {
				cfg.AutoRun.QuestionsFile = questionsFile
			}

			// ── Validate ──────────────────────────────────────────────────────
			if !configMissing {
				result := config.Validate(cfg, configPath)
				if verbose {
					printValidation(result)
				}
				if !result.Valid {
					printValidation(result) // always show on failure
					exitWithError(errConfigInvalid, 2, jsonOutput)
					return nil
				}
			}

			// ── Build overrides ───────────────────────────────────────────────
			overrides := runner.RunOverrides{
				Version:                version,
				Suites:                 suites,
				QueryCount:             count,
				TestsetID:              testsetID,
				UsePrevious:            usePrevious,
				GenerateNew:            generateNew,
				FailUnder:              failUnder,
				NoStore:                noStore,
				Redact:                 redact,
				ProjectID:              projectID,
				TestID:                 testID,
				Scenario:               scenario,
				IntegrationID:          integrationID,
				DatasetSnapshotID:      testsetID,
				KnowledgeBaseID:        knowledgeBaseID,
				SystemPrompt:           systemPrompt,
				QAItemsFile:            qaFile,
				QuestionsFile:          questionsFile,
				Questions:              questions,
				Language:               language,
				AppContext:             appContext,
				AttackFocus:            attackFocus,
				Aggressiveness:         aggressiveness,
				DynamicProbeGeneration: dynamicProbeGeneration,
			}

			// --freeze and --no-freeze are mutually exclusive
			if freeze && noFreeze {
				exitWithError(errFreezeConflict, 2, jsonOutput)
				return nil
			}
			if freeze {
				t := true
				overrides.Freeze = &t
			} else if noFreeze {
				f := false
				overrides.Freeze = &f
			}

			// ── Runner setup ──────────────────────────────────────────────────
			r, err := runner.New(cfg)
			if err != nil {
				exitWithError(err, 2, jsonOutput)
				return nil
			}

			// ── Output / hooks ────────────────────────────────────────────────
			var hooks runner.Hooks
			if jsonOutput {
				hooks = runner.NoopHooks{}
			} else {
				hooks = output.NewTerminalHooks()
			}

			// ── Context with signal handling ──────────────────────────────────
			ctx, stop := signal.NotifyContext(context.Background(),
				os.Interrupt, syscall.SIGTERM)
			defer stop()

			// ── Print header ──────────────────────────────────────────────────
			if !jsonOutput {
				// We don't have run_id / testset_id yet — print partial header.
				// Full header with IDs is printed after CreateAutoTestRun returns.
				// For now, just print the project line so the terminal isn't blank.
				output.PrintHeader(
					cfg.Project.Name,
					version,
					"…", // run_id known after step 1
					"…", // testset_id known after step 1
					resolveSuites(cfg, overrides),
					resolveQueryCount(cfg, overrides),
				)
			}

			// ── Run ───────────────────────────────────────────────────────────
			report, err := r.RunAutoTests(ctx, overrides, hooks)

			// ── Output ────────────────────────────────────────────────────────
			if jsonOutput {
				if err != nil {
					exitCode := 2
					if report != nil && report.ExitCode != 0 {
						exitCode = report.ExitCode
					}
					output.PrintJSONError(err, exitCode)
					os.Exit(exitCode)
					return nil
				}
				if printErr := output.NewJSONOutput().Print(report); printErr != nil {
					os.Exit(4)
				}
				os.Exit(report.ExitCode)
				return nil
			}

			// Terminal path
			if err != nil {
				exitCode := 2
				if report != nil && report.ExitCode != 0 {
					exitCode = report.ExitCode
				}
				output.PrintError(err, exitCode)
				os.Exit(exitCode)
				return nil
			}

			output.PrintReport(report)
			os.Exit(report.ExitCode)
			return nil
		},
	}

	// ── Flag definitions ──────────────────────────────────────────────────────
	f := cmd.Flags()

	f.StringVar(&configPath, "config", "./heimdal.yaml", "Config file path")
	f.StringVar(&projectID, "project", "", "Override project.id")
	f.StringVar(&version, "version", "", "Version tag for this run")
	f.StringArrayVar(&suites, "suite", nil, "Repeatable test suite filter")
	f.IntVar(&count, "count", 0, "Number of generated queries")
	f.StringVar(&testsetID, "testset", "", "Use a specific frozen benchmark dataset snapshot ID")
	f.BoolVar(&usePrevious, "use-previous", false, "Use the latest frozen testset for the project")
	f.BoolVar(&generateNew, "generate-new", false, "Generate a new testset")
	f.BoolVar(&freeze, "freeze", false, "Store the generated testset")
	f.BoolVar(&noFreeze, "no-freeze", false, "Do not store the generated testset")
	f.Float64Var(&failUnder, "fail-under", 0, "Exit 1 if reliability score is below this value")
	f.BoolVar(&noStore, "no-store", false, "Do not store raw cases")
	f.BoolVar(&redact, "redact", false, "Redact PII before storage")
	f.BoolVar(&verbose, "verbose", false, "Show detailed logs")
	f.BoolVar(&jsonOutput, "json", false, "Machine-readable JSON output")
	f.StringVar(&testID, "test-id", "", "Auto Run Test ID (AT-01, CS-01, IM-01, IM-02, RC-01, DL-01, TG-01, TG-02)")
	f.StringVar(&scenario, "scenario", "", "Scenario (A, B, C)")
	f.StringVar(&integrationID, "integration", "", "Integration ID for scenario A/C")
	f.StringVar(&knowledgeBaseID, "knowledge-base", "", "Knowledge base ID for TG-01/TG-02/DL-01")
	f.StringVar(&systemPrompt, "system-prompt", "", "System prompt for IM-01/DL-01")
	f.StringVar(&qaFile, "qa-file", "", "JSON or CSV Q&A file for scenario B")
	f.StringVar(&questionsFile, "questions-file", "", "Questions file (.json array or line-based .txt)")
	f.StringArrayVar(&questions, "question", nil, "Repeatable inline question")
	f.StringVar(&language, "language", "", "Generation language for scenario C")
	f.StringVar(&appContext, "app-context", "", "Application/product context for scenario C")
	f.StringVar(&attackFocus, "attack-focus", "", "Focus mode for IM-01 generator")
	f.StringVar(&aggressiveness, "aggressiveness", "", "Aggressiveness for IM-01 generator")
	f.BoolVar(&dynamicProbeGeneration, "dynamic-probes", false, "Enable KB/system-prompt aware dynamic probes for DL-01")

	_ = f.MarkHidden("count")
	_ = f.MarkHidden("suite")
	_ = f.MarkHidden("freeze")
	_ = f.MarkHidden("no-freeze")
	_ = f.MarkHidden("no-store")
	_ = f.MarkHidden("redact")
	_ = f.MarkHidden("use-previous")

	// --use-previous ve --testset aynı anda kullanılamaz
	cmd.MarkFlagsMutuallyExclusive("use-previous", "testset")
	// --generate-new ve --testset aynı anda kullanılamaz
	cmd.MarkFlagsMutuallyExclusive("generate-new", "testset")

	return cmd
}

// ── Sentinel errors ───────────────────────────────────────────────────────────

var (
	errConfigInvalid  = fmt.Errorf("heimdal.yaml is invalid — fix the errors above")
	errFreezeConflict = fmt.Errorf("--freeze and --no-freeze cannot be used together")
)

// ── Helpers ───────────────────────────────────────────────────────────────────

func exitWithError(err error, code int, asJSON bool) {
	if asJSON {
		output.PrintJSONError(err, code)
	} else {
		output.PrintError(err, code)
	}
	os.Exit(code)
}

func printValidation(result config.ValidationResult) {
	for _, c := range result.Checks {
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
}

// resolveSuites / resolveQueryCount mirror runner logic so the header can be
// printed before RunAutoTests is called.
func resolveSuites(cfg *config.Config, ov runner.RunOverrides) []string {
	if len(ov.Suites) > 0 {
		return ov.Suites
	}
	return cfg.Tests.Suites
}

func resolveQueryCount(cfg *config.Config, ov runner.RunOverrides) int {
	if ov.QueryCount > 0 {
		return ov.QueryCount
	}
	return cfg.Simulation.QueryCount
}
