module github.com/ai-la/cli

go 1.20

require (
	github.com/charmbracelet/bubbletea v0.26.1
	github.com/charmbracelet/lipgloss v0.4.0
	github.com/spf13/cobra v1.8.1
	gopkg.in/yaml.v3 v3.0.1
// NOTE: `viper` and `go-keyring` were listed previously but are not used
// by the current codebase. Keep dependencies minimal; run `go mod tidy`
// to refresh indirect requirements.
)

require (
	github.com/aymanbagabas/go-osc52/v2 v2.0.1 // indirect
	github.com/erikgeiser/coninput v0.0.0-20211004153227-1c3628e74d0f // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/lucasb-eyer/go-colorful v1.2.0 // indirect
	github.com/mattn/go-isatty v0.0.18 // indirect
	github.com/mattn/go-localereader v0.0.1 // indirect
	github.com/mattn/go-runewidth v0.0.15 // indirect
	github.com/muesli/ansi v0.0.0-20230316100256-276c6243b2f6 // indirect
	github.com/muesli/cancelreader v0.2.2 // indirect
	github.com/muesli/reflow v0.3.0 // indirect
	github.com/muesli/termenv v0.15.2 // indirect
	github.com/rivo/uniseg v0.4.6 // indirect
	github.com/spf13/pflag v1.0.5 // indirect
	golang.org/x/sync v0.7.0 // indirect
	golang.org/x/sys v0.19.0 // indirect
	golang.org/x/term v0.19.0 // indirect
	golang.org/x/text v0.3.8 // indirect
)
