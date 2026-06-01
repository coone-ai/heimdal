package output

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// ─── Helper functions (using exported colors from steps.go) ──────────────────

func fg(c lipgloss.Color) lipgloss.Style {
	return lipgloss.NewStyle().Foreground(c)
}

func bold(c lipgloss.Color) lipgloss.Style {
	return lipgloss.NewStyle().Foreground(c).Bold(true)
}

// ─── Hexagon logo (yellow/white) ────────────────────────────────────────────

func logoLines() []string {
	white := bold(ColWhite)
	yellow := bold(ColOrange)
	shadow := fg(ColDimGray)

	return []string{
		"        " + white.Render("▗▄▄▄▖") + yellow.Render("▗▄▄▄▖"),
		"      " + white.Render("▐▌") + shadow.Render("▝▖") + yellow.Render("▗▞") + yellow.Render("▐"),
		"    " + white.Render("▐▌") + shadow.Render(" ▐") + yellow.Render("▐▐"),
		"    " + white.Render("▐▌") + shadow.Render(" ▐") + yellow.Render("▐▌"),
		"      " + white.Render("▝▚") + shadow.Render("▗▘") + yellow.Render("▝▞") + yellow.Render("▘"),
		"        " + white.Render("▝▄▄▄▘") + yellow.Render("▝▄▄▄▘"),
	}
}

// ─── ANSI-safe border + title ───────────────────────────────────────────────
// Dış kutu: ┌─ title ─┐ şeklinde, ANSI-safe

func boxWithTitle(lines []string, title string, totalWidth int) string {
	bdr := fg(ColOrange)
	innerW := totalWidth - 2 // iki │ karakteri

	// Üst kenarlık: ┌─ Claude Code v2.1.19 ──────┐
	var topLine string
	if title == "" {
		topLine = bdr.Render("┌" + strings.Repeat("─", innerW) + "┐")
	} else {
		styledTitle := bold(ColOrange).Render(title)
		prefix := "─ "
		suffix := " "
		used := lipgloss.Width(prefix + title + suffix)
		filler := innerW - used
		if filler < 0 {
			filler = 0
		}
		topLine = bdr.Render("┌"+prefix) +
			styledTitle +
			bdr.Render(suffix+strings.Repeat("─", filler)+"┐")
	}

	// Orta satırlar: │ content │
	var midLines []string
	for _, line := range lines {
		vw := lipgloss.Width(line)
		pad := innerW - 2 - vw
		if pad < 0 {
			pad = 0
		}
		midLines = append(midLines,
			bdr.Render("│")+" "+line+strings.Repeat(" ", pad)+" "+bdr.Render("│"))
	}

	// Alt kenarlık: └──────────┘
	bottomLine := bdr.Render("└" + strings.Repeat("─", innerW) + "┘")

	all := []string{topLine}
	all = append(all, midLines...)
	all = append(all, bottomLine)
	return strings.Join(all, "\n")
}

// ─── Tool tipi ──────────────────────────────────────────────────────────────

type Tool string

const (
	ToolBash  Tool = "Bash"
	ToolEdit  Tool = "Edit"
	ToolRead  Tool = "Read"
	ToolWrite Tool = "Write"
	ToolNone  Tool = ""
)

func styledTool(t Tool) string {
	switch t {
	case ToolBash:
		return bold(ColOrange).Render("Bash")
	case ToolEdit:
		return bold(ColYellow).Render("Edit")
	case ToolRead:
		return bold(ColCyan).Render("Read")
	case ToolWrite:
		return bold(ColYellow).Render("Write")
	}
	return ""
}

// ─── PaneStep (bir adımın durumu) ───────────────────────────────────────────

type PaneStepStatus int

const (
	PaneStepDone PaneStepStatus = iota
	PaneStepWarn
	PaneStepFail
	PaneStepRunning
	PaneStepInfo // ikon yok, gri
)

type PaneStep struct {
	Tool    Tool
	Command string
	Detail  string
	Status  PaneStepStatus
}

func (s PaneStep) Render() string {
	bullet := bold(ColOrange).Render("•")

	if s.Tool == ToolNone {
		return bullet + " " + fg(ColWhite).Render(s.Command)
	}

	cmd := fg(ColGray).Render("( " + s.Command + " )")
	line := bullet + " " + styledTool(s.Tool) + " " + cmd

	if s.Detail != "" {
		pipe := fg(ColDimGray).Render("ℒ")
		var detail string
		switch s.Status {
		case PaneStepDone:
			detail = fg(ColGreen).Render(s.Detail + " ✓")
		case PaneStepWarn:
			detail = fg(ColAmber).Render(s.Detail + " △")
		case PaneStepFail:
			detail = fg(ColRed).Render(s.Detail + " ✗")
		case PaneStepRunning:
			detail = fg(ColOrange).Render(s.Detail + " …")
		default:
			detail = fg(ColGray).Render(s.Detail)
		}
		line += "  " + pipe + " " + detail
	}
	return line
}

// ─── ClaudePane ────────────────────────────────────────────────────────────

type ClaudePane struct {
	Version string     // e.g., "2.1.19"
	Prompt  string     // User task
	Steps   []PaneStep // Workflow steps
	Input   string     // Input placeholder
	Width   int        // Total box width
}

func (p ClaudePane) Render() string {
	w := p.Width
	if w == 0 {
		w = 74
	}

	innerW := w - 4 // kenar boşluğu

	// ── Prompt satırı ────────────────────────────────────────────────────
	promptLine := bold(ColOrange).Render("> ") + fg(ColWhite).Render(p.Prompt)

	// ── Adım satırları ───────────────────────────────────────────────────
	stepLines := []string{promptLine, ""}
	for _, s := range p.Steps {
		stepLines = append(stepLines, s.Render())
	}

	// ── Logo + adımlar yan yana ──────────────────────────────────────────
	logo := logoLines()
	logoColW := lipgloss.Width(logo[0]) + 2 // +2 sağ boşluk

	var bodyLines []string
	for i, sl := range stepLines {
		logoRow := i - 2 // prompt + boşluk sonrası başlar
		if logoRow >= 0 && logoRow < len(logo) {
			cell := lipgloss.NewStyle().Width(logoColW).Render(logo[logoRow])
			bodyLines = append(bodyLines, cell+sl)
		} else {
			bodyLines = append(bodyLines, strings.Repeat(" ", logoColW)+sl)
		}
	}

	// ── Alt input kutusu ─────────────────────────────────────────────────
	inputContent := bold(ColOrange).Render("> ") + fg(ColDimGray).Render(p.Input)
	inputBox := boxWithTitle([]string{inputContent}, "", innerW)

	hint := fg(ColDimGray).Render("? for shortcuts")

	// ── Dış kutu ─────────────────────────────────────────────────────────
	var outerLines []string
	outerLines = append(outerLines, "")
	outerLines = append(outerLines, bodyLines...)
	outerLines = append(outerLines, "")
	for _, ibLine := range strings.Split(inputBox, "\n") {
		outerLines = append(outerLines, ibLine)
	}
	outerLines = append(outerLines, hint)
	outerLines = append(outerLines, "")

	return boxWithTitle(outerLines, fmt.Sprintf("Heimdal v%s", p.Version), w)
}
