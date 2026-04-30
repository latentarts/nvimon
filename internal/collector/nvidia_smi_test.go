package collector

import (
	"errors"
	"os/exec"
	"strings"
	"testing"

	"github.com/prods/nvimon/internal/model"
)

func TestParseGPUCSV(t *testing.T) {
	raw := []byte("0, GPU-0, RTX 4090, 87, 42, 12000, 24564, 70, 55, 250.5, 450.0, 2100, 10500, P2, Disabled\n")

	gpus, indexByUUID, err := parseGPUCSV(raw)
	if err != nil {
		t.Fatalf("parseGPUCSV: %v", err)
	}

	if len(gpus) != 1 {
		t.Fatalf("gpu count = %d, want 1", len(gpus))
	}

	if indexByUUID["GPU-0"] != 0 {
		t.Fatalf("gpu index map = %+v", indexByUUID)
	}

	if !gpus[0].GPUUtilPct.IsKnown() || gpus[0].MemoryUsedBytes == 0 {
		t.Fatalf("gpu row not parsed correctly: %+v", gpus[0])
	}
}

func TestParseProcessCSV(t *testing.T) {
	raw := []byte("1234, GPU-0, 4096\n")

	processes, errs := parseProcessCSV(raw, map[string]int{"GPU-0": 0})
	if len(errs) != 1 {
		t.Fatalf("collector errors = %d, want 1 for missing /proc pid", len(errs))
	}

	if len(processes) != 1 {
		t.Fatalf("process count = %d, want 1", len(processes))
	}

	if processes[0].GPUIndex != 0 || processes[0].SMUtilPct.Status != model.MetricStatusUnknown {
		t.Fatalf("process row = %+v", processes[0])
	}
}

func TestParseMetric(t *testing.T) {
	if got := parseMetric("N/A"); got.Status != model.MetricStatusUnknown {
		t.Fatalf("N/A status = %s, want unknown", got.Status)
	}

	if got := parseMetric("[Not Supported]"); got.Status != model.MetricStatusUnsupported {
		t.Fatalf("unsupported status = %s, want unsupported", got.Status)
	}

	if got := parseMetric("88.5"); !got.IsKnown() || got.Value != 88.5 {
		t.Fatalf("known metric = %+v", got)
	}
}

func TestClassifyNvidiaSMIError(t *testing.T) {
	err := classifyNvidiaSMIError("gpu-query", &exec.ExitError{})
	if err == nil {
		t.Fatal("expected classified error")
	}

	got := classifyNvidiaSMIError("gpu-query", errors.New("exit status 9")).Error()
	if got == "" || !containsAll(got, "gpu-query failed", "NVIDIA driver") {
		t.Fatalf("classified error = %q", got)
	}
}

func TestParseGPUCSVEmptyNormalizesToEmptySlice(t *testing.T) {
	gpus, indexByUUID, err := parseGPUCSV([]byte(""))
	if err != nil {
		t.Fatalf("parseGPUCSV empty: %v", err)
	}
	if gpus == nil || len(gpus) != 0 {
		t.Fatalf("gpus = %+v, want empty slice", gpus)
	}
	if indexByUUID == nil || len(indexByUUID) != 0 {
		t.Fatalf("index map = %+v, want empty map", indexByUUID)
	}
}

func TestParseProcessCSVEmptyNormalizesToEmptySlice(t *testing.T) {
	processes, errs := parseProcessCSV([]byte(""), map[string]int{})
	if errs != nil && len(errs) != 0 {
		t.Fatalf("errs = %+v, want none", errs)
	}
	if processes == nil || len(processes) != 0 {
		t.Fatalf("processes = %+v, want empty slice", processes)
	}
}

func containsAll(s string, subs ...string) bool {
	for _, sub := range subs {
		if !strings.Contains(s, sub) {
			return false
		}
	}
	return true
}
