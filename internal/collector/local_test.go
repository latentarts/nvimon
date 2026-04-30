package collector

import (
	"context"
	"testing"
	"time"

	"github.com/prods/nvimon/internal/model"
)

type stubHostCollector struct {
	snap hostSnapshot
	err  error
}

func (s stubHostCollector) Collect(context.Context) (hostSnapshot, error) {
	return s.snap, s.err
}

type stubGPUCollector struct {
	gpus      []model.GPUSnapshot
	processes []model.GPUProcess
	errs      []model.CollectorError
	err       error
}

func (s stubGPUCollector) Collect(context.Context) (string, []model.GPUSnapshot, []model.GPUProcess, []model.CollectorError, error) {
	return "stub", s.gpus, s.processes, s.errs, s.err
}

func TestLocalCollectorAggregatesSnapshots(t *testing.T) {
	c := &LocalCollector{
		TimedCollector: NewTimedCollector("host-a", "gpu-a", time.Second),
		host: stubHostCollector{
			snap: hostSnapshot{
				UptimeSeconds: 5,
				CPUUsedPct:    model.NewMetricValue(50),
				RAMUsedBytes:  1,
				RAMTotalBytes: 2,
				LoadAvg1:      model.NewMetricValue(1),
				LoadAvg5:      model.NewMetricValue(2),
				LoadAvg15:     model.NewMetricValue(3),
			},
		},
		gpu: stubGPUCollector{
			gpus: []model.GPUSnapshot{{Index: 0, UUID: "GPU-0"}},
			processes: []model.GPUProcess{{
				PID:      123,
				GPUUUID:  "GPU-0",
				GPUIndex: 0,
			}},
		},
	}
	c.Now = func() time.Time { return time.Unix(10, 0) }

	snapshot, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("collect: %v", err)
	}

	if snapshot.HostID != "host-a" || snapshot.GPUCount() != 1 || len(snapshot.GPUProcesses) != 1 {
		t.Fatalf("unexpected snapshot: %+v", snapshot)
	}
	if snapshot.GPUBackend != "stub" {
		t.Fatalf("gpu backend = %q, want stub", snapshot.GPUBackend)
	}
}
