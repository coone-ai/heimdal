package main

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/ai-la/cli/cmd"
	"github.com/ai-la/cli/internal/auth"
	"github.com/ai-la/cli/internal/config"
	"github.com/ai-la/cli/internal/runner"
	tea "github.com/charmbracelet/bubbletea"
)

// ─── Model ───────────────────────────────────────────────────────────────────

type model struct {
	input         string
	inputCursor   int
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
	mouseCopyMode bool
	tourActive    bool
	tourIndex     int
	tourRunning   bool
	tourTyping    bool
	tourTyped     int
	tourStepIdx   int
	tourSummary   string
	activeTasks   string
	updateNotice  string
	commandEvents <-chan commandEvent
}

func initialModel() model {
	loggedIn := isLoggedIn()
	return model{
		version:       cmd.DisplayVersion(),
		prompt:        "Commands",
		steps:         welcomeSteps(loggedIn, ""),
		width:         86,
		authStatus:    currentAuthStatus(),
		orgStatus:     currentOrgStatus(),
		projectStatus: currentProjectStatus(),
		loggedIn:      loggedIn,
		placeholder: placeholderState{
			commands: placeholderCommands(loggedIn, ""),
		},
		history:      []string{},
		historyIdx:   0,
		scrollOffset: 0,
	}
}

// ─── Bubble Tea interface ─────────────────────────────────────────────────────

func (m model) Init() tea.Cmd {
	return tea.Batch(tickPlaceholder(), fetchActiveTasks(), fetchUpdateNotice())
}

type commandDoneMsg struct {
	result CommandResult
	ok     bool
}

type commandEvent struct {
	steps  []Step
	result CommandResult
	ok     bool
	done   bool
}

type commandEventMsg struct {
	event commandEvent
}

type placeholderTickMsg struct{}
type activeTasksMsg struct {
	summary string
}
type updateNoticeMsg struct {
	notice string
}
type tourTickMsg struct{}
type tourFinishMsg struct{}

type tourItem struct {
	Command    string
	Note       string
	MockOutput []string
}

var quickTour = []tourItem{
	{
		Command: "auth login",
		Note:    "Authenticate your account in browser.",
		MockOutput: []string{
			"Starting localhost callback server  ✓",
			"Generating CSRF state token  ✓",
			"Opening browser  ✓",
			"Waiting for sign-in  Token received ✓",
			"Saving token  user@coone.ai ✓",
		},
	},
	{
		Command: "org use 85e66f2c-e5c8-4128-a572-e2e747c3387f",
		Note:    "Select your working organization.",
		MockOutput: []string{
			"Active organization: 85e66f2c-e5c8-4128-a572-e2e747c3387f",
		},
	},
	{
		Command: "use 9c954b3b-bfc7-4ce9-bb67-31f5fad7eef9",
		Note:    "Set active project context.",
		MockOutput: []string{
			"Active project: 9c954b3b-bfc7-4ce9-bb67-31f5fad7eef9",
		},
	},
	{
		Command: "init --integration 46bc18c8-a52c-47f1-9675-91ac29767ffe",
		Note:    "Create/update heimdal.yaml for your integration.",
		MockOutput: []string{
			"Creating heimdal.yaml...",
			"Integration endpoint: https://api.ailab.co-one.co/v1/chat/completions",
			"Auto Run defaults: test=AT-01 scenario=A",
			"heimdal.yaml created ✓",
		},
	},
	{
		Command: "test auto --test-id AT-01 --scenario A",
		Note:    "Run your first Auto Run test.",
		MockOutput: []string{
			"Run ID: 3ffb6...  status=completed",
			"Score: 0.91  verdict=PASS",
			"Cases: 40  failed: 2",
			"Report URL: https://ailab.co-one.co/runs/3ffb6...",
		},
	},
}

const (
	tourTypeDelay   = 55 * time.Millisecond
	tourRunDelay    = 700 * time.Millisecond
	tourGapDelay    = 4 * time.Second
	tourFinishDelay = 8 * time.Second
	activeTaskDelay = 10 * time.Second
	updateDelay     = 1 * time.Hour
)

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case placeholderTickMsg:
		if m.input == "" && !m.running {
			m.placeholder.advance()
		}
		return m, tickPlaceholder()

	case activeTasksMsg:
		m.activeTasks = msg.summary
		return m, tickActiveTasks()

	case updateNoticeMsg:
		m.updateNotice = msg.notice
		m.placeholder.commands = placeholderCommands(m.loggedIn, m.updateNotice)
		if !m.running && !m.tourActive && m.prompt == "Commands" {
			m.steps = welcomeSteps(m.loggedIn, m.updateNotice)
		}
		return m, tickUpdateNotice()

	case tourTickMsg:
		if !m.tourActive {
			return m, nil
		}
		if m.tourIndex >= len(quickTour) {
			return m, tickTourFinish()
		}

		item := quickTour[m.tourIndex]
		label := "heimdal " + item.Command

		if !m.tourRunning && !m.tourTyping {
			m.tourSummary = fmt.Sprintf("Step %d/%d: %s", m.tourIndex+1, len(quickTour), item.Note)
			m.steps = append(m.steps, Step{None, fmt.Sprintf("Step %d/%d", m.tourIndex+1, len(quickTour)), "", Info})
			m.steps = append(m.steps, Step{None, tourProgressLine(m.tourIndex+1, len(quickTour)), "", Info})
			m.steps = append(m.steps, Step{Bash, "", "running", Running})
			m.tourStepIdx = len(m.steps) - 1
			m.tourTyped = 0
			m.tourTyping = true
			m.scrollToBottom()
			return m, tickTourType()
		}

		if m.tourTyping {
			fullRunes := []rune(label)
			advance := 2
			if m.tourTyped+advance > len(fullRunes) {
				advance = len(fullRunes) - m.tourTyped
			}
			m.tourTyped += advance
			if m.tourTyped < 0 {
				m.tourTyped = 0
			}
			if m.tourStepIdx >= 0 && m.tourStepIdx < len(m.steps) {
				m.steps[m.tourStepIdx] = Step{Bash, string(fullRunes[:m.tourTyped]), "running", Running}
			}
			m.scrollToBottom()
			if m.tourTyped >= len(fullRunes) {
				m.tourTyping = false
				m.tourRunning = true
				return m, tickTourRun()
			}
			return m, tickTourType()
		}

		last := len(m.steps) - 1
		if last >= 0 {
			m.steps[last] = Step{Bash, label, "done", Done}
		}
		if strings.TrimSpace(item.Note) != "" {
			m.steps = append(m.steps, Step{None, item.Note, "", Info})
		}
		for _, line := range item.MockOutput {
			if strings.TrimSpace(line) == "" {
				continue
			}
			m.steps = append(m.steps, Step{None, "  " + line, "", Info})
		}
		m.steps = append(m.steps, Step{None, "", "", Info})
		m.tourIndex++
		m.tourRunning = false
		m.tourTyping = false
		m.tourTyped = 0
		m.scrollToBottom()
		if m.tourIndex >= len(quickTour) {
			m.tourSummary = "Welcome to Co-one AI Lab. You're all set."
			m.steps = append(m.steps, Step{None, "Welcome to Co-one AI Lab. You're all set.", "", Info})
			m.scrollToBottom()
			return m, tickTourFinish()
		}
		m.tourSummary = fmt.Sprintf("Up next: %s", quickTour[m.tourIndex].Note)
		return m, tickTourGap()

	case tourFinishMsg:
		m.running = false
		m.tourActive = false
		m.tourRunning = false
		m.tourTyping = false
		m.tourTyped = 0
		m.tourStepIdx = -1
		m.tourSummary = ""
		m.tourIndex = 0
		m.prompt = "Commands"
		m.authStatus = currentAuthStatus()
		m.orgStatus = currentOrgStatus()
		m.projectStatus = currentProjectStatus()
		m.loggedIn = isLoggedIn()
		m.placeholder.commands = placeholderCommands(m.loggedIn, m.updateNotice)
		m.steps = welcomeSteps(m.loggedIn, m.updateNotice)
		m.scrollOffset = 0
		return m, tea.ClearScreen

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
		m.placeholder.commands = placeholderCommands(m.loggedIn, m.updateNotice)
		m.steps = postCommandSteps(msg.result.Steps, msg.ok, m.loggedIn, m.updateNotice)
		m.scrollToBottom()
		return m, m.afterCommandDoneCmd(msg.result.Prompt)

	case commandEventMsg:
		if msg.event.done {
			m.running = false
			m.commandEvents = nil
			m.prompt = "Commands"
			m.authStatus = currentAuthStatus()
			m.orgStatus = currentOrgStatus()
			m.projectStatus = currentProjectStatus()
			m.loggedIn = isLoggedIn()
			m.placeholder.commands = placeholderCommands(m.loggedIn, m.updateNotice)
			m.steps = postCommandSteps(msg.event.result.Steps, msg.event.ok, m.loggedIn, m.updateNotice)
			m.scrollToBottom()
			return m, m.afterCommandDoneCmd(msg.event.result.Prompt)
		}
		if len(msg.event.steps) > 0 {
			m.steps = append(m.steps, msg.event.steps...)
			m.scrollToBottom()
		}
		return m, waitCommandEvent(m.commandEvents)

	case tea.KeyMsg:
		if m.tourActive {
			switch msg.Type {
			case tea.KeyCtrlC, tea.KeyEsc:
				return m, tea.Quit
			default:
				return m, nil
			}
		}

		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			return m, tea.Quit

		case tea.KeyEnter:
			if m.running {
				return m, nil
			}
			cmdText := strings.TrimSpace(m.input)
			m.input = ""
			m.inputCursor = 0
			if cmdText == "" {
				break
			}
			m.pushHistory(cmdText)
			mouseCmd := m.commandStartMouseCmd(cmdText)
			if cmdText == "exit" || cmdText == "quit" {
				return m, tea.Quit
			}
			if cmdText == "clear" {
				m.prompt = "Commands"
				m.steps = welcomeSteps(m.loggedIn, m.updateNotice)
				m.scrollOffset = 0
				return m, tea.Batch(tea.ClearScreen, mouseCmd)
			}
			if cmdText == "back" || cmdText == "home" {
				m.prompt = "Commands"
				m.steps = welcomeSteps(m.loggedIn, m.updateNotice)
				m.scrollOffset = 0
				return m, tea.Batch(tea.ClearScreen, mouseCmd)
			}
			if cmdText == "?" {
				m.prompt = "Commands"
				m.steps = welcomeSteps(m.loggedIn, m.updateNotice)
				m.scrollOffset = 0
				return m, tea.Batch(tea.ClearScreen, mouseCmd)
			}
			if strings.EqualFold(cmdText, "open guide tour") || strings.EqualFold(cmdText, "tour") {
				m.running = true
				m.tourActive = true
				m.tourRunning = false
				m.tourTyping = false
				m.tourTyped = 0
				m.tourStepIdx = -1
				m.tourSummary = "Starting quick tour..."
				m.tourIndex = 0
				m.prompt = "TOUR MODE"
				m.steps = []Step{
					{None, "TOUR MODE", "", Info},
					{None, "", "", Info},
				}
				m.scrollOffset = 0
				return m, tea.Batch(mouseCmd, tickTourRun())
			}

			m.running = true
			m.prompt = "Commands"
			m.steps = []Step{{Bash, "heimdal " + cmdText, "running", Running}}
			m.scrollOffset = 0
			events := make(chan commandEvent, 16)
			m.commandEvents = events
			return m, tea.Batch(mouseCmd, startCommandStream(cmdText, events), waitCommandEvent(events))

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

		case tea.KeyLeft:
			if m.running {
				return m, nil
			}
			m.moveInputCursor(-1)

		case tea.KeyRight:
			if m.running {
				return m, nil
			}
			m.moveInputCursor(1)

		case tea.KeyHome:
			if m.running {
				return m, nil
			}
			m.inputCursor = 0

		case tea.KeyEnd:
			if m.running {
				return m, nil
			}
			m.inputCursor = len([]rune(m.input))

		case tea.KeyBackspace:
			if m.running {
				return m, nil
			}
			m.deleteBeforeCursor()

		case tea.KeyDelete:
			if m.running {
				return m, nil
			}
			m.deleteAtCursor()

		default:
			if m.running {
				return m, nil
			}
			if msg.Type == tea.KeyRunes {
				m.insertInput(string(msg.Runes))
			} else if msg.Type == tea.KeySpace {
				m.insertInput(" ")
			}
		}

	case tea.MouseMsg:
		if !mouseScrollEnabled() || m.mouseCopyMode {
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
	inputCursor := m.inputCursor
	if m.tourActive {
		if v := m.tourInputText(); v != "" {
			input = v
		} else {
			input = "tour mode..."
		}
		inputCursor = len([]rune(input))
	} else if m.running {
		input = "running..."
		inputCursor = len([]rune(input))
	} else if input == "" {
		input = m.placeholder.current()
		inputCursor = len([]rune(input))
	} else {
		inputCursor = clampInt(inputCursor, 0, len([]rune(input)))
	}
	pane := m.currentPane(input, inputCursor)
	view := "\n" + pane.Render() + "\n"
	return padToHeight(view, m.height)
}

func (m model) tourInputText() string {
	if !m.tourActive || m.tourIndex < 0 {
		return ""
	}
	if m.tourIndex >= len(quickTour) {
		return m.tourSummary
	}
	full := "heimdal " + quickTour[m.tourIndex].Command
	if m.tourTyping {
		r := []rune(full)
		n := m.tourTyped
		if n < 0 {
			n = 0
		}
		if n > len(r) {
			n = len(r)
		}
		return string(r[:n])
	}
	if m.tourRunning {
		return full
	}
	return m.tourSummary
}

func (m model) currentPane(input string, inputCursor int) ClaudePane {
	return ClaudePane{
		Version:       m.version,
		AuthStatus:    m.authStatus,
		OrgStatus:     m.orgStatus,
		ProjectStatus: m.projectStatus,
		TourMode:      m.tourActive,
		Prompt:        m.prompt,
		Steps:         m.steps,
		InputValue:    input,
		InputCursor:   inputCursor,
		InputIsHint:   !m.running && m.input == "",
		Width:         m.width,
		Height:        m.height,
		ScrollOffset:  m.scrollOffset,
		ActiveTasks:   m.activeTasks,
		UpdateNotice:  m.updateNotice,
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
		m.inputCursor = len([]rune(m.input))
	}
}

func (m *model) historyNext() {
	if len(m.history) == 0 {
		return
	}
	if m.historyIdx < len(m.history)-1 {
		m.historyIdx++
		m.input = m.history[m.historyIdx]
		m.inputCursor = len([]rune(m.input))
		return
	}
	m.historyIdx = len(m.history)
	m.input = ""
	m.inputCursor = 0
}

func (m *model) insertInput(value string) {
	r := []rune(m.input)
	m.inputCursor = clampInt(m.inputCursor, 0, len(r))
	insert := []rune(value)
	next := make([]rune, 0, len(r)+len(insert))
	next = append(next, r[:m.inputCursor]...)
	next = append(next, insert...)
	next = append(next, r[m.inputCursor:]...)
	m.input = string(next)
	m.inputCursor += len(insert)
}

func (m *model) deleteBeforeCursor() {
	r := []rune(m.input)
	m.inputCursor = clampInt(m.inputCursor, 0, len(r))
	if m.inputCursor == 0 {
		return
	}
	next := make([]rune, 0, len(r)-1)
	next = append(next, r[:m.inputCursor-1]...)
	next = append(next, r[m.inputCursor:]...)
	m.input = string(next)
	m.inputCursor--
}

func (m *model) deleteAtCursor() {
	r := []rune(m.input)
	m.inputCursor = clampInt(m.inputCursor, 0, len(r))
	if m.inputCursor >= len(r) {
		return
	}
	next := make([]rune, 0, len(r)-1)
	next = append(next, r[:m.inputCursor]...)
	next = append(next, r[m.inputCursor+1:]...)
	m.input = string(next)
}

func (m *model) moveInputCursor(delta int) {
	r := []rune(m.input)
	m.inputCursor = clampInt(m.inputCursor+delta, 0, len(r))
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
	max := m.currentPane(m.input, m.inputCursor).MaxScrollOffset()
	if m.scrollOffset > max {
		m.scrollOffset = max
	}
}

func (m *model) scrollToBottom() {
	m.scrollOffset = m.currentPane(m.input, m.inputCursor).MaxScrollOffset()
}

func (m *model) afterCommandDoneCmd(prompt string) tea.Cmd {
	if isLoginCommand(prompt) && mouseScrollEnabled() {
		m.mouseCopyMode = true
		return tea.Batch(tea.ClearScreen, tea.DisableMouse)
	}
	return tea.ClearScreen
}

func (m *model) enableMouseAfterCopyModeCmd() tea.Cmd {
	if !m.mouseCopyMode {
		return nil
	}
	m.mouseCopyMode = false
	if !mouseScrollEnabled() {
		return nil
	}
	return tea.EnableMouseCellMotion
}

func (m *model) commandStartMouseCmd(input string) tea.Cmd {
	if isLoginCommand(input) && mouseScrollEnabled() {
		m.mouseCopyMode = true
		return tea.DisableMouse
	}
	return m.enableMouseAfterCopyModeCmd()
}

func isLoginCommand(input string) bool {
	args := parseCommand(input)
	if len(args) == 0 {
		return false
	}
	if args[0] == "heimdal" || args[0] == "coval" {
		args = args[1:]
	}
	if len(args) == 0 {
		return false
	}
	if args[0] == "login" {
		return true
	}
	return len(args) >= 2 && args[0] == "auth" && args[1] == "login"
}

func clampInt(v, min, max int) int {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
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

func startCommandStream(input string, events chan<- commandEvent) tea.Cmd {
	return func() tea.Msg {
		go func() {
			result, ok := DispatchStream(input, func(steps []Step) {
				events <- commandEvent{steps: steps}
			})
			events <- commandEvent{result: result, ok: ok, done: true}
			close(events)
		}()
		return nil
	}
}

func waitCommandEvent(events <-chan commandEvent) tea.Cmd {
	return func() tea.Msg {
		if events == nil {
			return nil
		}
		event, ok := <-events
		if !ok {
			return nil
		}
		return commandEventMsg{event: event}
	}
}

func tickActiveTasks() tea.Cmd {
	return tea.Tick(activeTaskDelay, func(time.Time) tea.Msg {
		return fetchActiveTasks()()
	})
}

func tickUpdateNotice() tea.Cmd {
	return tea.Tick(updateDelay, func(time.Time) tea.Msg {
		return fetchUpdateNotice()()
	})
}

func fetchUpdateNotice() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		defer cancel()
		latest, err := cmd.LatestReleaseTag(ctx)
		if err != nil || !cmd.IsNewerVersion(cmd.Version, latest) {
			return updateNoticeMsg{}
		}
		return updateNoticeMsg{notice: latest}
	}
}

func fetchActiveTasks() tea.Cmd {
	return func() tea.Msg {
		summary := activeTaskSummary()
		return activeTasksMsg{summary: summary}
	}
}

func tickTourRun() tea.Cmd {
	return tea.Tick(tourRunDelay, func(time.Time) tea.Msg {
		return tourTickMsg{}
	})
}

func tickTourType() tea.Cmd {
	return tea.Tick(tourTypeDelay, func(time.Time) tea.Msg {
		return tourTickMsg{}
	})
}

func tickTourGap() tea.Cmd {
	return tea.Tick(tourGapDelay, func(time.Time) tea.Msg {
		return tourTickMsg{}
	})
}

func tickTourFinish() tea.Cmd {
	return tea.Tick(tourFinishDelay, func(time.Time) tea.Msg {
		return tourFinishMsg{}
	})
}

func tourProgressLine(current, total int) string {
	if total <= 0 {
		return ""
	}
	if current < 0 {
		current = 0
	}
	if current > total {
		current = total
	}
	const width = 12
	filled := (current * width) / total
	if filled < 0 {
		filled = 0
	}
	if filled > width {
		filled = width
	}
	return fmt.Sprintf("Progress: [%s%s] %d/%d", strings.Repeat("=", filled), strings.Repeat("-", width-filled), current, total)
}

func currentAuthStatus() string {
	ts, err := auth.LoadToken()
	if err != nil || ts == nil || strings.TrimSpace(ts.Email) == "" {
		return "Not logged in"
	}
	return "Logged in: " + strings.TrimSpace(ts.Email)
}

func activeTaskSummary() string {
	if !isLoggedIn() {
		return ""
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	client, err := runner.NewClientFromToken(&config.Config{})
	if err != nil {
		return ""
	}
	resp, err := client.ListJobs(ctx, "", "", "", 100)
	if err != nil {
		return ""
	}

	count := 0
	kindCounts := map[string]int{}
	for _, job := range resp.Jobs {
		status := strings.ToLower(strings.TrimSpace(job.Status))
		if status != "running" && status != "pending" {
			continue
		}
		count++
		label := strings.TrimSpace(job.JobType)
		if label == "" {
			label = strings.TrimSpace(job.EntityType)
		}
		if label == "" {
			label = "task"
		}
		kindCounts[label]++
	}
	if count == 0 {
		return ""
	}

	label := "active tasks"
	if count == 1 {
		label = "active task"
	}
	kinds := make([]string, 0, len(kindCounts))
	for kind := range kindCounts {
		kinds = append(kinds, kind)
	}
	sort.Strings(kinds)
	parts := make([]string, 0, len(kinds))
	for _, kind := range kinds {
		n := kindCounts[kind]
		if n > 1 {
			parts = append(parts, fmt.Sprintf("%s=%d", kind, n))
		} else {
			parts = append(parts, kind)
		}
	}
	if len(parts) == 0 {
		return fmt.Sprintf("%s: %d", label, count)
	}
	return fmt.Sprintf("%s: %d (%s)", label, count, strings.Join(parts, ", "))
}

func welcomeSteps(loggedIn bool, updateNotice string) []Step {
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
	if label := updateCommandLabel(updateNotice); label != "" {
		steps = append(steps, Step{None, label, "", Info})
	}
	if !loggedIn {
		steps = append(steps, Step{None, "", "", Info}, Step{None, "Tip: run `auth login` first.", "", Info})
	}
	return steps
}

func placeholderCommands(loggedIn bool, updateNotice string) []string {
	if !loggedIn {
		commands := []string{
			"open tour",
			"auth login",
			"org list",
			"org use <org-id>",
			"init [--project <id>] --integration <id>",
			"integrations help",
			"knowledge-bases help",
			"use <project-id>",
			"test auto --test-id AT-01 --scenario A",
		}
		if updateCommandLabel(updateNotice) != "" {
			commands = append([]string{"update"}, commands...)
		}
		return commands
	}
	commands := []string{
		"open tour",
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
	if updateCommandLabel(updateNotice) != "" {
		commands = append([]string{"update"}, commands...)
	}
	return commands
}

func updateCommandLabel(updateNotice string) string {
	latest := strings.TrimSpace(updateNotice)
	if latest == "" {
		return ""
	}
	return "update (new version " + latest + ")"
}

func postCommandSteps(base []Step, ok bool, loggedIn bool, updateNotice string) []Step {
	home := welcomeSteps(loggedIn, updateNotice)
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
