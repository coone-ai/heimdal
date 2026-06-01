package output

import (
	"fmt"
	"math"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/ai-la/cli/internal/runner"
)

var (
	titleStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("230"))
	mutedStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	sectionStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("250"))
	successStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("42"))
	warnStyle     = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("214"))
	errorStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("196"))
	cyanStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("45"))
	accentStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("141"))
	keyStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("245")).Width(14)
	valueStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
	barDoneStyle  = lipgloss.NewStyle().Background(lipgloss.Color("45"))
	barTodoStyle  = lipgloss.NewStyle().Background(lipgloss.Color("236"))
	panelStyle    = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("238")).Padding(0, 1)
	footerStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("243"))
	deployStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("42"))
	denyStyle     = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("196"))
	atRiskStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("214"))
	barStartStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("45"))
)

func colorize(style lipgloss.Style, s string) string {
	if !isTTY() {
		return s
	}
	return style.Render(s)
}

func isTTY() bool {
	fi, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}

// Printer implements runner.Hooks and renders progress + final report
// to stdout.
type Printer struct {
	mu          sync.Mutex
	spinnerStop chan struct{}
	currentStep string

	progressDone  atomic.Int64
	progressTotal atomic.Int64
}

func New() *Printer { return &Printer{} }

func (p *Printer) OnStepStart(label string) {
	p.mu.Lock()
	p.currentStep = label
	p.mu.Unlock()
	if !isTTY() {
		// Non-interactive outputs (pipes, captured TUI subprocess output) should
		// stay line-oriented; spinner carriage-returns corrupt rendered logs.
		fmt.Printf("  • %s\n", label)
		return
	}
	p.startSpinner(label)
}

func (p *Printer) OnStepDone(label string) {
	p.stopSpinner()
	fmt.Printf("  %s %s\n", colorize(successStyle, "✓"), label)
}

func (p *Printer) OnProgress(done, total int) {
	p.progressDone.Store(int64(done))
	p.progressTotal.Store(int64(total))
}

func (p *Printer) OnWarn(msg string) {
	p.stopSpinner()
	fmt.Printf("  %s %s\n", colorize(warnStyle, "⚠"), msg)
}

var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

func (p *Printer) startSpinner(label string) {
	if !isTTY() {
		return
	}
	p.spinnerStop = make(chan struct{})
	go func() {
		i := 0
		for {
			select {
			case <-p.spinnerStop:
				fmt.Print("\r\033[K")
				return
			case <-time.After(80 * time.Millisecond):
				done := int(p.progressDone.Load())
				total := int(p.progressTotal.Load())
				frame := colorize(cyanStyle, spinnerFrames[i%len(spinnerFrames)])

				line := []string{"  ", frame, " ", label}
				if total > 0 {
					line = append(line, " ", progressBar(done, total, 20), " ", fmt.Sprintf("%d/%d", done, total))
				}
				fmt.Print("\r" + strings.Join(line, ""))
				i++
			}
		}
	}()
}

func (p *Printer) stopSpinner() {
	if p.spinnerStop != nil {
		close(p.spinnerStop)
		p.spinnerStop = nil
		p.progressDone.Store(0)
		p.progressTotal.Store(0)
	}
}

func progressBar(done, total, width int) string {
	if total == 0 {
		return ""
	}
	filled := int(math.Round(float64(done) / float64(total) * float64(width)))
	if filled > width {
		filled = width
	}
	if filled < 0 {
		filled = 0
	}
	left := strings.Repeat("█", filled)
	right := strings.Repeat("░", width-filled)
	return barStartStyle.Render("[") + colorize(barDoneStyle, left) + colorize(barTodoStyle, right) + barStartStyle.Render("]")
}

func PrintHeader(projectName, version, runID, testsetID string, suites []string, caseCount int) {
	lines := []string{
		titleStyle.Render("Heimdal Auto-Running Tests"),
		mutedStyle.Render(strings.Repeat("─", 40)),
		kvLine("Project", projectName),
	}
	if version != "" {
		lines = append(lines, kvLine("Version", version))
	}
	lines = append(lines, kvLine("Run ID", runID))
	if testsetID != "" {
		lines = append(lines, kvLine("Testset", testsetID))
	}
	lines = append(lines,
		kvLine("Suites", strings.Join(suites, ", ")),
		kvLine("Cases", fmt.Sprintf("%d", caseCount)),
		mutedStyle.Render(strings.Repeat("─", 40)),
		"",
	)
	fmt.Println(strings.Join(lines, "\n"))
}

func PrintReport(report *runner.RunReport) {
	fmt.Println()

	if report.Summary == nil {
		fmt.Println(errorStyle.Render("  ✗ Evaluation could not be completed — no result returned."))
		return
	}

	s := report.Summary

	// ── Metrics Panel ─────────────────────────────────────────────────────────
	metricsPanel := NewMetricsPanel("📊 Test Metrics")
	metricsPanel.AddMetric("Reliability", s.ReliabilityScore, ResolveMetricLevel(s.ReliabilityScore, 0.90, 0.80))
	metricsPanel.AddMetric("Grounding", s.GroundingScore, ResolveMetricLevel(s.GroundingScore, 0.90, 0.80))
	metricsPanel.AddMetric("Obedience", s.InstructionObedienceScore, ResolveMetricLevel(s.InstructionObedienceScore, 0.90, 0.80))
	metricsPanel.AddMetric("Safety", s.SafetyScore, ResolveMetricLevel(s.SafetyScore, 0.90, 0.80))
	fmt.Println(metricsPanel.Render())

	// ── Classic Report Panel ──────────────────────────────────────────────────
	reliabilityLabel, reliabilityIcon := scoreLabel(s.ReliabilityScore)

	panel := []string{
		fmt.Sprintf("%s %s", titleStyle.Render("Agent Reliability Score:"), titleStyle.Render(fmt.Sprintf("%.2f", s.ReliabilityScore))),
		fmt.Sprintf("%s %s", reliabilityIcon, labelStyleForScore(reliabilityLabel).Render(reliabilityLabel)),
		"",
		sectionStyle.Render("Breakdown:"),
		printScoreLine("Knowledge Grounding", s.GroundingScore),
		printScoreLine("Instruction Obedience", s.InstructionObedienceScore),
		printScoreLine("Runtime Safety", s.SafetyScore),
	}

	if len(report.FailedGates) > 0 {
		panel = append(panel, "", denyStyle.Render("Failed Gates:"))
		for _, g := range report.FailedGates {
			panel = append(panel, fmt.Sprintf("%s %s required %.2f, got %.2f", errorStyle.Render("✗"), g.Name, g.Required, g.Got))
		}
	}

	if len(report.TopFailures) > 0 {
		panel = append(panel, "", sectionStyle.Render("Top Failures:"))
		for i, f := range report.TopFailures {
			panel = append(panel, fmt.Sprintf("%s%d. %d %s found.", mutedStyle.Render(""), i+1, f.Count, formatFailureType(f.Type)))
		}
	}

	panel = append(panel, "", decisionLine(report.ExitCode, s.Decision))

	if report.ReportURL != "" {
		panel = append(panel, "", sectionStyle.Render("Report:"), accentStyle.Render(report.ReportURL))
	}

	fmt.Println(panelStyle.Width(maxLineWidth(panel...)).Render(strings.Join(panel, "\n")))
	fmt.Println()
}

func printScoreLine(label string, score float64) string {
	_, icon := scoreLabel(score)
	formatted := fmt.Sprintf("%.2f", score)
	return fmt.Sprintf("  - %-24s %s %s", label, titleStyle.Render(formatted), icon)
}

func labelStyleForScore(scoreLabel string) lipgloss.Style {
	switch scoreLabel {
	case "Passed":
		return successStyle
	case "At Risk":
		return warnStyle
	default:
		return errorStyle
	}
}

func scoreLabel(score float64) (string, string) {
	switch {
	case score >= 0.90:
		return "Passed", successStyle.Render("🟢")
	case score >= 0.80:
		return "At Risk", warnStyle.Render("🟡")
	default:
		return "Failed", errorStyle.Render("🔴")
	}
}

func decisionLine(exitCode int, backendDecision string) string {
	var label string
	switch {
	case exitCode == 0:
		label = deployStyle.Render("✓  DEPLOY")
	case exitCode == 1:
		label = denyStyle.Render("✗  DO NOT DEPLOY")
	case exitCode == 5:
		label = warnStyle.Render("⚠  INCONCLUSIVE — partial evaluation")
	default:
		switch backendDecision {
		case "passed":
			label = deployStyle.Render("✓  DEPLOY")
		case "at_risk":
			label = atRiskStyle.Render("⚠  AT RISK")
		default:
			label = denyStyle.Render("✗  DO NOT DEPLOY")
		}
	}
	return sectionStyle.Render("Decision:") + "\n  " + label
}

var failureTypeLabels = map[string]string{
	"unsupported_claim":              "unsupported claims",
	"instruction_boundary_violation": "instruction boundary violations",
	"hallucination":                  "hallucinations",
	"safety_violation":               "safety violations",
	"grounding_failure":              "grounding failures",
}

func formatFailureType(t string) string {
	if label, ok := failureTypeLabels[t]; ok {
		return label
	}
	return strings.ReplaceAll(t, "_", " ")
}

func PrintError(err error, exitCode int) {
	label := exitCodeLabel(exitCode)
	fmt.Fprintf(os.Stderr, "\n%s [%s] %v\n\n",
		errorStyle.Render("✗"),
		mutedStyle.Render(label),
		err,
	)
}

func exitCodeLabel(code int) string {
	switch code {
	case 1:
		return "THRESHOLD_FAIL"
	case 2:
		return "CONFIG_ERROR"
	case 3:
		return "AGENT_ERROR"
	case 4:
		return "HEIMDAL_API_ERROR"
	case 5:
		return "PARTIAL_EVAL"
	default:
		return "ERROR"
	}
}

func kvLine(key, value string) string {
	return fmt.Sprintf("%s  %s", keyStyle.Render(key+":"), valueStyle.Render(value))
}

func maxLineWidth(lines ...string) int {
	max := 0
	for _, line := range lines {
		for _, segment := range strings.Split(line, "\n") {
			if l := lipgloss.Width(segment); l > max {
				max = l
			}
		}
	}
	if max < 60 {
		return 60
	}
	return max + 2
}
