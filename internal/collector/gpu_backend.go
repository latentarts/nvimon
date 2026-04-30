package collector

import (
	"context"

	"github.com/prods/nvimon/internal/model"
)

type preferredGPUCollector struct {
	primary  gpuMetrics
	fallback gpuMetrics
}

func newPreferredGPUCollector() gpuMetrics {
	return &preferredGPUCollector{
		primary:  newNVMLCollector(),
		fallback: newNvidiaSMICollector(),
	}
}

func (c *preferredGPUCollector) Collect(ctx context.Context) (string, []model.GPUSnapshot, []model.GPUProcess, []model.CollectorError, error) {
	backend, gpus, processes, errs, err := c.primary.Collect(ctx)
	if err == nil {
		return backend, gpus, processes, errs, nil
	}

	fallbackBackend, fallbackGPUs, fallbackProcesses, fallbackErrs, fallbackErr := c.fallback.Collect(ctx)
	if fallbackErr != nil {
		return fallbackBackend, nil, nil, append(errs, fallbackErrs...), fallbackErr
	}

	fallbackErrs = append([]model.CollectorError{{
		Component: "nvml",
		Message:   "falling back to nvidia-smi: " + err.Error(),
		Fatal:     false,
	}}, fallbackErrs...)

	return fallbackBackend, fallbackGPUs, fallbackProcesses, fallbackErrs, nil
}
