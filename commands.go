package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
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
