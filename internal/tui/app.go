package tui

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/prods/nvimon/internal/collector"
	"github.com/prods/nvimon/internal/config"
	"github.com/prods/nvimon/internal/history"
	"github.com/prods/nvimon/internal/model"
	"github.com/prods/nvimon/internal/transport/httpapi"
)

type aggregateMode int

const (
	aggregateCompute aggregateMode = iota
	aggregateMemory
	aggregatePower
)

type hostSource interface {
	Name() string
	Fetch(context.Context) (model.HostSnapshot, error)
}

type hostState struct {
	name           string
	snapshot       model.HostSnapshot
	lastSuccessful time.Time
	lastError      string
	lastLatency    time.Duration
}

type hostSnapshotResult struct {
	index    int
	snapshot model.HostSnapshot
	err      error
}

type snapshotsMsg struct {
	results []hostSnapshotResult
}

type tickMsg time.Time

type Model struct {
	sources         []hostSource
	refreshInterval time.Duration
	history         *history.Store

	width  int
	height int

	hosts []hostState

	selectedIndex int
	lastError     string
	paused        bool
	showHelp      bool
	showWarnings  bool
	showProcesses bool
	showProcessViz bool
	aggregateMode aggregateMode
	filterMode    bool
	processFilter string
}

func NewFromConfig(cfg config.Config) Model {
	sources := make([]hostSource, 0, len(cfg.Hosts))
	for _, host := range cfg.Hosts {
		switch host.Mode {
		case config.HostModeRemote:
			sources = append(sources, remoteSource{
				name: host.Name,
				client: httpapi.Client{
					BaseURL:   host.URL,
					AuthToken: host.Token,
					HTTPClient: &http.Client{
						Timeout: cfg.Timeouts.Request,
					},
				},
			})
		default:
			sources = append(sources, localSource{
				name:      host.Name,
				collector: collector.NewLocalCollector(cfg.RefreshInterval),
			})
		}
	}

	if len(sources) == 0 {
		sources = append(sources, localSource{
			name:      "localhost",
			collector: collector.NewLocalCollector(cfg.RefreshInterval),
		})
	}

	return New(sources, cfg.RefreshInterval, cfg.HistoryLength)
}

func New(sources []hostSource, refreshInterval time.Duration, historyLength int) Model {
	hosts := make([]hostState, len(sources))
	for i, source := range sources {
		hosts[i] = hostState{name: source.Name()}
	}

	return Model{
		sources:         sources,
		refreshInterval: refreshInterval,
		history:         history.NewStore(historyLength),
		hosts:           hosts,
		selectedIndex:   -1,
		showProcesses:   true,
		aggregateMode:   aggregateCompute,
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(fetchSnapshotsCmd(m.sources, m.refreshInterval), tickCmd(m.refreshInterval))
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "p":
			m.paused = !m.paused
			return m, nil
		case "r":
			if m.paused {
				return m, fetchSnapshotsCmd(m.sources, m.refreshInterval)
			}
			return m, tea.Batch(fetchSnapshotsCmd(m.sources, m.refreshInterval), tickCmd(m.refreshInterval))
		case "g":
			m.aggregateMode = (m.aggregateMode + 1) % 3
			return m, nil
		case "?", "h":
			m.showHelp = !m.showHelp
			if m.showHelp {
				m.showWarnings = false
			}
			return m, nil
		case "w":
			m.showWarnings = !m.showWarnings
			if m.showWarnings {
				m.showHelp = false
			}
			return m, nil
		case "x":
			m.showProcesses = !m.showProcesses
			if !m.showProcesses {
				m.filterMode = false
			}
			return m, nil
		case "v":
			m.showProcessViz = !m.showProcessViz
			return m, nil
		case "/":
			if m.showProcesses {
				m.filterMode = true
			}
			return m, nil
		case "esc":
			if m.showWarnings {
				m.showWarnings = false
				return m, nil
			}
			m.filterMode = false
			return m, nil
		case "j", "down":
			if len(m.hosts) > 0 {
				m.selectedIndex++
				if m.selectedIndex >= len(m.hosts) {
					m.selectedIndex = -1
				}
			}
			return m, nil
		case "k", "up":
			if len(m.hosts) > 0 {
				m.selectedIndex--
				if m.selectedIndex < -1 {
					m.selectedIndex = len(m.hosts) - 1
				}
			}
			return m, nil
		}
		if len(msg.Runes) == 1 && msg.Runes[0] >= '0' && msg.Runes[0] <= '9' {
			n := int(msg.Runes[0] - '0')
			if n == 0 {
				m.selectedIndex = -1
				return m, nil
			}
			if n-1 < len(m.hosts) {
				m.selectedIndex = n - 1
				return m, nil
			}
		}
		if m.filterMode {
			switch msg.Type {
			case tea.KeyBackspace:
				if len(m.processFilter) > 0 {
					m.processFilter = m.processFilter[:len(m.processFilter)-1]
				}
				return m, nil
			case tea.KeyEnter:
				m.filterMode = false
				return m, nil
			case tea.KeyRunes:
				m.processFilter += string(msg.Runes)
				return m, nil
			}
		}
	case tea.MouseMsg:
		if msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonLeft {
			if index, ok := m.hostSelectorHit(msg.X, msg.Y); ok {
				m.selectedIndex = index
				return m, nil
			}
		}
	case tickMsg:
		if m.paused {
			return m, tickCmd(m.refreshInterval)
		}
		return m, tea.Batch(fetchSnapshotsCmd(m.sources, m.refreshInterval), tickCmd(m.refreshInterval))
	case snapshotsMsg:
		m.lastError = ""
		for _, result := range msg.results {
			if result.index < 0 || result.index >= len(m.hosts) {
				continue
			}

			if result.err != nil {
				m.hosts[result.index].lastError = result.err.Error()
				if m.lastError == "" {
					m.lastError = fmt.Sprintf("%s: %s", m.hosts[result.index].name, result.err.Error())
				}
				continue
			}

			m.hosts[result.index].snapshot = result.snapshot
			m.hosts[result.index].lastSuccessful = result.snapshot.Timestamp
			m.hosts[result.index].lastLatency = result.snapshot.CollectorLatency
			m.hosts[result.index].lastError = ""
			m.recordHistory(result.snapshot)
		}
		return m, nil
	}

	return m, nil
}

func (m Model) View() string {
	if m.width == 0 || m.height == 0 {
		return "Loading nvimon..."
	}

	return m.render()
}

func (m Model) hostSelectorHit(x, y int) (int, bool) {
	if len(m.hosts) == 0 {
		return 0, false
	}

	headerHeight := lipgloss.Height(m.renderHeader())
	selectorWidth := lipgloss.Width(m.renderHostSelector())
	if x < 0 || y < headerHeight || x >= selectorWidth {
		return 0, false
	}

	panelY := y - headerHeight
	if panelY < 1 || panelY > len(m.hosts)+2 {
		return 0, false
	}

	switch panelY {
	case 1:
		return 0, false
	case 2:
		return -1, true
	default:
		index := panelY - 3
		if index >= 0 && index < len(m.hosts) {
			return index, true
		}
		return 0, false
	}
}

func (m *Model) selectedHost() *hostState {
	if len(m.hosts) == 0 {
		return nil
	}
	if m.selectedIndex < 0 {
		return nil
	}
	if m.selectedIndex >= len(m.hosts) {
		m.selectedIndex = len(m.hosts) - 1
	}
	return &m.hosts[m.selectedIndex]
}

func (m Model) allGPUs() []hostGPU {
	gpus := make([]hostGPU, 0)
	for _, host := range m.scopedHosts() {
		for _, gpu := range host.snapshot.GPUs {
			gpus = append(gpus, hostGPU{host: host, gpu: gpu})
		}
	}
	return gpus
}

func (m Model) filteredProcesses() []hostProcess {
	processes := make([]hostProcess, 0)
	filter := strings.ToLower(strings.TrimSpace(m.processFilter))
	for _, host := range m.scopedHosts() {
		for _, proc := range host.snapshot.GPUProcesses {
			row := hostProcess{host: host, process: proc}
			if filter == "" || row.matches(filter) {
				processes = append(processes, row)
			}
		}
	}
	return processes
}

func (m Model) scopedHosts() []hostState {
	if m.selectedIndex >= 0 && m.selectedIndex < len(m.hosts) {
		return []hostState{m.hosts[m.selectedIndex]}
	}
	return m.hosts
}

type hostGPU struct {
	host hostState
	gpu  model.GPUSnapshot
}

type hostProcess struct {
	host    hostState
	process model.GPUProcess
}

func (p hostProcess) colorKey() string {
	return fmt.Sprintf("%s/%s/%d", p.host.snapshot.HostID, p.process.GPUUUID, p.process.PID)
}

func (p hostProcess) matches(filter string) bool {
	fields := []string{
		strings.ToLower(p.host.name),
		strings.ToLower(p.process.User),
		strings.ToLower(p.process.Command),
		fmt.Sprintf("%d", p.process.PID),
		fmt.Sprintf("%d", p.process.GPUIndex),
	}
	for _, field := range fields {
		if strings.Contains(field, filter) {
			return true
		}
	}
	return false
}

func (m *Model) recordHistory(snapshot model.HostSnapshot) {
	ts := snapshot.Timestamp
	if snapshot.CPUUsedPct.IsKnown() {
		m.history.Append(snapshot.HostID+"/cpu", ts, snapshot.CPUUsedPct.Value)
	}

	for _, gpu := range snapshot.GPUs {
		key := gpu.Key(snapshot.HostID)
		if gpu.GPUUtilPct.IsKnown() {
			m.history.Append(key+"/gpu", ts, gpu.GPUUtilPct.Value)
		}
		memPct := gpu.MemoryUsedPct()
		if memPct.IsKnown() {
			m.history.Append(key+"/mem", ts, memPct.Value)
		}
		if gpu.PowerW.IsKnown() {
			m.history.Append(key+"/power", ts, gpu.PowerW.Value)
		}
		if gpu.TemperatureC.IsKnown() {
			m.history.Append(key+"/temp", ts, gpu.TemperatureC.Value)
		}
	}
}

func (m Model) processesForGPU(host hostState, gpu model.GPUSnapshot) []hostProcess {
	processes := make([]hostProcess, 0)
	for _, proc := range host.snapshot.GPUProcesses {
		if proc.GPUUUID == gpu.UUID || proc.GPUIndex == gpu.Index {
			processes = append(processes, hostProcess{host: host, process: proc})
		}
	}
	return processes
}

func (m Model) processColor(process hostProcess) lipgloss.Color {
	key := process.colorKey()
	var sum int
	for _, ch := range key {
		sum += int(ch)
	}
	return palette[sum%len(palette)]
}

func fetchSnapshotsCmd(sources []hostSource, refresh time.Duration) tea.Cmd {
	timeout := refresh
	if timeout < 2*time.Second {
		timeout = 2 * time.Second
	}

	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()

		results := make([]hostSnapshotResult, len(sources))
		var wg sync.WaitGroup
		for i, source := range sources {
			wg.Add(1)
			go func(idx int, src hostSource) {
				defer wg.Done()
				snapshot, err := src.Fetch(ctx)
				results[idx] = hostSnapshotResult{
					index:    idx,
					snapshot: snapshot,
					err:      err,
				}
			}(i, source)
		}
		wg.Wait()

		return snapshotsMsg{results: results}
	}
}

func tickCmd(refresh time.Duration) tea.Cmd {
	return tea.Tick(refresh, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func formatSince(t time.Time) string {
	if t.IsZero() {
		return "never"
	}

	d := time.Since(t).Round(time.Second)
	return fmt.Sprintf("%s ago", d)
}

type localSource struct {
	name      string
	collector collector.Collector
}

func (s localSource) Name() string {
	return s.name
}

func (s localSource) Fetch(ctx context.Context) (model.HostSnapshot, error) {
	snapshot, err := s.collector.Collect(ctx)
	if err == nil && s.name != "" {
		snapshot.HostID = s.name
	}
	return snapshot, err
}

type remoteSource struct {
	name   string
	client httpapi.Client
}

func (s remoteSource) Name() string {
	return s.name
}

func (s remoteSource) Fetch(ctx context.Context) (model.HostSnapshot, error) {
	snapshot, err := s.client.Snapshot(ctx)
	if err == nil && s.name != "" {
		snapshot.HostID = s.name
	}
	return snapshot, err
}
