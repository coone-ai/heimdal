package runner

// Hooks is the interface the runner uses to report progress.
// The output package provides a concrete implementation that renders
// to the terminal. Tests can use a no-op or recording implementation.
type Hooks interface {
	// OnStepStart is called when a new phase begins (e.g. "Generating queries").
	OnStepStart(label string)

	// OnStepDone is called when the current phase completes successfully.
	OnStepDone(label string)

	// OnProgress is called periodically during a phase with current counts.
	OnProgress(done, total int)

	// OnWarn is called for non-fatal warnings.
	OnWarn(msg string)
}

// NoopHooks is a Hooks implementation that does nothing.
// Useful for tests and non-interactive usage.
type NoopHooks struct{}

func (NoopHooks) OnStepStart(string)  {}
func (NoopHooks) OnStepDone(string)   {}
func (NoopHooks) OnProgress(int, int) {}
func (NoopHooks) OnWarn(string)       {}
