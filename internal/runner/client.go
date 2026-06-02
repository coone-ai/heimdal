package runner

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const defaultBaseURL = "https://ailab.co-one.co"

// ── Client ────────────────────────────────────────────────────────────────────

type HeimdalClient struct {
	apiKey         string
	baseURL        string
	organizationID string
	http           *http.Client
}

func NewHeimdalClient(apiKey string) *HeimdalClient {
	return &HeimdalClient{
		apiKey:  apiKey,
		baseURL: defaultBaseURL,
		http:    &http.Client{Timeout: 60 * time.Second},
	}
}

// WithBaseURL overrides the API base URL (useful for testing / self-hosted).
func (c *HeimdalClient) WithBaseURL(u string) *HeimdalClient {
	c.baseURL = u
	return c
}

func (c *HeimdalClient) WithOrganizationID(orgID string) *HeimdalClient {
	c.organizationID = strings.TrimSpace(orgID)
	return c
}

// ── Request helpers ───────────────────────────────────────────────────────────

func (c *HeimdalClient) post(ctx context.Context, path string, body any, out any) error {
	return c.do(ctx, http.MethodPost, path, body, out)
}

func (c *HeimdalClient) get(ctx context.Context, path string, out any) error {
	return c.do(ctx, http.MethodGet, path, nil, out)
}

func (c *HeimdalClient) do(ctx context.Context, method, path string, body any, out any) error {
	var reqBody io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("failed to marshal request body: %w", err)
		}
		reqBody = bytes.NewReader(b)
	}

	req, err := http.NewRequestWithContext(ctx, method, strings.TrimRight(c.baseURL, "/")+path, reqBody)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Source", "cli")
	if c.organizationID != "" {
		req.Header.Set("X-Organization-Id", c.organizationID)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("http client: %w", err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode >= 400 {
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(respBytes))
	}

	if out != nil {
		if err := json.Unmarshal(respBytes, out); err != nil {
			return fmt.Errorf("response parse error (%s %s): %w; body=%s", method, strings.TrimRight(c.baseURL, "/")+path, err, string(respBytes))
		}
	}

	return nil
}

func (c *HeimdalClient) doMultipart(ctx context.Context, path string, build func(*multipart.Writer) error, out any) error {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	if err := build(writer); err != nil {
		_ = writer.Close()
		return err
	}
	if err := writer.Close(); err != nil {
		return fmt.Errorf("failed to close multipart body: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(c.baseURL, "/")+path, &body)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("X-Source", "cli")
	if c.organizationID != "" {
		req.Header.Set("X-Organization-Id", c.organizationID)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("http client: %w", err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode >= 400 {
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(respBytes))
	}

	if out != nil {
		if err := json.Unmarshal(respBytes, out); err != nil {
			return fmt.Errorf("response parse error (POST %s): %w; body=%s", strings.TrimRight(c.baseURL, "/")+path, err, string(respBytes))
		}
	}

	return nil
}

func (c *HeimdalClient) GetAutoRunTests(ctx context.Context) (*AutoRunTestsResponse, error) {
	var resp AutoRunTestsResponse
	if err := c.get(ctx, "/api/auto-run-test/tests", &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *HeimdalClient) RunAutoRunTest(ctx context.Context, projectID string, req AutoRunRequest) (*AutoRunStartResponse, error) {
	path := "/api/auto-run-test/run"
	if projectID != "" {
		path += "?project_id=" + url.QueryEscape(projectID)
	}
	var resp AutoRunStartResponse
	if err := c.post(ctx, path, req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *HeimdalClient) GetAutoRunResults(ctx context.Context, testRunID string) (*AutoRunResultsResponse, error) {
	var resp AutoRunResultsResponse
	if err := c.get(ctx, "/api/auto-run-test/"+url.PathEscape(testRunID)+"/results", &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *HeimdalClient) ListAutoRunRuns(ctx context.Context, projectID, testID, scenario string, limit, offset int) (*AutoRunRunsResponse, error) {
	q := url.Values{}
	q.Set("project_id", projectID)
	if limit <= 0 {
		limit = 20
	}
	q.Set("limit", fmt.Sprintf("%d", limit))
	q.Set("offset", fmt.Sprintf("%d", offset))
	if testID != "" {
		q.Set("test_id", testID)
	}
	if scenario != "" {
		q.Set("scenario", scenario)
	}
	var resp AutoRunRunsResponse
	if err := c.get(ctx, "/api/auto-run-test/runs?"+q.Encode(), &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *HeimdalClient) ListAutoRunDatasets(ctx context.Context, projectID, testID, scenario string, limit, offset int) (*AutoRunDatasetsResponse, error) {
	q := url.Values{}
	q.Set("test_id", testID)
	if scenario != "" {
		q.Set("scenario", scenario)
	}
	if limit <= 0 {
		limit = 50
	}
	q.Set("limit", fmt.Sprintf("%d", limit))
	q.Set("offset", fmt.Sprintf("%d", offset))
	var resp AutoRunDatasetsResponse
	if err := c.get(ctx, "/api/auto-run-test/"+url.PathEscape(projectID)+"/datasets?"+q.Encode(), &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *HeimdalClient) ListProjects(ctx context.Context) (*ProjectsResponse, error) {
	var resp ProjectsResponse
	if err := c.get(ctx, "/api/projects", &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *HeimdalClient) ListOrganizations(ctx context.Context) (*OrganizationsResponse, error) {
	var resp []OrganizationWithRole
	if err := c.get(ctx, "/api/organizations", &resp); err != nil {
		return nil, err
	}
	return &OrganizationsResponse{Organizations: resp}, nil
}

func (c *HeimdalClient) ListIntegrations(ctx context.Context, projectID string) (*IntegrationsResponse, error) {
	q := url.Values{}
	if projectID != "" {
		q.Set("project_id", projectID)
	}
	path := "/api/integrations"
	if encoded := q.Encode(); encoded != "" {
		path += "?" + encoded
	}
	var resp IntegrationsResponse
	if err := c.get(ctx, path, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *HeimdalClient) GetActiveIntegrationConfig(ctx context.Context, integrationID string) (*IntegrationConfigResponse, error) {
	var resp IntegrationConfigResponse
	if err := c.get(ctx, "/api/integrations/"+url.PathEscape(integrationID)+"/configs/active", &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *HeimdalClient) CreateIntegration(ctx context.Context, req IntegrationCreateRequest) (*IntegrationSummary, error) {
	var resp IntegrationSummary
	if err := c.post(ctx, "/api/integrations", req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *HeimdalClient) CreateIntegrationConfig(ctx context.Context, integrationID string, req IntegrationConfigCreateRequest) (*IntegrationConfigCreateEnvelope, error) {
	var resp IntegrationConfigCreateEnvelope
	if err := c.post(ctx, "/api/integrations/"+url.PathEscape(integrationID)+"/configs", req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *HeimdalClient) PublishIntegrationConfig(ctx context.Context, integrationID, configID string) (*IntegrationConfigPublishEnvelope, error) {
	var resp IntegrationConfigPublishEnvelope
	if err := c.post(ctx, "/api/integrations/"+url.PathEscape(integrationID)+"/configs/"+url.PathEscape(configID)+"/publish", map[string]any{}, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *HeimdalClient) ListKnowledgeBases(ctx context.Context, projectID string) (*KnowledgeBasesResponse, error) {
	q := url.Values{}
	if projectID != "" {
		q.Set("project_id", projectID)
	}
	path := "/api/knowledge-base"
	if encoded := q.Encode(); encoded != "" {
		path += "?" + encoded
	}
	var resp KnowledgeBasesResponse
	if err := c.get(ctx, path, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *HeimdalClient) UploadKnowledgeBase(ctx context.Context, req KnowledgeBaseUploadRequest) (*KnowledgeBaseSummary, error) {
	var resp KnowledgeBaseSummary
	err := c.doMultipart(ctx, "/api/knowledge-base/upload", func(w *multipart.Writer) error {
		for _, filePath := range req.FilePaths {
			if strings.TrimSpace(filePath) == "" {
				continue
			}
			file, err := os.Open(filePath)
			if err != nil {
				return fmt.Errorf("failed to open %s: %w", filePath, err)
			}
			defer file.Close()

			part, err := w.CreateFormFile("files", filepath.Base(filePath))
			if err != nil {
				return fmt.Errorf("failed to attach %s: %w", filePath, err)
			}
			if _, err := io.Copy(part, file); err != nil {
				return fmt.Errorf("failed to read %s: %w", filePath, err)
			}
		}
		if err := w.WriteField("name", req.Name); err != nil {
			return err
		}
		if req.Description != "" {
			if err := w.WriteField("description", req.Description); err != nil {
				return err
			}
		}
		if err := w.WriteField("project_id", req.ProjectID); err != nil {
			return err
		}
		if req.SourceLanguage != "" {
			if err := w.WriteField("source_language", req.SourceLanguage); err != nil {
				return err
			}
		}
		if req.TargetLanguage != "" {
			if err := w.WriteField("target_language", req.TargetLanguage); err != nil {
				return err
			}
		}
		return nil
	}, &resp)
	if err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *HeimdalClient) GetJobByEntity(ctx context.Context, entityType, entityID string) (*JobDetailResponse, error) {
	q := url.Values{}
	q.Set("entity_type", entityType)
	q.Set("entity_id", entityID)
	path := "/api/jobs?" + q.Encode()
	var resp JobDetailResponse
	if err := c.get(ctx, path, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *HeimdalClient) ListJobs(ctx context.Context, status, entityType, projectID string, limit int) (*JobsResponse, error) {
	q := url.Values{}
	if strings.TrimSpace(status) != "" {
		q.Set("status", strings.TrimSpace(status))
	}
	if strings.TrimSpace(entityType) != "" {
		q.Set("entity_type", strings.TrimSpace(entityType))
	}
	if strings.TrimSpace(projectID) != "" {
		q.Set("project_id", strings.TrimSpace(projectID))
	}
	if limit > 0 {
		q.Set("limit", fmt.Sprintf("%d", limit))
	}
	path := "/api/jobs/list"
	if encoded := q.Encode(); encoded != "" {
		path += "?" + encoded
	}
	var resp JobsResponse
	if err := c.get(ctx, path, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

type AutoRunRequest struct {
	TestID                 string   `json:"test_id"`
	Scenario               string   `json:"scenario"`
	IntegrationID          *string  `json:"integration_id,omitempty"`
	DatasetSnapshotID      *string  `json:"dataset_snapshot_id,omitempty"`
	KnowledgeBaseID        *string  `json:"knowledge_base_id,omitempty"`
	SystemPrompt           *string  `json:"system_prompt,omitempty"`
	RepeatCount            int      `json:"repeat_count,omitempty"`
	QAItems                []QAItem `json:"qa_items,omitempty"`
	Questions              []string `json:"questions,omitempty"`
	Language               string   `json:"language,omitempty"`
	AppContext             *string  `json:"app_context,omitempty"`
	AttackFocus            string   `json:"attack_focus,omitempty"`
	Aggressiveness         string   `json:"aggressiveness,omitempty"`
	GenerateNewSnapshot    bool     `json:"generate_new_snapshot,omitempty"`
	DynamicProbeGeneration bool     `json:"dynamic_probe_generation,omitempty"`
}

type QAItem struct {
	Question        string `json:"question"`
	Answer          string `json:"answer"`
	ReferenceAnswer string `json:"reference_answer,omitempty"`
	Category        string `json:"category,omitempty"`
	SubTest         string `json:"sub_test,omitempty"`
}

type AutoRunStartResponse struct {
	JobID             string `json:"job_id"`
	TestRunID         string `json:"test_run_id"`
	DatasetSnapshotID string `json:"dataset_snapshot_id"`
	Status            string `json:"status"`
	Message           string `json:"message"`
}

type AutoRunResultsResponse struct {
	JobID        string          `json:"job_id"`
	Status       string          `json:"status"`
	Progress     float64         `json:"progress"`
	ErrorMessage string          `json:"error_message"`
	TestRun      *AutoRunTestRun `json:"test_run"`
}

type AutoRunTestRun struct {
	ID                string              `json:"id"`
	TestID            string              `json:"test_id"`
	Scenario          string              `json:"scenario"`
	Status            string              `json:"status"`
	DatasetSnapshotID string              `json:"dataset_snapshot_id"`
	BenchmarkVersion  string              `json:"benchmark_version"`
	RunVersion        string              `json:"run_version"`
	OverallScore      *float64            `json:"overall_score"`
	CoveragePercent   *float64            `json:"coverage_percent"`
	DeltaScore        *float64            `json:"delta_score"`
	Verdict           string              `json:"verdict"`
	VerdictThreshold  *float64            `json:"verdict_threshold"`
	TotalQuestions    int                 `json:"total_questions"`
	PassedQuestions   int                 `json:"passed_questions"`
	FailedQuestions   int                 `json:"failed_questions"`
	ErrorQuestions    int                 `json:"error_questions"`
	ErrorMessage      string              `json:"error_message"`
	RunMetadata       map[string]any      `json:"run_metadata"`
	Results           []AutoRunResultItem `json:"results"`
}

type AutoRunResultItem struct {
	QuestionIndex   int            `json:"question_index"`
	Question        string         `json:"question"`
	Answer          string         `json:"answer"`
	ReferenceAnswer string         `json:"reference_answer"`
	Passed          *bool          `json:"passed"`
	Score           *float64       `json:"score"`
	ErrorType       string         `json:"error_type"`
	ErrorDetail     string         `json:"error_detail"`
	SubTest         string         `json:"sub_test"`
	Category        string         `json:"category"`
	JudgeOutput     map[string]any `json:"judge_output"`
}

type AutoRunTestsResponse struct {
	Tests             []AutoRunTestDefinition `json:"tests"`
	VerdictThresholds map[string]float64      `json:"verdict_thresholds"`
	FailThresholds    map[string]float64      `json:"fail_thresholds"`
}

type AutoRunTestDefinition struct {
	ID                   string   `json:"id"`
	Name                 string   `json:"name"`
	DescriptionShort     string   `json:"description_short"`
	Tags                 []string `json:"tags"`
	Scenarios            []string `json:"scenarios"`
	RequiresModel        bool     `json:"requires_model"`
	RequiresKB           bool     `json:"requires_kb"`
	RequiresSystemPrompt bool     `json:"requires_system_prompt"`
	QuestionSource       string   `json:"question_source"`
	MaxQuestionsC        int      `json:"max_questions_c"`
}

type AutoRunRunsResponse struct {
	Runs  []AutoRunRunSummary `json:"runs"`
	Total int                 `json:"total"`
}

type AutoRunRunSummary struct {
	ID               string   `json:"id"`
	TestID           string   `json:"test_id"`
	Scenario         string   `json:"scenario"`
	Status           string   `json:"status"`
	BenchmarkVersion string   `json:"benchmark_version"`
	RunVersion       string   `json:"run_version"`
	OverallScore     *float64 `json:"overall_score"`
	DeltaScore       *float64 `json:"delta_score"`
	Verdict          string   `json:"verdict"`
	TotalQuestions   int      `json:"total_questions"`
	FailedQuestions  int      `json:"failed_questions"`
	CreatedAt        string   `json:"created_at"`
}

type AutoRunDatasetsResponse struct {
	Datasets []AutoRunDatasetSummary `json:"datasets"`
	Total    int                     `json:"total"`
}

type AutoRunDatasetSummary struct {
	ID               string `json:"id"`
	TestID           string `json:"test_id"`
	Scenario         string `json:"scenario"`
	Name             string `json:"name"`
	BenchmarkVersion string `json:"benchmark_version"`
	QuestionCount    int    `json:"question_count"`
	SnapshotHash     string `json:"snapshot_hash"`
	CreatedAt        string `json:"created_at"`
}

type ProjectsResponse struct {
	Projects []ProjectSummary `json:"projects"`
}

type OrganizationsResponse struct {
	Organizations []OrganizationWithRole `json:"organizations"`
}

type OrganizationWithRole struct {
	Organization OrganizationSummary `json:"organization"`
	Role         string              `json:"role"`
	JoinedAt     string              `json:"joined_at"`
}

type OrganizationSummary struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Slug        string `json:"slug"`
	IsPersonal  bool   `json:"is_personal"`
	IsTemplate  bool   `json:"is_template"`
	Description string `json:"description"`
}

type ProjectSummary struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	OrganizationID string `json:"organization_id"`
}

type KnowledgeBasesResponse struct {
	KnowledgeBases []KnowledgeBaseSummary `json:"knowledge_bases"`
	Total          int                    `json:"total"`
}

type KnowledgeBaseSummary struct {
	ID                 string `json:"id"`
	Name               string `json:"name"`
	Status             string `json:"status"`
	IsDefault          bool   `json:"is_default"`
	ItemCount          int    `json:"item_count"`
	EmbeddingDimension int    `json:"embedding_dimension"`
	EmbeddingModel     string `json:"embedding_model"`
	CreatedAt          string `json:"created_at"`
}

type KnowledgeBaseUploadRequest struct {
	FilePaths      []string
	Name           string
	Description    string
	ProjectID      string
	SourceLanguage string
	TargetLanguage string
}

type JobDetailResponse struct {
	Job          JobSummary     `json:"job"`
	Steps        []JobStep      `json:"steps"`
	UsageSummary map[string]any `json:"usage_summary"`
}

type JobsResponse struct {
	Jobs  []JobSummary `json:"jobs"`
	Total int          `json:"total"`
}

type JobSummary struct {
	ID             string  `json:"id"`
	JobType        string  `json:"job_type"`
	EntityType     string  `json:"entity_type"`
	EntityID       string  `json:"entity_id"`
	Status         string  `json:"status"`
	Progress       float64 `json:"progress"`
	CurrentStep    string  `json:"current_step"`
	TotalItems     int     `json:"total_items"`
	ProcessedItems int     `json:"processed_items"`
	ErrorMessage   string  `json:"error_message"`
	CreatedAt      string  `json:"created_at"`
	StartedAt      string  `json:"started_at"`
	CompletedAt    string  `json:"completed_at"`
}

type JobStep struct {
	ID           string  `json:"id"`
	StepName     string  `json:"step_name"`
	Status       string  `json:"status"`
	Progress     float64 `json:"progress"`
	StartedAt    string  `json:"started_at"`
	CompletedAt  string  `json:"completed_at"`
	ErrorMessage string  `json:"error_message"`
}

type IntegrationsResponse struct {
	Integrations []IntegrationSummary `json:"integrations"`
}

type IntegrationSummary struct {
	ID          string `json:"id"`
	ProjectID   string `json:"project_id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	IsActive    bool   `json:"is_active"`
}

type IntegrationAuthConfigItem struct {
	Type      string `json:"type"`
	Key       string `json:"key"`
	SecretRef string `json:"secret_ref,omitempty"`
	Value     string `json:"value,omitempty"`
}

type IntegrationCreateRequest struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	ProjectID   string `json:"project_id,omitempty"`
}

type IntegrationConfigCreateRequest struct {
	Method              string                      `json:"method"`
	URL                 string                      `json:"url"`
	AuthConfig          []IntegrationAuthConfigItem `json:"auth_config,omitempty"`
	RequestBodyTemplate string                      `json:"request_body_template"`
	ResponsePath        string                      `json:"response_path,omitempty"`
	Description         string                      `json:"description,omitempty"`
	ProjectID           string                      `json:"project_id,omitempty"`
	TimeoutSeconds      int                         `json:"timeout_seconds,omitempty"`
}

type IntegrationConfigResponse struct {
	ID                  string                      `json:"id"`
	IntegrationID       string                      `json:"integration_id"`
	Method              string                      `json:"method"`
	URL                 string                      `json:"url"`
	AuthConfig          []IntegrationAuthConfigItem `json:"auth_config"`
	RequestBodyTemplate string                      `json:"request_body_template"`
	ResponsePath        string                      `json:"response_path"`
	TimeoutSeconds      int                         `json:"timeout_seconds"`
}

type IntegrationConfigCreateEnvelope struct {
	Config     IntegrationConfigResponse `json:"config"`
	TestResult map[string]any            `json:"test_result"`
}

type IntegrationConfigPublishPayload struct {
	ID            string `json:"id"`
	VersionNumber int    `json:"version_number"`
	Status        string `json:"status"`
	IsActive      bool   `json:"is_active"`
	PublishedAt   string `json:"published_at"`
}

type IntegrationConfigPublishEnvelope struct {
	Config     IntegrationConfigPublishPayload `json:"config"`
	TestResult map[string]any                  `json:"test_result"`
}

// ── API Methods ───────────────────────────────────────────────────────────────

// CreateAutoTestRun → POST /v1/auto-test-runs
func (c *HeimdalClient) CreateAutoTestRun(ctx context.Context, req CreateAutoTestRunRequest) (*CreateAutoTestRunResponse, error) {
	var resp CreateAutoTestRunResponse
	if err := c.post(ctx, "/v1/auto-test-runs", req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// SubmitAgentResponses → POST /v1/auto-test-runs/{run_id}/responses
func (c *HeimdalClient) SubmitAgentResponses(ctx context.Context, runID string, req SubmitResponsesRequest) (*SubmitResponsesResponse, error) {
	var resp SubmitResponsesResponse
	if err := c.post(ctx, fmt.Sprintf("/v1/auto-test-runs/%s/responses", runID), req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// GetRunStatus → GET /v1/auto-test-runs/{run_id}
func (c *HeimdalClient) GetRunStatus(ctx context.Context, runID string) (*RunStatusResponse, error) {
	var resp RunStatusResponse
	if err := c.get(ctx, fmt.Sprintf("/v1/auto-test-runs/%s", runID), &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// ── Request / Response models ─────────────────────────────────────────────────

type CreateAutoTestRunRequest struct {
	ProjectID  string            `json:"project_id"`
	Version    string            `json:"version,omitempty"`
	Suites     []string          `json:"suites"`
	Simulation SimulationPayload `json:"simulation"`
	Testset    TestsetPayload    `json:"testset"`
	Source     string            `json:"source"`
}

type SimulationPayload struct {
	Mode             string `json:"mode"`
	QueryCount       int    `json:"query_count"`
	UseKnowledgeBase bool   `json:"use_knowledge_base"`
	Language         string `json:"language,omitempty"`
	Difficulty       string `json:"difficulty,omitempty"`
}

type TestsetPayload struct {
	Mode   string  `json:"mode"`
	Freeze bool    `json:"freeze"`
	ID     *string `json:"id"`
}

type CreateAutoTestRunResponse struct {
	RunID     string        `json:"run_id"`
	ProjectID string        `json:"project_id"`
	TestsetID string        `json:"testset_id"`
	Cases     []BackendCase `json:"cases"`
}

type BackendCase struct {
	CaseID  string         `json:"case_id"`
	TestID  string         `json:"test_id"`
	SuiteID string         `json:"suite_id"`
	Query   string         `json:"query"`
	Meta    map[string]any `json:"metadata"`
}

type SubmitResponsesRequest struct {
	Responses []CaseResponse `json:"responses"`
	Privacy   PrivacyPayload `json:"privacy"`
}

type CaseResponse struct {
	CaseID          string           `json:"case_id"`
	TestID          string           `json:"test_id"`
	SuiteID         string           `json:"suite_id"`
	Query           string           `json:"query"`
	AgentAnswer     string           `json:"agent_answer"`
	ToolTraces      []map[string]any `json:"tool_traces,omitempty"`
	RetrievedChunks []map[string]any `json:"retrieved_chunks,omitempty"`
	AgentMetadata   AgentMetadata    `json:"agent_metadata"`
}

type AgentMetadata struct {
	LatencyMs  int `json:"latency_ms"`
	StatusCode int `json:"status_code"`
}

type PrivacyPayload struct {
	StoreCases          bool `json:"store_cases"`
	RedactPII           bool `json:"redact_pii"`
	SendToolTraces      bool `json:"send_tool_traces"`
	SendRetrievedChunks bool `json:"send_retrieved_chunks"`
}

type SubmitResponsesResponse struct {
	RunID          string `json:"run_id"`
	Status         string `json:"status"`
	SubmittedCases int    `json:"submitted_cases"`
}

type RunStatusResponse struct {
	RunID       string           `json:"run_id"`
	Status      string           `json:"status"` // "evaluating" | "completed" | "failed"
	Progress    *RunProgress     `json:"progress,omitempty"`
	Summary     *RunSummary      `json:"summary,omitempty"`
	TopFailures []TopFailure     `json:"top_failures,omitempty"`
	Thresholds  *ThresholdResult `json:"thresholds,omitempty"`
	ReportURL   string           `json:"report_url,omitempty"`
}

type RunProgress struct {
	TotalCases     int `json:"total_cases"`
	EvaluatedCases int `json:"evaluated_cases"`
}

type RunSummary struct {
	ReliabilityScore          float64 `json:"reliability_score"`
	GroundingScore            float64 `json:"grounding_score"`
	InstructionObedienceScore float64 `json:"instruction_obedience_score"`
	SafetyScore               float64 `json:"safety_score"`
	TotalCases                int     `json:"total_cases"`
	FailedCases               int     `json:"failed_cases"`
	Decision                  string  `json:"decision"` // "passed" | "at_risk" | "do_not_deploy"
}

type TopFailure struct {
	Type     string `json:"type"`
	Count    int    `json:"count"`
	Severity string `json:"severity"`
}

type ThresholdResult struct {
	ReliabilityMin float64 `json:"reliability_min"`
	Passed         bool    `json:"passed"`
}

// ── Helpers ───────────────────────────────────────────────────────────────────
