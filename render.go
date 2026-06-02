package main

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// ─── Palette (yellow/black) ────────────────────────────────────────────────

var (
	colOrange = lipgloss.Color("#ffce03")
	colYellow = lipgloss.Color("#ffce03")
	colCyan   = lipgloss.Color("75")
	colGreen  = lipgloss.Color("82")
	colRed    = lipgloss.Color("196")
	colAmber  = lipgloss.Color("#ffce03")
	colWhite  = lipgloss.Color("255")
	colGray   = lipgloss.Color("250")
	colDim    = lipgloss.Color("240")
)

func fg(c lipgloss.Color) lipgloss.Style { return lipgloss.NewStyle().Foreground(c) }
func bold(c lipgloss.Color) lipgloss.Style {
	return lipgloss.NewStyle().Foreground(c).Bold(true)
}

func boxWithTitleColor(lines []string, title string, totalWidth int, borderColor lipgloss.Color) string {
	if totalWidth < 20 {
		totalWidth = 20
	}
	bdr := fg(borderColor)
	innerW := totalWidth - 2

	var topLine string
	if title == "" {
		topLine = bdr.Render("┌" + strings.Repeat("─", innerW) + "┐")
	} else {
		styledTitle := bold(borderColor).Render(title)
		prefix := "─ "
		suffix := " "
		used := lipgloss.Width(prefix + title + suffix)
		filler := innerW - used
		if filler < 0 {
			filler = 0
		}
		topLine = bdr.Render("┌"+prefix) + styledTitle +
			bdr.Render(suffix+strings.Repeat("─", filler)+"┐")
	}

	var midLines []string
	for _, line := range lines {
		vw := lipgloss.Width(line)
		pad := innerW - 2 - vw
		if pad < 0 {
			pad = 0
		}
		midLines = append(midLines, bdr.Render("│")+" "+line+strings.Repeat(" ", pad)+" "+bdr.Render("│"))
	}

	bottomLine := bdr.Render("└" + strings.Repeat("─", innerW) + "┘")
	all := []string{topLine}
	all = append(all, midLines...)
	all = append(all, bottomLine)
	return strings.Join(all, "\n")
}

// ─── Hexagon logo ───────────────────────────────────────────────────────────

// func robotLines() []string {
// 	w := bold(colWhite)
// 	y := bold(colOrange)

// 	return []string{
// 		"       " + y.Render("▀▀▙"),           // Sarı Üst (2 satır yukarıda)
// 		"         " + y.Render("▀▙"),         // Sarı Üst Eğim
// 		"  " + w.Render("▟▀▀") + "     " + y.Render("█"),    // Beyaz Üst / Sarı Sağ Yan
// 		" " + w.Render("▟▀") + "      " + y.Render("▄▛"),  // Beyaz Eğim / Sarı Alt Eğim
// 		" " + w.Render("█") + "     " + y.Render("▄▄▛"),   // Beyaz Yan / Sarı Alt
// 		" " + w.Render("▜▄"),                                 // Beyaz Sol Alt Eğim
// 		"  " + w.Render("▜▄▄"),                               // Beyaz Alt
// 	}
// }

// func robotLines() []string {
// 	// w := bold(colWhite)
// 	y := bold(colOrange)

// 	return []string{
// 		"      " + y.Render("▀▜"),
// 		"    " + "▛▀" + "  " + y.Render("▜"),
// 		"   " + "▛" + "     " + y.Render("▜"),
// 		"  " + "▛" + "     " + y.Render("▟"),
// 		"   " + "▙" + "  " + y.Render("▄▟"),
// 		"    " + "▙▄",
// 	}
// }

// func robotLines() []string {
// 	y := bold(colOrange)

// 	return []string{
// 		"      " + y.Render("▀▀▜ "),
// 		"   " + "▛▀▀" + "   " + y.Render("▜"),
// 		"  " + "▛" + "       " + y.Render("▜"),
// 		" " + "▛" + "         " + y.Render("▜"),
// 		"" + "▛" + "         " + y.Render("▟"),
// 		" " + "▙" + "       " + y.Render("▟"),
// 		"  " + "▙" + "   " + y.Render("▄▄▟"),
// 		"   " + "▙▄▄",
// 	}
// }

func robotLines() []string {
	w := bold(colWhite)
	y := bold(colOrange)

	return []string{
		"      " + y.Render("▄▄▖"),
		"   " + w.Render("▗▄▄") + "  " + y.Render("▜▖"),
		"  " + w.Render("▗▛  ") + "   " + y.Render("▜▖"),
		" " + w.Render("▗▛   ") + "   " + y.Render("▟▘"),
		" " + w.Render("▝▙   ") + "  " + y.Render("▟▘"),
		"  " + w.Render("▝▙  ") + y.Render("▀▀▘"),
		"   " + w.Render("▝▀▀"),
	}
	// return []string{
	// 	"       " + y.Render("▄▄▖"),
	// 	"   " + w.Render("▗▄▄") + "   " + y.Render("▜▖"),
	// 	"  " + w.Render("▗▛  ") + "    " + y.Render("▜▖"),
	// 	" " + w.Render("▗▛   ") + "    " + y.Render("▟▘"),
	// 	" " + w.Render("▝▙   ") + "   " + y.Render("▟▘"),
	// 	"  " + w.Render("▝▙   ") + y.Render("▀▀▘"),
	// 	"   " + w.Render("▝▀▀"),
	// }
}

// func robotLines() []string {
// 	// Sol taraf gri/beyaz (renksiz), sağ taraf turuncu konsepti
// 	y := bold(colOrange).Render

// 	return []string{
// 		"   ▄▀▀" + y("▀▀▄"),
// 		" ▄▀   " + y("   ▀▄"),
// 		" █ ◉  " + y("  ◉ █"),
// 		" █    " + y("    █"),
// 		" ▀▄ ▀▀" + y("▀▀ ▄▀"),
// 		"   ▀▄▄" + y("▄▄▀"),
// 	}
// }

// ─── ANSI-safe bordered box ──────────────────────────────────────────────────

func boxWithTitle(lines []string, title string, totalWidth int) string {
	return boxWithTitleColor(lines, title, totalWidth, colOrange)
}

// ─── Tool & Step ────────────────────────────────────────────────────────────

type Tool string

const (
	Bash  Tool = "Bash"
	Edit  Tool = "Edit"
	Read  Tool = "Read"
	Write Tool = "Write"
	None  Tool = ""
)

func styledTool(t Tool) string {
	switch t {
	case Bash:
		return bold(colOrange).Render("Bash")
	case Edit:
		return bold(colYellow).Render("Edit")
	case Read:
		return bold(colCyan).Render("Read")
	case Write:
		return bold(colYellow).Render("Write")
	}
	return ""
}

type Status int

const (
	Done Status = iota
	Warn
	Fail
	Running
	Info
)

type Step struct {
	Tool    Tool
	Command string
	Detail  string
	Status  Status
}

func (s Step) Render() string {
	if s.Tool == None && strings.TrimSpace(s.Command) == "" {
		return ""
	}
	bullet := bold(colOrange).Render("•")
	if s.Tool == None {
		trimmed := strings.TrimSpace(s.Command)
		// Allow indented helper lines to render without bullets.
		if strings.HasPrefix(s.Command, "  ") {
			return fg(colDim).Render(s.Command)
		}
		if isSectionHeading(trimmed) {
			return bullet + " " + bold(colOrange).Render(trimmed)
		}
		return bullet + " " + fg(colWhite).Render(s.Command)
	}
	cmd := fg(colGray).Render("( " + s.Command + " )")
	line := bullet + " " + styledTool(s.Tool) + " " + cmd
	if s.Detail != "" {
		pipe := fg(colDim).Render("ℒ")
		var detail string
		switch s.Status {
		case Done:
			detail = fg(colGreen).Render(s.Detail + " ✓")
		case Warn:
			detail = fg(colAmber).Render(s.Detail + " △")
		case Fail:
			detail = fg(colRed).Render(s.Detail + " ✗")
		case Running:
			detail = fg(colOrange).Render(s.Detail + " …")
		default:
			detail = fg(colGray).Render(s.Detail)
		}
		line += "  " + pipe + " " + detail
	}
	return line
}

func isSectionHeading(v string) bool {
	switch v {
	case "Project Setup", "Test Execution", "Session":
		return true
	default:
		return false
	}
}

// ─── ClaudePane renderer ─────────────────────────────────────────────────────

type ClaudePane struct {
	Version       string
	AuthStatus    string
	OrgStatus     string
	ProjectStatus string
	TourMode      bool
	Prompt        string
	Steps         []Step
	InputValue    string
	InputCursor   int
	InputIsHint   bool
	Width         int
	Height        int
	ScrollOffset  int
	ActiveTasks   string
}

func (p ClaudePane) Render() string {
	w := p.Width
	if w == 0 {
		w = 74
	}
	if w < 20 {
		w = 20
	}
	innerW := w - 4

	statusLine := fg(colDim).Render(
		"auth: " + p.AuthStatus + "    " +
			strings.ToLower(p.OrgStatus) + "    " +
			strings.ToLower(p.ProjectStatus),
	)

	bodyLines := p.renderBodyLines(statusLine, innerW-2)
	totalBodyLines := len(bodyLines)
	bodyCapacity := p.bodyCapacity()
	scrollOffset := p.ScrollOffset
	if scrollOffset < 0 {
		scrollOffset = 0
	}
	maxOffset := 0
	if totalBodyLines > bodyCapacity {
		maxOffset = totalBodyLines - bodyCapacity
	}
	if scrollOffset > maxOffset {
		scrollOffset = maxOffset
	}
	if bodyCapacity > 0 && totalBodyLines > bodyCapacity {
		bodyLines = bodyLines[scrollOffset : scrollOffset+bodyCapacity]
	}

	borderColor := colOrange
	if p.TourMode {
		borderColor = lipgloss.Color("#ffe46b")
	}
	inputCapacity := innerW - 8
	if inputCapacity < 4 {
		inputCapacity = 4
	}
	inputLine := renderInputLine(p.InputValue, p.InputCursor, inputCapacity, borderColor, p.InputIsHint)
	inputBox := boxWithTitleColor([]string{inputLine}, "", innerW, borderColor)

	var outerLines []string
	outerLines = append(outerLines, "")
	outerLines = append(outerLines, bodyLines...)
	outerLines = append(outerLines, "")
	for _, ibLine := range strings.Split(inputBox, "\n") {
		outerLines = append(outerLines, ibLine)
	}
	footerText := "clear=clear output  tour=open tour  scroll=pgup/pgdn  quit=exit"
	if totalBodyLines > bodyCapacity {
		footerText += fmt.Sprintf("  [%d/%d]", scrollOffset+1, maxOffset+1)
	}
	if strings.TrimSpace(p.ActiveTasks) != "" {
		footerText += "  " + strings.TrimSpace(p.ActiveTasks)
	}
	footer := fg(colDim).Render(footerText)
	outerLines = append(outerLines, footer)
	outerLines = append(outerLines, "")

	return boxWithTitleColor(outerLines, "heimdal "+p.Version, w, borderColor)
}

func renderInputLine(value string, cursor int, capacity int, borderColor lipgloss.Color, isHint bool) string {
	r := []rune(value)
	cursor = clampRenderInt(cursor, 0, len(r))
	if capacity < 1 {
		capacity = 1
	}

	start := 0
	if cursor >= capacity {
		start = cursor - capacity + 1
	}
	if start < 0 {
		start = 0
	}
	end := start + capacity
	if end > len(r) {
		end = len(r)
	}

	visible := r[start:end]
	visibleCursor := cursor - start
	if visibleCursor < 0 {
		visibleCursor = 0
	}
	if visibleCursor > len(visible) {
		visibleCursor = len(visible)
	}

	textStyle := fg(colWhite)
	if isHint {
		textStyle = fg(colDim)
	}
	prefix := bold(borderColor).Render("> ")
	before := textStyle.Render(string(visible[:visibleCursor]))
	after := textStyle.Render(string(visible[visibleCursor:]))
	cursorBlock := bold(borderColor).Render("█")
	return prefix + before + cursorBlock + after
}

func clampRenderInt(v, min, max int) int {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

func (p ClaudePane) renderBodyLines(statusLine string, bodyWidth int) []string {
	stepLines := []string{statusLine, ""}
	for _, s := range p.Steps {
		stepLines = append(stepLines, s.Render())
	}

	robot := robotLines()
	logoLeftPad := 0
	logoTextGap := 4
	robotColW := logoLeftPad + lipgloss.Width(robot[0]) + logoTextGap

	var bodyLines []string
	for i, sl := range stepLines {
		// Anchor the logo to the command block (after status + prompt spacer rows)
		// so it doesn't appear to "drift" when header rows change.
		robotRow := i - 5
		if robotRow >= 0 && robotRow < len(robot) {
			cell := lipgloss.NewStyle().Width(robotColW).Render(strings.Repeat(" ", logoLeftPad) + robot[robotRow])
			bodyLines = append(bodyLines, cell+sl)
		} else {
			bodyLines = append(bodyLines, strings.Repeat(" ", robotColW)+sl)
		}
	}
	return fitLines(bodyLines, bodyWidth)
}

func (p ClaudePane) bodyCapacity() int {
	if p.Height <= 0 {
		return 1000
	}
	// Rendered view adds one blank line above and below the pane in main.View().
	// Pane chrome (title/borders/input/footer/etc.) takes ~11 lines.
	capacity := p.Height - 11
	if capacity < 1 {
		return 1
	}
	return capacity
}

func (p ClaudePane) MaxScrollOffset() int {
	w := p.Width
	if w == 0 {
		w = 74
	}
	if w < 20 {
		w = 20
	}
	innerW := w - 4
	statusLine := "auth: " + p.AuthStatus + "    " + strings.ToLower(p.OrgStatus) + "    " + strings.ToLower(p.ProjectStatus)
	total := len(p.renderBodyLines(statusLine, innerW-2))
	capacity := p.bodyCapacity()
	if total <= capacity {
		return 0
	}
	return total - capacity
}

func fitLines(lines []string, maxWidth int) []string {
	if maxWidth < 1 {
		return lines
	}
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		clamped := lipgloss.NewStyle().MaxWidth(maxWidth).Render(line)
		for _, part := range strings.Split(clamped, "\n") {
			out = append(out, part)
		}
	}
	return out
}
