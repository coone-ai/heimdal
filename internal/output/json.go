package output

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/ai-la/cli/internal/runner"
)

// JSONOutput prints the final RunReport as a single JSON object to stdout.
// Used when the CLI is invoked with --json.
type JSONOutput struct{}

func NewJSONOutput() *JSONOutput { return &JSONOutput{} }

func (j *JSONOutput) Print(report *runner.RunReport) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(toJSONReport(report))
}

// jsonReport is the stable JSON schema emitted by --json.
// Kept separate from RunReport so we can version/evolve it independently.
type jsonReport struct {
	RunID            string        `json:"run_id"`
	JobID            string        `json:"job_id,omitempty"`
	ProjectID        string        `json:"project_id"`
	Version          string        `json:"version,omitempty"`
	TestsetID        string        `json:"testset_id,omitempty"`
	Suites           []string      `json:"suites"`
	TotalCases       int           `json:"total_cases"`
	Summary          *jsonSummary  `json:"summary,omitempty"`
	FailedGates      []jsonGate    `json:"failed_gates,omitempty"`
	TopFailures      []jsonFailure `json:"top_failures,omitempty"`
	ReportURL        string        `json:"report_url,omitempty"`
	ExitCode         int           `json:"exit_code"`
	Status           string        `json:"status,omitempty"`
	Verdict          string        `json:"verdict,omitempty"`
	OverallScore     float64       `json:"overall_score,omitempty"`
	BenchmarkVersion string        `json:"benchmark_version,omitempty"`
	RunVersion       string        `json:"run_version,omitempty"`
	DeltaScore       *float64      `json:"delta_score,omitempty"`
}

type jsonSummary struct {
	ReliabilityScore          float64 `json:"reliability_score"`
	GroundingScore            float64 `json:"grounding_score"`
	InstructionObedienceScore float64 `json:"instruction_obedience_score"`
	SafetyScore               float64 `json:"safety_score"`
	TotalCases                int     `json:"total_cases"`
	FailedCases               int     `json:"failed_cases"`
	Decision                  string  `json:"decision"`
}

type jsonGate struct {
	Name     string  `json:"name"`
	Required float64 `json:"required"`
	Got      float64 `json:"got"`
}

type jsonFailure struct {
	Type     string `json:"type"`
	Count    int    `json:"count"`
	Severity string `json:"severity"`
}

func toJSONReport(r *runner.RunReport) jsonReport {
	out := jsonReport{
		RunID:            r.RunID,
		JobID:            r.JobID,
		ProjectID:        r.ProjectID,
		Version:          r.Version,
		TestsetID:        r.TestsetID,
		Suites:           r.Suites,
		TotalCases:       r.TotalCases,
		ReportURL:        r.ReportURL,
		ExitCode:         r.ExitCode,
		Status:           r.Status,
		Verdict:          r.Verdict,
		OverallScore:     r.OverallScore,
		BenchmarkVersion: r.BenchmarkVersion,
		RunVersion:       r.RunVersion,
		DeltaScore:       r.DeltaScore,
	}

	if r.Summary != nil {
		out.Summary = &jsonSummary{
			ReliabilityScore:          r.Summary.ReliabilityScore,
			GroundingScore:            r.Summary.GroundingScore,
			InstructionObedienceScore: r.Summary.InstructionObedienceScore,
			SafetyScore:               r.Summary.SafetyScore,
			TotalCases:                r.Summary.TotalCases,
			FailedCases:               r.Summary.FailedCases,
			Decision:                  r.Summary.Decision,
		}
	}

	for _, g := range r.FailedGates {
		out.FailedGates = append(out.FailedGates, jsonGate{
			Name:     g.Name,
			Required: g.Required,
			Got:      g.Got,
		})
	}

	for _, f := range r.TopFailures {
		out.TopFailures = append(out.TopFailures, jsonFailure{
			Type:     f.Type,
			Count:    f.Count,
			Severity: f.Severity,
		})
	}

	return out
}

// PrintError writes an error as JSON to stderr. Used when --json is active
// and the run fails before producing a report.
func PrintJSONError(err error, exitCode int) {
	obj := map[string]any{
		"error":     err.Error(),
		"exit_code": exitCode,
	}
	b, _ := json.MarshalIndent(obj, "", "  ")
	fmt.Fprintln(os.Stderr, string(b))
}
