package tui

import (
	"context"
	"strings"
	"testing"
	"time"

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
