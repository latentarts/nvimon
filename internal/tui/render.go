package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/prods/nvimon/internal/history"
	"github.com/prods/nvimon/internal/model"
)

const cardBorder = 2 // rounded border adds 2 chars (1 each side)

func (m Model) render() string {
	header := m.renderHeader()
	selectorWidth := hostSelectorWidth()
	summaryWidth := max(30, m.width-selectorWidth-1)
	topRow := lipgloss.JoinHorizontal(
		lipgloss.Top,
		m.renderHostSelector(),
		m.renderSummary(summaryWidth),
	)
	gpus := m.renderGPUCards()
	processes := ""
	if m.showProcesses {
		processes = m.renderProcesses()
	}
	footer := m.renderFooter()

	parts := []string{topRow, gpus}
	if m.showProcesses {
		parts = append(parts, processes)
	}
	body := lipgloss.JoinVertical(lipgloss.Left, parts...)
	body = padToHeight(body, max(0, m.height-lipgloss.Height(header)-lipgloss.Height(footer)))

	view := lipgloss.JoinVertical(lipgloss.Left, header, body, footer)
	if m.showHelp {
		overlay := styles.helpBox.Render(strings.Join([]string{
			"tap/click host rows to select scope",
			"/ filter processes",
			"enter apply filter",
			"esc clear/exit filter",
			"p pause/resume refresh",
			"r refresh now",
			"g cycle aggregate bar mode",
			"? toggle help",
		}, "\n"))
		view = lipgloss.Place(max(20, m.width), max(1, m.height), lipgloss.Center, lipgloss.Center, overlay)
	}
	if m.showWarnings {
		view = lipgloss.Place(max(20, m.width), max(1, m.height), lipgloss.Center, lipgloss.Center, m.renderWarningsDialog())
	}

	return view
}

func (m Model) renderHeader() string {
	title := styles.title.Render("NVIMON")
	state := "running"
	if m.paused {
		state = "paused"
	}

	line := lipgloss.JoinHorizontal(
		lipgloss.Center,
		title,
		styles.muted.Render(fmt.Sprintf(" hosts:%d", len(m.hosts))),
		styles.muted.Render(fmt.Sprintf(" gpus:%d", len(m.allGPUs()))),
		styles.muted.Render(fmt.Sprintf(" refresh:%s", m.refreshInterval)),
		styles.muted.Render(fmt.Sprintf(" state:%s", state)),
	)

	return styles.header.Width(max(20, m.width)).Render(line)
}

func (m Model) renderHostSelector() string {
	lines := []string{styles.label.Render("Hosts")}
	lines = append(lines, hostSelectorRow("0", "all", m.selectedIndex == -1))
	for i, host := range m.hosts {
		status := hostStatus(host, time.Now())
		label := fmt.Sprintf("%d", i+1)
		row := fmt.Sprintf("%-11s %s", truncate(host.name, 11), status)
		lines = append(lines, hostSelectorRow(label, row, i == m.selectedIndex))
	}
	return styles.panel.Width(hostSelectorWidth()).Render(strings.Join(lines, "\n"))
}

func (m Model) renderSummary(width int) string {
	if len(m.hosts) == 0 {
		return styles.panel.Width(max(30, width)).Render(strings.Join([]string{
			styles.label.Render("Fleet Summary"),
			"No hosts configured.",
		}, "\n"))
	}

	title := styles.label.Render("Fleet Summary")
	panelContent := max(28, width-4) // panel border (2) + padding (2)
	cardWidth := 34
	gap := 2
	cols := max(1, (panelContent+gap)/(cardWidth+cardBorder+gap))
	if cols > len(m.hosts) {
		cols = len(m.hosts)
	}
	if cols > 0 {
		cardWidth = max(30, (panelContent-cols*cardBorder-gap*(cols-1))/cols)
	}

	rows := make([]string, 0, (len(m.hosts)+cols-1)/cols)
	for i := 0; i < len(m.hosts); i += cols {
		end := min(i+cols, len(m.hosts))
		rowCards := make([]string, 0, (end-i)*2)
		for j, host := range m.hosts[i:end] {
			rowCards = append(rowCards, m.renderSummaryCard(host, cardWidth))
			if j < (end-i-1) {
				rowCards = append(rowCards, "  ")
			}
		}
		rows = append(rows, lipgloss.JoinHorizontal(lipgloss.Top, rowCards...))
	}

	lines := []string{title}
	lines = append(lines, rows...)
	return styles.panel.Width(max(30, width)).Render(strings.Join(lines, "\n"))
}

func (m Model) renderSummaryCard(host hostState, width int) string {
	status := hostStatus(host, time.Now())
	backend := fallbackString(host.snapshot.GPUBackend, "unknown")
	noData := host.snapshot.Timestamp.IsZero()

	ramPct := memoryPercent(host.snapshot)

	var gpuPct, vramPct, pwrPct, pwrUsedMetric model.MetricValue
	if !noData {
		var gpuTotal, gpuCount float64
		var vramUsed, vramTotal uint64
		var pwrUsed, pwrLimit float64
		var pwrKnown bool
		for _, g := range host.snapshot.GPUs {
			if g.GPUUtilPct.IsKnown() {
				gpuTotal += g.GPUUtilPct.Value
				gpuCount++
			}
			vramUsed += g.MemoryUsedBytes
			vramTotal += g.MemoryTotalBytes
			if g.PowerW.IsKnown() {
				pwrUsed += g.PowerW.Value
				pwrKnown = true
			}
			if g.PowerLimitW.IsKnown() {
				pwrLimit += g.PowerLimitW.Value
			}
		}
		if gpuCount > 0 {
			gpuPct = model.NewMetricValue(gpuTotal / gpuCount)
		}
		if vramTotal > 0 {
			vramPct = model.NewMetricValue(float64(vramUsed) / float64(vramTotal) * 100)
		}
		if pwrLimit > 0 {
			pwrPct = model.NewMetricValue(pwrUsed / pwrLimit * 100)
		}
		if pwrKnown {
			pwrUsedMetric = model.NewMetricValue(pwrUsed)
		}
	}

	contentWidth := width - 2
	gaugeBarWidth := max(4, contentWidth-12)

	rowStyle := lipgloss.NewStyle().Width(contentWidth)

	headerLine := rowStyle.Render(fmt.Sprintf("[%s] %s %s %s",
		styles.value.Render(status),
		styles.tableHead.Render(truncate(host.name, width-22)),
		styles.muted.Render("("+truncate(backend, 6)+")"),
		styles.muted.Render(summaryAge(host)),
	))

	lastLine := rowStyle.Render(
		styles.label.Render("LAT ") + styles.value.Render(truncate(hostLatency(host.lastLatency), 8)) +
			"  " + styles.label.Render("WRN ") + styles.value.Render(fmt.Sprintf("%d", warningCount(host))))

	lines := []string{
		headerLine,
		rowStyle.Render(styles.label.Render("CPU  ") + gauge(host.snapshot.CPUUsedPct, gaugeBarWidth) + " " + styles.value.Render(model.FormatPercent(host.snapshot.CPUUsedPct))),
		rowStyle.Render(styles.label.Render("RAM  ") + gauge(ramPct, gaugeBarWidth) + " " + styles.value.Render(model.FormatPercent(ramPct))),
		rowStyle.Render(styles.label.Render("GPU  ") + gauge(gpuPct, gaugeBarWidth) + " " + styles.value.Render(model.FormatPercent(gpuPct))),
		rowStyle.Render(styles.label.Render("VRAM ") + gauge(vramPct, gaugeBarWidth) + " " + styles.value.Render(model.FormatPercent(vramPct))),
		rowStyle.Render(styles.label.Render("PWR  ") + gauge(pwrPct, gaugeBarWidth) + " " + styles.value.Render(metricCompact(pwrUsedMetric)+"W")),
		lastLine,
	}

	return styles.gpuCard.Width(width).Render(strings.Join(lines, "\n"))
}

func (m Model) renderGPUCards() string {
	all := m.allGPUs()
	if len(all) == 0 {
		return styles.panel.Width(max(30, m.width)).Render("No NVIDIA GPUs detected.")
	}

	cardWidth := 34
	gap := 2
	cols := max(1, (m.width+gap)/(cardWidth+cardBorder+gap))
	if cols > len(all) {
		cols = len(all)
	}
	if cols > 0 {
		cardWidth = max(30, (m.width-cols*cardBorder-gap*(cols-1))/cols)
	}

	rows := make([]string, 0, (len(all)+cols-1)/cols)
	for i := 0; i < len(all); i += cols {
		end := min(i+cols, len(all))
		rowCards := make([]string, 0, (end-i)*2)
		for j, item := range all[i:end] {
			rowCards = append(rowCards, m.renderGPUCard(item, cardWidth))
			if j < (end-i-1) {
				rowCards = append(rowCards, "  ")
			}
		}
		rows = append(rows, lipgloss.JoinHorizontal(lipgloss.Top, rowCards...))
	}

	return lipgloss.JoinVertical(lipgloss.Left, rows...)
}

func (m Model) renderGPUCard(item hostGPU, width int) string {
	gpu := item.gpu
	key := gpu.Key(item.host.snapshot.HostID)
	contentWidth := max(10, width-2)

	memPct := gpu.MemoryUsedPct()
	lines := []string{
		styles.label.Render(fmt.Sprintf("%s GPU %d", truncate(item.host.name, 10), gpu.Index)),
		styles.value.Render(fmt.Sprintf("%s (%s)", truncate(gpu.Name, contentWidth-7), fallbackString(gpu.PState, "n/a"))),
		styles.label.Render("GPU ") + gauge(gpu.GPUUtilPct, contentWidth-12) + " " + styles.value.Render(model.FormatPercent(gpu.GPUUtilPct)),
		styles.label.Render("MEM ") + gauge(memPct, contentWidth-12) + " " + styles.value.Render(model.FormatPercent(memPct)),
		styles.label.Render("TMP ") + styles.value.Render(metricCompact(gpu.TemperatureC)+"C") + styles.label.Render(" FAN ") + styles.value.Render(metricCompact(gpu.FanPct)) + styles.label.Render(" PWR ") + styles.value.Render(metricCompact(gpu.PowerW)+"W"),
		styles.label.Render("GPU ") + sparklineWithStyle(m.history.Series(key+"/gpu"), contentWidth-10, 100, styles.sparkGPU) + " " + styles.value.Render(model.FormatPercent(gpu.GPUUtilPct)),
		styles.label.Render("MEM ") + sparklineWithStyle(m.history.Series(key+"/mem"), contentWidth-10, 100, styles.sparkMem) + " " + styles.value.Render(model.FormatPercent(memPct)),
		styles.label.Render("PWR ") + sparklineWithStyle(m.history.Series(key+"/power"), contentWidth-10, maxMetric(gpu.PowerLimitW, 300), styles.sparkPower) + " " + styles.value.Render(metricCompact(gpu.PowerW)+"W"),
		styles.label.Render("TMP ") + sparklineWithStyle(m.history.Series(key+"/temp"), contentWidth-10, 100, styles.sparkTemp) + " " + styles.value.Render(metricCompact(gpu.TemperatureC)+"C"),
	}

	return styles.gpuCard.Width(width).Render(strings.Join(lines, "\n"))
}

func (m Model) renderProcesses() string {
	lines := []string{styles.label.Render("GPU Processes")}
	processes := m.filteredProcesses()
	scopeLabel := "all servers"
	if m.selectedIndex >= 0 && m.selectedIndex < len(m.hosts) {
		scopeLabel = m.hosts[m.selectedIndex].name
	}
	lines = append(lines, styles.label.Render("scope: ")+styles.value.Render(scopeLabel))
	if m.processFilter != "" {
		lines = append(lines, styles.label.Render("filter: ")+styles.value.Render(m.processFilter))
	}
	if len(processes) == 0 {
		lines = append(lines, "No matching GPU compute processes.")
		return styles.panel.Width(max(30, m.width)).Render(strings.Join(lines, "\n"))
	}

	lines = append(lines, styles.tableHead.Render(fmt.Sprintf("%-10s %-7s %-10s %-4s %-9s %-8s %s", "SERVER", "PID", "USER", "GPU", "VRAM", "UTIL", "COMMAND")))
	for _, row := range processes {
		lines = append(lines, styles.value.Render(fmt.Sprintf(
			"%-10s %-7d %-10s %-4d %-9s %-8s %s",
			truncate(row.host.name, 10),
			row.process.PID,
			truncate(row.process.User, 10),
			row.process.GPUIndex,
			truncate(model.FormatBytes(row.process.UsedGPUMemoryBytes), 9),
			truncate(metricCompact(row.process.SMUtilPct), 8),
			truncate(row.process.Command, max(12, m.width-60)),
		)))
	}

	return styles.panel.Width(max(40, m.width)).Render(strings.Join(lines, "\n"))
}

func (m Model) renderFooter() string {
	message := "0 all  1-9 server  j/k cycle  click/tap host  / filter  w warnings  x toggle-proc  esc clear  p pause  r refresh  g aggregate  ? help"
	if m.lastError != "" {
		message += "  error: " + m.lastError
	}
	return styles.footer.Width(max(20, m.width)).Render(message)
}

func (m Model) renderWarningsDialog() string {
	lines := []string{styles.tableHead.Render("Warnings")}
	for _, host := range m.scopedHosts() {
		hostWarnings := collectedWarnings(host)
		if len(hostWarnings) == 0 {
			continue
		}
		lines = append(lines, styles.label.Render(host.name))
		for _, warning := range hostWarnings {
			lines = append(lines, styles.value.Width(max(28, min(m.width-10, 80))).Render("- "+warning))
		}
	}
	if len(lines) == 1 {
		lines = append(lines, styles.value.Render("No warnings for the current scope."))
	}
	lines = append(lines, styles.muted.Render("esc or w to close"))
	return styles.helpBox.Width(max(32, min(m.width-8, 84))).Render(strings.Join(lines, "\n"))
}

func gauge(metric model.MetricValue, width int) string {
	if !metric.IsKnown() {
		return styles.muted.Render(strings.Repeat("·", width))
	}
	filled := int((metric.Value / 100.0) * float64(width))
	if filled < 0 {
		filled = 0
	}
	if filled > width {
		filled = width
	}
	return styles.barFilled.Render(strings.Repeat("█", filled)) + styles.barEmpty.Render(strings.Repeat("░", width-filled))
}

func sparkline(points []history.Point, width int, maxValue float64) string {
	return sparklineWithStyle(points, width, maxValue, styles.sparkGPU)
}

func sparklineWithStyle(points []history.Point, width int, maxValue float64, style lipgloss.Style) string {
	if len(points) == 0 {
		return styles.muted.Render(strings.Repeat("·", width))
	}
	if len(points) > width {
		points = points[len(points)-width:]
	}
	glyphs := []rune("▁▂▃▄▅▆▇█")
	builder := strings.Builder{}
	padding := width - len(points)
	for i := 0; i < padding; i++ {
		builder.WriteRune('·')
	}
	if maxValue <= 0 {
		maxValue = 100
	}
	for _, point := range points {
		value := point.Value
		if value < 0 {
			value = 0
		}
		if value > maxValue {
			value = maxValue
		}
		idx := int((value / maxValue) * float64(len(glyphs)-1))
		builder.WriteRune(glyphs[idx])
	}
	return style.Render(builder.String())
}

func metricCompact(v model.MetricValue) string {
	if !v.IsKnown() {
		return "n/a"
	}
	return fmt.Sprintf("%.0f", v.Value)
}

func aggregateSummary(gpus []model.GPUSnapshot, mode aggregateMode) (string, string) {
	switch mode {
	case aggregateMemory:
		var used uint64
		var total uint64
		for _, gpu := range gpus {
			used += gpu.MemoryUsedBytes
			total += gpu.MemoryTotalBytes
		}
		if total == 0 {
			return "memory", "n/a"
		}
		pct := (float64(used) / float64(total)) * 100
		return "memory", fmt.Sprintf("%.0f%%", pct)
	case aggregatePower:
		var used float64
		var limit float64
		for _, gpu := range gpus {
			if gpu.PowerW.IsKnown() {
				used += gpu.PowerW.Value
			}
			if gpu.PowerLimitW.IsKnown() {
				limit += gpu.PowerLimitW.Value
			}
		}
		if limit > 0 {
			return "power", fmt.Sprintf("%.0f/%.0fW", used, limit)
		}
		if used > 0 {
			return "power", fmt.Sprintf("%.0fW", used)
		}
		return "power", "n/a"
	default:
		var total float64
		var count float64
		for _, gpu := range gpus {
			if gpu.GPUUtilPct.IsKnown() {
				total += gpu.GPUUtilPct.Value
				count++
			}
		}
		if count == 0 {
			return "compute", "n/a"
		}
		return "compute", fmt.Sprintf("%.0f%% avg", total/count)
	}
}

func memoryPercent(snapshot model.HostSnapshot) model.MetricValue {
	if snapshot.RAMTotalBytes == 0 {
		return model.UnknownMetric()
	}
	return model.NewMetricValue((float64(snapshot.RAMUsedBytes) / float64(snapshot.RAMTotalBytes)) * 100)
}

func maxMetric(v model.MetricValue, fallback float64) float64 {
	if v.IsKnown() && v.Value > 0 {
		return v.Value
	}
	return fallback
}

func formatUptime(seconds uint64) string {
	if seconds == 0 {
		return "n/a"
	}
	days := seconds / 86400
	hours := (seconds % 86400) / 3600
	minutes := (seconds % 3600) / 60
	if days > 0 {
		return fmt.Sprintf("%dd %02dh %02dm", days, hours, minutes)
	}
	return fmt.Sprintf("%02dh %02dm", hours, minutes)
}

func truncate(s string, width int) string {
	if width <= 0 || len(s) <= width {
		return s
	}
	if width <= 3 {
		return s[:width]
	}
	return s[:width-3] + "..."
}

func fallbackString(v, fallback string) string {
	if strings.TrimSpace(v) == "" {
		return fallback
	}
	return v
}

func shortAge(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	switch {
	case d < time.Second:
		return "0s"
	case d < time.Minute:
		return fmt.Sprintf("%ds", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	default:
		return fmt.Sprintf("%dh", int(d.Hours()))
	}
}

func hostLatency(d time.Duration) string {
	if d <= 0 {
		return "n/a"
	}
	return d.Round(time.Millisecond).String()
}

func summaryAge(host hostState) string {
	if !host.lastSuccessful.IsZero() {
		return shortAge(time.Since(host.lastSuccessful))
	}
	if !host.snapshot.Timestamp.IsZero() {
		return shortAge(time.Since(host.snapshot.Timestamp))
	}
	return "n/a"
}

func hostStatus(host hostState, now time.Time) string {
	switch {
	case host.lastError != "":
		return "err"
	case host.snapshot.Timestamp.IsZero():
		return "wait"
	case host.snapshot.StaleAt(now):
		return "stale"
	default:
		return "ok"
	}
}

func latestWarning(host hostState) string {
	if host.lastError != "" {
		return host.lastError
	}
	if len(host.snapshot.CollectorErrors) == 0 {
		return ""
	}
	return host.snapshot.CollectorErrors[0].Message
}

func warningCount(host hostState) int {
	count := len(host.snapshot.CollectorErrors)
	if host.lastError != "" {
		count++
	}
	return count
}

func collectedWarnings(host hostState) []string {
	warnings := make([]string, 0, len(host.snapshot.CollectorErrors)+1)
	if host.lastError != "" {
		warnings = append(warnings, host.lastError)
	}
	for _, collectorErr := range host.snapshot.CollectorErrors {
		if collectorErr.Message == "" {
			continue
		}
		warnings = append(warnings, collectorErr.Message)
	}
	return warnings
}

func chip(label string, active bool) string {
	base := lipgloss.NewStyle().
		Padding(0, 0).
		MarginRight(1)
	if active {
		return base.Background(lipgloss.Color("#5B7A6B")).Foreground(lipgloss.Color("#FFF8E1")).Bold(true).Render(label)
	}
	return base.Foreground(lipgloss.Color("#AAB8AA")).Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("#4F6D5D")).Render(label)
}

func hostSelectorRow(key, text string, active bool) string {
	row := fmt.Sprintf("%s %-14s", key, text)
	if active {
		return styles.label.Render(row)
	}
	return styles.value.Render(row)
}

func hostSelectorWidth() int {
	return 18
}

func padToHeight(s string, height int) string {
	if height <= 0 {
		return s
	}
	current := lipgloss.Height(s)
	if current >= height {
		return s
	}
	return s + strings.Repeat("\n", height-current)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func clampHeight(s string, height int) string {
	lines := strings.Split(s, "\n")
	if len(lines) <= height {
		return s
	}
	return strings.Join(lines[:height], "\n")
}
