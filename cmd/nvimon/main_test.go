package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/prods/nvimon/internal/model"
)

func TestPrintSnapshotTextIncludesBackend(t *testing.T) {
	var buf bytes.Buffer
	snapshot := model.HostSnapshot{
		Hostname:      "gpu-a",
		GPUBackend:    "nvidia-smi",
		CPUUsedPct:    model.NewMetricValue(42),
		RAMUsedBytes:  2,
		RAMTotalBytes: 4,
	}

	err := printSnapshotBuffer(&buf, snapshot, false)
	if err != nil {
		t.Fatalf("print snapshot: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "backend=nvidia-smi") {
		t.Fatalf("output missing backend: %q", out)
	}
}

func TestPrintSnapshotJSONIncludesBackend(t *testing.T) {
	var buf bytes.Buffer
	snapshot := model.HostSnapshot{
		Hostname:   "gpu-a",
		GPUBackend: "sample",
	}

	err := printSnapshotBuffer(&buf, snapshot, true)
	if err != nil {
		t.Fatalf("print snapshot json: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "\"gpu_backend\": \"sample\"") {
		t.Fatalf("json missing backend: %q", out)
	}
}
