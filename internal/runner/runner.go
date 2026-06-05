package runner

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ai-la/cli/internal/auth"
	"github.com/ai-la/cli/internal/config"
)

// ── Poll settings ─────────────────────────────────────────────────────────────

const (
	pollInterval = 3 * time.Second
	pollTimeout  = 10 * time.Minute
)

// ── Runner ────────────────────────────────────────────────────────────────────

type Runner struct {
	cfg    *config.Config
	client *HeimdalClient
}

func New(cfg *config.Config) (*Runner, error) {
	token := os.Getenv("HEIMDAL_API_KEY")
	var stored *auth.TokenStore
	if token == "" {
		var err error
		stored, err = auth.LoadToken()
		if err != nil {
			return nil, fmt.Errorf("login not found: run `heimdal login` first or set HEIMDAL_API_KEY (%w)", err)
		}
		token = stored.Token
	}
	token = normalizeAPIKey(token)

	storedAPIURL := ""
	storedOrgID := ""
	if stored != nil {
		storedAPIURL = stored.APIBaseURL
		storedOrgID = stored.ActiveOrgID
	}
	baseURL := resolveAPIBaseURL(cfg.API.BaseURL, storedAPIURL)
	orgID := firstNonEmpty(os.Getenv("HEIMDAL_ORG_ID"), storedOrgID)
	return &Runner{
		cfg:    cfg,
		client: NewHeimdalClient(token).WithBaseURL(baseURL).WithOrganizationID(orgID),
	}, nil
}

func NewClientFromToken(cfg *config.Config) (*HeimdalClient, error) {
	if cfg == nil {
		cfg = &config.Config{}
	}
	token := os.Getenv("HEIMDAL_API_KEY")
	var stored *auth.TokenStore
	if token == "" {
		var err error
		stored, err = auth.LoadToken()
		if err != nil {
			return nil, fmt.Errorf("login not found: run `heimdal login` first or set HEIMDAL_API_KEY (%w)", err)
		}
		token = stored.Token
	}
	token = normalizeAPIKey(token)
	storedAPIURL := ""
	storedOrgID := ""
	if stored != nil {
		storedAPIURL = stored.APIBaseURL
		storedOrgID = stored.ActiveOrgID
	}
	baseURL := resolveAPIBaseURL(cfg.API.BaseURL, storedAPIURL)
	orgID := firstNonEmpty(os.Getenv("HEIMDAL_ORG_ID"), storedOrgID)
	return NewHeimdalClient(token).WithBaseURL(baseURL).WithOrganizationID(orgID), nil
}

func resolveAPIBaseURL(configBaseURL, storedAPIURL string) string {
	normalizeLegacy := func(v string) string {
		v = strings.TrimSpace(v)
		lower := strings.ToLower(v)
		if strings.Contains(lower, "api.heimdal.dev") || strings.TrimRight(lower, "/") == "https://ailab.co-one.co" {
			return defaultBaseURL
		}
		return v
	}
	configBaseURL = normalizeLegacy(configBaseURL)
	storedAPIURL = normalizeLegacy(storedAPIURL)
	if isDevMode() {
		return firstNonEmpty(os.Getenv("HEIMDAL_API_URL"), "http://localhost:5002", configBaseURL)
	}
	return firstNonEmpty(nonLocalAPIURL(configBaseURL), nonLocalAPIURL(os.Getenv("HEIMDAL_API_URL")), nonLocalAPIURL(storedAPIURL), defaultBaseURL)
}

func isDevMode() bool {
	return strings.EqualFold(os.Getenv("HEIMDAL_ENV"), "dev") || os.Getenv("HEIMDAL_DEV") == "1"
}

func activeProjectID() string {
	ts, err := auth.LoadToken()
	if err != nil || ts == nil {
		return ""
	}
	return strings.TrimSpace(ts.ActiveProjectID)
}

func nonLocalAPIURL(v string) string {
	v = strings.TrimSpace(v)
	lower := strings.ToLower(v)
	if strings.Contains(lower, "localhost") || strings.Contains(lower, "127.0.0.1") || strings.Contains(lower, "::1") {
		return ""
	}
	return v
}

func normalizeAPIKey(v string) string {
	v = strings.TrimSpace(v)
	v = strings.Trim(v, `"'`)
	if strings.HasPrefix(strings.ToLower(v), "bearer ") {
		v = strings.TrimSpace(v[7:])
	}
	return strings.TrimSpace(v)
}

// ── Main entry point ──────────────────────────────────────────────────────────

// RunAutoTests orchestrates the full auto-running-test flow.
// It calls the provided hooks so the output package can render progress
// without the runner knowing anything about terminal formatting.
func (r *Runner) RunAutoTests(ctx context.Context, overrides RunOverrides, hooks Hooks) (*RunReport, error) {
	if hooks == nil {
		hooks = NoopHooks{}
	}

	cfg := r.cfg
	projectID := firstNonEmpty(overrides.ProjectID, activeProjectID(), cfg.Project.ID)
	cfg.Project.ID = projectID
	report := &RunReport{
		ProjectID: projectID,
		Version:   resolveVersion(cfg, overrides),
	}

	hooks.OnStepStart("Starting Auto Run test")
	req, err := buildAutoRunRequest(cfg, overrides)
	if err != nil {
		report.ExitCode = 2
		return report, err
	}
	startResp, err := r.client.RunAutoRunTest(ctx, projectID, req)
	if err != nil {
		report.ExitCode = 4
		return report, fmt.Errorf("failed to start auto run test: %w", err)
	}
	report.JobID = startResp.JobID
	report.RunID = startResp.TestRunID
	report.TestsetID = startResp.DatasetSnapshotID
	report.Status = startResp.Status
	hooks.OnStepDone(fmt.Sprintf("%s / job %s", report.RunID, report.JobID))

	hooks.OnStepStart("Waiting for job result")
	resultsResp, err := r.pollAutoRunUntilComplete(ctx, report.RunID, func(progress float64) {
		hooks.OnProgress(int(progress), 100)
	})
	if err != nil {
		report.ExitCode = 4
		return report, fmt.Errorf("auto run test did not complete successfully: %w", err)
	}
	if resultsResp.TestRun == nil {
		report.ExitCode = 5
		return report, fmt.Errorf("test run did not return a result: %s", resultsResp.ErrorMessage)
	}
	if resultsResp.TestRun.Status == "failed" {
		report.ExitCode = 5
		return report, fmt.Errorf("auto run test failed: %s", resultsResp.TestRun.ErrorMessage)
	}
	hooks.OnStepDone("Evaluation completed")

	populateReportFromAutoRun(report, resultsResp.TestRun, overrides)

	// ── Step 6: Write local artifact ─────────────────────────────────────────
	if err := writeLocalArtifact(report); err != nil {
		// Non-fatal: log but don't fail the run.
		hooks.OnWarn(fmt.Sprintf("Failed to write local artifact: %v", err))
	}

	return report, nil
}

// RunTests loads heimdal.yaml, creates a runner, and executes the auto flow.
func RunTests() error {
	cfg, err := config.Load("heimdal.yaml")
	if err != nil {
		return err
	}

	r, err := New(cfg)
	if err != nil {
		return err
	}

	report, err := r.RunAutoTests(context.Background(), RunOverrides{}, NoopHooks{})
	if err != nil {
		return err
	}

	if data, err := json.MarshalIndent(report, "", "  "); err == nil {
		_ = os.WriteFile(".heimdal/last-run.json", data, 0o644)
	}

	if report.ExitCode != 0 {
		return fmt.Errorf("run finished with exit code %d", report.ExitCode)
	}

	return nil
}

// ── Polling ───────────────────────────────────────────────────────────────────

func (r *Runner) pollUntilComplete(
	ctx context.Context,
	runID string,
	onProgress func(evaluated, total int),
) (*RunStatusResponse, error) {
	deadline := time.Now().Add(pollTimeout)

	for {
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("poll timeout (%s)", pollTimeout)
		}

		resp, err := r.client.GetRunStatus(ctx, runID)
		if err != nil {
			return nil, err
		}

		if resp.Progress != nil {
			onProgress(resp.Progress.EvaluatedCases, resp.Progress.TotalCases)
		}

		switch resp.Status {
		case "completed", "failed":
			return resp, nil
		}

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(pollInterval):
		}
	}
}

func (r *Runner) pollAutoRunUntilComplete(
	ctx context.Context,
	testRunID string,
	onProgress func(progress float64),
) (*AutoRunResultsResponse, error) {
	deadline := time.Now().Add(pollTimeout)
	for {
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("poll timeout (%s)", pollTimeout)
		}

		resp, err := r.client.GetAutoRunResults(ctx, testRunID)
		if err != nil {
			return nil, err
		}
		if onProgress != nil {
			onProgress(resp.Progress)
		}
		status := resp.Status
		if resp.TestRun != nil && resp.TestRun.Status != "" {
			status = resp.TestRun.Status
		}
		switch status {
		case "completed", "failed":
			return resp, nil
		}

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(pollInterval):
		}
	}
}

// ── Builder helpers ───────────────────────────────────────────────────────────

func buildCreateRequest(cfg *config.Config, ov RunOverrides, report *RunReport) CreateAutoTestRunRequest {
	suites := report.Suites

	queryCount := cfg.Simulation.QueryCount
	if ov.QueryCount > 0 {
		queryCount = ov.QueryCount
	}

	testsetMode := cfg.Testset.Mode
	var testsetID *string
	if ov.TestsetID != "" {
		testsetMode = "specific"
		testsetID = &ov.TestsetID
	} else if ov.UsePrevious {
		testsetMode = "previous"
	} else if ov.GenerateNew {
		testsetMode = "generate_new"
	}

	freeze := cfg.Testset.Freeze
	if ov.Freeze != nil {
		freeze = *ov.Freeze
	}

	useKB := cfg.Simulation.UseKnowledgeBase
	if cfg.KnowledgeBase == nil {
		useKB = false
	}

	return CreateAutoTestRunRequest{
		ProjectID: cfg.Project.ID,
		Version:   report.Version,
		Suites:    suites,
		Simulation: SimulationPayload{
			Mode:             cfg.Simulation.Mode,
			QueryCount:       queryCount,
			UseKnowledgeBase: useKB,
			Language:         cfg.Simulation.Language,
			Difficulty:       cfg.Simulation.Difficulty,
		},
		Testset: TestsetPayload{
			Mode:   testsetMode,
			Freeze: freeze,
			ID:     testsetID,
		},
		Source: "cli",
	}
}

func buildAutoRunRequest(cfg *config.Config, ov RunOverrides) (AutoRunRequest, error) {
	ar := cfg.AutoRun
	testID := firstNonEmpty(ov.TestID, ar.TestID)
	scenario := strings.ToUpper(firstNonEmpty(ov.Scenario, ar.Scenario))
	if testID == "" {
		return AutoRunRequest{}, fmt.Errorf("test_id is required: use --test-id or auto_run.test_id")
	}
	if scenario == "" {
		return AutoRunRequest{}, fmt.Errorf("scenario is required: use --scenario or auto_run.scenario")
	}
	integrationID := firstNonEmpty(ov.IntegrationID, ar.IntegrationID)
	knowledgeBaseID := firstNonEmpty(ov.KnowledgeBaseID, ar.KnowledgeBaseID)
	dynamicProbeGeneration := ov.DynamicProbeGeneration || ar.DynamicProbeGeneration

	if (scenario == "A" || scenario == "C") && strings.TrimSpace(integrationID) == "" {
		return AutoRunRequest{}, fmt.Errorf("integration_id is required for scenario %s; use --integration or set auto_run.integration_id", scenario)
	}
	switch strings.ToUpper(strings.TrimSpace(testID)) {
	case "TG-01", "TG-02":
		if strings.TrimSpace(knowledgeBaseID) == "" {
			return AutoRunRequest{}, fmt.Errorf("knowledge_base_id is required for %s; use --knowledge-base or set auto_run.knowledge_base_id", testID)
		}
	case "DL-01":
		if dynamicProbeGeneration && strings.TrimSpace(knowledgeBaseID) == "" {
			return AutoRunRequest{}, fmt.Errorf("knowledge_base_id is required for DL-01 when --dynamic-probes is enabled")
		}
	}

	questions := append([]string{}, ar.Questions...)
	if ar.QuestionsFile != "" {
		loaded, err := loadQuestions(ar.QuestionsFile)
		if err != nil {
			return AutoRunRequest{}, err
		}
		questions = append(questions, loaded...)
	}
	if ov.QuestionsFile != "" {
		loaded, err := loadQuestions(ov.QuestionsFile)
		if err != nil {
			return AutoRunRequest{}, err
		}
		questions = append(questions, loaded...)
	}
	questions = append(questions, ov.Questions...)

	qaFile := firstNonEmpty(ov.QAItemsFile, ar.QAItemsFile)
	var qaItems []QAItem
	if qaFile != "" {
		loaded, err := loadQAItems(qaFile)
		if err != nil {
			return AutoRunRequest{}, err
		}
		qaItems = loaded
	}

	req := AutoRunRequest{
		TestID:                 testID,
		Scenario:               scenario,
		IntegrationID:          strPtr(integrationID),
		DatasetSnapshotID:      strPtr(firstNonEmpty(ov.DatasetSnapshotID, ar.DatasetSnapshotID, ov.TestsetID)),
		KnowledgeBaseID:        strPtr(knowledgeBaseID),
		SystemPrompt:           strPtr(firstNonEmpty(ov.SystemPrompt, ar.SystemPrompt)),
		RepeatCount:            firstNonZero(ov.RepeatCount, ar.RepeatCount, 5),
		QAItems:                qaItems,
		Questions:              questions,
		Language:               firstNonEmpty(ov.Language, ar.Language, "English"),
		AppContext:             strPtr(firstNonEmpty(ov.AppContext, ar.AppContext)),
		AttackFocus:            firstNonEmpty(ov.AttackFocus, ar.AttackFocus, "balanced"),
		Aggressiveness:         firstNonEmpty(ov.Aggressiveness, ar.Aggressiveness, "realistic"),
		GenerateNewSnapshot:    ov.GenerateNew || ov.GenerateNewSnapshot || ar.GenerateNewSnapshot,
		DynamicProbeGeneration: dynamicProbeGeneration,
	}
	return req, nil
}

func loadQuestions(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read questions file: %w", err)
	}
	if strings.EqualFold(filepath.Ext(path), ".json") {
		var questions []string
		if err := json.Unmarshal(data, &questions); err != nil {
			return nil, fmt.Errorf("failed to parse questions JSON: %w", err)
		}
		return cleanQuestions(questions), nil
	}
	return cleanQuestions(strings.Split(string(data), "\n")), nil
}

func loadQAItems(path string) ([]QAItem, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read QA items file: %w", err)
	}
	if strings.EqualFold(filepath.Ext(path), ".json") {
		var items []QAItem
		if err := json.Unmarshal(data, &items); err != nil {
			return nil, fmt.Errorf("failed to parse QA items JSON: %w", err)
		}
		return items, nil
	}
	reader := csv.NewReader(strings.NewReader(string(data)))
	rows, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("failed to parse QA CSV: %w", err)
	}
	if len(rows) == 0 {
		return nil, nil
	}
	header := map[string]int{}
	for i, col := range rows[0] {
		header[strings.ToLower(strings.TrimSpace(col))] = i
	}
	value := func(row []string, name string) string {
		idx, ok := header[name]
		if !ok || idx >= len(row) {
			return ""
		}
		return strings.TrimSpace(row[idx])
	}
	var items []QAItem
	for _, row := range rows[1:] {
		item := QAItem{
			Question:        value(row, "question"),
			Answer:          value(row, "answer"),
			ReferenceAnswer: value(row, "reference_answer"),
			Category:        value(row, "category"),
			SubTest:         value(row, "sub_test"),
		}
		if item.Question != "" || item.Answer != "" {
			items = append(items, item)
		}
	}
	return items, nil
}

func cleanQuestions(in []string) []string {
	out := make([]string, 0, len(in))
	for _, q := range in {
		q = strings.TrimSpace(q)
		if q != "" {
			out = append(out, q)
		}
	}
	return out
}

func strPtr(s string) *string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return &s
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func firstNonZero(values ...int) int {
	for _, v := range values {
		if v != 0 {
			return v
		}
	}
	return 0
}

func populateReportFromAutoRun(report *RunReport, run *AutoRunTestRun, ov RunOverrides) {
	report.RunID = run.ID
	report.TestsetID = run.DatasetSnapshotID
	report.Status = run.Status
	report.Verdict = run.Verdict
	report.BenchmarkVersion = run.BenchmarkVersion
	report.RunVersion = run.RunVersion
	report.DeltaScore = run.DeltaScore
	report.TotalCases = run.TotalQuestions
	report.TopFailures = summarizeAutoRunFailures(run.Results)

	score := 0.0
	if run.OverallScore != nil {
		score = *run.OverallScore
	}
	report.OverallScore = score
	normalized := score / 100
	report.Summary = &RunSummary{
		ReliabilityScore: normalized,
		TotalCases:       run.TotalQuestions,
		FailedCases:      run.FailedQuestions + run.ErrorQuestions,
		Decision:         strings.ToLower(run.Verdict),
	}
	report.Thresholds = &ThresholdResult{
		ReliabilityMin: thresholdToRatio(run.VerdictThreshold),
		Passed:         strings.EqualFold(run.Verdict, "PASS"),
	}
	required := thresholdToRatio(run.VerdictThreshold)
	if ov.FailUnder > 0 {
		required = ov.FailUnder
	}
	if required > 0 && normalized < required {
		report.FailedGates = []FailedGate{{Name: "overall_score", Required: required, Got: normalized}}
	}
	if strings.EqualFold(run.Verdict, "FAIL") || len(report.FailedGates) > 0 {
		report.ExitCode = 1
		return
	}
	report.ExitCode = 0
}

func thresholdToRatio(v *float64) float64 {
	if v == nil {
		return 0
	}
	if *v > 1 {
		return *v / 100
	}
	return *v
}

func summarizeAutoRunFailures(items []AutoRunResultItem) []TopFailure {
	counts := map[string]int{}
	for _, item := range items {
		if item.Passed != nil && *item.Passed {
			continue
		}
		key := firstNonEmpty(item.ErrorType, item.Category, item.SubTest, "failed_question")
		counts[key]++
	}
	out := make([]TopFailure, 0, len(counts))
	for key, count := range counts {
		out = append(out, TopFailure{Type: key, Count: count, Severity: "high"})
	}
	return out
}

// ── Gate evaluation ───────────────────────────────────────────────────────────

func buildFailedGates(cfg *config.Config, ov RunOverrides, summary *RunSummary) []FailedGate {
	if summary == nil {
		return nil
	}

	var gates []FailedGate

	check := func(name string, min, got float64) {
		if min > 0 && got < min {
			gates = append(gates, FailedGate{Name: name, Required: min, Got: got})
		}
	}

	t := cfg.Thresholds
	check("reliability_min", t.ReliabilityMin, summary.ReliabilityScore)
	check("grounding_min", t.GroundingMin, summary.GroundingScore)
	check("instruction_obedience_min", t.InstructionObedienceMin, summary.InstructionObedienceScore)
	check("safety_min", t.SafetyMin, summary.SafetyScore)

	// --fail-under overrides reliability gate
	if ov.FailUnder > 0 {
		if summary.ReliabilityScore < ov.FailUnder {
			// Replace or add reliability gate with CLI value.
			replaced := false
			for i, g := range gates {
				if g.Name == "reliability_min" {
					gates[i].Required = ov.FailUnder
					replaced = true
					break
				}
			}
			if !replaced {
				gates = append(gates, FailedGate{
					Name:     "reliability_min (--fail-under)",
					Required: ov.FailUnder,
					Got:      summary.ReliabilityScore,
				})
			}
		}
	}

	return gates
}

func resolveExitCode(report *RunReport) int {
	if report.Summary == nil {
		return 5 // partial evaluation
	}
	if len(report.FailedGates) > 0 {
		return 1 // threshold fail
	}
	return 0
}

// ── Conversion helpers ────────────────────────────────────────────────────────

func resolveVersion(cfg *config.Config, ov RunOverrides) string {
	if ov.Version != "" {
		return ov.Version
	}
	return ""
}

func resolveSuites(cfg *config.Config, ov RunOverrides) []string {
	if len(ov.Suites) > 0 {
		return ov.Suites
	}
	return cfg.Tests.Suites
}

// ── Local artifact ────────────────────────────────────────────────────────────

type localArtifact struct {
	RunID     string      `json:"run_id"`
	ProjectID string      `json:"project_id"`
	Version   string      `json:"version"`
	TestsetID string      `json:"testset_id"`
	Summary   *RunSummary `json:"summary"`
	ReportURL string      `json:"report_url"`
}

func writeLocalArtifact(report *RunReport) error {
	if err := os.MkdirAll(".heimdal/runs", 0o755); err != nil {
		return err
	}
	if err := os.MkdirAll(".heimdal/reports", 0o755); err != nil {
		return err
	}

	artifact := localArtifact{
		RunID:     report.RunID,
		ProjectID: report.ProjectID,
		Version:   report.Version,
		TestsetID: report.TestsetID,
		Summary:   report.Summary,
		ReportURL: report.ReportURL,
	}

	data, err := json.MarshalIndent(artifact, "", "  ")
	if err != nil {
		return err
	}

	runPath := fmt.Sprintf(".heimdal/runs/%s.json", report.RunID)
	if err := os.WriteFile(runPath, data, 0o644); err != nil {
		return err
	}

	// Overwrite latest.json
	return os.WriteFile(".heimdal/reports/latest.json", data, 0o644)
}
