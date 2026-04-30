package tui

import "github.com/charmbracelet/lipgloss"

var palette = []lipgloss.Color{
	lipgloss.Color("#FFB000"),
	lipgloss.Color("#58A6FF"),
	lipgloss.Color("#7ED957"),
	lipgloss.Color("#FF7A59"),
	lipgloss.Color("#C678DD"),
	lipgloss.Color("#4DD0E1"),
}

var styles = struct {
	title      lipgloss.Style
	header     lipgloss.Style
	panel      lipgloss.Style
	gpuCard    lipgloss.Style
	footer     lipgloss.Style
	helpBox    lipgloss.Style
	muted      lipgloss.Style
	label      lipgloss.Style
	value      lipgloss.Style
	tableHead  lipgloss.Style
	barFilled  lipgloss.Style
	barEmpty   lipgloss.Style
	sparkGPU   lipgloss.Style
	sparkMem   lipgloss.Style
	sparkPower lipgloss.Style
	sparkTemp  lipgloss.Style
}{
	title: lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FFF8E1")).
		Background(lipgloss.Color("#2A3A1D")).
		Padding(0, 0),
	header: lipgloss.NewStyle().
		Padding(0, 0),
	panel: lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("#4F6D5D")).
		Padding(0, 1),
	gpuCard: lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#5B7A6B")).
		Padding(0, 0),
	footer: lipgloss.NewStyle().
		Foreground(lipgloss.Color("#C7D7C7")).
		Padding(0, 0),
	helpBox: lipgloss.NewStyle().
		Border(lipgloss.DoubleBorder()).
		BorderForeground(lipgloss.Color("#FFB000")).
		Padding(0, 1),
	muted: lipgloss.NewStyle().
		Foreground(lipgloss.Color("#AAB8AA")),
	label: lipgloss.NewStyle().
		Foreground(lipgloss.Color("#F3F5EF")).
		Bold(true),
	value: lipgloss.NewStyle().
		Foreground(lipgloss.Color("#D7E4D7")),
	tableHead: lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FFF8E1")).
		Bold(true),
	barFilled: lipgloss.NewStyle().
		Foreground(lipgloss.Color("#7ED957")),
	barEmpty: lipgloss.NewStyle().
		Foreground(lipgloss.Color("#35513D")),
	sparkGPU: lipgloss.NewStyle().
		Foreground(lipgloss.Color("#D7E86A")),
	sparkMem: lipgloss.NewStyle().
		Foreground(lipgloss.Color("#58A6FF")),
	sparkPower: lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FFB000")),
	sparkTemp: lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FF7A59")),
}
