package output

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// ─── Palette (yellow/black style) ───────────────────────────────────────────

var (
	ColOrange  = lipgloss.Color("#ffce03") // primary accent (brand yellow)
	ColYellow  = lipgloss.Color("#ffce03") // secondary accent (brand yellow)
	ColCyan    = lipgloss.Color("75")      // Read tool
	ColGreen   = lipgloss.Color("82")      // success
	ColRed     = lipgloss.Color("196")     // error
	ColAmber   = lipgloss.Color("#ffce03") // warn
	ColWhite   = lipgloss.Color("255")     // primary text
	ColGray    = lipgloss.Color("250")     // secondary text
	ColDimGray = lipgloss.Color("240")     // black-ish shadow / muted text
)

// ─── Base styles ────────────────────────────────────────────────────────────

var (
	sOrange  = lipgloss.NewStyle().Foreground(ColOrange).Bold(true)
	sYellow  = lipgloss.NewStyle().Foreground(ColYellow).Bold(true)
	sGreen   = lipgloss.NewStyle().Foreground(ColGreen)
	sRed     = lipgloss.NewStyle().Foreground(ColRed)
	sWhite   = lipgloss.NewStyle().Foreground(ColWhite)
	sGray    = lipgloss.NewStyle().Foreground(ColGray)
	sDim     = lipgloss.NewStyle().Foreground(ColDimGray)
	sYellowW = lipgloss.NewStyle().Foreground(ColAmber) // warn
)

// ─── Step Status ─────────────────────────────────────────────────────────────

type StepStatus int

const (
	StepDone StepStatus = iota
	StepWarn
	StepFail
	StepRunning
	StepPlain // no icon, just text
)

// Step represents a workflow step (matches Claude Code format).
type Step struct {
	Title  string     // the action/command
	Detail string     // right-side annotation/metric
	Status StepStatus // controls icon & color
}

// renderMetric styles the detail with an icon.
func (s *Step) renderMetric() string {
	if s.Detail == "" {
		return ""
	}
	pipe := sDim.Render("ℒ")
	var detail string
	switch s.Status {
	case StepDone:
		detail = sGreen.Render(s.Detail + " ✓")
	case StepWarn:
		detail = sYellowW.Render(s.Detail + " △")
	case StepFail:
		detail = sRed.Render(s.Detail + " ✗")
	case StepRunning:
		detail = sOrange.Render(s.Detail + " …")
	default:
		detail = sGray.Render(s.Detail)
	}
	return "  " + pipe + " " + detail
}

// Render returns one formatted step line.
func (s *Step) Render() string {
	bullet := sOrange.Render("•")
	title := sWhite.Render(s.Title)
	return bullet + " " + title + s.renderMetric()
}

// ─── StepTracker ────────────────────────────────────────────────────────────

type StepTracker struct {
	steps []*Step
}

// NewStepTracker creates a tracker.
func NewStepTracker() *StepTracker {
	return &StepTracker{steps: []*Step{}}
}

// Add appends a new step.
func (st *StepTracker) Add(title string) *Step {
	step := &Step{Title: title, Status: StepPlain}
	st.steps = append(st.steps, step)
	return step
}

// Done marks step at idx as done with optional detail.
func (st *StepTracker) Done(idx int, detail string) {
	if idx >= 0 && idx < len(st.steps) {
		st.steps[idx].Status = StepDone
		st.steps[idx].Detail = detail
	}
}

// Warn marks step at idx with warning.
func (st *StepTracker) Warn(idx int, detail string) {
	if idx >= 0 && idx < len(st.steps) {
		st.steps[idx].Status = StepWarn
		st.steps[idx].Detail = detail
	}
}

// Error marks step at idx as failed.
func (st *StepTracker) Error(idx int, detail string) {
	if idx >= 0 && idx < len(st.steps) {
		st.steps[idx].Status = StepFail
		st.steps[idx].Detail = detail
	}
}

// Start marks step at idx as running.
func (st *StepTracker) Start(idx int) {
	if idx >= 0 && idx < len(st.steps) {
		st.steps[idx].Status = StepRunning
	}
}

// Render returns the formatted step list with Claude Code styling.
func (st *StepTracker) Render() string {
	if len(st.steps) == 0 {
		return ""
	}

	var lines []string
	for _, step := range st.steps {
		lines = append(lines, step.Render())
	}

	content := strings.Join(lines, "\n")

	// Outer border with yellow accent
	style := lipgloss.NewStyle().
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(ColOrange).
		Padding(1, 2)

	return style.Render(content)
}
