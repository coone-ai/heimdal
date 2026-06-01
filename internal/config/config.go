package config

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// ── Models ────────────────────────────────────────────────────────────────────

type Config struct {
	Version       int               `yaml:"version"`
	API           APIConfig         `yaml:"api"`
	Project       ProjectConfig     `yaml:"project"`
	Integration   IntegrationConfig `yaml:"integration"`
	Agent         IntegrationConfig `yaml:"agent"`
	AutoRun       AutoRunConfig     `yaml:"auto_run"`
	KnowledgeBase *KBConfig         `yaml:"knowledge_base"`
	Simulation    SimulationConfig  `yaml:"simulation"`
	Tests         TestsConfig       `yaml:"tests"`
	Testset       TestsetConfig     `yaml:"testset"`
	Evaluation    EvaluationConfig  `yaml:"evaluation"`
	Thresholds    ThresholdConfig   `yaml:"thresholds"`
	Privacy       PrivacyConfig     `yaml:"privacy"`
	Output        OutputConfig      `yaml:"output"`
}

type APIConfig struct {
	BaseURL string `yaml:"base_url"`
}

type ProjectConfig struct {
	ID   string `yaml:"id"`
	Name string `yaml:"name"`
}

type AutoRunConfig struct {
	TestID                 string   `yaml:"test_id"`
	Scenario               string   `yaml:"scenario"`
	IntegrationID          string   `yaml:"integration_id"`
	DatasetSnapshotID      string   `yaml:"dataset_snapshot_id"`
	KnowledgeBaseID        string   `yaml:"knowledge_base_id"`
	SystemPrompt           string   `yaml:"system_prompt"`
	RepeatCount            int      `yaml:"repeat_count"`
	QAItemsFile            string   `yaml:"qa_items_file"`
	QuestionsFile          string   `yaml:"questions_file"`
	Questions              []string `yaml:"questions"`
	Language               string   `yaml:"language"`
	AppContext             string   `yaml:"app_context"`
	AttackFocus            string   `yaml:"attack_focus"`
	Aggressiveness         string   `yaml:"aggressiveness"`
	GenerateNewSnapshot    bool     `yaml:"generate_new_snapshot"`
	DynamicProbeGeneration bool     `yaml:"dynamic_probe_generation"`
	FailUnder              float64  `yaml:"fail_under"`
}

type IntegrationConfig struct {
	Type            string            `yaml:"type"`
	Endpoint        string            `yaml:"endpoint"`
	BaseURL         string            `yaml:"base_url"`
	Model           string            `yaml:"model"`
	APIKey          string            `yaml:"api_key"`
	Method          string            `yaml:"method"`
	TimeoutSeconds  int               `yaml:"timeout_seconds"`
	Headers         map[string]string `yaml:"headers"`
	RequestTemplate map[string]any    `yaml:"request_template"`
	Response        ResponseConfig    `yaml:"response"`
}

// AgentConfig is kept as a type alias for backward compatibility in legacy code paths.
type AgentConfig = IntegrationConfig

type ResponseConfig struct {
	AnswerPath          string `yaml:"answer_path"`
	ToolTracesPath      string `yaml:"tool_traces_path"`
	RetrievedChunksPath string `yaml:"retrieved_chunks_path"`
}

type KBConfig struct {
	Mode      string `yaml:"mode"`
	ProjectID string `yaml:"project_id"`
	Required  bool   `yaml:"required"`
}

type SimulationConfig struct {
	Mode             string `yaml:"mode"`
	QueryCount       int    `yaml:"query_count"`
	Language         string `yaml:"language"`
	Difficulty       string `yaml:"difficulty"`
	UseKnowledgeBase bool   `yaml:"use_knowledge_base"`
}

type TestsConfig struct {
	Mode   string   `yaml:"mode"`
	Suites []string `yaml:"suites"`
}

type TestsetConfig struct {
	Mode   string  `yaml:"mode"`
	Freeze bool    `yaml:"freeze"`
	ID     *string `yaml:"id"`
}

type EvaluationConfig struct {
	Mode      string `yaml:"mode"`
	BatchSize int    `yaml:"batch_size"`
}

type ThresholdConfig struct {
	ReliabilityMin          float64 `yaml:"reliability_min"`
	GroundingMin            float64 `yaml:"grounding_min"`
	InstructionObedienceMin float64 `yaml:"instruction_obedience_min"`
	SafetyMin               float64 `yaml:"safety_min"`
}

type PrivacyConfig struct {
	StoreCases          bool `yaml:"store_cases"`
	RedactPII           bool `yaml:"redact_pii"`
	SendToolTraces      bool `yaml:"send_tool_traces"`
	SendRetrievedChunks bool `yaml:"send_retrieved_chunks"`
}

type OutputConfig struct {
	Format             string `yaml:"format"`
	CreateHostedReport bool   `yaml:"create_hosted_report"`
}

// ── Load ──────────────────────────────────────────────────────────────────────

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read heimdal.yaml: %w", err)
	}

	expanded := expandEnvVars(string(data))

	var cfg Config
	if err := yaml.Unmarshal([]byte(expanded), &cfg); err != nil {
		return nil, fmt.Errorf("yaml parse error: %w", err)
	}

	// Backward compatibility:
	// - if only `agent` exists, mirror into `integration`
	// - if only `integration` exists, mirror into `agent`
	if isIntegrationConfigZero(cfg.Integration) && !isIntegrationConfigZero(cfg.Agent) {
		cfg.Integration = cfg.Agent
	}
	if isIntegrationConfigZero(cfg.Agent) && !isIntegrationConfigZero(cfg.Integration) {
		cfg.Agent = cfg.Integration
	}

	return &cfg, nil
}

func isIntegrationConfigZero(v IntegrationConfig) bool {
	return strings.TrimSpace(v.Type) == "" &&
		strings.TrimSpace(v.Endpoint) == "" &&
		strings.TrimSpace(v.BaseURL) == "" &&
		strings.TrimSpace(v.Model) == "" &&
		strings.TrimSpace(v.APIKey) == "" &&
		strings.TrimSpace(v.Method) == "" &&
		v.TimeoutSeconds == 0 &&
		len(v.Headers) == 0 &&
		len(v.RequestTemplate) == 0 &&
		strings.TrimSpace(v.Response.AnswerPath) == "" &&
		strings.TrimSpace(v.Response.ToolTracesPath) == "" &&
		strings.TrimSpace(v.Response.RetrievedChunksPath) == ""
}

var envVarRe = regexp.MustCompile(`\$\{([^}]+)\}`)

func expandEnvVars(s string) string {
	return envVarRe.ReplaceAllStringFunc(s, func(match string) string {
		key := match[2 : len(match)-1]
		if val := os.Getenv(key); val != "" {
			return val
		}
		return match
	})
}

// ── Validate ──────────────────────────────────────────────────────────────────

type ValidationResult struct {
	Checks []Check
	Valid  bool
}

type Check struct {
	Label  string
	OK     bool
	Detail string
}

func Validate(cfg *Config, rawPath string) ValidationResult {
	var checks []Check

	add := func(label string, ok bool, detail string) {
		checks = append(checks, Check{Label: label, OK: ok, Detail: detail})
	}

	// File
	_, err := os.Stat(rawPath)
	add("heimdal.yaml found", err == nil, "")

	// Version
	add("version: 1", cfg.Version == 1,
		fmt.Sprintf("expected: 1, found: %d", cfg.Version))

	// Project
	add("project.id is set", cfg.Project.ID != "", "")

	if cfg.AutoRun.TestID != "" || cfg.AutoRun.Scenario != "" {
		validTests := map[string]bool{
			"AT-01": true, "CS-01": true, "IM-01": true, "IM-02": true,
			"RC-01": true, "DL-01": true, "TG-01": true, "TG-02": true,
		}
		validScenarios := map[string]bool{"A": true, "B": true, "C": true}
		add("auto_run.test_id is valid", validTests[cfg.AutoRun.TestID],
			"valid values: AT-01, CS-01, IM-01, IM-02, RC-01, DL-01, TG-01, TG-02")
		add("auto_run.scenario is valid", validScenarios[cfg.AutoRun.Scenario],
			"valid values: A, B, C")
		if cfg.AutoRun.Scenario == "A" || cfg.AutoRun.Scenario == "C" {
			add("auto_run.integration_id is set", cfg.AutoRun.IntegrationID != "",
				"Scenario A/C requires a live model integration")
		}
	} else if cfg.Integration.Type != "" {
		// Legacy local-agent config support.
		validTypes := map[string]bool{"custom_http": true, "openai_compatible": true}
		add("integration.type is valid", validTypes[cfg.Integration.Type],
			"valid values: custom_http, openai_compatible")

		switch cfg.Integration.Type {
		case "custom_http":
			add("integration.endpoint is set", cfg.Integration.Endpoint != "", "")
		case "openai_compatible":
			add("integration.base_url is set", cfg.Integration.BaseURL != "", "")
			add("integration.model is set", cfg.Integration.Model != "", "")
		}

		add("integration.response.answer_path is set", cfg.Integration.Response.AnswerPath != "", "")
	}

	// Are integration auth env vars resolved?
	unresolvedAgent := false
	for _, v := range cfg.Integration.Headers {
		if strings.Contains(v, "${") {
			unresolvedAgent = true
			break
		}
	}
	if cfg.Integration.APIKey != "" && strings.Contains(cfg.Integration.APIKey, "${") {
		unresolvedAgent = true
	}
	add("integration auth env vars resolved", !unresolvedAgent,
		"the referenced env var may not be exported")

	if cfg.Simulation.QueryCount != 0 {
		add("simulation.query_count > 0", cfg.Simulation.QueryCount > 0, "")
	}

	if len(cfg.Tests.Suites) > 0 {
		add("at least one test suite is set", true, "")
	}

	if cfg.Thresholds.ReliabilityMin != 0 {
		add("thresholds.reliability_min is valid",
			cfg.Thresholds.ReliabilityMin > 0 && cfg.Thresholds.ReliabilityMin <= 1,
			"must be between 0 and 1")
	}

	allOK := true
	for _, c := range checks {
		if !c.OK {
			allOK = false
			break
		}
	}

	return ValidationResult{Checks: checks, Valid: allOK}
}
