package output

import "github.com/ai-la/cli/internal/runner"

// TerminalHooks implements runner.Hooks using Printer.
// The CLI creates one of these and passes it to runner.RunAutoTests.
type TerminalHooks struct {
	p *Printer
}

func NewTerminalHooks() *TerminalHooks {
	return &TerminalHooks{p: New()}
}

func (h *TerminalHooks) OnStepStart(label string)   { h.p.OnStepStart(label) }
func (h *TerminalHooks) OnStepDone(label string)    { h.p.OnStepDone(label) }
func (h *TerminalHooks) OnProgress(done, total int) { h.p.OnProgress(done, total) }
func (h *TerminalHooks) OnWarn(msg string)          { h.p.OnWarn(msg) }

// Compile-time check: TerminalHooks must satisfy runner.Hooks.
var _ runner.Hooks = (*TerminalHooks)(nil)
