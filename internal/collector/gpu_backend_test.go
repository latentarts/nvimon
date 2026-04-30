package collector

import (
	"context"
	"errors"
	"testing"

	"github.com/prods/nvimon/internal/model"
)

type stubGPUBackend struct {
	gpus      []model.GPUSnapshot
	processes []model.GPUProcess
	errs      []model.CollectorError
	err       error
}

func (s stubGPUBackend) Collect(context.Context) (string, []model.GPUSnapshot, []model.GPUProcess, []model.CollectorError, error) {
	return "stub", s.gpus, s.processes, s.errs, s.err
}

func TestPreferredGPUCollectorFallsBack(t *testing.T) {
	c := &preferredGPUCollector{
		primary: stubGPUBackend{err: errors.New("nvml failed")},
		fallback: stubGPUBackend{
			gpus: []model.GPUSnapshot{{Index: 0, UUID: "GPU-0"}},
		},
	}

	backend, gpus, _, errs, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("collect: %v", err)
	}
	if backend != "stub" {
		t.Fatalf("backend = %q, want stub", backend)
	}
	if len(gpus) != 1 {
		t.Fatalf("gpu count = %d, want 1", len(gpus))
	}
	if len(errs) == 0 || errs[0].Component != "nvml" {
		t.Fatalf("fallback errs = %+v, want nvml fallback note", errs)
	}
}

func TestPreferredGPUCollectorUsesPrimaryOnSuccess(t *testing.T) {
	c := &preferredGPUCollector{
		primary: stubGPUBackend{
			gpus: []model.GPUSnapshot{{Index: 0, UUID: "GPU-0"}},
		},
		fallback: stubGPUBackend{err: errors.New("should not be used")},
	}

	backend, gpus, _, _, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("collect: %v", err)
	}
	if backend != "stub" {
		t.Fatalf("backend = %q, want stub", backend)
	}
	if len(gpus) != 1 || gpus[0].UUID != "GPU-0" {
		t.Fatalf("gpus = %+v", gpus)
	}
}
