package model

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestMemoryUsedPct(t *testing.T) {
	gpu := GPUSnapshot{
		MemoryUsedBytes:  4,
		MemoryTotalBytes: 8,
	}

	got := gpu.MemoryUsedPct()
	if !got.IsKnown() || got.Value != 50 {
		t.Fatalf("memory used pct = %+v, want 50%%", got)
	}
}

func TestStaleAt(t *testing.T) {
	now := time.Now()
	snapshot := HostSnapshot{
		Timestamp:      now.Add(-4 * time.Second),
		SampleInterval: time.Second,
	}

	if !snapshot.StaleAt(now) {
		t.Fatal("snapshot should be stale")
	}
}

func TestMarshalSnapshot(t *testing.T) {
	snapshot := HostSnapshot{
		HostID:         "host-a",
		Hostname:       "gpu-a",
		Timestamp:      time.Unix(100, 0).UTC(),
		CPUUsedPct:     NewMetricValue(73.25),
		RAMUsedBytes:   4 * 1024 * 1024,
		RAMTotalBytes:  8 * 1024 * 1024,
		SampleInterval: time.Second,
		GPUs: []GPUSnapshot{
			{
				Index:      0,
				UUID:       "GPU-0",
				Name:       "RTX",
				GPUUtilPct: NewMetricValue(90),
			},
		},
	}

	data, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatalf("marshal snapshot: %v", err)
	}

	if len(data) == 0 {
		t.Fatal("marshal snapshot returned empty payload")
	}

	if string(data) == "" || !json.Valid(data) {
		t.Fatalf("invalid json payload: %q", string(data))
	}
	if !containsJSONField(data, "\"gpus\":[]") && !containsJSONField(data, "\"gpus\":[") {
		t.Fatalf("expected gpus field in payload: %s", string(data))
	}
}

func TestFormatBytes(t *testing.T) {
	got := FormatBytes(3 * 1024 * 1024 * 1024)
	if got != "3.0 GiB" {
		t.Fatalf("FormatBytes = %q, want %q", got, "3.0 GiB")
	}
}

func TestMarshalSnapshotEmptySlicesNotNull(t *testing.T) {
	snapshot := HostSnapshot{
		HostID:       "host-a",
		Hostname:     "gpu-a",
		GPUBackend:   "none",
		Timestamp:    time.Unix(100, 0).UTC(),
		GPUs:         []GPUSnapshot{},
		GPUProcesses: []GPUProcess{},
	}

	data, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatalf("marshal snapshot: %v", err)
	}

	if !containsJSONField(data, "\"gpus\":[]") {
		t.Fatalf("expected empty gpus slice, got %s", string(data))
	}
	if !containsJSONField(data, "\"gpu_processes\":[]") {
		t.Fatalf("expected empty gpu_processes slice, got %s", string(data))
	}
}

func containsJSONField(data []byte, want string) bool {
	return string(data) != "" && strings.Contains(string(data), want)
}
