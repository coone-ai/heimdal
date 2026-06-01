package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ai-la/cli/internal/config"
)

// GeneratedCase represents one backend-generated test case that should be sent to the user's agent.
type GeneratedCase struct {
	CaseID  string
	TestID  string
	SuiteID string
	Query   string
	Meta    map[string]any
}

// CompletedCase contains the final response details for a generated case.
type CompletedCase struct {
	GeneratedCase
	AgentAnswer     string
	ToolTraces      []map[string]any
	RetrievedChunks []map[string]any
	AgentLatencyMs  int
	AgentStatusCode int
	Error           string
}

// CallAgent posts payload to a user endpoint and returns response body.
func CallAgent(endpoint string, payload []byte, timeout time.Duration) ([]byte, error) {
	client := &http.Client{Timeout: timeout}
	resp, err := client.Post(endpoint, "application/json", bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}

// CallAgentForCase sends a single generated case to the configured agent and returns the completed result.
func CallAgentForCase(ctx context.Context, cas GeneratedCase, agentCfg config.AgentConfig, runID string) CompletedCase {
	started := time.Now()
	result := CompletedCase{GeneratedCase: cas}

	requestBody, endpoint, method, headers, timeoutSeconds, err := buildRequest(cas, agentCfg, runID)
	if err != nil {
		result.Error = err.Error()
		return result
	}

	req, err := http.NewRequestWithContext(ctx, method, endpoint, bytes.NewReader(requestBody))
	if err != nil {
		result.Error = err.Error()
		return result
	}
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	if req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}

	client := &http.Client{Timeout: time.Duration(timeoutSeconds) * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		result.AgentLatencyMs = int(time.Since(started).Milliseconds())
		result.Error = err.Error()
		return result
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	result.AgentLatencyMs = int(time.Since(started).Milliseconds())
	result.AgentStatusCode = resp.StatusCode
	if err != nil {
		result.Error = err.Error()
		return result
	}

	result.AgentAnswer = extractAnswer(body, agentCfg)
	if result.AgentAnswer == "" {
		result.AgentAnswer = strings.TrimSpace(string(body))
	}
	result.ToolTraces = extractObjectSlice(body, agentCfg.Response.ToolTracesPath)
	result.RetrievedChunks = extractObjectSlice(body, agentCfg.Response.RetrievedChunksPath)

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		result.Error = fmt.Sprintf("agent returned status %d", resp.StatusCode)
	}

	return result
}

// CallAll sends all generated cases to the agent concurrently and returns
// completed cases in the same order as the input slice.
// maxConcurrency controls how many in-flight requests are allowed at once.
// A value of 0 defaults to 10.
func CallAll(
	ctx context.Context,
	cases []GeneratedCase,
	agentCfg config.AgentConfig,
	runID string,
	maxConcurrency int,
	progress func(done, total int), // optional; called after each completion
) []CompletedCase {
	if maxConcurrency <= 0 {
		maxConcurrency = 10
	}

	results := make([]CompletedCase, len(cases))
	sem := make(chan struct{}, maxConcurrency)

	var wg sync.WaitGroup
	var doneCount int64

	for i, cas := range cases {
		wg.Add(1)
		go func(idx int, c GeneratedCase) {
			defer wg.Done()

			select {
			case sem <- struct{}{}:
			case <-ctx.Done():
				results[idx] = CompletedCase{
					GeneratedCase: c,
					Error:         ctx.Err().Error(),
				}
				done := int(atomic.AddInt64(&doneCount, 1))
				if progress != nil {
					progress(done, len(cases))
				}
				return
			}
			defer func() { <-sem }()

			results[idx] = CallAgentForCase(ctx, c, agentCfg, runID)

			done := int(atomic.AddInt64(&doneCount, 1))
			if progress != nil {
				progress(done, len(cases))
			}
		}(i, cas)
	}

	wg.Wait()
	return results
}

// Stats returns a quick summary over a slice of completed cases.
type Stats struct {
	Total   int
	Success int
	Errors  int
}

func Summarize(cases []CompletedCase) Stats {
	s := Stats{Total: len(cases)}
	for _, c := range cases {
		if c.Error != "" {
			s.Errors++
		} else {
			s.Success++
		}
	}
	return s
}

func buildRequest(cas GeneratedCase, agentCfg config.AgentConfig, runID string) ([]byte, string, string, map[string]string, int, error) {
	endpoint, method, headers, timeoutSeconds, err := resolveTransport(agentCfg)
	if err != nil {
		return nil, "", "", nil, 0, err
	}

	payload := requestPayload(cas, runID, agentCfg)
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, "", "", nil, 0, fmt.Errorf("failed to marshal agent request: %w", err)
	}

	return body, endpoint, method, headers, timeoutSeconds, nil
}

func requestPayload(cas GeneratedCase, runID string, agentCfg config.AgentConfig) map[string]any {
	if strings.EqualFold(agentCfg.Type, "openai_compatible") {
		payload := map[string]any{
			"model": agentCfg.Model,
			"messages": []map[string]any{{
				"role":    "user",
				"content": cas.Query,
			}},
			"run_id":   runID,
			"case_id":  cas.CaseID,
			"test_id":  cas.TestID,
			"suite_id": cas.SuiteID,
		}
		if len(cas.Meta) > 0 {
			payload["metadata"] = cas.Meta
		}
		return payload
	}

	payload := map[string]any{
		"query":    cas.Query,
		"run_id":   runID,
		"case_id":  cas.CaseID,
		"test_id":  cas.TestID,
		"suite_id": cas.SuiteID,
	}
	if len(cas.Meta) > 0 {
		payload["metadata"] = cas.Meta
	}
	return payload
}

func resolveTransport(agentCfg config.AgentConfig) (endpoint string, method string, headers map[string]string, timeoutSeconds int, err error) {
	method = agentCfg.Method
	if method == "" {
		method = http.MethodPost
	}
	if agentCfg.TimeoutSeconds <= 0 {
		agentCfg.TimeoutSeconds = 45
	}
	timeoutSeconds = agentCfg.TimeoutSeconds

	headers = make(map[string]string, len(agentCfg.Headers)+1)
	for key, value := range agentCfg.Headers {
		headers[key] = value
	}

	switch strings.ToLower(agentCfg.Type) {
	case "openai_compatible":
		if agentCfg.BaseURL == "" {
			return "", "", nil, 0, fmt.Errorf("agent.base_url is required")
		}
		if agentCfg.Model == "" {
			return "", "", nil, 0, fmt.Errorf("agent.model is required")
		}
		endpoint = strings.TrimRight(agentCfg.BaseURL, "/") + "/chat/completions"
		headers["Authorization"] = fmt.Sprintf("Bearer %s", strings.TrimSpace(agentCfg.APIKey))
		headers["Content-Type"] = "application/json"
	case "custom_http", "":
		if agentCfg.Endpoint == "" {
			return "", "", nil, 0, fmt.Errorf("agent.endpoint is required")
		}
		endpoint = agentCfg.Endpoint
	default:
		return "", "", nil, 0, fmt.Errorf("unsupported agent type: %s", agentCfg.Type)
	}

	return endpoint, method, headers, timeoutSeconds, nil
}

func extractAnswer(body []byte, agentCfg config.AgentConfig) string {
	if agentCfg.Response.AnswerPath == "" {
		return ""
	}
	if v, ok := extractPath(body, agentCfg.Response.AnswerPath); ok {
		return stringify(v)
	}
	return ""
}

func extractObjectSlice(body []byte, path string) []map[string]any {
	if path == "" {
		return nil
	}
	v, ok := extractPath(body, path)
	if !ok {
		return nil
	}

	switch typed := v.(type) {
	case []any:
		out := make([]map[string]any, 0, len(typed))
		for _, item := range typed {
			if m, ok := item.(map[string]any); ok {
				out = append(out, m)
			}
		}
		return out
	case map[string]any:
		return []map[string]any{typed}
	default:
		return nil
	}
}

func extractPath(body []byte, path string) (any, bool) {
	var root any
	if err := json.Unmarshal(body, &root); err != nil {
		return nil, false
	}
	current := root
	for _, segment := range strings.Split(path, ".") {
		field, indexes := splitSegment(segment)
		m, ok := current.(map[string]any)
		if !ok {
			return nil, false
		}
		current, ok = m[field]
		if !ok {
			return nil, false
		}
		for _, index := range indexes {
			arr, ok := current.([]any)
			if !ok || index < 0 || index >= len(arr) {
				return nil, false
			}
			current = arr[index]
		}
	}
	return current, true
}

func splitSegment(segment string) (string, []int) {
	fieldEnd := strings.Index(segment, "[")
	if fieldEnd == -1 {
		return segment, nil
	}

	field := segment[:fieldEnd]
	indexes := make([]int, 0)
	rest := segment[fieldEnd:]
	for len(rest) > 0 {
		if rest[0] != '[' {
			break
		}
		end := strings.IndexByte(rest, ']')
		if end == -1 {
			break
		}
		idx, err := strconv.Atoi(rest[1:end])
		if err != nil {
			break
		}
		indexes = append(indexes, idx)
		rest = rest[end+1:]
	}
	return field, indexes
}

func stringify(v any) string {
	switch typed := v.(type) {
	case string:
		return typed
	case fmt.Stringer:
		return typed.String()
	case float64:
		return strconv.FormatFloat(typed, 'f', -1, 64)
	case bool:
		if typed {
			return "true"
		}
		return "false"
	default:
		b, err := json.Marshal(typed)
		if err != nil {
			return fmt.Sprint(typed)
		}
		return string(b)
	}
}
