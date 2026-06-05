package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"sync"
)

// CommandResult is the result returned by the dispatcher.
type CommandResult struct {
	Prompt string
	Steps  []Step
}

// Dispatch executes real CLI commands from inside the TUI.
func Dispatch(input string) (CommandResult, bool) {
	args := parseCommand(input)
	if len(args) == 0 {
		return CommandResult{Prompt: input}, false
	}

	if args[0] == "heimdal" || args[0] == "coval" {
		args = args[1:]
	}
	if len(args) == 0 {
		args = []string{"--help"}
	}

	if args[0] == "help" || args[0] == "?" {
		args = []string{"--help"}
	}
	args = removeFlag(args, "--watch")
	if len(args) > 0 && args[0] == "init" && !hasAnyFlag(args[1:]) {
		return CommandResult{
			Prompt: input,
			Steps: []Step{
				{Bash, "heimdal init", "failed", Fail},
				{None, "Interactive init prompts are not available inside TUI.", "", Warn},
				{None, "Run in terminal, or use flags: init [--project <id>] --integration <id>", "", Info},
				{None, "If a project is already selected with `use`, --project is optional.", "", Info},
				{None, "If heimdal.yaml already exists, init updates it directly.", "", Info},
			},
		}, false
	}

	exe, err := os.Executable()
	if err != nil {
		return CommandResult{
			Prompt: input,
			Steps:  []Step{{None, fmt.Sprintf("Executable not found: %v", err), "", Fail}},
		}, false
	}

	commandLabel := "heimdal " + strings.Join(args, " ")
	steps := []Step{{Bash, commandLabel, "running", Running}}

	cmd := exec.Command(exe, args...)
	cmd.Env = os.Environ()
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err = cmd.Run()
	status := Done
	detail := "done"
	if err != nil {
		status = Fail
		detail = "failed"
	}
	steps[0] = Step{Bash, commandLabel, detail, status}

	output := strings.TrimSpace(sanitizeCommandOutput(stdout.String()))
	errOutput := strings.TrimSpace(sanitizeCommandOutput(stderr.String()))
	if output != "" {
		steps = append(steps, outputToSteps(output, Info)...)
	}
	if errOutput != "" {
		steps = append(steps, outputToSteps(errOutput, Warn)...)
		steps = append(steps, explainCommonErrors(errOutput)...)
	}
	if output == "" && errOutput == "" && err != nil {
		steps = append(steps, Step{None, err.Error(), "", Fail})
	}

	return CommandResult{Prompt: input, Steps: steps}, err == nil
}

func DispatchStream(input string, emit func([]Step)) (CommandResult, bool) {
	args := parseCommand(input)
	if len(args) == 0 {
		return CommandResult{Prompt: input}, false
	}

	if args[0] == "heimdal" || args[0] == "coval" {
		args = args[1:]
	}
	if len(args) == 0 {
		args = []string{"--help"}
	}
	if args[0] == "help" || args[0] == "?" {
		args = []string{"--help"}
	}
	args = removeFlag(args, "--watch")
	if len(args) > 0 && args[0] == "init" && !hasAnyFlag(args[1:]) {
		return CommandResult{
			Prompt: input,
			Steps: []Step{
				{Bash, "heimdal init", "failed", Fail},
				{None, "Interactive init prompts are not available inside TUI.", "", Warn},
				{None, "Run in terminal, or use flags: init [--project <id>] --integration <id>", "", Info},
				{None, "If a project is already selected with `use`, --project is optional.", "", Info},
				{None, "If heimdal.yaml already exists, init updates it directly.", "", Info},
			},
		}, false
	}

	exe, err := os.Executable()
	if err != nil {
		return CommandResult{
			Prompt: input,
			Steps:  []Step{{None, fmt.Sprintf("Executable not found: %v", err), "", Fail}},
		}, false
	}

	commandLabel := "heimdal " + strings.Join(args, " ")
	cmd := exec.Command(exe, args...)
	cmd.Env = os.Environ()

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return CommandResult{Prompt: input, Steps: []Step{{Bash, commandLabel, "failed", Fail}, {None, err.Error(), "", Fail}}}, false
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return CommandResult{Prompt: input, Steps: []Step{{Bash, commandLabel, "failed", Fail}, {None, err.Error(), "", Fail}}}, false
	}
	if err := cmd.Start(); err != nil {
		return CommandResult{Prompt: input, Steps: []Step{{Bash, commandLabel, "failed", Fail}, {None, err.Error(), "", Fail}}}, false
	}

	var mu sync.Mutex
	collected := []Step{}
	errLines := []string{}
	collect := func(steps []Step, isErr bool) {
		if len(steps) == 0 {
			return
		}
		mu.Lock()
		collected = append(collected, steps...)
		if isErr {
			for _, s := range steps {
				errLines = append(errLines, s.Command)
			}
		}
		mu.Unlock()
		if emit != nil {
			emit(steps)
		}
	}

	var wg sync.WaitGroup
	wg.Add(2)
	go scanCommandOutput(stdout, Info, false, collect, &wg)
	go scanCommandOutput(stderr, Warn, true, collect, &wg)
	err = cmd.Wait()
	wg.Wait()

	status := Done
	detail := "done"
	if err != nil {
		status = Fail
		detail = "failed"
	}
	mu.Lock()
	finalSteps := make([]Step, 0, len(collected)+4)
	finalSteps = append(finalSteps, Step{Bash, commandLabel, detail, status})
	finalSteps = append(finalSteps, collected...)
	if len(errLines) > 0 {
		finalSteps = append(finalSteps, explainCommonErrors(strings.Join(errLines, "\n"))...)
	}
	mu.Unlock()
	if len(finalSteps) == 1 && err != nil {
		finalSteps = append(finalSteps, Step{None, err.Error(), "", Fail})
	}
	return CommandResult{Prompt: input, Steps: finalSteps}, err == nil
}

func scanCommandOutput(r io.Reader, status Status, isErr bool, collect func([]Step, bool), wg *sync.WaitGroup) {
	defer wg.Done()
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 1024), 1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(sanitizeCommandOutput(scanner.Text()))
		if line == "" {
			continue
		}
		collect(outputToSteps(line, status), isErr)
	}
}

func explainCommonErrors(errText string) []Step {
	lower := strings.ToLower(errText)
	if strings.Contains(lower, "uuid_parsing") && strings.Contains(lower, "project_id") {
		return []Step{
			{None, "Hint: project_id must be a full UUID.", "", Warn},
			{None, "Use `projects` to list IDs, or pass a longer unique prefix.", "", Info},
		}
	}
	if strings.Contains(lower, `unknown command "test" for "heimdal auto"`) {
		return []Step{
			{None, "Did you mean: `test auto --test-id <id> --scenario <A|B|C>` ?", "", Info},
		}
	}
	if strings.Contains(lower, "unknown command") && strings.Contains(lower, `for "heimdal"`) {
		return []Step{
			{None, "Use `help` to see grouped commands.", "", Info},
		}
	}
	if strings.Contains(lower, "knowledge_base_id is required") {
		return []Step{
			{None, "Hint: list KB ids with `knowledge-bases --project <id>`.", "", Info},
			{None, "Then rerun with `--knowledge-base <id>`.", "", Info},
		}
	}
	return nil
}

func hasAnyFlag(args []string) bool {
	for _, a := range args {
		if strings.HasPrefix(a, "-") {
			return true
		}
	}
	return false
}

func removeFlag(args []string, flag string) []string {
	clean := make([]string, 0, len(args))
	for _, a := range args {
		if a == flag {
			continue
		}
		clean = append(clean, a)
	}
	return clean
}

func parseCommand(input string) []string {
	var fields []string
	var current strings.Builder
	var quote rune
	escaped := false

	for _, r := range strings.TrimSpace(input) {
		switch {
		case escaped:
			current.WriteRune(r)
			escaped = false
		case r == '\\':
			escaped = true
		case quote != 0:
			if r == quote {
				quote = 0
			} else {
				current.WriteRune(r)
			}
		case r == '"' || r == '\'':
			quote = r
		case r == ' ' || r == '\t' || r == '\n':
			if current.Len() > 0 {
				fields = append(fields, current.String())
				current.Reset()
			}
		default:
			current.WriteRune(r)
		}
	}

	if escaped {
		current.WriteRune('\\')
	}
	if current.Len() > 0 {
		fields = append(fields, current.String())
	}

	return fields
}

func outputToSteps(out string, status Status) []Step {
	lines := strings.Split(out, "\n")
	steps := make([]Step, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimRight(line, "\r")
		if strings.TrimSpace(line) == "" {
			continue
		}
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "• ") {
			line = strings.TrimSpace(strings.TrimPrefix(trimmed, "• "))
		}
		for _, part := range chunkByRunes(line, 88) {
			steps = append(steps, Step{None, part, "", status})
		}
	}
	return steps
}

var ansiSeqRe = regexp.MustCompile(`\x1b\[[0-9;?]*[ -/]*[@-~]`)

func sanitizeCommandOutput(out string) string {
	if out == "" {
		return ""
	}
	// Normalize carriage-return based updates (spinner/progress lines) into
	// stable, line-oriented output for the TUI renderer.
	out = strings.ReplaceAll(out, "\r\n", "\n")
	out = strings.ReplaceAll(out, "\r", "\n")
	out = ansiSeqRe.ReplaceAllString(out, "")

	lines := strings.Split(out, "\n")
	clean := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimRight(line, " \t")
		if strings.TrimSpace(line) == "" {
			continue
		}
		// Drop consecutive duplicates (common with spinner/progress refresh).
		if len(clean) > 0 && clean[len(clean)-1] == line {
			continue
		}
		clean = append(clean, line)
	}
	return strings.Join(clean, "\n")
}

func chunkByRunes(s string, chunkSize int) []string {
	if chunkSize <= 0 {
		return []string{s}
	}
	r := []rune(s)
	if len(r) <= chunkSize {
		return []string{s}
	}
	out := make([]string, 0, (len(r)/chunkSize)+1)
	for len(r) > chunkSize {
		n := chunkSize
		// Prefer wrapping on the last whitespace to avoid broken words.
		for i := chunkSize - 1; i >= 0; i-- {
			if r[i] == ' ' || r[i] == '\t' {
				n = i
				break
			}
		}
		// If no reasonable whitespace split was found, hard-wrap.
		if n <= chunkSize/3 {
			n = chunkSize
		}
		part := strings.TrimSpace(string(r[:n]))
		if part != "" {
			out = append(out, part)
		}
		r = r[n:]
		for len(r) > 0 && (r[0] == ' ' || r[0] == '\t') {
			r = r[1:]
		}
	}
	if len(r) > 0 {
		last := strings.TrimSpace(string(r))
		if last != "" {
			out = append(out, last)
		}
	}
	return out
}
