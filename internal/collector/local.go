package collector

import (
	"context"
	"os"
	"time"

	"github.com/prods/nvimon/internal/model"
)

type hostMetrics interface {
	Collect(context.Context) (hostSnapshot, error)
}

type gpuMetrics interface {
	Collect(context.Context) (string, []model.GPUSnapshot, []model.GPUProcess, []model.CollectorError, error)
}

type LocalCollector struct {
	TimedCollector
	host hostMetrics
	gpu  gpuMetrics
}

type hostSnapshot struct {
	UptimeSeconds uint64
	CPUUsedPct    model.MetricValue
	RAMUsedBytes  uint64
	RAMTotalBytes uint64
	LoadAvg1      model.MetricValue
	LoadAvg5      model.MetricValue
	LoadAvg15     model.MetricValue
}

func NewLocalCollector(sampleInterval time.Duration) *LocalCollector {
	hostname, err := os.Hostname()
	if err != nil || hostname == "" {
		hostname = "localhost"
	}

	return &LocalCollector{
		TimedCollector: NewTimedCollector(hostname, hostname, sampleInterval),
		host:           newProcHostCollector(),
		gpu:            newPreferredGPUCollector(),
	}
}

func (c *LocalCollector) Collect(ctx context.Context) (model.HostSnapshot, error) {
	start := c.Now()

	hostData, hostErr := c.host.Collect(ctx)
	gpuBackend, gpus, processes, gpuCollectorErrors, gpuErr := c.gpu.Collect(ctx)

	now := c.Now().UTC()
	if gpus == nil {
		gpus = []model.GPUSnapshot{}
	}
	if processes == nil {
		processes = []model.GPUProcess{}
	}
	snapshot := model.HostSnapshot{
		HostID:           c.HostID,
		Hostname:         c.Hostname,
		GPUBackend:       gpuBackend,
		Timestamp:        now,
		CollectorLatency: now.Sub(start),
		SampleInterval:   c.SampleInterval,
		GPUs:             gpus,
		GPUProcesses:     processes,
		CollectorErrors:  append([]model.CollectorError(nil), gpuCollectorErrors...),
	}

	if hostErr == nil {
		snapshot.UptimeSeconds = hostData.UptimeSeconds
		snapshot.CPUUsedPct = hostData.CPUUsedPct
		snapshot.RAMUsedBytes = hostData.RAMUsedBytes
		snapshot.RAMTotalBytes = hostData.RAMTotalBytes
		snapshot.LoadAvg1 = hostData.LoadAvg1
		snapshot.LoadAvg5 = hostData.LoadAvg5
		snapshot.LoadAvg15 = hostData.LoadAvg15
	} else {
		snapshot.CPUUsedPct = model.ErrorMetric()
		snapshot.LoadAvg1 = model.ErrorMetric()
		snapshot.LoadAvg5 = model.ErrorMetric()
		snapshot.LoadAvg15 = model.ErrorMetric()
		snapshot.CollectorErrors = append(snapshot.CollectorErrors, model.CollectorError{
			Component: "host",
			Message:   hostErr.Error(),
			Fatal:     false,
		})
	}

	if gpuErr != nil {
		snapshot.CollectorErrors = append(snapshot.CollectorErrors, model.CollectorError{
			Component: "gpu",
			Message:   gpuErr.Error(),
			Fatal:     false,
		})
	}

	return snapshot, nil
}
