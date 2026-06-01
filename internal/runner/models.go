package runner

// ── Overrides (CLI flags → runner) ───────────────────────────────────────────

// RunOverrides holds values that the user passed as CLI flags,
// taking precedence over heimdal.yaml when set.
type RunOverrides struct {
	Version     string   // --version
	Suites      []string // --suite (repeatable)
	QueryCount  int      // --count
	TestsetID   string   // --testset
	UsePrevious bool     // --use-previous
	GenerateNew bool     // --generate-new
	Freeze      *bool    // --freeze / --no-freeze
	FailUnder   float64  // --fail-under  (0 = not set)
	NoStore     bool     // --no-store
	Redact      bool     // --redact

	TestID                 string
	Scenario               string
	IntegrationID          string
	DatasetSnapshotID      string
	KnowledgeBaseID        string
	SystemPrompt           string
	RepeatCount            int
	QAItemsFile            string
	QuestionsFile          string
	Questions              []string
	Language               string
	AppContext             string
	AttackFocus            string
	Aggressiveness         string
	GenerateNewSnapshot    bool
	DynamicProbeGeneration bool
	ProjectID              string
	APIBaseURL             string
}

// ── Run Report (runner → CLI / output) ───────────────────────────────────────

// RunReport is the final result of a completed auto-test run.
// The output package renders this; the CLI uses ExitCode to set os.Exit.
type RunReport struct {
	RunID      string
	JobID      string
	ProjectID  string
	Version    string
	TestsetID  string
	Suites     []string
	TotalCases int

	Summary     *RunSummary
	TopFailures []TopFailure
	Thresholds  *ThresholdResult

	ReportURL string

	// Decision derived locally when --fail-under is used.
	FailedGates []FailedGate

	// ExitCode follows the spec:
	// 0 = passed, 1 = threshold fail, 2 = config error,
	// 3 = agent error, 4 = heimdal API error, 5 = partial eval
	ExitCode int

	Status           string
	Verdict          string
	OverallScore     float64
	BenchmarkVersion string
	RunVersion       string
	DeltaScore       *float64
}

type FailedGate struct {
	Name     string
	Required float64
	Got      float64
}
