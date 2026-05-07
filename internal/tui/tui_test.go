package tui

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/prods/nvimon/internal/collector"
	"github.com/prods/nvimon/internal/history"
	"github.com/prods/nvimon/internal/model"
)

func TestViewRendersDashboard(t *testing.T) {
	sample := collector.NewSampleCollector("host-a", "gpu-a", time.Second)
	snapshot, err := sample.Collect(context.Background())
	if err != nil {
		t.Fatalf("collect sample snapshot: %v", err)
	}

	m := New([]hostSource{
		localSource{name: "host-a", collector: sample},
		localSource{name: "host-b", collector: sample},
	}, time.Second, 16)
	m.width = 120
	m.height = 40
	m.hosts[0].snapshot = snapshot
	m.hosts[0].lastSuccessful = snapshot.Timestamp
	m.recordHistory(snapshot)

	view := m.View()
	if !strings.Contains(view, "NVIMON") {
		t.Fatalf("view missing title: %q", view)
	}

	if !strings.Contains(view, "GPU Processes") {
		t.Fatalf("view missing processes section: %q", view)
	}

	if !strings.Contains(view, "Fleet Summary") {
		t.Fatalf("view missing fleet summary: %q", view)
	}

	if !strings.Contains(view, "VRAM") {
		t.Fatalf("view missing vram summary bar: %q", view)
	}

	if !strings.Contains(view, "sample") {
		t.Fatalf("view missing gpu backend label: %q", view)
	}

	if !strings.Contains(view, "host-a") {
		t.Fatalf("view missing originating server label: %q", view)
	}

	if !strings.Contains(view, "SERVER") {
		t.Fatalf("view missing merged process server column: %q", view)
	}
}

func TestAggregateSummaryAndSparklineFormatting(t *testing.T) {
	gpus := []model.GPUSnapshot{
		{
			GPUUtilPct:       model.NewMetricValue(10),
			MemoryUsedBytes:  4,
			MemoryTotalBytes: 8,
			PowerW:           model.NewMetricValue(100),
			PowerLimitW:      model.NewMetricValue(200),
		},
		{
			GPUUtilPct:       model.NewMetricValue(20),
			MemoryUsedBytes:  2,
			MemoryTotalBytes: 8,
			PowerW:           model.NewMetricValue(50),
			PowerLimitW:      model.NewMetricValue(100),
		},
	}

	label, summary := aggregateSummary(gpus, aggregateCompute)
	if label != "compute" || summary != "15% avg" {
		t.Fatalf("compute summary = %q %q", label, summary)
	}

	label, summary = aggregateSummary(gpus, aggregateMemory)
	if label != "memory" || summary != "38%" {
		t.Fatalf("memory summary = %q %q", label, summary)
	}

	label, summary = aggregateSummary(gpus, aggregatePower)
	if label != "power" || summary != "150/300W" {
		t.Fatalf("power summary = %q %q", label, summary)
	}

	line := sparkline([]history.Point{
		{Value: 0},
		{Value: 50},
		{Value: 100},
	}, 3, 100)

	if strings.Contains(line, "?") {
		t.Fatalf("sparkline contains question marks: %q", line)
	}

}
func TestSegmentedGaugePreservesFilledWidth(t *testing.T) {
	line := segmentedGauge(model.NewMetricValue(50), 10, []barSegment{
		{value: 3, style: lipgloss.NewStyle().Foreground(lipgloss.Color("#FFB000"))},
		{value: 2, style: lipgloss.NewStyle().Foreground(lipgloss.Color("#58A6FF"))},
	})

	if strings.Count(line, "█") != 5 {
		t.Fatalf("filled cells = %d, want 5 in %q", strings.Count(line, "█"), line)
	}
	if strings.Count(line, "░") != 5 {
		t.Fatalf("empty cells = %d, want 5 in %q", strings.Count(line, "░"), line)
	}
}

func TestProcessVizViewShowsProcessDots(t *testing.T) {
	sample := collector.NewSampleCollector("host-a", "gpu-a", time.Second)
	snapshot, err := sample.Collect(context.Background())
	if err != nil {
		t.Fatalf("collect sample snapshot: %v", err)
	}

	m := New([]hostSource{
		localSource{name: "host-a", collector: sample},
	}, time.Second, 16)
	m.width = 120
	m.height = 40
	m.showProcessViz = true
	m.hosts[0].snapshot = snapshot
	m.hosts[0].lastSuccessful = snapshot.Timestamp
	m.recordHistory(snapshot)

	view := m.View()
	if strings.Count(view, "●") < len(snapshot.GPUProcesses) {
		t.Fatalf("expected process dots for visible processes: %q", view)
	}
}

func TestGPUCardsExpandWhenProcessPanelHidden(t *testing.T) {
	sample := collector.NewSampleCollector("host-a", "gpu-a", time.Second)
	snapshot, err := sample.Collect(context.Background())
	if err != nil {
		t.Fatalf("collect sample snapshot: %v", err)
	}

	m := New([]hostSource{
		localSource{name: "host-a", collector: sample},
	}, time.Second, 16)
	m.width = 120
	m.height = 48
	m.showProcesses = false
	m.hosts[0].snapshot = snapshot
	m.hosts[0].lastSuccessful = snapshot.Timestamp
	m.recordHistory(snapshot)

	height := lipgloss.Height(m.renderGPUCards())
	if height <= gpuCardContentLines+cardBorder {
		t.Fatalf("gpu cards height = %d, want > %d when process panel hidden", height, gpuCardContentLines+cardBorder)
	}
}

func TestRenderProcessesUsesViewport(t *testing.T) {
	m := New([]hostSource{
		stubHostSource{name: "host-a"},
	}, time.Second, 8)
	m.width = 120
	m.height = 12
	m.processScroll = 2
	m.hosts[0].snapshot = model.HostSnapshot{
		HostID:   "host-a",
		Hostname: "host-a",
		GPUs: []model.GPUSnapshot{
			{Index: 0, UUID: "GPU-0"},
		},
		GPUProcesses: []model.GPUProcess{
			{PID: 1, GPUIndex: 0, Command: "cmd-1"},
			{PID: 2, GPUIndex: 0, Command: "cmd-2"},
			{PID: 3, GPUIndex: 0, Command: "cmd-3"},
			{PID: 4, GPUIndex: 0, Command: "cmd-4"},
			{PID: 5, GPUIndex: 0, Command: "cmd-5"},
		},
	}

	view := m.renderProcesses()
	if strings.Contains(view, "cmd-1") || strings.Contains(view, "cmd-2") {
		t.Fatalf("expected top rows to be clipped from viewport: %q", view)
	}
	if !strings.Contains(view, "cmd-3") || !strings.Contains(view, "cmd-5") {
		t.Fatalf("expected scrolled rows to be visible: %q", view)
	}
	if !strings.Contains(view, "showing 3-5 of 5") {
		t.Fatalf("expected viewport status line: %q", view)
	}
}

func TestRenderSummaryCardShowsPowerInWatts(t *testing.T) {
	m := New([]hostSource{stubHostSource{name: "host-a"}}, time.Second, 8)
	card := m.renderSummaryCard(hostState{
		name: "host-a",
		snapshot: model.HostSnapshot{
			HostID:       "host-a",
			Hostname:     "host-a",
			GPUBackend:   "sample",
			Timestamp:    time.Unix(100, 0).UTC(),
			CPUUsedPct:   model.NewMetricValue(10),
			RAMUsedBytes: 8,
			RAMTotalBytes: 16,
			GPUs: []model.GPUSnapshot{
				{
					PowerW:      model.NewMetricValue(120),
					PowerLimitW: model.NewMetricValue(200),
				},
				{
					PowerW:      model.NewMetricValue(80),
					PowerLimitW: model.NewMetricValue(200),
				},
			},
		},
	}, 34)

	if !strings.Contains(card, "200W") {
		t.Fatalf("summary card missing power watts value: %q", card)
	}
	if strings.Contains(card, "50%") {
		t.Fatalf("summary card still shows power percent: %q", card)
	}
}
