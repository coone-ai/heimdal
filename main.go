package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/ai-la/cli/cmd"
	"github.com/ai-la/cli/internal/auth"
	tea "github.com/charmbracelet/bubbletea"
)

// ─── Model ───────────────────────────────────────────────────────────────────

type model struct {
	input         string
	version       string
	prompt        string
	steps         []Step
	width         int
	height        int
	running       bool
	authStatus    string
	orgStatus     string
	projectStatus string
	placeholder   placeholderState
	loggedIn      bool
	history       []string
	historyIdx    int
	scrollOffset  int
}

func initialModel() model {
	loggedIn := isLoggedIn()
	return model{
		version:       "0.0.1",
		prompt:        "Commands",
		steps:         welcomeSteps(loggedIn),
		width:         86,
		authStatus:    currentAuthStatus(),
		orgStatus:     currentOrgStatus(),
		projectStatus: currentProjectStatus(),
		loggedIn:      loggedIn,
		placeholder: placeholderState{
			commands: placeholderCommands(loggedIn),
		},
		history:      []string{},
		historyIdx:   0,
		scrollOffset: 0,
	}
}

// ─── Bubble Tea interface ─────────────────────────────────────────────────────

func (m model) Init() tea.Cmd {
	return tickPlaceholder()
}

type commandDoneMsg struct {
	result CommandResult
	ok     bool
}

type placeholderTickMsg struct{}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case placeholderTickMsg:
		if m.input == "" && !m.running {
			m.placeholder.advance()
		}
		return m, tickPlaceholder()

	case tea.WindowSizeMsg:
		m.width = msg.Width - 2
		m.height = msg.Height
		if m.width > 110 {
			m.width = 110
		}
		if m.width < 66 {
			m.width = 66
		}
		m.clampScroll()

	case commandDoneMsg:
		m.running = false
		m.prompt = "Commands"
		m.authStatus = currentAuthStatus()
		m.orgStatus = currentOrgStatus()
		m.projectStatus = currentProjectStatus()
		m.loggedIn = isLoggedIn()
		m.placeholder.commands = placeholderCommands(m.loggedIn)
		m.steps = postCommandSteps(msg.result.Steps, msg.ok, m.loggedIn)
		m.scrollToBottom()
		return m, tea.ClearScreen

	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			return m, tea.Quit

		case tea.KeyEnter:
			if m.running {
				return m, nil
			}
			cmdText := strings.TrimSpace(m.input)
			m.input = ""
			if cmdText == "" {
				break
			}
			m.pushHistory(cmdText)
			if cmdText == "exit" || cmdText == "quit" {
				return m, tea.Quit
			}
			if cmdText == "clear" {
				m.prompt = "Commands"
				m.steps = []Step{}
				m.scrollOffset = 0
				return m, tea.ClearScreen
			}
			if cmdText == "back" || cmdText == "home" {
				m.prompt = "Commands"
				m.steps = welcomeSteps(m.loggedIn)
				m.scrollOffset = 0
				return m, tea.ClearScreen
			}
			if cmdText == "?" {
				m.prompt = "Commands"
				m.steps = welcomeSteps(m.loggedIn)
				m.scrollOffset = 0
				return m, tea.ClearScreen
			}

			m.running = true
			m.prompt = "Commands"
			m.steps = []Step{{Bash, "heimdal " + cmdText, "running", Running}}
			m.scrollOffset = 0
			return m, func() tea.Msg {
				result, ok := Dispatch(cmdText)
				return commandDoneMsg{result: result, ok: ok}
			}

		case tea.KeyUp:
			if m.running {
				return m, nil
			}
			m.historyPrev()
			return m, nil

		case tea.KeyDown:
			if m.running {
				return m, nil
			}
			m.historyNext()
			return m, nil

		case tea.KeyPgUp:
			if m.running {
				return m, nil
			}
			m.scrollBy(-m.pageSize())
			return m, nil

		case tea.KeyPgDown:
			if m.running {
				return m, nil
			}
			m.scrollBy(m.pageSize())
			return m, nil

		case tea.KeyBackspace, tea.KeyDelete:
			if m.running {
				return m, nil
			}
			if len(m.input) > 0 {
				runes := []rune(m.input)
				m.input = string(runes[:len(runes)-1])
			}

		default:
			if m.running {
				return m, nil
			}
			if msg.Type == tea.KeyRunes {
				m.input += string(msg.Runes)
			} else if msg.Type == tea.KeySpace {
				m.input += " "
			}
		}

	case tea.MouseMsg:
		if !mouseScrollEnabled() {
			return m, nil
		}
		if m.running {
			return m, nil
		}
		switch msg.Button {
		case tea.MouseButtonWheelUp:
			m.scrollBy(-3)
		case tea.MouseButtonWheelDown:
			m.scrollBy(3)
		}
		return m, nil
	}

	return m, nil
}

func (m model) View() string {
	input := m.input
	if m.running {
		input = "running..."
	} else if input == "" {
		input = m.placeholder.current()
	}
	pane := m.currentPane(input)
	view := "\n" + pane.Render() + "\n"
	return padToHeight(view, m.height)
}

func (m model) currentPane(input string) ClaudePane {
	return ClaudePane{
		Version:       m.version,
		AuthStatus:    m.authStatus,
		OrgStatus:     m.orgStatus,
		ProjectStatus: m.projectStatus,
		Prompt:        m.prompt,
		Steps:         m.steps,
		InputValue:    input,
		InputIsHint:   !m.running && m.input == "",
		Width:         m.width,
		Height:        m.height,
		ScrollOffset:  m.scrollOffset,
	}
}

func (m *model) pushHistory(cmd string) {
	cmd = strings.TrimSpace(cmd)
	if cmd == "" {
		return
	}
	if n := len(m.history); n > 0 && m.history[n-1] == cmd {
		m.historyIdx = len(m.history)
		return
	}
	m.history = append(m.history, cmd)
	m.historyIdx = len(m.history)
}

func (m *model) historyPrev() {
	if len(m.history) == 0 {
		return
	}
	if m.historyIdx > 0 {
		m.historyIdx--
	}
	if m.historyIdx >= 0 && m.historyIdx < len(m.history) {
		m.input = m.history[m.historyIdx]
	}
}

func (m *model) historyNext() {
	if len(m.history) == 0 {
		return
	}
	if m.historyIdx < len(m.history)-1 {
		m.historyIdx++
		m.input = m.history[m.historyIdx]
		return
	}
	m.historyIdx = len(m.history)
	m.input = ""
}

func (m *model) pageSize() int {
	if m.height <= 0 {
		return 8
	}
	ps := m.height / 3
	if ps < 3 {
		return 3
	}
	return ps
}

func (m *model) scrollBy(delta int) {
	if delta == 0 {
		return
	}
	m.scrollOffset += delta
	m.clampScroll()
}

func (m *model) clampScroll() {
	if m.scrollOffset < 0 {
		m.scrollOffset = 0
	}
	max := m.currentPane(m.input).MaxScrollOffset()
	if m.scrollOffset > max {
		m.scrollOffset = max
	}
}

func (m *model) scrollToBottom() {
	m.scrollOffset = m.currentPane(m.input).MaxScrollOffset()
}

func padToHeight(view string, height int) string {
	if height <= 0 {
		return view
	}
	lines := strings.Count(view, "\n") + 1
	if lines >= height {
		return view
	}
	return view + strings.Repeat("\n", height-lines)
}

type placeholderState struct {
	commands []string
	cmdIdx   int
	charIdx  int
	pause    int
}

func (p *placeholderState) current() string {
	if len(p.commands) == 0 {
		return ""
	}
	cmd := p.commands[p.cmdIdx%len(p.commands)]
	if p.charIdx < 0 {
		p.charIdx = 0
	}
	if p.charIdx > len([]rune(cmd)) {
		p.charIdx = len([]rune(cmd))
	}
	return string([]rune(cmd)[:p.charIdx])
}

func (p *placeholderState) advance() {
	if len(p.commands) == 0 {
		return
	}
	cmd := p.commands[p.cmdIdx%len(p.commands)]
	r := []rune(cmd)
	if p.charIdx < len(r) {
		p.charIdx++
		p.pause = 0
		return
	}
	if p.pause < 10 {
		p.pause++
		return
	}
	p.cmdIdx = (p.cmdIdx + 1) % len(p.commands)
	p.charIdx = 0
	p.pause = 0
}

func tickPlaceholder() tea.Cmd {
	return tea.Tick(90*time.Millisecond, func(time.Time) tea.Msg {
		return placeholderTickMsg{}
	})
}

func currentAuthStatus() string {
	ts, err := auth.LoadToken()
	if err != nil || ts == nil || strings.TrimSpace(ts.Email) == "" {
		return "Not logged in"
	}
	return "Logged in: " + strings.TrimSpace(ts.Email)
}

func welcomeSteps(loggedIn bool) []Step {
	steps := []Step{
		{None, "Project Setup", "", Info},
		{None, "init [--project <id>] [--integration <id>]", "", Info},
		{None, "config validate --file ./heimdal.yaml", "", Info},
		{None, "projects", "", Info},
		{None, "integrations help", "", Info},
		{None, "knowledge-bases help", "", Info},
		{None, "", "", Info},
		{None, "Test Execution", "", Info},
		{None, "auto runs [--project <id>] [--test-id <id>]", "", Info},
		{None, "test auto --test-id AT-01 --scenario A", "", Info},
		{None, "test auto help", "", Info},
		{None, "auto results <test-run-id>", "", Info},
	}
	if loggedIn {
		steps = append(steps,
			Step{None, "auto datasets --project <id> --test-id <id>", "", Info},
		)
	}
	steps = append(steps,
		Step{None, "", "", Info},
		Step{None, "Session", "", Info},
		Step{None, "auth login", "", Info},
		Step{None, "auth logout", "", Info},
		Step{None, "org list", "", Info},
		Step{None, "org use <org-id>", "", Info},
		Step{None, "use <project-id>", "", Info},
	)
	if !loggedIn {
		steps = append(steps, Step{None, "", "", Info}, Step{None, "Tip: run `auth login` first.", "", Info})
	}
	return steps
}

func placeholderCommands(loggedIn bool) []string {
	if !loggedIn {
		return []string{
			"auth login",
			"org list",
			"org use <org-id>",
			"init [--project <id>] --integration <id>",
			"integrations help",
			"knowledge-bases help",
			"use <project-id>",
			"test auto --test-id AT-01 --scenario A",
		}
	}
	return []string{
		"org current",
		"use <project-id>",
		"init --integration <id>",
		"projects",
		"integrations help",
		"knowledge-bases help",
		"config validate --file ./heimdal.yaml",
		"test auto --test-id AT-01 --scenario A",
		"auto runs",
		"projects",
		"integrations",
		"auto runs --project <id>",
		"auto datasets --project <id> --test-id <id>",
		"auto results <test-run-id>",
	}
}

func postCommandSteps(base []Step, ok bool, loggedIn bool) []Step {
	home := welcomeSteps(loggedIn)
	steps := make([]Step, 0, len(base)+4+len(home))
	steps = append(steps, home...)
	steps = append(steps, Step{None, "", "", Info})
	steps = append(steps, base...)
	if len(base) > 0 {
		steps = append(steps, Step{None, "", "", Info})
	}
	return steps
}

func isLoggedIn() bool {
	ts, err := auth.LoadToken()
	return err == nil && ts != nil && strings.TrimSpace(ts.Token) != ""
}

func currentOrgStatus() string {
	ts, err := auth.LoadToken()
	if err != nil || ts == nil || strings.TrimSpace(ts.ActiveOrgID) == "" {
		return "Org: not selected"
	}
	return "Org: " + compactID(strings.TrimSpace(ts.ActiveOrgID), 16, 7, 6)
}

func currentProjectStatus() string {
	ts, err := auth.LoadToken()
	if err != nil || ts == nil || strings.TrimSpace(ts.ActiveProjectID) == "" {
		return "Project: not selected"
	}
	return "Project: " + compactID(strings.TrimSpace(ts.ActiveProjectID), 16, 7, 6)
}

func compactID(v string, total, head, tail int) string {
	v = strings.TrimSpace(v)
	if v == "" {
		return v
	}
	if total <= 0 || head <= 0 || tail <= 0 {
		return v
	}
	if len(v) <= total {
		return v
	}
	if head+3+tail != total {
		return v
	}
	return v[:head] + "..." + v[len(v)-tail:]
}

func main() {
	if shouldLaunchTUI(os.Args[1:]) {
		if hasDevFlag(os.Args[1:]) {
			_ = os.Setenv("HEIMDAL_DEV", "1")
		}
		opts := []tea.ProgramOption{tea.WithAltScreen()}
		if mouseScrollEnabled() {
			opts = append(opts, tea.WithMouseCellMotion())
		}
		p := tea.NewProgram(initialModel(), opts...)
		if _, err := p.Run(); err != nil {
			fmt.Println("Error:", err)
			os.Exit(1)
		}
		return
	}

	cmd.Execute()
}

func shouldLaunchTUI(args []string) bool {
	if len(args) == 0 {
		return true
	}
	if len(args) == 1 && args[0] == "tui" {
		return true
	}
	for _, a := range args {
		if a == "-h" || a == "--help" {
			return false
		}
		if !strings.HasPrefix(a, "-") {
			return false
		}
	}
	return true
}

func hasDevFlag(args []string) bool {
	for _, a := range args {
		if a == "--dev" || a == "--dev=true" {
			return true
		}
	}
	return false
}

func mouseScrollEnabled() bool {
	v := strings.TrimSpace(strings.ToLower(os.Getenv("HEIMDAL_TUI_MOUSE_SCROLL")))
	if v == "" {
		return true
	}
	return v == "1" || v == "true" || v == "yes" || v == "on"
}
