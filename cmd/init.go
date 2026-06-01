package cmd

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"text/template"

	"github.com/ai-la/cli/internal/auth"
	"github.com/ai-la/cli/internal/runner"
	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:     "init",
	Short:   "Create heimdal.yaml in the current project",
	GroupID: "setup",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runInit()
	},
}

var initProjectID string
var initIntegrationID string

func init() {
	initCmd.Flags().StringVar(&initProjectID, "project", "", "Default project ID for heimdal.yaml")
	initCmd.Flags().StringVar(&initIntegrationID, "integration", "", "Default integration ID for scenario A/C")
}

var heimdalYamlTmpl = `version: 1

project:
    id: "{{ .ProjectID }}"
    name: "{{ .ProjectName }}"

integration:
    type: "{{ .IntegrationType }}"
    endpoint: "{{ .IntegrationEndpoint }}"
    method: "{{ .IntegrationMethod }}"
    timeout_seconds: {{ .IntegrationTimeoutSeconds }}
    headers:
        Authorization: "{{ .IntegrationAuthHeader }}"
        Content-Type: "application/json"
    request_template:
        query: "{{ "{{" }} query {{ "}}" }}"
        session_id: "{{ "{{" }} run_id {{ "}}" }}"
        metadata:
            source: "heimdal-cli"
            test_id: "{{ "{{" }} test_id {{ "}}" }}"
            suite_id: "{{ "{{" }} suite_id {{ "}}" }}"
    response:
        answer_path: "{{ .IntegrationAnswerPath }}"
        tool_traces_path: "{{ .IntegrationToolTracesPath }}"
        retrieved_chunks_path: "{{ .IntegrationRetrievedChunksPath }}"

auto_run:
    test_id: "{{ .TestID }}"
    scenario: "{{ .Scenario }}"
    integration_id: "{{ .IntegrationID }}"
    knowledge_base_id: "{{ .KnowledgeBaseID }}"
    system_prompt: "{{ .SystemPrompt }}"
    repeat_count: {{ .RepeatCount }}
    qa_items_file: "{{ .QAItemsFile }}"
    questions_file: "{{ .QuestionsFile }}"
    language: "{{ .Language }}"
    app_context: "{{ .AppContext }}"
    attack_focus: "{{ .AttackFocus }}"
    aggressiveness: "{{ .Aggressiveness }}"
    generate_new_snapshot: {{ .GenerateNewSnapshot }}
    dynamic_probe_generation: {{ .DynamicProbeGeneration }}
    fail_under: {{ .FailUnder }}

knowledge_base:
    mode: "project"
    project_id: "{{ .ProjectID }}"
    required: false
`

type initAnswers struct {
	ProjectID                      string
	ProjectName                    string
	IntegrationType                string
	IntegrationEndpoint            string
	IntegrationMethod              string
	IntegrationTimeoutSeconds      int
	IntegrationAuthHeader          string
	IntegrationAnswerPath          string
	IntegrationToolTracesPath      string
	IntegrationRetrievedChunksPath string
	TestID                         string
	Scenario                       string
	IntegrationID                  string
	KnowledgeBaseID                string
	SystemPrompt                   string
	RepeatCount                    int
	QAItemsFile                    string
	QuestionsFile                  string
	Language                       string
	AppContext                     string
	AttackFocus                    string
	Aggressiveness                 string
	GenerateNewSnapshot            bool
	DynamicProbeGeneration         bool
	FailUnder                      float64
}

func runInit() error {
	if _, err := os.Stat("heimdal.yaml"); err == nil {
		fmt.Println("heimdal.yaml already exists. Updating...")
	}

	r := bufio.NewReader(os.Stdin)
	a := initAnswers{}

	fmt.Println()
	fmt.Println("Creating heimdal.yaml...")
	fmt.Println(strings.Repeat("-", 40))

	projectDefault := strings.TrimSpace(initProjectID)
	if projectDefault == "" {
		projectDefault = activeProjectContextID()
	}
	if projectDefault != "" {
		if resolved, err := resolveProjectIDPrefix(projectDefault); err == nil {
			projectDefault = resolved
		}
	}
	a.ProjectID = prompt(r, "Project ID", projectDefault)
	if a.ProjectID == "" {
		return fmt.Errorf("project ID cannot be empty")
	}
	a.ProjectName = prompt(r, "Project name", a.ProjectID)
	a.IntegrationType = prompt(r, "Integration type [custom_http/openai_compatible]", "custom_http")
	a.IntegrationEndpoint = prompt(r, "Integration endpoint", "")
	a.IntegrationMethod = strings.ToUpper(prompt(r, "Integration HTTP method", "POST"))
	integrationTimeoutStr := prompt(r, "Integration timeout seconds", "45")
	integrationTimeout, timeoutErr := strconv.Atoi(integrationTimeoutStr)
	if timeoutErr != nil || integrationTimeout <= 0 {
		integrationTimeout = 45
	}
	a.IntegrationTimeoutSeconds = integrationTimeout
	a.IntegrationAuthHeader = prompt(r, "Integration auth header", "Bearer ${AGENT_API_KEY}")
	a.IntegrationAnswerPath = prompt(r, "Integration response.answer_path", "data.answer")
	a.IntegrationToolTracesPath = prompt(r, "Integration response.tool_traces_path", "data.tool_traces")
	a.IntegrationRetrievedChunksPath = prompt(r, "Integration response.retrieved_chunks_path", "data.retrieved_chunks")

	fmt.Println()
	fmt.Println("  Tests: AT-01, CS-01, IM-01, IM-02, RC-01, DL-01, TG-01, TG-02")
	fmt.Println("  Scenarios: A=live model, B=existing Q&A, C=generate/reuse benchmark")
	fmt.Println()

	a.TestID = prompt(r, "Auto Run Test ID", "CS-01")
	a.Scenario = strings.ToUpper(prompt(r, "Scenario [A/B/C]", "A"))

	integrationDefault := strings.TrimSpace(initIntegrationID)
	if integrationDefault != "" && a.ProjectID != "" {
		if resolvedIntegrationID, err := resolveIntegrationIDPrefix(a.ProjectID, integrationDefault); err == nil {
			integrationDefault = resolvedIntegrationID
		}
	}
	a.IntegrationID = prompt(r, "Integration ID (for A/C)", integrationDefault)
	a.KnowledgeBaseID = prompt(r, "Knowledge Base ID (optional)", "")
	a.SystemPrompt = prompt(r, "System prompt (optional)", "")

	if (a.Scenario == "A" || a.Scenario == "C") && strings.TrimSpace(a.IntegrationID) == "" {
		return fmt.Errorf("integration_id is required for scenario %s; use --integration <id> or choose scenario B", a.Scenario)
	}

	repeatStr := prompt(r, "Repeat count (RC-01)", "5")
	repeatCount, err := strconv.Atoi(repeatStr)
	if err != nil || repeatCount <= 0 {
		repeatCount = 5
	}
	a.RepeatCount = repeatCount

	a.QAItemsFile = prompt(r, "Q&A file (Scenario B, JSON/CSV)", "")
	a.QuestionsFile = prompt(r, "Questions file (JSON/TXT)", "")
	a.Language = prompt(r, "Language", "English")
	a.AppContext = prompt(r, "App context (optional)", "")
	a.AttackFocus = prompt(r, "Attack focus (IM-01)", "balanced")
	a.Aggressiveness = prompt(r, "Aggressiveness (IM-01)", "realistic")
	a.GenerateNewSnapshot = strings.ToLower(prompt(r, "Generate a new benchmark for scenario C? [y/N]", "n")) == "y"
	a.DynamicProbeGeneration = strings.ToLower(prompt(r, "Enable DL-01 dynamic probes? [y/N]", "n")) == "y"
	a.FailUnder = 0

	fmt.Println()
	fmt.Println(strings.Repeat("-", 40))

	tmpl, err := template.New("heimdal").Parse(heimdalYamlTmpl)
	if err != nil {
		return fmt.Errorf("template error: %w", err)
	}

	f, err := os.Create("heimdal.yaml")
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer f.Close()

	if err := tmpl.Execute(f, a); err != nil {
		return fmt.Errorf("failed to write yaml: %w", err)
	}

	fmt.Println("✓ heimdal.yaml created")
	fmt.Println()
	fmt.Println("Next steps:")
	if _, err := auth.LoadToken(); err != nil {
		fmt.Println("  auth login")
	}
	fmt.Println("  integrations apply")
	fmt.Println("  config validate")
	fmt.Println("  test auto")
	fmt.Println()

	return nil
}

func prompt(r *bufio.Reader, label, defaultVal string) string {
	if defaultVal != "" {
		fmt.Printf("  %s [%s]: ", label, defaultVal)
	} else {
		fmt.Printf("  %s: ", label)
	}

	line, _ := r.ReadString('\n')
	line = strings.TrimSpace(line)

	if line == "" {
		return defaultVal
	}
	return line
}

func resolveIntegrationIDPrefix(projectID, input string) (string, error) {
	projectID = strings.TrimSpace(projectID)
	input = strings.TrimSpace(input)
	if projectID == "" || input == "" {
		return "", fmt.Errorf("project id and integration id are required")
	}
	client, err := runner.NewClientFromToken(nil)
	if err != nil {
		return "", err
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
