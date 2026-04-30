package collector

import (
	"context"
	"math"
	"time"

	"github.com/prods/nvimon/internal/model"
)

type SampleCollector struct {
	TimedCollector
}

func NewSampleCollector(hostID, hostname string, sampleInterval time.Duration) *SampleCollector {
	return &SampleCollector{
		TimedCollector: NewTimedCollector(hostID, hostname, sampleInterval),
	}
}

func (c *SampleCollector) Collect(ctx context.Context) (model.HostSnapshot, error) {
	_ = ctx

	now := c.Now().UTC()
	sec := float64(now.Unix()%60) / 60.0
	cpu := 40 + 25*math.Sin(sec*2*math.Pi)
	gpuUtil := 55 + 35*math.Sin((sec+0.15)*2*math.Pi)
	memUtil := 62 + 18*math.Sin((sec+0.35)*2*math.Pi)
	temp := 63 + 6*math.Sin((sec+0.05)*2*math.Pi)
	power := 210 + 35*math.Sin((sec+0.10)*2*math.Pi)

	return model.HostSnapshot{
		HostID:         c.HostID,
		Hostname:       c.Hostname,
		GPUBackend:     "sample",
		Timestamp:      now,
		SampleInterval: c.SampleInterval,
		UptimeSeconds:  86400,
		CPUUsedPct:     model.NewMetricValue(cpu),
		RAMUsedBytes:   48 * 1024 * 1024 * 1024,
		RAMTotalBytes:  64 * 1024 * 1024 * 1024,
		LoadAvg1:       model.NewMetricValue(4.2),
		LoadAvg5:       model.NewMetricValue(3.8),
		LoadAvg15:      model.NewMetricValue(3.1),
		GPUs: []model.GPUSnapshot{
			{
				Index:            0,
				UUID:             "GPU-sample-0",
				Name:             "NVIDIA Sample 0",
				GPUUtilPct:       model.NewMetricValue(gpuUtil),
				MemUtilPct:       model.NewMetricValue(memUtil),
				MemoryUsedBytes:  18 * 1024 * 1024 * 1024,
				MemoryTotalBytes: 24 * 1024 * 1024 * 1024,
				TemperatureC:     model.NewMetricValue(temp),
				FanPct:           model.NewMetricValue(52),
				PowerW:           model.NewMetricValue(power),
				PowerLimitW:      model.NewMetricValue(300),
				SMClockMHz:       model.NewMetricValue(1845),
				MemClockMHz:      model.NewMetricValue(9751),
				PState:           "P2",
			},
			{
				Index:            1,
				UUID:             "GPU-sample-1",
				Name:             "NVIDIA Sample 1",
				GPUUtilPct:       model.NewMetricValue(24 + 20*math.Sin((sec+0.55)*2*math.Pi)),
				MemUtilPct:       model.NewMetricValue(31 + 14*math.Sin((sec+0.25)*2*math.Pi)),
				MemoryUsedBytes:  7 * 1024 * 1024 * 1024,
				MemoryTotalBytes: 24 * 1024 * 1024 * 1024,
				TemperatureC:     model.NewMetricValue(49),
				FanPct:           model.NewMetricValue(33),
				PowerW:           model.NewMetricValue(98),
				PowerLimitW:      model.NewMetricValue(300),
				SMClockMHz:       model.NewMetricValue(1410),
				MemClockMHz:      model.NewMetricValue(8100),
				PState:           "P5",
			},
		},
		GPUProcesses: []model.GPUProcess{
			{
				PID:                4242,
				User:               "trainer",
				Command:            "python train.py",
				GPUUUID:            "GPU-sample-0",
				GPUIndex:           0,
				UsedGPUMemoryBytes: 12 * 1024 * 1024 * 1024,
				SMUtilPct:          model.NewMetricValue(88),
				ProcessRAMBytes:    11 * 1024 * 1024 * 1024,
				AgeSeconds:         7200,
			},
			{
				PID:                5252,
				User:               "serve",
				Command:            "text-generation-server",
				GPUUUID:            "GPU-sample-1",
				GPUIndex:           1,
				UsedGPUMemoryBytes: 6 * 1024 * 1024 * 1024,
				SMUtilPct:          model.NewMetricValue(28),
				ProcessRAMBytes:    2 * 1024 * 1024 * 1024,
				AgeSeconds:         1800,
			},
		},
	}, nil
}
