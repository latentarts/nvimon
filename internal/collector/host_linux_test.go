package collector

import (
	"context"
	"testing"
	"time"
)

func TestProcHostCollectorFirstSampleRetriesForCPU(t *testing.T) {
	c := &procHostCollector{
		prevTotal:   100,
		prevIdle:    50,
		initialized: true,
		readCPU: funcSequence(
			cpuSample{total: 100, idle: 50},
			cpuSample{total: 120, idle: 55},
		),
		sleep: func(context.Context, time.Duration) error { return nil },
	}

	snap, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("collect: %v", err)
	}

	if !snap.CPUUsedPct.IsKnown() {
		t.Fatalf("expected known CPU metric, got %+v", snap.CPUUsedPct)
	}

	if snap.CPUUsedPct.Value <= 0 {
		t.Fatalf("expected positive CPU usage, got %+v", snap.CPUUsedPct)
	}
}

type cpuSample struct {
	total uint64
	idle  uint64
}

func funcSequence(samples ...cpuSample) func() (uint64, uint64, error) {
	index := 0
	return func() (uint64, uint64, error) {
		if index >= len(samples) {
			last := samples[len(samples)-1]
			return last.total, last.idle, nil
		}
		sample := samples[index]
		index++
		return sample.total, sample.idle, nil
	}
}
