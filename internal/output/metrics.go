package output

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
)

// MetricLevel represents how good a metric is.
type MetricLevel int

const (
	MetricBad MetricLevel = iota
	MetricWarning
	MetricGood
)

// Metric represents a single metric with name and value.
type Metric struct {
	Name  string
	Value float64 // e.g., 0.92
	Level MetricLevel
}

// MetricColor returns the color based on the metric level (Claude Code palette).
func (m *Metric) MetricColor() lipgloss.Color {
	switch m.Level {
	case MetricGood:
		return lipgloss.Color("10") // Green ✓
	case MetricWarning:
		return lipgloss.Color("226") // Yellow
	case MetricBad:
		return lipgloss.Color("9") // Red ✗
	}
	return lipgloss.Color("8") // Gray
}

// String renders the metric with color, Claude Code style.
func (m *Metric) String() string {
	valueStr := fmt.Sprintf("%.2f", m.Value)
	styledValue := lipgloss.NewStyle().
		Foreground(m.MetricColor()).
		Bold(true).
		Render(valueStr)

	// Add checkmark or X
	symbol := ""
	switch m.Level {
	case MetricGood:
		symbol = " ✓"
	case MetricBad:
		symbol = " ✗"
	}

	if symbol != "" {
		symbol = lipgloss.NewStyle().
			Foreground(m.MetricColor()).
			Render(symbol)
	}

	return fmt.Sprintf("%-20s %s%s", m.Name, styledValue, symbol)
}

// MetricsPanel renders multiple metrics in a styled container, Claude Code style.
type MetricsPanel struct {
	Title   string
	Metrics []*Metric
}

// NewMetricsPanel creates a new metrics panel.
func NewMetricsPanel(title string) *MetricsPanel {
	return &MetricsPanel{Title: title, Metrics: []*Metric{}}
}

// AddMetric appends a metric.
func (mp *MetricsPanel) AddMetric(name string, value float64, level MetricLevel) {
	mp.Metrics = append(mp.Metrics, &Metric{
		Name:  name,
		Value: value,
		Level: level,
	})
}

// Render returns the formatted metrics panel with Claude Code styling.
func (mp *MetricsPanel) Render() string {
	if len(mp.Metrics) == 0 {
		return ""
	}

	// Title in yellow/accent
	titleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("226")).
		Bold(true)
	titleLine := titleStyle.Render(mp.Title)

	// Metrics lines with padding
	var metricLines []string
	for _, m := range mp.Metrics {
		metricLines = append(metricLines, "  "+m.String())
	}

	// Combine with title
	content := lipgloss.JoinVertical(
		lipgloss.Left,
		append([]string{titleLine}, metricLines...)...,
	)

	// Wrap in panel with normal (bold) border
	panelStyle := lipgloss.NewStyle().
		Padding(1, 2).
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("8"))

	return panelStyle.Render(content)
}

// ResolveMetricLevel determines the level based on value and threshold.
func ResolveMetricLevel(value float64, greenThreshold, yellowThreshold float64) MetricLevel {
	if value >= greenThreshold {
		return MetricGood
	}
	if value >= yellowThreshold {
		return MetricWarning
	}
	return MetricBad
}
